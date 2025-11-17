# Troubleshooting Guide

Common issues and solutions for Plinko PIR deployment and development.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Service Issues](#service-issues)
- [Network & Connectivity](#network--connectivity)
- [Database & Data Issues](#database--data-issues)
- [IPFS & CDN Issues](#ipfs--cdn-issues)
- [Performance Problems](#performance-problems)
- [Development Environment](#development-environment)

---

## Quick Diagnostics

### Health Check All Services

```bash
# Check all service health endpoints
curl http://localhost:3000/health  # PIR Server
curl http://localhost:3001/health  # Update Service
curl http://localhost:8080/health  # CDN

# Check Docker Compose status
docker-compose ps

# View logs for all services
docker-compose logs --tail=50
```

### Common Quick Fixes

```bash
# Restart all services
docker-compose restart

# Rebuild and restart (after code changes)
docker-compose up -d --build

# Clean restart (removes volumes - DATA LOSS!)
docker-compose down -v
docker-compose up -d --build
```

---

## Service Issues

### PIR Server Not Starting

**Symptoms**:
- Container exits immediately
- Error: "Failed to read database file"
- Port 3000 not accessible

**Diagnosis**:
```bash
# Check container logs
docker-compose logs plinko-pir-server

# Check if database file exists
docker-compose exec plinko-pir-server ls -lh /data/database.bin

# Check file permissions
docker-compose exec plinko-pir-server stat /data/database.bin
```

**Solutions**:

**1. Missing Database File**:
```bash
# Generate database from parquet files
python3 scripts/build_database_from_parquet.py \
  --input raw_balances \
  --output data

# OR use mock data for testing
docker-compose up eth-mock
# Wait for mock to generate database
```

**2. Corrupted Database**:
```bash
# Check file size (should be multiple of 8 bytes)
ls -l data/database.bin

# Rebuild database
rm data/database.bin
python3 scripts/build_database_from_parquet.py --input raw_balances --output data
```

**3. Port Conflict**:
```bash
# Check if port 3000 is already in use
lsof -i :3000

# Change port in docker-compose.yml
ports:
  - "3001:3000"  # Use 3001 externally instead
```

---

### Update Service Not Processing Blocks

**Symptoms**:
- No new deltas in `/data/deltas/`
- Logs show "waiting for new block" repeatedly
- Metrics show stale `last_processed_block`

**Diagnosis**:
```bash
# Check update service logs
docker-compose logs plinko-update-service --tail=100

# Check if eth-mock is producing blocks
docker-compose logs eth-mock | grep "Block"

# Verify delta directory
docker-compose exec plinko-update-service ls -la /data/deltas/
```

**Solutions**:

**1. eth-mock Not Running**:
```bash
# Restart eth-mock
docker-compose restart eth-mock

# Check eth-mock is reachable
docker-compose exec plinko-update-service curl http://eth-mock:8545 -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

**2. Network Connectivity**:
```bash
# Verify services are on same network
docker network ls
docker network inspect plinko-pir_plinko-network

# Ensure services can ping each other
docker-compose exec plinko-update-service ping eth-mock
```

**3. Volume Permissions**:
```bash
# Check volume permissions
docker-compose exec plinko-update-service ls -la /data

# Fix permissions (if needed)
docker-compose exec plinko-update-service chmod 777 /data/deltas
```

---

### State Syncer Not Syncing

**Symptoms**:
- No snapshots being generated
- HTTP endpoint (port 3002) not responding
- IPFS errors in logs

**Diagnosis**:
```bash
# Check state-syncer logs
docker-compose logs state-syncer --tail=100

# Check if IPFS is reachable
docker-compose exec state-syncer curl http://ipfs:5001/api/v0/version

# Verify HTTP endpoint
curl http://localhost:3002/metrics
```

**Solutions**:

**1. IPFS Not Running**:
```bash
# Check IPFS container
docker-compose ps ipfs

# Restart IPFS
docker-compose restart ipfs

# Wait for IPFS to be ready
docker-compose logs ipfs | grep "Daemon is ready"
```

**2. Hypersync Connection Issues**:
```bash
# Check environment variables
docker-compose exec state-syncer env | grep PLINKO_STATE

# Test Hypersync endpoint
curl https://eth.rpc.hypersync.xyz -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

**3. Simulated Mode Issues**:
```bash
# Ensure PLINKO_STATE_SIMULATED is set correctly
# For PoC: PLINKO_STATE_SIMULATED=true
# For production: PLINKO_STATE_SIMULATED=false

# Check .env file
cat .env | grep PLINKO_STATE_SIMULATED
```

---

## Network & Connectivity

### Cannot Access Services from Host

**Symptoms**:
- `curl http://localhost:3000` fails
- Browser cannot connect to wallet at localhost:5173
- "Connection refused" errors

**Diagnosis**:
```bash
# Check port mappings
docker-compose ps

# Verify ports are exposed
docker port plinko-pir-server 3000
docker port rabby-wallet 5173

# Check if ports are listening
netstat -an | grep LISTEN | grep -E '3000|5173|8080'
```

**Solutions**:

**1. Services Not Started**:
```bash
# Start all services
docker-compose up -d

# Wait for services to be healthy
sleep 10
docker-compose ps
```

**2. Firewall Blocking Ports**:
```bash
# macOS: Check firewall
sudo pfctl -s all

# Linux: Check iptables
sudo iptables -L -n

# Temporary: Disable firewall (testing only!)
# macOS: System Preferences → Security & Privacy → Firewall
# Linux: sudo ufw disable
```

**3. Docker Network Issues**:
```bash
# Recreate Docker network
docker-compose down
docker network prune -f
docker-compose up -d
```

---

### Services Cannot Communicate Internally

**Symptoms**:
- eth-mock unreachable from update service
- PIR server cannot reach CDN
- DNS resolution failures

**Diagnosis**:
```bash
# Check if services are on same network
docker network inspect plinko-pir_plinko-network

# Test DNS resolution
docker-compose exec plinko-update-service nslookup eth-mock
docker-compose exec plinko-update-service ping -c 3 eth-mock
```

**Solutions**:

**1. Network Misconfiguration**:
```yaml
# Verify all services use same network in docker-compose.yml
networks:
  - plinko-network
```

**2. Service Name Typos**:
```bash
# Check service names match docker-compose.yml
docker-compose config --services

# Use exact service names (case-sensitive):
# eth-mock (not ethereum-mock or eth_mock)
# plinko-pir-server (not pir-server)
```

**3. Network Recreation**:
```bash
# Force network recreation
docker-compose down
docker network rm plinko-pir_plinko-network
docker-compose up -d
```

---

### Port 8545 Externally Exposed (Security Issue)

**Symptoms**:
- `curl http://localhost:8545` succeeds (should fail)
- eth-mock accessible from outside Docker

**Diagnosis**:
```bash
# Check if port 8545 is exposed
docker port plinko-pir-eth-mock 8545

# Test external access (should fail)
curl http://localhost:8545
```

**Solution**:
```yaml
# Remove port mapping from docker-compose.yml
# BEFORE (incorrect):
eth-mock:
  ports:
    - "8545:8545"

# AFTER (correct):
eth-mock:
  # No ports section - internal only
```

```bash
# Apply fix
docker-compose down
docker-compose up -d

# Verify (should return empty)
docker port plinko-pir-eth-mock 8545
```

---

## Database & Data Issues

### Database Size Mismatch

**Symptoms**:
- PIR queries return wrong values
- Database size reported as 0
- "Invalid database file" errors

**Diagnosis**:
```bash
# Check database file size
ls -lh data/database.bin

# Expected: ~43 MB for 5.6M accounts (production)
# Expected: ~67 MB for 8.4M accounts (PoC)

# Check if file is multiple of 8 bytes
stat data/database.bin | grep Size

# Verify database integrity
docker-compose exec plinko-pir-server /bin/sh -c \
  "wc -c < /data/database.bin"
```

**Solutions**:

**1. Incomplete Database Generation**:
```bash
# Rebuild database
rm data/database.bin
python3 scripts/build_database_from_parquet.py \
  --input raw_balances \
  --output data

# Verify file size
ls -lh data/database.bin
```

**2. Wrong Database for Environment**:
```bash
# PoC/Development: Should use simulated 8.4M accounts
# Production: Should use real 5.6M accounts

# Check which service is running
docker-compose ps | grep -E "eth-mock|state-syncer"

# Use correct database source
# PoC: eth-mock generates database automatically
# Production: Use build_database_from_parquet.py
```

---

### Missing Address Mapping

**Symptoms**:
- Cannot look up balances by address
- address-mapping.bin not found
- Client wallet cannot derive hints

**Diagnosis**:
```bash
# Check if mapping file exists
ls -lh data/address-mapping.bin

# Expected size: ~128 MB for 5.6M accounts
# Expected size: ~193 MB for 8.4M accounts

# Verify file is accessible
curl http://localhost:8080/address-mapping.bin -I
```

**Solutions**:

**1. Rebuild Address Mapping**:
```bash
# Generate from parquet files
python3 scripts/build_database_from_parquet.py \
  --input raw_balances \
  --output data

# Verify both files exist
ls -lh data/database.bin data/address-mapping.bin
```

**2. CDN Not Serving File**:
```bash
# Check CDN is serving public directory
docker-compose logs cdn-mock

# Verify file is in public directory
docker-compose exec cdn-mock ls -la /public/

# Restart CDN
docker-compose restart cdn-mock
```

---

## IPFS & CDN Issues

### IPFS Daemon Not Starting

**Symptoms**:
- state-syncer cannot pin snapshots
- "connection refused" to ipfs:5001
- /ipfs/{cid} returns 502 Bad Gateway

**Diagnosis**:
```bash
# Check IPFS container status
docker-compose ps ipfs

# Check IPFS logs
docker-compose logs ipfs --tail=50

# Test IPFS API
curl http://localhost:5001/api/v0/version
```

**Solutions**:

**1. IPFS Not Running**:
```bash
# Start IPFS
docker-compose up -d ipfs

# Wait for "Daemon is ready"
docker-compose logs ipfs -f | grep "Daemon is ready"
```

**2. IPFS Repo Corruption**:
```bash
# Remove IPFS volume and recreate
docker-compose down
docker volume rm plinko-pir_ipfs-data
docker-compose up -d ipfs
```

**3. IPFS Port Conflicts**:
```bash
# Check if ports 4001, 5001, 8080 are in use
lsof -i :4001
lsof -i :5001
lsof -i :8081  # IPFS gateway (internal)

# Change port mappings if needed
```

---

### CDN 404 Errors

**Symptoms**:
- GET /snapshots/manifest.json returns 404
- GET /deltas/delta-{block}.bin fails
- Directory listing empty

**Diagnosis**:
```bash
# Check public directory contents
docker-compose exec cdn-mock ls -la /public/snapshots/
docker-compose exec cdn-mock ls -la /public/deltas/

# Check nginx logs
docker-compose logs cdn-mock --tail=50

# Test CDN health
curl http://localhost:8080/health
```

**Solutions**:

**1. Empty Public Directory**:
```bash
# Ensure services are writing to shared volume
docker-compose exec plinko-update-service ls -la /public/deltas/
docker-compose exec state-syncer ls -la /public/snapshots/

# Check volume mounts
docker-compose config | grep -A 5 "public-artifacts"
```

**2. Nginx Configuration Error**:
```bash
# Check nginx config syntax
docker-compose exec cdn-mock nginx -t

# Reload nginx
docker-compose exec cdn-mock nginx -s reload
```

**3. Permissions Issues**:
```bash
# Fix public directory permissions
docker-compose exec cdn-mock chmod -R 755 /public
```

---

### IPFS Content Not Accessible

**Symptoms**:
- GET /ipfs/{cid} returns 404 or timeout
- "Content not found" errors
- Slow IPFS gateway responses

**Diagnosis**:
```bash
# Check IPFS gateway proxy
curl http://localhost:8080/ipfs/QmTest -I

# Check IPFS directly
docker-compose exec ipfs ipfs cat QmTest

# Verify content is pinned
docker-compose exec ipfs ipfs pin ls
```

**Solutions**:

**1. Content Not Pinned**:
```bash
# Check state-syncer logs for pinning errors
docker-compose logs state-syncer | grep -i ipfs

# Manually pin content
docker-compose exec ipfs ipfs pin add QmYourCID
```

**2. IPFS Gateway Timeout**:
```yaml
# Increase proxy timeout in nginx.conf
location /ipfs/ {
    proxy_pass http://ipfs:8080$request_uri;
    proxy_read_timeout 300s;  # Add this
}
```

**3. Using External IPFS Gateway**:
```bash
# Switch to web3.storage or Pinata
export PLINKO_STATE_IPFS_API=https://api.web3.storage/upload
export PLINKO_STATE_IPFS_GATEWAY=https://w3s.link/ipfs

# Update docker-compose.yml
docker-compose up -d --build
```

---

## Performance Problems

### Slow Query Response Times

**Symptoms**:
- /query/fullset takes >50ms (expected: ~5ms)
- High CPU usage on PIR server
- Queries timeout

**Diagnosis**:
```bash
# Monitor query latency
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:3000/query/plaintext?index=100

# Create curl-format.txt:
echo 'time_total: %{time_total}s\n' > curl-format.txt

# Check PIR server CPU usage
docker stats plinko-pir-server --no-stream

# Monitor logs
docker-compose logs plinko-pir-server -f | grep "completed in"
```

**Solutions**:

**1. Database Not in Memory**:
```bash
# Increase Docker memory limit (Docker Desktop)
# Settings → Resources → Memory: 8 GB minimum

# For 8.4M accounts: 16 GB recommended
# For 5.6M accounts: 8 GB minimum
```

**2. Disk I/O Bottleneck**:
```bash
# Use tmpfs for database (Linux only)
# Add to docker-compose.yml:
tmpfs:
  - /data:size=2G,mode=1777

# Warm up cache
for i in {1..1000}; do
  curl -s "http://localhost:3000/query/plaintext?index=$i" > /dev/null
done
```

**3. Network Latency**:
```bash
# Test localhost latency
ping -c 10 localhost

# Use host network mode for testing
# docker-compose.yml:
network_mode: host
```

---

### High Memory Usage

**Symptoms**:
- Docker containers being killed (OOM)
- System slowdown
- "Out of memory" errors

**Diagnosis**:
```bash
# Check memory usage
docker stats --no-stream

# Check system memory
free -h  # Linux
vm_stat  # macOS

# Check container limits
docker inspect plinko-pir-server | grep -i memory
```

**Solutions**:

**1. Increase Docker Memory**:
```bash
# Docker Desktop: Settings → Resources → Memory
# Recommended: 16 GB for PoC (8.4M), 8 GB for production (5.6M)

# Linux: Edit /etc/docker/daemon.json
{
  "default-runtime": "runc",
  "default-shm-size": "2G"
}
```

**2. Reduce Database Size**:
```bash
# Use production dataset (5.6M) instead of PoC (8.4M)
# ~40% memory reduction

# Switch from eth-mock to state-syncer
docker-compose stop eth-mock
docker-compose up -d state-syncer
```

**3. Set Memory Limits**:
```yaml
# docker-compose.yml
services:
  plinko-pir-server:
    mem_limit: 4g
    memswap_limit: 4g
```

---

### Update Service Lagging Behind Blocks

**Symptoms**:
- Update service processes blocks slower than 12s block time
- Delta backlog growing
- High update_latency_ms in /metrics

**Diagnosis**:
```bash
# Check metrics
curl http://localhost:3001/metrics

# Monitor update speed
docker-compose logs plinko-update-service -f | grep "Update applied"

# Check CPU usage
docker stats plinko-update-service --no-stream
```

**Solutions**:

**1. Optimize Batch Size**:
```bash
# Reduce batch size in update service
# Edit services/plinko-update-service/main.go
const batchSize = 1000  # Try smaller batches
```

**2. Use SSD Storage**:
```bash
# Ensure /data volume is on SSD
df -h /var/lib/docker

# Move Docker data directory to SSD
# Edit /etc/docker/daemon.json
{
  "data-root": "/path/to/ssd/docker"
}
```

**3. Parallel Processing**:
```bash
# Run multiple update workers (future enhancement)
# Currently single-threaded by design
```

---

## Development Environment

### Hot Reload Not Working (Wallet)

**Symptoms**:
- Changes to wallet code don't reflect
- Need to rebuild container for every change
- Vite HMR not triggering

**Diagnosis**:
```bash
# Check volume mount
docker-compose config | grep -A 5 "rabby-wallet" | grep volumes

# Verify files are mounted
docker-compose exec rabby-wallet ls -la /app/src

# Check Vite logs
docker-compose logs rabby-wallet -f
```

**Solutions**:

**1. Volume Mount Missing**:
```yaml
# Ensure source is mounted in docker-compose.yml
rabby-wallet:
  volumes:
    - ./services/rabby-wallet:/app
    - /app/node_modules  # Exclude node_modules
```

**2. File Watcher Limits (Linux)**:
```bash
# Increase inotify watchers
echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

**3. macOS Performance Issues**:
```yaml
# Use :cached or :delegated mount
volumes:
  - ./services/rabby-wallet:/app:cached
```

---

### Tests Failing

**Symptoms**:
- `go test ./...` fails
- Test timeouts
- Flaky tests

**Diagnosis**:
```bash
# Run tests with verbose output
cd services/state-syncer
go test -v ./...

# Run specific test
go test -v -run TestIPRFInverse

# Check for race conditions
go test -race ./...
```

**Solutions**:

**1. Missing Dependencies**:
```bash
# Install Go dependencies
cd services/state-syncer
go mod download
go mod tidy
```

**2. Test Data Missing**:
```bash
# Ensure test data exists
ls -la services/state-syncer/testdata/

# Regenerate test data if needed
go test -v -run TestGenerateTestData
```

**3. Timing Issues**:
```bash
# Increase test timeout
go test -timeout 5m ./...

# Run tests sequentially
go test -p 1 ./...
```

---

### Build Failures

**Symptoms**:
- `docker-compose build` fails
- "no such file or directory" errors
- Dependency resolution errors

**Diagnosis**:
```bash
# Build with verbose output
docker-compose build --progress=plain

# Check Dockerfile syntax
docker-compose config

# Verify source files exist
ls -la services/*/
```

**Solutions**:

**1. Cache Issues**:
```bash
# Clear build cache
docker-compose build --no-cache

# Prune Docker system
docker system prune -a -f
```

**2. Missing Build Context**:
```bash
# Ensure build context includes required files
# Check .dockerignore

# Verify Dockerfile COPY paths
docker-compose config | grep -A 10 "build:"
```

**3. Dependency Version Conflicts**:
```bash
# Go: Update dependencies
cd services/state-syncer
go get -u ./...
go mod tidy

# Node.js: Clear cache
rm -rf services/rabby-wallet/node_modules
docker-compose build rabby-wallet --no-cache
```

---

## Advanced Debugging

### Enable Debug Logging

```bash
# PIR Server
docker-compose exec plinko-pir-server /bin/sh -c 'LOG_LEVEL=debug ./pir-server'

# Update Service
docker-compose exec plinko-update-service /bin/sh -c 'LOG_LEVEL=debug ./update-service'

# Docker Compose
docker-compose --verbose up -d
```

### Inspect Container Filesystems

```bash
# Enter container shell
docker-compose exec plinko-pir-server /bin/sh

# Inspect file permissions
ls -la /data/

# Check disk usage
du -sh /data/*

# View environment variables
env | sort
```

### Network Packet Capture

```bash
# Install tcpdump in container
docker-compose exec plinko-pir-server apk add tcpdump

# Capture HTTP traffic
docker-compose exec plinko-pir-server tcpdump -i any port 3000 -w /tmp/capture.pcap

# Copy capture file to host
docker cp plinko-pir-server:/tmp/capture.pcap ./
```

### Memory Profiling

```bash
# Go services: Enable pprof
# Add to main.go:
import _ "net/http/pprof"

# Access profiling endpoint
curl http://localhost:6060/debug/pprof/heap > heap.prof

# Analyze with go tool
go tool pprof heap.prof
```

---

## Getting Help

### Before Opening an Issue

1. **Check logs**:
   ```bash
   docker-compose logs > debug.log
   ```

2. **Collect system info**:
   ```bash
   docker version
   docker-compose version
   uname -a
   free -h
   ```

3. **Test minimal reproduction**:
   ```bash
   docker-compose down -v
   docker-compose up -d
   # Try again
   ```

### Reporting Issues

Include:
- Operating system and version
- Docker and Docker Compose versions
- Complete error messages
- Steps to reproduce
- Relevant logs

**GitHub Issues**: https://github.com/igor53627/plinko-pir-research/issues

---

## Quick Reference

### Essential Commands

```bash
# Health checks
make health-check  # Check all services

# Restart everything
make restart

# Clean slate (⚠️ DATA LOSS!)
make clean && make build && make start

# View all logs
make logs

# View specific service
docker-compose logs -f plinko-pir-server

# Enter container
docker-compose exec plinko-pir-server /bin/sh

# Check resource usage
docker stats

# Verify network connectivity
docker-compose exec plinko-pir-server ping -c 3 eth-mock
```

### Log Locations

```
Docker containers: docker-compose logs <service>
Host system: /var/lib/docker/containers/
Nginx: /var/log/nginx/ (inside cdn-mock container)
```

### Port Reference

| Port | Service | External Access |
|------|---------|-----------------|
| 3000 | PIR Server | ✅ Yes |
| 3001 | Update Service | ✅ Yes (metrics only) |
| 3002 | State Syncer | ✅ Yes (metrics only) |
| 5173 | Wallet UI | ✅ Yes |
| 8080 | CDN | ✅ Yes |
| 8545 | eth-mock | ❌ Internal only |
| 5001 | IPFS API | ❌ Internal only |

---

*For additional help, see: [DEPLOYMENT.md](DEPLOYMENT.md), [API_REFERENCE.md](API_REFERENCE.md), [SERVICE_ADDRESSING.md](SERVICE_ADDRESSING.md)*
