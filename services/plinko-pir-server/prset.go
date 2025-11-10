package main

import (
	"encoding/binary"
)

// PrfKey128 is a 16-byte PRF key
type PrfKey128 [16]byte

// PRSet represents a pseudorandom set for Plinko PIR
type PRSet struct {
	Key PrfKey128
}

// NewPRSet creates a new PRSet with the given key
func NewPRSet(key PrfKey128) *PRSet {
	return &PRSet{Key: key}
}

// Expand generates a pseudorandom set of database indices
// setSize: number of chunks (k in Plinko PIR)
// chunkSize: size of each chunk
// Returns: array of setSize indices, one per chunk
func (prs *PRSet) Expand(setSize uint64, chunkSize uint64) []uint64 {
	indices := make([]uint64, setSize)

	for i := uint64(0); i < setSize; i++ {
		// Generate pseudorandom offset within chunk i
		// offset ∈ [0, chunkSize)
		offset := prs.prfEvalMod(i, chunkSize)

		// Database index = chunk_start + offset
		indices[i] = i*chunkSize + offset
	}

	return indices
}

// prfEvalMod evaluates PRF(key, x) mod m
// Uses simple FNV-1a hash for PoC
func (prs *PRSet) prfEvalMod(x uint64, m uint64) uint64 {
	if m == 0 {
		return 0
	}

	// FNV-1a hash
	hash := uint64(2166136261)

	// Mix in key
	for i := 0; i < 16; i++ {
		hash ^= uint64(prs.Key[i])
		hash *= 16777619
	}

	// Mix in x
	xBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(xBytes, x)
	for _, b := range xBytes {
		hash ^= uint64(b)
		hash *= 16777619
	}

	// Return hash mod m
	return hash % m
}
