package main

import (
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type prfVector struct {
	Index  uint64 `json:"index"`
	RawHex string `json:"raw_hex"`
}

type prfVectorFile struct {
	KeyHex  string      `json:"key_hex"`
	Indices []prfVector `json:"indices"`
}

func loadPRFVectors(t *testing.T) prfVectorFile {
	t.Helper()
	path := filepath.Join("..", "rabby-wallet", "src", "testdata", "prf_vectors.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read PRF vectors: %v", err)
	}

	var vectors prfVectorFile
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("failed to parse PRF vectors: %v", err)
	}

	return vectors
}

func hexToKey(t *testing.T, hexStr string) PrfKey128 {
	t.Helper()
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("failed to decode hex key: %v", err)
	}
	if len(bytes) != 16 {
		t.Fatalf("expected 16-byte key, got %d bytes", len(bytes))
	}
	var key PrfKey128
	copy(key[:], bytes)
	return key
}

func hexToUint64(t *testing.T, hexStr string) uint64 {
	t.Helper()
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("failed to decode hex value: %v", err)
	}
	if len(bytes) != 8 {
		t.Fatalf("expected 8-byte value, got %d bytes", len(bytes))
	}
	return binary.BigEndian.Uint64(bytes)
}

func TestPRFAESVectors(t *testing.T) {
	vectors := loadPRFVectors(t)
	key := hexToKey(t, vectors.KeyHex)
	prSet := NewPRSet(key)

	for _, vec := range vectors.Indices {
		raw := hexToUint64(t, vec.RawHex)

		var input [aes.BlockSize]byte
		binary.BigEndian.PutUint64(input[aes.BlockSize-8:], vec.Index)
		var output [aes.BlockSize]byte
		prSet.block.Encrypt(output[:], input[:])
		gotRaw := binary.BigEndian.Uint64(output[:8])
		if gotRaw != raw {
			t.Fatalf("raw AES mismatch for index %d: got %016x want %016x", vec.Index, gotRaw, raw)
		}

		gotMod := prSet.prfEvalMod(vec.Index, 8192)
		if gotMod != raw%8192 {
			t.Fatalf("mod mismatch for index %d: got %d want %d", vec.Index, gotMod, raw%8192)
		}
	}
}

func TestExpandMatchesDirectEvaluation(t *testing.T) {
	vectors := loadPRFVectors(t)
	key := hexToKey(t, vectors.KeyHex)
	prSet := NewPRSet(key)

	const setSize = 16
	const chunkSize = uint64(8192)

	indices := prSet.Expand(setSize, chunkSize)
	if len(indices) != setSize {
		t.Fatalf("expected %d indices, got %d", setSize, len(indices))
	}

	for i := uint64(0); i < setSize; i++ {
		expected := i*chunkSize + prSet.prfEvalMod(i, chunkSize)
		if indices[i] != expected {
			t.Fatalf("expand mismatch at position %d: got %d want %d", i, indices[i], expected)
		}
	}
}
