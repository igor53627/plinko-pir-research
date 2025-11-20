package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	inputPath := flag.String("input", "", "Path to 8-byte per entry input file")
	outputPath := flag.String("output", "", "Path to 32-byte per entry output file")
	entries := flag.Int("entries", 0, "Number of entries to convert (0 = auto based on file size)")
	flag.Parse()

	if *inputPath == "" || *outputPath == "" {
		fmt.Println("input and output paths are required")
		os.Exit(1)
	}

	inFile, err := os.Open(*inputPath)
	if err != nil {
		fmt.Printf("failed to open input: %v\n", err)
		os.Exit(1)
	}
	defer inFile.Close()

	outFile, err := os.Create(*outputPath)
	if err != nil {
		fmt.Printf("failed to create output: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	// Determine number of entries if not provided
	if *entries == 0 {
		fi, err := inFile.Stat()
		if err != nil {
			fmt.Printf("failed to stat input: %v\n", err)
			os.Exit(1)
		}
		*entries = int(fi.Size() / 8)
	}

	buf := make([]byte, 8)
	zeroPad := make([]byte, 24) // remaining bytes for 32-byte entry
	for i := 0; i < *entries; i++ {
		_, err := io.ReadFull(inFile, buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("read error at entry %d: %v\n", i, err)
			os.Exit(1)
		}
		// Write the original 8 bytes (little endian) then 24 zero bytes
		if _, err := outFile.Write(buf); err != nil {
			fmt.Printf("write error at entry %d: %v\n", i, err)
			os.Exit(1)
		}
		if _, err := outFile.Write(zeroPad); err != nil {
			fmt.Printf("write padding error at entry %d: %v\n", i, err)
			os.Exit(1)
		}
	}
	fmt.Printf("Converted %d entries to 32-byte format, written to %s\n", *entries, *outputPath)
}
