# Plinko PIR - Vultr Kubernetes Engine Deployment Guide

Complete guide for deploying Plinko PIR PoC to Vultr Kubernetes Engine (VKE).

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Detailed Setup](#detailed-setup)
- [Configuration](#configuration)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)
- [Cost Analysis](#cost-analysis)
- [Production Considerations](#production-considerations)

## Overview

This deployment guide covers deploying the complete Plinko PIR system to Vultr Kubernetes Engine using Helm charts. The deployment includes:

- **7 microservices** orchestrated with Kubernetes
- **Persistent storage** using Vultr Block Storage
- **Load balancing** with Vultr LoadBalancer
- **Auto-scaling** for high availability
- **TLS/HTTPS** with cert-manager (optional)

### Key Features

- **One-command deployment** with Helm
- **Automated initialization** with Kubernetes Jobs
- **High availability** with multiple replicas
- **Horizontal auto-scaling** based on CPU/memory
- **Production-ready** configuration for Vultr VKE

## Architecture

### Service Topology

```
Internet
   │
   ├─→ Vultr LoadBalancer (Ingress)
   │   │
   │   ├─→ Rabby Wallet (React SPA) - Port 80
   │   ├─→ PIR Server (Go) - Port 3000
   │   └─→ CDN Mock (nginx) - Port 8080
   │
Kubernetes Cluster (VKE)
   │
   ├─→ eth-mock (Anvil) - ClusterIP:8545
   │   └─→ Simulated Ethereum blockchain
   │
   ├─→ db-generator (Job, one-time)
   │   └─→ Generates database.bin
   │
   ├─→ hint-generator (Job, one-time)
   │   └─→ Generates hint.bin
   │
   ├─→ update-service (Deployment)
   │   └─→ Generates delta files
   │
   └─→ Shared PVC (Vultr Block Storage, 20GB)
       └─→ /data (database.bin, hint.bin, deltas/)
```

### Data Flow

1. **Initialization** (one-time):
   - eth-mock starts with 8.4M accounts
   - db-generator queries all accounts → database.bin
   - hint-generator creates PIR hints → hint.bin

2. **Runtime**:
   - update-service monitors blockchain → delta files
   - pir-server serves private queries
   - cdn-mock serves hint.bin and deltas
   - rabby-wallet provides UI

3. **Client interaction**:
   - User downloads hint.bin (~70 MB)
   - User queries PIR server (private)
   - Client applies deltas to stay synced

## Prerequisites

### Required

1. **Vultr Account** with payment method
2. **kubectl** 1.21+ installed
3. **helm** 3.8+ installed
4. **Docker images** built and pushed to registry (or use existing)

### Optional

5. **vultr-cli** for command-line management
6. **Domain name** for production deployment
7. **cert-manager** for automatic TLS certificates

### Install Tools

```bash
# macOS
brew install kubectl helm

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && sudo mv kubectl /usr/local/bin/

curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify
kubectl version --client
helm version
```

## Quick Start

### 1. Create VKE Cluster

```bash
# Using Vultr Web Console (recommended):
# 1. Go to https://my.vultr.com/kubernetes/
# 2. Click "Deploy New Kubernetes Cluster"
# 3. Select:
#    - Region: Atlanta (or closest to you)
#    - Version: Kubernetes 1.28+
#    - Node Pool: 3 nodes x "4 CPU, 8GB RAM"
# 4. Click "Deploy Now" (~5 minutes)

# Download kubeconfig
# From cluster page → "Download Configuration"
# Save to ~/.kube/config-vultr

export KUBECONFIG=~/.kube/config-vultr
kubectl cluster-info
```

### 2. Install Nginx Ingress

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

helm install nginx-ingress ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer

# Wait for LoadBalancer IP
kubectl --namespace ingress-nginx get service -w
```

### 3. Deploy Plinko PIR

```bash
cd deploy/helm/plinko-pir

# Development deployment
./scripts/deploy.sh

# Production deployment
./scripts/deploy.sh --production

# Wait for initialization (10-15 minutes)
kubectl --namespace plinko-pir get pods -w
```

### 4. Verify Deployment

```bash
./scripts/verify.sh

# Expected output:
# [PASS] All tests passed!
```

### 5. Access Wallet

```bash
# Get LoadBalancer IP
export LB_IP=$(kubectl --namespace ingress-nginx get service nginx-ingress-ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

echo "Access wallet at: http://$LB_IP"

# Or use port-forward for testing
kubectl --namespace plinko-pir port-forward service/plinko-pir-rabby-wallet 5173:80

# Open http://localhost:5173
```

## Detailed Setup

### Step 1: Build Docker Images

If you need to build custom images:

```bash
# Build all services
cd plinko-pir-research
make build

# Tag for registry
docker tag plinko-pir/db-generator:latest your-registry/plinko-pir/db-generator:latest
docker tag plinko-pir/hint-generator:latest your-registry/plinko-pir/hint-generator:latest
docker tag plinko-pir/update-service:latest your-registry/plinko-pir/update-service:latest
docker tag plinko-pir/pir-server:latest your-registry/plinko-pir/pir-server:latest
docker tag plinko-pir/rabby-wallet:latest your-registry/plinko-pir/rabby-wallet:latest

# Push to registry
docker push your-registry/plinko-pir/db-generator:latest
docker push your-registry/plinko-pir/hint-generator:latest
docker push your-registry/plinko-pir/update-service:latest
docker push your-registry/plinko-pir/pir-server:latest
docker push your-registry/plinko-pir/rabby-wallet:latest
```

### Step 2: Configure Values

Create custom values file:

```bash
cd deploy/helm/plinko-pir
cp values-vultr.yaml values-production.yaml
```

Edit `values-production.yaml`:

```yaml
# Set your domain
global:
  domain: "example.com"
  tls:
    enabled: true
    issuer: "letsencrypt-prod"

# Update image repositories if using custom registry
dbGenerator:
  image:
    repository: your-registry/plinko-pir/db-generator
    tag: latest

# Configure resource limits for production
plinkoPirServer:
  replicaCount: 5
  resources:
    limits:
      memory: "4Gi"
      cpu: "2000m"

# Configure ingress hosts
ingress:
  hosts:
    - host: "plinko-wallet.example.com"
      paths:
        - path: /
          pathType: Prefix
          service: rabby-wallet
          port: 80
    - host: "plinko-api.example.com"
      paths:
        - path: /
          pathType: Prefix
          service: plinko-pir-server
          port: 3000
    - host: "plinko-cdn.example.com"
      paths:
        - path: /
          pathType: Prefix
          service: cdn-mock
          port: 8080
```

### Step 3: Deploy with Custom Values

```bash
helm install plinko-pir . \
  --namespace plinko-pir \
  --create-namespace \
  --values values-production.yaml

# Watch deployment
kubectl --namespace plinko-pir get pods -w
```

### Step 4: Configure DNS

```bash
# Get LoadBalancer IP
export LB_IP=$(kubectl --namespace ingress-nginx get service nginx-ingress-ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

echo "Configure DNS A records:"
echo "plinko-wallet.example.com -> $LB_IP"
echo "plinko-api.example.com -> $LB_IP"
echo "plinko-cdn.example.com -> $LB_IP"

# Verify DNS propagation
dig plinko-wallet.example.com +short
```

### Step 5: Install cert-manager (for TLS)

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true

# Create ClusterIssuer
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF

# Certificates will be automatically provisioned
kubectl --namespace plinko-pir get certificate
```

## Configuration

### Resource Requirements

#### Minimum (Development)

- **Nodes**: 3 x "2 vCPU, 4GB RAM"
- **Storage**: 15GB Vultr Block Storage
- **Total RAM**: 12GB
- **Total CPU**: 6 vCPU

#### Recommended (Production)

- **Nodes**: 5 x "4 vCPU, 8GB RAM"
- **Storage**: 50GB Vultr Block Storage
- **Total RAM**: 40GB
- **Total CPU**: 20 vCPU

#### High Availability

- **Nodes**: 10 x "4 vCPU, 8GB RAM"
- **Storage**: 100GB Vultr Block Storage
- **Total RAM**: 80GB
- **Total CPU**: 40 vCPU

### Service Configuration

#### PIR Server Scaling

```yaml
plinkoPirServer:
  replicaCount: 3
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 20
    targetCPUUtilizationPercentage: 70
```

#### CDN Configuration

```yaml
cdnMock:
  config:
    gzipEnabled: true
    corsEnabled: true
    cacheMaxAge: 3600
  replicaCount: 3
```

#### Update Service

```yaml
plinkoUpdateService:
  config:
    cacheEnabled: true
    cacheSizeMB: 64
    blockPollInterval: 12
```

### Storage Configuration

```yaml
persistence:
  enabled: true
  storageClass: "vultr-block-storage"
  size: 20Gi
  accessMode: ReadWriteMany
```

## Verification

### Automated Verification

```bash
cd deploy/helm/plinko-pir
./scripts/verify.sh

# Expected output:
# [PASS] Namespace exists
# [PASS] PVC is bound
# [PASS] db-generator job completed
# [PASS] hint-generator job completed
# [PASS] plinko-pir-eth-mock is ready (1/1)
# [PASS] plinko-pir-update-service is ready (1/1)
# [PASS] plinko-pir-pir-server is ready (3/3)
# [PASS] plinko-pir-cdn-mock is ready (3/3)
# [PASS] plinko-pir-rabby-wallet is ready (3/3)
# [PASS] PIR server health check passed
# [PASS] CDN health check passed
# [PASS] Wallet health check passed
# [PASS] database.bin exists (64 MB)
# [PASS] hint.bin exists (70 MB)
# [PASS] deltas directory exists (120 files)
#
# All tests passed!
```

### Manual Verification

```bash
# Check all resources
kubectl --namespace plinko-pir get all

# Check data files
kubectl --namespace plinko-pir run data-check --rm -it --image=busybox -- sh
ls -lh /data/

# Test PIR query
kubectl --namespace plinko-pir run test-query --rm -it --image=curlimages/curl -- \
  curl -X POST http://plinko-pir-pir-server:3000/query/plaintext \
  -H "Content-Type: application/json" \
  -d '{"index": 42}'

# Check logs
kubectl --namespace plinko-pir logs -l app.kubernetes.io/component=pir-server --tail=100
```

## Troubleshooting

### Common Issues

#### 1. Jobs Not Completing

**Symptom**: db-generator or hint-generator stuck

```bash
# Check job status
kubectl --namespace plinko-pir get jobs

# Check logs
kubectl --namespace plinko-pir logs job/plinko-pir-db-generator -f

# Common causes:
# - eth-mock not ready
# - Insufficient resources
# - PVC not bound

# Solution: Delete and recreate
kubectl --namespace plinko-pir delete job plinko-pir-db-generator
helm upgrade plinko-pir . --namespace plinko-pir --reuse-values
```

#### 2. PVC Not Binding

**Symptom**: Pods stuck in "Pending" state

```bash
# Check PVC status
kubectl --namespace plinko-pir describe pvc plinko-pir-data

# Check storage class
kubectl get storageclass

# Solution: Ensure Vultr CSI driver is installed
# VKE should have this by default, but if not:
kubectl apply -f https://raw.githubusercontent.com/vultr/vultr-csi/master/deploy/kubernetes/releases/latest/deploy.yaml
```

#### 3. Ingress Not Working

**Symptom**: Cannot access wallet via domain

```bash
# Check ingress status
kubectl --namespace plinko-pir describe ingress plinko-pir

# Check nginx ingress logs
kubectl --namespace ingress-nginx logs -l app.kubernetes.io/component=controller

# Verify LoadBalancer
kubectl --namespace ingress-nginx get service

# Test internally
kubectl --namespace plinko-pir run test --rm -it --image=curlimages/curl -- \
  curl http://plinko-pir-rabby-wallet:80
```

#### 4. Performance Issues

**Symptom**: Slow queries or high latency

```bash
# Check resource usage
kubectl --namespace plinko-pir top pods

# Check HPA status
kubectl --namespace plinko-pir get hpa

# Scale manually
kubectl --namespace plinko-pir scale deployment plinko-pir-pir-server --replicas=10

# Increase resource limits
helm upgrade plinko-pir . \
  --namespace plinko-pir \
  --set plinkoPirServer.resources.limits.memory=4Gi
```

#### 5. Out of Memory

**Symptom**: Pods being OOMKilled

```bash
# Check events
kubectl --namespace plinko-pir get events --sort-by='.lastTimestamp' | grep OOM

# Increase memory limits
helm upgrade plinko-pir . \
  --namespace plinko-pir \
  --set ethMock.resources.limits.memory=8Gi \
  --set plinkoHintGenerator.resources.limits.memory=8Gi
```

### Debug Commands

```bash
# Get all events
kubectl --namespace plinko-pir get events --sort-by='.lastTimestamp'

# Describe pod
kubectl --namespace plinko-pir describe pod <pod-name>

# Get logs
kubectl --namespace plinko-pir logs <pod-name>

# Shell into pod
kubectl --namespace plinko-pir exec -it <pod-name> -- sh

# Check PVC mount
kubectl --namespace plinko-pir run debug --rm -it --image=busybox -- sh
ls -la /data/

# Port-forward for local testing
kubectl --namespace plinko-pir port-forward service/plinko-pir-pir-server 3000:3000
```

## Cost Analysis

### Vultr VKE Pricing (as of 2025)

#### Node Costs

| Plan | vCPU | RAM | Storage | Price/Month |
|------|------|-----|---------|-------------|
| 2c-4gb | 2 | 4GB | 100GB | $24 |
| 4c-8gb | 4 | 8GB | 180GB | $48 |
| 8c-16gb | 8 | 16GB | 320GB | $96 |

#### Additional Costs

| Resource | Price |
|----------|-------|
| Block Storage | $0.10/GB/month |
| LoadBalancer | $10/month |
| Bandwidth | First 1TB free, $0.01/GB after |
| Snapshots | $0.05/GB/month |

### Monthly Cost Estimates

#### Development (3 nodes, 2c-4gb)

- Nodes: 3 × $24 = $72
- Block Storage: 20GB × $0.10 = $2
- LoadBalancer: $10
- Bandwidth: Included
- **Total: ~$84/month**

#### Production (5 nodes, 4c-8gb)

- Nodes: 5 × $48 = $240
- Block Storage: 50GB × $0.10 = $5
- LoadBalancer: $10
- Bandwidth: Included
- **Total: ~$255/month**

#### High Availability (10 nodes, 4c-8gb)

- Nodes: 10 × $48 = $480
- Block Storage: 100GB × $0.10 = $10
- LoadBalancer: $10
- Bandwidth: ~$50 (5TB)
- **Total: ~$550/month**

## Production Considerations

### Security

1. **Network Policies**: Restrict pod-to-pod communication
2. **TLS/HTTPS**: Enable cert-manager for automatic certificates
3. **Secret Management**: Use Kubernetes Secrets or external vault
4. **RBAC**: Configure role-based access control
5. **Pod Security**: Enable pod security policies

### Monitoring

1. **Prometheus**: Install for metrics collection
2. **Grafana**: Visualize metrics and create dashboards
3. **Alerting**: Configure alerts for critical events
4. **Logging**: Centralize logs with ELK or Loki

### Backup

1. **Volume Snapshots**: Enable Vultr Block Storage snapshots
2. **Backup Schedule**: Daily at 2 AM UTC
3. **Retention**: Keep 7 days of backups
4. **Disaster Recovery**: Test restoration procedures

### Scaling

1. **Horizontal Pod Autoscaler**: Automatically scale based on metrics
2. **Cluster Autoscaler**: Add/remove nodes based on demand
3. **Resource Requests**: Set appropriate requests and limits
4. **Load Testing**: Test at expected peak load

### Updates

1. **Rolling Updates**: Zero-downtime deployments
2. **Helm Upgrades**: Use `helm upgrade` with `--reuse-values`
3. **Rollback Plan**: Test rollback procedures
4. **Blue-Green**: Consider blue-green deployments for major updates

## Additional Resources

- **Helm Chart**: `/deploy/helm/plinko-pir/`
- **Documentation**: `/deploy/helm/plinko-pir/README.md`
- **Scripts**: `/deploy/helm/plinko-pir/scripts/`
- **Vultr Docs**: https://www.vultr.com/docs/vultr-kubernetes-engine
- **Kubernetes Docs**: https://kubernetes.io/docs/

## Support

For issues and questions:

1. Check the troubleshooting section
2. Review logs and events
3. Open GitHub issue
4. Contact Vultr support for infrastructure issues

---

**Ready to deploy?** Run `./scripts/deploy.sh --production` and follow the verification steps!
