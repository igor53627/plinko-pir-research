# Build and Deploy Locally - Complete Guide

Skip GitHub Actions, build everything locally and deploy directly.

## Prerequisites

```bash
# 1. Login to GitHub Container Registry (one-time setup)
docker login ghcr.io
# Username: igor53627
# Password: [Your GitHub Personal Access Token]
#   Create at: https://github.com/settings/tokens
#   Scopes needed: write:packages, read:packages
```

## Single Command Deployment

```bash
# Build all images, push, and deploy (one command)
./deploy/scripts/build-local-and-deploy.sh
```

## Manual Step-by-Step (if needed)

### Step 1: Build All Images (~5-10 min first time, ~2-3 min cached)

```bash
cd /Users/user/pse/plinko-pir-research

# Build with BuildKit cache optimizations
docker buildx build \
  --platform linux/amd64 \
  --cache-to type=local,dest=/tmp/docker-cache \
  --cache-from type=local,src=/tmp/docker-cache \
  -t ghcr.io/igor53627/plinko-rabby-wallet:latest \
  ./services/rabby-wallet

# Repeat for all services (or use script below)
```

### Step 2: Push Images

```bash
docker push ghcr.io/igor53627/plinko-rabby-wallet:latest
# Repeat for all 6 services
```

### Step 3: Deploy to VKE

```bash
KUBECONFIG=~/pse/k8s/pir-vke-*.yaml helm upgrade plinko-pir ./deploy/helm/plinko-pir \
  -f values-vke-simple.yaml \
  -f values-ingress-single-ip.yaml \
  -n plinko-pir \
  --wait --timeout 15m
```

## What You'll Get

After successful deployment:

**Single LoadBalancer IP**: `45.77.227.177`

- Wallet UI:   http://45.77.227.177/
- PIR Server:  http://45.77.227.177/api
- CDN:         http://45.77.227.177/cdn

## Build Optimizations Included

✅ **BuildKit cache mounts**:
- npm cache → 2-5x faster npm install
- Go modules cache → 3-10x faster go mod download

✅ **Platform-specific**: linux/amd64 only (no multi-arch overhead)

✅ **Layer caching**: Docker layer cache for unchanged dependencies

## Troubleshooting

### Docker Login Fails
```bash
# Generate GitHub token at https://github.com/settings/tokens
# Select scopes: write:packages, read:packages
# Then retry: docker login ghcr.io
```

### Build Fails - Out of Disk Space
```bash
# Clean up Docker
docker system prune -a --volumes -f
```

### Deploy Timeout
```bash
# Check pod status
KUBECONFIG=~/pse/k8s/pir-vke-*.yaml kubectl get pods -n plinko-pir

# Check events
KUBECONFIG=~/pse/k8s/pir-vke-*.yaml kubectl get events -n plinko-pir --sort-by='.lastTimestamp' | tail -20
```
