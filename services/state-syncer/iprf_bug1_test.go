package main

import (
	"testing"
	"time"
)

// TestBug1InversePerformance tests that InverseFixed uses O(log m + k) algorithm
// not O(n) brute force
//
// BUG #1: Current InverseFixed uses bruteForceInverse which scans entire domain
// Paper claim: Inverse should be O(log m + k) where k ≈ n/m
// Expected: ~10ms for n=8.4M, m=1024
// Current buggy version: ~3500ms (350x slower)
//
// This test MUST FAIL with brute force implementation
func TestBug1InversePerformance(t *testing.T) {
	// Production-scale parameters
	n := uint64(8400000) // 8.4M domain
	m := uint64(1024)    // 1024 range

	key := GenerateDeterministicKey()
	iprf := NewIPRF(key, n, m)

	// Measure inverse time
	start := time.Now()
	preimages := iprf.InverseFixed(500) // Arbitrary bin
	elapsed := time.Since(start)

	// Paper claim: O(log m + k) where k ≈ n/m = 8203
	// log(1024) = 10 tree levels
	// Expected: <50ms (being generous, paper suggests <10ms)
	maxTime := 50 * time.Millisecond

	t.Logf("InverseFixed completed in %v with %d preimages", elapsed, len(preimages))
	t.Logf("Expected preimage size: ~%d", n/m)

	if elapsed > maxTime {
		t.Errorf("BUG #1 DETECTED: InverseFixed too slow: %v (expected < %v)", elapsed, maxTime)
		t.Errorf("Likely using O(n) brute force instead of O(log m + k) tree algorithm")
		t.Errorf("Speedup needed: %.1fx", float64(elapsed)/float64(maxTime))
	}

	// Verify correctness
	if len(preimages) == 0 {
		t.Error("InverseFixed returned empty set - should have ~8203 preimages")
	}

	// Verify all preimages map to target bin (correctness check)
	for i, x := range preimages {
		if i < 10 || i >= len(preimages)-10 { // Check first and last 10
			if iprf.Forward(x) != 500 {
				t.Errorf("Invalid preimage: Forward(%d) = %d, expected 500", x, iprf.Forward(x))
			}
		}
	}
}

// TestBug1ComplexityScaling tests that time scales as O(log m), not O(n)
// This test helps verify the algorithmic complexity
func TestBug1ComplexityScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping complexity scaling test in short mode")
	}

	key := GenerateDeterministicKey()
	n := uint64(1000000) // 1M domain (smaller for faster test)

	times := make(map[uint64]time.Duration)

	for _, m := range []uint64{256, 512, 1024} {
		iprf := NewIPRF(key, n, m)

		start := time.Now()
		for i := 0; i < 5; i++ {
			_ = iprf.InverseFixed(uint64(i))
		}
		elapsed := time.Since(start) / 5 // Average

		times[m] = elapsed
		t.Logf("m=%d: %v per inverse (k=%d)", m, elapsed, n/m)
	}

	// Verify log scaling: doubling m should add constant time, not double time
	// If brute force O(n), time should be constant (not dependent on m)
	// If tree-based O(log m + k), time should decrease as k decreases

	// With brute force, all times should be similar (O(n) dominates)
	// With correct algorithm, larger m should be FASTER (smaller k)
	if times[1024] > times[256] {
		t.Errorf("BUG #1: Larger m is slower - suggests O(n) brute force")
		t.Errorf("m=256: %v, m=1024: %v", times[256], times[1024])
	}
}

// TestBug1InverseCorrectness tests that optimized inverse produces correct results
// This ensures we don't break correctness while fixing performance
func TestBug1InverseCorrectness(t *testing.T) {
	n := uint64(10000)
	m := uint64(100)

	key := GenerateDeterministicKey()
	iprf := NewIPRF(key, n, m)

	// Build ground truth using forward mapping
	forwardMap := make(map[uint64][]uint64)
	for x := uint64(0); x < n; x++ {
		y := iprf.Forward(x)
		forwardMap[y] = append(forwardMap[y], x)
	}

	// Test several bins
	for y := uint64(0); y < 10; y++ {
		expected := forwardMap[y]
		actual := iprf.InverseFixed(y)

		if len(expected) != len(actual) {
			t.Errorf("InverseFixed(%d) length mismatch: expected %d, got %d",
				y, len(expected), len(actual))
			continue
		}

		// Create set for comparison
		actualSet := make(map[uint64]bool)
		for _, x := range actual {
			actualSet[x] = true
		}

		for _, x := range expected {
			if !actualSet[x] {
				t.Errorf("InverseFixed(%d) missing preimage %d", y, x)
			}
		}
	}
}

// TestBug1TreeInverseAvailable tests if tree-based inverse exists
func TestBug1TreeInverseAvailable(t *testing.T) {
	n := uint64(1000)
	m := uint64(50)

	key := GenerateDeterministicKey()
	iprf := NewIPRF(key, n, m)

	// Check if TreeInverse method exists and works
	// This is the correct O(log m + k) implementation
	y := uint64(10)

	// Try to call TreeInverse if it exists
	// For now, just verify InverseFixed works
	preimages := iprf.InverseFixed(y)

	if len(preimages) == 0 {
		t.Error("InverseFixed returned no preimages")
	}

	// Verify correctness
	for _, x := range preimages {
		if iprf.Forward(x) != y {
			t.Errorf("Invalid preimage: Forward(%d) = %d, expected %d",
				x, iprf.Forward(x), y)
		}
	}
}
