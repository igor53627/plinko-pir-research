package main

import (
	"sort"
	"testing"
)

// ========================================
// ENHANCED iPRF TESTS (Bug 2)
// ========================================
// These tests validate the EnhancedIPRF composition: iF.F = S ∘ P
// Testing the complete iPRF as specified in the paper

// TestEnhancedIPRFInverseSpace validates inverse returns elements in original space
// Bug 2: InverseFixed returns data in wrong space (permuted vs original)
// Expected: CHECK - This bug may already be fixed
func TestEnhancedIPRFInverseSpace(t *testing.T) {
	testCases := []struct {
		name string
		n    uint64
		m    uint64
	}{
		{"small", 100, 10},
		{"medium", 1000, 100},
		{"large", 10000, 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prpKey := GenerateDeterministicKeyWithSeed(42)
			baseKey := GenerateDeterministicKeyWithSeed(24)
			eiprf := NewEnhancedIPRF(prpKey, baseKey, tc.n, tc.m)

			t.Run("preimages in original space", func(t *testing.T) {
				// Test several outputs
				testOutputs := tc.m
				if testOutputs > 100 {
					testOutputs = 100
				}

				stride := tc.m / testOutputs
				if stride == 0 {
					stride = 1
				}

				for i := uint64(0); i < testOutputs; i++ {
					y := i * stride
					if y >= tc.m {
						break
					}

					preimages := eiprf.Inverse(y)

					for _, x := range preimages {
						// All preimages must be in ORIGINAL space [0, n)
						if x >= tc.n {
							t.Errorf("Inverse(%d) returned x=%d >= n=%d (out of original domain)",
								y, x, tc.n)
							t.Errorf("This is Bug 2: returning permuted space instead of original space")
						}

						// Each preimage must satisfy Forward(x) = y IN ORIGINAL SPACE
						yCheck := eiprf.Forward(x)
						if yCheck != y {
							t.Errorf("Inverse(%d) returned x=%d, but Forward(%d)=%d ≠ %d",
								y, x, x, yCheck, y)
							t.Errorf("This is Bug 2: preimages are in wrong space")
						}
					}
				}
			})

			t.Run("not in permuted space", func(t *testing.T) {
				// Verify that preimages are NOT in the permuted space
				// If Bug 2 exists, preimages would be permuted values

				prp := NewPRP(prpKey)
				base := NewIPRF(baseKey, tc.n, tc.m)

				// Test a specific output
				y := uint64(0)
				preimages := eiprf.Inverse(y)

				// Get what the base iPRF thinks are the preimages (in permuted space)
				permutedPreimages := base.Inverse(y)

				// If Bug 2 exists, these sets would be identical
				// They should NOT be identical (unless by coincidence for small sets)

				// Convert to sets for comparison
				preimageSet := make(map[uint64]bool)
				for _, x := range preimages {
					preimageSet[x] = true
				}

				permutedSet := make(map[uint64]bool)
				for _, px := range permutedPreimages {
					permutedSet[px] = true
				}

				// Count overlap
				overlap := 0
				for px := range permutedSet {
					if preimageSet[px] {
						overlap++
					}
				}

				// For large enough sets, overlap should be minimal
				if len(preimages) > 5 {
					overlapRatio := float64(overlap) / float64(len(preimages))
					if overlapRatio > 0.5 {
						t.Logf("Warning: High overlap (%.1f%%) between original and permuted preimages for y=%d",
							overlapRatio*100, y)
						t.Logf("Original preimages: %v", preimages[:min(10, len(preimages))])
						t.Logf("Permuted preimages: %v", permutedPreimages[:min(10, len(permutedPreimages))])
						t.Logf("This may indicate Bug 2: returning permuted space instead of original")
					}
				}

				// The correct preimages should be: P^-1(permutedPreimage) for each permutedPreimage
				expectedPreimages := make([]uint64, 0, len(permutedPreimages))
				for _, px := range permutedPreimages {
					x := prp.InversePermute(px, tc.n)
					expectedPreimages = append(expectedPreimages, x)
				}

				sort.Slice(expectedPreimages, func(i, j int) bool {
					return expectedPreimages[i] < expectedPreimages[j]
				})

				// Compare with actual preimages
				actualSorted := make([]uint64, len(preimages))
				copy(actualSorted, preimages)
				sort.Slice(actualSorted, func(i, j int) bool {
					return actualSorted[i] < actualSorted[j]
				})

				if len(expectedPreimages) != len(actualSorted) {
					t.Errorf("Inverse(%d) returned %d preimages, expected %d",
						y, len(actualSorted), len(expectedPreimages))
				}

				// Check contents match
				matches := 0
				for i := 0; i < len(expectedPreimages) && i < len(actualSorted); i++ {
					if expectedPreimages[i] == actualSorted[i] {
						matches++
					}
				}

				if matches != len(expectedPreimages) {
					t.Errorf("Only %d/%d preimages match expected values",
						matches, len(expectedPreimages))
					if len(expectedPreimages) <= 10 {
						t.Errorf("Expected: %v", expectedPreimages)
						t.Errorf("Actual: %v", actualSorted)
					}
				}
			})

			t.Run("forward-inverse round trip", func(t *testing.T) {
				// Property: For all x ∈ [0, n), x ∈ Inverse(Forward(x))
				testCount := tc.n
				if testCount > 1000 {
					testCount = 1000
				}

				stride := tc.n / testCount
				if stride == 0 {
					stride = 1
				}

				for i := uint64(0); i < testCount; i++ {
					x := i * stride
					if x >= tc.n {
						break
					}

					y := eiprf.Forward(x)
					preimages := eiprf.Inverse(y)

					found := false
					for _, preimage := range preimages {
						if preimage == x {
							found = true
							break
						}
					}

					if !found {
						t.Errorf("Round trip failed: x=%d → y=%d, but x ∉ Inverse(%d)",
							x, y, y)
						t.Errorf("Inverse(%d) = %v (size=%d)", y, preimages, len(preimages))
						t.Errorf("This may indicate Bug 2: wrong space transformation")
					}
				}
			})
		})
	}
}

// TestEnhancedIPRFComposition validates the composition property
func TestEnhancedIPRFComposition(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(1000)
	m := uint64(100)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	prp := NewPRP(prpKey)
	base := NewIPRF(baseKey, n, m)

	t.Run("forward composition correctness", func(t *testing.T) {
		// Property: eiprf.Forward(x) = base.Forward(prp.Permute(x, n))
		for x := uint64(0); x < 100; x++ {
			yExpected := base.Forward(prp.Permute(x, n))
			yActual := eiprf.Forward(x)

			if yActual != yExpected {
				t.Errorf("Forward composition failed: eiprf.Forward(%d)=%d, expected %d",
					x, yActual, yExpected)
			}
		}
	})

	t.Run("inverse composition correctness", func(t *testing.T) {
		// Property: eiprf.Inverse(y) = {P^-1(x) : x ∈ base.Inverse(y)}
		for y := uint64(0); y < 10; y++ {
			// Get permuted preimages from base
			permutedPreimages := base.Inverse(y)

			// Apply inverse PRP to get original preimages
			expectedPreimages := make([]uint64, 0, len(permutedPreimages))
			for _, px := range permutedPreimages {
				x := prp.InversePermute(px, n)
				expectedPreimages = append(expectedPreimages, x)
			}

			sort.Slice(expectedPreimages, func(i, j int) bool {
				return expectedPreimages[i] < expectedPreimages[j]
			})

			// Get actual preimages
			actualPreimages := eiprf.Inverse(y)
			sort.Slice(actualPreimages, func(i, j int) bool {
				return actualPreimages[i] < actualPreimages[j]
			})

			// Compare
			if len(actualPreimages) != len(expectedPreimages) {
				t.Errorf("Inverse(%d) size mismatch: got %d, expected %d",
					y, len(actualPreimages), len(expectedPreimages))
				continue
			}

			for i := range expectedPreimages {
				if actualPreimages[i] != expectedPreimages[i] {
					t.Errorf("Inverse(%d)[%d] = %d, expected %d",
						y, i, actualPreimages[i], expectedPreimages[i])
				}
			}
		}
	})
}

// TestEnhancedIPRFCorrectness validates complete correctness properties
func TestEnhancedIPRFCorrectness(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(1000)
	m := uint64(100)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("complete forward mapping", func(t *testing.T) {
		// Build complete forward mapping
		forwardMap := make(map[uint64][]uint64)
		for x := uint64(0); x < n; x++ {
			y := eiprf.Forward(x)
			if y >= m {
				t.Errorf("Forward(%d) = %d >= m=%d", x, y, m)
			}
			forwardMap[y] = append(forwardMap[y], x)
		}

		// Check total coverage
		totalMapped := 0
		for _, xs := range forwardMap {
			totalMapped += len(xs)
		}

		if totalMapped != int(n) {
			t.Errorf("Forward mapping incomplete: %d/%d elements mapped", totalMapped, n)
		}
	})

	t.Run("inverse matches forward", func(t *testing.T) {
		// Build forward mapping
		forwardMap := make(map[uint64][]uint64)
		for x := uint64(0); x < n; x++ {
			y := eiprf.Forward(x)
			forwardMap[y] = append(forwardMap[y], x)
		}

		// Sort forward map
		for y := range forwardMap {
			sort.Slice(forwardMap[y], func(i, j int) bool {
				return forwardMap[y][i] < forwardMap[y][j]
			})
		}

		// Compare with inverse
		for y := uint64(0); y < m; y++ {
			expected := forwardMap[y]
			actual := eiprf.Inverse(y)

			sort.Slice(actual, func(i, j int) bool {
				return actual[i] < actual[j]
			})

			if len(actual) != len(expected) {
				t.Errorf("Inverse(%d) size mismatch: got %d, expected %d",
					y, len(actual), len(expected))
				continue
			}

			for i := range expected {
				if actual[i] != expected[i] {
					t.Errorf("Inverse(%d)[%d] = %d, expected %d",
						y, i, actual[i], expected[i])
				}
			}
		}
	})

	t.Run("bijection on domain", func(t *testing.T) {
		// Check that every element in [0, n) appears exactly once
		elementsSeen := make(map[uint64]int)

		for y := uint64(0); y < m; y++ {
			preimages := eiprf.Inverse(y)
			for _, x := range preimages {
				elementsSeen[x]++
			}
		}

		// Check coverage
		for x := uint64(0); x < n; x++ {
			count := elementsSeen[x]
			if count != 1 {
				t.Errorf("Element %d appears %d times (expected 1)", x, count)
			}
		}
	})
}

// TestInverseVsInverseFixed compares the two inverse implementations
func TestInverseVsInverseFixed(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(500)
	m := uint64(50)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("implementations match", func(t *testing.T) {
		for y := uint64(0); y < m; y++ {
			inv1 := eiprf.Inverse(y)
			inv2 := eiprf.InverseFixed(y)

			// Sort both
			sort.Slice(inv1, func(i, j int) bool {
				return inv1[i] < inv1[j]
			})
			sort.Slice(inv2, func(i, j int) bool {
				return inv2[i] < inv2[j]
			})

			if len(inv1) != len(inv2) {
				t.Errorf("Inverse vs InverseFixed size mismatch for y=%d: %d vs %d",
					y, len(inv1), len(inv2))
				continue
			}

			for i := range inv1 {
				if inv1[i] != inv2[i] {
					t.Errorf("Inverse(%d)[%d]: Inverse=%d, InverseFixed=%d",
						y, i, inv1[i], inv2[i])
				}
			}
		}
	})
}

// TestEnhancedIPRFDeterminism validates deterministic behavior
func TestEnhancedIPRFDeterminism(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(1000)
	m := uint64(100)

	t.Run("same keys produce same results", func(t *testing.T) {
		eiprf1 := NewEnhancedIPRF(prpKey, baseKey, n, m)
		eiprf2 := NewEnhancedIPRF(prpKey, baseKey, n, m)

		for x := uint64(0); x < 100; x++ {
			y1 := eiprf1.Forward(x)
			y2 := eiprf2.Forward(x)

			if y1 != y2 {
				t.Errorf("Forward(%d) non-deterministic: %d vs %d", x, y1, y2)
			}
		}

		for y := uint64(0); y < 10; y++ {
			inv1 := eiprf1.Inverse(y)
			inv2 := eiprf2.Inverse(y)

			if len(inv1) != len(inv2) {
				t.Errorf("Inverse(%d) non-deterministic size: %d vs %d",
					y, len(inv1), len(inv2))
				continue
			}

			for i := range inv1 {
				if inv1[i] != inv2[i] {
					t.Errorf("Inverse(%d)[%d] non-deterministic: %d vs %d",
						y, i, inv1[i], inv2[i])
				}
			}
		}
	})

	t.Run("different keys produce different results", func(t *testing.T) {
		prpKey2 := GenerateDeterministicKeyWithSeed(99)
		eiprf1 := NewEnhancedIPRF(prpKey, baseKey, n, m)
		eiprf2 := NewEnhancedIPRF(prpKey2, baseKey, n, m)

		differences := 0
		for x := uint64(0); x < 100; x++ {
			y1 := eiprf1.Forward(x)
			y2 := eiprf2.Forward(x)

			if y1 != y2 {
				differences++
			}
		}

		// Most outputs should differ with different keys
		if differences < 50 {
			t.Errorf("Different keys produced only %d/100 different outputs", differences)
		}
	})
}

// TestEnhancedIPRFEdgeCases validates edge case handling
func TestEnhancedIPRFEdgeCases(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)

	t.Run("n=m case", func(t *testing.T) {
		// When n=m, expected ~1 element per bin
		n := uint64(100)
		m := uint64(100)
		eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

		// Check distribution
		for y := uint64(0); y < m; y++ {
			preimages := eiprf.Inverse(y)
			// Most bins should have 0 or 1 element
			if len(preimages) > 5 {
				t.Logf("Warning: Bin %d has %d elements (n=m case)", y, len(preimages))
			}
		}
	})

	t.Run("m=1 case", func(t *testing.T) {
		// All elements map to single output
		n := uint64(100)
		m := uint64(1)
		eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

		// All elements should map to 0
		for x := uint64(0); x < n; x++ {
			y := eiprf.Forward(x)
			if y != 0 {
				t.Errorf("Forward(%d) = %d, expected 0 (m=1 case)", x, y)
			}
		}

		// Inverse(0) should return all elements
		preimages := eiprf.Inverse(0)
		if uint64(len(preimages)) != n {
			t.Errorf("Inverse(0) has %d elements, expected n=%d", len(preimages), n)
		}
	})

	t.Run("large m/n ratio", func(t *testing.T) {
		// Many bins will be empty
		n := uint64(100)
		m := uint64(1000)
		eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

		nonEmpty := 0
		for y := uint64(0); y < m; y++ {
			preimages := eiprf.Inverse(y)
			if len(preimages) > 0 {
				nonEmpty++
			}
		}

		// Should have roughly n non-empty bins
		if nonEmpty < 50 || nonEmpty > 150 {
			t.Errorf("Expected ~100 non-empty bins, got %d", nonEmpty)
		}
	})
}

// Note: min() helper function is defined in plinko_inverse_demo.go
