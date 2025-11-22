package main

// PrfKey128 is a 16-byte PRF key
type PrfKey128 [16]byte

// PRSet represents a pseudorandom set for Plinko PIR
type PRSet struct {
	iprf *IPRF
}

// NewPRSet creates a new PRSet with the given key
// setSize: domain size (number of chunks/blocks)
// chunkSize: range size (size of each chunk)
func NewPRSet(key PrfKey128, setSize, chunkSize uint64) *PRSet {
	var k32 [32]byte
	// Expand 16-byte key to 32 bytes for IPRF (key1 || key2)
	copy(k32[:16], key[:])
	copy(k32[16:], key[:])

	return &PRSet{
		iprf: NewIPRF(k32, setSize, chunkSize),
	}
}

// Expand generates a pseudorandom set of database indices
// Returns: array of setSize indices, one per chunk
func (prs *PRSet) Expand() []uint64 {
	// The setSize is implicit in the IPRF configuration
	setSize := prs.iprf.DomainSize()
	chunkSize := prs.iprf.RangeSize()
	
	indices := make([]uint64, setSize)

	for i := uint64(0); i < setSize; i++ {
		offset := prs.iprf.Forward(i)
		indices[i] = i*chunkSize + offset
	}

	return indices
}