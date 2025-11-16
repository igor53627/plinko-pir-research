package main

import (
	"testing"
)

// BenchmarkEncodeNode benchmarks the hash-based node encoding
// Paper requirement: Node encoding should be fast enough to not bottleneck
// tree traversal (which is O(log m) depth)
func BenchmarkEncodeNode(b *testing.B) {
	low := uint64(0)
	high := uint64(1023)
	n := uint64(8400000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = encodeNode(low, high, n)
	}
}

// BenchmarkEncodeNodeVarying benchmarks with different parameters
func BenchmarkEncodeNodeVarying(b *testing.B) {
	testCases := []struct {
		name string
		low  uint64
		high uint64
		n    uint64
	}{
		{"small", 0, 10, 100},
		{"medium", 0, 1023, 100000},
		{"production", 0, 1023, 8400000},
		{"large", 0, 10000, 100000000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = encodeNode(tc.low, tc.high, tc.n)
			}
		})
	}
}

// BenchmarkTreeTraversalWithEncoding benchmarks a full tree traversal
// to ensure encoding overhead is acceptable
func BenchmarkTreeTraversalWithEncoding(b *testing.B) {
	n := uint64(8400000)
	m := uint64(1024)

	key := GenerateDeterministicKey()
	iprf := NewIPRF(key, n, m)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Forward operation involves ~log2(1024) = 10 encodeNode calls
		_ = iprf.Forward(uint64(i % 10000))
	}
}
