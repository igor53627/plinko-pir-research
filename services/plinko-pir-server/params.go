package main

import (
	"math"
)

// derivePlinkoParams mirrors the hint generator logic to keep chunk and set
// sizes deterministic for a given canonical database size.
func derivePlinkoParams(dbEntries uint64) (uint64, uint64) {
	if dbEntries == 0 {
		return 1, 1
	}

	targetChunkSize := uint64(2 * math.Sqrt(float64(dbEntries)))
	chunkSize := uint64(1)
	for chunkSize < targetChunkSize {
		chunkSize *= 2
	}

	setSize := uint64(math.Ceil(float64(dbEntries) / float64(chunkSize)))
	setSize = (setSize + 3) / 4 * 4

	return chunkSize, setSize
}
