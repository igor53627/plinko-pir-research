package main

import (
	"encoding/hex"
	"testing"
)

func hexToKey(t *testing.T, hexStr string) PrfKey128 {
	t.Helper()
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("failed to decode hex key: %v", err)
	}
	if len(bytes) != 16 {
		t.Fatalf("expected 16-byte key, got %d bytes", len(bytes))
	}
	var key PrfKey128
	copy(key[:], bytes)
	return key
}

func TestPRSetExpandDeterminism(t *testing.T) {
	keyHex := "000102030405060708090a0b0c0d0e0f"
	key := hexToKey(t, keyHex)

	const setSize = 16
	const chunkSize = 8192

	prSet1 := NewPRSet(key, setSize, chunkSize)
	indices1 := prSet1.Expand()

	prSet2 := NewPRSet(key, setSize, chunkSize)
	indices2 := prSet2.Expand()

	if len(indices1) != setSize {
		t.Fatalf("expected %d indices, got %d", setSize, len(indices1))
	}

	for i := 0; i < len(indices1); i++ {
		if indices1[i] != indices2[i] {
			t.Errorf("mismatch at index %d: %d != %d", i, indices1[i], indices2[i])
		}
		
		// Check bounds
		start := uint64(i) * chunkSize
		end := start + chunkSize
		if indices1[i] < start || indices1[i] >= end {
			t.Errorf("index %d out of bounds: got %d, want [%d, %d)", i, indices1[i], start, end)
		}
	}
}

func TestIPRFInvertibility(t *testing.T) {
	// This tests the core Plinko property: IPRF is efficiently invertible.
	keyHex := "000102030405060708090a0b0c0d0e0f"
	key := hexToKey(t, keyHex)
	
	const n = 1024 // Domain size
	const m = 256  // Range size
	
	var k32 [32]byte
	copy(k32[:16], key[:])
	copy(k32[16:], key[:])
	
	iprf := NewIPRF(k32, n, m)
	
	// Check for a subset of inputs
	for x := uint64(0); x < n; x++ {
		y := iprf.Forward(x)
		
		// Invert y
		preimages := iprf.Inverse(y)
		
		found := false
		for _, val := range preimages {
			if val == x {
				found = true
				break
			}
		}
		
		if !found {
			t.Errorf("inverse of %d -> %d did not contain %d. Preimages: %v", x, y, x, preimages)
		}
	}
}