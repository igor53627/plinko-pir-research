// +build ignore

package main

import (
	"fmt"
	"math"
)

func main() {
	// Create iPRF with production parameters
	key := GenerateDeterministicKey()
	n := uint64(8400000)
	m := uint64(1024)
	
	iprf := NewIPRF(key, n, m)
	
	// Test distribution across key bins
	testBins := []uint64{0, 100, 500, 1000, 1023}
	
	fmt.Println("Distribution Analysis After Bug 4 Fix")
	fmt.Println("=====================================")
	fmt.Printf("Domain size: %d\n", n)
	fmt.Printf("Range size: %d\n", m)
	fmt.Printf("Expected per bin: %.0f\n\n", float64(n)/float64(m))
	
	totalDeviation := 0.0
	expected := float64(n) / float64(m)
	
	for _, bin := range testBins {
		// Count preimages
		count := 0
		for x := uint64(0); x < n; x++ {
			if iprf.Forward(x) == bin {
				count++
			}
			if count > 10000 {
				// Early termination for efficiency
				break
			}
		}
		
		deviation := math.Abs(float64(count) - expected)
		deviationPct := (deviation / expected) * 100
		totalDeviation += deviationPct
		
		fmt.Printf("Bin %4d: %d preimages (deviation: %.1f%%)\n", bin, count, deviationPct)
	}
	
	avgDeviation := totalDeviation / float64(len(testBins))
	fmt.Printf("\nAverage deviation: %.1f%%\n", avgDeviation)
	
	if avgDeviation < 10 {
		fmt.Println("✅ Distribution is uniform (deviation < 10%)")
	} else {
		fmt.Println("❌ Distribution is skewed (deviation >= 10%)")
	}
}
