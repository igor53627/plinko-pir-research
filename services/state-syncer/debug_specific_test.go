package main

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func TestDebugSpecificCase(t *testing.T) {
	// Same parameters as the failing test
	n := uint64(1000) // domain size
	m := uint64(100)  // range size
	
	// Create random keys (same as test)
	var prpKey, baseKey PrfKey128
	rand.Read(prpKey[:])
	rand.Read(baseKey[:])
	
	// Create enhanced iPRF
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Debug the specific failing case
	testIndex := uint64(92)
	y := iprf.Forward(testIndex)
	preimages := iprf.Inverse(y)
	
	fmt.Printf("Testing specific case: index %d\n", testIndex)
	fmt.Printf("Forward(%d) = %d\n", testIndex, y)
	fmt.Printf("Inverse(%d) found %d preimages\n", y, len(preimages))
	
	found := false
	for _, preimage := range preimages {
		if preimage == testIndex {
			found = true
			break
		}
	}
	
	if found {
		fmt.Printf("✅ SUCCESS: Inverse(%d) contains original index %d\n", y, testIndex)
	} else {
		fmt.Printf("❌ FAILURE: Inverse(%d) does NOT contain original index %d\n", y, testIndex)
		
		// Let's check a few preimages to see what's happening
		showLen := 10
		if len(preimages) < 10 {
			showLen = len(preimages)
		}
		fmt.Printf("First %d preimages: %v\n", showLen, preimages[:showLen])
		
		// Let's also check what the base iPRF does
		baseIprf := NewIPRF(baseKey, n, m)
		permutedX := baseIprf.Forward(testIndex)
		basePreimages := baseIprf.InverseFixed(y)
		
		fmt.Printf("Base iPRF Forward(%d) = %d\n", testIndex, permutedX)
		fmt.Printf("Base iPRF Inverse(%d) found %d preimages\n", y, len(basePreimages))
		
		// Check if the permuted index is in the base preimages
		permutedFound := false
		for _, basePreimage := range basePreimages {
			if basePreimage == permutedX {
				permutedFound = true
				break
			}
		}
		
		if permutedFound {
			fmt.Printf("✅ Base preimages contain permuted index %d\n", permutedX)
		} else {
			fmt.Printf("❌ Base preimages do NOT contain permuted index %d\n", permutedX)
		}
	}
}