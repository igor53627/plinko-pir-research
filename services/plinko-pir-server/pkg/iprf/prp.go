package iprf

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
)

// PRP implements a Small-Domain Pseudorandom Permutation
// using a generalized Feistel network with cycle walking.
type PRP struct {
	key    []byte
	block  cipher.Block
	n      uint64 // Domain size [0, n-1]
	rounds int
}

// NewPRP creates a new PRP with the given key and domain size n.
func NewPRP(key []byte, n uint64) *PRP {
	if len(key) != 16 {
		panic("PRP key must be 16 bytes (AES-128)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	return &PRP{
		key:    key,
		block:  block,
		n:      n,
		rounds: 4, // Sufficient for random permutation indistinguishability
	}
}

// Permute implements P(k, x) -> y
func (p *PRP) Permute(x uint64) uint64 {
	if x >= p.n {
		panic("input x out of bounds")
	}

	// Cycle walking: retry if result is out of bounds
	// We operate on a domain of size 2^k >= n
	// Find smallest k such that 2^k >= n
	// Actually, we can just use a Feistel on 2 halves.
	// Let's find bits needed.
	bits := uint(0)
	for (uint64(1) << bits) < p.n {
		bits++
	}

	// If n is small, cycle walking is fast.
	// If n is large, we need balanced Feistel.
	// We split bits into left and right halves.
	// L has ceil(bits/2), R has floor(bits/2)

	// Cycle walking loop
	curr := x
	for {
		curr = p.feistelEncrypt(curr, bits)
		if curr < p.n {
			return curr
		}
		// If out of bounds (>= n), permute again (cycle walking)
	}
}

// Inverse implements P^-1(k, y) -> x
func (p *PRP) Inverse(y uint64) uint64 {
	if y >= p.n {
		panic("input y out of bounds")
	}

	bits := uint(0)
	for (uint64(1) << bits) < p.n {
		bits++
	}

	curr := y
	for {
		curr = p.feistelDecrypt(curr, bits)
		if curr < p.n {
			return curr
		}
	}
}

// feistelEncrypt runs the Feistel network forward
func (p *PRP) feistelEncrypt(val uint64, bits uint) uint64 {
	// Split into L and R
	rightBits := bits / 2
	leftBits := bits - rightBits

	maskRight := (uint64(1) << rightBits) - 1
	maskLeft := (uint64(1) << leftBits) - 1

	right := val & maskRight
	left := (val >> rightBits) & maskLeft

	for i := 0; i < p.rounds; i++ {

		// If we swap, L becomes R.
		// If leftBits != rightBits, we can't just swap.
		// We need a structure that preserves domain.
		// Generalized Feistel for arbitrary domain usually alternates or uses equal splits.
		// Since we cycle walk on 2^bits, we can ensure we are close to balanced.
		// bits = ceil(log2(n)).
		// If bits is odd, leftBits = rightBits + 1.
		// We can use the "Luby-Rackoff" construction which works for equal bits.
		// For unequal, we can use Thorp shuffle or just simple Feistel if we pad?
		// Or just use modular addition.

		// Let's use the property that we can swap if we change the modulus.
		// But simpler: Just use cycle walking on a domain where we CAN split evenly?
		// No, 2^bits might have odd bits.

		// Let's use a simple Feistel where we alternate applying F to L and R?
		// Or just:
		// L = L ^ F(R) (if L fits mask)
		// Swap L, R (if sizes match)

		// Actually, for small domain PRP, we can use a simpler approach:
		// Format Preserving Encryption (FPE) - FF1 or FF3.
		// But that's complex to implement.

		// Let's use a simple "Feistel-like" structure that works for any bits.
		// We can treat the value as a number and use modular addition.
		// But we need invertibility.

		// Let's stick to standard balanced Feistel if bits is even.
		// If bits is odd, we have L (k+1 bits) and R (k bits).
		// Round:
		// T = F(R)
		// L = L ^ T (truncated to k+1 bits)
		// Swap L and R? No, sizes differ.

		// Correct Generalized Feistel for A (a bits) and B (b bits):
		// 1. Let T = F(B)
		// 2. A' = (A + T) mod 2^a
		// 3. Swap A' and B? No, A' becomes B (size b) and B becomes A (size a)? Only if a=b.

		// If a != b, we can't swap directly.
		// But we can just keep A and B in place and alternate?
		// Round 1: A = A + F(B)
		// Round 2: B = B + F(A)
		// ...
		// This works and is invertible!
		// Inverse:
		// ...
		// Round 2: B = B - F(A)
		// Round 1: A = A - F(B)

		// This is perfect.

		fValA := p.roundFunction(right, 2*i, leftBits)
		left = (left + fValA) & maskLeft

		fValB := p.roundFunction(left, 2*i+1, rightBits)
		right = (right + fValB) & maskRight
	}

	return (left << rightBits) | right
}

// feistelDecrypt runs the Feistel network backward
func (p *PRP) feistelDecrypt(val uint64, bits uint) uint64 {
	rightBits := bits / 2
	leftBits := bits - rightBits

	maskRight := (uint64(1) << rightBits) - 1
	maskLeft := (uint64(1) << leftBits) - 1

	right := val & maskRight
	left := (val >> rightBits) & maskLeft

	for i := p.rounds - 1; i >= 0; i-- {
		// Inverse of Round 2: B = B - F(A)
		fValB := p.roundFunction(left, 2*i+1, rightBits)
		// (right - fValB) mod 2^rightBits
		// Go handles unsigned underflow correctly (modulo 2^64), but we need mod 2^rightBits
		// (x - y) & mask is correct for 2^k modular subtraction
		right = (right - fValB) & maskRight

		// Inverse of Round 1: A = A - F(B)
		fValA := p.roundFunction(right, 2*i, leftBits)
		left = (left - fValA) & maskLeft
	}

	return (left << rightBits) | right
}

// roundFunction F(val, round) -> output
func (p *PRP) roundFunction(val uint64, round int, outBits uint) uint64 {
	var input [16]byte
	binary.BigEndian.PutUint64(input[0:], val)
	binary.BigEndian.PutUint64(input[8:], uint64(round))

	var output [16]byte
	p.block.Encrypt(output[:], input[:])

	res := binary.BigEndian.Uint64(output[:8])

	// Mask to output size
	mask := (uint64(1) << outBits) - 1
	return res & mask
}
