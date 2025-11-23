package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	BundleSize = 100
)

type DeltaBundler struct {
	cfg           Config
	ipfsPublisher *IPFSPublisher
	mu            sync.Mutex
	latestBlock   uint64
}

type Manifest struct {
	LatestBlock uint64       `json:"latestBlock"`
	Bundles     []BundleInfo `json:"bundles"`
	Deltas      []DeltaInfo  `json:"deltas,omitempty"`
}

type BundleInfo struct {
	StartBlock uint64 `json:"startBlock"`
	EndBlock   uint64 `json:"endBlock"`
	CID        string `json:"cid,omitempty"`
	URL        string `json:"url,omitempty"`
}

type DeltaInfo struct {
	Block uint64 `json:"block"`
	CID   string `json:"cid"`
}

func NewDeltaBundler(cfg Config) (*DeltaBundler, error) {
	ipfsPublisher, err := newIPFSPublisher(cfg.IPFSAPI, cfg.IPFSGateway)
	if err != nil {
		log.Printf("⚠️ Failed to initialize IPFS publisher: %v. Bundles will not be pinned to IPFS.", err)
	} else if ipfsPublisher != nil {
		log.Printf("✅ IPFS publisher initialized (API: %s)", cfg.IPFSAPI)
	}

	return &DeltaBundler{
		cfg:           cfg,
		ipfsPublisher: ipfsPublisher,
	}, nil
}

func (b *DeltaBundler) ProcessBlock(blockNumber uint64) error {
	// Deprecated: Use PublishDelta instead
	return nil
}

func (b *DeltaBundler) PublishDelta(blockNumber uint64, path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.latestBlock = blockNumber

	// Pin to IPFS
	var cid string
	if b.ipfsPublisher != nil {
		var err error
		cid, err = b.ipfsPublisher.PublishFile(path)
		if err != nil {
			log.Printf("⚠️ Failed to publish delta %d to IPFS: %v", blockNumber, err)
		} else {
			log.Printf("🌐 Delta %d pinned to IPFS: %s", blockNumber, cid)
		}
	}

	// Update manifest with new delta
	return b.addDeltaToManifest(blockNumber, cid)
}

func (b *DeltaBundler) createBundle(startBlock, endBlock uint64) error {
	log.Printf("📦 Creating delta bundle for blocks %d-%d...", startBlock, endBlock)

	var bundleData bytes.Buffer

	// Concatenate deltas
	for i := startBlock; i <= endBlock; i++ {
		filename := fmt.Sprintf("delta-%06d.bin", i)
		path := filepath.Join(b.cfg.DeltaOutputDir, filename)
		
		data, err := os.ReadFile(path)
		if err != nil {
			// If a delta is missing, we can't create the bundle
			// This might happen if the node was down.
			// We could skip or fail. For now, fail.
			return fmt.Errorf("missing delta file %s: %w", filename, err)
		}
		
		bundleData.Write(data)
	}

	// Write bundle file
	bundleFilename := fmt.Sprintf("bundle-%06d-%06d.bin", startBlock, endBlock)
	bundlePath := filepath.Join(b.cfg.DeltaOutputDir, bundleFilename)
	
	if err := os.WriteFile(bundlePath, bundleData.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write bundle file: %w", err)
	}

	log.Printf("✅ Bundle created: %s (%.2f MB)", bundleFilename, float64(bundleData.Len())/1024/1024)

	// Publish to IPFS
	var cid string
	if b.ipfsPublisher != nil {
		var err error
		cid, err = b.ipfsPublisher.PublishFile(bundlePath)
		if err != nil {
			log.Printf("⚠️ Failed to publish bundle to IPFS: %v", err)
		} else {
			log.Printf("🌐 Bundle pinned to IPFS: %s", cid)
		}
	}

	// Add to manifest
	if err := b.addBundleToManifest(startBlock, endBlock, cid); err != nil {
		return err
	}

	// Clean up individual deltas from manifest that are now bundled
	return b.cleanupBundledDeltas(endBlock)
}

func (b *DeltaBundler) updateManifest() error {
	manifestPath := filepath.Join(b.cfg.DeltaOutputDir, "manifest.json")
	
	var manifest Manifest
	// Read existing manifest if it exists
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}

	manifest.LatestBlock = b.latestBlock

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, data, 0644)
}

func (b *DeltaBundler) addBundleToManifest(start, end uint64, cid string) error {
	manifestPath := filepath.Join(b.cfg.DeltaOutputDir, "manifest.json")
	
	var manifest Manifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}

	// Check if bundle already exists
	exists := false
	for i, bundle := range manifest.Bundles {
		if bundle.StartBlock == start && bundle.EndBlock == end {
			manifest.Bundles[i].CID = cid // Update CID if changed
			exists = true
			break
		}
	}

	if !exists {
		manifest.Bundles = append(manifest.Bundles, BundleInfo{
			StartBlock: start,
			EndBlock:   end,
			CID:        cid,
		})
		// Sort bundles
		sort.Slice(manifest.Bundles, func(i, j int) bool {
			return manifest.Bundles[i].StartBlock < manifest.Bundles[j].StartBlock
		})
	}

	manifest.LatestBlock = b.latestBlock

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, data, 0644)
}

func (b *DeltaBundler) addDeltaToManifest(block uint64, cid string) error {
	manifestPath := filepath.Join(b.cfg.DeltaOutputDir, "manifest.json")
	
	var manifest Manifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}

	// Check if delta already exists
	exists := false
	for i, delta := range manifest.Deltas {
		if delta.Block == block {
			manifest.Deltas[i].CID = cid
			exists = true
			break
		}
	}

	if !exists {
		manifest.Deltas = append(manifest.Deltas, DeltaInfo{
			Block: block,
			CID:   cid,
		})
		// Sort deltas
		sort.Slice(manifest.Deltas, func(i, j int) bool {
			return manifest.Deltas[i].Block < manifest.Deltas[j].Block
		})
	}

	manifest.LatestBlock = b.latestBlock
	
	// Trigger bundle creation if needed
	if block > 0 && block%BundleSize == 0 {
		// We need to create a bundle. But we are inside a lock (from PublishDelta).
		// Wait, createBundle calls addBundleToManifest which writes to file.
		// This is fine as long as we don't deadlock. PublishDelta holds lock. createBundle does NOT hold lock (it's private).
		// But createBundle was called from ProcessBlock which held lock.
		
		// Refactor: createBundle should be called here?
		// No, let's keep it simple.
		
		startBlock := block - BundleSize + 1
		endBlock := block
		
		// We are holding lock in PublishDelta.
		// createBundle reads files (IO) and writes file (IO) and IPFS (Net).
		// It's better to do this outside lock or allow long lock?
		// For simplicity, do it here.
		
		if err := b.createBundle(startBlock, endBlock); err != nil {
			log.Printf("Failed to create bundle %d-%d: %v", startBlock, endBlock, err)
		}
		
		// createBundle will update manifest again. This is a bit inefficient (write twice) but safe.
		// Reload manifest because createBundle updated it?
		// Yes, createBundle reads/writes manifest.
		return nil
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, data, 0644)
}

func (b *DeltaBundler) cleanupBundledDeltas(bundledUpTo uint64) error {
	manifestPath := filepath.Join(b.cfg.DeltaOutputDir, "manifest.json")
	
	var manifest Manifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		json.Unmarshal(data, &manifest)
	}

	// Filter out deltas that are <= bundledUpTo
	newDeltas := make([]DeltaInfo, 0)
	for _, delta := range manifest.Deltas {
		if delta.Block > bundledUpTo {
			newDeltas = append(newDeltas, delta)
		}
	}
	manifest.Deltas = newDeltas

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, data, 0644)
}
