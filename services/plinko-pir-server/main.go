package main

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	// Server configuration
	ServerPort = "3000"
	HintPath   = "/data/hint.bin"

	// Database configuration
	DBEntrySize   = 8
	DBEntryLength = 1 // DBEntrySize / 8
)

type DBEntry [DBEntryLength]uint64

type PlinkoPIRServer struct {
	database  []uint64 // In-memory database
	dbSize    uint64   // Number of database entries
	chunkSize uint64   // Plinko PIR chunk size
	setSize   uint64   // Plinko PIR set size
}

// Query request/response types
type PlaintextQueryRequest struct {
	Index uint64 `json:"index"`
}

type PlaintextQueryResponse struct {
	Value           uint64 `json:"value"`
	ServerTimeNanos uint64 `json:"server_time_nanos"`
}

type FullSetQueryRequest struct {
	PRFKey []byte `json:"prf_key"` // 16-byte PRF key
}

type FullSetQueryResponse struct {
	Value           uint64 `json:"value"`
	ServerTimeNanos uint64 `json:"server_time_nanos"`
}

type SetParityQueryRequest struct {
	Indices []uint64 `json:"indices"` // Set of database indices
}

type SetParityQueryResponse struct {
	Parity          uint64 `json:"parity"`
	ServerTimeNanos uint64 `json:"server_time_nanos"`
}

// CORS middleware to enable cross-origin requests from the browser
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next(w, r)
	}
}

func main() {
	log.Println("========================================")
	log.Println("Plinko PIR Server")
	log.Println("========================================")
	log.Println()

	// Wait for hint.bin
	waitForHint()

	// Load database
	log.Println("Loading database from hint.bin...")
	server := loadServer()
	log.Printf("✅ Database loaded: %d entries (%d MB)\n",
		server.dbSize, server.dbSize*DBEntrySize/1024/1024)
	log.Printf("   ChunkSize: %d, SetSize: %d\n", server.chunkSize, server.setSize)
	log.Println()

	// Setup HTTP handlers with CORS middleware
	http.HandleFunc("/health", corsMiddleware(server.healthHandler))
	http.HandleFunc("/query/plaintext", corsMiddleware(server.plaintextQueryHandler))
	http.HandleFunc("/query/fullset", corsMiddleware(server.fullSetQueryHandler))
	http.HandleFunc("/query/setparity", corsMiddleware(server.setParityQueryHandler))

	// Start server
	addr := ":" + ServerPort
	log.Printf("🚀 Plinko PIR Server listening on %s\n", addr)
	log.Println("========================================")
	log.Println()
	log.Println("Privacy Mode: ENABLED")
	log.Println("⚠️  Server will NEVER log queried addresses")
	log.Println()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
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

func loadServer() *PlinkoPIRServer {
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

	// Extract database (skip 32-byte header)
	dbBytes := data[32:]
	dbEntries := len(dbBytes) / DBEntrySize

	database := make([]uint64, dbEntries)
	for i := 0; i < dbEntries; i++ {
		database[i] = binary.LittleEndian.Uint64(dbBytes[i*DBEntrySize : (i+1)*DBEntrySize])
	}

	return &PlinkoPIRServer{
		database:  database,
		dbSize:    dbSize,
		chunkSize: chunkSize,
		setSize:   setSize,
	}
}

// DBAccess safely accesses database entry by index
func (s *PlinkoPIRServer) DBAccess(id uint64) DBEntry {
	if id < uint64(len(s.database)/DBEntryLength) {
		startIdx := id * DBEntryLength
		return DBEntry{s.database[startIdx]}
	}
	// Return zero for out-of-bounds
	return DBEntry{0}
}

// healthHandler returns server health status
func (s *PlinkoPIRServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "healthy",
		"service":    "plinko-pir-server",
		"db_size":    s.dbSize,
		"chunk_size": s.chunkSize,
		"set_size":   s.setSize,
	})
}

// plaintextQueryHandler handles direct database lookups (for testing)
// ⚠️  Privacy: Does NOT log the queried index
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
		// GET request: parse index from query parameter
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

	// Execute query
	startTime := time.Now()
	entry := s.DBAccess(req.Index)
	elapsed := time.Since(startTime)

	// ⚠️  PRIVACY: Never log the queried index!
	// log.Printf("Query completed in %v\n", elapsed) // OK - no index
	// log.Printf("Query for index %d\n", req.Index) // NEVER DO THIS!

	// Return response
	resp := PlaintextQueryResponse{
		Value:           entry[0],
		ServerTimeNanos: uint64(elapsed.Nanoseconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// fullSetQueryHandler handles Plinko PIR FullSet queries
// ⚠️  Privacy: Query is information-theoretically private
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

	// Validate PRF key
	if len(req.PRFKey) != 16 {
		http.Error(w, "PRF key must be 16 bytes", http.StatusBadRequest)
		return
	}

	// Log what server sees (privacy-preserving)
	log.Println("========================================")
	log.Println("🔒 PRIVATE QUERY RECEIVED")
	log.Println("========================================")
	log.Printf("Server sees: PRF Key (16 bytes): %x\n", req.PRFKey[:8]) // Show first 8 bytes
	log.Println("Server CANNOT determine:")
	log.Println("  ❌ Which address is being queried")
	log.Println("  ❌ Which balance is being requested")
	log.Println("  ❌ Any user information")
	log.Println("Server will compute parity over ~1024 database entries...")
	log.Println("========================================")

	// Execute Plinko PIR FullSet query
	startTime := time.Now()
	parity := s.HandleFullSetQuery(req.PRFKey)
	elapsed := time.Since(startTime)

	// Log query completion without revealing content
	log.Printf("✅ FullSet query completed in %v\n", elapsed)
	log.Printf("Server response: Parity value (uint64): %d\n", parity[0])
	log.Println("Server remains oblivious to queried address!")
	log.Println("========================================")
	log.Println()

	resp := FullSetQueryResponse{
		Value:           parity[0],
		ServerTimeNanos: uint64(elapsed.Nanoseconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleFullSetQuery implements Plinko PIR FullSet query
func (s *PlinkoPIRServer) HandleFullSetQuery(prfKeyBytes []byte) DBEntry {
	// Convert PRF key
	var prfKey PrfKey128
	copy(prfKey[:], prfKeyBytes)

	// Expand PRF key to set of indices
	prSet := NewPRSet(prfKey)
	expandedSet := prSet.Expand(s.setSize, s.chunkSize)

	// Compute XOR parity over the set
	var parity DBEntry
	for _, id := range expandedSet {
		entry := s.DBAccess(id)
		parity[0] ^= entry[0]
	}

	return parity
}

// setParityQueryHandler handles SetParity queries (simplified Plinko PIR)
// ⚠️  Privacy: Does not log which indices were queried
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

	// Execute query
	startTime := time.Now()
	parity := s.HandleSetParityQuery(req.Indices)
	elapsed := time.Since(startTime)

	// Log query completion (count only, never the indices!)
	log.Printf("SetParity query (%d indices) completed in %v\n",
		len(req.Indices), elapsed)

	resp := SetParityQueryResponse{
		Parity:          parity[0],
		ServerTimeNanos: uint64(elapsed.Nanoseconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSetParityQuery computes XOR parity over a set of indices
func (s *PlinkoPIRServer) HandleSetParityQuery(indices []uint64) DBEntry {
	var parity DBEntry
	for _, index := range indices {
		entry := s.DBAccess(index)
		parity[0] ^= entry[0]
	}
	return parity
}
