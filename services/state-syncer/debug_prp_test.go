package main

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func TestDebugPRPIssue(t *testing.T) {
	// Same parameters as the failing test
	n := uint64(1000) // domain size
	m := uint64(100)  // range size
	
	// Create random keys (same as test)
	var prpKey, baseKey PrfKey128
	rand.Read(prpKey[:])
	rand.Read(baseKey[:])
	
	// Test base iPRF (without PRP)
	baseIprf := NewIPRF(baseKey, n, m)
	
	fmt.Println("Testing BASE iPRF (without PRP layer):")
	for x := uint64(0); x < 10; x++ {
		y := baseIprf.Forward(x)
		preimages := baseIprf.InverseFixed(y)
		
		found := false
		for _, preimage := range preimages {
			if preimage == x {
				found = true
				break
			}
		}
		
		if found {
			fmt.Printf("✅ Base: Forward(%d) = %d, Inverse(%d) contains %d\n", x, y, y, x)
		} else {
			fmt.Printf("❌ Base: Forward(%d) = %d, Inverse(%d) does NOT contain %d\n", x, y, y, x)
		}
	}
	
	fmt.Println("\nTesting ENHANCED iPRF (with PRP layer):")
	// Test enhanced iPRF (with PRP)
	enhancedIprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	for x := uint64(0); x < 10; x++ {
		y := enhancedIprf.Forward(x)
		preimages := enhancedIprf.Inverse(y)
		
		found := false
		for _, preimage := range preimages {
			if preimage == x {
				found = true
				break
			}
		}
		
		if found {
			fmt.Printf("✅ Enhanced: Forward(%d) = %d, Inverse(%d) contains %d\n", x, y, y, x)
		} else {
			fmt.Printf("❌ Enhanced: Forward(%d) = %d, Inverse(%d) does NOT contain %d\n", x, y, y, x)
		}
	}
	
	// Test PRP directly
	fmt.Println("\nTesting PRP layer directly:")
	prp := NewPRP(prpKey)
	
	for x := uint64(0); x < 10; x++ {
		permuted := prp.Permute(x, n)
		inverse := prp.InversePermute(permuted, n)
		
		if inverse == x {
			fmt.Printf("✅ PRP: Permute(%d) = %d, InversePermute(%d) = %d\n", x, permuted, permuted, inverse)
		} else {
			fmt.Printf("❌ PRP: Permute(%d) = %d, InversePermute(%d) = %d (expected %d)\n", 
				x, permuted, permuted, inverse, x)
		}
	}
}