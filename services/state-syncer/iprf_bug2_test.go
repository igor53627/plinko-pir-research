package main

import (
	"testing"
)

// TestBug2EnhancedIPRFInverseSpaceCorrect validates that Enhanced iPRF
// inverse returns preimages in the ORIGINAL domain [0, n), not permuted space.
//
// Bug #2 Context: EnhancedIPRF composes PRP and PMNS:
//   Forward:  x → P(x) → S(P(x)) = y
//   Inverse:  y → S⁻¹(y) = {permuted_x} → P⁻¹(permuted_x) = {x}
//
// The bug occurs if inverse returns {permuted_x} instead of {x}.
func TestBug2EnhancedIPRFInverseSpaceCorrect(t *testing.T) {
	// Use deterministic keys for reproducibility
	prpKey := PrfKey128{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	baseKey := PrfKey128{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}

	n := uint64(1000) // Domain size
	m := uint64(100)  // Range size

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	outOfBoundsCount := 0
	missingPreimageCount := 0

	// Test all inputs
	for x := uint64(0); x < n; x++ {
		y := eiprf.Forward(x)
		preimages := eiprf.Inverse(y)

		// CRITICAL CHECK: All preimages must be in original domain [0, n)
		for _, preimage := range preimages {
			if preimage >= n {
				outOfBoundsCount++
				t.Errorf("Bug #2 detected: Preimage %d outside domain [0, %d)", preimage, n)
				t.Errorf("  For x=%d, Forward(x)=%d, got out-of-bounds preimage", x, y)

				// This indicates inverse returned value in permuted space
				if preimage < eiprf.base.domain {
					t.Errorf("  Preimage %d is in base iPRF domain - likely in permuted space!", preimage)
				}
			}
		}

		// CRITICAL CHECK: x must be in its own preimage set
		found := false
		for _, preimage := range preimages {
			if preimage == x {
				found = true
				break
			}
		}

		if !found {
			missingPreimageCount++
			t.Errorf("Bug #2: x=%d not in Inverse(Forward(%d)) = %v", x, x, preimages)
			t.Errorf("  Forward(%d) = %d, but Inverse(%d) doesn't contain %d", x, y, y, x)
		}
	}

	if outOfBoundsCount > 0 {
		t.Fatalf("Bug #2 CONFIRMED: %d out-of-bounds preimages (wrong space)", outOfBoundsCount)
	}

	if missingPreimageCount > 0 {
		t.Fatalf("Bug #2 RELATED: %d missing preimages (inverse incorrect)", missingPreimageCount)
	}

	t.Logf("✓ Bug #2 validation PASSED: All %d preimages in correct space", n)
}

// TestBug2SpaceTransformation validates the mathematical correctness
// of the space transformation in Enhanced iPRF
func TestBug2SpaceTransformation(t *testing.T) {
	prpKey := PrfKey128{0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27,
		0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f}
	baseKey := PrfKey128{0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37,
		0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f}

	n := uint64(500)
	m := uint64(50)

	prp := NewPRP(prpKey)
	baseIPRF := NewIPRF(baseKey, n, m)
	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	// Test a sample of inputs
	for x := uint64(0); x < n; x += 10 {
		// Manual composition
		permutedX := prp.Permute(x, n)       // P(x)
		y := baseIPRF.Forward(permutedX)     // S(P(x))

		// Enhanced iPRF should produce same result
		yEnhanced := eiprf.Forward(x)

		if y != yEnhanced {
			t.Errorf("Forward mismatch: x=%d, manual=%d, enhanced=%d", x, y, yEnhanced)
		}

		// Inverse decomposition
		permutedPreimages := baseIPRF.InverseFixed(y) // S⁻¹(y) in permuted space

		// Apply PRP inverse to get back to original space
		expectedOriginalPreimages := make(map[uint64]bool)
		for _, permutedPreimage := range permutedPreimages {
			originalPreimage := prp.InversePermute(permutedPreimage, n) // P⁻¹
			expectedOriginalPreimages[originalPreimage] = true
		}

		// Enhanced iPRF inverse should match manual decomposition
		actualPreimages := eiprf.Inverse(y)

		// Check all actual preimages are expected
		for _, actual := range actualPreimages {
			if !expectedOriginalPreimages[actual] {
				t.Errorf("Unexpected preimage: y=%d has preimage %d not in manual set", y, actual)
			}
		}

		// Check all expected preimages are present
		for expected := range expectedOriginalPreimages {
			found := false
			for _, actual := range actualPreimages {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Missing preimage: y=%d should have preimage %d", y, expected)
			}
		}
	}

	t.Log("✓ Space transformation mathematically correct")
}

// TestBug2RegressionCheck ensures existing passing tests still work
func TestBug2RegressionCheck(t *testing.T) {
	// This should already exist and pass
	t.Run("EnhancedIPRFInverseSpace", func(t *testing.T) {
		// Run the existing test that was blocked by Bug #1
		// Now that Bug #1 is fixed, this should pass if Bug #2 is not present

		prpKey := GenerateDeterministicKey()
		baseKey := GenerateDeterministicKey()

		testCases := []struct {
			name string
			n    uint64
			m    uint64
		}{
			{"Small domain", 100, 10},
			{"Medium domain", 1000, 100},
			{"Production scale", 10000, 1024},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				eiprf := NewEnhancedIPRF(prpKey, baseKey, tc.n, tc.m)

				// Sample test (not exhaustive for large domains)
				sampleSize := tc.n
				if sampleSize > 100 {
					sampleSize = 100
				}

				for x := uint64(0); x < sampleSize; x++ {
					y := eiprf.Forward(x)
					preimages := eiprf.Inverse(y)

					// All preimages in [0, n)
					for _, p := range preimages {
						if p >= tc.n {
							t.Fatalf("Preimage %d out of bounds [0, %d)", p, tc.n)
						}
					}

					// x is in preimage set
					found := false
					for _, p := range preimages {
						if p == x {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("x=%d not in Inverse(Forward(%d))", x, x)
					}
				}
			})
		}
	})
}
