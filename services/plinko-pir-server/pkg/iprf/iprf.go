package iprf

// IPRF implements an Invertible Pseudorandom Function
// by composing a PRP and a PMNS.
// F(k, x) = S(k2, P(k1, x))
// F^-1(k, y) = { P^-1(k1, x) : x \in S^-1(k2, y) }
type IPRF struct {
	pmns *PMNS
	prp  *PRP
}

// New creates a new IPRF with the given keys and domain/range sizes.
// key1 is for PRP, key2 is for PMNS.
func New(key1, key2 []byte, n, m uint64) *IPRF {
	return &IPRF{
		pmns: NewPMNS(key2, n, m),
		prp:  NewPRP(key1, n),
	}
}

// F implements the forward evaluation: F(k, x) -> y
func (f *IPRF) F(x uint64) uint64 {
	// 1. Permute x using PRP
	permuted := f.prp.Permute(x)

	// 2. Map to bin using PMNS
	return f.pmns.S(permuted)
}

// Inverse implements the backward evaluation: F^-1(k, y) -> {x}
// Returns a slice of all inputs x that map to y.
func (f *IPRF) Inverse(y uint64) []uint64 {
	// 1. Get set of permuted balls from PMNS
	start, count := f.pmns.SInverse(y)

	if count == 0 {
		return nil
	}

	result := make([]uint64, count)

	// 2. Inverse permute each ball to get original x
	for i := uint64(0); i < count; i++ {
		permuted := start + i
		result[i] = f.prp.Inverse(permuted)
	}

	return result
}
