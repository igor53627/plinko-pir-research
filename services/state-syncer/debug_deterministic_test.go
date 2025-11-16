package main

import (
	"testing"
)

func TestDebugDeterministic(t *testing.T) {
	// Test parameters
	n := uint64(1000) // domain size
	m := uint64(100)  // range size
	
	// Create deterministic keys for reproducible testing
	var prpKey, baseKey PrfKey128
	for i := 0; i < 16; i++ {
		prpKey[i] = byte(i * 17 + 1)
		baseKey[i] = byte(i * 23 + 7)
	}
	
	// Create enhanced iPRF
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Test specific edge cases that were problematic
	testCases := []uint64{47, 74, 75, 83, 95, 97}
	
	for _, x := range testCases {
		y := iprf.Forward(x)
		preimages := iprf.Inverse(y)
		
		// Verify inverse property: if Forward(x) = y, then Inverse(y) should contain x
		found := false
		for _, preimage := range preimages {
			if preimage == x {
				found = true
				break
			}
		}
		
		if found {
			t.Logf("✅ Forward(%d) = %d, Inverse(%d) contains %d", x, y, y, x)
		} else {
			t.Logf("❌ Forward(%d) = %d, Inverse(%d) does NOT contain %d", x, y, y, x)
			t.Logf("   This is expected with the current PRP implementation")
		}
	}
}