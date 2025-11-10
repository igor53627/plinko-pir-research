# Service Addressing Configuration

This document describes the network addressing scheme for the Plinko PIR + Plinko PoC.

## Table of Contents

- [Overview](#overview)
- [Default Configuration (localhost)](#default-configuration-localhost)
- [Custom Domain Configuration](#custom-domain-configuration)
- [Docker Network Architecture](#docker-network-architecture)
- [Configuration Files](#configuration-files)
- [Troubleshooting](#troubleshooting)

## Overview

The PoC uses a **dual addressing scheme**:

1. **External Access** (from host machine/browser): Uses `localhost` with distinct ports
2. **Docker Internal** (container-to-container): Uses Docker network service names

This design provides:
- Clear separation between external and internal communication
- Simple localhost access for development
- Optional custom domain support
- Proper network isolation for security

## Default Configuration (localhost)

### External Access URLs

Access services from your host machine or browser:

| Service | URL | Purpose |
|---------|-----|---------|
| **Ambire Wallet** | `http://localhost:5173` | User-facing wallet UI |
| **Plinko PIR Server** | `http://localhost:3000` | PIR query endpoint |
| **CDN Mock** | `http://localhost:8080` | Hint and delta files |
| **Plinko Update Service** | `http://localhost:3001` | Health check only |
| **Anvil (Ethereum Mock)** | Not exposed | Docker internal only |

### Port Mapping Summary

```
Host Port → Container Port
5173      → 80           (Ambire Wallet - nginx)
3000      → 3000         (Plinko PIR Server)
8080      → 8080         (CDN Mock)
3001      → 3001         (Plinko Update Service)
(none)    → 8545         (Anvil - internal only)
```

## Custom Domain Configuration

You can configure custom local domains for better organization.

### Setup Instructions

#### 1. Edit /etc/hosts

Add custom domain mappings:

```bash
sudo nano /etc/hosts
```

Add these lines:

```
127.0.0.1  plinko-server.local
127.0.0.1  plinko-cdn.local
127.0.0.1  plinko-wallet.local
127.0.0.1  plinko-updates.local
```

#### 2. Update Environment Variables

Create or edit `.env` file:

```bash
cp .env.example .env
nano .env
```

Update these variables:

```bash
VITE_PIR_SERVER_URL=http://plinko-server.local:3000
VITE_CDN_URL=http://plinko-cdn.local:8080
```

#### 3. Restart Services

```bash
docker-compose down
docker-compose up -d
```

#### 4. Access Services

- Wallet: `http://plinko-wallet.local:5173`
- PIR Server: `http://plinko-server.local:3000`
- CDN: `http://plinko-cdn.local:8080`

### Custom Domain URLs

| Service | Custom Domain URL |
|---------|-------------------|
| **Ambire Wallet** | `http://plinko-wallet.local:5173` |
| **Plinko PIR Server** | `http://plinko-server.local:3000` |
| **CDN Mock** | `http://plinko-cdn.local:8080` |
| **Plinko Update Service** | `http://plinko-updates.local:3001` |

## Docker Network Architecture

### Network Name

All services run on the `plinko-pir-network` bridge network.

### Service Discovery

Containers communicate using service names defined in `docker-compose.yml`:

| Service Name | Internal Address | Purpose |
|--------------|------------------|---------|
| `eth-mock` | `http://eth-mock:8545` | Ethereum RPC |
| `plinko-pir-server` | `http://plinko-pir-server:3000` | PIR queries |
| `cdn-mock` | `http://cdn-mock:8080` | CDN files |
| `plinko-pir-updates` | `http://plinko-pir-updates:3001` | Update service |

### Internal Communication Examples

**Database Generator → Anvil:**
```go
client, err := ethclient.Dial("http://eth-mock:8545")
```

**Plinko Update Service → Anvil:**
```go
httpURL := "http://eth-mock:8545"
client, err := ethclient.Dial(httpURL)
```

**Wallet Frontend → PIR Server (from browser):**
```javascript
const response = await fetch('http://localhost:3000/query/fullset', {
  method: 'POST',
  body: JSON.stringify(query)
});
```

### Network Isolation

- **Anvil (eth-mock)**: Only exposed within Docker network
  - No external port mapping
  - Only accessible by containers on `plinko-pir-network`

- **Other Services**: Exposed to host machine
  - Port mappings for external access
  - Still accessible within Docker network via service names

## Configuration Files

### 1. docker-compose.yml

Defines service addresses and port mappings:

```yaml
services:
  plinko-pir-server:
    ports:
      - "3000:3000"  # External access
    networks:
      - plinko-network  # Internal communication

networks:
  plinko-network:
    driver: bridge
    name: plinko-pir-network
```

### 2. .env / .env.example

Defines environment variables for service URLs:

```bash
# External access (from browser)
VITE_PIR_SERVER_URL=http://localhost:3000
VITE_CDN_URL=http://localhost:8080

# Docker internal (container-to-container)
DOCKER_ETH_MOCK_URL=http://eth-mock:8545
```

### 3. Ambire Wallet Environment

Runtime environment variables injected during build:

```dockerfile
ENV VITE_PIR_SERVER_URL=${VITE_PIR_SERVER_URL:-http://localhost:3000}
ENV VITE_CDN_URL=${VITE_CDN_URL:-http://localhost:8080}
```

### 4. Service Code

Backend services use hardcoded Docker network addresses:

```go
// services/db-generator/main.go
client, err := ethclient.Dial("http://eth-mock:8545")

// services/plinko-update-service/main.go
httpURL := "http://eth-mock:8545"
```

## Troubleshooting

### Issue: Cannot access services at localhost

**Symptoms:**
- Browser cannot reach `http://localhost:3000`
- Connection refused errors

**Solutions:**

1. Check if services are running:
```bash
docker-compose ps
```

2. Verify port mappings:
```bash
docker-compose port plinko-pir-server 3000
# Should output: 0.0.0.0:3000
```

3. Check for port conflicts:
```bash
# macOS/Linux
lsof -i :3000
lsof -i :5173
lsof -i :8080

# Windows
netstat -ano | findstr :3000
```

4. Restart Docker Compose:
```bash
docker-compose down
docker-compose up -d
```

### Issue: Custom domains not working

**Symptoms:**
- Cannot access `http://plinko-server.local:3000`
- DNS resolution fails

**Solutions:**

1. Verify /etc/hosts entries:
```bash
cat /etc/hosts | grep plinko
```

2. Test DNS resolution:
```bash
ping plinko-server.local
# Should resolve to 127.0.0.1
```

3. Check browser cache:
- Clear browser cache
- Try incognito/private mode

4. Verify .env configuration:
```bash
cat .env | grep VITE_
```

### Issue: Services cannot communicate internally

**Symptoms:**
- Database generator cannot connect to Anvil
- "dial tcp: lookup eth-mock: no such host" errors

**Solutions:**

1. Verify network configuration:
```bash
docker network ls | grep plinko
docker network inspect plinko-pir-network
```

2. Check service is on network:
```bash
docker inspect plinko-pir-eth-mock | grep NetworkMode
```

3. Rebuild services:
```bash
docker-compose down
docker-compose build
docker-compose up -d
```

4. Test connectivity between containers:
```bash
# Execute shell in one container
docker exec -it plinko-pir-server sh

# Try to reach another service
wget -O- http://eth-mock:8545
curl http://cdn-mock:8080/health
```

### Issue: Environment variables not updating

**Symptoms:**
- Changed VITE_PIR_SERVER_URL but wallet still uses old URL
- Services using wrong addresses

**Solutions:**

1. Rebuild wallet container (uses build-time env vars):
```bash
docker-compose build ambire-wallet
docker-compose up -d ambire-wallet
```

2. Verify environment variables:
```bash
docker exec plinko-wallet env | grep VITE_
```

3. Clear browser cache:
- Wallet is a static site built with Vite
- Environment variables are baked into JavaScript at build time
- Browser may cache old version

4. Force complete rebuild:
```bash
docker-compose down -v
docker-compose build --no-cache
docker-compose up -d
```

### Issue: CORS errors in browser

**Symptoms:**
- "Access to fetch blocked by CORS policy" errors
- Wallet cannot load hint.bin from CDN

**Solutions:**

1. Check CDN CORS headers:
```bash
curl -I http://localhost:8080/hint.bin
# Should see: Access-Control-Allow-Origin: *
```

2. Verify nginx configuration:
```bash
docker exec plinko-pir-cdn cat /etc/nginx/nginx.conf
```

3. Restart CDN service:
```bash
docker-compose restart cdn-mock
```

4. Try with custom domains:
- CORS is stricter with different origins
- Using custom domains may require additional configuration

### Issue: Port already in use

**Symptoms:**
- "bind: address already in use" errors
- Docker Compose fails to start

**Solutions:**

1. Identify conflicting process:
```bash
# macOS/Linux
lsof -i :3000
lsof -i :5173

# Windows
netstat -ano | findstr :3000
```

2. Stop conflicting process:
```bash
# macOS/Linux
kill -9 <PID>

# Windows
taskkill /PID <PID> /F
```

3. Use different ports:

Edit `.env`:
```bash
PIR_SERVER_PORT=4000
WALLET_PORT=4173
CDN_PORT=9080
```

Then update `docker-compose.yml` port mappings to use `${PIR_SERVER_PORT}:3000` syntax.

## Additional Resources

- [Docker Compose Networking Documentation](https://docs.docker.com/compose/networking/)
- [Main README.md](../README.md) - PoC overview and quick start
- [.env.example](../.env.example) - Environment variable template
- [docker-compose.yml](../docker-compose.yml) - Service orchestration

## Summary

**Default Setup** (localhost):
- No configuration needed
- Access at `http://localhost:<port>`
- Services communicate via Docker network

**Custom Domains** (optional):
- Add /etc/hosts entries
- Update .env file
- Rebuild and restart services
- Access at `http://<service>.local:<port>`

**Key Principle**:
- External access: `localhost` or custom domains
- Internal communication: Docker service names
- Separation ensures flexibility and security
