package main

import (
	"fmt"
	"testing"
)

func TestDebugEnhancedIPRF(t *testing.T) {
	// Small test case to debug the enhanced iPRF
	n := uint64(100)
	m := uint64(10)
	
	var prpKey, baseKey PrfKey128
	for i := 0; i < 16; i++ {
		prpKey[i] = byte(i)
		baseKey[i] = byte(i + 16)
	}
	
	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	baseIprf := NewIPRF(baseKey, n, m)
	
	// Test a specific case
	testIndex := uint64(5)
	
	// Test base iPRF
	baseForward := baseIprf.Forward(testIndex)
	baseInverse := baseIprf.InverseFixed(baseForward)
	fmt.Printf("Base iPRF: Forward(%d) = %d, Inverse(%d) = %d preimages\n", 
		testIndex, baseForward, baseForward, len(baseInverse))
	
	// Test enhanced iPRF
	enhancedForward := eiprf.Forward(testIndex)
	enhancedInverse := eiprf.Inverse(enhancedForward)
	fmt.Printf("Enhanced iPRF: Forward(%d) = %d, Inverse(%d) = %d preimages\n", 
		testIndex, enhancedForward, enhancedForward, len(enhancedInverse))
	
	// Check if original index is in inverse result
	found := false
	for _, idx := range enhancedInverse {
		if idx == testIndex {
			found = true
			break
		}
	}
	
	if found {
		fmt.Printf("✅ Original index %d found in enhanced inverse result\n", testIndex)
	} else {
		fmt.Printf("❌ Original index %d NOT found in enhanced inverse result\n", testIndex)
		fmt.Printf("   Expected to find %d in: %v\n", testIndex, enhancedInverse)
	}
	
	// Test that enhanced forward is different from base forward
	if baseForward != enhancedForward {
		fmt.Printf("✅ Enhanced iPRF produces different result from base (PRP layer working)\n")
	} else {
		fmt.Printf("⚠️  Enhanced iPRF produces same result as base\n")
	}
}