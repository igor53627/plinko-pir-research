package main

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	DBEntrySize   = 32
	DBEntryLength = 4
)

type DBEntry [DBEntryLength]uint64

type PlinkoPIRServer struct {
	database  []uint64
	dbSize    uint64
	chunkSize uint64
	setSize   uint64
}

type PlaintextQueryRequest struct {
	Index uint64 `json:"index"`
}

type PlaintextQueryResponse struct {
	Value           string `json:"value"`
	ServerTimeNanos uint64 `json:"server_time_nanos"`
}

type FullSetQueryRequest struct {
	PRFKey []byte `json:"prf_key"`
}

type FullSetQueryResponse struct {
	Value           string `json:"value"`
	ServerTimeNanos uint64 `json:"server_time_nanos"`
}

type SetParityQueryRequest struct {
	Indices []uint64 `json:"indices"`
}

type SetParityQueryResponse struct {
	Parity          string `json:"parity"`
	ServerTimeNanos uint64 `json:"server_time_nanos"`
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		w.Header().Set("Access-Control-Max-Age", "3600")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func loadServer(databasePath string) *PlinkoPIRServer {
	data, err := os.ReadFile(databasePath)
	if err != nil {
		log.Fatalf("Failed to read database file %s: %v", databasePath, err)
	}

	if len(data)%DBEntrySize != 0 {
		log.Fatalf("Invalid database file: size %d is not a multiple of %d", len(data), DBEntrySize)
	}

	entryCount := len(data) / DBEntrySize
	if entryCount == 0 {
		log.Fatal("Invalid database file: contains zero entries")
	}

	dbSize := uint64(entryCount)
	chunkSize, setSize := derivePlinkoParams(dbSize)
	totalEntries := chunkSize * setSize

	// database slice holds flattened uint64 words
	database := make([]uint64, totalEntries*DBEntryLength)

	for i := 0; i < entryCount; i++ {
		for j := 0; j < DBEntryLength; j++ {
			offset := i*DBEntrySize + j*8
			if offset+8 <= len(data) {
				database[i*DBEntryLength+j] = binary.LittleEndian.Uint64(data[offset : offset+8])
			}
		}
	}

	return &PlinkoPIRServer{
		database:  database,
		dbSize:    dbSize,
		chunkSize: chunkSize,
		setSize:   setSize,
	}
}

func (s *PlinkoPIRServer) DBAccess(id uint64) DBEntry {
	if id < uint64(len(s.database)/DBEntryLength) {
		startIdx := id * DBEntryLength
		var entry DBEntry
		for i := 0; i < DBEntryLength; i++ {
			entry[i] = s.database[startIdx+uint64(i)]
		}
		return entry
	}
	return DBEntry{}
}

func (s *PlinkoPIRServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "healthy",
		"service":    "plinko-pir-server",
		"db_size":    s.dbSize,
		"chunk_size": s.chunkSize,
		"set_size":   s.setSize,
		"entry_size": DBEntrySize,
	})
}

func (s *PlinkoPIRServer) plaintextQueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PlaintextQueryRequest

	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
	} else {
		indexStr := r.URL.Query().Get("index")
		if indexStr == "" {
			http.Error(w, "Missing index parameter", http.StatusBadRequest)
			return
		}
		index, err := strconv.ParseUint(indexStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid index", http.StatusBadRequest)
			return
		}
		req.Index = index
	}

	startTime := time.Now()
	entry := s.DBAccess(req.Index)
	elapsed := time.Since(startTime)

	resp := PlaintextQueryResponse{
		Value:           entry.String(),
		ServerTimeNanos: uint64(elapsed.Nanoseconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *PlinkoPIRServer) fullSetQueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FullSetQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if len(req.PRFKey) != 16 {
		http.Error(w, "PRF key must be 16 bytes", http.StatusBadRequest)
		return
	}

	log.Println("========================================")
	log.Println("🔒 PRIVATE QUERY RECEIVED")
	log.Println("========================================")
	log.Printf("Server sees: PRF Key (16 bytes): %x\n", req.PRFKey[:8])
	log.Println("Server CANNOT determine:")
	log.Println("  ❌ Which address is being queried")
	log.Println("  ❌ Which balance is being requested")
	log.Println("  ❌ Any user information")
	log.Println("Server will compute parity over ~1024 database entries...")
	log.Println("========================================")

	startTime := time.Now()
	parity := s.HandleFullSetQuery(req.PRFKey)
	elapsed := time.Since(startTime)

	log.Printf("✅ FullSet query completed in %v\n", elapsed)
	log.Printf("Server response: Parity value: %s\n", parity.String())
	log.Println("Server remains oblivious to queried address!")
	log.Println("========================================")
	log.Println()

	resp := FullSetQueryResponse{
		Value:           parity.String(),
		ServerTimeNanos: uint64(elapsed.Nanoseconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *PlinkoPIRServer) HandleFullSetQuery(prfKeyBytes []byte) DBEntry {
	var prfKey PrfKey128
	copy(prfKey[:], prfKeyBytes)

	prSet := NewPRSet(prfKey)
	expandedSet := prSet.Expand(s.setSize, s.chunkSize)

	var parity DBEntry
	for _, id := range expandedSet {
		entry := s.DBAccess(id)
		for k := 0; k < DBEntryLength; k++ {
			parity[k] ^= entry[k]
		}
	}

	return parity
}

func (s *PlinkoPIRServer) setParityQueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SetParityQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	startTime := time.Now()
	parity := s.HandleSetParityQuery(req.Indices)
	elapsed := time.Since(startTime)

	log.Printf("SetParity query (%d indices) completed in %v\n", len(req.Indices), elapsed)

	resp := SetParityQueryResponse{
		Parity:          parity.String(),
		ServerTimeNanos: uint64(elapsed.Nanoseconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *PlinkoPIRServer) HandleSetParityQuery(indices []uint64) DBEntry {
	var parity DBEntry
	for _, index := range indices {
		entry := s.DBAccess(index)
		for k := 0; k < DBEntryLength; k++ {
			parity[k] ^= entry[k]
		}
	}
	return parity
}

// String returns the decimal string representation of the 256-bit integer
func (e DBEntry) String() string {
	// Convert [4]uint64 (little-endian) to big.Int
	val := new(big.Int)
	for i := 0; i < DBEntryLength; i++ {
		word := new(big.Int).SetUint64(e[i])
		word.Lsh(word, uint(i*64))
		val.Add(val, word)
	}
	return val.String()
}
