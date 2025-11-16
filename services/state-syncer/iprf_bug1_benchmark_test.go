package main

import (
	"testing"
)

// BenchmarkInverseFixed benchmarks the optimized O(log m + k) inverse
func BenchmarkInverseFixed(b *testing.B) {
	n := uint64(8400000)
	m := uint64(1024)
	key := GenerateDeterministicKey()
	iprf := NewIPRF(key, n, m)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iprf.InverseFixed(uint64(i % int(m)))
	}
}

// BenchmarkInverseFixedVaryingSize benchmarks with different domain/range sizes
func BenchmarkInverseFixedVaryingSize(b *testing.B) {
	testCases := []struct {
		name string
		n    uint64
		m    uint64
	}{
		{"small", 10000, 100},
		{"medium", 1000000, 512},
		{"production", 8400000, 1024},
		{"large", 10000000, 2048},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			key := GenerateDeterministicKey()
			iprf := NewIPRF(key, tc.n, tc.m)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = iprf.InverseFixed(uint64(i % int(tc.m)))
			}
		})
	}
}

// BenchmarkBruteForceInverse benchmarks the old O(n) brute force (for comparison)
func BenchmarkBruteForceInverse(b *testing.B) {
	// Use smaller domain for brute force (otherwise takes too long)
	n := uint64(100000)
	m := uint64(100)
	key := GenerateDeterministicKey()
	iprf := NewIPRF(key, n, m)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = iprf.bruteForceInverse(uint64(i % int(m)))
	}
}

// BenchmarkInverseComparison compares optimized vs brute force on same small domain
func BenchmarkInverseComparison(b *testing.B) {
	n := uint64(100000)
	m := uint64(100)
	key := GenerateDeterministicKey()
	iprf := NewIPRF(key, n, m)

	b.Run("optimized", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = iprf.InverseFixed(uint64(i % int(m)))
		}
	})

	b.Run("bruteforce", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = iprf.bruteForceInverse(uint64(i % int(m)))
		}
	})
}
