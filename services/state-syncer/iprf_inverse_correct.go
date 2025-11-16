package main

import (
	"sort"
)

// InverseFixed returns all x such that Forward(x) = y
// Uses paper-correct O(log m + k) tree-based algorithm instead of O(n) brute force
//
// BUG #1 FIX: Previous implementation used bruteForceInverse which scans entire
// domain in O(n) time (~7.8s for n=8.4M). Paper specifies O(log m + k) algorithm
// via tree traversal where k ≈ n/m.
//
// Paper specification (Theorem 4.4, Section 4.3):
// S⁻¹(k, y) traverses binary tree to target bin (O(log m) depth = 10 levels for m=1024)
// and collects all balls in that bin (O(k) enumeration where k ≈ 8203 for n=8.4M, m=1024)
//
// Expected performance: <10ms (vs 7800ms for brute force = ~780x speedup)
func (iprf *IPRF) InverseFixed(y uint64) []uint64 {
	if y >= iprf.range_ {
		return []uint64{}
	}

	// Use paper-correct O(log m + k) algorithm via tree enumeration
	// This replaces the O(n) brute force that was scanning entire domain
	return iprf.enumerateBallsInBin(y, iprf.domain, iprf.range_)
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
	
	// Use the tree-based O(log n) implementation
	// This builds the complete tree and finds all paths that lead to the target bin
	return iprf.TreeInverse(y)
}

// findAllPreimages finds all indices that map to the target output
// This works by reversing the tree traversal logic used in traceBall
func (iprf *IPRF) findAllPreimages(targetBin uint64, domainSize uint64, rangeSize uint64) []uint64 {
	if targetBin >= rangeSize {
		return []uint64{}
	}
	
	// Use tree-based approach that mirrors the forward function
	// We traverse the same tree structure but collect all paths that lead to targetBin
	return iprf.collectPreimagesTree(targetBin, 0, rangeSize-1, domainSize, 0, domainSize-1)
}

// collectPreimagesTree recursively finds all indices that map to the target bin
// by working backwards through the tree structure
func (iprf *IPRF) collectPreimagesTree(
	targetBin uint64,
	lowBin uint64, highBin uint64,
	totalBalls uint64,
	startIdx uint64, endIdx uint64) []uint64 {
	
	if startIdx > endIdx {
		return []uint64{}
	}
	
	if lowBin == highBin {
		// Leaf node - if this is our target bin, all indices in this range map to it
		if lowBin == targetBin {
			result := make([]uint64, endIdx-startIdx+1)
			for i := startIdx; i <= endIdx; i++ {
				result[i-startIdx] = i
			}
			return result
		}
		return []uint64{}
	}
	
	// Binary tree traversal - same logic as traceBall but in reverse
	midBin := (lowBin + highBin) / 2
	leftBins := midBin - lowBin + 1
	totalBins := highBin - lowBin + 1
	p := float64(leftBins) / float64(totalBins)
	
	// Sample the binomial split point for this node
	nodeID := encodeNode(lowBin, highBin, totalBalls)
	leftCount := iprf.sampleBinomial(nodeID, totalBalls, p)
	
	// Determine the split point in the current range
	splitPoint := startIdx + leftCount
	if splitPoint > endIdx+1 {
		splitPoint = endIdx + 1
	}
	
	var result []uint64
	
	// Check both subtrees
	if targetBin <= midBin {
		// Target is in left subtree
		leftResult := iprf.collectPreimagesTree(targetBin, lowBin, midBin, leftCount, startIdx, splitPoint-1)
		result = append(result, leftResult...)
	}
	
	if targetBin > midBin {
		// Target is in right subtree
		rightStart := splitPoint
		rightEnd := endIdx
		rightBalls := totalBalls - leftCount
		rightLow := midBin + 1
		rightHigh := highBin
		
		rightResult := iprf.collectPreimagesTree(targetBin, rightLow, rightHigh, rightBalls, rightStart, rightEnd)
		result = append(result, rightResult...)
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