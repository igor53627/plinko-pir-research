# Local Kubernetes Testing Guide

This guide explains how to test the Plinko PIR Helm chart on your local machine using Kind (Kubernetes in Docker) or Minikube before deploying to Vultr.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Detailed Setup](#detailed-setup)
  - [Option 1: Kind (Recommended)](#option-1-kind-recommended)
  - [Option 2: Minikube](#option-2-minikube)
- [Accessing Services](#accessing-services)
- [Testing and Validation](#testing-and-validation)
- [Troubleshooting](#troubleshooting)
- [Cleanup](#cleanup)
- [Differences from Production](#differences-from-production)

## Prerequisites

### Required Tools

1. **Docker Desktop** or **Docker Engine**
   - macOS: Download from [docker.com](https://www.docker.com/products/docker-desktop/)
   - Linux: Follow [official installation guide](https://docs.docker.com/engine/install/)
   - Ensure Docker is running: `docker info`

2. **kubectl** (Kubernetes CLI)
   ```bash
   # macOS
   brew install kubectl

   # Linux
   curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
   sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
   ```

3. **Helm 3.8+** (Kubernetes package manager)
   ```bash
   # macOS
   brew install helm

   # Linux
   curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
   ```

4. **Kind** (Recommended) or **Minikube**

   **Kind (Kubernetes in Docker):**
   ```bash
   # macOS
   brew install kind

   # Linux
   curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
   chmod +x ./kind
   sudo mv ./kind /usr/local/bin/
   ```

   **Minikube (Alternative):**
   ```bash
   # macOS
   brew install minikube

   # Linux
   curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
   sudo install minikube-linux-amd64 /usr/local/bin/minikube
   ```

### System Requirements

- **CPU**: 4+ cores recommended
- **RAM**: 8GB minimum, 16GB recommended
- **Disk**: 20GB free space
- **OS**: macOS, Linux, or Windows (with WSL2)

## Quick Start

The fastest way to get started with local testing:

```bash
# Navigate to the Helm chart directory
cd deploy/helm/plinko-pir

# Run the automated deployment script (uses Kind by default)
./scripts/deploy-local.sh

# Wait for deployment (10-15 minutes)
# The script will show access URLs when ready

# Run validation tests
./scripts/test-local.sh

# Access the wallet
open http://localhost:30173
```

That's it! The `deploy-local.sh` script handles:
- Creating the Kind cluster
- Setting up port mappings
- Deploying the Helm chart
- Waiting for initialization
- Showing access information

## Detailed Setup

### Option 1: Kind (Recommended)

Kind is faster, lighter, and better for local development.

#### 1. Create Kind Cluster

```bash
cd deploy/helm/plinko-pir

# Automated (recommended)
./scripts/deploy-local.sh

# Manual cluster creation
kind create cluster --name plinko-pir-local --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30173
    hostPort: 30173
    protocol: TCP
  - containerPort: 30000
    hostPort: 30000
    protocol: TCP
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
  - containerPort: 30545
    hostPort: 30545
    protocol: TCP
EOF
```

#### 2. Deploy Plinko PIR

```bash
# Using the deployment script
./scripts/deploy-local.sh

# Or manually with Helm
helm install plinko-pir . \
  --namespace plinko-pir \
  --create-namespace \
  --values values-local.yaml \
  --wait \
  --timeout 15m
```

#### 3. Wait for Initialization

The deployment includes two initialization jobs:
1. **db-generator** (~5 minutes): Extracts 8.4M account balances from Anvil
2. **hint-generator** (~8 minutes): Generates Plinko PIR hints

```bash
# Watch job progress
kubectl --namespace plinko-pir get jobs -w

# Watch pod status
kubectl --namespace plinko-pir get pods -w

# View job logs
kubectl --namespace plinko-pir logs job/plinko-pir-db-generator
kubectl --namespace plinko-pir logs job/plinko-pir-hint-generator
```

### Option 2: Minikube

Minikube is an alternative if Kind doesn't work on your system.

#### 1. Create Minikube Cluster

```bash
cd deploy/helm/plinko-pir

# Automated
./scripts/deploy-local.sh --minikube

# Manual
minikube start \
  --profile plinko-pir-local \
  --memory 8192 \
  --cpus 4 \
  --disk-size 20g \
  --driver docker
```

#### 2. Deploy Plinko PIR

```bash
# Using the deployment script
./scripts/deploy-local.sh --minikube

# Or manually with Helm
helm install plinko-pir . \
  --namespace plinko-pir \
  --create-namespace \
  --values values-local.yaml \
  --wait \
  --timeout 15m
```

#### 3. Access Services

With Minikube, you may need to use `minikube service` or port forwarding:

```bash
# Option 1: Minikube service (creates tunnel)
minikube service -n plinko-pir plinko-pir-rabby-wallet

# Option 2: Port forwarding
./scripts/port-forward.sh
```

## Accessing Services

### Direct Access (Kind with NodePort)

If you used Kind with the deployment script, services are accessible via NodePort:

| Service | URL | Description |
|---------|-----|-------------|
| Rabby Wallet | http://localhost:30173 | User-facing wallet UI |
| PIR Server | http://localhost:30000 | Private query API |
| CDN Mock | http://localhost:30080 | Hint and delta files |
| Anvil RPC | http://localhost:30545 | Mock Ethereum node |

```bash
# Quick health checks
curl http://localhost:30000/health  # PIR Server
curl http://localhost:30080/health  # CDN Mock
curl -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:30545  # Anvil RPC

# Open wallet in browser
open http://localhost:30173
```

### Port Forwarding (Alternative)

If NodePort doesn't work or you're using Minikube, use the port forwarding script:

```bash
# Start port forwarding
./scripts/port-forward.sh

# Access URLs
open http://localhost:5173   # Wallet
curl http://localhost:3000/health  # PIR Server
curl http://localhost:8080/health  # CDN Mock

# Stop port forwarding
./scripts/port-forward.sh --stop

# Check status
./scripts/port-forward.sh --status
```

## Testing and Validation

### Automated Tests

Run the comprehensive test suite:

```bash
# Full test suite (includes data validation)
./scripts/test-local.sh

# Quick tests only (skip data validation)
./scripts/test-local.sh --quick

# With port-forward URLs
./scripts/test-local.sh --port-forward
```

The test suite validates:
- ✓ Namespace and resource creation
- ✓ Initialization jobs completed successfully
- ✓ All pods are running and healthy
- ✓ Services are accessible
- ✓ HTTP endpoints respond correctly
- ✓ Data files exist and have content
- ✓ Basic PIR query functionality

### Manual Testing

#### 1. Check Pod Status

```bash
kubectl --namespace plinko-pir get pods

# Expected output:
# NAME                                          READY   STATUS      RESTARTS
# plinko-pir-db-generator-xxxxx                 0/1     Completed   0
# plinko-pir-hint-generator-xxxxx               0/1     Completed   0
# plinko-pir-eth-mock-xxxxxxxxxx-xxxxx          1/1     Running     0
# plinko-pir-pir-server-xxxxxxxxxx-xxxxx        1/1     Running     0
# plinko-pir-update-service-xxxxxxxxxx-xxxxx    1/1     Running     0
# plinko-pir-cdn-mock-xxxxxxxxxx-xxxxx          1/1     Running     0
# plinko-pir-rabby-wallet-xxxxxxxxxx-xxxxx      1/1     Running     0
```

#### 2. View Logs

```bash
# PIR Server logs
kubectl --namespace plinko-pir logs -f deployment/plinko-pir-pir-server

# Update Service logs
kubectl --namespace plinko-pir logs -f deployment/plinko-pir-update-service

# Anvil logs
kubectl --namespace plinko-pir logs -f deployment/plinko-pir-eth-mock

# Job logs
kubectl --namespace plinko-pir logs job/plinko-pir-db-generator
kubectl --namespace plinko-pir logs job/plinko-pir-hint-generator
```

#### 3. Check Data Files

```bash
# Get PIR server pod name
PIR_POD=$(kubectl --namespace plinko-pir get pods -l app=plinko-pir-server -o jsonpath='{.items[0].metadata.name}')

# List data files
kubectl --namespace plinko-pir exec $PIR_POD -- ls -lh /data

# Check database size
kubectl --namespace plinko-pir exec $PIR_POD -- du -h /data/database.bin

# Check hint size
kubectl --namespace plinko-pir exec $PIR_POD -- du -h /data/hint.bin

# List delta files
kubectl --namespace plinko-pir exec $PIR_POD -- ls -lh /data/deltas/ | head
```

#### 4. Test PIR Queries

```bash
# Access the wallet
open http://localhost:30173

# Enable Privacy Mode in the UI
# Try querying account balances
# Observe that queries use PIR instead of direct RPC
```

## Troubleshooting

### Common Issues

#### 1. Docker Not Running

**Error:**
```
Cannot connect to the Docker daemon
```

**Solution:**
```bash
# Start Docker Desktop (macOS)
open -a Docker

# Or check Docker service (Linux)
sudo systemctl start docker
```

#### 2. Insufficient Resources

**Error:**
```
Failed to create pod: insufficient memory/cpu
```

**Solution:**
- Increase Docker Desktop resources (Preferences → Resources)
- Recommended: 8GB RAM, 4 CPUs
- Or use `values-local.yaml` which has reduced resource requests

#### 3. Port Already in Use

**Error:**
```
Bind for 0.0.0.0:30173 failed: port is already allocated
```

**Solution:**
```bash
# Find what's using the port
lsof -i :30173

# Kill the process or use different ports
# Edit values-local.yaml to change NodePort values
```

#### 4. Initialization Jobs Timeout

**Error:**
```
Waiting for db-generator job... timeout
```

**Solution:**
```bash
# Check job status
kubectl --namespace plinko-pir describe job plinko-pir-db-generator

# View logs
kubectl --namespace plinko-pir logs job/plinko-pir-db-generator

# Common causes:
# - Insufficient resources (increase Docker limits)
# - Anvil not ready (wait for eth-mock pod)
# - Networking issues (check pod connectivity)
```

#### 5. Services Not Accessible

**Error:**
```
curl: (7) Failed to connect to localhost port 30173
```

**Solution:**
```bash
# Check if pods are running
kubectl --namespace plinko-pir get pods

# Check if services exist
kubectl --namespace plinko-pir get services

# For Kind: Ensure port mappings were set during cluster creation
kind get clusters
kind delete cluster --name plinko-pir-local
./scripts/deploy-local.sh  # Recreate with correct config

# For Minikube: Use port-forward instead
./scripts/port-forward.sh
```

#### 6. PVC Pending

**Error:**
```
PersistentVolumeClaim is in Pending state
```

**Solution:**
```bash
# Check PVC status
kubectl --namespace plinko-pir get pvc

# Check events
kubectl --namespace plinko-pir describe pvc plinko-pir-data-pvc

# For Kind: Should auto-provision with local-path
# For Minikube: Enable storage provisioner
minikube addons enable storage-provisioner -p plinko-pir-local
```

### Debug Commands

```bash
# Get all resources
kubectl --namespace plinko-pir get all

# Check events
kubectl --namespace plinko-pir get events --sort-by='.lastTimestamp'

# Describe failing pod
kubectl --namespace plinko-pir describe pod <pod-name>

# Get logs from crashed pod
kubectl --namespace plinko-pir logs <pod-name> --previous

# Execute commands in pod
kubectl --namespace plinko-pir exec -it <pod-name> -- /bin/sh

# Check resource usage
kubectl top nodes
kubectl top pods -n plinko-pir
```

## Cleanup

### Uninstall Helm Chart

```bash
# Remove the deployment
helm uninstall plinko-pir --namespace plinko-pir

# Delete namespace
kubectl delete namespace plinko-pir
```

### Delete Cluster

#### Kind
```bash
# List clusters
kind get clusters

# Delete cluster
kind delete cluster --name plinko-pir-local

# Verify deletion
kind get clusters
```

#### Minikube
```bash
# List profiles
minikube profile list

# Delete cluster
minikube delete -p plinko-pir-local

# Verify deletion
minikube profile list
```

### Clean Up Docker

```bash
# Remove unused images (optional)
docker image prune -a

# Remove unused volumes (optional)
docker volume prune
```

## Differences from Production

Understanding key differences between local and Vultr production:

| Aspect | Local (Kind/Minikube) | Production (Vultr VKE) |
|--------|----------------------|------------------------|
| **Storage** | hostPath/local-path (ReadWriteOnce) | Vultr Block Storage (ReadWriteMany) |
| **Networking** | NodePort (localhost:30XXX) | LoadBalancer with public IPs |
| **Ingress** | Disabled (NodePort instead) | Nginx Ingress with TLS/HTTPS |
| **Resources** | Reduced (512Mi-2Gi RAM) | Production-grade (2-8Gi RAM) |
| **Replicas** | Single instance (no HA) | Multiple replicas (2-3+) |
| **Auto-scaling** | Disabled | Enabled (HPA) |
| **PVC Size** | 10Gi | 20Gi |
| **TLS** | Disabled | Enabled (Let's Encrypt) |
| **Monitoring** | Disabled | Optional (Prometheus/Grafana) |
| **Backup** | Disabled | Optional (Volume Snapshots) |

### Local Testing Best Practices

1. **Storage Persistence**
   - Local storage is ephemeral (lost when cluster deleted)
   - Backup important data before cleanup
   - Production uses persistent block storage

2. **Performance**
   - Local performance may differ from production
   - Resource limits are reduced for laptop/desktop
   - Use for functional testing, not performance benchmarking

3. **Networking**
   - NodePort only works on localhost
   - No external access unless you set up tunneling
   - Production LoadBalancer provides public IPs

4. **Data Validation**
   - Always run `./scripts/test-local.sh` before production deployment
   - Verify data files exist and have correct sizes
   - Test basic PIR queries work

5. **Iterative Development**
   - Use `helm upgrade` for quick iterations
   - Port-forward for debugging specific services
   - Check logs frequently during development

## Next Steps

After successful local testing:

1. **Review Configuration**
   - Verify `values-vultr.yaml` has correct production settings
   - Update domain names for your production environment
   - Configure TLS settings for cert-manager

2. **Deploy to Vultr**
   ```bash
   cd deploy/helm/plinko-pir
   ./scripts/deploy.sh --production
   ```

3. **Run Production Validation**
   ```bash
   ./scripts/verify.sh
   ```

4. **Monitor Deployment**
   - Check pod health and logs
   - Verify LoadBalancer IPs assigned
   - Configure DNS records
   - Test public access

## Additional Resources

- [Kind Documentation](https://kind.sigs.k8s.io/)
- [Minikube Documentation](https://minikube.sigs.k8s.io/docs/)
- [Helm Documentation](https://helm.sh/docs/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Vultr VKE Documentation](https://www.vultr.com/docs/vultr-kubernetes-engine/)

## Support

For issues with local testing:

1. Check troubleshooting section above
2. Review logs: `kubectl --namespace plinko-pir logs <pod-name>`
3. Check events: `kubectl --namespace plinko-pir get events`
4. Validate with test script: `./scripts/test-local.sh`
5. See [../DEPLOYMENT.md](DEPLOYMENT.md) for production deployment

---

**Happy Local Testing!** 🚀
