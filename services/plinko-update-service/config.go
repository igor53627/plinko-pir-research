package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDatabasePath        = "/data/database.bin"
	defaultAddressMappingPath  = "/data/address-mapping.bin"
	defaultPublicRoot          = "/public"
	defaultDeltaDir            = "/public/deltas"
	defaultHealthPort          = "3001"
	defaultDatabaseWaitTimeout = 120 * time.Second
)

type Config struct {
	DatabasePath        string
	AddressMappingPath  string
	PublicRoot          string
	DeltaOutputDir      string
	SnapshotVersion     string
	HealthPort          string
	DatabaseWaitTimeout time.Duration
	RPCURL              string
	RPCToken            string
	UseSimulated        bool
	IPFSAPI             string
	IPFSGateway         string
}

func LoadConfig() Config {
	cfg := Config{
		DatabasePath:        defaultDatabasePath,
		AddressMappingPath:  defaultAddressMappingPath,
		PublicRoot:          defaultPublicRoot,
		DeltaOutputDir:      defaultDeltaDir,
		SnapshotVersion:     "",
		HealthPort:          defaultHealthPort,
		DatabaseWaitTimeout: defaultDatabaseWaitTimeout,
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_DATABASE_PATH"),
		os.Getenv("DATABASE_PATH"),
	); v != "" {
		cfg.DatabasePath = v
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_ADDRESS_MAPPING_PATH"),
		os.Getenv("ADDRESS_MAPPING_PATH"),
	); v != "" {
		cfg.AddressMappingPath = v
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_PUBLIC_ROOT"),
		os.Getenv("PUBLIC_ROOT"),
	); v != "" {
		cfg.PublicRoot = v
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_DELTA_DIR"),
		os.Getenv("DELTA_DIR"),
	); v != "" {
		cfg.DeltaOutputDir = v
	}

	if v := strings.TrimSpace(os.Getenv("PLINKO_UPDATE_SNAPSHOT_VERSION")); v != "" {
		cfg.SnapshotVersion = v
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_HEALTH_PORT"),
		os.Getenv("HEALTH_PORT"),
	); v != "" {
		cfg.HealthPort = v
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_DATABASE_TIMEOUT_SECONDS"),
		os.Getenv("DATABASE_TIMEOUT_SECONDS"),
	); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds >= 0 {
			cfg.DatabaseWaitTimeout = time.Duration(seconds) * time.Second
		} else {
			log.Printf("Invalid database timeout value %q, using default %v", v, defaultDatabaseWaitTimeout)
		}
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_RPC_URL"),
		os.Getenv("PLINKO_RPC_URL"),
		os.Getenv("RPC_URL"),
	); v != "" {
		cfg.RPCURL = v
	} else {
		cfg.RPCURL = "http://eth-mock:8545"
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_RPC_TOKEN"),
		os.Getenv("PLINKO_RPC_TOKEN"),
	); v != "" {
		cfg.RPCToken = v
	}

	cfg.UseSimulated = true
	if v := firstNonEmpty(
		os.Getenv("PLINKO_UPDATE_SIMULATED"),
		os.Getenv("PLINKO_SIMULATED_UPDATES"),
	); v != "" {
		if parsed, ok := parseBool(v); ok {
			cfg.UseSimulated = parsed
		}
	}

	if v := firstNonEmpty(os.Getenv("PLINKO_STATE_IPFS_API"), os.Getenv("IPFS_API")); v != "" {
		cfg.IPFSAPI = v
	}

	if v := firstNonEmpty(os.Getenv("PLINKO_STATE_IPFS_GATEWAY"), os.Getenv("IPFS_GATEWAY")); v != "" {
		cfg.IPFSGateway = v
	}

	cfg.DatabasePath = strings.TrimSpace(cfg.DatabasePath)
	cfg.AddressMappingPath = strings.TrimSpace(cfg.AddressMappingPath)
	cfg.PublicRoot = strings.TrimSpace(cfg.PublicRoot)
	cfg.DeltaOutputDir = strings.TrimSpace(cfg.DeltaOutputDir)
	cfg.HealthPort = strings.TrimSpace(cfg.HealthPort)
	cfg.RPCURL = strings.TrimSpace(cfg.RPCURL)

	if cfg.DeltaOutputDir == defaultDeltaDir && cfg.PublicRoot != defaultPublicRoot {
		cfg.DeltaOutputDir = filepath.Join(cfg.PublicRoot, "deltas")
	}

	return cfg
}

func (c Config) PublicSnapshotsDir() string {
	if c.PublicRoot == "" {
		return "snapshots"
	}
	return filepath.Join(c.PublicRoot, "snapshots")
}

func (c Config) PublicAddressMappingPath() string {
	if c.PublicRoot == "" {
		return "address-mapping.bin"
	}
	return filepath.Join(c.PublicRoot, "address-mapping.bin")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}
