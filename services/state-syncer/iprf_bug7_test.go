package main

import (
	"testing"
)

// TestBug7NodeEncodingCollisions specifically tests the n & 0xFFFF truncation bug
// This test MUST FAIL with current implementation and PASS after fix
//
// BUG #7: encodeNode uses (n & 0xFFFF) which truncates n to 16 bits
// This causes n=65536 to collide with n=0, n=65537 with n=1, etc.
func TestBug7NodeEncodingCollisions(t *testing.T) {
	// Test Case 1: Values that differ only in upper bits (modulo 65536 collision)
	testCollisions := []struct {
		name string
		n1   uint64
		n2   uint64
	}{
		{"zero vs 2^16", 0, 65536},
		{"zero vs 2^17", 0, 131072},
		{"1 vs 65537", 1, 65537},
		{"1000 vs 66536", 1000, 66536},
		{"production value collision", 8400000, 8400000 + 65536},
		{"multiple of 2^16", 65536, 131072},
	}

	for _, tc := range testCollisions {
		enc1 := encodeNode(0, 1023, tc.n1)
		enc2 := encodeNode(0, 1023, tc.n2)

		if enc1 == enc2 {
			t.Errorf("BUG #7 DETECTED - %s: encodeNode(0, 1023, %d) == encodeNode(0, 1023, %d) = %d",
				tc.name, tc.n1, tc.n2, enc1)
			t.Logf("n1 & 0xFFFF = %d, n2 & 0xFFFF = %d (values match due to truncation)",
				tc.n1&0xFFFF, tc.n2&0xFFFF)
		} else {
			t.Logf("✓ %s: No collision (%d vs %d)", tc.name, enc1, enc2)
		}
	}
}

// TestBug7ProductionScenario tests the actual production scenario from the paper
// Domain n=8.4M, Range m=1024, which requires proper node identification
func TestBug7ProductionScenario(t *testing.T) {
	n := uint64(8400000)
	m := uint64(1024)

	// Collect all node encodings during a typical tree traversal
	encodings := make(map[uint64]int)
	collisionCount := 0

	// Simulate tree levels (log2(1024) = 10 levels)
	for level := 0; level < 10; level++ {
		nodesAtLevel := 1 << level
		binsPerNode := m / uint64(nodesAtLevel)

		for nodeIdx := 0; nodeIdx < nodesAtLevel; nodeIdx++ {
			low := uint64(nodeIdx) * binsPerNode
			high := low + binsPerNode - 1

			// At each node, we might have different ball counts due to binomial splits
			// Test several ball count scenarios
			ballCounts := []uint64{
				n,
				n / 2,
				n / 4,
				n * 3 / 4,
			}

			for _, bc := range ballCounts {
				if bc == 0 {
					continue
				}
				encoding := encodeNode(low, high, bc)

				if count, exists := encodings[encoding]; exists {
					collisionCount++
					t.Errorf("Collision at level %d, node %d, ballCount %d: encoding %d seen %d times",
						level, nodeIdx, bc, encoding, count+1)
				}
				encodings[encoding]++
			}
		}
	}

	if collisionCount > 0 {
		t.Fatalf("BUG #7: Found %d encoding collisions in production scenario", collisionCount)
	}

	t.Logf("✓ No collisions found across %d unique node encodings", len(encodings))
}

// TestBug7SpecificModuloPattern tests the exact mathematical bug pattern
func TestBug7SpecificModuloPattern(t *testing.T) {
	// The bug is: encoding = (low << 32) | (high << 16) | (n & 0xFFFF)
	// This means any n1, n2 where n1 ≡ n2 (mod 2^16) will collide

	low := uint64(100)
	high := uint64(200)

	// Test values that are congruent modulo 2^16
	baseN := uint64(12345)

	// These should all produce DIFFERENT encodings but will COLLIDE in buggy version
	for i := 0; i < 5; i++ {
		n := baseN + uint64(i)*65536
		encoding := encodeNode(low, high, n)
		t.Logf("encodeNode(%d, %d, %d) = %d (n & 0xFFFF = %d)",
			low, high, n, encoding, n&0xFFFF)

		// Check against base encoding
		if i > 0 {
			baseEncoding := encodeNode(low, high, baseN)
			if encoding == baseEncoding {
				t.Errorf("BUG #7: n=%d collides with n=%d (both have bottom 16 bits = %d)",
					n, baseN, baseN&0xFFFF)
			}
		}
	}
}
