package main

import (
	"fmt"
	"testing"
	"time"
)

// TestInversePerformance compares brute force vs optimized inverse performance
func TestInversePerformance(t *testing.T) {
	// Test with different database sizes
	testSizes := []uint64{1000, 10000, 100000}
	
	for _, size := range testSizes {
		t.Run(fmt.Sprintf("Size_%d", size), func(t *testing.T) {
			// Create test iPRF
			key := GenerateDeterministicKey()
			iprf := NewIPRF(key, size, size/10) // range = size/10
			
			// Test multiple output values
			testOutputs := []uint64{0, size/4, size/2, size*3/4, size/10 - 1}
			
			fmt.Printf("\nTesting with database size: %d, range: %d\n", size, size/10)
			
			for _, output := range testOutputs {
				if output >= iprf.range_ {
					continue
				}
				
				// Test optimized version
				start := time.Now()
				optimizedResult := iprf.InverseFixed(output)
				optimizedTime := time.Since(start)
				
				// Test brute force version (for comparison)
				start = time.Now()
				bruteResult := iprf.bruteForceInverse(output)
				bruteTime := time.Since(start)
				
				// Verify results are identical
				if len(optimizedResult) != len(bruteResult) {
					t.Errorf("Result length mismatch for output %d: optimized=%d, brute=%d", 
						output, len(optimizedResult), len(bruteResult))
					continue
				}
				
				for i := range optimizedResult {
					if optimizedResult[i] != bruteResult[i] {
						t.Errorf("Result mismatch at index %d for output %d", i, output)
						continue
					}
				}
				
				// Report performance
				speedup := float64(bruteTime.Nanoseconds()) / float64(optimizedTime.Nanoseconds())
				fmt.Printf("  Output %d: Optimized=%v (%.2fns), Brute=%v (%.2fns), Speedup=%.2fx\n",
					output, len(optimizedResult), float64(optimizedTime.Nanoseconds()),
					len(bruteResult), float64(bruteTime.Nanoseconds()), speedup)
				
				// Verify significant speedup
				if speedup < 2.0 {
					t.Errorf("Insufficient speedup for output %d: %.2fx (expected at least 2x)", output, speedup)
				}
			}
		})
	}
}

// TestInverseCorrectness verifies that optimized inverse produces correct results
func TestInverseCorrectness(t *testing.T) {
	// Create test iPRF
	key := GenerateDeterministicKey()
	n := uint64(10000)
	m := uint64(1000)
	iprf := NewIPRF(key, n, m)
	
	// Test that inverse is correct for all possible outputs
	for y := uint64(0); y < m; y++ {
		// Get preimages using optimized method
		preimages := iprf.InverseFixed(y)
		
		// Verify each preimage maps to y
		for _, x := range preimages {
			if x >= n {
				t.Errorf("Invalid preimage x=%d for domain size %d", x, n)
				continue
			}
			
			forwardResult := iprf.Forward(x)
			if forwardResult != y {
				t.Errorf("Inverse correctness failed: Forward(%d) = %d, expected %d", x, forwardResult, y)
			}
		}
	}
	
	fmt.Printf("✅ Inverse correctness verified for all %d possible outputs\n", m)
}

// BenchmarkInverse compares performance of different inverse implementations
func BenchmarkInverse(b *testing.B) {
	sizes := []uint64{1000, 10000, 100000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			key := GenerateDeterministicKey()
			iprf := NewIPRF(key, size, size/10)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = iprf.InverseFixed(uint64(i) % iprf.range_)
			}
		})
	}
}

// BenchmarkBruteForce compares brute force performance (for reference)
func BenchmarkBruteForce(b *testing.B) {
	sizes := []uint64{1000, 10000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			key := GenerateDeterministicKey()
			iprf := NewIPRF(key, size, size/10)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = iprf.bruteForceInverse(uint64(i) % iprf.range_)
			}
		})
	}
}