# CDN Mock Service (nginx)

**Purpose**: Serve public Plinko snapshot packages, manifests, address-mapping files, and delta feeds over HTTP for browsers/SDKs.

## Configuration

- **Port**: 8080
- **Root Directory**: `/public`
- **Served Paths**:
  - `snapshots/<version>/database.bin` – canonical snapshot chunks (clients derive hints locally)
  - `snapshots/<version>/manifest.json` – integrity + hash manifest
  - `address-mapping.bin` – Address→index mapping (~192 MB)
  - `deltas/` – Incremental XOR updates per block (20-40 KB)
- `ipfs/<cid>` – Reverse proxy to the bundled `ipfs/kubo` gateway for browsers that cannot speak `ipfs://` natively

## Features

### CORS Support
All endpoints emit permissive headers for browser access:
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, HEAD, OPTIONS
Access-Control-Allow-Headers: Range
Access-Control-Expose-Headers: Content-Length, Content-Range
```

### Caching Policies

**Snapshots (`/snapshots/`)** – immutable packages
```
Cache-Control: public, max-age=86400, immutable
```
Clients download a version once, verify hashes, then derive hints locally.

**Delta feeds (`/deltas/`)** – append-only
```
Cache-Control: public, max-age=86400, immutable
```

**Address mapping** – rare changes
```
Cache-Control: public, max-age=86400
```

### Directory Listings

Both `/snapshots/` and `/deltas/` expose `autoindex` listings for debugging:
```
http://localhost:8080/snapshots/
http://localhost:8080/deltas/
```

### Range Requests

Snapshot binaries support HTTP range requests for resumable downloads:
```bash
curl -H "Range: bytes=0-1048576" \
  http://localhost:8080/snapshots/latest/database.bin
```

### Compression

Gzip enabled for:
- `application/octet-stream` (snapshots, deltas)
- `application/json` (manifests)

## Usage

### Start with Docker Compose
```bash
docker compose up cdn-mock
```

### Manual Testing
```bash
# Build image
docker compose build cdn-mock

# Run detached
docker compose up -d cdn-mock

# Health
curl http://localhost:8080/health

# List snapshot versions
curl http://localhost:8080/snapshots/

# Download manifest
curl http://localhost:8080/snapshots/latest/manifest.json

# List deltas
curl http://localhost:8080/deltas/
```

### Browser Testing
```javascript
// Fetch manifest + snapshot chunk
const manifest = await fetch('http://localhost:8080/snapshots/latest/manifest.json').then(r => r.json());
const snapshot = await fetch(`http://localhost:8080/${manifest.database.path}`).then(r => r.arrayBuffer());
console.log('Snapshot bytes:', snapshot.byteLength);
```

## Endpoints

### GET /health
Simple health check (`200 healthy`).

### GET /snapshots/
Lists available snapshot versions (autoindex HTML). Each snapshot folder typically contains:
- `database.bin`
- `manifest.json`
- optional integrity proofs

### GET /snapshots/<version>/manifest.json
JSON manifest describing hashes and chunk sizes for the snapshot package.

### GET /snapshots/<version>/database.bin
Binary snapshot for the canonical database. Enables range requests and immutable caching.

### GET /address-mapping.bin
Address→index mapping file (20-byte address + 4-byte index per row).

### GET /deltas/
Directory listing of per-block XOR delta files.

### GET /deltas/delta-XXXXXX.bin
Download individual delta artifacts (immutable, cached for 24h).

### GET /ipfs/<cid>
Reverse proxy to the local `ipfs` service (`ipfs/kubo` gateway on port 8080), preserving permissive CORS headers so browsers can fetch snapshot chunks directly from IPFS using only the CDN origin.

## Performance

- **Initial snapshot**: ~70 MB download (once per client version)
- **Per-block deltas**: 20-40 KB
- **CORS + range support** keeps downloads resumable and cacheable via CDN.
