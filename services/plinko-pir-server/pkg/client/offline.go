package client

import (
	"math/rand"
	"plinko-pir-server/pkg/iprf"
)

// Hint represents a client-side hint
// In the paper, hints are parities of subsets of blocks.
// For simplicity, we store the parity and the hint index/metadata.
type Hint struct {
	Parity [4]uint64
	Used   bool // Track if hint has been consumed
	// For backup hints, we might need to store the set of indices explicitly
	// since they are not defined by iPRF.
	// For simplicity in this PoC, we'll assume backup hints are also "structured"
	// or we store the indices.
	// Let's store indices for backup hints.
	Indices []uint64
}

// Client represents a Plinko PIR client
type Client struct {
	iprf        *iprf.IPRF
	hints       []Hint // Regular hints H (indexed by iPRF range)
	backupHints []Hint // Backup hints T (list of random hints)

	n uint64 // Database size
	m uint64 // Number of hints (range of iPRF)

	// Keys
	keyAlpha []byte // Key for iPRF
	keyBeta  []byte // Key for PMNS (part of iPRF)
}

// NewClient creates a new client
func NewClient(n, m uint64, keyAlpha, keyBeta []byte) *Client {
	return &Client{
		iprf:     iprf.New(keyAlpha, keyBeta, n, m),
		hints:    make([]Hint, m),
		n:        n,
		m:        m,
		keyAlpha: keyAlpha,
		keyBeta:  keyBeta,
	}
}

// DBEntry represents a database entry (index and value)
type DBEntry struct {
	Index uint64
	Value [4]uint64
}

// HintInit performs the offline phase
// It iterates over the entire database and updates hints.
// dbStream is a function that yields database entries.
func (c *Client) HintInit(dbStream func() (DBEntry, bool)) {
	// Initialize hints to 0
	for i := range c.hints {
		c.hints[i].Parity = [4]uint64{0, 0, 0, 0}
		c.hints[i].Used = false
	}

	// Stream database
	for {
		entry, ok := dbStream()
		if !ok {
			break
		}

		// For each entry x, find which hints it belongs to using iPRF forward
		// F(k, x) -> y
		y := c.iprf.F(entry.Index)

		if y < uint64(len(c.hints)) {
			// XOR 256-bit value
			for k := 0; k < 4; k++ {
				c.hints[y].Parity[k] ^= entry.Value[k]
			}
		}
	}
}

// InitBackupHints generates backup hints
// count: number of backup hints to generate
// setSize: size of each backup hint set
// dbStream: needs to stream DB again (or we do it in one pass if possible)
// For simplicity, we assume we can stream again or this is called during HintInit if we merge logic.
// Here we implement it as a separate pass for clarity.
func (c *Client) InitBackupHints(count int, setSize int, dbStream func() (DBEntry, bool)) {
	c.backupHints = make([]Hint, count)

	// Generate random sets for backup hints
	// We use a fixed seed for reproducibility in this PoC
	rng := rand.New(rand.NewSource(42))

	for i := 0; i < count; i++ {
		// Generate a random set of indices
		indices := make([]uint64, setSize)
		seen := make(map[uint64]bool)
		for j := 0; j < setSize; j++ {
			for {
				idx := uint64(rng.Int63n(int64(c.n)))
				if !seen[idx] {
					seen[idx] = true
					indices[j] = idx
					break
				}
			}
		}
		c.backupHints[i] = Hint{
			Indices: indices,
			Parity:  [4]uint64{0, 0, 0, 0},
			Used:    false,
		}
	}

	// Compute parities
	for {
		entry, ok := dbStream()
		if !ok {
			break
		}

		// Check which backup hints contain this entry
		for i := range c.backupHints {
			for _, idx := range c.backupHints[i].Indices {
				if idx == entry.Index {
					for k := 0; k < 4; k++ {
						c.backupHints[i].Parity[k] ^= entry.Value[k]
					}
					break
				}
			}
		}
	}
}
