package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"
)

type SyncMetrics struct {
	mu              sync.RWMutex
	startedAt       time.Time
	mode            string
	blocksSynced    uint64
	lastBlock       uint64
	lastUpdateCount int
	lastDeltaCount  int
	lastDuration    time.Duration
	lastError       string
}

func NewSyncMetrics(simulated bool) *SyncMetrics {
	mode := "rpc"
	if simulated {
		mode = "simulated"
	}
	return &SyncMetrics{
		startedAt: time.Now(),
		mode:      mode,
	}
}

func (m *SyncMetrics) RecordBlock(block uint64, updates, deltas int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocksSynced++
	m.lastBlock = block
	m.lastUpdateCount = updates
	m.lastDeltaCount = deltas
	m.lastDuration = duration
}

func (m *SyncMetrics) RecordError(err error) {
	if err == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastError = err.Error()
}

func (m *SyncMetrics) snapshot() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]any{
		"mode":              m.mode,
		"blocks_synced":     m.blocksSynced,
		"last_block":        m.lastBlock,
		"last_update_count": m.lastUpdateCount,
		"last_delta_count":  m.lastDeltaCount,
		"last_duration_ms":  m.lastDuration.Seconds() * 1000,
		"last_error":        m.lastError,
		"uptime_seconds":    time.Since(m.startedAt).Seconds(),
	}
}

func startMetricsServer(port string, metrics *SyncMetrics) {
	if metrics == nil {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		payload := metrics.snapshot()
		status := "ready"
		if payload["last_error"] != "" {
			status = "degraded"
		}
		payload["status"] = status
		writeJSONResponse(w, payload)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResponse(w, metrics.snapshot())
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("State Syncer metrics listening on :%s\n", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("state syncer metrics server error: %v", err)
	}
}

func writeJSONResponse(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
