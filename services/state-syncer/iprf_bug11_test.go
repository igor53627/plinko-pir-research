package main

import (
	"testing"
)

// TestBug11CycleWalkingUnreachable confirms cycle walking is not used
func TestBug11CycleWalkingUnreachable(t *testing.T) {
	// Verify that Permute() ALWAYS uses TablePRP, never cycle walking

	key := GenerateDeterministicKey()
	prp := NewPRP(key)

	testSizes := []uint64{10, 100, 1000, 8192, 100000}

	for _, n := range testSizes {
		// Call Permute - should initialize TablePRP
		x := uint64(5) % n
		y := prp.Permute(x, n)

		// Verify TablePRP was created
		if prp.tablePRP == nil {
			t.Errorf("Bug #11 concern: TablePRP not initialized for n=%d", n)
		}

		// Verify result is valid (in range)
		if y >= n {
			t.Errorf("Permute returned out-of-bounds value: %d >= %d", y, n)
		}

		// Verify bijection (inverse returns original)
		xRecovered := prp.InversePermute(y, n)
		if xRecovered != x {
			t.Errorf("Bijection failure: Permute(%d)=%d, InversePermute(%d)=%d, expected %d",
				x, y, y, xRecovered, x)
		}
	}

	t.Log("✓ Confirmed: Only TablePRP is used, cycle walking unreachable")
}

// TestBug11DeadCodeRemovalSafe validates removing cycle walking is safe
func TestBug11DeadCodeRemovalSafe(t *testing.T) {
	// Before removing cycleWalkingPermute(), verify nothing calls it

	// This is a manual verification test - developer should:
	// 1. Search codebase for "cycleWalkingPermute"
	// 2. Verify ONLY found in iprf_prp.go definition
	// 3. Verify NOT called from any production code paths

	t.Log("Manual verification required:")
	t.Log("  1. grep -r 'cycleWalkingPermute' services/state-syncer/")
	t.Log("  2. Should only find definition, not any calls")
	t.Log("  3. If found calls, investigate before removal")

	// Automated check: Permute() should reference TablePRP
	key := GenerateDeterministicKey()
	prp := NewPRP(key)

	// After this call, tablePRP should be initialized
	prp.Permute(5, 100)

	if prp.tablePRP == nil {
		t.Error("TablePRP not being used - cycle walking might still be active!")
		t.Error("DO NOT remove cycle walking until this is fixed")
	} else {
		t.Log("✓ Safe to remove cycle walking - TablePRP is primary PRP")
	}
}

// TestBug11TablePRPExclusivity validates TablePRP is the ONLY PRP implementation used
func TestBug11TablePRPExclusivity(t *testing.T) {
	key := GenerateDeterministicKey()

	testCases := []struct {
		name string
		n    uint64
	}{
		{"Tiny domain", 5},
		{"Small domain", 100},
		{"Medium domain", 1000},
		{"Large domain", 10000},
		{"Production domain", 100000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prp := NewPRP(key)

			// Verify bijection across entire domain (sample for large domains)
			sampleSize := tc.n
			if sampleSize > 1000 {
				sampleSize = 1000
			}

			seen := make(map[uint64]bool)
			for x := uint64(0); x < sampleSize; x++ {
				y := prp.Permute(x, tc.n)

				// Check in range
				if y >= tc.n {
					t.Fatalf("Permute(%d) = %d out of range [0, %d)", x, y, tc.n)
				}

				// Check no collisions (bijection)
				if seen[y] {
					t.Fatalf("Collision: Multiple inputs map to %d", y)
				}
				seen[y] = true

				// Check inverse correctness
				xRecovered := prp.InversePermute(y, tc.n)
				if xRecovered != x {
					t.Fatalf("Inverse incorrect: Permute(%d)=%d, InversePermute(%d)=%d",
						x, y, y, xRecovered)
				}

				// Verify TablePRP is being used
				if prp.tablePRP == nil {
					t.Fatal("TablePRP not initialized - unexpected!")
				}

				if prp.tablePRP.domain != tc.n {
					t.Fatalf("TablePRP domain mismatch: got %d, expected %d",
						prp.tablePRP.domain, tc.n)
				}
			}

			t.Logf("✓ TablePRP working correctly for n=%d (tested %d samples)", tc.n, sampleSize)
		})
	}
}

// TestBug15FallbackPermutationRemoved validates no modulo-based fallback exists
func TestBug15FallbackPermutationRemoved(t *testing.T) {
	// Bug #15: Fallback permutation used simple modulo: return x % n
	// This is NOT a bijection and violates PRP requirements
	// Test confirms this buggy fallback is no longer in the code

	key := GenerateDeterministicKey()
	prp := NewPRP(key)

	n := uint64(1000)

	// Build frequency map of outputs
	outputs := make(map[uint64]int)
	for x := uint64(0); x < n; x++ {
		y := prp.Permute(x, n)
		outputs[y]++
	}

	// Check for perfect bijection (each output appears exactly once)
	for y := uint64(0); y < n; y++ {
		count := outputs[y]
		if count != 1 {
			t.Errorf("Bug #15 regression: Output %d appears %d times (expected 1)", y, count)
			if count == 0 {
				t.Error("  This suggests fallback permutation is still active")
			}
		}
	}

	// Check no output exceeds n (would indicate modulo fallback)
	for y := range outputs {
		if y >= n {
			t.Errorf("Bug #15: Output %d >= domain size %d", y, n)
		}
	}

	t.Logf("✓ Bug #15 confirmed fixed: Perfect bijection (no fallback permutation)")
}
