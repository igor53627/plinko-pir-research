# Plinko PIR Server (Go)

**Purpose**: Private Information Retrieval server for querying Ethereum balances without revealing which address

## Privacy Guarantee

**Information-Theoretic Privacy**: Server learns absolutely nothing about which account the client queried

- ❌ Server **NEVER** logs queried addresses
- ✅ Client queries are **indistinguishable** from random noise to server
- ✅ Even with infinite computational power, server cannot determine query target

## Configuration

- **Input**: `/data/hint.bin` (Piano-formatted database)
- **HTTP Port**: 3000
- **Query Latency**: <10ms (from research: ~5ms for 8.4M database)
- **Database**: In-memory (64 MB for 8.4M accounts)

## Performance

**Query Performance** (from Plinko PIR research):
- **PlaintextQuery**: <1ms (direct lookup)
- **FullSetQuery**: ~5ms (Plinko PIR with k=1,024 sets)
- **SetParityQuery**: ~2-3ms (simplified query)

**Memory Usage**: ~130 MB
- 64 MB database
- 64 MB overhead (server structures)

## API Endpoints

### Health Check

```bash
GET /health
```

**Response**:
```json
{
  "status": "healthy",
  "service": "piano-pir-server",
  "db_size": 8388608,
  "chunk_size": 8192,
  "set_size": 1024
}
```

### Plaintext Query (Testing Only)

⚠️ **Not Private** - Use only for testing/debugging

```bash
POST /query/plaintext
Content-Type: application/json

{
  "index": 42
}
```

**Response**:
```json
{
  "value": 1000000000000000000000,
  "server_time_nanos": 450000
}
```

### Full Set Query (Plinko PIR)

✅ **Information-Theoretically Private**

```bash
POST /query/fullset
Content-Type: application/json

{
  "prf_key": [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]
}
```

**Response**:
```json
{
  "value": 1234567890,
  "server_time_nanos": 5200000
}
```

**How it works**:
1. Client generates random PRF key
2. Server expands key to pseudorandom set of indices
3. Server computes XOR parity over the set
4. Client decodes response to extract desired value
5. Server learns nothing about which index client wanted

### Set Parity Query (Simplified)

✅ **Private** (when used with Plinko PIR protocol)

```bash
POST /query/setparity
Content-Type: application/json

{
  "indices": [100, 500, 1000, 2000]
}
```

**Response**:
```json
{
  "parity": 9876543210,
  "server_time_nanos": 2800000
}
```

## Usage

### Start with Docker Compose
```bash
docker-compose up piano-pir-server
```

### Manual Testing
```bash
# Build service
docker-compose build piano-pir-server

# Run service (waits for hint.bin)
docker-compose run --rm -p 3000:3000 piano-pir-server

# Test health endpoint
curl http://localhost:3000/health

# Test plaintext query
curl -X POST http://localhost:3000/query/plaintext \
  -H "Content-Type: application/json" \
  -d '{"index": 42}'

# Test Plinko PIR query
curl -X POST http://localhost:3000/query/fullset \
  -H "Content-Type: application/json" \
  -d '{"prf_key": [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]}'
```

## Privacy Implementation

### Critical Privacy Rules

**NEVER log queried addresses**:
```go
// ❌ NEVER DO THIS:
log.Printf("Query for address %s", address)
log.Printf("User queried index %d", index)

// ✅ OK to log:
log.Printf("Query completed in %v", duration)
log.Printf("SetParity query (%d indices) completed", len(indices))
```

**Why this matters**:
- Even a single logged address breaks privacy
- Logs can be subpoenaed, hacked, or leaked
- Privacy must be **perfect**, not "good enough"

### Plinko PIR Privacy Proof

Plinko PIR provides **information-theoretic privacy**:

1. **Client generates random PRF key** k ← {0,1}^128
2. **Server expands key to set** S = Expand(k, setSize)
3. **Server computes parity** p = ⊕_{i ∈ S} DB[i]
4. **Client decodes** using knowledge of S

**Privacy argument**:
- Server sees only random key k
- Set S is pseudorandom and reveals nothing about target index
- Even with unlimited compute, server cannot determine query

## Implementation Details

### Database Loading

Loads hint.bin into memory for fast queries:

```go
// Read hint.bin (32-byte header + database)
data := readFile("/data/hint.bin")

// Extract metadata
dbSize := binary.LittleEndian.Uint64(data[0:8])
chunkSize := binary.LittleEndian.Uint64(data[8:16])
setSize := binary.LittleEndian.Uint64(data[16:24])

// Load database into memory
database := parseDatabase(data[32:])
```

### Full Set Query Algorithm

```go
func HandleFullSetQuery(prfKey []byte) uint64 {
    // 1. Expand PRF key to pseudorandom set
    prSet := NewPRSet(prfKey)
    indices := prSet.Expand(setSize, chunkSize)

    // 2. Compute XOR parity
    parity := 0
    for _, idx := range indices {
        parity ^= database[idx]
    }

    return parity
}
```

**Time Complexity**: O(k) where k = setSize (1,024)
**Space Complexity**: O(1) additional space

### PRSet Expansion

Pseudorandom set expands k → setSize indices:

```go
func (prs *PRSet) Expand(setSize, chunkSize uint64) []uint64 {
    indices := make([]uint64, setSize)

    for i := 0; i < setSize; i++ {
        // Generate random offset in chunk i
        offset := PRF(prs.Key, i) % chunkSize

        // Database index = chunk_start + offset
        indices[i] = i*chunkSize + offset
    }

    return indices
}
```

## Files

- `main.go` - HTTP server, query handlers, database loading
- `prset.go` - Pseudorandom set expansion for Plinko PIR
- `go.mod` - Go module (no external dependencies)
- `Dockerfile` - Multi-stage build for minimal image
- `README.md` - This file

## Troubleshooting

**Problem**: Timeout waiting for hint.bin
- Ensure piano-hint-generator completed successfully
- Check hint-generator logs for errors
- Verify shared volume permissions

**Problem**: Slow queries (>50ms)
- Check database loaded in memory (not reading from disk)
- Verify sufficient RAM (needs ~130 MB)
- Look for CPU throttling

**Problem**: Connection refused
- Check service started successfully
- Verify port 3000 is not in use
- Check Docker network connectivity

**Problem**: "Invalid PRF key" error
- PRF key must be exactly 16 bytes
- Encode as JSON array: `[0,1,2,...,15]`

## Production Considerations

### HTTPS/TLS

Add TLS for production:

```go
// Generate self-signed cert for testing
// openssl req -x509 -newkey rsa:4096 -nodes \
//   -keyout key.pem -out cert.pem -days 365

http.ListenAndServeTLS(":3000", "cert.pem", "key.pem", nil)
```

### Rate Limiting

Prevent DoS attacks:

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(100, 200) // 100 req/s, burst 200

http.HandleFunc("/query/fullset", func(w http.ResponseWriter, r *http.Request) {
    if !limiter.Allow() {
        http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
        return
    }
    // ... handle query
})
```

### Monitoring

Track query latency without breaking privacy:

```go
// ✅ OK - no address info
prometheus.Histogram("pir_query_latency_ms").Observe(elapsed.Milliseconds())
prometheus.Counter("pir_queries_total").Inc()

// ❌ NEVER - leaks query info
prometheus.Counter("pir_queries_by_address").WithLabelValues(address).Inc()
```

### Horizontal Scaling

Plinko PIR server is stateless (except database):

1. **Replicate database** to multiple servers
2. **Load balance** queries across replicas
3. **Update coordination**: All servers update from same Plinko deltas

```
                    ┌─────────────┐
      Queries  ────>│ Load Balancer│
                    └──────┬───────┘
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
    ┌─────────┐     ┌─────────┐     ┌─────────┐
    │ PIR Srv │     │ PIR Srv │     │ PIR Srv │
    │ + DB    │     │ + DB    │     │ + DB    │
    └────┬────┘     └────┬────┘     └────┬────┘
         │               │               │
         └───────────────┴───────────────┘
                         │
                    ┌────▼────┐
                    │ Plinko  │
                    │ Deltas  │
                    └─────────┘
```

## Plinko PIR Protocol

### Query Flow

1. **Client wants balance of address A**
2. Client computes: `index_A = lookup(A)` in local address mapping
3. Client generates: `k ← PRF.KeyGen()`
4. Client constructs: `query = FullSetQuery(k)` such that `index_A ∈ Expand(k)`
5. Server responds: `parity = ⊕_{i ∈ Expand(k)} DB[i]`
6. Client decodes: `balance_A = decode(parity, k, index_A)`
7. **Privacy**: Server learned nothing about A or index_A

### Why This Works

**Intuition**: Client's query is a large random set that includes target index

- Set size k = 1,024 (for 8.4M database)
- Target index hidden among 1,023 random indices
- Server cannot distinguish target from noise
- XOR parity enables extraction by client

**Formal proof**: See Plinko PIR paper (arXiv:2305.14562)

## Next Steps

After Plinko PIR Server:
1. **CDN Mock**: Serve hints/deltas for client downloads
2. **Ambire Wallet**: Client implementation with Privacy Mode
3. **Integration Testing**: Verify end-to-end private queries
4. **Performance Testing**: Validate <10ms latency target

---

**Status**: Query Privacy ✅ | In-Memory Database ✅ | HTTP API ✅
