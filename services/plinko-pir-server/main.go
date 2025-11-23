package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	log.Println("========================================")
	log.Println("Plinko PIR Server")
	log.Println("========================================")
	log.Println()

	cfg := LoadConfig()
	log.Printf("Configuration: port=%s, database_path=%s, database_timeout=%s\n",
		cfg.ListenAddress(), cfg.DatabasePath, cfg.DatabaseWaitTimeout)

	waitForDatabase(cfg.DatabasePath, cfg.DatabaseWaitTimeout)

	log.Println("Loading canonical database snapshot...")
	server := loadServer(cfg.DatabasePath)
	log.Printf("✅ Database loaded: %d entries (%d MB)\n",
		server.dbSize, server.dbSize*DBEntrySize/1024/1024)
	log.Printf("   ChunkSize: %d, SetSize: %d\n", server.chunkSize, server.setSize)
	log.Println()

	http.HandleFunc("/health", corsMiddleware(server.healthHandler))
	http.HandleFunc("/query/plaintext", corsMiddleware(server.plaintextQueryHandler))
	http.HandleFunc("/query/plinko", corsMiddleware(server.plinkoQueryHandler))

	addr := cfg.ListenAddress()
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

func waitForDatabase(path string, timeout time.Duration) {
	log.Printf("Waiting for canonical database at %s...\n", path)

	if timeout <= 0 {
		if _, err := os.Stat(path); err != nil {
			log.Fatalf("Database file %s not found and timeout disabled", path)
		}
		log.Println("✅ database file found")
		return
	}

	start := time.Now()
	attempts := 0

	for {
		if _, err := os.Stat(path); err == nil {
			log.Println("✅ database file found")
			return
		}

		attempts++
		if attempts%10 == 0 {
			elapsed := time.Since(start)
			log.Printf("  Still waiting... (%ds/%ds)\n", int(elapsed.Seconds()), int(timeout.Seconds()))
		}

		if time.Since(start) >= timeout {
			log.Fatalf("Timeout waiting for database file at %s", path)
		}

		time.Sleep(time.Second)
	}
}
