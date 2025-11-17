# API Reference

Complete API documentation for all Plinko PIR services.

## Overview

The Plinko PIR system exposes the following HTTP endpoints:

| Service | Port | Purpose |
|---------|------|---------|
| **PIR Server** | 3000 | Private information retrieval queries |
| **Update Service** | 3001 | Real-time update metrics and health |
| **CDN** | 8080 | Snapshot/delta distribution and IPFS proxy |

---

## PIR Server API

**Base URL**: `http://localhost:3000` (external) or `plinko-pir-server:3000` (internal)

### Health Check

Get server health status and configuration.

**Endpoint**: `GET /health`

**Request**: None

**Response** (200 OK):
```json
{
  "status": "healthy",
  "service": "plinko-pir-server",
  "db_size": 5575868,
  "chunk_size": 256,
  "set_size": 21781
}
```

**Example**:
```bash
curl http://localhost:3000/health
```

---

### Plaintext Query

Query a single database entry by index (non-private).

**Endpoint**: `POST /query/plaintext` or `GET /query/plaintext?index={index}`

**Request** (POST):
```json
{
  "index": 12345
}
```

**Request** (GET):
```
GET /query/plaintext?index=12345
```

**Response** (200 OK):
```json
{
  "value": 1500000000000000000,
  "server_time_nanos": 125000
}
```

**Fields**:
- `index` (uint64): Database index to query (0 to db_size-1)
- `value` (uint64): Balance value in wei
- `server_time_nanos` (uint64): Server processing time in nanoseconds

**Example**:
```bash
# POST request
curl -X POST http://localhost:3000/query/plaintext \
  -H "Content-Type: application/json" \
  -d '{"index": 12345}'

# GET request
curl "http://localhost:3000/query/plaintext?index=12345"
```

**Status Codes**:
- `200 OK`: Query successful
- `400 Bad Request`: Invalid index parameter
- `405 Method Not Allowed`: Invalid HTTP method

---

### Full Set Query (Private PIR)

Execute a private PIR query using a PRF key.

**Endpoint**: `POST /query/fullset`

**Request**:
```json
{
  "prf_key": [base64-encoded 16-byte PRF key]
}
```

**Response** (200 OK):
```json
{
  "value": 1500000000000000000,
  "server_time_nanos": 5250000
}
```

**Fields**:
- `prf_key` ([]byte): 16-byte PRF key (base64 encoded in JSON)
- `value` (uint64): XOR parity of all entries in the hint set
- `server_time_nanos` (uint64): Server processing time (~5ms typical)

**Privacy Guarantee**:
The server computes XOR parity over ~1024 database entries determined by the PRF key. The server **cannot determine which specific index** was queried.

**Example**:
```bash
curl -X POST http://localhost:3000/query/fullset \
  -H "Content-Type: application/json" \
  -d '{
    "prf_key": "AAAAAAAAAAAAAAAAAAAAAA=="
  }'
```

**Server Logs** (Privacy Mode):
```
🔒 PRIVATE QUERY RECEIVED
Server sees: PRF Key (16 bytes): 0000000000000000
Server CANNOT determine:
  ❌ Which address is being queried
  ❌ Which balance is being requested
  ❌ Any user information
```

**Status Codes**:
- `200 OK`: Query successful
- `400 Bad Request`: Invalid PRF key (must be 16 bytes)
- `405 Method Not Allowed`: Only POST allowed

---

### Set Parity Query

Compute XOR parity over a custom set of indices.

**Endpoint**: `POST /query/setparity`

**Request**:
```json
{
  "indices": [100, 200, 300, 400, 500]
}
```

**Response** (200 OK):
```json
{
  "parity": 1234567890123456789,
  "server_time_nanos": 850000
}
```

**Fields**:
- `indices` ([]uint64): Array of database indices
- `parity` (uint64): XOR of all values at specified indices
- `server_time_nanos` (uint64): Server processing time

**Example**:
```bash
curl -X POST http://localhost:3000/query/setparity \
  -H "Content-Type: application/json" \
  -d '{
    "indices": [100, 200, 300, 400, 500]
  }'
```

**Status Codes**:
- `200 OK`: Query successful
- `400 Bad Request`: Invalid request body
- `405 Method Not Allowed`: Only POST allowed

---

## Update Service API

**Base URL**: `http://localhost:3001` (external) or `plinko-update-service:3001` (internal)

### Health Check

Get update service health status.

**Endpoint**: `GET /health`

**Request**: None

**Response** (200 OK):
```json
{
  "status": "healthy",
  "service": "plinko-update-service"
}
```

**Example**:
```bash
curl http://localhost:3001/health
```

---

### Metrics

Get real-time update performance metrics.

**Endpoint**: `GET /metrics`

**Request**: None

**Response** (200 OK):
```json
{
  "update_latency_ms": 23.75,
  "last_processed_block": 19234567,
  "total_updates": 1523,
  "average_batch_duration_ms": 18.42,
  "uptime_seconds": 3600
}
```

**Fields**:
- `update_latency_ms` (float64): Average update processing time
- `last_processed_block` (uint64): Most recent Ethereum block processed
- `total_updates` (uint64): Total number of update batches processed
- `average_batch_duration_ms` (float64): Average time per batch
- `uptime_seconds` (uint64): Service uptime

**Example**:
```bash
curl http://localhost:3001/metrics
```

**Status Codes**:
- `200 OK`: Metrics retrieved successfully

---

## CDN API

**Base URL**: `http://localhost:8080` (external) or `cdn-mock:8080` (internal)

### Health Check

Get CDN health status.

**Endpoint**: `GET /health`

**Request**: None

**Response** (200 OK):
```
healthy
```

**Example**:
```bash
curl http://localhost:8080/health
```

---

### Snapshots (Directory Listing)

Browse available snapshot packages.

**Endpoint**: `GET /snapshots/`

**Request**: None

**Response** (200 OK):
HTML directory listing with:
- `manifest.json` - Snapshot metadata
- `database.bin` - Database snapshot
- `ipfs.cid` - IPFS content identifier

**Example**:
```bash
curl http://localhost:8080/snapshots/
```

**Download Manifest**:
```bash
curl http://localhost:8080/snapshots/manifest.json
```

**Manifest Format**:
```json
{
  "version": "1.0",
  "block_number": 19234567,
  "timestamp": 1234567890,
  "database_sha256": "abc123...",
  "database_size": 44606944,
  "ipfs_cid": "QmXyz..."
}
```

---

### Deltas (Directory Listing)

Browse available delta updates.

**Endpoint**: `GET /deltas/`

**Request**: None

**Response** (200 OK):
HTML directory listing with delta files:
- `delta-{block_number}.bin` - Delta update files

**Example**:
```bash
curl http://localhost:8080/deltas/
```

**Download Delta**:
```bash
curl http://localhost:8080/deltas/delta-19234567.bin
```

**Delta Format**: Binary XOR diff file

---

### Address Mapping

Download the address-to-index mapping file.

**Endpoint**: `GET /address-mapping.bin`

**Request**: None

**Response** (200 OK):
Binary file containing address-to-index mappings (128 MB typical).

**Example**:
```bash
curl -O http://localhost:8080/address-mapping.bin
```

**Cache Headers**:
- `Cache-Control: public, max-age=86400` (24 hour cache)

---

### IPFS Gateway Proxy

Access IPFS content via HTTP (CDN proxy to IPFS gateway).

**Endpoint**: `GET /ipfs/{cid}`

**Request**: None

**Response** (200 OK):
Content from IPFS gateway

**Example**:
```bash
# Download snapshot via IPFS
curl http://localhost:8080/ipfs/QmXyz.../database.bin -O
```

**Purpose**:
Provides HTTP access to IPFS content for browsers that don't support `ipfs://` protocol.

**Proxy Configuration**:
- Backend: `ipfs:8080` (local Kubo daemon)
- CORS: Enabled for browser access

---

## Common Headers

All endpoints support CORS:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type, Accept, Range
```

---

## Error Responses

All services use standard HTTP status codes:

**400 Bad Request**:
```json
{
  "error": "Invalid request format"
}
```

**404 Not Found**:
```
404 Not Found
```

**405 Method Not Allowed**:
```json
{
  "error": "Method not allowed"
}
```

**500 Internal Server Error**:
```json
{
  "error": "Internal server error"
}
```

---

## Performance Benchmarks

Typical response times on localhost:

| Endpoint | Latency | Notes |
|----------|---------|-------|
| `/health` | <1ms | No database access |
| `/query/plaintext` | ~5ms | Direct database lookup |
| `/query/fullset` | ~5ms | ~1024 XOR operations |
| `/query/setparity` | Variable | Depends on indices count |
| `/metrics` | <1ms | In-memory metrics |
| `/snapshots/manifest.json` | ~10ms | File read |
| `/ipfs/{cid}` | Variable | IPFS gateway latency |

---

## Client Usage Examples

### JavaScript (Wallet Client)

```javascript
// Private balance query
async function queryBalance(prfKey) {
  const response = await fetch('http://localhost:3000/query/fullset', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ prf_key: prfKey })
  });

  const data = await response.json();
  return data.value;
}

// Download snapshot manifest
async function downloadManifest() {
  const response = await fetch('http://localhost:8080/snapshots/manifest.json');
  return await response.json();
}
```

### Python

```python
import requests
import base64

# Private PIR query
def query_balance(prf_key_bytes):
    response = requests.post(
        'http://localhost:3000/query/fullset',
        json={'prf_key': base64.b64encode(prf_key_bytes).decode()}
    )
    return response.json()['value']

# Download snapshot
def download_snapshot():
    response = requests.get('http://localhost:8080/snapshots/manifest.json')
    return response.json()
```

### Go

```go
// Private PIR query
func QueryBalance(prfKey []byte) (uint64, error) {
    body, _ := json.Marshal(map[string]interface{}{
        "prf_key": prfKey,
    })

    resp, err := http.Post(
        "http://localhost:3000/query/fullset",
        "application/json",
        bytes.NewBuffer(body),
    )
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var result struct {
        Value uint64 `json:"value"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Value, nil
}
```

---

## Rate Limiting

Currently **no rate limiting** is implemented. For production deployment, consider:

- CDN-level rate limiting (Cloudflare, etc.)
- Per-IP request limits
- API key authentication for high-volume clients

---

## Security Considerations

### PIR Privacy Guarantees

- **Full Set Query**: Information-theoretic privacy (server cannot determine queried index)
- **Plaintext Query**: No privacy (server sees exact index)
- **Set Parity Query**: Privacy depends on set size and distribution

### TLS/HTTPS

For production:
```bash
# Enable HTTPS via reverse proxy (nginx, Caddy)
server {
    listen 443 ssl;
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://plinko-pir-server:3000;
    }
}
```

### CORS

All endpoints allow cross-origin requests (`Access-Control-Allow-Origin: *`). For production, restrict to specific origins.

---

## Monitoring & Observability

### Metrics Collection

Monitor these endpoints:
- `GET /health` - Service health checks
- `GET /metrics` - Update service performance

### Logging

PIR Server logs all queries in privacy mode (no address information):
```
🔒 PRIVATE QUERY RECEIVED
Server CANNOT determine which address was queried
✅ FullSet query completed in 5.2ms
```

### Prometheus Integration (Future)

Planned metrics exports:
- `plinko_query_duration_seconds` (histogram)
- `plinko_query_total` (counter)
- `plinko_update_latency_seconds` (histogram)
- `plinko_database_size_bytes` (gauge)

---

## Troubleshooting

### Connection Refused

```bash
# Check service is running
docker ps | grep plinko-pir-server

# Check port mapping
docker port plinko-pir-server 3000
```

### CORS Errors

Ensure request includes proper headers:
```bash
curl -H "Origin: http://localhost:5173" http://localhost:3000/health -v
```

### Invalid PRF Key

PRF key must be exactly 16 bytes:
```javascript
// Correct: 16 bytes
const prfKey = new Uint8Array(16);

// Incorrect: Wrong size
const badKey = new Uint8Array(8); // ❌ Too small
```

---

## API Changelog

### v1.0 (Current)
- Initial API release
- PIR Server: 4 endpoints
- Update Service: 2 endpoints
- CDN: Static file serving + IPFS proxy

### Planned (v1.1)
- Batch query support
- WebSocket streaming for deltas
- Prometheus metrics export
- API key authentication

---

## Support

- **Issues**: https://github.com/igor53627/plinko-pir-research/issues
- **Documentation**: See `/docs` directory
- **Examples**: See `/services/rabby-wallet` for client implementation
