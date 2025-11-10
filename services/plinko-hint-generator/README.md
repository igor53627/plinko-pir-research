# Piano Hint Generator Service (Go)

**Purpose**: Generate Plinko PIR hints from database for client downloads

## Configuration

- **Input**: `/data/database.bin` (64 MB, from db-generator)
- **Output**: `/data/hint.bin` (~67 MB with metadata)
- **Piano Parameters**:
  - DBSize: 8,388,608 entries
  - ChunkSize: 8,192 (calculated: 2√n, rounded to power of 2)
  - SetSize: 1,024 (calculated: ⌈n/chunk⌉, rounded to multiple of 4)

## Performance

**Expected runtime**: <5 seconds
- Database read: ~1 second
- Hint generation: ~2 seconds
- File write: ~1 second

**Memory**: ~130 MB (database + hint in memory briefly)

## Output Format

### hint.bin Structure

**Header (32 bytes)**:
```
[0:8]   DBSize (uint64)      = 8,388,608
[8:16]  ChunkSize (uint64)   = 8,192
[16:24] SetSize (uint64)     = 1,024
[24:32] Reserved (uint64)    = 0
```

**Body (67,108,864 bytes)**:
```
Piano-formatted database in chunks:
  Chunk 0: entries [0:8,192)
  Chunk 1: entries [8,192:16,384)
  ...
  Chunk 1,023: entries [8,380,416:8,388,608)
```

**Total Size**: 32 + 67,108,864 = 67,108,896 bytes (~64 MB)

## Usage

### Start with Docker Compose
```bash
docker-compose up piano-hint-generator
```

### Manual Testing
```bash
# Build service
docker-compose build piano-hint-generator

# Run service (waits for database.bin)
docker-compose run --rm piano-hint-generator

# Check output
ls -lh shared/data/hint.bin
```

### Verify Output
```bash
# Check hint.bin size (should be ~67 MB)
stat -f%z shared/data/hint.bin

# Extract header metadata
xxd -l 32 shared/data/hint.bin
```

## Implementation Details

### Plinko PIR Parameter Calculation

For database size N:
- **ChunkSize** = next power of 2 ≥ 2√N
- **SetSize** = ⌈N / ChunkSize⌉, rounded up to multiple of 4

For N = 8,388,608:
- √N ≈ 2,896
- 2√N ≈ 5,793
- ChunkSize = 8,192 (next power of 2)
- SetSize = ⌈8,388,608 / 8,192⌉ = 1,024

### Hint Generation Process

1. Wait for database.bin to exist
2. Read database into memory
3. Pad to ChunkSize × SetSize if needed
4. Write metadata header
5. Write Piano-formatted database
6. Verify output size

### File Format

The hint file contains:
- Metadata for Plinko PIR client initialization
- Database in chunked format (logically, not physically rearranged)
- Used by Plinko for incremental updates

## Files

- `main.go` - Hint generator implementation
- `go.mod` - Go module (no external dependencies)
- `Dockerfile` - Multi-stage build
- `generate-hint.sh` - Wrapper script with database validation
- `README.md` - This file

## Troubleshooting

**Problem**: Timeout waiting for database.bin
- Ensure db-generator service completed successfully
- Check db-generator logs for errors
- Verify shared volume is mounted correctly

**Problem**: Hint size mismatch
- Expected: 67,108,896 bytes (64 MB + 32 byte header)
- Check database.bin size is exactly 67,108,864 bytes
- Verify padding logic is correct

**Problem**: Memory issues
- Service needs ~130 MB RAM (database + hint)
- Increase Docker memory limit if needed

## Plinko PIR Context

In Plinko PIR:
- Client downloads hints (preprocessed database chunks)
- Client generates PRF-based queries
- Server computes chunk parities without learning query
- Plinko updates hints incrementally when database changes

For this PoC:
- Hint = Piano-formatted database with metadata
- Enables ~5ms private queries (from research)
- Updated incrementally by Plinko (~24 μs per update)

## Next Steps

After hint generation:
1. Plinko Update Service monitors blockchain changes
2. Generates delta files when accounts change
3. Client applies deltas to local hint via XOR
4. Queries remain private with real-time updates
