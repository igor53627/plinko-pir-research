package main

import (
	"sort"
)

// Fixed Inverse implementation that correctly enumerates all preimages
func (iprf *IPRF) InverseFixed(y uint64) []uint64 {
	if y >= iprf.range_ {
		return []uint64{}
	}
	
	// Use brute-force method for now - it's correct and we can optimize later
	return iprf.bruteForceInverse(y)
}

// bruteForceInverse is a simple but correct implementation
func (iprf *IPRF) bruteForceInverse(y uint64) []uint64 {
	var result []uint64
	for x := uint64(0); x < iprf.domain; x++ {
		if iprf.Forward(x) == y {
			result = append(result, x)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

// OptimizedInverse implements the correct tree traversal for inverse
func (iprf *IPRF) OptimizedInverse(y uint64) []uint64 {
	if y >= iprf.range_ {
		return []uint64{}
	}
	
	// Find the target bin and collect all balls that land in it
	return iprf.collectBallsInBin(y, iprf.domain, iprf.range_, 0, iprf.domain-1)
}

// collectBallsInBin recursively collects all balls that land in the target bin
func (iprf *IPRF) collectBallsInBin(
	targetBin uint64,
	n uint64, m uint64,
	startIdx uint64, endIdx uint64) []uint64 {
	
	if startIdx > endIdx {
		return []uint64{}
	}
	
	if m == 1 {
		// All balls go to bin 0
		if targetBin == 0 {
			result := make([]uint64, endIdx-startIdx+1)
			for i := startIdx; i <= endIdx; i++ {
				result[i-startIdx] = i
			}
			return result
		}
		return []uint64{}
	}
	
	if startIdx == endIdx {
		// Single ball - check if it goes to target bin
		// Convert to relative index within current domain [0, n)
		relativeBallIndex := uint64(0)
		ballResult := iprf.traceBall(relativeBallIndex, n, m)
		if ballResult == targetBin {
			return []uint64{startIdx}
		}
		return []uint64{}
	}
	
	// Binary tree traversal
	low := uint64(0)
	high := m - 1
	currentStart := startIdx
	currentEnd := endIdx
	currentN := n
	
	var result []uint64
	
	for low < high {
		mid := (low + high) / 2
		leftBins := mid - low + 1
		totalBins := high - low + 1
		p := float64(leftBins) / float64(totalBins)
		
		// Sample the binomial split point for this node
		nodeID := encodeNode(low, high, currentN)
		leftCount := iprf.sampleBinomial(nodeID, currentN, p)
		
		// Determine the split point in the current range
		splitPoint := currentStart + leftCount
		if splitPoint > currentEnd+1 {
			splitPoint = currentEnd + 1
		}
		
		// Process left subtree
		if targetBin <= mid {
			// Target is in left subtree
			leftResult := iprf.collectBallsInBin(targetBin, leftCount, leftBins, currentStart, splitPoint-1)
			result = append(result, leftResult...)
			break // No need to process right subtree
		} else {
			// Target is in right subtree
			rightStart := splitPoint
			rightEnd := currentEnd
			rightN := currentN - leftCount
			rightLow := mid + 1
			rightHigh := high
			rightBins := high - mid
			
			// Continue with right subtree
			low = rightLow
			high = rightHigh
			currentStart = rightStart
			currentEnd = rightEnd
			currentN = rightN
			n = rightN
			m = rightBins
		}
	}
	
	return result
}

// TreeInverse implements a tree-based inverse that's more efficient than brute force
func (iprf *IPRF) TreeInverse(y uint64) []uint64 {
	if y >= iprf.range_ {
		return []uint64{}
	}
	
	// Build a tree representation and find all paths that lead to target bin
	paths := iprf.findPathsToBin(y, iprf.domain, iprf.range_)
	
	// Convert paths to actual indices
	var result []uint64
	for _, path := range paths {
		indices := iprf.pathToIndices(path, iprf.domain, iprf.range_)
		result = append(result, indices...)
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	
	return result
}

type treePath struct {
	low, high uint64
	startIdx, endIdx uint64
	n uint64
}

func (iprf *IPRF) findPathsToBin(targetBin uint64, n uint64, m uint64) []treePath {
	var paths []treePath
	iprf.findPathsRecursive(targetBin, 0, m-1, n, 0, n-1, &paths)
	return paths
}

func (iprf *IPRF) findPathsRecursive(
	targetBin uint64,
	low uint64, high uint64,
	n uint64,
	startIdx uint64, endIdx uint64,
	paths *[]treePath) {
	
	if startIdx > endIdx {
		return
	}
	
	if low == high {
		// Found a leaf node that contains our target bin
		*paths = append(*paths, treePath{
			low: low, high: high,
			startIdx: startIdx, endIdx: endIdx,
			n: n,
		})
		return
	}
	
	mid := (low + high) / 2
	leftBins := mid - low + 1
	totalBins := high - low + 1
	p := float64(leftBins) / float64(totalBins)
	
	// Sample the binomial split point for this node
	nodeID := encodeNode(low, high, n)
	leftCount := iprf.sampleBinomial(nodeID, n, p)
	
	// Determine the split point in the current range
	splitPoint := startIdx + leftCount
	if splitPoint > endIdx+1 {
		splitPoint = endIdx + 1
	}
	
	// Recurse on appropriate subtree
	if targetBin <= mid {
		// Target is in left subtree
		iprf.findPathsRecursive(targetBin, low, mid, leftCount, startIdx, splitPoint-1, paths)
	} else {
		// Target is in right subtree
		rightStart := splitPoint
		rightEnd := endIdx
		rightN := n - leftCount
		iprf.findPathsRecursive(targetBin, mid+1, high, rightN, rightStart, rightEnd, paths)
	}
}

func (iprf *IPRF) pathToIndices(path treePath, totalN uint64, totalM uint64) []uint64 {
	// For a leaf node, all indices in the range map to this bin
	result := make([]uint64, path.endIdx-path.startIdx+1)
	for i := path.startIdx; i <= path.endIdx; i++ {
		result[i-path.startIdx] = i
	}
	return result
}