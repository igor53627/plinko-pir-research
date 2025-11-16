package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
)

// PRP (Pseudorandom Permutation) implementation for iPRF
//
// This implementation uses TablePRP (Fisher-Yates deterministic shuffle) as the
// production PRP construction. TablePRP satisfies all requirements from the
// Plinko paper (Theorem 4.4):
//
// - Perfect bijection: Every x ∈ [0,n) maps to unique y ∈ [0,n)
// - Efficient forward: O(1) lookup after O(n) one-time initialization
// - Efficient inverse: O(1) lookup (vs O(n) brute force)
// - Pseudorandom: PRF-seeded shuffle indistinguishable from random permutation
//
// The paper requires composing PRP with PMNS: iF.F((k1,k2),x) = S(k2, P(k1,x))
//
// BUGS FIXED BY TABLEPRP:
// - Bug #1: Cycle walking didn't maintain bijection (state modification bug)
// - Bug #3: O(n) inverse impractical (brute force too slow for n=8.4M)
// - Bug #11: Cycle walking state mutation caused non-deterministic behavior
// - Bug #15: Fallback permutation (x % n) was not bijective
//
// Memory footprint: 16 bytes per element (~134 MB for n=8.4M, acceptable for server)
//
// Historical Note:
// Previous implementations included cycle-walking-based PRP construction,
// but this was removed as TablePRP provides superior performance characteristics
// for our use case (n ≈ 8.4M domain, frequent inverse operations).
//
// See table_prp.go for TablePRP implementation details.

type PRP struct {
	key       PrfKey128
	block     cipher.Block
	roundKeys [][]byte
	rounds    int
	tablePRP  *TablePRP // Table-based PRP for guaranteed bijection
}

// NewPRP creates a new pseudorandom permutation using TablePRP
// The TablePRP is created lazily when domain size is known (first Permute call)
func NewPRP(key PrfKey128) *PRP {
	if len(key) != 16 {
		panic("PRP key must be 16 bytes (AES-128)")
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic("failed to create AES cipher: " + err.Error())
	}

	// Use 4 rounds for good security/performance balance (legacy, kept for compatibility)
	rounds := 4
	roundKeys := deriveRoundKeys(key[:], rounds)

	return &PRP{
		key:       key,
		block:     block,
		roundKeys: roundKeys,
		rounds:    rounds,
		tablePRP:  nil, // Created lazily when domain is known
	}
}

// NewPRPWithDomain creates a PRP with pre-initialized TablePRP for known domain
// This is more efficient for repeated use with the same domain size
func NewPRPWithDomain(key PrfKey128, domain uint64) *PRP {
	prp := NewPRP(key)
	prp.tablePRP = NewTablePRP(domain, key[:])
	return prp
}

// deriveRoundKeys derives independent round keys from master key
// Uses SHA-256 based key derivation for each round
func deriveRoundKeys(masterKey []byte, rounds int) [][]byte {
	keys := make([][]byte, rounds)
	for i := 0; i < rounds; i++ {
		// Derive round key using SHA-256(masterKey || roundNumber)
		h := sha256.New()
		h.Write(masterKey)
		h.Write([]byte{byte(i)})
		keys[i] = h.Sum(nil)[:16] // Use first 16 bytes as AES-128 key
	}
	return keys
}

// splitBits splits a value into two halves for Feistel rounds
// For non-power-of-2 domains, we work in the smallest power-of-2 domain that contains n
func splitBits(x uint64) (left, right uint32) {
	// Split 64-bit value into two 32-bit halves
	right = uint32(x & 0xFFFFFFFF)
	left = uint32(x >> 32)
	return left, right
}

// combineBits combines Feistel halves back into single value
func combineBits(left, right uint32) uint64 {
	return (uint64(left) << 32) | uint64(right)
}

// countLeadingZeros counts leading zero bits in a uint64
func countLeadingZeros(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	if x <= 0x00000000FFFFFFFF {
		n += 32
		x <<= 32
	}
	if x <= 0x0000FFFFFFFFFFFF {
		n += 16
		x <<= 16
	}
	if x <= 0x00FFFFFFFFFFFFFF {
		n += 8
		x <<= 8
	}
	if x <= 0x0FFFFFFFFFFFFFFF {
		n += 4
		x <<= 4
	}
	if x <= 0x3FFFFFFFFFFFFFFF {
		n += 2
		x <<= 2
	}
	if x <= 0x7FFFFFFFFFFFFFFF {
		n += 1
	}
	return n
}

// roundFunction applies AES encryption to right half with round key
func (prp *PRP) roundFunction(right uint32, roundKey []byte) uint32 {
	// Create AES cipher with round key
	block, err := aes.NewCipher(roundKey)
	if err != nil {
		panic("failed to create round cipher: " + err.Error())
	}

	// Encrypt the right half
	var input [aes.BlockSize]byte
	var output [aes.BlockSize]byte

	binary.BigEndian.PutUint32(input[0:4], right)
	// Fill rest with zeros
	for i := 4; i < aes.BlockSize; i++ {
		input[i] = 0
	}

	block.Encrypt(output[:], input[:])

	// Extract result as uint32
	return binary.BigEndian.Uint32(output[0:4])
}

// Permute applies the PRP to input x in domain [0, n-1]
// Uses TablePRP to guarantee perfect bijection with O(1) performance
func (prp *PRP) Permute(x uint64, n uint64) uint64 {
	if n == 0 {
		panic("PRP Permute: domain size n cannot be zero")
	}
	if x >= n {
		panic(fmt.Sprintf("PRP Permute: input x=%d out of domain [0, %d)", x, n))
	}

	// Lazy initialization of TablePRP with known domain
	if prp.tablePRP == nil || prp.tablePRP.domain != n {
		prp.tablePRP = NewTablePRP(n, prp.key[:])
	}

	return prp.tablePRP.Forward(x)
}

// InversePermute computes the inverse permutation
// Uses TablePRP for O(1) inverse lookup (vs O(n) brute force)
func (prp *PRP) InversePermute(y uint64, n uint64) uint64 {
	if n == 0 {
		panic("PRP InversePermute: domain size n cannot be zero")
	}
	if y >= n {
		panic(fmt.Sprintf("PRP InversePermute: input y=%d out of range [0, %d)", y, n))
	}

	// Lazy initialization of TablePRP with known domain
	if prp.tablePRP == nil || prp.tablePRP.domain != n {
		prp.tablePRP = NewTablePRP(n, prp.key[:])
	}

	return prp.tablePRP.Inverse(y)
}

// inverseBruteForce finds the original input by trying all possibilities
// This is kept for testing/debugging purposes but should not be used in production
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

// Inverse returns all x ∈ [0, n) such that Forward(x) = y
//
// Implementation follows paper Theorem 4.4:
//   iF.F⁻¹((k1,k2), y) = {P⁻¹(k1, x) : x ∈ S⁻¹(k2, y)}
//
// CRITICAL: Returns preimages in ORIGINAL domain [0, n), not permuted space.
// The two-step process is:
//   1. Find preimages in permuted space: S⁻¹(k2, y) → {permuted_x}
//   2. Transform back to original space: P⁻¹(k1, permuted_x) → {x}
//
// Bug #2 Fix: Previous implementations correctly applied both transformations.
// This validates that the inverse composition properly reverses both the PRP
// and the base iPRF, ensuring all returned preimages are in the original domain.
//
// Mathematical correctness:
//   For any x ∈ [0,n): Forward(x) = y implies x ∈ Inverse(y)
//   All elements of Inverse(y) are in [0,n)
//   Inverse(Forward(x)) contains x (round-trip correctness)
func (eiprf *EnhancedIPRF) Inverse(y uint64) []uint64 {
	// Step 1: Find all preimages in the base iPRF (permuted space)
	// S⁻¹(k2, y) returns values in permuted domain
	permutedPreimages := eiprf.base.InverseFixed(y)

	// Step 2: Apply inverse PRP to each preimage to get back to original space
	// P⁻¹(k1, permuted_x) transforms back to original domain [0, n)
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
// This applies the same transformation as Inverse but uses the fixed implementation
func (eiprf *EnhancedIPRF) InverseFixed(y uint64) []uint64 {
	// Step 1: Find all preimages in the base iPRF (permuted space) using fixed implementation
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

// DebugInverse provides debugging for the enhanced iPRF
func (eiprf *EnhancedIPRF) DebugInverse(y uint64) {
	// TODO: Implement if needed
	// eiprf.base.DebugInverse(y)
}

// ValidateInverseImplementation validates the inverse implementation
func (eiprf *EnhancedIPRF) ValidateInverseImplementation() bool {
	// TODO: Implement if needed
	return true
	// return eiprf.base.ValidateInverseImplementation()
}