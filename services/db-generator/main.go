package main

import (
	"context"
	"encoding/binary"
	"log"
	"math/big"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	// Database configuration
	TotalAccounts    = 8388608 // 2^23 accounts
	DatabasePath     = "/data/database.bin"
	AddressMappingPath = "/data/address-mapping.bin"

	// Performance tuning
	ConcurrentWorkers = 10000  // High concurrency for fast queries
	BatchSize         = 1000   // Progress reporting interval

	// Anvil default mnemonic (well-known test mnemonic)
	AnvilMnemonic = "test test test test test test test test test test test junk"
)

type AccountData struct {
	Address common.Address
	Balance *big.Int
}

func main() {
	log.Println("========================================")
	log.Println("Plinko PIR Database Generator (Go)")
	log.Println("========================================")
	log.Printf("Accounts: %d (2^23)\n", TotalAccounts)
	log.Printf("Concurrent workers: %d\n", ConcurrentWorkers)
	log.Println()

	// Check if database already exists
	if _, err := os.Stat(DatabasePath); err == nil {
		log.Println("✓ Database already exists at", DatabasePath)
		log.Println("✓ Skipping generation (delete file to regenerate)")
		return
	}

	// Connect to Anvil
	client, err := ethclient.Dial("http://eth-mock:8545")
	if err != nil {
		log.Fatalf("Failed to connect to Anvil: %v", err)
	}
	defer client.Close()

	log.Println("Connected to Anvil successfully")

	// Wait for Anvil to be ready
	waitForAnvil(client)

	// Generate all account addresses deterministically
	log.Println("Generating account addresses...")
	startGen := time.Now()
	addresses := generateAnvilAddresses(TotalAccounts)
	log.Printf("Generated %d addresses in %v\n", len(addresses), time.Since(startGen))

	// Query balances concurrently
	log.Println("Querying account balances...")
	startQuery := time.Now()
	accounts := queryBalancesConcurrent(client, addresses)
	log.Printf("Queried %d balances in %v\n", len(accounts), time.Since(startQuery))

	// Sort accounts by address (deterministic ordering)
	log.Println("Sorting accounts by address...")
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Address.Hex() < accounts[j].Address.Hex()
	})

	// Write database.bin (8 bytes per account)
	log.Println("Writing database.bin...")
	if err := writeDatabaseBin(accounts); err != nil {
		log.Fatalf("Failed to write database.bin: %v", err)
	}

	// Write address-mapping.bin (20 bytes address + 4 bytes index)
	log.Println("Writing address-mapping.bin...")
	if err := writeAddressMapping(accounts); err != nil {
		log.Fatalf("Failed to write address-mapping.bin: %v", err)
	}

	// Verify output
	verifyOutput()

	log.Println()
	log.Println("✅ Database generation complete!")
	log.Printf("Total time: %v\n", time.Since(startGen))
}

// waitForAnvil waits for Anvil to be ready
func waitForAnvil(client *ethclient.Client) {
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		_, err := client.BlockNumber(ctx)
		if err == nil {
			return
		}
		log.Printf("Waiting for Anvil to be ready... (%d/30)\n", i+1)
		time.Sleep(2 * time.Second)
	}
	log.Fatal("Anvil did not become ready in time")
}

// generateAnvilAddresses generates Anvil account addresses deterministically
// Anvil uses derivation path m/44'/60'/0'/0/i for account i
func generateAnvilAddresses(count int) []common.Address {
	addresses := make([]common.Address, count)

	// Anvil generates sequential addresses starting from a base
	// For PoC, we'll query Anvil directly for its first account,
	// then generate sequential addresses from there
	// In production, use proper BIP-39/BIP-44 derivation from mnemonic

	// Simple sequential address generation for PoC
	// This creates deterministic, sortable addresses for testing
	baseAddr := common.HexToAddress("0x1000000000000000000000000000000000000000")

	for i := 0; i < count; i++ {
		// Generate sequential addresses by adding index to base
		addr := common.BigToAddress(new(big.Int).Add(baseAddr.Big(), big.NewInt(int64(i))))
		addresses[i] = addr

		if (i+1)%1000000 == 0 {
			log.Printf("  Generated %d/%d addresses...\n", i+1, count)
		}
	}

	return addresses
}

// queryBalancesConcurrent queries account balances with high concurrency
func queryBalancesConcurrent(client *ethclient.Client, addresses []common.Address) []AccountData {
	accounts := make([]AccountData, len(addresses))

	// Worker pool
	jobs := make(chan int, len(addresses))
	var wg sync.WaitGroup

	// Progress tracking
	var processed int64
	var mu sync.Mutex

	// Start workers
	for w := 0; w < ConcurrentWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()

			for i := range jobs {
				balance, err := client.BalanceAt(ctx, addresses[i], nil)
				if err != nil {
					log.Printf("Error querying balance for %s: %v\n", addresses[i].Hex(), err)
					balance = big.NewInt(0)
				}

				accounts[i] = AccountData{
					Address: addresses[i],
					Balance: balance,
				}

				// Progress reporting
				mu.Lock()
				processed++
				if processed%BatchSize == 0 {
					log.Printf("  Processed %d/%d accounts (%.1f%%)\n",
						processed, len(addresses),
						float64(processed)/float64(len(addresses))*100)
				}
				mu.Unlock()
			}
		}()
	}

	// Send jobs
	for i := range addresses {
		jobs <- i
	}
	close(jobs)

	// Wait for completion
	wg.Wait()

	return accounts
}

// writeDatabaseBin writes database.bin with 8-byte balance entries
func writeDatabaseBin(accounts []AccountData) error {
	f, err := os.Create(DatabasePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write 8 bytes per account (uint64 balance in wei, truncated)
	for _, acc := range accounts {
		// Convert big.Int balance to uint64 (sufficient for PoC)
		balance := acc.Balance.Uint64()

		// Write as little-endian uint64
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], balance)
		if _, err := f.Write(buf[:]); err != nil {
			return err
		}
	}

	return nil
}

// writeAddressMapping writes address-mapping.bin with address→index mapping
func writeAddressMapping(accounts []AccountData) error {
	f, err := os.Create(AddressMappingPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write 24 bytes per account (20 bytes address + 4 bytes index)
	for i, acc := range accounts {
		// Write address (20 bytes)
		if _, err := f.Write(acc.Address.Bytes()); err != nil {
			return err
		}

		// Write index (4 bytes, little-endian)
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(i))
		if _, err := f.Write(buf[:]); err != nil {
			return err
		}
	}

	return nil
}

// verifyOutput checks file sizes match expected values
func verifyOutput() {
	// Check database.bin
	dbInfo, err := os.Stat(DatabasePath)
	if err != nil {
		log.Printf("⚠️  Could not stat database.bin: %v\n", err)
	} else {
		expectedDB := int64(TotalAccounts * 8)
		if dbInfo.Size() == expectedDB {
			log.Printf("✅ database.bin: %d bytes (expected %d)\n", dbInfo.Size(), expectedDB)
		} else {
			log.Printf("❌ database.bin: %d bytes (expected %d)\n", dbInfo.Size(), expectedDB)
		}
	}

	// Check address-mapping.bin
	mapInfo, err := os.Stat(AddressMappingPath)
	if err != nil {
		log.Printf("⚠️  Could not stat address-mapping.bin: %v\n", err)
	} else {
		expectedMap := int64(TotalAccounts * 24)
		if mapInfo.Size() == expectedMap {
			log.Printf("✅ address-mapping.bin: %d bytes (expected %d)\n", mapInfo.Size(), expectedMap)
		} else {
			log.Printf("❌ address-mapping.bin: %d bytes (expected %d)\n", mapInfo.Size(), expectedMap)
		}
	}
}
