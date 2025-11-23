package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Config struct {
	DatabasePath       string
	AddressMappingPath string
	PublicRoot         string
	DeltaDir           string
	HTTPPort           string
	IPFSAPI            string
	IPFSGateway        string
	RPCURL             string
	RPCToken           string
	StartBlock         uint64
	Simulated          bool
	PollInterval       time.Duration
	SnapshotEvery      uint64
}

func LoadConfig() Config {
	cfg := Config{
		DatabasePath:       getEnv("PLINKO_STATE_DB_PATH", "/data/database.bin"),
		AddressMappingPath: getEnv("PLINKO_STATE_ADDRESS_MAPPING_PATH", "/data/address-mapping.bin"),
		PublicRoot:         getEnv("PLINKO_STATE_PUBLIC_ROOT", "/public"),
		DeltaDir:           getEnv("PLINKO_STATE_DELTA_DIR", "/public/deltas"),
		HTTPPort:           getEnv("PLINKO_STATE_HTTP_PORT", "3002"),
		IPFSAPI:            strings.TrimSpace(os.Getenv("PLINKO_STATE_IPFS_API")),
		IPFSGateway:        getEnv("PLINKO_STATE_IPFS_GATEWAY", "http://localhost:8080/ipfs"),
		RPCURL:             getEnv("PLINKO_STATE_RPC_URL", "http://eth-mock:8545"),
		RPCToken:           os.Getenv("PLINKO_STATE_RPC_TOKEN"),
		Simulated:          getEnvBool("PLINKO_STATE_SIMULATED", true),
		PollInterval:       getEnvDuration("PLINKO_STATE_POLL_INTERVAL", 5*time.Second),
		SnapshotEvery:      getEnvUint("PLINKO_STATE_SNAPSHOT_EVERY", 0),
	}
	if start := strings.TrimSpace(os.Getenv("PLINKO_STATE_START_BLOCK")); start != "" {
		if val, err := strconv.ParseUint(start, 10, 64); err == nil {
			cfg.StartBlock = val
		}
	}
	cfg.IPFSAPI = strings.TrimSpace(cfg.IPFSAPI)
	cfg.IPFSGateway = strings.TrimRight(strings.TrimSpace(cfg.IPFSGateway), "/")
	return cfg
}

func (c Config) SnapshotsRoot() string {
	if c.PublicRoot == "" {
		return "snapshots"
	}
	return filepath.Join(c.PublicRoot, "snapshots")
}

func (c Config) LatestSnapshotLink() string {
	return filepath.Join(c.SnapshotsRoot(), "latest")
}

func (c Config) PublicAddressMappingPath() string {
	if c.PublicRoot == "" {
		return "address-mapping.bin"
	}
	return filepath.Join(c.PublicRoot, "address-mapping.bin")
}

func main() {
	cfg := LoadConfig()
	log.Printf("State Syncer starting (rpc=%s, simulated=%v)\n", cfg.RPCURL, cfg.Simulated)

	metrics := NewSyncMetrics(cfg.Simulated)
	go startMetricsServer(cfg.HTTPPort, metrics)

	ipfsPublisher, err := newIPFSPublisher(cfg.IPFSAPI, cfg.IPFSGateway)
	if err != nil {
		log.Fatalf("ipfs publisher init: %v", err)
	}
	if ipfsPublisher != nil {
		log.Printf("IPFS publishing enabled (api=%s)\n", cfg.IPFSAPI)
	}

	addressIndex, err := loadAddressMapping(cfg.AddressMappingPath)
	if err != nil {
		log.Fatalf("load address mapping: %v", err)
	}
	log.Printf("Loaded %d address mappings\n", len(addressIndex))

	db, dbSize, chunkSize, setSize, err := loadDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("load database: %v", err)
	}

	manager := NewPlinkoUpdateManager(db, dbSize, chunkSize, setSize)
	manager.EnableCacheMode()

	if err := os.MkdirAll(cfg.DeltaDir, 0o755); err != nil {
		log.Fatalf("create delta dir: %v", err)
	}
	if err := os.MkdirAll(cfg.SnapshotsRoot(), 0o755); err != nil {
		log.Fatalf("create snapshot dir: %v", err)
	}

	bundler := NewDeltaBundler(cfg, ipfsPublisher)

	if err := ensureAddressMappingPublished(cfg.AddressMappingPath, cfg.PublicAddressMappingPath()); err != nil {
		log.Fatalf("publish address-mapping: %v", err)
	}

	var client *ethclient.Client
	var chainID *big.Int
	if !cfg.Simulated {
		client, err = dialEthereumClient(cfg.RPCURL, cfg.RPCToken)
		if err != nil {
			log.Fatalf("dial rpc: %v", err)
		}
		defer client.Close()
		chainID, err = client.ChainID(context.Background())
		if err != nil {
			log.Fatalf("read chain id: %v", err)
		}
	}

	if version, err := writeSnapshot(cfg, db, dbSize, cfg.StartBlock, chunkSize, setSize, ipfsPublisher); err != nil {
		log.Printf("initial snapshot error: %v", err)
		metrics.RecordError(err)
	} else {
		log.Printf("Published snapshot %s\n", version)
	}

	lastBlock := cfg.StartBlock
	for {
		nextBlock := lastBlock + 1

		if !cfg.Simulated {
			head, err := client.BlockNumber(context.Background())
			if err != nil {
				log.Printf("blockNumber error: %v", err)
				metrics.RecordError(err)
				time.Sleep(cfg.PollInterval)
				continue
			}
			if head < nextBlock {
				time.Sleep(cfg.PollInterval)
				continue
			}
		}

		var updates []DBUpdate
		if cfg.Simulated {
			updates = simulateUpdates(db, dbSize, nextBlock)
		} else {
			updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			updates = fetchUpdates(updateCtx, client, chainID, addressIndex, db, nextBlock)
			cancel()
		}

		if len(updates) == 0 {
			lastBlock = nextBlock
			continue
		}

		deltas, duration := manager.ApplyUpdates(updates)
		log.Printf("block %d: %d updates, %d deltas (%s)\n", nextBlock, len(updates), len(deltas), duration)

		if err := flushDatabase(cfg.DatabasePath, db, dbSize); err != nil {
			log.Printf("flush database failed: %v", err)
			metrics.RecordError(err)
		}

		deltaPath := filepath.Join(cfg.DeltaDir, fmt.Sprintf("delta-%06d.bin", nextBlock))
		if err := saveDelta(deltaPath, deltas); err != nil {
			log.Printf("save delta failed: %v", err)
			metrics.RecordError(err)
		} else {
			if err := bundler.PublishDelta(nextBlock, deltaPath); err != nil {
				log.Printf("bundler error: %v", err)
			}
		}

		if cfg.SnapshotEvery > 0 && nextBlock%cfg.SnapshotEvery == 0 {
			if _, err := writeSnapshot(cfg, db, dbSize, nextBlock, chunkSize, setSize, ipfsPublisher); err != nil {
				log.Printf("snapshot error: %v", err)
				metrics.RecordError(err)
			}
		}

		metrics.RecordBlock(nextBlock, len(updates), len(deltas), duration)
		lastBlock = nextBlock
	}
}

func loadAddressMapping(path string) (map[string]uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read address-mapping: %w", err)
	}
	if len(data)%24 != 0 {
		return nil, fmt.Errorf("address-mapping size %d invalid", len(data))
	}

	mapping := make(map[string]uint64, len(data)/24)
	for i := 0; i < len(data); i += 24 {
		addr := common.BytesToAddress(data[i : i+20]).Hex()
		index := binary.LittleEndian.Uint32(data[i+20 : i+24])
		mapping[strings.ToLower(addr)] = uint64(index)
	}
	return mapping, nil
}

func loadDatabase(path string) ([]uint64, uint64, uint64, uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("read database: %w", err)
	}
	if len(data)%DBEntrySize != 0 {
		return nil, 0, 0, 0, fmt.Errorf("database size %d invalid (not multiple of %d)", len(data), DBEntrySize)
	}
	dbEntries := uint64(len(data) / DBEntrySize)
	chunkSize, setSize := derivePlinkoParams(dbEntries)
	totalEntries := chunkSize * setSize

	database := make([]uint64, totalEntries*DBEntryLength)
	for i := uint64(0); i < dbEntries; i++ {
		for j := 0; j < DBEntryLength; j++ {
			offset := i*DBEntrySize + uint64(j)*8
			database[i*DBEntryLength+uint64(j)] = binary.LittleEndian.Uint64(data[offset : offset+8])
		}
	}

	return database, dbEntries, chunkSize, setSize, nil
}

func flushDatabase(path string, db []uint64, dbSize uint64) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close() // Ensure file is closed even on error

	buf := make([]byte, DBEntrySize)
	limit := dbSize * DBEntryLength // Iterate over actual uint64 words in the flattened array

	if limit > uint64(len(db)) {
		limit = uint64(len(db))
	}

	for i := uint64(0); i < dbSize; i++ {
		for j := 0; j < DBEntryLength; j++ {
			binary.LittleEndian.PutUint64(buf[j*8:(j+1)*8], db[i*DBEntryLength+uint64(j)])
		}
		if _, err := f.Write(buf); err != nil {
			return err
		}
	}

	// Ensure all data is written to disk before renaming
	if err := f.Sync(); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

func saveDelta(path string, deltas []HintDelta) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close() // Ensure file is closed even on error

	var header [16]byte
	binary.LittleEndian.PutUint64(header[0:8], uint64(len(deltas)))
	// Use 8 bytes for DBEntryLength (4), so header is still 16 bytes for future compatibility
	binary.LittleEndian.PutUint64(header[8:16], uint64(DBEntryLength))

	if _, err := f.Write(header[:]); err != nil {
		return err
	}

	// Each delta record is: HintSetID (8 bytes) + IsBackupSet (8 bytes) + Delta (32 bytes = 4 * uint64)
	// Total size: 8 + 8 + 32 = 48 bytes
	var buf [48]byte

	for _, delta := range deltas {
		binary.LittleEndian.PutUint64(buf[0:8], delta.HintSetID)
		if delta.IsBackupSet {
			binary.LittleEndian.PutUint64(buf[8:16], 1)
		} else {
			binary.LittleEndian.PutUint64(buf[8:16], 0)
		}
		// Write the 4 uint64 words of the DBEntry
		for i := 0; i < DBEntryLength; i++ {
			binary.LittleEndian.PutUint64(buf[16+i*8:16+(i+1)*8], delta.Delta[i])
		}
		if _, err := f.Write(buf[:]); err != nil {
			return err
		}
	}
	// Ensure all data is written to disk before renaming
	if err := f.Sync(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func simulateUpdates(db []uint64, dbSize uint64, block uint64) []DBUpdate {
	if dbSize == 0 {
		return nil
	}
	updates := make([]DBUpdate, simulatedUpdatesPerBlock)
	for i := 0; i < simulatedUpdatesPerBlock; i++ {
		index := uint64((block*uint64(simulatedUpdatesPerBlock) + uint64(i)) % dbSize)
		oldValue := DBEntry{db[index]}
		newValue := DBEntry{uint64(block)*1000 + uint64(i)}
		updates[i] = DBUpdate{
			Index:    index,
			OldValue: oldValue,
			NewValue: newValue,
		}
	}
	return updates
}

func fetchUpdates(ctx context.Context, client *ethclient.Client, chainID *big.Int, indexMap map[string]uint64, db []uint64, blockNumber uint64) []DBUpdate {
	block, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(blockNumber))
	if err != nil {
		log.Printf("BlockByNumber error: %v", err)
		return nil
	}

	signer := types.LatestSignerForChainID(chainID)
	addresses := make(map[string]struct{})
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

	results := make([]DBUpdate, 0, len(addresses))
	for addrHex := range addresses {
		idx, ok := indexMap[addrHex]
		if !ok {
			continue
		}
		balance, err := client.BalanceAt(ctx, common.HexToAddress(addrHex), new(big.Int).SetUint64(blockNumber))
		if err != nil {
			log.Printf("BalanceAt error for %s: %v", addrHex, err)
			continue
		}
		oldValue := DBEntry{db[idx]}
		newValue := DBEntry{balance.Uint64()}
		if oldValue[0] == newValue[0] {
			continue
		}
		results = append(results, DBUpdate{
			Index:    idx,
			OldValue: oldValue,
			NewValue: newValue,
		})
	}
	return results
}

type SnapshotFile struct {
	Path   string            `json:"path"`
	Size   int64             `json:"size"`
	SHA256 string            `json:"sha256"`
	IPFS   *SnapshotFileIPFS `json:"ipfs,omitempty"`
}

type SnapshotFileIPFS struct {
	CID        string `json:"cid"`
	GatewayURL string `json:"gateway_url,omitempty"`
}

type SnapshotManifest struct {
	Version     string         `json:"version"`
	Block       uint64         `json:"block"`
	GeneratedAt time.Time      `json:"generated_at"`
	DBSize      uint64         `json:"db_size"`
	ChunkSize   uint64         `json:"chunk_size"`
	SetSize     uint64         `json:"set_size"`
	Files       []SnapshotFile `json:"files"`
}

func writeSnapshot(cfg Config, db []uint64, dbSize, block, chunkSize, setSize uint64, publisher *IPFSPublisher) (string, error) {
	version := fmt.Sprintf("block-%06d", block)
	dir := filepath.Join(cfg.SnapshotsRoot(), version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "database.bin")
	if err := flushDatabase(path, db, dbSize); err != nil {
		return "", err
	}

	size, hash, err := hashFile(path)
	if err != nil {
		return "", err
	}

	fileEntry := SnapshotFile{
		Path:   "database.bin",
		Size:   size,
		SHA256: hash,
	}

	if publisher != nil {
		if cid, err := publisher.PublishFile(path); err != nil {
			log.Printf("ipfs publish (database.bin) failed: %v", err)
		} else {
			fileEntry.IPFS = &SnapshotFileIPFS{
				CID:        cid,
				GatewayURL: publisher.GatewayURL(cid),
			}
			log.Printf("Pinned database.bin to IPFS CID %s\n", cid)
		}
	}

	manifest := SnapshotManifest{
		Version:     version,
		Block:       block,
		GeneratedAt: time.Now().UTC(),
		DBSize:      dbSize,
		ChunkSize:   chunkSize,
		SetSize:     setSize,
		Files:       []SnapshotFile{fileEntry},
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return "", err
	}
	if publisher != nil {
		if cid, err := publisher.PublishFile(manifestPath); err != nil {
			log.Printf("ipfs publish (manifest.json) failed: %v", err)
		} else {
			log.Printf("Pinned manifest.json to IPFS CID %s\n", cid)
		}
	}
	if err := updateLatestSnapshotSymlink(cfg.SnapshotsRoot(), version); err != nil {
		return "", err
	}
	return version, nil
}

func ensureAddressMappingPublished(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func hashFile(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(h.Sum(nil)), nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func updateLatestSnapshotSymlink(root, version string) error {
	latest := filepath.Join(root, "latest")
	if _, err := os.Lstat(latest); err == nil {
		if err := os.Remove(latest); err != nil {
			return err
		}
	}
	return os.Symlink(version, latest)
}

func dialEthereumClient(url, token string) (*ethclient.Client, error) {
	if token == "" || !strings.HasPrefix(url, "http") {
		return ethclient.Dial(url)
	}
	httpClient := &http.Client{
		Transport: &authTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}
	rpcClient, err := rpc.DialHTTPWithClient(url, httpClient)
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(rpcClient), nil
}

type authTransport struct {
	token string
	base  http.RoundTripper
}

func (a *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}
	return a.base.RoundTrip(req)
}

func getEnv(key, def string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	switch strings.ToLower(val) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	if d, err := time.ParseDuration(val); err == nil {
		return d
	}
	return def
}

func getEnvUint(key string, def uint64) uint64 {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
		return parsed
	}
	return def
}

func derivePlinkoParams(dbEntries uint64) (uint64, uint64) {
	if dbEntries == 0 {
		return 1, 1
	}
	targetChunk := uint64(2 * math.Sqrt(float64(dbEntries)))
	chunkSize := uint64(1)
	for chunkSize < targetChunk {
		chunkSize *= 2
	}
	setSize := uint64(math.Ceil(float64(dbEntries) / float64(chunkSize)))
	setSize = (setSize + 3) / 4 * 4
	return chunkSize, setSize
}

const simulatedUpdatesPerBlock = 2000
