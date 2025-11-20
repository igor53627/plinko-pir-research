package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/log/v3"
)

// Account represents the Ethereum account structure in Erigon's PlainState.
type Account struct {
	Nonce    uint64
	Balance  *uint256.Int
	Root     common.Hash
	CodeHash common.Hash
}

func main() {
	dbPath := flag.String("chaindata", "", "Path to chaindata directory (containing data.mdb)")
	outDir := flag.String("out", "output", "Output directory")
	limit := flag.Uint64("limit", 0, "Limit number of accounts")
	flag.Parse()

	// Setup logger
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StderrHandler))

	if *dbPath == "" {
		log.Error("Please provide -chaindata path")
		os.Exit(1)
	}

	// Open MDBX
	log.Info("Opening DB", "path", *dbPath)
	opts := mdbx.NewMDBX(log.New()).Path(*dbPath).Readonly()
	db, err := opts.Open()
	if err != nil {
		log.Error("Failed to open DB", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// Prepare output
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Error("Failed to create output dir", "err", err)
		os.Exit(1)
	}

	dbFile, err := os.Create(filepath.Join(*outDir, "database.bin"))
	if err != nil {
		panic(err)
	}
	defer dbFile.Close()

	mapFile, err := os.Create(filepath.Join(*outDir, "address-mapping.bin"))
	if err != nil {
		panic(err)
	}
	defer mapFile.Close()

	// Start transaction
	tx, err := db.BeginRo(context.Background())
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	// Erigon bucket for accounts/storage is "PlainState"
	cursor, err := tx.Cursor("PlainState")
	if err != nil {
		// Fallback or check error. 
		// "PlainState" is the standard name in Erigon.
		log.Error("Failed to open cursor on PlainState", "err", err)
		panic(err)
	}
	defer cursor.Close()

	count := uint64(0)
	start := time.Now()
	lastLog := start

	log.Info("Starting export...")

	// Iterate
	// k: Address (20 bytes) or Address + Incarnation + Key (60+ bytes)
	for k, v, err := cursor.First(); k != nil; k, v, err = cursor.Next() {
		if err != nil {
			panic(err)
		}

		// Filter for Accounts only
		if len(k) != 20 {
			continue
		}

		var acc Account
		if err := rlp.DecodeBytes(v, &acc); err != nil {
			// Attempt to decode with a legacy structure or just skip?
			// Usually this implies it's not an account or DB corruption, 
			// OR it's an optimized encoding (e.g. for empty code hash).
			// Erigon *does* use standard RLP for PlainState values.
			log.Warn("Failed to decode account", "key", fmt.Sprintf("%x", k), "err", err)
			continue
		}

		// Write Address (20 bytes)
		if _, err := mapFile.Write(k); err != nil {
			panic(err)
		}

		// Write Balance (32 bytes, Big Endian)
		// uint256.Int Bytes32() returns big-endian 32 bytes.
		var balanceBytes [32]byte
		if acc.Balance != nil {
			balanceBytes = acc.Balance.Bytes32()
		} else {
			// Zero balance
			balanceBytes = [32]byte{}
		}

		if _, err := dbFile.Write(balanceBytes[:]); err != nil {
			panic(err)
		}

		count++
		if count%100000 == 0 {
			now := time.Now()
			if now.Sub(lastLog) > 5*time.Second {
				log.Info("Processed accounts", "count", count/1000000, "M", "elapsed", now.Sub(start))
				lastLog = now
			}
		}

		if *limit > 0 && count >= *limit {
			break
		}
	}

	log.Info("Done", "total_accounts", count, "total_time", time.Since(start))
}