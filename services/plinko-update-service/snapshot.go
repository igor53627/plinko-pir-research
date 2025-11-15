package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type SnapshotFile struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Comment string `json:"comment,omitempty"`
}

type SnapshotManifest struct {
	Version     string         `json:"version"`
	GeneratedAt time.Time      `json:"generated_at"`
	DBSize      uint64         `json:"db_size"`
	ChunkSize   uint64         `json:"chunk_size"`
	SetSize     uint64         `json:"set_size"`
	Files       []SnapshotFile `json:"files"`
}

func publishSnapshot(cfg Config, dbPath string, dbSize, chunkSize, setSize uint64) (string, error) {
	size, hash, err := hashFile(dbPath)
	if err != nil {
		return "", fmt.Errorf("hash database: %w", err)
	}

	version := cfg.SnapshotVersion
	if version == "" {
		if len(hash) >= 12 {
			version = hash[:12]
		} else {
			version = hash
		}
	}

	snapshotDir := filepath.Join(cfg.PublicSnapshotsDir(), version)
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot dir: %w", err)
	}

	destPath := filepath.Join(snapshotDir, "database.bin")
	if err := copyFile(dbPath, destPath); err != nil {
		return "", fmt.Errorf("copy database to public snapshot: %w", err)
	}

	manifest := SnapshotManifest{
		Version:     version,
		GeneratedAt: time.Now().UTC(),
		DBSize:      dbSize,
		ChunkSize:   chunkSize,
		SetSize:     setSize,
		Files: []SnapshotFile{
			{
				Path:   "database.bin",
				Size:   size,
				SHA256: hash,
			},
		},
	}

	if err := writeJSON(filepath.Join(snapshotDir, "manifest.json"), manifest); err != nil {
		return "", fmt.Errorf("write snapshot manifest: %w", err)
	}

	if err := updateLatestSnapshotSymlink(cfg.PublicSnapshotsDir(), version); err != nil {
		return "", fmt.Errorf("update snapshot symlink: %w", err)
	}

	return version, nil
}

func ensureAddressMappingPublished(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	return copyFile(srcPath, dstPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmpPath := dst + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		return err
	}

	return nil
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

func writeJSON(path string, v interface{}) error {
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

func updateLatestSnapshotSymlink(rootDir, version string) error {
	latestPath := filepath.Join(rootDir, "latest")

	if _, err := os.Lstat(latestPath); err == nil {
		if err := os.Remove(latestPath); err != nil {
			return err
		}
	}

	return os.Symlink(version, latestPath)
}
