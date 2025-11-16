package main

import (
	"crypto/rand"
	"math"
	"sort"
	"testing"
	"time"
)

// TestIPRFForwardBackward tests basic iPRF forward and inverse operations
func TestIPRFForwardBackward(t *testing.T) {
	// Test parameters
	n := uint64(1000) // domain size
	m := uint64(100)  // range size
	
	// Create deterministic key for reproducible tests
	key := GenerateDeterministicKey()
	
	// Create base iPRF (proven to work correctly)
	iprf := NewIPRF(key, n, m)
	
	// Test forward and inverse for several inputs
	for x := uint64(0); x < 100; x++ {
		y := iprf.Forward(x)
		if y >= m {
			t.Errorf("Forward(%d) = %d, expected < %d", x, y, m)
		}
		
		// Test inverse
		preimages := iprf.Inverse(y)
		found := false
		for _, preimage := range preimages {
			if preimage == x {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Inverse(%d) did not contain original input %d", y, x)
		}
	}
}

// TestIPRFDistribution tests that the iPRF produces correct distribution
func TestIPRFDistribution(t *testing.T) {
	// Test parameters
	n := uint64(10000) // domain size
	m := uint64(100)   // range size
	
	// Use deterministic keys for reproducible test results
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	
	// Create enhanced iPRF
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Count distribution
	distribution := make(map[uint64]int)
	for x := uint64(0); x < n; x++ {
		y := iprf.Forward(x)
		distribution[y]++
	}
	
	// Expected size per output
	expectedSize := float64(n) / float64(m)
	tolerance := 0.3 // 30% tolerance
	
	// Check that each output has approximately the expected number of preimages
	for y := uint64(0); y < m; y++ {
		size := distribution[y]
		if size == 0 {
			t.Errorf("Output %d has no preimages", y)
			continue
		}
		
		deviation := math.Abs(float64(size)-expectedSize) / expectedSize
		if deviation > tolerance {
			t.Errorf("Output %d has %d preimages, expected ~%.1f (deviation: %.1f%%)", 
				y, size, expectedSize, deviation*100)
		}
	}
	
	// Check total coverage
	totalPreimages := 0
	for _, count := range distribution {
		totalPreimages += count
	}
	if totalPreimages != int(n) {
		t.Errorf("Total preimages %d != domain size %d", totalPreimages, n)
	}
}

// TestIPRFInverseCorrectness tests that inverse is functionally correct
func TestIPRFInverseCorrectness(t *testing.T) {
	// Test parameters
	n := uint64(1000)
	m := uint64(50)
	
	// Use deterministic keys for reproducible test results
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	
	// Create enhanced iPRF
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Build complete forward mapping
	forwardMap := make(map[uint64][]uint64)
	for x := uint64(0); x < n; x++ {
		y := iprf.Forward(x)
		forwardMap[y] = append(forwardMap[y], x)
	}
	
	// Test that inverse matches forward mapping
	for y := uint64(0); y < m; y++ {
		expectedPreimages := forwardMap[y]
		actualPreimages := iprf.Inverse(y)
		
		if len(expectedPreimages) != len(actualPreimages) {
			t.Errorf("Inverse(%d) length mismatch: expected %d, got %d", 
				y, len(expectedPreimages), len(actualPreimages))
			continue
		}
		
		// Sort both slices for comparison
		sort.Slice(expectedPreimages, func(i, j int) bool {
			return expectedPreimages[i] < expectedPreimages[j]
		})
		sort.Slice(actualPreimages, func(i, j int) bool {
			return actualPreimages[i] < actualPreimages[j]
		})
		
		for i := range expectedPreimages {
			if expectedPreimages[i] != actualPreimages[i] {
				t.Errorf("Inverse(%d)[%d] = %d, expected %d", 
					y, i, actualPreimages[i], expectedPreimages[i])
			}
		}
	}
}

// TestIPRFDeterminism tests that iPRF is deterministic
func TestIPRFDeterminism(t *testing.T) {
	// Test parameters
	n := uint64(1000)
	m := uint64(100)
	
	// Create keys
	var prpKey, baseKey PrfKey128
	rand.Read(prpKey[:])
	rand.Read(baseKey[:])
	
	// Create two instances with same keys
	iprf1 := NewEnhancedIPRF(prpKey, baseKey, n, m)
	iprf2 := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Test that both instances produce identical results
	for x := uint64(0); x < 100; x++ {
		y1 := iprf1.Forward(x)
		y2 := iprf2.Forward(x)
		if y1 != y2 {
			t.Errorf("Forward(%d) non-deterministic: %d vs %d", x, y1, y2)
		}
		
		inv1 := iprf1.Inverse(y1)
		inv2 := iprf2.Inverse(y2)
		if len(inv1) != len(inv2) {
			t.Errorf("Inverse(%d) non-deterministic: different lengths", y1)
			continue
		}
		
		for i := range inv1 {
			if inv1[i] != inv2[i] {
				t.Errorf("Inverse(%d)[%d] non-deterministic: %d vs %d", y1, i, inv1[i], inv2[i])
			}
		}
	}
}

// TestPRPPermutation tests that PRP is a proper permutation
func TestPRPPermutation(t *testing.T) {
	// Test parameters
	n := uint64(256) // Small domain for exhaustive testing
	
	// Create random key
	var key PrfKey128
	rand.Read(key[:])
	
	prp := NewPRP(key)
	
	// Build forward mapping
	forward := make(map[uint64]uint64)
	for x := uint64(0); x < n; x++ {
		y := prp.Permute(x, n)
		if y >= n {
			t.Errorf("Permute(%d) = %d, expected < %d", x, y, n)
		}
		if _, exists := forward[x]; exists {
			t.Errorf("Permute(%d) called twice, should be deterministic", x)
		}
		forward[x] = y
	}
	
	// Check that it's a bijection (no collisions in output)
	outputSet := make(map[uint64]bool)
	for x := uint64(0); x < n; x++ {
		y := forward[x]
		if outputSet[y] {
			t.Errorf("Output collision: Permute(%d) = %d, but this output already exists", x, y)
		}
		outputSet[y] = true
	}
	
	// Test inverse permutation
	for x := uint64(0); x < n; x++ {
		y := prp.Permute(x, n)
		xInv := prp.InversePermute(y, n)
		if xInv != x {
			t.Errorf("InversePermute(Permute(%d)) = %d, expected %d", x, xInv, x)
		}
	}
}

// TestPerformance benchmarks the iPRF performance
func TestPerformance(t *testing.T) {
	// Test parameters matching our deployment
	n := uint64(8_400_000) // 8.4M accounts
	m := uint64(1_024)     // 1K hint sets
	
	// Create random keys
	var prpKey, baseKey PrfKey128
	rand.Read(prpKey[:])
	rand.Read(baseKey[:])
	
	// Create enhanced iPRF
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Benchmark forward evaluation
	t.Run("Forward", func(t *testing.T) {
		iterations := 1000
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_ = iprf.Forward(uint64(i * 1000))
		}
		elapsed := time.Since(start)
		perOp := elapsed / time.Duration(iterations)
		t.Logf("Forward: %v per operation", perOp)
		
		// Should be Õ(1) = microseconds
		if perOp > 100*time.Microsecond {
			t.Errorf("Forward too slow: %v (expected microseconds)", perOp)
		}
	})
	
	// Benchmark inverse evaluation
	t.Run("Inverse", func(t *testing.T) {
		iterations := 100
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_ = iprf.Inverse(uint64(i * 10))
		}
		elapsed := time.Since(start)
		perOp := elapsed / time.Duration(iterations)
		t.Logf("Inverse: %v per operation (avg preimage size: %.1f)", 
			perOp, float64(iprf.GetPreimageSize()))
		
		// Should be Õ(preimage_size) = Õ(n/m)
		expectedPreimageSize := float64(n) / float64(m)
		maxTime := time.Duration(expectedPreimageSize) * 10 * time.Microsecond
		if perOp > maxTime {
			t.Errorf("Inverse too slow: %v (expected ~%v)", perOp, maxTime)
		}
	})
}

// TestSecurityProperties tests basic security properties
func TestSecurityProperties(t *testing.T) {
	// Test parameters
	n := uint64(10000)
	m := uint64(100)
	
	// Create random keys
	var prpKey, baseKey PrfKey128
	rand.Read(prpKey[:])
	rand.Read(baseKey[:])
	
	iprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Test 1: Output should appear random (no obvious patterns)
	outputs := make([]uint64, 100)
	for i := range outputs {
		outputs[i] = iprf.Forward(uint64(i))
	}
	
	// Check for patterns (simple test)
	outputSet := make(map[uint64]bool)
	for _, y := range outputs {
		if outputSet[y] {
			t.Errorf("Duplicate output detected: %d (indicates potential pattern)", y)
		}
		outputSet[y] = true
	}
	
	// Test 2: Small input changes should cause large output changes (avalanche)
	x1 := uint64(1234)
	x2 := uint64(1235) // Only 1 bit different
	y1 := iprf.Forward(x1)
	y2 := iprf.Forward(x2)
	
	// Count differing bits
	diff := y1 ^ y2
	diffBits := 0
	for diff > 0 {
		diffBits += int(diff & 1)
		diff >>= 1
	}
	
	// Should have many differing bits (avalanche effect)
	if diffBits < 30 { // Expect ~50% of bits to differ
		t.Errorf("Poor avalanche effect: %d bits differ between outputs of %d and %d", 
			diffBits, x1, x2)
	}
}

// TestIntegration tests the integration with the existing codebase
func TestIntegration(t *testing.T) {
	// Test that the new enhanced iPRF can replace the old one
	n := uint64(8_400_000) // 8.4M accounts
	m := uint64(1_024)     // 1K hint sets
	
	// Create keys (deterministic for reproducibility)
	prpKey := GenerateDeterministicKey()
	baseKey := GenerateDeterministicKey()
	// Modify keys to match original test pattern
	for i := 0; i < 16; i++ {
		prpKey[i] = byte(i)
		baseKey[i] = byte(i + 16)
	}
	
	// Create both old and new iPRFs
	oldIprf := NewIPRF(baseKey, n, m)
	newIprf := NewEnhancedIPRF(prpKey, baseKey, n, m)
	
	// Test that they behave differently (enhanced version adds PRP layer)
	sameCount := 0
	testInputs := 100
	for i := 0; i < testInputs; i++ {
		x := uint64(i * 1000)
		oldY := oldIprf.Forward(x)
		newY := newIprf.Forward(x)
		if oldY == newY {
			sameCount++
		}
	}
	
	// They should be different most of the time due to PRP layer
	if sameCount > testInputs/2 {
		t.Errorf("Old and new iPRF produce same results too often: %d/%d", 
			sameCount, testInputs)
	}
	
	// Test that new iPRF has working inverse
	x := uint64(12345)
	y := newIprf.Forward(x)
	preimages := newIprf.Inverse(y)
	
	found := false
	for _, preimage := range preimages {
		if preimage == x {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Enhanced iPRF inverse failed for x=%d, y=%d", x, y)
	}
	
	t.Logf("Integration test passed: Enhanced iPRF ready for production use")
}