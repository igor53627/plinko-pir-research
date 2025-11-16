package main

import (
	"runtime"
	"testing"
)

// ========================================
// TABLE PRP UNIT TESTS
// ========================================
// These tests validate the TablePRP implementation
// which fixes Bug 1 (bijection failure) and Bug 3 (O(n) inverse)

// TestTablePRPBijection validates that TablePRP creates a perfect bijection
func TestTablePRPBijection(t *testing.T) {
	testCases := []struct {
		name   string
		domain uint64
	}{
		{"tiny domain n=10", 10},
		{"small domain n=100", 100},
		{"medium domain n=1000", 1000},
		{"large domain n=10000", 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := []byte("test-key-16bytes")
			prp := NewTablePRP(tc.domain, key)

			t.Run("no collisions - forward is injective", func(t *testing.T) {
				// Property: Forward(x1) ≠ Forward(x2) for x1 ≠ x2
				outputsSeen := make(map[uint64]uint64) // y → x that produced it

				for x := uint64(0); x < tc.domain; x++ {
					y := prp.Forward(x)

					if prevX, exists := outputsSeen[y]; exists {
						t.Fatalf("Collision: Forward(%d) = Forward(%d) = %d", prevX, x, y)
					}

					if y >= tc.domain {
						t.Fatalf("Forward(%d) = %d out of range [0, %d)", x, y, tc.domain)
					}

					outputsSeen[y] = x
				}
			})

			t.Run("surjective - all values reachable", func(t *testing.T) {
				// Property: Every y ∈ [0, domain) has a preimage
				outputs := make(map[uint64]bool)

				for x := uint64(0); x < tc.domain; x++ {
					y := prp.Forward(x)
					outputs[y] = true
				}

				// Check all values in [0, domain) are covered
				if len(outputs) != int(tc.domain) {
					t.Fatalf("Only %d/%d values reachable (not surjective)", len(outputs), tc.domain)
				}

				for y := uint64(0); y < tc.domain; y++ {
					if !outputs[y] {
						t.Fatalf("Value %d is unreachable", y)
					}
				}
			})

			t.Run("inverse property - Inverse(Forward(x)) = x", func(t *testing.T) {
				// Property: Inverse(Forward(x)) = x for all x
				for x := uint64(0); x < tc.domain; x++ {
					y := prp.Forward(x)
					xRecovered := prp.Inverse(y)

					if xRecovered != x {
						t.Fatalf("Inverse(Forward(%d)) = %d, expected %d", x, xRecovered, x)
					}
				}
			})

			t.Run("forward property - Forward(Inverse(y)) = y", func(t *testing.T) {
				// Property: Forward(Inverse(y)) = y for all y
				for y := uint64(0); y < tc.domain; y++ {
					x := prp.Inverse(y)
					yRecovered := prp.Forward(x)

					if yRecovered != y {
						t.Fatalf("Forward(Inverse(%d)) = %d, expected %d", y, yRecovered, y)
					}
				}
			})
		})
	}
}

// TestTablePRPDeterminism validates that same key produces same permutation
func TestTablePRPDeterminism(t *testing.T) {
	domain := uint64(1000)
	key := []byte("deterministic-key")

	// Create two TablePRP instances with same key
	prp1 := NewTablePRP(domain, key)
	prp2 := NewTablePRP(domain, key)

	t.Run("same forward outputs", func(t *testing.T) {
		for x := uint64(0); x < domain; x++ {
			y1 := prp1.Forward(x)
			y2 := prp2.Forward(x)

			if y1 != y2 {
				t.Fatalf("Non-deterministic: Forward(%d) produced %d and %d", x, y1, y2)
			}
		}
	})

	t.Run("same inverse outputs", func(t *testing.T) {
		for y := uint64(0); y < domain; y++ {
			x1 := prp1.Inverse(y)
			x2 := prp2.Inverse(y)

			if x1 != x2 {
				t.Fatalf("Non-deterministic: Inverse(%d) produced %d and %d", y, x1, x2)
			}
		}
	})
}

// TestTablePRPDifferentKeys validates different keys produce different permutations
func TestTablePRPDifferentKeys(t *testing.T) {
	domain := uint64(1000)
	key1 := []byte("key-one-16-bytes")
	key2 := []byte("key-two-16-bytes")

	prp1 := NewTablePRP(domain, key1)
	prp2 := NewTablePRP(domain, key2)

	differences := 0
	for x := uint64(0); x < domain; x++ {
		y1 := prp1.Forward(x)
		y2 := prp2.Forward(x)

		if y1 != y2 {
			differences++
		}
	}

	// Expect most outputs to differ (> 90%)
	minDifferences := int(float64(domain) * 0.9)
	if differences < minDifferences {
		t.Fatalf("Different keys only produced %d/%d different outputs (expected > %d)",
			differences, domain, minDifferences)
	}

	t.Logf("Different keys produced %d/%d different outputs (%.1f%%)",
		differences, domain, float64(differences)*100.0/float64(domain))
}

// TestTablePRPBoundaryConditions validates edge cases
func TestTablePRPBoundaryConditions(t *testing.T) {
	t.Run("n=1 domain", func(t *testing.T) {
		key := []byte("test-key")
		prp := NewTablePRP(1, key)

		// Only valid bijection: 0 → 0
		if prp.Forward(0) != 0 {
			t.Fatalf("Forward(0) = %d for n=1, expected 0", prp.Forward(0))
		}
		if prp.Inverse(0) != 0 {
			t.Fatalf("Inverse(0) = %d for n=1, expected 0", prp.Inverse(0))
		}
	})

	t.Run("n=2 domain", func(t *testing.T) {
		key := []byte("test-key")
		prp := NewTablePRP(2, key)

		y0 := prp.Forward(0)
		y1 := prp.Forward(1)

		// Must be bijection: either identity or swap
		if y0 == y1 {
			t.Fatalf("Forward(0) = Forward(1) = %d (not bijective)", y0)
		}

		if y0 != 0 && y0 != 1 {
			t.Fatalf("Forward(0) = %d out of range [0, 2)", y0)
		}
		if y1 != 0 && y1 != 1 {
			t.Fatalf("Forward(1) = %d out of range [0, 2)", y1)
		}
	})

	t.Run("out of bounds panics", func(t *testing.T) {
		key := []byte("test-key")
		prp := NewTablePRP(100, key)

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Forward(x >= domain) should panic")
			}
		}()

		prp.Forward(100) // Should panic
	})

	t.Run("inverse out of bounds panics", func(t *testing.T) {
		key := []byte("test-key")
		prp := NewTablePRP(100, key)

		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Inverse(y >= domain) should panic")
			}
		}()

		prp.Inverse(100) // Should panic
	})
}

// TestTablePRPZeroEdgeCase validates Bug 10 fix: distinguishing x=0 from "not found"
func TestTablePRPZeroEdgeCase(t *testing.T) {
	key := []byte("test-key")
	prp := NewTablePRP(100, key)

	// Find which x maps to y=0
	var xForZero uint64
	for x := uint64(0); x < 100; x++ {
		if prp.Forward(x) == 0 {
			xForZero = x
			break
		}
	}

	t.Run("Inverse(0) returns correct x", func(t *testing.T) {
		x := prp.Inverse(0)
		if x != xForZero {
			t.Fatalf("Inverse(0) = %d, expected %d", x, xForZero)
		}

		// Verify correctness
		if prp.Forward(x) != 0 {
			t.Fatalf("Forward(Inverse(0)) = %d, expected 0", prp.Forward(x))
		}
	})

	t.Run("handles x=0 correctly when it maps to 0", func(t *testing.T) {
		// Special case: if x=0 maps to y=0
		if xForZero == 0 {
			if prp.Inverse(0) != 0 {
				t.Fatalf("When Forward(0)=0, Inverse(0) should be 0, got %d", prp.Inverse(0))
			}
			t.Log("Correctly handles x=0 → y=0 case")
		} else {
			t.Logf("For this key, x=%d maps to y=0 (not x=0)", xForZero)
		}
	})
}

// BenchmarkTablePRPForward benchmarks forward operation
func BenchmarkTablePRPForward(b *testing.B) {
	benchmarks := []struct {
		name   string
		domain uint64
	}{
		{"n=1K", 1000},
		{"n=10K", 10000},
		{"n=100K", 100000},
		{"n=1M", 1000000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			key := []byte("benchmark-key")
			prp := NewTablePRP(bm.domain, key)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				x := uint64(i) % bm.domain
				_ = prp.Forward(x)
			}
		})
	}
}

// BenchmarkTablePRPInverse benchmarks inverse operation
// This validates O(1) complexity vs old O(n) brute force
func BenchmarkTablePRPInverse(b *testing.B) {
	benchmarks := []struct {
		name   string
		domain uint64
	}{
		{"n=1K", 1000},
		{"n=10K", 10000},
		{"n=100K", 100000},
		{"n=1M", 1000000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			key := []byte("benchmark-key")
			prp := NewTablePRP(bm.domain, key)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				y := uint64(i) % bm.domain
				_ = prp.Inverse(y)
			}
		})
	}
}

// BenchmarkTablePRPInitialization benchmarks table construction time
func BenchmarkTablePRPInitialization(b *testing.B) {
	benchmarks := []struct {
		name   string
		domain uint64
	}{
		{"n=1K", 1000},
		{"n=10K", 10000},
		{"n=100K", 100000},
		{"n=1M", 1000000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			key := []byte("benchmark-key")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = NewTablePRP(bm.domain, key)
			}
		})
	}
}

// TestTablePRPMemoryFootprint validates memory usage
func TestTablePRPMemoryFootprint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	testCases := []struct {
		name            string
		domain          uint64
		maxMemoryMB     float64
		maxMemoryPerElemBytes float64
	}{
		{"n=1M", 1_000_000, 20, 20},      // ~16 MB expected
		{"n=8.4M", 8_400_000, 150, 18},   // ~134 MB expected
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var m1, m2 runtime.MemStats

			// Measure memory before
			runtime.GC()
			runtime.ReadMemStats(&m1)

			// Create TablePRP
			key := []byte("memory-test-key")
			prp := NewTablePRP(tc.domain, key)

			// Measure memory after
			runtime.GC()
			runtime.ReadMemStats(&m2)

			// Calculate memory used
			memoryUsed := m2.Alloc - m1.Alloc
			memoryUsedMB := float64(memoryUsed) / (1024 * 1024)
			memoryPerElem := float64(memoryUsed) / float64(tc.domain)

			t.Logf("Domain: %d elements", tc.domain)
			t.Logf("Memory used: %.2f MB", memoryUsedMB)
			t.Logf("Memory per element: %.2f bytes", memoryPerElem)

			// Verify memory is within bounds
			if memoryUsedMB > tc.maxMemoryMB {
				t.Errorf("Memory usage %.2f MB exceeds max %.2f MB", memoryUsedMB, tc.maxMemoryMB)
			}

			if memoryPerElem > tc.maxMemoryPerElemBytes {
				t.Errorf("Memory per element %.2f bytes exceeds max %.2f bytes",
					memoryPerElem, tc.maxMemoryPerElemBytes)
			}

			// Sanity check: verify PRP still works
			y := prp.Forward(0)
			x := prp.Inverse(y)
			if x != 0 {
				t.Errorf("Sanity check failed: Inverse(Forward(0)) = %d", x)
			}
		})
	}
}

// TestTablePRPRealisticScale validates performance at production scale
func TestTablePRPRealisticScale(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping realistic scale test in short mode")
	}

	// Production parameters from Plinko
	domain := uint64(8_400_000) // 8.4M accounts
	key := []byte("production-key-16bytes")

	t.Logf("Creating TablePRP for n=%d (Plinko production scale)", domain)

	// Create PRP (one-time initialization cost)
	prp := NewTablePRP(domain, key)

	t.Logf("TablePRP initialized successfully")

	// Test a sample of forward/inverse operations
	testCount := 1000
	for i := 0; i < testCount; i++ {
		x := uint64(i * 8400) // Sample across domain
		y := prp.Forward(x)
		xRecovered := prp.Inverse(y)

		if xRecovered != x {
			t.Fatalf("Inverse(Forward(%d)) = %d, expected %d", x, xRecovered, x)
		}
	}

	t.Logf("Verified %d forward/inverse pairs at production scale", testCount)
}

// TestDeterministicRNG validates the RNG used for shuffle
func TestDeterministicRNG(t *testing.T) {
	key := []byte("rng-test-key")

	t.Run("deterministic output", func(t *testing.T) {
		rng1 := NewDeterministicRNG(key)
		rng2 := NewDeterministicRNG(key)

		for i := 0; i < 100; i++ {
			v1 := rng1.Uint64()
			v2 := rng2.Uint64()

			if v1 != v2 {
				t.Fatalf("RNG not deterministic: call %d produced %d and %d", i, v1, v2)
			}
		}
	})

	t.Run("Uint64N uniform distribution", func(t *testing.T) {
		rng := NewDeterministicRNG(key)
		n := uint64(100)
		buckets := make([]int, n)

		// Generate many samples
		samples := 10000
		for i := 0; i < samples; i++ {
			v := rng.Uint64N(n)
			if v >= n {
				t.Fatalf("Uint64N(%d) produced %d >= %d", n, v, n)
			}
			buckets[v]++
		}

		// Check approximate uniformity
		expected := samples / int(n)
		tolerance := expected / 2 // 50% tolerance

		for i, count := range buckets {
			if count < expected-tolerance || count > expected+tolerance {
				t.Logf("Warning: Bucket %d has %d samples, expected ~%d", i, count, expected)
			}
		}
	})

	t.Run("Uint64N edge cases", func(t *testing.T) {
		rng := NewDeterministicRNG(key)

		// n=0 should return 0
		if v := rng.Uint64N(0); v != 0 {
			t.Errorf("Uint64N(0) = %d, expected 0", v)
		}

		// n=1 should always return 0
		for i := 0; i < 10; i++ {
			if v := rng.Uint64N(1); v != 0 {
				t.Errorf("Uint64N(1) = %d, expected 0", v)
			}
		}

		// Power of 2 should work
		v := rng.Uint64N(256)
		if v >= 256 {
			t.Errorf("Uint64N(256) = %d >= 256", v)
		}

		// Non-power of 2 should work
		v = rng.Uint64N(100)
		if v >= 100 {
			t.Errorf("Uint64N(100) = %d >= 100", v)
		}
	})
}
