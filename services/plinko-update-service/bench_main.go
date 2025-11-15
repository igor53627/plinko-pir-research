//go:build bench

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type benchEntry struct {
	Index uint64 `json:"index"`
	Old   uint64 `json:"old"`
	New   uint64 `json:"new"`
}

func main() {
	inputPath := flag.String("input", "", "Path to JSON file containing updates")
	dbSizeFlag := flag.Uint64("db-size", 8388608, "Database size (number of entries)")
	repeat := flag.Int("repeat", 25, "Number of iterations to run")
	enableCache := flag.Bool("cache", true, "Enable cache mode")
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("input file is required (use -input)")
		os.Exit(1)
	}

	entries, err := loadBenchEntries(*inputPath)
	if err != nil {
		fmt.Printf("failed to load bench input: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("bench input must contain at least one update")
		os.Exit(1)
	}

	chunkSize, setSize := derivePlinkoParams(*dbSizeFlag)
	totalEntries := chunkSize * setSize
	fmt.Printf("📦 Bench DB size=%d entries (chunk=%d, set=%d, total=%d)\n",
		*dbSizeFlag, chunkSize, setSize, totalEntries)

	database := make([]uint64, totalEntries)
	baseDB := make([]uint64, totalEntries)

	pm := NewPlinkoUpdateManager(database, *dbSizeFlag, chunkSize, setSize)
	if *enableCache {
		fmt.Println("⚙️  Building cache...")
		cacheDur := pm.EnableCacheMode()
		fmt.Printf("✅ Cache ready in %v\n", cacheDur)
	}

	dbUpdates := make([]DBUpdate, len(entries))
	for i, entry := range entries {
		dbUpdates[i] = DBUpdate{
			Index: entry.Index,
			OldValue: DBEntry{
				entry.Old,
			},
			NewValue: DBEntry{
				entry.New,
			},
		}
	}

	var totalDuration time.Duration
	var minDuration = time.Duration(1<<63 - 1)
	var maxDuration time.Duration

	for iter := 1; iter <= *repeat; iter++ {
		copy(pm.database, baseDB)
		start := time.Now()
		_, duration := pm.ApplyUpdates(dbUpdates)
		if duration == 0 {
			duration = time.Since(start)
		}
		nsPerUpdate := float64(duration.Nanoseconds()) / float64(len(dbUpdates))
		fmt.Printf("Iteration %3d: %9s total (%0.2f ns/update)\n", iter, duration, nsPerUpdate)

		if duration < minDuration {
			minDuration = duration
		}
		if duration > maxDuration {
			maxDuration = duration
		}
		totalDuration += duration
	}

	avg := totalDuration / time.Duration(*repeat)
	fmt.Println("========================================")
	fmt.Printf("Batches: %d | Updates per batch: %d\n", *repeat, len(dbUpdates))
	fmt.Printf("Average: %s | Min: %s | Max: %s\n", avg, minDuration, maxDuration)
	fmt.Printf("Avg per update: %.2f ns\n", float64(avg.Nanoseconds())/float64(len(dbUpdates)))
	fmt.Println("========================================")
}

func loadBenchEntries(path string) ([]benchEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []benchEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
