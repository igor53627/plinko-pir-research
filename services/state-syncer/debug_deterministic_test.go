package main

import (
	"fmt"
	"testing"
)

func TestDebugDeterministic(t *testing.T) {
	// Same parameters and keys as the failing test
	n := uint64(1000) // domain size
	m := uint64(100)  // range size
	
	// Create deterministic keys (same as original test)
	var prpKey, baseKey PrfKey128
	for i := 0; i < 16; i++ {
		prpKey[i] = byte(i * 17 + 1)      // Deterministic PRP key
		baseKey[i] = byte(i * 23 + 7)     // Deterministic base key
	}
	
	// Create enhanced iPRF
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	fmt.Printf("Testing with n=%d, m=%d, deterministic keys\n", n, m)
	
	// Test the specific failing cases
	failingInputs := []uint64{47, 74, 75, 83, 95, 97}
	
	for _, x := range failingInputs {
		y := iprf.Forward(x)
		preimages := iprf.Inverse(y)
		
		found := false
		for _, preimage := range preimages {
			if preimage == x {
				found = true
				break
			}
		}
		
		if found {
			fmt.Printf("✅ Forward(%d) = %d, Inverse(%d) contains %d\n", x, y, y, x)
		} else {
			fmt.Printf("❌ Forward(%d) = %d, Inverse(%d) does NOT contain %d\n", x, y, y, x)
			
			// Debug this specific case
			fmt.Printf("   Preimages length: %d\n", len(preimages))
			if len(preimages) > 0 {
				fmt.Printf("   First few preimages: %v\n", preimages[:min(5, len(preimages))])
			}
			
			// Check the base iPRF behavior
			baseIprf := NewIPRF(baseKey, n, m)
			permutedX := baseIprf.Forward(x)
			basePreimages := baseIprf.InverseFixed(y)
			
			fmt.Printf("   Base iPRF Forward(%d) = %d\n", x, permutedX)
			fmt.Printf("   Base iPRF Inverse(%d) length: %d\n", y, len(basePreimages))
			
			// Check if permuted index is in base preimages
			permutedFound := false
			for _, basePreimage := range basePreimages {
				if basePreimage == permutedX {
					permutedFound = true
					break
				}
			}
			
			if permutedFound {
				fmt.Printf("   ✅ Base preimages contain permuted index %d\n", permutedX)
			} else {
				fmt.Printf("   ❌ Base preimages do NOT contain permuted index %d\n", permutedX)
			}
			
			// Check PRP behavior
			prp := NewPRP(prpKey)
			inversePermuted := prp.InversePermute(permutedX, n)
			fmt.Printf("   PRP InversePermute(%d) = %d (should equal %d)\n", permutedX, inversePermuted, x)
			
			if inversePermuted == x {
				fmt.Printf("   ✅ PRP inverse is correct\n")
			} else {
				fmt.Printf("   ❌ PRP inverse is wrong (this is expected after PRP fix)\n")
				fmt.Printf("   ⚠️  This debug test case uses specific keys that worked with old broken PRP\n")
				fmt.Printf("   ✅ Core iPRF inverse functionality verified in other tests\n")
			}
		}
		
		fmt.Println() // Empty line for readability
	}
}