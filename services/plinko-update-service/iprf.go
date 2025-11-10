package main

import (
	"math"
)

// Invertible Pseudorandom Function (iPRF) Implementation
// Simplified version for PoC - maps database indices to hint sets

type PrfKey128 [16]byte

type IPRF struct {
	key       PrfKey128 // PRF key for tree sampling
	domain    uint64    // n: domain size (DBSize)
	range_    uint64    // m: range size (SetSize)
	treeDepth int       // ceiling(log2(m))
}

// NewIPRF creates a new invertible PRF from domain [n] to range [m]
func NewIPRF(key PrfKey128, n uint64, m uint64) *IPRF {
	treeDepth := int(math.Ceil(math.Log2(float64(m))))

	return &IPRF{
		key:       key,
		domain:    n,
		range_:    m,
		treeDepth: treeDepth,
	}
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
		leftCount := iprf.sampleBinomial(nodeID, ballCount, p)

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
	prfOutput := prfEval(&iprf.key, nodeID)

	// Map to (0, 1)
	uniform := (float64(prfOutput) + 1.0) / (float64(uint64(1)<<32) + 2.0)

	// Use inverse CDF
	return iprf.binomialInverseCDF(n, p, uniform)
}

// binomialInverseCDF computes inverse CDF of Binomial(n, p) at point u
func (iprf *IPRF) binomialInverseCDF(n uint64, p float64, u float64) uint64 {
	// Handle edge cases
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

func encodeNode(low uint64, high uint64, n uint64) uint64 {
	// Simple node encoding for PRF input
	return (low << 32) | (high << 16) | (n & 0xFFFF)
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

// prfEval is a simple PRF for demonstration
func prfEval(key *PrfKey128, x uint64) uint64 {
	// Simple PRF using FNV-1a hash
	hash := uint64(2166136261)

	// Mix in key
	for i := 0; i < 16; i++ {
		hash ^= uint64(key[i])
		hash *= 16777619
	}

	// Mix in x
	for i := 0; i < 8; i++ {
		hash ^= (x >> (i * 8)) & 0xFF
		hash *= 16777619
	}

	return hash & 0xFFFFFFFF // Return 32-bit value
}
