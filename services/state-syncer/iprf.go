package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// Invertible Pseudorandom Function (iPRF) Implementation
// Simplified version for PoC - maps database indices to hint sets

type PrfKey128 [16]byte

type IPRF struct {
	key       PrfKey128 // PRF key for tree sampling
	block     cipher.Block
	domain    uint64 // n: domain size (DBSize)
	range_    uint64 // m: range size (SetSize)
	treeDepth int    // ceiling(log2(m))
}

const invTwoTo53 = 1.0 / (1 << 53)

// NewIPRF creates a new invertible PRF from domain [n] to range [m]
func NewIPRF(key PrfKey128, n uint64, m uint64) *IPRF {
	treeDepth := int(math.Ceil(math.Log2(float64(m))))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		panic(err)
	}

	return &IPRF{
		key:       key,
		block:     block,
		domain:    n,
		range_:    m,
		treeDepth: treeDepth,
	}
}
// GenerateRandomKey creates a cryptographically secure random key
// WARNING: For production use, prefer DeriveIPRFKey for deterministic key derivation
// Random keys break persistence across server restarts, invalidating cached hints
func GenerateRandomKey() PrfKey128 {
	var key PrfKey128
	_, err := rand.Read(key[:])
	if err != nil {
		panic("failed to generate random key: " + err.Error())
	}
	return key
}

// DeriveIPRFKey derives a deterministic iPRF key from master secret and context
// This ensures the same key is generated across server restarts, preventing
// hint invalidation.
//
// BUG #6 FIX: Using random keys (GenerateRandomKey) causes different iPRF mappings
// after each server restart, invalidating all cached hints. Paper (Section 5.2)
// specifies using PRF-based key derivation from a master secret.
//
// Paper context (Section 5.2):
// "The n/r keys for each of the iPRFs can also be pseudorandomly generated
// using a PRF. Therefore, this only requires storing a single PRF key."
//
// Parameters:
//   - masterSecret: Long-term secret stored securely (e.g., from config/KMS)
//   - context: Application-specific context string (e.g., "plinko-iprf-v1")
//
// Returns: Deterministic 128-bit key for iPRF
func DeriveIPRFKey(masterSecret []byte, context string) PrfKey128 {
	h := sha256.New()
	h.Write(masterSecret)
	h.Write([]byte("iprf-key-derivation-v1")) // Domain separator
	h.Write([]byte(context))

	var key PrfKey128
	copy(key[:], h.Sum(nil)[:16])
	return key
}

// NewIPRFFromMasterSecret creates iPRF with deterministic key derivation
// Use this instead of NewIPRF(GenerateRandomKey(), ...) for production deployments
//
// Example usage:
//   masterSecret := loadMasterSecret("/etc/app/master.key")
//   iprf := NewIPRFFromMasterSecret(masterSecret, "plinko-iprf-v1", 8400000, 1024)
func NewIPRFFromMasterSecret(masterSecret []byte, context string, domain uint64, range_ uint64) *IPRF {
	key := DeriveIPRFKey(masterSecret, context)
	return NewIPRF(key, domain, range_)
}

// GenerateDeterministicKey creates a deterministic key for testing (NOT for production)
func GenerateDeterministicKey() PrfKey128 {
	var key PrfKey128
	for i := 0; i < 16; i++ {
		key[i] = byte(i)
	}
	return key
}

// GenerateDeterministicKeyWithSeed creates a deterministic key from a seed (NOT for production)
func GenerateDeterministicKeyWithSeed(seed uint64) PrfKey128 {
	var key PrfKey128
	for i := 0; i < 16; i++ {
		key[i] = byte((seed + uint64(i)*17 + 1) % 256)
	}
	return key
}

// Forward evaluates the iPRF: maps x in [n] to y in [m]
// Uses binomial tree sampling
func (iprf *IPRF) Forward(x uint64) uint64 {
	if x >= iprf.domain {
		return 0
	}

	// Trace through binary tree to find bin
	return iprf.traceBall(x, iprf.domain, iprf.range_)
}

// traceBall follows ball x through the binary tree to find its bin
func (iprf *IPRF) traceBall(xPrime uint64, n uint64, m uint64) uint64 {
	if m == 1 {
		return 0 // Only one bin
	}

	// Current position in tree
	low := uint64(0)
	high := m - 1
	ballCount := n
	ballIndex := xPrime

	for low < high {
		mid := (low + high) / 2
		leftBins := mid - low + 1
		totalBins := high - low + 1

		// Probability ball goes left
		p := float64(leftBins) / float64(totalBins)

		// Sample binomial to determine split point
		nodeID := encodeNode(low, high, n)
		prfOutput := iprf.prfEval(nodeID)

		// Map to (0, 1)
		uniform := (float64(prfOutput>>11) + 0.5) * invTwoTo53

		// Use inverse CDF
		leftCount := iprf.binomialInverseCDF(ballCount, p, uniform)

		// Determine if ball xPrime goes left or right
		if ballIndex < leftCount {
			// Ball goes left
			high = mid
			ballCount = leftCount
		} else {
			// Ball goes right
			low = mid + 1
			ballIndex = ballIndex - leftCount
			ballCount = ballCount - leftCount
		}
	}

	return low
}

// sampleBinomial samples from Binomial(n, p) using PRF
func (iprf *IPRF) sampleBinomial(nodeID uint64, n uint64, p float64) uint64 {
	// Use PRF to generate deterministic random value
	prfOutput := iprf.prfEval(nodeID)

	// Map to (0, 1)
	uniform := (float64(prfOutput>>11) + 0.5) * invTwoTo53

	// Use inverse CDF
	return iprf.binomialInverseCDF(n, p, uniform)
}

// binomialInverseCDF computes inverse CDF of Binomial(n, p) at point u
//
// Edge cases:
//   u <= 0.0 → returns 0 (no balls)
//   u >= 1.0 → returns n (all balls)
//   0 < u < 1 → returns k such that P(X ≤ k) ≥ u and P(X ≤ k-1) < u
func (iprf *IPRF) binomialInverseCDF(n uint64, p float64, u float64) uint64 {
	// Handle u edge cases first (FIX #2: u=1.0 should return n)
	if u <= 0.0 {
		return 0
	}
	if u >= 1.0 {
		return n
	}

	// Handle p edge cases
	if p == 0 {
		return 0
	}
	if p == 1 {
		return n
	}
	if n == 0 {
		return 0
	}

	// For large n, use normal approximation
	if n > 100 {
		return iprf.normalApproxBinomial(n, p, u)
	}

	// For small n, use exact cumulative distribution
	cumProb := 0.0
	q := 1.0 - p

	// Start with P(X = 0) = q^n
	prob := math.Pow(q, float64(n))
	cumProb += prob

	if u <= cumProb {
		return 0
	}

	// Compute remaining probabilities using recurrence
	for k := uint64(0); k < n; k++ {
		prob = prob * float64(n-k) / float64(k+1) * p / q
		cumProb += prob

		if u <= cumProb {
			return k + 1
		}
	}

	return n
}

// normalApproxBinomial uses normal approximation for large n
func (iprf *IPRF) normalApproxBinomial(n uint64, p float64, u float64) uint64 {
	// Normal approximation: X ~ N(np, np(1-p))
	mean := float64(n) * p
	variance := float64(n) * p * (1 - p)
	stddev := math.Sqrt(variance)

	// Clamp u to safe range
	uClamped := u
	if uClamped <= 0.001 {
		uClamped = 0.001
	}
	if uClamped >= 0.999 {
		uClamped = 0.999
	}

	// Inverse normal CDF
	z := invNormalCDF(uClamped)
	result := mean + z*stddev

	// Clamp to valid range [0, n]
	if result < 0 {
		return 0
	}
	if result > float64(n) {
		return n
	}

	return uint64(math.Round(result))
}

// GetPreimageSize returns expected size of Inverse(y) for any y
func (iprf *IPRF) GetPreimageSize() uint64 {
	return uint64(math.Ceil(float64(iprf.domain) / float64(iprf.range_)))
}

// Helper functions

// encodeNode creates a unique identifier for a tree node using cryptographic hash
// This ensures no collisions even for large domain sizes (n > 2^16)
//
// BUG #7 FIX: Previous implementation used (low << 32) | (high << 16) | (n & 0xFFFF)
// which truncated n to 16 bits, causing collisions when n > 65535.
//
// Paper requirement (Figure 4): Node must uniquely identify position in tree
// for deterministic PRF evaluation: F(k, node)
//
// Hash-based approach guarantees uniqueness across all three parameters without
// bit-width limitations while maintaining determinism.
func encodeNode(low uint64, high uint64, n uint64) uint64 {
	// Use SHA-256 to guarantee uniqueness across all parameters
	h := sha256.New()

	// Encode all three parameters in big-endian format
	var buf [24]byte
	binary.BigEndian.PutUint64(buf[0:8], low)
	binary.BigEndian.PutUint64(buf[8:16], high)
	binary.BigEndian.PutUint64(buf[16:24], n)

	h.Write(buf[:])

	// Use first 8 bytes of hash as 64-bit node ID
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}

// invNormalCDF computes approximate inverse normal CDF
func invNormalCDF(p float64) float64 {
	if p <= 0 || p >= 1 {
		if p == 0 {
			return -10.0
		}
		if p == 1 {
			return 10.0
		}
		return 0.0
	}

	// Rational approximation for central region
	const (
		a0 = 2.50662823884
		a1 = -18.61500062529
		a2 = 41.39119773534
		a3 = -25.44106049637

		b0 = -8.47351093090
		b1 = 23.08336743743
		b2 = -21.06224101826
		b3 = 3.13082909833
	)

	y := p - 0.5

	if math.Abs(y) < 0.42 {
		// Central region
		r := y * y
		return y * (((a3*r+a2)*r+a1)*r + a0) / ((((b3*r+b2)*r+b1)*r+b0)*r + 1)
	}

	// Tail region - simplified
	if y > 0 {
		return 2.0
	}
	return -2.0
}

// prfEval evaluates AES-128(key, x) and returns the upper 64 bits
func (iprf *IPRF) prfEval(x uint64) uint64 {
	var input [aes.BlockSize]byte
	binary.BigEndian.PutUint64(input[aes.BlockSize-8:], x)

	var output [aes.BlockSize]byte
	iprf.block.Encrypt(output[:], input[:])

	return binary.BigEndian.Uint64(output[:8])
}
