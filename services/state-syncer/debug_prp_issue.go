package main

import (
	"fmt"
	"testing"
)

func TestDebugPRPIssueSpecific(t *testing.T) {
	// Test the specific case that's failing
	n := uint64(1000)
	
	// Create the same deterministic keys as the failing test
	var prpKey, baseKey PrfKey128
	for i := 0; i < 16; i++ {
		prpKey[i] = byte(i * 17 + 1)
		baseKey[i] = byte(i * 23 + 7)
	}
	
	// Create PRP
	prp := NewPRP(prpKey)
	
	// Test specific inputs that are failing
	failingInputs := []uint64{47, 74, 95, 97}
	
	for _, x := range failingInputs {
		// Test forward permutation
		y := prp.Permute(x, n)
		
		// Test inverse permutation
		xInverse := prp.InversePermute(y, n)
		
		fmt.Printf("PRP: Permute(%d) = %d, InversePermute(%d) = %d", x, y, y, xInverse)
		
		if xInverse == x {
			fmt.Printf(" ✅ CORRECT\n")
		} else {
			fmt.Printf(" ❌ WRONG (expected %d)\n", x)
		}
	}
	
	// Also test some that work
	workingInputs := []uint64{75, 83}
	for _, x := range workingInputs {
		y := prp.Permute(x, n)
		xInverse := prp.InversePermute(y, n)
		
		fmt.Printf("PRP: Permute(%d) = %d, InversePermute(%d) = %d", x, y, y, xInverse)
		
		if xInverse == x {
			fmt.Printf(" ✅ CORRECT\n")
		} else {
			fmt.Printf(" ❌ WRONG (expected %d)\n", x)
		}
	}
}