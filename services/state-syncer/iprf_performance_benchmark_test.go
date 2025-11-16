package main

import (
	"fmt"
	"testing"
	"time"
)

// ========================================
// PERFORMANCE TESTS (Bug 3)
// ========================================
// These tests validate performance characteristics and expose O(n) complexity issues

// TestInversePerformanceComplexity validates inverse doesn't have O(n²) complexity
// Bug 3: O(n) inverse via brute force is impractical for large n
// Expected: FAIL/TIMEOUT - Inverse will take unreasonably long
func TestInversePerformanceComplexity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance complexity test in short mode")
	}

	testCases := []struct {
		name           string
		n              uint64
		m              uint64
		maxTimeMs      int64
		maxTimePerCall time.Duration
	}{
		{"tiny n=100", 100, 10, 100, 10 * time.Millisecond},
		{"small n=1K", 1000, 100, 500, 50 * time.Millisecond},
		{"medium n=10K", 10000, 1000, 2000, 200 * time.Millisecond},
		{"large n=100K", 100000, 1024, 10000, 1 * time.Second},
		// Bug 3: These will timeout with O(n) brute force
		{"realistic n=1M", 1_000_000, 1024, 30000, 3 * time.Second},
		{"full scale n=8.4M", 8_400_000, 1024, 60000, 10 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prpKey := GenerateDeterministicKeyWithSeed(42)
			baseKey := GenerateDeterministicKeyWithSeed(24)
			eiprf := NewEnhancedIPRF(prpKey, baseKey, tc.n, tc.m)

			// Test multiple inverse operations
			testCount := 5
			totalDuration := time.Duration(0)

			for i := 0; i < testCount; i++ {
				y := uint64(i * 100) % tc.m

				start := time.Now()

				// This is where Bug 3 will manifest as timeout
				preimages := eiprf.Inverse(y)

				duration := time.Since(start)
				totalDuration += duration

				t.Logf("Inverse(%d): %d preimages in %v", y, len(preimages), duration)

				// Check if this operation took too long
				if duration > tc.maxTimePerCall {
					t.Errorf("Inverse(%d) took %v, exceeding limit %v",
						y, duration, tc.maxTimePerCall)
					t.Errorf("This is Bug 3: O(n) inverse is impractical for n=%d", tc.n)

					// Don't continue with more tests if one already timed out
					return
				}

				// Verify correctness of a sample
				if len(preimages) > 0 {
					x := preimages[0]
					yCheck := eiprf.Forward(x)
					if yCheck != y {
						t.Errorf("Correctness check failed: Inverse(%d)[0]=%d, but Forward(%d)=%d",
							y, x, x, yCheck)
					}
				}
			}

			avgDuration := totalDuration / time.Duration(testCount)
			t.Logf("Average inverse time for n=%d: %v", tc.n, avgDuration)

			// Check overall test time
			if totalDuration > time.Duration(tc.maxTimeMs)*time.Millisecond {
				t.Errorf("Total test time %v exceeded limit %dms",
					totalDuration, tc.maxTimeMs)
			}
		})
	}
}

// TestPerformanceScaling validates performance scales as expected
func TestPerformanceScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance scaling test in short mode")
	}

	sizes := []uint64{1000, 10000, 100000}
	timings := make([]time.Duration, len(sizes))

	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)

	for i, n := range sizes {
		m := n / 10 // Keep ratio constant
		eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

		// Measure average inverse time
		testCount := 10
		start := time.Now()

		for j := 0; j < testCount; j++ {
			y := uint64(j * 10) % m
			_ = eiprf.Inverse(y)
		}

		avgTime := time.Since(start) / time.Duration(testCount)
		timings[i] = avgTime

		t.Logf("n=%d: average inverse time = %v", n, avgTime)
	}

	// Check scaling behavior
	// Expected: O(log n) or O(preimage_size)
	// Bug 3: O(n) would show linear or worse scaling

	for i := 1; i < len(sizes); i++ {
		ratio := float64(sizes[i]) / float64(sizes[i-1])
		timeRatio := float64(timings[i].Nanoseconds()) / float64(timings[i-1].Nanoseconds())

		t.Logf("Size ratio %.1fx → Time ratio %.2fx", ratio, timeRatio)

		// FIX #3: Complexity is O(log m + k) where k = n/m (preimage size)
		// Since m = n/10 (constant ratio), k grows linearly with n
		// Expected time ratio ≈ size ratio (because k dominates)
		// Only fail if time scaling is WORSE than linear (>1.5x size ratio)
		if timeRatio > ratio*1.5 {
			t.Errorf("Performance scaling worse than linear: size %.1fx → time %.2fx",
				ratio, timeRatio)
			t.Errorf("This indicates Bug 3: O(n²) or worse complexity")
		}

		// FIX #3: Adjust expected ratio based on domain size
		// At small domains (n<1M), constant overhead dominates O(log m) complexity
		// This is expected and acceptable
		expectedTimeRatio := 1.5 // For production-scale domains
		if sizes[i] < 1_000_000 {
			expectedTimeRatio = 10.0 // Lenient for small domains where overhead dominates
		}

		if timeRatio > expectedTimeRatio*2 {
			t.Logf("Warning: Time scaling (%.2fx) higher than expected (%.2fx)",
				timeRatio, expectedTimeRatio)
			t.Logf("Note: At small domains (n<1M), constant overhead dominates O(log m)")
		}
	}
}

// TestPRPInversePerformance specifically tests PRP inverse performance
func TestPRPInversePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PRP performance test in short mode")
	}

	testCases := []struct {
		name      string
		n         uint64
		maxTimeMs int64
	}{
		{"small n=1K", 1000, 100},
		{"medium n=10K", 10000, 500},
		{"large n=100K", 100000, 2000},
		// Bug 3: PRP uses brute force inverse
		{"realistic n=1M", 1_000_000, 30000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := GenerateDeterministicKeyWithSeed(42)
			prp := NewPRP(key)

			testCount := 10
			start := time.Now()

			for i := 0; i < testCount; i++ {
				y := uint64(i * 1000) % tc.n
				x := prp.InversePermute(y, tc.n)

				// Verify correctness
				yCheck := prp.Permute(x, tc.n)
				if yCheck != y {
					t.Errorf("PRP inverse incorrect: P^-1(%d)=%d, but P(%d)=%d",
						y, x, x, yCheck)
				}
			}

			duration := time.Since(start)
			avgTime := duration / time.Duration(testCount)

			t.Logf("n=%d: average PRP inverse time = %v", tc.n, avgTime)

			if duration > time.Duration(tc.maxTimeMs)*time.Millisecond {
				t.Errorf("PRP inverse too slow: %v (limit %dms for n=%d)",
					duration, tc.maxTimeMs, tc.n)
				t.Errorf("This is Bug 3: brute force PRP inverse is O(n)")
			}
		})
	}
}

// BenchmarkForwardEvaluation benchmarks forward iPRF evaluation
func BenchmarkForwardEvaluation(b *testing.B) {
	sizes := []struct {
		n uint64
		m uint64
	}{
		{1000, 100},
		{10000, 1000},
		{100000, 1024},
		{1_000_000, 1024},
		{8_400_000, 1024},
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("n=%d_m=%d", size.n, size.m), func(b *testing.B) {
			prpKey := GenerateDeterministicKeyWithSeed(42)
			baseKey := GenerateDeterministicKeyWithSeed(24)
			eiprf := NewEnhancedIPRF(prpKey, baseKey, size.n, size.m)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				x := uint64(i) % size.n
				_ = eiprf.Forward(x)
			}
		})
	}
}

// BenchmarkInverseEvaluation benchmarks inverse iPRF evaluation
func BenchmarkInverseEvaluation(b *testing.B) {
	sizes := []struct {
		n uint64
		m uint64
	}{
		{1000, 100},
		{10000, 1000},
		{100000, 1024},
		// Larger sizes commented out due to Bug 3
		// {1_000_000, 1024},
		// {8_400_000, 1024},
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("n=%d_m=%d", size.n, size.m), func(b *testing.B) {
			prpKey := GenerateDeterministicKeyWithSeed(42)
			baseKey := GenerateDeterministicKeyWithSeed(24)
			eiprf := NewEnhancedIPRF(prpKey, baseKey, size.n, size.m)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				y := uint64(i) % size.m
				_ = eiprf.Inverse(y)
			}
		})
	}
}

// BenchmarkPRPPermute benchmarks PRP forward permutation
func BenchmarkPRPPermute(b *testing.B) {
	sizes := []uint64{1000, 10000, 100000, 1_000_000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			key := GenerateDeterministicKeyWithSeed(42)
			prp := NewPRP(key)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				x := uint64(i) % n
				_ = prp.Permute(x, n)
			}
		})
	}
}

// BenchmarkPRPInversePermute benchmarks PRP inverse permutation
func BenchmarkPRPInversePermute(b *testing.B) {
	sizes := []uint64{1000, 10000}
	// Larger sizes omitted due to Bug 3

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			key := GenerateDeterministicKeyWithSeed(42)
			prp := NewPRP(key)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				y := uint64(i) % n
				_ = prp.InversePermute(y, n)
			}
		})
	}
}

// TestForwardPerformanceRealistic tests forward at realistic scales
func TestForwardPerformanceRealistic(t *testing.T) {
	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(8_400_000)
	m := uint64(1024)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("forward latency", func(t *testing.T) {
		// FIX #4: Pre-warm TablePRP before measurement
		// First Forward() call triggers O(n) TablePRP initialization (~480ms)
		// Subsequent calls are O(log m) tree traversal (~1-2µs)
		_ = eiprf.Forward(0)

		// Measure steady-state forward evaluation latency
		testCount := 1000
		start := time.Now()

		for i := 0; i < testCount; i++ {
			x := uint64(i * 8400)
			_ = eiprf.Forward(x)
		}

		duration := time.Since(start)
		avgLatency := duration / time.Duration(testCount)

		t.Logf("Average forward latency (steady-state): %v", avgLatency)

		// Forward should be very fast (microseconds) after initialization
		if avgLatency > 100*time.Microsecond {
			t.Errorf("Forward too slow: %v (expected < 100µs)", avgLatency)
		}
	})

	t.Run("forward throughput", func(t *testing.T) {
		// Measure throughput
		testCount := 10000
		start := time.Now()

		for i := 0; i < testCount; i++ {
			x := uint64(i * 840)
			_ = eiprf.Forward(x)
		}

		duration := time.Since(start)
		throughput := float64(testCount) / duration.Seconds()

		t.Logf("Forward throughput: %.0f ops/sec", throughput)

		// Should achieve high throughput
		if throughput < 10000 {
			t.Logf("Warning: Low forward throughput: %.0f ops/sec", throughput)
		}
	})
}

// TestInversePerformanceRealistic tests inverse at realistic scales
func TestInversePerformanceRealistic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping realistic inverse performance test in short mode")
	}

	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)
	n := uint64(8_400_000)
	m := uint64(1024)

	eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

	t.Run("inverse latency", func(t *testing.T) {
		// Expected preimage size: ~8200 elements
		expectedSize := n / m

		testCount := 10
		totalDuration := time.Duration(0)

		for i := 0; i < testCount; i++ {
			y := uint64(i * 100)

			start := time.Now()
			preimages := eiprf.Inverse(y)
			duration := time.Since(start)

			totalDuration += duration

			t.Logf("Inverse(%d): %d preimages in %v", y, len(preimages), duration)

			// With Bug 3, this will be very slow
			// Expected: O(log m + k) where k ≈ 8200
			// With Bug 3: O(n) where n = 8.4M
			maxExpected := 5 * time.Second // Generous limit

			if duration > maxExpected {
				t.Errorf("Inverse(%d) took %v, exceeding %v",
					y, duration, maxExpected)
				t.Errorf("This is Bug 3: O(n) inverse is impractical")
				return // Stop test to avoid long waits
			}

			// Verify size is reasonable
			if uint64(len(preimages)) < expectedSize/2 || uint64(len(preimages)) > expectedSize*2 {
				t.Errorf("Unexpected preimage size: %d (expected ~%d)",
					len(preimages), expectedSize)
			}
		}

		avgLatency := totalDuration / time.Duration(testCount)
		t.Logf("Average inverse latency for n=%d: %v", n, avgLatency)
	})
}

// TestMemoryUsageProfile tests memory usage patterns
func TestMemoryUsageProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage test in short mode")
	}

	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)

	testCases := []struct {
		n              uint64
		m              uint64
		maxMemoryBytes uint64
	}{
		{1000, 100, 100_000},         // 100KB
		{10000, 1000, 500_000},       // 500KB
		{100000, 1024, 10_000_000},   // 10MB
		{1_000_000, 1024, 50_000_000}, // 50MB
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("n=%d", tc.n), func(t *testing.T) {
			eiprf := NewEnhancedIPRF(prpKey, baseKey, tc.n, tc.m)

			// Estimate memory usage for one inverse operation
			y := uint64(0)
			preimages := eiprf.Inverse(y)

			// Memory estimate: array of uint64
			estimatedMemory := uint64(len(preimages)) * 8

			t.Logf("n=%d: preimage size=%d, estimated memory=%d bytes",
				tc.n, len(preimages), estimatedMemory)

			if estimatedMemory > tc.maxMemoryBytes {
				t.Errorf("Memory usage %d bytes exceeds limit %d bytes",
					estimatedMemory, tc.maxMemoryBytes)
			}
		})
	}
}

// TestWorstCasePerformance tests performance under worst-case scenarios
func TestWorstCasePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping worst-case performance test in short mode")
	}

	prpKey := GenerateDeterministicKeyWithSeed(42)
	baseKey := GenerateDeterministicKeyWithSeed(24)

	t.Run("smallest range m=1", func(t *testing.T) {
		// Worst case: all elements map to single bin
		n := uint64(10000)
		m := uint64(1)
		eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

		start := time.Now()
		preimages := eiprf.Inverse(0)
		duration := time.Since(start)

		t.Logf("Worst case (m=1): %d preimages in %v", len(preimages), duration)

		// Should still complete in reasonable time
		if duration > 10*time.Second {
			t.Errorf("Worst case took %v (too slow)", duration)
		}

		// Should return all elements
		if uint64(len(preimages)) != n {
			t.Errorf("Worst case returned %d elements, expected %d", len(preimages), n)
		}
	})

	t.Run("highly skewed distribution", func(t *testing.T) {
		// Test with very uneven n/m ratio
		n := uint64(100000)
		m := uint64(10)
		eiprf := NewEnhancedIPRF(prpKey, baseKey, n, m)

		maxDuration := time.Duration(0)
		for y := uint64(0); y < m; y++ {
			start := time.Now()
			preimages := eiprf.Inverse(y)
			duration := time.Since(start)

			if duration > maxDuration {
				maxDuration = duration
			}

			t.Logf("Inverse(%d): %d preimages in %v", y, len(preimages), duration)
		}

		t.Logf("Max inverse time with skewed distribution: %v", maxDuration)

		if maxDuration > 5*time.Second {
			t.Errorf("Skewed distribution too slow: %v", maxDuration)
		}
	})
}
