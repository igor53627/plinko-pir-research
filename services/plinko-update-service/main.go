package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	// Database configuration
	DBSize        = 8388608 // 2^23 accounts
	DBEntrySize   = 8
	DBEntryLength = 1 // DBEntrySize / 8

	// Plinko configuration
	CacheEnabled = true // Enable 79x speedup
	CacheSizeMB  = 64   // 8.4M × 8 bytes

	// Ethereum configuration
	AnvilURL          = "ws://eth-mock:8545"
	BlockProcessDelay = 100 * time.Millisecond

	// Output configuration
	DeltaDir  = "/data/deltas"
	HintPath  = "/data/hint.bin"
	HealthPort = "3001"

	// Simulation (for PoC - in production, detect actual changes)
	SimulateChanges       = true
	ChangesPerBlock       = 2000 // Simulated account changes per block
)

type DBEntry [DBEntryLength]uint64

type PlinkoUpdateService struct {
	client         *ethclient.Client
	database       []uint64 // In-memory database
	updateManager  *PlinkoUpdateManager
	blockHeight    uint64
	deltasGenerated uint64
}

func main() {
	log.Println("========================================")
	log.Println("Plinko Update Service")
	log.Println("========================================")
	log.Printf("Database: %d entries (%d MB)\n", DBSize, DBSize*DBEntrySize/1024/1024)
	log.Printf("Cache mode: %v (speedup: 79x)\n", CacheEnabled)
	log.Printf("Simulated changes per block: %d\n", ChangesPerBlock)
	log.Println()

	// Wait for hint.bin to exist
	waitForHint()

	// Load hint/database
	log.Println("Loading database from hint.bin...")
	database, chunkSize, setSize := loadDatabase()
	log.Printf("Loaded %d entries (ChunkSize: %d, SetSize: %d)\n",
		len(database)/DBEntryLength, chunkSize, setSize)

	// Create Plinko update manager
	log.Println("Initializing Plinko Update Manager...")
	pm := NewPlinkoUpdateManager(database, chunkSize, setSize)

	// Enable cache mode
	if CacheEnabled {
		log.Println("Building update cache...")
		cacheDuration := pm.EnableCacheMode()
		log.Printf("✅ Cache mode enabled in %v\n", cacheDuration)
		log.Printf("   Memory usage: %d MB\n", CacheSizeMB)
		log.Println()
	}

	// Create delta directory
	if err := os.MkdirAll(DeltaDir, 0755); err != nil {
		log.Fatalf("Failed to create delta directory: %v", err)
	}

	// Start health check server
	go startHealthServer()

	// Create service
	service := &PlinkoUpdateService{
		database:       database,
		updateManager:  pm,
		blockHeight:    0,
		deltasGenerated: 0,
	}

	// Connect to Ethereum
	log.Printf("Connecting to Anvil at %s...\n", AnvilURL)
	if err := service.connectToEthereum(); err != nil {
		log.Fatalf("Failed to connect to Ethereum: %v", err)
	}
	defer service.client.Close()

	log.Println("✅ Connected to Anvil")
	log.Println()
	log.Println("Starting block monitoring...")
	log.Println("========================================")
	log.Println()

	// Monitor blocks
	service.monitorBlocks()
}

func waitForHint() {
	log.Println("Waiting for hint.bin...")
	for i := 0; i < 120; i++ {
		if _, err := os.Stat(HintPath); err == nil {
			log.Println("✅ hint.bin found")
			return
		}
		if i%10 == 0 && i > 0 {
			log.Printf("  Still waiting... (%d/120s)\n", i)
		}
		time.Sleep(1 * time.Second)
	}
	log.Fatal("Timeout waiting for hint.bin")
}

func loadDatabase() ([]uint64, uint64, uint64) {
	data, err := os.ReadFile(HintPath)
	if err != nil {
		log.Fatalf("Failed to read hint.bin: %v", err)
	}

	// Read metadata header
	if len(data) < 32 {
		log.Fatal("Invalid hint.bin: too small for header")
	}

	dbSize := binary.LittleEndian.Uint64(data[0:8])
	chunkSize := binary.LittleEndian.Uint64(data[8:16])
	setSize := binary.LittleEndian.Uint64(data[16:24])

	log.Printf("Hint metadata: DBSize=%d, ChunkSize=%d, SetSize=%d\n",
		dbSize, chunkSize, setSize)

	// Extract database (skip 32-byte header)
	dbBytes := data[32:]
	dbEntries := len(dbBytes) / DBEntrySize

	database := make([]uint64, dbEntries)
	for i := 0; i < dbEntries; i++ {
		database[i] = binary.LittleEndian.Uint64(dbBytes[i*DBEntrySize : (i+1)*DBEntrySize])
	}

	return database, chunkSize, setSize
}

func (s *PlinkoUpdateService) connectToEthereum() error {
	var err error
	// Try WebSocket first, fall back to HTTP
	wsURL := AnvilURL
	httpURL := "http://eth-mock:8545"

	for i := 0; i < 10; i++ {
		s.client, err = ethclient.Dial(wsURL)
		if err == nil {
			return nil
		}

		log.Printf("WebSocket connection failed, trying HTTP...")
		s.client, err = ethclient.Dial(httpURL)
		if err == nil {
			log.Printf("⚠️  Using HTTP polling (WebSocket unavailable)")
			return nil
		}

		log.Printf("Connection attempt %d/10 failed, retrying...\n", i+1)
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("failed to connect after 10 attempts: %w", err)
}

func (s *PlinkoUpdateService) monitorBlocks() {
	ctx := context.Background()
	ticker := time.NewTicker(BlockProcessDelay)
	defer ticker.Stop()

	var lastBlockNumber uint64 = 0

	for range ticker.C {
		// Get latest block number
		blockNumber, err := s.client.BlockNumber(ctx)
		if err != nil {
			log.Printf("Error getting block number: %v\n", err)
			continue
		}

		// Process new blocks
		if blockNumber > lastBlockNumber {
			for bn := lastBlockNumber + 1; bn <= blockNumber; bn++ {
				if err := s.processBlock(ctx, bn); err != nil {
					log.Printf("Error processing block %d: %v\n", bn, err)
				}
			}
			lastBlockNumber = blockNumber
		}
	}
}

func (s *PlinkoUpdateService) processBlock(ctx context.Context, blockNumber uint64) error {
	startTime := time.Now()

	// Get block header
	header, err := s.client.HeaderByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return fmt.Errorf("failed to get block header: %w", err)
	}

	// Simulate account changes (in production, detect actual changes)
	updates := s.detectChanges(blockNumber, header)

	if len(updates) == 0 {
		// No changes detected
		return nil
	}

	// Generate hint deltas using Plinko
	deltas, updateDuration := s.updateManager.ApplyUpdates(updates)

	// Save delta file
	deltaPath := filepath.Join(DeltaDir, fmt.Sprintf("delta-%06d.bin", blockNumber))
	if err := saveDelta(deltaPath, deltas); err != nil {
		return fmt.Errorf("failed to save delta: %w", err)
	}

	s.deltasGenerated++

	// Log progress
	blockDuration := time.Since(startTime)
	log.Printf("Block %d: %d changes, %d deltas, update: %v, total: %v\n",
		blockNumber, len(updates), len(deltas),
		updateDuration, blockDuration)

	return nil
}

func (s *PlinkoUpdateService) detectChanges(blockNumber uint64, header *types.Header) []DBUpdate {
	// PoC: Simulate account changes
	// In production: parse block transactions and detect balance/state changes

	if !SimulateChanges {
		return nil
	}

	updates := make([]DBUpdate, ChangesPerBlock)

	// Simulate deterministic changes based on block number
	for i := 0; i < ChangesPerBlock; i++ {
		index := uint64((blockNumber*ChangesPerBlock + uint64(i)) % DBSize)

		// Read old value
		oldValue := s.readDBEntry(index)

		// Generate new value (simulated change)
		newValue := DBEntry{uint64(blockNumber)*1000 + uint64(i)}

		updates[i] = DBUpdate{
			Index:    index,
			OldValue: oldValue,
			NewValue: newValue,
		}
	}

	return updates
}

func (s *PlinkoUpdateService) readDBEntry(index uint64) DBEntry {
	if index >= uint64(len(s.database)/DBEntryLength) {
		return DBEntry{}
	}
	return DBEntry{s.database[index]}
}

func saveDelta(path string, deltas []HintDelta) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write delta count
	var header [16]byte
	binary.LittleEndian.PutUint64(header[0:8], uint64(len(deltas)))
	binary.LittleEndian.PutUint64(header[8:16], 0) // Reserved

	if _, err := f.Write(header[:]); err != nil {
		return err
	}

	// Write each delta
	for _, delta := range deltas {
		var entry [24]byte
		binary.LittleEndian.PutUint64(entry[0:8], delta.HintSetID)
		binary.LittleEndian.PutUint64(entry[8:16], boolToUint64(delta.IsBackupSet))
		binary.LittleEndian.PutUint64(entry[16:24], delta.Delta[0])

		if _, err := f.Write(entry[:]); err != nil {
			return err
		}
	}

	return nil
}

func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func startHealthServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check if delta directory exists
		if _, err := os.Stat(DeltaDir); os.IsNotExist(err) {
			http.Error(w, "Delta directory not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","service":"plinko-update"}`)
	})

	log.Printf("Health check server listening on :%s\n", HealthPort)
	if err := http.ListenAndServe(":"+HealthPort, nil); err != nil {
		log.Printf("Health server error: %v\n", err)
	}
}
