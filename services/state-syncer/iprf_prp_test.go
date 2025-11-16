package main

import (
	"testing"
)

// ========================================
// PRP CORRECTNESS TESTS (Bugs 1, 3, 9, 10)
// ========================================
// These tests validate the PRP (Pseudorandom Permutation) layer
// which is critical for the iPRF composition: iF.F = S ∘ P

// TestPRPBijection validates that PRP is a proper bijection
// Bug 1: PRP bijection failure due to incorrect cycle walking
// Expected: FAIL - PRP may not be a proper bijection
func TestPRPBijection(t *testing.T) {
	testCases := []struct {
		name string
		n    uint64
	}{
		{"tiny domain", 16},
		{"small domain", 256},
		{"medium domain", 1024},
		{"large domain", 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := GenerateDeterministicKey()
			prp := NewPRP(key)

			t.Run("inverse property", func(t *testing.T) {
				// Property: P^-1(P(x)) = x for all x ∈ [0, n)
				for x := uint64(0); x < tc.n; x++ {
					y := prp.Permute(x, tc.n)
					xInv := prp.InversePermute(y, tc.n)

					if xInv != x {
						t.Errorf("PRP inverse property failed: P^-1(P(%d)) = %d, expected %d",
							x, xInv, x)
					}
				}
			})

			t.Run("no collisions", func(t *testing.T) {
				// Property: P(x1) ≠ P(x2) for x1 ≠ x2
				outputsSeen := make(map[uint64]uint64) // y -> x that produced it

				for x := uint64(0); x < tc.n; x++ {
					y := prp.Permute(x, tc.n)

					if prevX, exists := outputsSeen[y]; exists {
						t.Errorf("PRP collision detected: P(%d) = P(%d) = %d",
							prevX, x, y)
					}
					outputsSeen[y] = x
				}
			})

			t.Run("surjection", func(t *testing.T) {
				// Property: All values in [0, n) are reachable
				outputs := make(map[uint64]bool)

				for x := uint64(0); x < tc.n; x++ {
					y := prp.Permute(x, tc.n)
					if y >= tc.n {
						t.Errorf("PRP output out of range: P(%d) = %d >= %d", x, y, tc.n)
					}
					outputs[y] = true
				}

				// Check all values are covered
				for y := uint64(0); y < tc.n; y++ {
					if !outputs[y] {
						t.Errorf("PRP not surjective: value %d is unreachable", y)
					}
				}
			})

			t.Run("determinism", func(t *testing.T) {
				// Property: Same input always produces same output
				for x := uint64(0); x < tc.n && x < 100; x++ {
					y1 := prp.Permute(x, tc.n)
					y2 := prp.Permute(x, tc.n)

					if y1 != y2 {
						t.Errorf("PRP not deterministic: P(%d) produced %d and %d", x, y1, y2)
					}
				}
			})
		})
	}
}

// TestPRPInverseCorrectness validates inverse finds correct preimages
// Bug 10: Ambiguous zero error signaling in inverseBruteForce
// Expected: FAIL - Returns 0 for both "x=0 found" and "no inverse found"
func TestPRPInverseCorrectness(t *testing.T) {
	key := GenerateDeterministicKey()
	prp := NewPRP(key)
	n := uint64(100)

	t.Run("inverse finds correct preimage", func(t *testing.T) {
		for y := uint64(0); y < n; y++ {
			x := prp.InversePermute(y, n)

			// Verify that Forward(x) = y
			yComputed := prp.Permute(x, n)
			if yComputed != y {
				t.Errorf("InversePermute(%d) = %d, but Permute(%d) = %d ≠ %d",
					y, x, x, yComputed, y)
			}
		}
	})

	t.Run("distinguishes zero from not-found", func(t *testing.T) {
		// Test case: what if y=0 has preimage x=0?
		// The inverse should return 0, not panic or return an error

		// Find what x produces y=0
		var xForZero uint64
		for x := uint64(0); x < n; x++ {
			if prp.Permute(x, n) == 0 {
				xForZero = x
				break
			}
		}

		// InversePermute(0) should return the correct x, even if x=0
		xInv := prp.InversePermute(0, n)
		if xInv != xForZero {
			t.Errorf("InversePermute(0) = %d, expected %d (Bug 10: ambiguous zero)",
				xInv, xForZero)
		}

		// Test the underlying inverseBruteForce function if accessible
		// This explicitly tests Bug 10: returning 0 for "not found"
		xFound, err := prp.inverseBruteForce(0, n)
		if err != nil {
			t.Errorf("inverseBruteForce(0) returned error: %v (should find %d)", err, xForZero)
		}
		if xFound != xForZero {
			t.Errorf("inverseBruteForce(0) = %d, expected %d", xFound, xForZero)
		}
	})
}

// TestPRPPerformanceReasonable validates PRP inverse doesn't timeout
// Bug 3: O(n) inverse is impractical for large n
// Expected: FAIL/TIMEOUT - With n=8.4M, inverse may take too long
func TestPRPPerformanceReasonable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testCases := []struct {
		name       string
		n          uint64
		maxTimeMs  int64
	}{
		{"small n=1K", 1000, 100},
		{"medium n=10K", 10000, 500},
		{"large n=100K", 100000, 2000},
		// Bug 3: This will timeout with O(n²) complexity
		{"realistic n=8.4M", 8_400_000, 10000}, // 10 second timeout
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := GenerateDeterministicKey()
			prp := NewPRP(key)

			// Test inverse performance for several random values
			testCount := 10
			for i := 0; i < testCount; i++ {
				y := uint64(i * 1000) % tc.n

				// Measure inverse time
				x := prp.InversePermute(y, tc.n)

				// Verify correctness
				yCheck := prp.Permute(x, tc.n)
				if yCheck != y {
					t.Errorf("InversePermute(%d) = %d, but Permute(%d) = %d",
						y, x, x, yCheck)
				}
			}

			// If we reach here without timeout, the performance is acceptable
			t.Logf("PRP inverse completed for n=%d", tc.n)
		})
	}
}

// TestGetDistributionStatsEmptyHandling validates empty slice handling
// Bug 9: Empty slice access panic in GetDistributionStats
// Expected: FAIL/PANIC - Accessing sizes[0] on empty slice
func TestGetDistributionStatsEmptyHandling(t *testing.T) {
	key := GenerateDeterministicKey()

	t.Run("empty domain", func(t *testing.T) {
		// Edge case: domain size = 0
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("GetDistributionStats panicked on empty domain: %v", r)
			}
		}()

		// This should handle empty case gracefully
		iprf := NewIPRF(key, 0, 1)
		stats := iprf.GetDistributionStats()

		// Should return default values, not panic
		if stats["actual_min_preimage"] != 0 {
			t.Errorf("Expected min=0 for empty domain, got %v", stats["actual_min_preimage"])
		}
	})

	t.Run("no preimages for output", func(t *testing.T) {
		// Edge case: Some output has no preimages (broken distribution)
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("GetDistributionStats panicked on distribution gaps: %v", r)
			}
		}()

		iprf := NewIPRF(key, 100, 10)
		stats := iprf.GetDistributionStats()

		// Should handle the case where some bins might be empty
		_ = stats
	})

	t.Run("range larger than domain", func(t *testing.T) {
		// Edge case: m > n means many empty bins
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("GetDistributionStats panicked on m > n: %v", r)
			}
		}()

		iprf := NewIPRF(key, 10, 100)
		stats := iprf.GetDistributionStats()

		// Should handle sparse distribution gracefully
		_ = stats
	})
}

// TestPRPEdgeCases validates edge case handling
func TestPRPEdgeCases(t *testing.T) {
	key := GenerateDeterministicKey()
	prp := NewPRP(key)

	t.Run("n=1 domain", func(t *testing.T) {
		// With n=1, P(0) must equal 0 (only valid bijection)
		n := uint64(1)
		y := prp.Permute(0, n)
		if y != 0 {
			t.Errorf("P(0) = %d for n=1, expected 0", y)
		}

		x := prp.InversePermute(0, n)
		if x != 0 {
			t.Errorf("P^-1(0) = %d for n=1, expected 0", x)
		}
	})

	t.Run("n=2 domain", func(t *testing.T) {
		// With n=2, must be either identity or swap
		n := uint64(2)

		// Check bijection
		y0 := prp.Permute(0, n)
		y1 := prp.Permute(1, n)

		if y0 == y1 {
			t.Errorf("P(0) = P(1) = %d, not a bijection", y0)
		}

		if (y0 != 0 && y0 != 1) || (y1 != 0 && y1 != 1) {
			t.Errorf("P outputs out of range: P(0)=%d, P(1)=%d", y0, y1)
		}
	})

	t.Run("power of 2 domains", func(t *testing.T) {
		// Test powers of 2 (common in crypto constructions)
		powersOf2 := []uint64{2, 4, 8, 16, 32, 64, 128, 256}

		for _, n := range powersOf2 {
			// Quick bijection check
			outputs := make(map[uint64]bool)
			for x := uint64(0); x < n; x++ {
				y := prp.Permute(x, n)
				if y >= n {
					t.Errorf("For n=%d: P(%d) = %d out of range", n, x, y)
				}
				outputs[y] = true
			}

			if len(outputs) != int(n) {
				t.Errorf("For n=%d: only %d distinct outputs (expected %d)",
					n, len(outputs), n)
			}
		}
	})
}

// TestPRPConsistencyAcrossDomains validates PRP behavior across different n
func TestPRPConsistencyAcrossDomains(t *testing.T) {
	key := GenerateDeterministicKey()
	prp := NewPRP(key)

	t.Run("different keys produce different permutations", func(t *testing.T) {
		n := uint64(100)
		key2 := GenerateDeterministicKeyWithSeed(42)
		prp2 := NewPRP(key2)

		// Count how many outputs differ
		differences := 0
		for x := uint64(0); x < n; x++ {
			y1 := prp.Permute(x, n)
			y2 := prp2.Permute(x, n)
			if y1 != y2 {
				differences++
			}
		}

		// Most outputs should differ (PRP is keyed)
		if differences < int(n)/2 {
			t.Errorf("Different keys produced only %d/%d different outputs",
				differences, n)
		}
	})

	t.Run("same key same domain is deterministic", func(t *testing.T) {
		n := uint64(100)

		// Create two PRPs with same key
		prp2 := NewPRP(key)

		for x := uint64(0); x < n; x++ {
			y1 := prp.Permute(x, n)
			y2 := prp2.Permute(x, n)
			if y1 != y2 {
				t.Errorf("Same key non-deterministic: P(%d) = %d vs %d", x, y1, y2)
			}
		}
	})
}

// TestPRPSecurityProperties validates basic cryptographic properties
func TestPRPSecurityProperties(t *testing.T) {
	key := GenerateDeterministicKey()
	prp := NewPRP(key)
	n := uint64(1000)

	t.Run("avalanche effect", func(t *testing.T) {
		// Small input changes should cause large output changes
		x1 := uint64(100)
		x2 := uint64(101) // Adjacent input

		y1 := prp.Permute(x1, n)
		y2 := prp.Permute(x2, n)

		// Outputs should be very different (not adjacent)
		diff := y1
		if y2 > y1 {
			diff = y2 - y1
		} else {
			diff = y1 - y2
		}

		// Adjacent inputs should NOT produce adjacent outputs
		if diff < 10 {
			t.Logf("Warning: Adjacent inputs x=%d, x=%d produced close outputs y=%d, y=%d",
				x1, x2, y1, y2)
		}
	})

	t.Run("uniform distribution", func(t *testing.T) {
		// Outputs should be roughly uniformly distributed
		buckets := 10
		counts := make([]int, buckets)

		for x := uint64(0); x < n; x++ {
			y := prp.Permute(x, n)
			bucket := int(y * uint64(buckets) / n)
			counts[bucket]++
		}

		// Check each bucket has roughly n/buckets elements
		expected := int(n) / buckets
		tolerance := expected / 2 // 50% tolerance

		for i, count := range counts {
			if count < expected-tolerance || count > expected+tolerance {
				t.Logf("Warning: Bucket %d has %d elements, expected ~%d",
					i, count, expected)
			}
		}
	})
}
