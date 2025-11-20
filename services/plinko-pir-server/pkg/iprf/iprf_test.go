package iprf

import (
	"testing"
)

func TestPMNS(t *testing.T) {
	key := make([]byte, 16)
	n := uint64(100)
	m := uint64(10)
	pmns := NewPMNS(key, n, m)

	// Test S bounds
	for x := uint64(0); x < n; x++ {
		y := pmns.S(x)
		if y >= m {
			t.Errorf("S(%d) = %d, want < %d", x, y, m)
		}
	}

	// Test SInverse consistency
	// For every bin y, SInverse(y) should return a range.
	// Checking if balls in that range actually map to y is hard without checking S(ball) == y
	// But we can check that SInverse returns valid ranges that sum to n?
	// Actually, PMNS is deterministic.
	// Let's check consistency: x \in SInverse(S(x))

	for x := uint64(0); x < n; x++ {
		y := pmns.S(x)
		start, count := pmns.SInverse(y)

		// Check if x is in the range [start, start+count)?
		// NO! PMNS maps x -> y. SInverse(y) returns a set of balls {z} such that S(z) = y.
		// But the balls in SInverse are contiguous in the "permuted" space if we consider the tree structure?
		// Wait, PMNS SInverse returns {start, ..., start+count-1}.
		// These are NOT the original x values. These are the "balls" in the PMNS domain.
		// In iPRF, we permute x before passing to PMNS.
		// So for PMNS itself, the input x IS the ball index.
		// So yes, if we pass x to S, we get y.
		// SInverse(y) gives us a range of x's that map to y.
		// So x MUST be in [start, start+count).

		// However, the PMNS construction in the paper (Figure 4) says:
		// S(k, x) -> returns low (bin index)
		// SInverse(k, y) -> returns {start, ..., start+count-1}
		// And the logic:
		// If x < start + s then node <- left
		// This implies that balls are ordered.
		// So yes, x should be in the range returned by SInverse(S(x)).

		if x < start || x >= start+count {
			t.Errorf("Consistency error: x=%d mapped to y=%d, but SInverse(y) returned [%d, %d)", x, y, start, start+count)
		}
	}
}

func TestPRP(t *testing.T) {
	key := make([]byte, 16)
	n := uint64(1000)
	prp := NewPRP(key, n)

	// Test Permute bounds
	for x := uint64(0); x < n; x++ {
		y := prp.Permute(x)
		if y >= n {
			t.Errorf("Permute(%d) = %d, want < %d", x, y, n)
		}

		// Test Inverse
		inv := prp.Inverse(y)
		if inv != x {
			t.Errorf("Inverse(Permute(%d)) = %d, want %d", x, inv, x)
		}
	}
}

func TestIPRF(t *testing.T) {
	key1 := make([]byte, 16) // PRP key
	key2 := make([]byte, 16) // PMNS key
	key2[0] = 1              // Make them different

	n := uint64(1000)
	m := uint64(50)

	f := New(key1, key2, n, m)

	// Test Forward/Inverse consistency
	for x := uint64(0); x < n; x++ {
		y := f.F(x)

		// Inverse should return a set containing x
		inverseSet := f.Inverse(y)

		found := false
		for _, val := range inverseSet {
			if val == x {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("IPRF consistency error: x=%d mapped to y=%d, but Inverse(y) %v did not contain x", x, y, inverseSet)
		}
	}
}
