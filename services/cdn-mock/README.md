# CDN Mock Service (nginx)

**Purpose**: Serve Plinko PIR hints and Plinko deltas for client downloads via HTTP

## Configuration

- **Port**: 8080
- **Root Directory**: `/data`
- **Served Files**:
  - `hint.bin` - Initial Plinko PIR hints (~70 MB)
  - `address-mapping.bin` - Address→index mapping (~192 MB)
  - `deltas/` - Incremental Plinko updates (20-40 KB per block)

## Features

### CORS Support
All endpoints include CORS headers for browser access:
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, HEAD, OPTIONS
Access-Control-Allow-Headers: Range
```

### Caching Policies

**hint.bin** (1 hour):
```
Cache-Control: public, max-age=3600
```
- Hint updated via deltas, so short cache OK
- Clients download once, then apply deltas

**deltas/** (24 hours, immutable):
```
Cache-Control: public, max-age=86400, immutable
```
- Delta files never change once created
- Safe for aggressive caching

**address-mapping.bin** (24 hours):
```
Cache-Control: public, max-age=86400
```
- Rarely changes (only on database regeneration)

### Directory Listing

`/deltas/` has autoindex enabled for browsing:
```
http://localhost:8080/deltas/
```

Shows:
```
delta-000001.bin    24 KB    2024-11-09 10:15
delta-000002.bin    28 KB    2024-11-09 10:15
delta-000003.bin    32 KB    2024-11-09 10:15
...
```

### Range Requests

`hint.bin` supports HTTP Range requests for resumable downloads:
```bash
curl -H "Range: bytes=0-1048576" http://localhost:8080/hint.bin
```

### Compression

Gzip compression enabled for:
- `application/octet-stream` (hint.bin, deltas)
- `application/json` (metadata)

## Usage

### Start with Docker Compose
```bash
docker-compose up cdn-mock
```

### Manual Testing
```bash
# Build service
docker-compose build cdn-mock

# Run service
docker-compose up -d cdn-mock

# Test health endpoint
curl http://localhost:8080/health

# Download hint
curl -I http://localhost:8080/hint.bin

# List deltas
curl http://localhost:8080/deltas/

# Download specific delta
curl http://localhost:8080/deltas/delta-000001.bin -o delta.bin
```

### Browser Testing
```javascript
// Fetch hint with CORS
fetch('http://localhost:8080/hint.bin')
  .then(response => response.arrayBuffer())
  .then(data => console.log('Hint downloaded:', data.byteLength, 'bytes'));

// List deltas
fetch('http://localhost:8080/deltas/')
  .then(response => response.text())
  .then(html => console.log('Delta directory:', html));
```

## Endpoints

### GET /health
Health check endpoint

**Response**:
```
HTTP/1.1 200 OK
Content-Type: text/plain

healthy
```

### GET /hint.bin
Download Plinko PIR hints

**Response**:
```
HTTP/1.1 200 OK
Content-Length: 67108896
Content-Type: application/octet-stream
Cache-Control: public, max-age=3600
Access-Control-Allow-Origin: *
Accept-Ranges: bytes

[binary data]
```

**Size**: ~67 MB (64 MB database + 32 byte header)

### GET /address-mapping.bin
Download address→index mapping

**Response**:
```
HTTP/1.1 200 OK
Content-Length: 201326592
Content-Type: application/octet-stream
Cache-Control: public, max-age=86400
Access-Control-Allow-Origin: *

[binary data]
```

**Size**: ~192 MB (8.4M accounts × 24 bytes)

### GET /deltas/
Browse delta files (HTML directory listing)

**Response**:
```html
<html>
<head><title>Index of /deltas/</title></head>
<body>
<h1>Index of /deltas/</h1>
<hr><pre>
<a href="../">../</a>
<a href="delta-000001.bin">delta-000001.bin</a>    24-Nov-2024 10:15    24576
<a href="delta-000002.bin">delta-000002.bin</a>    24-Nov-2024 10:15    28672
...
</pre><hr>
</body>
</html>
```

### GET /deltas/delta-XXXXXX.bin
Download specific delta file

**Response**:
```
HTTP/1.1 200 OK
Content-Length: 24576
Content-Type: application/octet-stream
Cache-Control: public, max-age=86400, immutable
Access-Control-Allow-Origin: *

[binary data]
```

**Size**: 20-40 KB per delta (varies with number of changes)

## Performance

### Bandwidth

**Initial Download** (first-time user):
- `hint.bin`: ~70 MB
- `address-mapping.bin`: ~192 MB
- **Total**: ~262 MB (one-time)

**Incremental Updates** (per block):
- Delta file: ~30 KB average
- **Per day**: ~30 KB × 7,200 blocks = ~216 MB
- **Per month**: ~6.3 GB

**Optimization**: Aggregate deltas weekly to reduce bandwidth

### Caching

With proper caching:
- Hint: 1-hour cache = ~24 requests/day → ~1.7 GB/day
- Deltas: Immutable = 1 request per delta = ~216 MB/day
- **Total**: ~2 GB/day per active user

### Compression

Gzip reduces transfer size:
- Hint: 70 MB → ~50 MB (30% savings)
- Deltas: 30 KB → ~20 KB (33% savings)

## nginx Configuration

### Key Features

**CORS for all endpoints**:
```nginx
add_header 'Access-Control-Allow-Origin' '*' always;
add_header 'Access-Control-Allow-Methods' 'GET, HEAD, OPTIONS' always;
```

**Directory listing for deltas**:
```nginx
location /deltas/ {
    autoindex on;
    autoindex_exact_size off;
    autoindex_localtime on;
}
```

**Immutable deltas**:
```nginx
location /deltas/ {
    add_header 'Cache-Control' 'public, max-age=86400, immutable' always;
}
```

**Range requests for hint**:
```nginx
location = /hint.bin {
    add_header 'Accept-Ranges' 'bytes' always;
}
```

## Files

- `nginx.conf` - nginx configuration with CORS, caching, autoindex
- `Dockerfile` - nginx:alpine image with custom config
- `README.md` - This file

## Troubleshooting

**Problem**: 403 Forbidden
- Check file permissions in `/data`
- Ensure nginx user can read files
- Run: `chmod -R 755 /data`

**Problem**: CORS errors in browser
- Check `Access-Control-Allow-Origin` header present
- Verify preflight OPTIONS requests handled
- Check browser console for specific error

**Problem**: Slow downloads
- Enable gzip compression (already configured)
- Check network bandwidth
- Consider CDN in production

**Problem**: Files not found
- Verify hint-generator and plinko-update services completed
- Check `/data` volume mounted correctly
- Look at nginx access logs

**Problem**: Directory listing empty
- Ensure delta files created in `/data/deltas/`
- Check plinko-update-service is running
- Verify blocks are being mined

## Production Considerations

### Real CDN

Replace nginx with production CDN:

**CloudFlare**:
- Global edge network
- Automatic compression
- DDoS protection
- Free tier available

**Fastly**:
- Edge computing (VCL/Compute@Edge)
- Real-time purging
- Advanced caching rules

**AWS CloudFront**:
- S3 origin integration
- Lambda@Edge for customization
- Pay-as-you-go pricing

### Upload Strategy

**Hint uploads**:
```bash
# Upload new hint to CDN
aws s3 cp /data/hint.bin s3://my-bucket/hint.bin \
  --cache-control "max-age=3600" \
  --content-type "application/octet-stream"

# Invalidate CloudFront cache
aws cloudfront create-invalidation \
  --distribution-id XXXXX \
  --paths "/hint.bin"
```

**Delta uploads** (automated):
```bash
# Watch for new deltas and upload
inotifywait -m /data/deltas/ -e create |
  while read dir action file; do
    aws s3 cp "/data/deltas/$file" "s3://my-bucket/deltas/$file" \
      --cache-control "max-age=86400, immutable"
  done
```

### Delta Pruning

Keep only recent deltas:
```bash
# Keep last 30 days of deltas
find /data/deltas/ -name "delta-*.bin" -mtime +30 -delete
```

Or implement client-side aggregation:
- Client requests deltas for blocks N to M
- Server aggregates into single delta
- Reduces request count

### Security

**HTTPS** (required in production):
```nginx
server {
    listen 443 ssl http2;
    ssl_certificate /etc/ssl/certs/cert.pem;
    ssl_certificate_key /etc/ssl/private/key.pem;

    # Modern TLS config
    ssl_protocols TLSv1.3;
    ssl_prefer_server_ciphers off;
}
```

**Rate Limiting**:
```nginx
limit_req_zone $binary_remote_addr zone=cdn:10m rate=10r/s;

server {
    location /deltas/ {
        limit_req zone=cdn burst=20;
    }
}
```

**Access Logs** (for analytics):
```nginx
log_format cdn '$remote_addr - [$time_local] "$request" '
               '$status $body_bytes_sent "$http_user_agent" '
               '$request_time';

access_log /var/log/nginx/cdn.log cdn;
```

## Client Integration

### JavaScript Example

```javascript
class PianoPIRClient {
  constructor(cdnUrl = 'http://localhost:8080') {
    this.cdnUrl = cdnUrl;
  }

  async downloadHint() {
    const response = await fetch(`${this.cdnUrl}/hint.bin`);
    return await response.arrayBuffer();
  }

  async listDeltas() {
    const response = await fetch(`${this.cdnUrl}/deltas/`);
    const html = await response.text();
    // Parse HTML to extract delta filenames
    return this.parseDeltaList(html);
  }

  async downloadDelta(blockNumber) {
    const filename = `delta-${blockNumber.toString().padStart(6, '0')}.bin`;
    const response = await fetch(`${this.cdnUrl}/deltas/${filename}`);
    return await response.arrayBuffer();
  }

  async syncDeltas(fromBlock, toBlock) {
    const deltas = [];
    for (let block = fromBlock; block <= toBlock; block++) {
      deltas.push(await this.downloadDelta(block));
    }
    return deltas;
  }
}
```

## Next Steps

After CDN Mock:
1. **Ambire Wallet Integration**: Client implementation with Privacy Mode
2. **Hint Download**: Client downloads hint.bin on first use
3. **Delta Sync**: Client applies deltas to keep hint current
4. **Private Queries**: Client sends queries to Plinko PIR Server

---

**Status**: Static File Serving ✅ | CORS ✅ | Caching ✅ | Directory Listing ✅
