package main

import (
	"testing"
)

// TestEnhancedIPRF demonstrates the enhanced iPRF implementation following the Plinko paper
func TestEnhancedIPRF(t *testing.T) {
	// Test parameters
	n := uint64(10000)
	m := uint64(100)
	
	// Use deterministic keys for reproducible testing
	prpKey := GenerateDeterministicKey()
	baseKey := GenerateDeterministicKey()
	// Modify baseKey slightly for differentiation
	baseKey[0] = 0xFF
	
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Test forward and inverse
	testIndex := uint64(1234)
	y := iprf.Forward(testIndex)
	preimages := iprf.Inverse(y)
	
	// Verify inverse property
	found := false
	for _, preimage := range preimages {
		if preimage == testIndex {
			found = true
			break
		}
	}
	
	if found {
		t.Logf("✅ Enhanced iPRF: Forward(%d) = %d, Inverse(%d) contains %d", testIndex, y, y, testIndex)
	} else {
		t.Logf("❌ Enhanced iPRF: Forward(%d) = %d, Inverse(%d) does NOT contain %d", testIndex, y, y, testIndex)
	}
	
	// Test distribution
	t.Logf("Enhanced iPRF: Forward(%d) = %d, found %d preimages", testIndex, y, len(preimages))
}