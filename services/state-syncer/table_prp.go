package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

// TablePRP implements a PRP using pre-computed permutation tables
// Guarantees O(1) forward and inverse operations with O(n) space
//
// This fixes Bug 1 (PRP bijection failure) and Bug 3 (O(n) inverse impractical)
// by using a deterministic Fisher-Yates shuffle to create a perfect bijection
// with pre-computed forward and inverse lookup tables.
//
// Memory footprint: 16 bytes per element (2 * 8 bytes for uint64 tables)
// For n=8,400,000: ~134 MB total (67 MB forward + 67 MB inverse)
type TablePRP struct {
	domain       uint64   // n: domain size
	forwardTable []uint64 // n entries: i → forwardTable[i]
	inverseTable []uint64 // n entries: j → inverseTable[j] where forwardTable[inverseTable[j]] = j
	key          []byte   // For deterministic generation
}

// NewTablePRP creates a deterministic PRP using Fisher-Yates shuffle
// The permutation is deterministic (same key + domain → same permutation)
// and guaranteed to be a perfect bijection (no collisions, every value reachable)
//
// Complexity:
//   - Initialization: O(n) time, O(n) space
//   - Forward lookup: O(1)
//   - Inverse lookup: O(1)
//
// This is a one-time initialization cost that enables fast lookups.
func NewTablePRP(domain uint64, key []byte) *TablePRP {
	if domain == 0 {
		panic("TablePRP: domain size cannot be zero")
	}

	if len(key) == 0 {
		panic("TablePRP: key cannot be empty")
	}

	// Initialize forward table as identity permutation [0, 1, 2, ..., domain-1]
	forwardTable := make([]uint64, domain)
	for i := uint64(0); i < domain; i++ {
		forwardTable[i] = i
	}

	// Apply Fisher-Yates shuffle with deterministic PRF-based RNG
	// This guarantees uniform random permutation with perfect bijection
	rng := NewDeterministicRNG(key)
	for i := domain - 1; i > 0; i-- {
		// Generate random index j in [0, i]
		j := rng.Uint64N(i + 1)

		// Swap forwardTable[i] with forwardTable[j]
		forwardTable[i], forwardTable[j] = forwardTable[j], forwardTable[i]
	}

	// Build inverse table from forward table
	// Property: inverseTable[forwardTable[i]] = i for all i
	inverseTable := make([]uint64, domain)
	for i := uint64(0); i < domain; i++ {
		y := forwardTable[i]
		inverseTable[y] = i
	}

	return &TablePRP{
		domain:       domain,
		forwardTable: forwardTable,
		inverseTable: inverseTable,
		key:          key,
	}
}

// Forward maps x ∈ [0, domain) to a permuted value
// Complexity: O(1) - single array lookup
func (t *TablePRP) Forward(x uint64) uint64 {
	if x >= t.domain {
		panic(fmt.Sprintf("TablePRP Forward: input x=%d out of domain [0, %d)", x, t.domain))
	}
	return t.forwardTable[x]
}

// Inverse computes the inverse mapping in O(1) time
// Property: Inverse(Forward(x)) = x for all x ∈ [0, domain)
// Complexity: O(1) - single array lookup (vs O(n) brute force)
func (t *TablePRP) Inverse(y uint64) uint64 {
	if y >= t.domain {
		panic(fmt.Sprintf("TablePRP Inverse: input y=%d out of domain [0, %d)", y, t.domain))
	}
	return t.inverseTable[y]
}

// DeterministicRNG provides a cryptographically strong deterministic RNG
// using AES in counter mode for Fisher-Yates shuffle
type DeterministicRNG struct {
	cipher  cipher.Block
	counter uint64
	buffer  [16]byte
}

// NewDeterministicRNG creates a deterministic RNG from a key
// Uses AES-128 in counter mode for cryptographic quality randomness
func NewDeterministicRNG(key []byte) *DeterministicRNG {
	// Ensure key is 16 bytes (AES-128)
	var aesKey [16]byte
	if len(key) >= 16 {
		copy(aesKey[:], key[:16])
	} else {
		copy(aesKey[:], key)
	}

	block, err := aes.NewCipher(aesKey[:])
	if err != nil {
		panic("DeterministicRNG: failed to create AES cipher: " + err.Error())
	}

	return &DeterministicRNG{
		cipher:  block,
		counter: 0,
	}
}

// Uint64 generates the next deterministic random uint64
// Uses AES-CTR mode: encrypt incrementing counter to get random bytes
func (rng *DeterministicRNG) Uint64() uint64 {
	// Encrypt counter to get random bytes
	var input [16]byte
	var output [16]byte

	binary.BigEndian.PutUint64(input[0:8], rng.counter)
	binary.BigEndian.PutUint64(input[8:16], rng.counter>>32) // Add extra entropy

	rng.cipher.Encrypt(output[:], input[:])
	rng.counter++

	// Extract 64-bit random value
	return binary.BigEndian.Uint64(output[0:8])
}

// Uint64N generates a uniform random uint64 in [0, n)
// Uses rejection sampling to avoid modulo bias
//
// This is critical for Fisher-Yates shuffle correctness:
// naive modulo would introduce bias for non-power-of-2 domains
func (rng *DeterministicRNG) Uint64N(n uint64) uint64 {
	if n == 0 {
		return 0
	}

	if n == 1 {
		return 0
	}

	// For power-of-2, simple mask works without bias
	if n&(n-1) == 0 {
		return rng.Uint64() & (n - 1)
	}

	// For non-power-of-2, use rejection sampling
	// Find the largest multiple of n that fits in uint64
	max := ^uint64(0) // Maximum uint64 value
	threshold := max - (max % n)

	for {
		r := rng.Uint64()
		if r < threshold {
			return r % n
		}
		// Reject and try again to avoid bias
	}
}
