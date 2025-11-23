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

type PlinkoQueryRequest struct {
	// P is the list of block indices included in the first partition
	P []uint64 `json:"p"`
	// Offsets is the list of offsets for each block
	Offsets []uint64 `json:"offsets"`
}

type PlinkoQueryResponse struct {
	// R0 is the parity of the blocks in P
	R0 string `json:"r0"`
	// R1 is the parity of the blocks NOT in P
	R1 string `json:"r1"`
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

func (s *PlinkoPIRServer) plinkoQueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PlinkoQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validation
	if uint64(len(req.Offsets)) != s.setSize {
		http.Error(w, "Invalid number of offsets", http.StatusBadRequest)
		return
	}

	startTime := time.Now()
	r0, r1 := s.HandlePlinkoQuery(req.P, req.Offsets)
	elapsed := time.Since(startTime)

	log.Printf("Plinko query completed in %v\n", elapsed)

	resp := PlinkoQueryResponse{
		R0:              r0.String(),
		R1:              r1.String(),
		ServerTimeNanos: uint64(elapsed.Nanoseconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *PlinkoPIRServer) HandlePlinkoQuery(P []uint64, offsets []uint64) (DBEntry, DBEntry) {
	// Convert P slice to map for O(1) lookup
	pMap := make(map[uint64]bool, len(P))
	for _, idx := range P {
		pMap[idx] = true
	}

	var r0, r1 DBEntry
	
	// Iterate through all blocks (0 to setSize-1)
	for i := uint64(0); i < s.setSize; i++ {
		// Calculate database index: block_start + offset
		// The client provides offsets for ALL blocks (including the 'removed' one, handled by client logic? 
		// No, in Plinko, client sends offsets for n/w blocks. Wait.
		// In Fig 7: q <- (P', (oj)j in [n/w]). The list of offsets is full length?
		// Fig 7 Answer: "Parse (P, o0...on/w-1) <- q". Yes, it receives offsets for ALL blocks.
		// The client generates a dummy offset for the removed block if needed, or the protocol handles it.
		// Actually, Fig 7 Query says "q <- (P', (oj)j in [n/w])". P' is P \ {alpha}.
		// The offset o_alpha is included in the query.
		
		offset := offsets[i]
		if offset >= s.chunkSize {
			// Invalid offset for this chunk size, treat as 0 or skip? 
			// Ideally shouldn't happen if client is well-behaved.
			// We'll wrap or clamp to be safe, or just proceed (DBAccess handles OOB)
			offset %= s.chunkSize
		}

		dbIndex := i*s.chunkSize + offset
		entry := s.DBAccess(dbIndex)

		if pMap[i] {
			// If block i is in P, add to r0
			for k := 0; k < DBEntryLength; k++ {
				r0[k] ^= entry[k]
			}
		} else {
			// If block i is NOT in P, add to r1
			for k := 0; k < DBEntryLength; k++ {
				r1[k] ^= entry[k]
			}
		}
	}

	return r0, r1
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
