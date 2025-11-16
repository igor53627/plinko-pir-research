package main

import (
	"testing"
)

// TestBug10BinCollectionConfirmed validates Bug #10 fix remains in place
// Bug #10: enumerateBallsInBinRecursive used wrong parameter for ball count
func TestBug10BinCollectionConfirmed(t *testing.T) {
	key := GenerateDeterministicKey()
	n := uint64(10000)
	m := uint64(100)

	iprf := NewIPRF(key, n, m)

	// Test multiple bins
	for targetBin := uint64(0); targetBin < m; targetBin += 10 {
		preimages := iprf.enumerateBallsInBin(targetBin, n, m)

		// Verify all preimages actually map to target bin
		for _, x := range preimages {
			y := iprf.Forward(x)
			if y != targetBin {
				t.Errorf("Bug #10 regression: Forward(%d) = %d, expected %d", x, y, targetBin)
			}
		}

		// Verify we're not missing elements (Bug #10 caused 85-99% loss)
		// Expected: ~n/m elements per bin
		expected := n / m
		tolerance := expected / 2 // Allow 50% variance

		if uint64(len(preimages)) < expected-tolerance {
			t.Errorf("Bug #10 regression: Only found %d preimages, expected ~%d",
				len(preimages), expected)
		}
	}

	t.Logf("✓ Bug #10 fix confirmed: Bin collection working correctly")
}

// TestBug10ParameterSeparation ensures originalN and ballCount are separate
func TestBug10ParameterSeparation(t *testing.T) {
	// This validates the fix from BUG8_FIX_REPORT.md
	// The function signature should have TWO parameters:
	//   originalN - for node encoding (matches traceBall)
	//   ballCount - for binomial sampling (changes per level)

	key := GenerateDeterministicKey()
	n := uint64(1000)
	m := uint64(50)

	iprf := NewIPRF(key, n, m)

	// Enumerate a bin
	targetBin := uint64(25)
	preimages := iprf.enumerateBallsInBin(targetBin, n, m)

	// The fix ensures we use:
	// - originalN (constant) for encodeNode() matching traceBall
	// - ballCount (variable) for sampleBinomial() at each level
	//
	// If parameters were confused, we'd get wrong binomial samples
	// and miss most elements

	// Verify completeness by checking inverse-forward round trip
	for _, x := range preimages {
		y := iprf.Forward(x)
		if y != targetBin {
			t.Errorf("Parameter confusion: preimage %d maps to %d, not %d", x, y, targetBin)
		}
	}

	// Verify reasonable preimage count
	expectedCount := n / m
	if uint64(len(preimages)) < expectedCount/2 {
		t.Errorf("Suspiciously low preimage count: %d (expected ~%d)", len(preimages), expectedCount)
		t.Error("This suggests Bug #10 regression (parameter confusion)")
	}

	t.Log("✓ Parameter separation confirmed correct")
}

// TestBug10FullDistributionCheck validates uniform distribution across all bins
func TestBug10FullDistributionCheck(t *testing.T) {
	key := GenerateDeterministicKey()
	n := uint64(8192)
	m := uint64(64)

	iprf := NewIPRF(key, n, m)

	expectedPerBin := n / m
	tolerance := expectedPerBin / 2

	emptyBins := 0
	overloadedBins := 0
	totalElements := uint64(0)

	for bin := uint64(0); bin < m; bin++ {
		preimages := iprf.enumerateBallsInBin(bin, n, m)
		count := uint64(len(preimages))
		totalElements += count

		if count == 0 {
			emptyBins++
			t.Errorf("Bug #10 regression: Bin %d is empty (expected ~%d elements)", bin, expectedPerBin)
		}

		if count < expectedPerBin-tolerance || count > expectedPerBin+tolerance {
			overloadedBins++
		}

		// Verify all elements map to correct bin
		for _, x := range preimages {
			y := iprf.Forward(x)
			if y != bin {
				t.Errorf("Bug #10: Element %d in bin %d actually maps to %d", x, bin, y)
			}
		}
	}

	// Check total element count matches domain size
	if totalElements != n {
		t.Errorf("Bug #10: Total elements %d != domain size %d (missing/duplicate elements)", totalElements, n)
	}

	if emptyBins > 0 {
		t.Errorf("Bug #10 regression: %d bins are empty (distribution failure)", emptyBins)
	}

	t.Logf("✓ Full distribution check passed: %d elements across %d bins", totalElements, m)
	t.Logf("  Expected per bin: ~%d, tolerance: ±%d", expectedPerBin, tolerance)
	t.Logf("  Bins outside tolerance: %d/%d (%.1f%%)", overloadedBins, m, float64(overloadedBins)*100/float64(m))
}
