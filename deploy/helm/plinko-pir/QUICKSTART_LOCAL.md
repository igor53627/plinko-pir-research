# Quick Start: Local Kubernetes Testing

Fast track to test Plinko PIR on local Kubernetes (Kind).

## One-Liner Install

```bash
cd deploy/helm/plinko-pir && ./scripts/deploy-local.sh && ./scripts/test-local.sh
```

## Prerequisites (Install Once)

```bash
# macOS
brew install docker kubectl helm kind

# Linux
# Install Docker: https://docs.docker.com/engine/install/
# Install kubectl, helm, kind: see LOCAL_TESTING.md
```

## Step-by-Step (5 Steps)

### 1. Start Docker
```bash
# macOS: Open Docker Desktop
open -a Docker

# Linux: Check Docker is running
docker info
```

### 2. Navigate to Chart
```bash
cd deploy/helm/plinko-pir
```

### 3. Deploy
```bash
./scripts/deploy-local.sh
# Wait 15 minutes for initialization
```

### 4. Test
```bash
./scripts/test-local.sh
# Should see: "✓ All tests passed!"
```

### 5. Access
```bash
# Open wallet in browser
open http://localhost:30173

# Check services
curl http://localhost:30000/health  # PIR Server
curl http://localhost:30080/health  # CDN Mock
```

## Common Commands

```bash
# View pods
kubectl --namespace plinko-pir get pods

# View logs
kubectl --namespace plinko-pir logs -f deployment/plinko-pir-pir-server

# Check all resources
kubectl --namespace plinko-pir get all

# Restart deployment
helm upgrade plinko-pir . -n plinko-pir -f values-local.yaml

# Clean up
helm uninstall plinko-pir --namespace plinko-pir
kind delete cluster --name plinko-pir-local
```

## Access URLs

| Service | URL | Description |
|---------|-----|-------------|
| Wallet | http://localhost:30173 | Rabby Wallet UI |
| PIR Server | http://localhost:30000 | Private query API |
| CDN | http://localhost:30080 | Hint and delta files |
| Anvil RPC | http://localhost:30545 | Mock Ethereum node |

## Troubleshooting

### Port Already in Use
```bash
# Find what's using port 30173
lsof -i :30173

# Or use port-forward instead
./scripts/port-forward.sh
open http://localhost:5173
```

### Services Not Ready
```bash
# Check pod status
kubectl --namespace plinko-pir get pods

# Wait for jobs to complete
kubectl --namespace plinko-pir wait --for=condition=complete job/plinko-pir-db-generator --timeout=600s
kubectl --namespace plinko-pir wait --for=condition=complete job/plinko-pir-hint-generator --timeout=900s

# Check logs
kubectl --namespace plinko-pir logs job/plinko-pir-db-generator
```

### Docker Not Running
```bash
# macOS
open -a Docker

# Linux
sudo systemctl start docker
```

### Insufficient Resources
- Open Docker Desktop → Preferences → Resources
- Set: 8GB RAM, 4 CPUs minimum
- Click "Apply & Restart"

## Next Steps

- **Full Documentation**: See [LOCAL_TESTING.md](../../LOCAL_TESTING.md)
- **Production Deployment**: See [../DEPLOYMENT.md](../../DEPLOYMENT.md)
- **Chart Details**: See [README.md](README.md)

## Quick Reference

```bash
# Deploy
./scripts/deploy-local.sh

# Test
./scripts/test-local.sh

# Port Forward
./scripts/port-forward.sh

# Logs
kubectl -n plinko-pir logs -f deployment/plinko-pir-pir-server

# Clean Up
helm uninstall plinko-pir -n plinko-pir
kind delete cluster --name plinko-pir-local
```

---

**Total Time**: ~20 minutes (15 min initialization + 5 min setup)
