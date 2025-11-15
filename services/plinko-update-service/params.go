package main

import "math"

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
