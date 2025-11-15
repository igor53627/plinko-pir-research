package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
)

// PrfKey128 is a 16-byte PRF key
type PrfKey128 [16]byte

// PRSet represents a pseudorandom set for Plinko PIR
type PRSet struct {
	Key   PrfKey128
	block cipher.Block
}

// NewPRSet creates a new PRSet with the given key
func NewPRSet(key PrfKey128) *PRSet {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err)
	}
	return &PRSet{Key: key, block: block}
}

// Expand generates a pseudorandom set of database indices
// setSize: number of chunks (k in Plinko PIR)
// chunkSize: size of each chunk
// Returns: array of setSize indices, one per chunk
func (prs *PRSet) Expand(setSize uint64, chunkSize uint64) []uint64 {
	indices := make([]uint64, setSize)

	for i := uint64(0); i < setSize; i++ {
		offset := prs.prfEvalMod(i, chunkSize)
		indices[i] = i*chunkSize + offset
	}

	return indices
}

// prfEvalMod evaluates PRF(key, x) mod m using AES-128
func (prs *PRSet) prfEvalMod(x uint64, m uint64) uint64 {
	if m == 0 {
		return 0
	}

	var input [aes.BlockSize]byte
	binary.BigEndian.PutUint64(input[aes.BlockSize-8:], x)

	var output [aes.BlockSize]byte
	prs.block.Encrypt(output[:], input[:])

	value := binary.BigEndian.Uint64(output[:8])
	return value % m
}
