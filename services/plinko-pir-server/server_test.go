package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestDerivePlinkoParams(t *testing.T) {
	tests := []struct {
		name        string
		dbEntries   uint64
		wantChunk   uint64
		wantSetSize uint64
	}{
		{"small_db", 16, 8, 4},
		{"non_power_of_two", 23, 16, 4},
		{"large_db", 8388608, 8192, 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, setSize := derivePlinkoParams(tt.dbEntries)
			if chunk != tt.wantChunk {
				t.Fatalf("chunk size mismatch: got %d want %d", chunk, tt.wantChunk)
			}
			if setSize != tt.wantSetSize {
				t.Fatalf("set size mismatch: got %d want %d", setSize, tt.wantSetSize)
			}
		})
	}
}

func TestLoadServerFromDatabase(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "database.bin")

	var data []byte
	for i := uint64(0); i < 10; i++ {
		// Create a 32-byte entry (4 uint64s). 
		// We put the index in the first uint64 and zeros in others.
		var entry [32]byte
		binary.LittleEndian.PutUint64(entry[0:], i+1)
		data = append(data, entry[:]...)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed writing db file: %v", err)
	}

	server := loadServer(path)

	if server.dbSize != 10 {
		t.Fatalf("dbSize mismatch: got %d want 10", server.dbSize)
	}

	expectedChunk, expectedSet := derivePlinkoParams(10)
	if server.chunkSize != expectedChunk {
		t.Fatalf("chunkSize mismatch: got %d want %d", server.chunkSize, expectedChunk)
	}
	if server.setSize != expectedSet {
		t.Fatalf("setSize mismatch: got %d want %d", server.setSize, expectedSet)
	}

	totalEntries := expectedChunk * expectedSet
	// server.database stores totalEntries * DBEntryLength uint64 words.
	if uint64(len(server.database)) != totalEntries*DBEntryLength {
		t.Fatalf("database length mismatch: got %d want %d", len(server.database), totalEntries*DBEntryLength)
	}

	// Check padding entries (starting after the valid entries)
	validWords := int(server.dbSize) * DBEntryLength
	for i := validWords; i < len(server.database); i++ {
		if server.database[i] != 0 {
			t.Fatalf("padding entry word %d expected zero got %d", i, server.database[i])
		}
	}
}
