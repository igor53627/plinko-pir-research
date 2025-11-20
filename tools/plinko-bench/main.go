package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"time"

	"plinko-pir-server/pkg/client"
)

const (
	DBEntrySize   = 32
	DBEntryLength = 4
)

func main() {
	dbPath := flag.String("db", "", "Path to database.bin")
	numHints := flag.Int("hints", 100000, "Number of hints (m)")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("Please provide -db path")
	}

	// Open file
	f, err := os.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Get size
	fi, _ := f.Stat()
	size := fi.Size()
	if size%DBEntrySize != 0 {
		log.Printf("Warning: DB size %d is not multiple of 32", size)
	}
	n := uint64(size / DBEntrySize)

	fmt.Printf("Database: %s\n", *dbPath)
	fmt.Printf("Entries (n): %d\n", n)
	fmt.Printf("Hints (m): %d\n", *numHints)

	// Initialize Client
	keyAlpha := make([]byte, 16) // Dummy key
	keyBeta := make([]byte, 16)
	c := client.NewClient(n, uint64(*numHints), keyAlpha, keyBeta)

	// Stateful stream
	currentIdx := uint64(0)
	streamFunc := func() (client.DBEntry, bool) {
		var buf [DBEntrySize]byte
		_, err := io.ReadFull(f, buf[:])
		if err == io.EOF {
			return client.DBEntry{}, false
		}
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				return client.DBEntry{}, false
			}
			log.Fatal(err)
		}

		var val [4]uint64
		for i := 0; i < 4; i++ {
			val[i] = binary.LittleEndian.Uint64(buf[i*8 : (i+1)*8])
		}
		
		entry := client.DBEntry{
			Index: currentIdx,
			Value: val,
		}
		currentIdx++
		return entry, true
	}

	fmt.Println("Starting Hint Generation...")
	start := time.Now()
    
    runtime.GC()
    var m1, m2 runtime.MemStats
    runtime.ReadMemStats(&m1)

	c.HintInit(streamFunc)

    duration := time.Since(start)
    runtime.ReadMemStats(&m2)
    
    fmt.Printf("Finished in %v\n", duration)
    fmt.Printf("Throughput: %.2f entries/sec\n", float64(n)/duration.Seconds())
    fmt.Printf("Allocated Mem (Delta): %v MB\n", (m2.TotalAlloc - m1.TotalAlloc) / 1024 / 1024)
    fmt.Printf("Heap Alloc: %v MB\n", m2.HeapAlloc / 1024 / 1024)
    fmt.Printf("System Mem: %v MB\n", m2.Sys / 1024 / 1024)
}
