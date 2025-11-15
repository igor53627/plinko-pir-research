package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

func loadAddressMapping(path string) (map[string]uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read address-mapping: %w", err)
	}

	const entrySize = 24
	if len(data)%entrySize != 0 {
		return nil, fmt.Errorf("address-mapping size %d is not a multiple of %d", len(data), entrySize)
	}

	entries := len(data) / entrySize
	mapping := make(map[string]uint64, entries)

	for offset := 0; offset < len(data); offset += entrySize {
		addrBytes := data[offset : offset+20]
		index := binary.LittleEndian.Uint32(data[offset+20 : offset+24])
		addr := strings.ToLower(common.BytesToAddress(addrBytes).Hex())
		mapping[addr] = uint64(index)
	}

	return mapping, nil
}
