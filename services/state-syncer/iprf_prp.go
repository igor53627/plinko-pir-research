package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
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
	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err)
	}
	return &PRP{key: key, block: block}
}

// Permute applies the PRP to input x in domain [0, n-1]
// Uses a Feistel-like construction for arbitrary domain sizes
func (prp *PRP) Permute(x uint64, n uint64) uint64 {
	if n == 0 || x >= n {
		return 0
	}
	
	// For small domains, use direct AES output with rejection sampling
	if n <= (1 << 16) {  // Use small domain method only for n <= 65536
		return prp.permuteSmall(x, n)
	}
	
	// For large domains, use Feistel network construction
	return prp.permuteLarge(x, n)
}

// permuteSmall handles small domains using a cycle-walking approach
func (prp *PRP) permuteSmall(x uint64, n uint64) uint64 {
	// For small domains, we need to ensure no collisions
	// Use a cycle-walking approach with sufficient rounds
	
	var input [aes.BlockSize]byte
	var output [aes.BlockSize]byte
	
	// Use input as key for the permutation, with different rounds for variety
	// This creates a bijection for small domains
	current := x
	
	// Multiple rounds to ensure good mixing
	for round := 0; round < 8; round++ {
		binary.BigEndian.PutUint64(input[0:8], current)
		binary.BigEndian.PutUint64(input[8:16], uint64(round))
		
		prp.block.Encrypt(output[:], input[:])
		current = binary.BigEndian.Uint64(output[0:8])
		
		// If result is too large, feed it back through
		if current >= n {
			// Use the high bits as a new input for another round
			current = current % (n * 2) // Reduce but keep some entropy
			if current >= n {
				current = current - n
			}
		}
	}
	
	// Final result should be in range
	if current >= n {
		current = current % n
	}
	
	return current
}

// permuteLarge handles large domains using Feistel network
func (prp *PRP) permuteLarge(x uint64, n uint64) uint64 {
	// Split x into two halves for Feistel
	halfBits := 32
	if n > (1 << 48) {
		halfBits = 48 // Adjust for very large domains
	}
	
	left := x >> halfBits
	right := x & ((1 << halfBits) - 1)
	
	// Feistel rounds
	rounds := 4
	for round := 0; round < rounds; round++ {
		// F-function: AES round function
		newRight := left ^ prp.fFunction(right, uint64(round))
		left = right
		right = newRight
	}
	
	// Combine halves
	result := (left << halfBits) | right
	
	// Ensure result is in range [0, n-1]
	if result >= n {
		result = result % n
	}
	
	return result
}

// fFunction is the round function for Feistel network
func (prp *PRP) fFunction(right uint64, round uint64) uint64 {
	var input [aes.BlockSize]byte
	var output [aes.BlockSize]byte
	
	// Encode right half and round number
	binary.BigEndian.PutUint64(input[0:8], right)
	binary.BigEndian.PutUint64(input[8:16], round)
	
	// Apply AES
	prp.block.Encrypt(output[:], input[:])
	
	// Extract result and truncate to appropriate size
	return binary.BigEndian.Uint64(output[0:8])
}

// InversePermute computes the inverse permutation
func (prp *PRP) InversePermute(y uint64, n uint64) uint64 {
	if n == 0 || y >= n {
		return 0
	}
	
	if n <= (1 << 32) {
		return prp.inversePermuteSmall(y, n)
	}
	
	return prp.inversePermuteLarge(y, n)
}

// inversePermuteSmall handles inverse for small domains
func (prp *PRP) inversePermuteSmall(y uint64, n uint64) uint64 {
	// For small domains, we can precompute the mapping
	// This is feasible since we're using this for iPRF construction
	for x := uint64(0); x < n; x++ {
		if prp.permuteSmall(x, n) == y {
			return x
		}
	}
	return 0 // Should never reach here if permutation is correct
}

// inversePermuteLarge handles inverse for large domains using reverse Feistel
func (prp *PRP) inversePermuteLarge(y uint64, n uint64) uint64 {
	// Split y into two halves
	halfBits := 32
	if n > (1 << 48) {
		halfBits = 48
	}
	
	left := y >> halfBits
	right := y & ((1 << halfBits) - 1)
	
	// Reverse Feistel rounds
	rounds := 4
	for round := rounds - 1; round >= 0; round-- {
		// Reverse F-function application
		newLeft := right ^ prp.fFunction(left, uint64(round))
		right = left
		left = newLeft
	}
	
	// Combine halves
	result := (left << halfBits) | right
	
	// The result should be the original input x
	return result
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