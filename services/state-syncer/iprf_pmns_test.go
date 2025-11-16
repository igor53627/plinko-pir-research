package main

import (
	"fmt"
	"math"
	"sort"
	"testing"
)

// ========================================
// PMNS CORRECTNESS TESTS (Bugs 4, 5, 8)
// ========================================
// These tests validate the PMNS (Pseudo-Multinomial Sampling) layer
// which uses binomial tree sampling to distribute elements into bins

// TestPMNSCorrectness validates the base iPRF (acting as PMNS)
// Bug 4: Binomial sampling uses n instead of ballCount
// Expected: FAIL - Forward-inverse round trips will fail
func TestPMNSCorrectness(t *testing.T) {
	testCases := []struct {
		name string
		n    uint64
		m    uint64
	}{
		{"small equal", 100, 10},
		{"medium skewed", 1000, 100},
		{"large skewed", 10000, 100},
		{"realistic scale", 100000, 1024},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := GenerateDeterministicKey()
			iprf := NewIPRF(key, tc.n, tc.m)

			t.Run("forward-inverse round trip", func(t *testing.T) {
				// Property: For all x ∈ [0, n), x ∈ Inverse(Forward(x))
				// This tests the fundamental PMNS correctness

				// Test all values for small n, sample for large n
				testCount := tc.n
				if testCount > 1000 {
					testCount = 1000
				}

				stride := tc.n / testCount
				if stride == 0 {
					stride = 1
				}

				for i := uint64(0); i < testCount; i++ {
					x := i * stride
					if x >= tc.n {
						break
					}

					y := iprf.Forward(x)
					if y >= tc.m {
						t.Errorf("Forward(%d) = %d >= m=%d (out of range)", x, y, tc.m)
						continue
					}

					preimages := iprf.Inverse(y)

					// Check x is in the preimage set
					found := false
					for _, preimage := range preimages {
						if preimage == x {
							found = true
							break
						}
					}

					if !found {
						t.Errorf("PMNS round trip failed: x=%d maps to y=%d, but x ∉ Inverse(%d)",
							x, y, y)
						t.Logf("Inverse(%d) = %v (size=%d)", y, preimages, len(preimages))
					}
				}
			})

			t.Run("no element in wrong bin", func(t *testing.T) {
				// Property: If x ∈ Inverse(y), then Forward(x) = y
				// Test a sample of outputs

				testOutputs := tc.m
				if testOutputs > 100 {
					testOutputs = 100
				}

				stride := tc.m / testOutputs
				if stride == 0 {
					stride = 1
				}

				for i := uint64(0); i < testOutputs; i++ {
					y := i * stride
					if y >= tc.m {
						break
					}

					preimages := iprf.Inverse(y)

					for _, x := range preimages {
						if x >= tc.n {
							t.Errorf("Inverse(%d) returned x=%d >= n=%d (out of domain)", y, x, tc.n)
							continue
						}

						yCheck := iprf.Forward(x)
						if yCheck != y {
							t.Errorf("PMNS consistency failed: x=%d in Inverse(%d), but Forward(%d)=%d",
								x, y, x, yCheck)
						}
					}
				}
			})

			t.Run("total coverage", func(t *testing.T) {
				// Property: Sum of all preimage sizes = n
				// Every element must map somewhere

				totalPreimages := uint64(0)
				for y := uint64(0); y < tc.m; y++ {
					preimages := iprf.Inverse(y)
					totalPreimages += uint64(len(preimages))
				}

				if totalPreimages != tc.n {
					t.Errorf("PMNS total coverage failed: sum of preimage sizes = %d, expected n=%d",
						totalPreimages, tc.n)
					t.Errorf("This indicates Bug 4: elements are missing or duplicated")
				}
			})
		})
	}
}

// TestNodeEncodingUniqueness validates node encoding doesn't cause collisions
// Bug 5: Node encoding uses bit-shifting that causes collisions
// Expected: FAIL - Large values will produce collisions
func TestNodeEncodingUniqueness(t *testing.T) {
	t.Run("determinism", func(t *testing.T) {
		// Same (low, high, n) should always produce same encoding
		testCases := []struct {
			low  uint64
			high uint64
			n    uint64
		}{
			{0, 10, 100},
			{5, 15, 100},
			{0, 1000, 10000},
			{500, 999, 10000},
		}

		for _, tc := range testCases {
			enc1 := encodeNode(tc.low, tc.high, tc.n)
			enc2 := encodeNode(tc.low, tc.high, tc.n)

			if enc1 != enc2 {
				t.Errorf("encodeNode(%d, %d, %d) non-deterministic: %d vs %d",
					tc.low, tc.high, tc.n, enc1, enc2)
			}
		}
	})

	t.Run("no collisions small values", func(t *testing.T) {
		// Different nodes should produce different encodings
		seen := make(map[uint64]string)

		// Test various tree configurations
		for n := uint64(100); n <= 1000; n += 100 {
			for low := uint64(0); low < 10; low++ {
				for high := low; high < low+10 && high < 100; high++ {
					encoding := encodeNode(low, high, n)

					key := fmt.Sprintf("(%d, %d, %d)", low, high, n)
					if prevKey, exists := seen[encoding]; exists {
						t.Errorf("Node encoding collision: %s and %s both encode to %d",
							prevKey, key, encoding)
					}
					seen[encoding] = key
				}
			}
		}
	})

	t.Run("no collisions large values", func(t *testing.T) {
		// Bug 5: Test with large values that exceed 16-bit limits
		seen := make(map[uint64]string)

		testCases := []struct {
			low  uint64
			high uint64
			n    uint64
		}{
			// Values > 2^16 (65536) will expose bit-shifting overflow
			{0, 100000, 1000000},
			{50000, 100000, 1000000},
			{0, 1000000, 10000000},
			{500000, 1000000, 10000000},
			// Values > 2^32 will definitely expose overflow
			{0, 5000000000, 10000000000},
			{1000000000, 5000000000, 10000000000},
		}

		for _, tc := range testCases {
			encoding := encodeNode(tc.low, tc.high, tc.n)
			key := fmt.Sprintf("(%d, %d, %d)", tc.low, tc.high, tc.n)

			if prevKey, exists := seen[encoding]; exists {
				t.Errorf("Node encoding collision (large values): %s and %s both encode to %d",
					prevKey, key, encoding)
				t.Errorf("This is Bug 5: bit-shifting overflow in encodeNode")
			}
			seen[encoding] = key
		}
	})

	t.Run("encoding properties", func(t *testing.T) {
		// Test that encoding preserves some basic properties
		enc1 := encodeNode(0, 10, 100)
		enc2 := encodeNode(0, 10, 200)

		// Different n should produce different encodings
		if enc1 == enc2 {
			t.Errorf("Different n values produced same encoding: (%d,%d,%d) and (%d,%d,%d)",
				0, 10, 100, 0, 10, 200)
		}

		enc3 := encodeNode(0, 10, 100)
		enc4 := encodeNode(0, 20, 100)

		// Different high should produce different encodings
		if enc3 == enc4 {
			t.Errorf("Different high values produced same encoding: (%d,%d,%d) and (%d,%d,%d)",
				0, 10, 100, 0, 20, 100)
		}
	})

	t.Run("collision probability estimate", func(t *testing.T) {
		// Generate many random node encodings and check collision rate
		seen := make(map[uint64]bool)
		totalTests := 10000
		collisions := 0

		for i := 0; i < totalTests; i++ {
			// Generate random node parameters
			low := uint64(i * 100)
			high := low + uint64(i%1000)
			n := uint64(1000000 + i*1000)

			encoding := encodeNode(low, high, n)

			if seen[encoding] {
				collisions++
			}
			seen[encoding] = true
		}

		collisionRate := float64(collisions) / float64(totalTests)
		if collisionRate > 0.01 { // More than 1% collision rate is problematic
			t.Errorf("High collision rate: %.2f%% (%d/%d collisions)",
				collisionRate*100, collisions, totalTests)
			t.Errorf("This indicates Bug 5: encoding function has insufficient entropy")
		}
	})
}

// TestBinCollectionComplete validates that collectBallsInBin finds all elements
// Bug 8: Incomplete recursion in bin collection misses elements
// Expected: FAIL - Some preimages will be missing
func TestBinCollectionComplete(t *testing.T) {
	testCases := []struct {
		name string
		n    uint64
		m    uint64
	}{
		{"small", 100, 10},
		{"medium", 1000, 100},
		{"large", 10000, 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := GenerateDeterministicKey()
			iprf := NewIPRF(key, tc.n, tc.m)

			t.Run("compare against brute force", func(t *testing.T) {
				// Build brute-force forward mapping
				forwardMap := make(map[uint64][]uint64)
				for x := uint64(0); x < tc.n; x++ {
					y := iprf.Forward(x)
					forwardMap[y] = append(forwardMap[y], x)
				}

				// Sort all preimage lists
				for y := range forwardMap {
					sort.Slice(forwardMap[y], func(i, j int) bool {
						return forwardMap[y][i] < forwardMap[y][j]
					})
				}

				// Test each output
				testOutputs := tc.m
				if testOutputs > 100 {
					testOutputs = 100
				}

				stride := tc.m / testOutputs
				if stride == 0 {
					stride = 1
				}

				for i := uint64(0); i < testOutputs; i++ {
					y := i * stride
					if y >= tc.m {
						break
					}

					expectedPreimages := forwardMap[y]
					actualPreimages := iprf.enumerateBallsInBin(y, tc.n, tc.m)

					// Sort actual preimages
					sort.Slice(actualPreimages, func(i, j int) bool {
						return actualPreimages[i] < actualPreimages[j]
					})

					// Check lengths match
					if len(actualPreimages) != len(expectedPreimages) {
						t.Errorf("enumerateBallsInBin(%d) incomplete: got %d elements, expected %d",
							y, len(actualPreimages), len(expectedPreimages))
						t.Errorf("This is Bug 8: incomplete recursion in bin collection")

						// Show which elements are missing
						missing := []uint64{}
						for _, exp := range expectedPreimages {
							found := false
							for _, act := range actualPreimages {
								if act == exp {
									found = true
									break
								}
							}
							if !found {
								missing = append(missing, exp)
							}
						}
						if len(missing) > 0 && len(missing) < 10 {
							t.Errorf("Missing elements: %v", missing)
						} else if len(missing) > 0 {
							t.Errorf("Missing %d elements (showing first 10): %v",
								len(missing), missing[:10])
						}
						continue
					}

					// Check contents match
					for i := range expectedPreimages {
						if actualPreimages[i] != expectedPreimages[i] {
							t.Errorf("enumerateBallsInBin(%d)[%d] = %d, expected %d",
								y, i, actualPreimages[i], expectedPreimages[i])
						}
					}
				}
			})

			t.Run("no duplicate elements", func(t *testing.T) {
				// Each element should appear exactly once across all bins
				elementCount := make(map[uint64]int)

				for y := uint64(0); y < tc.m; y++ {
					preimages := iprf.enumerateBallsInBin(y, tc.n, tc.m)

					for _, x := range preimages {
						elementCount[x]++
					}
				}

				// Check each element appears exactly once
				for x := uint64(0); x < tc.n; x++ {
					count := elementCount[x]
					if count != 1 {
						t.Errorf("Element %d appears %d times across all bins (expected 1)",
							x, count)
						if count == 0 {
							t.Errorf("This indicates Bug 8: element %d is missing from all bins", x)
						}
					}
				}
			})

			t.Run("all elements covered", func(t *testing.T) {
				// Union of all bin preimages should equal [0, n)
				allElements := make(map[uint64]bool)

				for y := uint64(0); y < tc.m; y++ {
					preimages := iprf.enumerateBallsInBin(y, tc.n, tc.m)
					for _, x := range preimages {
						allElements[x] = true
					}
				}

				if uint64(len(allElements)) != tc.n {
					t.Errorf("enumerateBallsInBin coverage incomplete: %d/%d elements found",
						len(allElements), tc.n)

					// Find missing elements
					missing := []uint64{}
					for x := uint64(0); x < tc.n; x++ {
						if !allElements[x] {
							missing = append(missing, x)
							if len(missing) >= 10 {
								break
							}
						}
					}
					t.Errorf("Missing elements (showing up to 10): %v", missing)
					t.Errorf("This is Bug 8: incomplete bin collection")
				}
			})
		})
	}
}

// TestPMNSDistribution validates statistical properties
// This helps detect Bug 4 through distribution anomalies
func TestPMNSDistribution(t *testing.T) {
	key := GenerateDeterministicKey()
	n := uint64(10000)
	m := uint64(100)
	iprf := NewIPRF(key, n, m)

	t.Run("binomial distribution shape", func(t *testing.T) {
		// Build distribution
		distribution := make(map[uint64]int)
		for x := uint64(0); x < n; x++ {
			y := iprf.Forward(x)
			distribution[y]++
		}

		// Calculate statistics
		expectedSize := float64(n) / float64(m)

		var sizes []int
		for _, size := range distribution {
			sizes = append(sizes, size)
		}
		sort.Ints(sizes)

		// Calculate mean and variance
		var sum, sumSq float64
		for _, size := range sizes {
			sum += float64(size)
			sumSq += float64(size) * float64(size)
		}
		mean := sum / float64(len(sizes))
		variance := (sumSq / float64(len(sizes))) - (mean * mean)
		stddev := math.Sqrt(variance)

		t.Logf("Distribution: mean=%.2f (expected=%.2f), stddev=%.2f",
			mean, expectedSize, stddev)

		// Mean should be close to expected
		if math.Abs(mean-expectedSize) > expectedSize*0.1 {
			t.Errorf("Distribution mean %.2f differs from expected %.2f by >10%%",
				mean, expectedSize)
		}

		// Check for reasonable spread (binomial variance = n*p*(1-p))
		p := 1.0 / float64(m)
		expectedVariance := float64(n) * p * (1 - p) / float64(m)
		expectedStddev := math.Sqrt(expectedVariance)

		t.Logf("Stddev: actual=%.2f, expected~%.2f", stddev, expectedStddev)
	})

	t.Run("chi-squared test", func(t *testing.T) {
		// Build distribution
		observed := make([]int, m)
		for x := uint64(0); x < n; x++ {
			y := iprf.Forward(x)
			observed[y]++
		}

		// Expected count per bin (uniform)
		expected := float64(n) / float64(m)

		// Calculate chi-squared statistic
		chiSquared := 0.0
		for _, obs := range observed {
			diff := float64(obs) - expected
			chiSquared += (diff * diff) / expected
		}

		// Degrees of freedom = m - 1
		df := m - 1

		// For large df, chi-squared is approximately normal
		// Mean = df, Variance = 2*df
		// Check if within reasonable bounds (say, within 3 standard deviations)
		mean := float64(df)
		stddev := math.Sqrt(2.0 * float64(df))
		zScore := (chiSquared - mean) / stddev

		t.Logf("Chi-squared: %.2f (df=%d, z-score=%.2f)", chiSquared, df, zScore)

		if math.Abs(zScore) > 3.0 {
			t.Errorf("Chi-squared test failed: z-score=%.2f (expected within ±3)",
				zScore)
			t.Logf("This may indicate Bug 4: incorrect binomial sampling")
		}
	})

	t.Run("no empty bins", func(t *testing.T) {
		// With n >> m, all bins should have at least one element
		distribution := make(map[uint64]int)
		for x := uint64(0); x < n; x++ {
			y := iprf.Forward(x)
			distribution[y]++
		}

		emptyBins := 0
		for y := uint64(0); y < m; y++ {
			if distribution[y] == 0 {
				emptyBins++
			}
		}

		if emptyBins > 0 {
			t.Errorf("Found %d empty bins (out of %d total)", emptyBins, m)
			t.Errorf("This indicates Bug 4: distribution is broken")
		}
	})
}

// TestPMNSTreeStructure validates internal tree structure
func TestPMNSTreeStructure(t *testing.T) {
	key := GenerateDeterministicKey()
	n := uint64(1000)
	m := uint64(100)
	iprf := NewIPRF(key, n, m)

	t.Run("tree depth correct", func(t *testing.T) {
		expectedDepth := int(math.Ceil(math.Log2(float64(m))))
		if iprf.treeDepth != expectedDepth {
			t.Errorf("Tree depth = %d, expected %d = ceil(log2(%d))",
				iprf.treeDepth, expectedDepth, m)
		}
	})

	t.Run("binary splits are consistent", func(t *testing.T) {
		// For a given node, left + right counts should equal total
		// Test a few known tree positions

		// Root node: should split n elements into two subtrees
		nodeID := encodeNode(0, m-1, n)
		mid := (m - 1) / 2
		leftBins := mid + 1
		totalBins := m
		p := float64(leftBins) / float64(totalBins)

		leftCount := iprf.sampleBinomial(nodeID, n, p)
		rightCount := n - leftCount

		t.Logf("Root split: left=%d, right=%d, total=%d", leftCount, rightCount, n)

		if leftCount+rightCount != n {
			t.Errorf("Root split inconsistent: left(%d) + right(%d) = %d ≠ n(%d)",
				leftCount, rightCount, leftCount+rightCount, n)
		}

		// Determinism: same node should always produce same split
		leftCount2 := iprf.sampleBinomial(nodeID, n, p)
		if leftCount != leftCount2 {
			t.Errorf("Binomial sampling non-deterministic: %d vs %d", leftCount, leftCount2)
		}
	})
}

// TestBinomialInverseCDF validates the binomial inverse CDF implementation
func TestBinomialInverseCDF(t *testing.T) {
	key := GenerateDeterministicKey()
	iprf := NewIPRF(key, 100, 10)

	t.Run("edge cases", func(t *testing.T) {
		// u=0 should give 0
		result := iprf.binomialInverseCDF(100, 0.5, 0.0)
		if result != 0 {
			t.Errorf("binomialInverseCDF(n=100, p=0.5, u=0) = %d, expected 0", result)
		}

		// u=1 should give n
		result = iprf.binomialInverseCDF(100, 0.5, 1.0)
		if result != 100 {
			t.Errorf("binomialInverseCDF(n=100, p=0.5, u=1) = %d, expected 100", result)
		}

		// p=0 should give 0
		result = iprf.binomialInverseCDF(100, 0.0, 0.5)
		if result != 0 {
			t.Errorf("binomialInverseCDF(n=100, p=0, u=0.5) = %d, expected 0", result)
		}

		// p=1 should give n
		result = iprf.binomialInverseCDF(100, 1.0, 0.5)
		if result != 100 {
			t.Errorf("binomialInverseCDF(n=100, p=1, u=0.5) = %d, expected 100", result)
		}
	})

	t.Run("monotonicity", func(t *testing.T) {
		// For fixed n, p, increasing u should produce non-decreasing results
		n := uint64(100)
		p := 0.5

		var prev uint64 = 0
		for i := 0; i < 100; i++ {
			u := float64(i) / 100.0
			result := iprf.binomialInverseCDF(n, p, u)

			if result < prev {
				t.Errorf("binomialInverseCDF not monotonic: u=%.2f gives %d < %d (u=%.2f)",
					u, result, prev, float64(i-1)/100.0)
			}
			prev = result
		}
	})

	t.Run("range bounds", func(t *testing.T) {
		// Result should always be in [0, n]
		n := uint64(100)
		p := 0.7

		for i := 0; i <= 100; i++ {
			u := float64(i) / 100.0
			result := iprf.binomialInverseCDF(n, p, u)

			if result > n {
				t.Errorf("binomialInverseCDF(n=%d, p=%.2f, u=%.2f) = %d > n",
					n, p, u, result)
			}
		}
	})
}
