# Plinko PIR PoC - Quick Reference

## Service URLs (Default - localhost)

```
Wallet:          http://localhost:5173
PIR Server:      http://localhost:3000
CDN:             http://localhost:8080
Update Service:  http://localhost:3001
```

## Quick Start

```bash
# Start everything
make start

# View logs
make logs

# Stop everything
docker-compose down

# Reset and clean
make reset
```

## Testing

```bash
# Privacy tests
make test

# Performance tests
make test-performance

# Manual test
curl http://localhost:3000/health
```

## Common Commands

```bash
# Check service status
docker-compose ps

# View specific service logs
docker logs plinko-pir-server
docker logs plinko-wallet
docker logs plinko-pir-cdn

# Restart a service
docker-compose restart plinko-pir-server

# Rebuild a service
docker-compose build ambire-wallet
docker-compose up -d ambire-wallet
```

## Service Endpoints

### Plinko PIR Server (port 3000)

```bash
# Health check
curl http://localhost:3000/health

# Plaintext query (testing only)
curl -X POST http://localhost:3000/query/plaintext \
  -H "Content-Type: application/json" \
  -d '{"index": 42}'

# FullSet PIR query (private)
curl -X POST http://localhost:3000/query/fullset \
  -H "Content-Type: application/json" \
  -d '{"query": "<base64-encoded-query>"}'
```

### CDN Mock (port 8080)

```bash
# Health check
curl http://localhost:8080/health

# Download hint
curl -O http://localhost:8080/hint.bin

# List deltas
curl http://localhost:8080/deltas/

# Download specific delta
curl -O http://localhost:8080/deltas/delta-000001.bin
```

### Plinko Update Service (port 3001)

```bash
# Check if service is running (no HTTP endpoint)
docker logs plinko-pir-updates

# View update activity
docker logs plinko-pir-updates --follow
```

## Docker Internal Addresses

Used by services communicating within Docker network:

```
eth-mock:8545                (Ethereum RPC)
plinko-pir-server:3000       (PIR queries)
cdn-mock:8080                (CDN)
plinko-pir-updates:3001      (Updates)
```

## File Locations

### Inside Containers

```
/data/database.bin           (64 MB)
/data/address-mapping.bin    (192 MB)
/data/hint.bin              (~70 MB)
/data/deltas/               (delta files)
```

### On Host (Docker volume)

```bash
# Find volume location
docker volume inspect plinko-pir-poc_shared-data

# List files (macOS/Linux)
docker run --rm -v plinko-pir-poc_shared-data:/data alpine ls -lh /data
```

## Port Mapping

```
5173 → 80    (Wallet nginx)
3000 → 3000  (PIR Server)
8080 → 8080  (CDN nginx)
3001 → 3001  (Update Service)
(no external mapping for Anvil)
```

## Troubleshooting

### Services not starting

```bash
# Check Docker is running
docker ps

# Check for port conflicts
lsof -i :3000
lsof -i :5173
lsof -i :8080

# View service logs
docker-compose logs
```

### Wallet not loading

```bash
# Check container is running
docker ps | grep plinko-wallet

# View logs
docker logs plinko-wallet

# Rebuild
docker-compose build ambire-wallet
docker-compose up -d ambire-wallet
```

### Database generation stuck

```bash
# Check Anvil is responding
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# View generator logs
docker logs plinko-pir-db-generator
```

### PIR queries failing

```bash
# Test PIR server
curl http://localhost:3000/health

# Check hint exists
docker exec plinko-pir-server ls -lh /data/hint.bin

# View server logs
docker logs plinko-pir-server
```

## Environment Variables

### Default Configuration

```bash
VITE_PIR_SERVER_URL=http://localhost:3000
VITE_CDN_URL=http://localhost:8080
VITE_FALLBACK_RPC=https://eth.llamarpc.com
```

### Custom Domains (Optional)

```bash
# 1. Edit /etc/hosts
sudo nano /etc/hosts

# Add:
127.0.0.1  plinko-server.local
127.0.0.1  plinko-cdn.local
127.0.0.1  plinko-wallet.local

# 2. Create .env
cp .env.example .env

# 3. Update URLs
VITE_PIR_SERVER_URL=http://plinko-server.local:3000
VITE_CDN_URL=http://plinko-cdn.local:8080

# 4. Rebuild
docker-compose down
docker-compose build ambire-wallet
docker-compose up -d
```

## Performance Metrics

### Expected Performance

```
Query Latency:       ~5-8ms
Update Latency:      ~24μs (2,000 accounts)
Delta Size:          ~30 KB per block
Hint Download:       ~1-2 seconds
Delta Application:   ~5ms
```

### Test Performance

```bash
# Automated performance tests
./scripts/test-performance.sh

# Manual timing
time curl http://localhost:3000/health
```

## Privacy Verification

```bash
# CRITICAL: Server logs should show NO addresses
docker logs plinko-pir-server | grep "0x"
# Expected: No output

# Run privacy test suite
./scripts/test-privacy.sh
```

## Network Architecture

```
┌──────────────────────────────────────────┐
│ Host Machine (macOS/Linux/Windows)      │
│                                          │
│  Browser → localhost:5173 (Wallet)      │
│         → localhost:3000 (PIR Server)   │
│         → localhost:8080 (CDN)          │
└──────────────────────────────────────────┘
                    ↓
┌──────────────────────────────────────────┐
│ Docker Network: plinko-pir-network       │
│                                          │
│  ambire-wallet:80                        │
│  plinko-pir-server:3000                  │
│  cdn-mock:8080                           │
│  eth-mock:8545 (internal only)           │
└──────────────────────────────────────────┘
```

## Resources

- **Main Documentation**: [README.md](../README.md)
- **Service Addressing**: [SERVICE_ADDRESSING.md](SERVICE_ADDRESSING.md)
- **Makefile**: [../Makefile](../Makefile)
- **Environment Variables**: [../.env.example](../.env.example)

## Support

For detailed configuration options, see:
- [docs/SERVICE_ADDRESSING.md](SERVICE_ADDRESSING.md) - Network configuration
- [README.md](../README.md) - Full PoC documentation
- [scripts/](../scripts/) - Utility scripts

---

**Quick Test**: `curl http://localhost:3000/health && echo "✓ PIR Server is running"`
