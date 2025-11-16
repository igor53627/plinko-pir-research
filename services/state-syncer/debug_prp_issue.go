package main

import (
	"testing"
)

func TestDebugPRPIssueSpecific(t *testing.T) {
	// Test the specific case that was problematic
	n := uint64(1000)
	
	// Create deterministic keys for reproducible testing
	var prpKey, baseKey PrfKey128
	for i := 0; i < 16; i++ {
		prpKey[i] = byte(i * 17 + 1)
		baseKey[i] = byte(i * 23 + 7)
	}
	
	// Create PRP
	prp := NewPRP(prpKey)
	
	// Test specific inputs that were problematic
	testCases := []uint64{47, 74, 95, 97}
	
	for _, x := range testCases {
		// Test forward permutation
		y := prp.Permute(x, n)
		
		// Test inverse permutation
		xInverse := prp.InversePermute(y, n)
		
		// Verify PRP property: InversePermute(Permute(x)) should equal x
		if xInverse == x {
			t.Logf("PRP: Permute(%d) = %d, InversePermute(%d) = %d ✅", x, y, y, xInverse)
		} else {
			t.Logf("PRP: Permute(%d) = %d, InversePermute(%d) = %d ❌ (expected %d)", x, y, y, xInverse, x)
		}
	}
}