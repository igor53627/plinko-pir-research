//go:build !bench

package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	DBEntrySize   = 32 // Size of each database entry in bytes
	DBEntryLength = 4  // Number of uint64 values per database entry (DBEntrySize / 8)

	// Note: The current implementation assumes DBEntryLength = 4.
	// If this changes, the indexing logic in applyDatabaseUpdate and other
	// functions may need to be reviewed for correctness.

	CacheEnabled      = true
	BlockProcessDelay = 100 * time.Millisecond
	ChangesPerBlock   = 2000 // Simulated account changes per block
)

type DBEntry [DBEntryLength]uint64

type PlinkoUpdateService struct {
	client          *ethclient.Client
	database        []uint64 // In-memory database
	updateManager   *PlinkoUpdateManager
	blockHeight     uint64
	deltasGenerated uint64
	cfg             Config
	dbSize          uint64
	chunkSize       uint64
	setSize         uint64
	snapshotVersion string
	addressIndex    map[string]uint64
	useSimulated    bool
	chainID         *big.Int
}

func main() {
	log.Println("========================================")
	log.Println("Plinko Update Service")
	log.Println("========================================")

	cfg := LoadConfig()
	log.Printf("Configuration: database=%s, public_root=%s, delta_dir=%s, rpc=%s, simulated_updates=%v\n",
		cfg.DatabasePath, cfg.PublicRoot, cfg.DeltaOutputDir, cfg.RPCURL, cfg.UseSimulated)

	waitForDatabase(cfg.DatabasePath, cfg.DatabaseWaitTimeout)

	log.Println("Loading canonical database snapshot...")
	database, dbSize, chunkSize, setSize := loadDatabase(cfg.DatabasePath)
	log.Printf("Loaded database: %d entries (ChunkSize: %d, SetSize: %d)\n",
		dbSize, chunkSize, setSize)

	addressIndex, err := loadAddressMapping(cfg.AddressMappingPath)
	if err != nil {
		log.Fatalf("Failed to read address-mapping: %v", err)
	}
	log.Printf("Loaded %d address mappings\n", len(addressIndex))

	// Publish snapshot + manifest to public artifacts
	if err := os.MkdirAll(cfg.PublicSnapshotsDir(), 0o755); err != nil {
		log.Fatalf("Failed to create snapshots directory: %v", err)
	}

	version, err := publishSnapshot(cfg, cfg.DatabasePath, dbSize, chunkSize, setSize)
	if err != nil {
		log.Fatalf("Failed to publish snapshot: %v", err)
	}
	log.Printf("Published snapshot version %s\n", version)

	if err := ensureAddressMappingPublished(cfg.AddressMappingPath, cfg.PublicAddressMappingPath()); err != nil {
		log.Fatalf("Failed to publish address-mapping.bin: %v", err)
	}
	log.Println("Address mapping exported for CDN")

	// Create Plinko update manager
	log.Println("Initializing Plinko Update Manager...")
	pm := NewPlinkoUpdateManager(database, dbSize, chunkSize, setSize)

	// Enable cache mode
	if CacheEnabled {
		log.Println("Building update cache...")
		cacheDuration := pm.EnableCacheMode()
		cacheMB := float64(dbSize*DBEntrySize) / 1024 / 1024
		log.Printf("✅ Cache mode enabled in %v (memory: %.1f MB)\n", cacheDuration, cacheMB)
		log.Println()
	}

	// Create delta directory
	if err := os.MkdirAll(cfg.DeltaOutputDir, 0o755); err != nil {
		log.Fatalf("Failed to create delta directory: %v", err)
	}

	// Create service
	service := &PlinkoUpdateService{
		database:        database,
		updateManager:   pm,
		blockHeight:     0,
		deltasGenerated: 0,
		cfg:             cfg,
		dbSize:          dbSize,
		chunkSize:       chunkSize,
		setSize:         setSize,
		snapshotVersion: version,
		addressIndex:    addressIndex,
		useSimulated:    cfg.UseSimulated,
	}

	// Start health check server
	go service.startHealthServer()

	// Connect to Ethereum
	log.Printf("Connecting to Ethereum RPC at %s...\n", cfg.RPCURL)
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

func waitForDatabase(path string, timeout time.Duration) {
	log.Printf("Waiting for database.bin at %s...\n", path)
	if timeout <= 0 {
		if _, err := os.Stat(path); err != nil {
			log.Fatalf("database file %s not found and timeout disabled", path)
		}
		log.Println("✅ database file found")
		return
	}

	start := time.Now()
	for {
		if _, err := os.Stat(path); err == nil {
			log.Println("✅ database file found")
			return
		}
		if time.Since(start) >= timeout {
			log.Fatalf("Timeout waiting for database file at %s", path)
		}
		time.Sleep(time.Second)
	}
}

func loadDatabase(path string) ([]uint64, uint64, uint64, uint64) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read database: %v", err)
	}

	if len(data)%DBEntrySize != 0 {
		log.Fatalf("Invalid database file size %d (not multiple of %d)", len(data), DBEntrySize)
	}

	dbEntries := len(data) / DBEntrySize
	dbSize := uint64(dbEntries)

	chunkSize, setSize := derivePlinkoParams(dbSize)
	totalEntries := chunkSize * setSize

	database := make([]uint64, totalEntries*DBEntryLength)
	for i := 0; i < dbEntries; i++ {
		for j := 0; j < DBEntryLength; j++ {
			offset := i*DBEntrySize + j*8
			if offset+8 <= len(data) {
				database[i*DBEntryLength+j] = binary.LittleEndian.Uint64(data[offset : offset+8])
			}
		}
	}

	return database, dbSize, chunkSize, setSize
}

func (s *PlinkoUpdateService) connectToEthereum() error {
	client, err := dialEthereumClient(s.cfg.RPCURL, s.cfg.RPCToken)
	if err != nil {
		return err
	}
	s.client = client

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chainID, err := s.client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch chain ID: %w", err)
	}
	s.chainID = chainID
	return nil
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

	// Detect updates for this block
	updates := s.detectChanges(ctx, blockNumber, header)

	if len(updates) == 0 {
		// No changes detected
		return nil
	}

	// Generate hint deltas using Plinko
	deltas, updateDuration := s.updateManager.ApplyUpdates(updates)
	recordBatch(len(updates), updateDuration)

	// Save delta file
	deltaPath := filepath.Join(s.cfg.DeltaOutputDir, fmt.Sprintf("delta-%06d.bin", blockNumber))
	if err := saveDelta(deltaPath, deltas); err != nil {
		return fmt.Errorf("failed to save delta: %w", err)
	}

	s.deltasGenerated++

	// Log progress
	blockDuration := time.Since(startTime)
	log.Printf("Block %d: %d changes, %d deltas, update: %v, total: %v\n",
		blockNumber, len(updates), len(deltas),
		updateDuration, blockDuration)
	recordBlock(blockNumber, len(updates), blockDuration)

	return nil
}

func (s *PlinkoUpdateService) detectChanges(ctx context.Context, blockNumber uint64, header *types.Header) []DBUpdate {
	if !s.useSimulated {
		return s.detectRPCChanges(ctx, blockNumber, header)
	}

	updates := make([]DBUpdate, ChangesPerBlock)

	// Simulate deterministic changes based on block number
	for i := 0; i < ChangesPerBlock; i++ {
		index := uint64((blockNumber*ChangesPerBlock + uint64(i)) % s.dbSize)

		// Read old value
		oldValue := s.readDBEntry(index)

		// Generate new value (simulated change)
		var newValue DBEntry
		newValue[0] = uint64(blockNumber)*1000 + uint64(i)

		updates[i] = DBUpdate{
			Index:    index,
			OldValue: oldValue,
			NewValue: newValue,
		}
	}

	return updates
}

func (s *PlinkoUpdateService) detectRPCChanges(ctx context.Context, blockNumber uint64, header *types.Header) []DBUpdate {
	if s.client == nil || s.chainID == nil {
		log.Println("Ethereum client not initialized; cannot fetch live updates")
		return nil
	}

	block, err := s.client.BlockByNumber(ctx, new(big.Int).SetUint64(blockNumber))
	if err != nil {
		log.Printf("Failed to load block %d: %v\n", blockNumber, err)
		return nil
	}

	addresses := make(map[string]struct{})
	signer := types.LatestSignerForChainID(s.chainID)

	for _, tx := range block.Transactions() {
		if from, err := types.Sender(signer, tx); err == nil {
			addresses[strings.ToLower(from.Hex())] = struct{}{}
		}
		if to := tx.To(); to != nil {
			addresses[strings.ToLower(to.Hex())] = struct{}{}
		}
	}

	if len(addresses) == 0 {
		return nil
	}

	blockRef := new(big.Int).SetUint64(blockNumber)
	updates := make([]DBUpdate, 0, len(addresses))

	for addrHex := range addresses {
		index, ok := s.addressIndex[addrHex]
		if !ok {
			continue
		}

		balance, err := s.client.BalanceAt(ctx, common.HexToAddress(addrHex), blockRef)
		if err != nil {
			log.Printf("BalanceAt failed for %s: %v\n", addrHex, err)
			continue
		}

		oldValue := s.readDBEntry(index)
		newValue := bigIntToDBEntry(balance)

		// Check if value changed
		changed := false
		for i := 0; i < DBEntryLength; i++ {
			if oldValue[i] != newValue[i] {
				changed = true
				break
			}
		}

		if !changed {
			continue
		}

		updates = append(updates, DBUpdate{
			Index:    index,
			OldValue: oldValue,
			NewValue: newValue,
		})
	}

	return updates
}

func bigIntToDBEntry(b *big.Int) DBEntry {
	var entry DBEntry
	if b == nil {
		return entry
	}
	// Make a copy to not modify original
	val := new(big.Int).Set(b)
	mask := new(big.Int).SetUint64(0xFFFFFFFFFFFFFFFF)

	for i := 0; i < DBEntryLength; i++ {
		word := new(big.Int).And(val, mask)
		entry[i] = word.Uint64()
		val.Rsh(val, 64)
	}
	return entry
}

func (s *PlinkoUpdateService) readDBEntry(index uint64) DBEntry {
	if index >= uint64(len(s.database)/DBEntryLength) {
		return DBEntry{}
	}
	startIdx := index * DBEntryLength
	var entry DBEntry
	for i := 0; i < DBEntryLength; i++ {
		entry[i] = s.database[startIdx+uint64(i)]
	}
	return entry
}

func saveDelta(path string, deltas []HintDelta) error {
	// Create a temporary file in the same directory
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "delta-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := f.Name()
	
	// Clean up temp file on error
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tempPath)
		}
	}()

	// Write delta count
	var header [16]byte
	binary.LittleEndian.PutUint64(header[0:8], uint64(len(deltas)))
	binary.LittleEndian.PutUint64(header[8:16], 0) // Reserved

	if _, err = f.Write(header[:]); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write each delta
	// Format: [8 bytes HintSetID] [8 bytes IsBackupSet] [32 bytes Delta]
	for _, delta := range deltas {
		var entry [48]byte
		binary.LittleEndian.PutUint64(entry[0:8], delta.HintSetID)
		binary.LittleEndian.PutUint64(entry[8:16], boolToUint64(delta.IsBackupSet))

		for i := 0; i < DBEntryLength; i++ {
			binary.LittleEndian.PutUint64(entry[16+i*8:24+i*8], delta.Delta[i])
		}

		if _, err = f.Write(entry[:]); err != nil {
			return fmt.Errorf("failed to write delta entry: %w", err)
		}
	}

	// Ensure data is written to disk
	if err = f.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err = os.Rename(tempPath, path); err != nil {
		// Attempt to remove temp file since rename failed and deferred cleanup
		// might not trigger if err was nil before this block
		os.Remove(tempPath) 
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func (s *PlinkoUpdateService) startHealthServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(s.cfg.DeltaOutputDir); os.IsNotExist(err) {
			http.Error(w, "Delta directory not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","service":"plinko-update","snapshot_version":"%s"}`, s.snapshotVersion)
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshotMetrics())
	})

	log.Printf("Health check server listening on :%s\n", s.cfg.HealthPort)
	if err := http.ListenAndServe(":"+s.cfg.HealthPort, nil); err != nil {
		log.Printf("Health server error: %v\n", err)
	}
}
