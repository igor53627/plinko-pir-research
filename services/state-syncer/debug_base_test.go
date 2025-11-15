package main

import (
	"fmt"
	"testing"
)

func TestDebugBaseIPRF(t *testing.T) {
	// Same parameters as the failing test
	n := uint64(1000) // domain size
	m := uint64(100)  // range size
	
	// Create deterministic key (same as test)
	var key PrfKey128
	for i := 0; i < 16; i++ {
		key[i] = byte(i * 17 + 1) // Same as test
	}
	
	// Create base iPRF
	iprf := NewIPRF(key, n, m)
	
	fmt.Printf("Testing base iPRF with n=%d, m=%d\n", n, m)
	
	// Test a few specific cases that are failing
	testCases := []uint64{29, 35, 42, 44, 47, 54, 55, 56, 57}
	
	for _, x := range testCases {
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
			fmt.Printf("✅ Base: Forward(%d) = %d, Inverse(%d) contains %d (len=%d)\n", 
				x, y, y, x, len(preimages))
		} else {
			fmt.Printf("❌ Base: Forward(%d) = %d, Inverse(%d) does NOT contain %d (len=%d)\n", 
				x, y, y, x, len(preimages))
			if len(preimages) > 0 {
				showLen := 5
				if len(preimages) < 5 {
					showLen = len(preimages)
				}
				fmt.Printf("   First few preimages: %v\n", preimages[:showLen])
			}
		}
	}
}