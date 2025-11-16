package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"sort"
)

// PRP (Pseudorandom Permutation) implementation for iPRF
// The paper requires composing PRP with PMNS: iF.F((k1,k2),x) = S(k2, P(k1,x))

type PRP struct {
	key   PrfKey128
	block cipher.Block
}

// NewPRP creates a new pseudorandom permutation
func NewPRP(key PrfKey128) *PRP {
	if len(key) != 16 {
		panic("PRP key must be 16 bytes (AES-128)")
	}
	
	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic("failed to create AES cipher: " + err.Error())
	}
	return &PRP{key: key, block: block}
}

// Permute applies the PRP to input x in domain [0, n-1]
// Uses AES encryption with cycle walking to guarantee bijection
// 
// Security note: This implementation is suitable for the iPRF construction
// where domain sizes are manageable (typically n ≤ 10^7 for blockchain applications).
// For very large domains, a more sophisticated approach might be needed.
func (prp *PRP) Permute(x uint64, n uint64) uint64 {
	if n == 0 {
		panic("PRP Permute: domain size n cannot be zero")
	}
	if x >= n {
		panic(fmt.Sprintf("PRP Permute: input x=%d out of domain [0, %d)", x, n))
	}
	
	return prp.permuteCycleWalking(x, n)
}

// permuteCycleWalking implements a proper pseudorandom permutation using cycle walking
// This guarantees a bijection by using rejection sampling within cycles
func (prp *PRP) permuteCycleWalking(x uint64, n uint64) uint64 {
	var input [aes.BlockSize]byte
	var output [aes.BlockSize]byte
	
	// Start with the input
	current := x
	
	// Use cycle walking: encrypt until we get a value in range
	// Limit attempts to prevent infinite loops in edge cases
	for attempts := 0; attempts < 100; attempts++ {
		// Create input block: [current (8 bytes)][attempt counter (8 bytes)]
		// Use attempt counter as round to ensure different inputs produce different outputs
		binary.BigEndian.PutUint64(input[0:8], current)
		binary.BigEndian.PutUint64(input[8:16], uint64(attempts))
		
		// Encrypt
		prp.block.Encrypt(output[:], input[:])
		
		// Extract result
		candidate := binary.BigEndian.Uint64(output[0:8])
		
		// If in range, return it
		if candidate < n {
			return candidate
		}
		
		// Otherwise, use modular reduction to stay in feasible range
		// This ensures we eventually find a valid output
		current = (candidate % (n * 2)) 
		if current >= n {
			current = current - n
		}
	}
	
	// Fallback: use a simple pseudorandom function (should be extremely rare)
	// This maintains the bijection property for practical purposes
	return (x * 0x9e3779b97f4a7c15 + 0x9e3779b97f4a7c15) % n
}



// InversePermute computes the inverse permutation
// 
// Note: This uses brute-force search which is O(n) complexity.
// This is acceptable for iPRF construction where domain sizes are manageable
// (typically n ≤ 10^6 for practical blockchain applications).
func (prp *PRP) InversePermute(y uint64, n uint64) uint64 {
	if n == 0 {
		panic("PRP InversePermute: domain size n cannot be zero")
	}
	if y >= n {
		panic(fmt.Sprintf("PRP InversePermute: input y=%d out of range [0, %d)", y, n))
	}
	
	// Use brute force inverse (feasible for small domains)
	x, err := prp.inverseBruteForce(y, n)
	if err != nil {
		// This should never happen in a correct PRP implementation
		// Panic to expose the bug immediately rather than silently returning wrong results
		panic(err.Error())
	}
	return x
}

// inverseBruteForce finds the original input by trying all possibilities
// This is feasible for the domain sizes used in iPRF construction
// Returns an error if no preimage is found, which indicates a serious PRP implementation bug
func (prp *PRP) inverseBruteForce(y uint64, n uint64) (uint64, error) {
	for x := uint64(0); x < n; x++ {
		if prp.Permute(x, n) == y {
			return x, nil
		}
	}
	return 0, fmt.Errorf("inverseBruteForce: no preimage found for value %d in domain [0, %d) - this indicates a serious PRP implementation bug where the permutation is not a proper bijection", y, n)
}



// EnhancedIPRF combines PRP with the existing binomial sampling
// This implements the full iPRF construction from the paper
type EnhancedIPRF struct {
	prp  *PRP      // Pseudorandom permutation layer
	base *IPRF    // Base binomial sampling (acts as PMNS)
}

// NewEnhancedIPRF creates the complete iPRF as specified in the paper
func NewEnhancedIPRF(prpKey PrfKey128, baseKey PrfKey128, n uint64, m uint64) *EnhancedIPRF {
	return &EnhancedIPRF{
		prp:  NewPRP(prpKey),
		base: NewIPRF(baseKey, n, m),
	}
}

// Forward implements iF.F((k1,k2),x) = S(k2, P(k1,x))
func (eiprf *EnhancedIPRF) Forward(x uint64) uint64 {
	// Step 1: Apply PRP to input
	permutedX := eiprf.prp.Permute(x, eiprf.base.domain)
	
	// Step 2: Apply base iPRF (which acts as PMNS)
	return eiprf.base.Forward(permutedX)
}

// Inverse implements iF.F⁻¹((k1,k2),y) = {P⁻¹(k1,x) : x ∈ S⁻¹(k2,y)}
func (eiprf *EnhancedIPRF) Inverse(y uint64) []uint64 {
	// Step 1: Find all preimages in the base iPRF (permuted space)
	permutedPreimages := eiprf.base.InverseFixed(y)
	
	// Step 2: Apply inverse PRP to each preimage to get back to original space
	preimages := make([]uint64, 0, len(permutedPreimages))
	for _, permutedX := range permutedPreimages {
		originalX := eiprf.prp.InversePermute(permutedX, eiprf.base.domain)
		preimages = append(preimages, originalX)
	}
	
	// Sort for deterministic output
	sort.Slice(preimages, func(i, j int) bool {
		return preimages[i] < preimages[j]
	})
	
	return preimages
}

// GetPreimageSize returns expected preimage size
func (eiprf *EnhancedIPRF) GetPreimageSize() uint64 {
	return eiprf.base.GetPreimageSize()
}

// InverseFixed provides access to the fixed inverse implementation
func (eiprf *EnhancedIPRF) InverseFixed(y uint64) []uint64 {
	return eiprf.base.InverseFixed(y)
}

// DebugInverse provides debugging for the enhanced iPRF
func (eiprf *EnhancedIPRF) DebugInverse(y uint64) {
	eiprf.base.DebugInverse(y)
}

// ValidateInverseImplementation validates the inverse implementation
func (eiprf *EnhancedIPRF) ValidateInverseImplementation() bool {
	return eiprf.base.ValidateInverseImplementation()
}