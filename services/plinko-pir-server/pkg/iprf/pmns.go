package iprf

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"math"
)

// PMNS implements a Pseudorandom Multinomial Sampler
// as defined in the Plinko paper (Section 4.3, Figure 4).
type PMNS struct {
	key   []byte
	block cipher.Block
	n     uint64 // Domain size (balls)
	m     uint64 // Range size (bins)
}

// NewPMNS creates a new PMNS with the given key, domain size n, and range size m.
func NewPMNS(key []byte, n, m uint64) *PMNS {
	if len(key) != 16 {
		panic("PMNS key must be 16 bytes (AES-128)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	return &PMNS{
		key:   key,
		block: block,
		n:     n,
		m:     m,
	}
}

// node represents a state in the recursion tree
type node struct {
	start uint64 // Start index of balls
	count uint64 // Number of balls
	low   uint64 // Start index of bins
	high  uint64 // End index of bins
}

// S implements the forward evaluation: S(k, x) -> y
// Maps a ball x in [n] to a bin y in [m].
func (p *PMNS) S(x uint64) uint64 {
	if x >= p.n {
		panic("input x out of bounds")
	}

	curr := node{
		start: 0,
		count: p.n,
		low:   0,
		high:  p.m - 1,
	}

	for curr.low < curr.high {
		left, right, s := p.children(curr)

		// If x is in the left set of balls
		if x < curr.start+s {
			curr = left
		} else {
			curr = right
		}
	}

	return curr.low
}

// SInverse implements the backward evaluation: S^-1(k, y) -> {x}
// Maps a bin y in [m] to a set of balls {x} in [n].
// Returns the start index and count of balls in the bin.
// The actual set is {start, start+1, ..., start+count-1}.
func (p *PMNS) SInverse(y uint64) (uint64, uint64) {
	if y >= p.m {
		panic("input y out of bounds")
	}

	curr := node{
		start: 0,
		count: p.n,
		low:   0,
		high:  p.m - 1,
	}

	for curr.low < curr.high {
		left, right, _ := p.children(curr)

		mid := (curr.high + curr.low) / 2

		if y <= mid {
			curr = left
		} else {
			curr = right
		}
	}

	return curr.start, curr.count
}

// children computes the children nodes and the split point s
func (p *PMNS) children(parent node) (node, node, uint64) {
	mid := (parent.high + parent.low) / 2

	// Probability of going left = (bins in left) / (total bins)
	// p = (mid - low + 1) / (high - low + 1)
	leftBins := mid - parent.low + 1
	totalBins := parent.high - parent.low + 1

	// Sample s ~ Binomial(count, p)
	s := p.sampleBinomial(parent.count, leftBins, totalBins, parent)

	left := node{
		start: parent.start,
		count: s,
		low:   parent.low,
		high:  mid,
	}

	right := node{
		start: parent.start + s,
		count: parent.count - s,
		low:   mid + 1,
		high:  parent.high,
	}

	return left, right, s
}

// sampleBinomial samples from Binomial(n, num/denom) using PRF
func (p *PMNS) sampleBinomial(n, num, denom uint64, seedNode node) uint64 {
	if n == 0 {
		return 0
	}

	// Generate pseudorandom float in [0, 1)
	// We use the node parameters to seed the PRF
	// Input: "PMNS" || start || count || low || high
	var input [32]byte
	binary.BigEndian.PutUint64(input[0:], seedNode.start)
	binary.BigEndian.PutUint64(input[8:], seedNode.count)
	binary.BigEndian.PutUint64(input[16:], seedNode.low)
	binary.BigEndian.PutUint64(input[24:], seedNode.high)

	// Encrypt to get randomness
	// var output [32]byte // Unused
	// Actually AES block size is 16 bytes. Let's just use 16 bytes.
	var blockOut [16]byte
	p.block.Encrypt(blockOut[:], input[:16]) // Encrypt first half
	// XOR with second half to mix? Or just encrypt a counter.
	// For simplicity and determinism, let's just encrypt the struct bytes.
	// But we need 16 bytes input.
	// Let's hash the node to 16 bytes or just use a counter mode if we needed more.
	// For now, let's just encrypt the first 16 bytes of the struct representation (start, count).
	// Wait, low and high are also important for uniqueness.
	// Let's use a hash or just multiple rounds.
	// Simple approach: Encrypt(start ^ low) and Encrypt(count ^ high) and XOR?

	// Better: Encrypt(Hash(node))
	// But we want to avoid heavy deps if possible.
	// Let's just use a simple mixing:
	var iv [16]byte
	binary.BigEndian.PutUint64(iv[0:], seedNode.low)
	binary.BigEndian.PutUint64(iv[8:], seedNode.high)

	// XOR input with IV
	for i := 0; i < 16; i++ {
		input[i] ^= iv[i]
	}

	p.block.Encrypt(blockOut[:], input[:16])

	// Convert to float for binomial sampling
	// This is a simplified binomial sampling.
	// For large n, we should use normal approximation or similar.
	// For the paper's exact distribution, we need to be careful.
	// The paper says: s <- Binomial(count, p; F(k, node))

	// Inverse Transform Sampling for Binomial is slow for large n.
	// Normal approximation is standard for large n.

	prob := float64(num) / float64(denom)

	// Use 64 bits of randomness for uniform float
	randVal := binary.BigEndian.Uint64(blockOut[:8])
	u := float64(randVal) / float64(math.MaxUint64)

	return inverseBinomial(n, prob, u)
}

// inverseBinomial computes the inverse CDF of the binomial distribution
// This is a placeholder for a robust implementation.
// For large n, we use Normal approximation.
func inverseBinomial(n uint64, p float64, u float64) uint64 {
	if n > 50 {
		// Normal approximation
		mean := float64(n) * p
		stdDev := math.Sqrt(float64(n) * p * (1 - p))

		// Inverse Error Function approximation or Box-Muller?
		// Actually we have u uniform.
		// z = InverseNormal(u)
		// k = mean + z * stdDev

		z := math.Sqrt(-2.0*math.Log(u)) * math.Cos(2.0*math.Pi*u) // Box-Muller requires 2 randoms
		// We only have 1 u.
		// Let's use a simple approximation for InverseNormal (probit function)
		// or just use a library if available. Go math doesn't have Erfinv.

		// For this PoC, let's use a very simple approximation or just return mean (which is bad for distribution).
		// Let's implement a simple quantile function for Normal.

		// Beasley-Springer-Moro algorithm for inverse normal CDF
		z = normalInv(u)
		res := math.Round(mean + z*stdDev)
		if res < 0 {
			return 0
		}
		if res > float64(n) {
			return n
		}
		return uint64(res)
	}

	// Exact calculation for small n
	var sum float64 = 0
	for k := uint64(0); k <= n; k++ {
		prob := binomialProb(n, k, p)
		sum += prob
		if sum >= u {
			return k
		}
	}
	return n
}

func binomialProb(n, k uint64, p float64) float64 {
	// nCk * p^k * (1-p)^(n-k)
	// Use log gamma for stability if needed, but for n<=50 direct is okay
	combinations := 1.0
	for i := uint64(0); i < k; i++ {
		combinations *= float64(n-i) / float64(i+1)
	}
	return combinations * math.Pow(p, float64(k)) * math.Pow(1-p, float64(n-k))
}

// normalInv approximates the inverse standard normal CDF
// Source: Beasley-Springer-Moro algorithm
func normalInv(p float64) float64 {
	if p <= 0 || p >= 1 {
		return 0
	}

	a := [4]float64{
		2.50662823884, -18.61500062529, 41.39119773534, -25.44106049637,
	}
	b := [4]float64{
		-8.47351093090, 23.08336743743, -21.06224101826, 3.13082909833,
	}
	c := [9]float64{
		0.3374754822726147, 0.9761690190917186, 0.1607979714918209,
		0.0276438810333863, 0.0038405729373609, 0.0003951896511919,
		0.0000321767881768, 0.0000002888167364, 0.0000003960315187,
	}

	y := p - 0.5
	if math.Abs(y) < 0.42 {
		r := y * y
		return y * (((a[3]*r+a[2])*r+a[1])*r + a[0]) / ((((b[3]*r+b[2])*r+b[1])*r+b[0])*r + 1)
	}

	r := p
	if y > 0 {
		r = 1 - p
	}
	r = math.Log(-math.Log(r))
	x := c[0] + r*(c[1]+r*(c[2]+r*(c[3]+r*(c[4]+r*(c[5]+r*(c[6]+r*(c[7]+r*c[8])))))))
	if y < 0 {
		return -x
	}
	return x
}
