package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"math"
)

// IPRF implements the Invertible Pseudorandom Function described in the Plinko paper.
type IPRF struct {
	prp  *FeistelPRP
	pmns *PMNS
}

// NewIPRF creates a new IPRF with the given key, domain size n, and range size m.
func NewIPRF(key [32]byte, n, m uint64) *IPRF {
	var key1, key2 [16]byte
	copy(key1[:], key[:16])
	copy(key2[:], key[16:])

	return &IPRF{
		prp:  NewFeistelPRP(key1, n),
		pmns: NewPMNS(key2, n, m),
	}
}

// Forward evaluates the iPRF at input x (returns y).
// F(x) = S(P(x))
func (iprf *IPRF) Forward(x uint64) uint64 {
	permuted := iprf.prp.Permute(x)
	return iprf.pmns.Forward(permuted)
}

// Inverse finds all inputs x such that F(x) = y.
// F^-1(y) = { P^-1(x) : x in S^-1(y) }
func (iprf *IPRF) Inverse(y uint64) []uint64 {
	preimages := iprf.pmns.Backward(y)
	results := make([]uint64, len(preimages))
	for i, val := range preimages {
		results[i] = iprf.prp.Inverse(val)
	}
	return results
}

// DomainSize returns the size of the domain [0, n-1]
func (iprf *IPRF) DomainSize() uint64 {
	return iprf.prp.n
}

// RangeSize returns the size of the range [0, m-1]
func (iprf *IPRF) RangeSize() uint64 {
	return iprf.pmns.m
}

// --- Pseudorandom Permutation (PRP) ---

// FeistelPRP implements a format-preserving encryption using a Feistel network
// and cycle-walking to achieve a permutation over [0, n-1].
type FeistelPRP struct {
	block cipher.Block
	n     uint64
	bits  uint
}

func NewFeistelPRP(key [16]byte, n uint64) *FeistelPRP {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err)
	}
	
	// Find smallest power of 2 >= n
	bits := uint(0)
	for (uint64(1) << bits) < n {
		bits++
	}
	// Ensure even bits for balanced Feistel if possible, or handle unbalanced.
	// For simplicity in this prototype, we'll just use a simple unbalanced construction 
	// if bits is odd, or just use a slightly larger even domain.
	// Actually, cycle walking works fine even if we process "too many" bits, 
	// as long as the domain isn't tiny compared to the block size.
	// To keep Feistel simple, we'll operate on the smallest even number of bits >= 'bits'.
	if bits%2 != 0 {
		bits++
	}
	if bits < 2 { // Minimum 2 bits for split
		bits = 2
	}

	return &FeistelPRP{
		block: block,
		n:     n,
		bits:  bits,
	}
}

func (p *FeistelPRP) Permute(x uint64) uint64 {
	if x >= p.n {
		// Should not happen if input is valid
		return x
	}

	// Cycle walking
	for {
		x = p.feistelEncrypt(x)
		if x < p.n {
			return x
		}
	}
}

func (p *FeistelPRP) Inverse(y uint64) uint64 {
	if y >= p.n {
		return y
	}
	
	for {
		y = p.feistelDecrypt(y)
		if y < p.n {
			return y
		}
	}
}

func (p *FeistelPRP) feistelEncrypt(val uint64) uint64 {
	halfBits := p.bits / 2
	lowerMask := (uint64(1) << halfBits) - 1
	
	left := (val >> halfBits) & lowerMask
	right := val & lowerMask

	rounds := 4 // 3 or 4 is sufficient for statistical randomness in this context
	
	for i := 0; i < rounds; i++ {
		tmp := right
		f := p.roundFunc(i, right)
		right = left ^ (f & lowerMask)
		left = tmp
	}
	
	return (left << halfBits) | right
}

func (p *FeistelPRP) feistelDecrypt(val uint64) uint64 {
	halfBits := p.bits / 2
	lowerMask := (uint64(1) << halfBits) - 1
	
	left := (val >> halfBits) & lowerMask
	right := val & lowerMask

	rounds := 4
	
	for i := rounds - 1; i >= 0; i-- {
		tmp := left
		f := p.roundFunc(i, left)
		left = right ^ (f & lowerMask)
		right = tmp
	}
	
	return (left << halfBits) | right
}

func (p *FeistelPRP) roundFunc(round int, input uint64) uint64 {
	var data [16]byte
	binary.LittleEndian.PutUint64(data[0:], input)
	binary.LittleEndian.PutUint64(data[8:], uint64(round))
	
	var out [16]byte
	p.block.Encrypt(out[:], data[:])
	
	return binary.LittleEndian.Uint64(out[:8])
}


// --- Pseudorandom Multinomial Sampler (PMNS) ---

type PMNS struct {
	block cipher.Block
	n     uint64 // Total items (set size)
	m     uint64 // Total bins (chunk size)
}

func NewPMNS(key [16]byte, n, m uint64) *PMNS {
	if n == 0 || m == 0 {
		panic("PMNS requires n > 0 and m > 0")
	}
	if m&(m-1) != 0 {
		panic("PMNS currently assumes m is a power of two for Plinko parameters")
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err)
	}
	return &PMNS{block: block, n: n, m: m}
}

// Node state for recursion
type pmnsNode struct {
	start uint64
	count uint64 // number of items in this subtree
	low   uint64 // bin range low
	high  uint64 // bin range high
}

// Forward determines which bin 'x' lands in.
// x is the rank of the item (0 to n-1).
func (s *PMNS) Forward(x uint64) uint64 {
	node := pmnsNode{start: 0, count: s.n, low: 0, high: s.m - 1}
	
	for node.low < node.high {
		leftNode, rightNode := s.children(node)
		
		// Check if x is in the left child's range of items
		// The left child contains items [node.start, node.start + leftNode.count - 1]
		if x < node.start + leftNode.count {
			node = leftNode
		} else {
			node = rightNode
		}
	}
	return node.low
}

// Backward returns all items that land in bin 'y'.
func (s *PMNS) Backward(y uint64) []uint64 {
	node := pmnsNode{start: 0, count: s.n, low: 0, high: s.m - 1}
	
	for node.low < node.high {
		leftNode, rightNode := s.children(node)
		mid := (node.high + node.low) / 2
		
		if y <= mid {
			node = leftNode
		} else {
			node = rightNode
		}
	}
	
	// Node represents the leaf for bin y.
	// Items are [node.start, node.start + node.count - 1]
	result := make([]uint64, node.count)
	for i := uint64(0); i < node.count; i++ {
		result[i] = node.start + i
	}
	return result
}

// children computes the left and right child nodes.
func (s *PMNS) children(node pmnsNode) (pmnsNode, pmnsNode) {
	mid := (node.high + node.low) / 2
	
	// Probability of going left = (bins in left) / (total bins)
	// Left bins: low to mid. Count = mid - low + 1
	// Total bins: high - low + 1
	leftBins := mid - node.low + 1
	totalBins := node.high - node.low + 1
	
	// Sample how many items go left ~ Binomial(node.count, leftBins/totalBins)
	leftCount := s.sampleBinomial(node.count, float64(leftBins)/float64(totalBins), node.low, node.high)
	
	leftNode := pmnsNode{
		start: node.start,
		count: leftCount,
		low:   node.low,
		high:  mid,
	}
	
	rightNode := pmnsNode{
		start: node.start + leftCount,
		count: node.count - leftCount,
		low:   mid + 1,
		high:  node.high,
	}
	
	return leftNode, rightNode
}

// sampleBinomial samples from Binomial(n, p) deterministically based on the node.
func (s *PMNS) sampleBinomial(n uint64, p float64, low, high uint64) uint64 {
	if n == 0 {
		return 0
	}
	// Optimization: if p is 0.5 (common case with power-of-2 range), simpler logic?
	// For now, generic approach.
	
	// Generate randomness using PRF(key, low || high)
	// We need enough random bits to sample Binomial(n, p).
	// Simple approach: n Bernoulli trials.
	// This is O(n). For optimizing, we would use a better sampler.
	// But for the prototype constraints, correctness > speed.
	
	seed := make([]byte, 16)
	binary.LittleEndian.PutUint64(seed[0:], low)
	binary.LittleEndian.PutUint64(seed[8:], high)
	
	// Use a stream of randomness from AES-CTR or similar based on the seed
	// Encrypt a counter 0, 1, 2...
	
	successes := uint64(0)
	
	// Prepare AES cipher for generating randomness
	// We reuse s.block.
	
	var input [16]byte
	copy(input[:], seed)
	var output [16]byte
	
	// We process bits in chunks of 128 (AES block size)
	// Threshold for p: p * 2^64 (for 64-bit comparison) or p * 256 (for byte comparison)
	// To be precise, let's do byte-level comparison if p is simple, or float comparison.
	
	// Optimization: if p == 0.5, we just count set bits in the random stream.
	isHalf := math.Abs(p - 0.5) < 0.000001
	
	processed := uint64(0)
	counter := uint64(0)
	
	for processed < n {
		// Update input with counter to vary the block
		// We use the last 8 bytes of input as a counter (seed is only 16 bytes, 
		// but low/high take 16 bytes. We should probably hash them or use a better IV).
		// Actually, 'low' and 'high' uniquely identify the node in the tree 
		// (for a fixed m and traversal path).
		// So we can just XOR the counter into the input or append.
		// Let's just use input = low || high XOR counter.
		
		binary.LittleEndian.PutUint64(input[0:], low ^ counter) // perturb
		// binary.LittleEndian.PutUint64(input[8:], high) // kept constant-ish
		
		s.block.Encrypt(output[:], input[:])
		counter++
		
		// Process 128 bits (16 bytes)
		for i := 0; i < 16 && processed < n; i++ {
			val := output[i]
			
			if isHalf {
				// Count bits in byte
				for b := 0; b < 8 && processed < n; b++ {
					if (val & (1 << b)) != 0 {
						successes++
					}
					processed++
				}
			} else {
				// Generic p. We need higher precision than 8 bits for arbitrary p?
				// If m is power of 2, p is always 0.5, except maybe at edges if we supported non-power-of-2?
				// Our `deriveParams` forces power of 2 chunk size.
				// But PMNS is generic.
				// If we want to be robust, we need a proper sampler.
				// For this prototype, assuming p=0.5 is heavily dominant,
				// we can fallback to a simple float conversion for the non-0.5 case 
				// or just use the bit method if we can justify p=0.5.
				// Since p = leftBins / totalBins.
				// If totalBins is even and leftBins = totalBins/2, p=0.5.
				// If m is power of 2, this holds recursively until totalBins=1.
				// So p is ALWAYS 0.5 in our setup.
				
				// Let's just implement the p=0.5 path or simple thresholding.
				// Use a random float from the byte? Low precision.
				// Use uint64 from 8 bytes? High precision.
				// Let's use 8 bytes for precision if not 0.5.
				
				// But wait, the loop 'i < 16' iterates bytes.
				// If we need 8 bytes per trial, we consume randomness faster.
				
				// Re-implementing loop for general case correctness:
				// We need a random float [0,1).
				// Let's fetch 4 bytes (uint32) -> normalized float.
				if i+4 <= 16 {
					rndVal := binary.LittleEndian.Uint32(output[i:])
					rndFloat := float64(rndVal) / float64(1<<32)
					if rndFloat < p {
						successes++
					}
					processed++
					i += 3 // skip 3 extra bytes
				}
			}
		}
	}
	
	return successes
}