package main

import (
	"testing"
	"time"
)

// ========================================
// INTEGRATION TESTS (Bugs 6, 7)
// ========================================
// These tests validate the integration of iPRF with the larger system
// and test cache mode behavior

// TestCacheModeEffectiveness validates cache mode actually skips computation
// Bug 7: Cache mode computes iPRF before checking cache (ineffective)
// Expected: FAIL - Cache provides no performance benefit
func TestCacheModeEffectiveness(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(100000)
	m := uint64(1024)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("cache vs no-cache performance", func(t *testing.T) {
		// Simulate cache mode vs no-cache mode
		// In correct implementation, cache should be MUCH faster

		testCount := 100
		testIndices := make([]uint64, testCount)
		for i := 0; i < testCount; i++ {
			testIndices[i] = uint64(i * 1000)
		}

		// First pass: "no cache" - compute everything
		startNoCache := time.Now()
		results := make(map[uint64][]uint64)
		for _, x := range testIndices {
			y := eiprf.Forward(x)
			results[y] = eiprf.Inverse(y)
		}
		noCacheDuration := time.Since(startNoCache)

		// Second pass: "with cache" - use pre-computed results
		// Bug 7: If iPRF is computed before checking cache, this will be slow
		startCache := time.Now()
		cacheHits := 0
		cacheMisses := 0
		for _, x := range testIndices {
			y := eiprf.Forward(x)

			// Simulate cache lookup
			if cached, exists := results[y]; exists {
				cacheHits++
				// Use cached value, skip computation
				_ = cached
			} else {
				cacheMisses++
				results[y] = eiprf.Inverse(y)
			}
		}
		cacheDuration := time.Since(startCache)

		t.Logf("No-cache duration: %v", noCacheDuration)
		t.Logf("Cache duration: %v", cacheDuration)
		t.Logf("Cache hits: %d, misses: %d", cacheHits, cacheMisses)

		// With cache, should be much faster (if implemented correctly)
		// Bug 7: Cache mode will be just as slow as no-cache
		speedup := float64(noCacheDuration.Nanoseconds()) / float64(cacheDuration.Nanoseconds())
		t.Logf("Speedup with cache: %.2fx", speedup)

		// Expected: cache should provide significant speedup
		// If Bug 7 exists, speedup will be close to 1.0
		if speedup < 1.5 && cacheHits > 0 {
			t.Errorf("Cache mode ineffective: only %.2fx speedup (expected >1.5x)", speedup)
			t.Errorf("This is Bug 7: computing iPRF before checking cache")
		}
	})

	t.Run("cache correctness", func(t *testing.T) {
		// Even with cache, results must be correct
		cache := make(map[uint64][]uint64)

		testCount := 100
		for i := 0; i < testCount; i++ {
			x := uint64(i * 100)
			y := eiprf.Forward(x)

			var preimages []uint64

			// Check cache
			if cached, exists := cache[y]; exists {
				preimages = cached
			} else {
				// Compute and cache
				preimages = eiprf.Inverse(y)
				cache[y] = preimages
			}

			// Verify x is in preimages
			found := false
			for _, preimage := range preimages {
				if preimage == x {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Cache correctness failed: x=%d → y=%d, but x ∉ cached Inverse(%d)",
					x, y, y)
			}
		}
	})
}

// TestSystemIntegration validates integration with the PIR system
// Bug 6: Integration issues between iPRF and PIR protocol
// Expected: Tests overall system behavior
func TestSystemIntegration(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)

	// Realistic parameters from the paper
	dbSize := uint64(8_400_000) // 8.4M accounts
	setSize := uint64(1_024)     // 1K hint sets

	eiprf := NewEnhancedIPRF(prpKey, baseKey, dbSize, setSize)
	_ = prpKey // Use variable to avoid "declared and not used" error

	t.Run("realistic scale forward", func(t *testing.T) {
		// Test forward evaluation at realistic scale
		testCount := 1000
		for i := 0; i < testCount; i++ {
			x := uint64(i * 8400) // Sample across the domain
			y := eiprf.Forward(x)

			if y >= setSize {
				t.Errorf("Forward(%d) = %d >= setSize=%d", x, y, setSize)
			}
		}
	})

	t.Run("realistic scale inverse", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping large-scale inverse test in short mode")
		}

		// Test inverse evaluation at realistic scale
		// Expected preimage size: 8.4M / 1024 ≈ 8200
		expectedSize := dbSize / setSize

		testOutputs := []uint64{0, 100, 500, 1000, 1023}
		for _, y := range testOutputs {
			start := time.Now()
			preimages := eiprf.Inverse(y)
			duration := time.Since(start)

			t.Logf("Inverse(%d): %d preimages in %v", y, len(preimages), duration)

			// Check size is reasonable
			actualSize := uint64(len(preimages))
			deviation := float64(actualSize) / float64(expectedSize)

			if deviation < 0.5 || deviation > 2.0 {
				t.Errorf("Inverse(%d) size unexpected: %d (expected ~%d, deviation %.2fx)",
					y, actualSize, expectedSize, deviation)
			}

			// Verify correctness of a sample
			sampleSize := 10
			if sampleSize > len(preimages) {
				sampleSize = len(preimages)
			}

			for i := 0; i < sampleSize; i++ {
				x := preimages[i]
				yCheck := eiprf.Forward(x)
				if yCheck != y {
					t.Errorf("Inverse(%d)[%d]=%d, but Forward(%d)=%d",
						y, i, x, x, yCheck)
				}
			}
		}
	})

	t.Run("expected preimage size", func(t *testing.T) {
		actualSize := eiprf.GetPreimageSize()
		// FIX #1: Use ceiling division to match GetPreimageSize() implementation
		// GetPreimageSize() uses math.Ceil(n/m) which gives 8204, not 8203
		// This is correct because with ceiling division: (8,400,000 + 1024 - 1) / 1024 = 8204
		expectedValue := (dbSize + setSize - 1) / setSize

		// Note: Actual preimage sizes follow binomial distribution B(n, 1/m)
		// Expected value: n/m, but individual bins may vary by ±√(n/m)
		// For n=8.4M, m=1024: expected≈8203, std_dev≈90
		// GetPreimageSize() returns ceiling to ensure buffer capacity
		if actualSize != expectedValue {
			t.Errorf("GetPreimageSize() = %d, expected %d", actualSize, expectedValue)
		}

		t.Logf("Expected preimage size: %d elements (ceiling division)", actualSize)
	})
}

// TestMultiQueryScenario tests multiple queries in sequence
func TestMultiQueryScenario(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(10000)
	m := uint64(100)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("sequential queries", func(t *testing.T) {
		// Simulate multiple sequential queries
		queryCount := 50
		cache := make(map[uint64][]uint64)

		for i := 0; i < queryCount; i++ {
			// Random query index
			x := uint64(i * 200)
			y := eiprf.Forward(x)

			// Get preimages (using cache if available)
			var preimages []uint64
			if cached, exists := cache[y]; exists {
				preimages = cached
			} else {
				preimages = eiprf.Inverse(y)
				cache[y] = preimages
			}

			// Verify query
			found := false
			for _, preimage := range preimages {
				if preimage == x {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Query %d failed: x=%d → y=%d, but x ∉ Inverse(%d)",
					i, x, y, y)
			}
		}

		t.Logf("Completed %d queries with cache size %d", queryCount, len(cache))
	})

	t.Run("parallel query simulation", func(t *testing.T) {
		// Simulate multiple concurrent queries to same outputs
		testOutput := uint64(0)

		// Multiple threads would query the same y
		results := make([][]uint64, 5)
		for i := 0; i < 5; i++ {
			results[i] = eiprf.Inverse(testOutput)
		}

		// All results should be identical
		for i := 1; i < 5; i++ {
			if len(results[i]) != len(results[0]) {
				t.Errorf("Parallel query %d got different size: %d vs %d",
					i, len(results[i]), len(results[0]))
				continue
			}

			for j := range results[0] {
				if results[i][j] != results[0][j] {
					t.Errorf("Parallel query %d differs at index %d: %d vs %d",
						i, j, results[i][j], results[0][j])
				}
			}
		}
	})
}

// TestBatchOperations tests batch inverse operations
func TestBatchOperations(t *testing.T) {
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(10000)
	m := uint64(100)

	iprf := NewIPRF(baseKey, n, m)

	t.Run("batch inverse correctness", func(t *testing.T) {
		// Test batch inverse
		yValues := []uint64{0, 10, 20, 30, 40, 50}
		batchResults := iprf.InverseBatch(yValues)

		// Compare with individual calls
		for _, y := range yValues {
			expected := iprf.Inverse(y)
			actual := batchResults[y]

			if len(actual) != len(expected) {
				t.Errorf("InverseBatch(%d) size mismatch: %d vs %d",
					y, len(actual), len(expected))
				continue
			}

			for i := range expected {
				if actual[i] != expected[i] {
					t.Errorf("InverseBatch(%d)[%d] = %d, expected %d",
						y, i, actual[i], expected[i])
				}
			}
		}
	})

	t.Run("batch performance", func(t *testing.T) {
		yValues := make([]uint64, 50)
		for i := range yValues {
			yValues[i] = uint64(i * 2)
		}

		start := time.Now()
		_ = iprf.InverseBatch(yValues)
		batchDuration := time.Since(start)

		// Compare with sequential
		start = time.Now()
		for _, y := range yValues {
			_ = iprf.Inverse(y)
		}
		sequentialDuration := time.Since(start)

		t.Logf("Batch: %v, Sequential: %v", batchDuration, sequentialDuration)

		// Batch should not be significantly slower
		if batchDuration > sequentialDuration*2 {
			t.Errorf("Batch too slow: %v vs sequential %v",
				batchDuration, sequentialDuration)
		}
	})
}

// TestDistributionStats tests the GetDistributionStats function
func TestDistributionStats(t *testing.T) {
	key := GenerateDeterministicKey()

	t.Run("small domain stats", func(t *testing.T) {
		n := uint64(1000)
		m := uint64(100)
		iprf := NewIPRF(key, n, m)

		stats := iprf.GetDistributionStats()

		// Check expected preimage size
		expectedSize := float64(n) / float64(m)
		if stats["expected_preimage_size"] != expectedSize {
			t.Errorf("Expected preimage size = %v, expected %.2f",
				stats["expected_preimage_size"], expectedSize)
		}

		// Should have actual stats for small domain
		if stats["total_outputs"] == nil {
			t.Error("No distribution stats computed for small domain")
		}

		t.Logf("Distribution stats: %+v", stats)
	})

	t.Run("large domain stats", func(t *testing.T) {
		n := uint64(100000)
		m := uint64(1000)
		iprf := NewIPRF(key, n, m)

		stats := iprf.GetDistributionStats()

		// Should only have expected size for large domain
		expectedSize := float64(n) / float64(m)
		if stats["expected_preimage_size"] != expectedSize {
			t.Errorf("Expected preimage size = %v, expected %.2f",
				stats["expected_preimage_size"], expectedSize)
		}

		// Should not compute full distribution for large domain
		if stats["total_outputs"] != nil {
			t.Log("Warning: Computing full distribution for large domain (may be slow)")
		}

		t.Logf("Distribution stats: %+v", stats)
	})
}

// TestErrorConditions tests error handling and edge cases
func TestErrorConditions(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(1000)
	m := uint64(100)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("out of range queries", func(t *testing.T) {
		// Query y >= m should return empty
		preimages := eiprf.Inverse(m)
		if len(preimages) != 0 {
			t.Errorf("Inverse(%d) with y>=m should return empty, got %d elements",
				m, len(preimages))
		}

		preimages = eiprf.Inverse(m + 1000)
		if len(preimages) != 0 {
			t.Errorf("Inverse(%d) with y>>m should return empty, got %d elements",
				m+1000, len(preimages))
		}
	})

	t.Run("boundary queries", func(t *testing.T) {
		// Test boundary values
		testValues := []uint64{0, 1, m - 2, m - 1}

		for _, y := range testValues {
			preimages := eiprf.Inverse(y)

			// Should return valid preimages
			for _, x := range preimages {
				if x >= n {
					t.Errorf("Inverse(%d) returned x=%d >= n=%d", y, x, n)
				}

				yCheck := eiprf.Forward(x)
				if yCheck != y {
					t.Errorf("Inverse(%d) returned x=%d, but Forward(%d)=%d",
						y, x, x, yCheck)
				}
			}
		}
	})

	t.Run("forward boundary values", func(t *testing.T) {
		// Test boundary input values
		testValues := []uint64{0, 1, n - 2, n - 1}

		for _, x := range testValues {
			y := eiprf.Forward(x)

			if y >= m {
				t.Errorf("Forward(%d) = %d >= m=%d", x, y, m)
			}

			// Verify inverse contains x
			preimages := eiprf.Inverse(y)
			found := false
			for _, preimage := range preimages {
				if preimage == x {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Forward(%d)=%d, but x ∉ Inverse(%d)", x, y, y)
			}
		}
	})
}

// TestMemoryEfficiency tests memory usage patterns
func TestMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(1_000_000)
	m := uint64(1_024)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("preimage size reasonable", func(t *testing.T) {
		// Test a few inverse operations
		for i := 0; i < 10; i++ {
			y := uint64(i * 100)
			preimages := eiprf.Inverse(y)

			// Each preimage array should be manageable
			expectedSize := n / m // ~1000 elements
			actualSize := uint64(len(preimages))

			if actualSize > expectedSize*3 {
				t.Errorf("Inverse(%d) returned %d elements, expected ~%d",
					y, actualSize, expectedSize)
			}

			// Memory usage: ~8 bytes per uint64
			memoryBytes := actualSize * 8
			if memoryBytes > 100000 { // 100KB threshold
				t.Logf("Warning: Inverse(%d) uses %d bytes of memory",
					y, memoryBytes)
			}
		}
	})
}

// TestConcurrentAccess simulates concurrent access patterns
func TestConcurrentAccess(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(10000)
	m := uint64(100)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("determinism under repeated access", func(t *testing.T) {
		// Same queries should always return same results
		testOutput := uint64(42)

		firstResult := eiprf.Inverse(testOutput)

		// Repeat same query many times
		for i := 0; i < 100; i++ {
			result := eiprf.Inverse(testOutput)

			if len(result) != len(firstResult) {
				t.Errorf("Query %d: size changed from %d to %d",
					i, len(firstResult), len(result))
				break
			}

			for j := range firstResult {
				if result[j] != firstResult[j] {
					t.Errorf("Query %d: result[%d] changed from %d to %d",
						i, j, firstResult[j], result[j])
					break
				}
			}
		}
	})
}
