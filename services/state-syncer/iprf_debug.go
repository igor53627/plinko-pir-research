package main

import (
	"fmt"
	"sort"
)

// DebugInverse provides detailed debugging for the inverse function
func (iprf *IPRF) DebugInverse(y uint64) {
	fmt.Printf("\n🔍 DEBUG: Inverse(%d)\n", y)
	fmt.Printf("   Domain: %d, Range: %d\n", iprf.domain, iprf.range_)
	
	if y >= iprf.range_ {
		fmt.Printf("   ERROR: y=%d >= range=%d\n", y, iprf.range_)
		return
	}
	
	// Build complete forward mapping for comparison
	fmt.Println("   Building complete forward mapping...")
	forwardMap := make(map[uint64][]uint64)
	for x := uint64(0); x < iprf.domain; x++ {
		result := iprf.Forward(x)
		forwardMap[result] = append(forwardMap[result], x)
		if x < 10 || x%1000 == 0 {
			fmt.Printf("   Forward(%d) = %d\n", x, result)
		}
	}
	
	// Expected result
	expectedPreimages := forwardMap[y]
	fmt.Printf("   Expected preimages for y=%d: %d indices\n", y, len(expectedPreimages))
	if len(expectedPreimages) <= 10 {
		fmt.Printf("   Expected: %v\n", expectedPreimages)
	}
	
	// Actual result
	actualPreimages := iprf.InverseFixed(y)
	fmt.Printf("   Actual preimages for y=%d: %d indices\n", y, len(actualPreimages))
	if len(actualPreimages) <= 10 {
		fmt.Printf("   Actual: %v\n", actualPreimages)
	}
	
	// Compare results
	if len(expectedPreimages) != len(actualPreimages) {
		fmt.Printf("   ❌ LENGTH MISMATCH: expected %d, got %d\n", 
			len(expectedPreimages), len(actualPreimages))
	}
	
	// Check if all expected are in actual
	missing := []uint64{}
	for _, expected := range expectedPreimages {
		found := false
		for _, actual := range actualPreimages {
			if expected == actual {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, expected)
		}
	}
	
	if len(missing) > 0 {
		fmt.Printf("   ❌ MISSING PREIMAGES: %v\n", missing)
	}
	
	// Check if any actual are not in expected
	extra := []uint64{}
	for _, actual := range actualPreimages {
		found := false
		for _, expected := range expectedPreimages {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			extra = append(extra, actual)
		}
	}
	
	if len(extra) > 0 {
		fmt.Printf("   ❌ EXTRA PREIMAGES: %v\n", extra)
	}
	
	if len(missing) == 0 && len(extra) == 0 {
		fmt.Printf("   ✅ PERFECT MATCH!\n")
	}
}

// SimpleInverse implements a brute-force inverse for validation
func (iprf *IPRF) SimpleInverse(y uint64) []uint64 {
	var result []uint64
	for x := uint64(0); x < iprf.domain; x++ {
		if iprf.Forward(x) == y {
			result = append(result, x)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

// ValidateInverseImplementation compares our inverse with brute-force approach
func (iprf *IPRF) ValidateInverseImplementation() bool {
	fmt.Println("\n🔍 Validating Inverse Implementation")
	fmt.Println("=" + fmt.Sprintf("%50s", "="))
	
	// Test a few values
	testValues := []uint64{0, 1, 2, 10, 50, 99}
	allPassed := true
	
	for _, y := range testValues {
		if y >= iprf.range_ {
			continue
		}
		
		fmt.Printf("\nTesting Inverse(%d):\n", y)
		
		// Get results from both methods
		simpleResult := iprf.SimpleInverse(y)
		optimizedResult := iprf.InverseFixed(y)
		
		fmt.Printf("   Simple method: %d preimages\n", len(simpleResult))
		fmt.Printf("   Optimized method: %d preimages\n", len(optimizedResult))
		
		// Compare results
		if len(simpleResult) != len(optimizedResult) {
			fmt.Printf("   ❌ Length mismatch: %d vs %d\n", len(simpleResult), len(optimizedResult))
			allPassed = false
			continue
		}
		
		// Check content
		match := true
		for i := range simpleResult {
			if simpleResult[i] != optimizedResult[i] {
				fmt.Printf("   ❌ Content mismatch at position %d: %d vs %d\n", 
					i, simpleResult[i], optimizedResult[i])
				match = false
				allPassed = false
				break
			}
		}
		
		if match {
			fmt.Printf("   ✅ Perfect match!\n")
		}
	}
	
	fmt.Printf("\n🔍 Validation Result: ")
	if allPassed {
		fmt.Println("✅ ALL TESTS PASSED")
	} else {
		fmt.Println("❌ SOME TESTS FAILED")
	}
	
	fmt.Println("=" + fmt.Sprintf("%50s", "="))
	return allPassed
}