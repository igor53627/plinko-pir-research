# Database Generator Service (Go)

**Purpose**: Extract Ethereum state from Anvil and create Plinko PIR database files

## Configuration

- **Accounts**: 8,388,608 (2^23)
- **Concurrent workers**: 10,000 goroutines
- **Output files**:
  - `database.bin`: 64 MB (8 bytes × 8.4M accounts)
  - `address-mapping.bin`: 192 MB (24 bytes × 8.4M accounts)

## Performance

**Expected runtime**: 1-5 minutes
- Address generation: ~5 seconds
- Balance queries: 1-4 minutes (depends on Anvil responsiveness)
- File writing: <10 seconds

**Concurrency**: 10,000 concurrent RPC requests to Anvil

## Output Format

### database.bin
- **Size**: 67,108,864 bytes (8,388,608 × 8)
- **Format**: Sequential 8-byte little-endian uint64 values
- **Content**: Account balances in wei (sorted by address)

### address-mapping.bin
- **Size**: 201,326,592 bytes (8,388,608 × 24)
- **Format**: 24-byte records (20-byte address + 4-byte index)
- **Content**: Address→database index mapping (sorted by address)

## Usage

### Start with Docker Compose
```bash
docker-compose up db-generator
```

### Manual Testing
```bash
# Build service
docker-compose build db-generator

# Run service
docker-compose run --rm db-generator

# Check output files
ls -lh shared/data/database.bin
ls -lh shared/data/address-mapping.bin
```

### Verify Output
```bash
# Check database.bin size (should be exactly 67,108,864 bytes)
stat -f%z shared/data/database.bin

# Check address-mapping.bin size (should be exactly 201,326,592 bytes)
stat -f%z shared/data/address-mapping.bin
```

## Implementation Details

### Address Generation
- Sequential deterministic addresses for PoC testing (base: 0x1000...0000)
- ⚠️ **PoC Approach**: Generates sequential addresses for scale testing
- ✅ **Production**: Would derive actual Anvil addresses from mnemonic using BIP-39/BIP-44
- Ensures consistent ordering across runs
- Tests database generation flow at full 8.4M scale

### Balance Queries
- 10,000 concurrent goroutines
- Work-stealing job queue pattern
- Automatic retry on RPC errors
- Progress reporting every 1,000 accounts

### Sorting
- Lexicographic sorting by address hex string
- Ensures deterministic database ordering
- Required for Plinko PIR consistency

## Files

- `main.go` - Database generator implementation
- `go.mod` - Go module dependencies
- `Dockerfile` - Multi-stage build for minimal image
- `README.md` - This file

## Troubleshooting

**Problem**: Connection refused to Anvil
- Ensure eth-mock service is healthy before running
- Check Docker network connectivity

**Problem**: Slow generation (>10 minutes)
- Normal if Anvil is still creating accounts
- Check Anvil logs for account creation progress
- Reduce ConcurrentWorkers if seeing RPC errors

**Problem**: File size mismatch
- Check account count matches TotalAccounts constant
- Verify all accounts were queried successfully
- Look for error messages in logs

**Problem**: Out of memory
- 10,000 concurrent workers may be too many
- Reduce ConcurrentWorkers constant in main.go
- Increase Docker memory limit

## Performance Optimization

Current optimizations:
- ✅ High concurrency (10,000 goroutines)
- ✅ Pre-generated addresses (no RPC discovery)
- ✅ Batch progress reporting
- ✅ Multi-stage Docker build (small image)

Potential improvements:
- Use Anvil's bulk state export API (if available)
- Cache results locally for re-runs
- Stream writes instead of buffering entire dataset
