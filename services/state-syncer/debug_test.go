package main

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func TestDebugInverse(t *testing.T) {
	// Small test case to debug the inverse function
	n := uint64(100)
	m := uint64(10)
	
	prpKey := GenerateDeterministicKey()
	baseKey := GenerateDeterministicKey()
	// Modify keys to match original test pattern
	for i := 0; i < 16; i++ {
		prpKey[i] = byte(i)
		baseKey[i] = byte(i + 16)
	}
	
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Test a specific case
	testIndex := uint64(5)
	forwardResult := iprf.Forward(testIndex)
	
	fmt.Printf("Forward(%d) = %d\n", testIndex, forwardResult)
	
	// Debug the inverse
	iprf.DebugInverse(forwardResult)
	
	// Validate with simple method
	iprf.ValidateInverseImplementation()
}

func TestDebugWithTestParams(t *testing.T) {
	// Same parameters as the failing test
	n := uint64(1000) // domain size
	m := uint64(100)  // range size
	
	// Create random keys (same as test)
	var prpKey, baseKey PrfKey128
	rand.Read(prpKey[:])
	rand.Read(baseKey[:])
	
	// Create enhanced iPRF
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	fmt.Printf("Testing with n=%d, m=%d, random keys\n", n, m)
	
	// Test the first few inputs that are failing
	failingInputs := []uint64{0, 1, 2, 4, 6, 10, 11, 12, 13}
	
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
			fmt.Printf("   Preimages: %v (len=%d)\n", preimages[:10], len(preimages))
		}
	}
}