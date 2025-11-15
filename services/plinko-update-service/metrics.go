package main

import (
	"sync/atomic"
	"time"
)

type updateMetrics struct {
	totalBatches      atomic.Int64
	totalUpdates      atomic.Int64
	totalBatchNanos   atomic.Int64
	totalBlocks       atomic.Int64
	totalBlockNanos   atomic.Int64
	lastBlockNumber   atomic.Uint64
	lastUpdatedNanos  atomic.Int64
	lastBlockDuration atomic.Int64
	lastBatchSize     atomic.Int64
}

var metricsCollector updateMetrics

func recordBatch(batchSize int, duration time.Duration) {
	if batchSize <= 0 {
		return
	}
	metricsCollector.totalBatches.Add(1)
	metricsCollector.totalUpdates.Add(int64(batchSize))
	metricsCollector.totalBatchNanos.Add(duration.Nanoseconds())
	metricsCollector.lastBatchSize.Store(int64(batchSize))
	metricsCollector.lastUpdatedNanos.Store(time.Now().UnixNano())
}

func recordBlock(blockNumber uint64, updates int, duration time.Duration) {
	metricsCollector.totalBlocks.Add(1)
	metricsCollector.totalBlockNanos.Add(duration.Nanoseconds())
	metricsCollector.lastBlockNumber.Store(blockNumber)
	metricsCollector.lastBlockDuration.Store(duration.Nanoseconds())
	if updates > 0 {
		metricsCollector.totalUpdates.Add(int64(updates))
	}
	metricsCollector.lastUpdatedNanos.Store(time.Now().UnixNano())
}

type metricsSnapshot struct {
	TotalBatches     int64   `json:"total_batches"`
	TotalUpdates     int64   `json:"total_updates"`
	AvgBatchMicros   float64 `json:"avg_batch_micros"`
	LastBatchSize    int64   `json:"last_batch_size"`
	TotalBlocks      int64   `json:"total_blocks"`
	AvgBlockMillis   float64 `json:"avg_block_millis"`
	LastBlockNumber  uint64  `json:"last_block_number"`
	LastBlockMillis  float64 `json:"last_block_millis"`
	LastUpdatedRFC33 string  `json:"last_updated"`
}

func snapshotMetrics() metricsSnapshot {
	batches := metricsCollector.totalBatches.Load()
	updates := metricsCollector.totalUpdates.Load()
	batchNanos := metricsCollector.totalBatchNanos.Load()
	blocks := metricsCollector.totalBlocks.Load()
	blockNanos := metricsCollector.totalBlockNanos.Load()

	var avgBatchMicros float64
	if batches > 0 {
		avgBatchMicros = float64(batchNanos) / float64(batches) / 1e3
	}

	var avgBlockMillis float64
	if blocks > 0 {
		avgBlockMillis = float64(blockNanos) / float64(blocks) / 1e6
	}

	lastUpdated := time.Unix(0, metricsCollector.lastUpdatedNanos.Load()).UTC()

	return metricsSnapshot{
		TotalBatches:     batches,
		TotalUpdates:     updates,
		AvgBatchMicros:   avgBatchMicros,
		LastBatchSize:    metricsCollector.lastBatchSize.Load(),
		TotalBlocks:      blocks,
		AvgBlockMillis:   avgBlockMillis,
		LastBlockNumber:  metricsCollector.lastBlockNumber.Load(),
		LastBlockMillis:  float64(metricsCollector.lastBlockDuration.Load()) / 1e6,
		LastUpdatedRFC33: lastUpdated.Format(time.RFC3339),
	}
}
