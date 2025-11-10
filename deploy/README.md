# Plinko PIR - Deployment

This directory contains deployment configurations for Plinko PIR PoC.

## Contents

- **[helm/plinko-pir/](helm/plinko-pir/)** - Helm chart for Kubernetes deployment
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Comprehensive deployment guide

## Quick Start

### Deploy to Vultr Kubernetes Engine

```bash
cd helm/plinko-pir

# Quick development deployment
./scripts/deploy.sh

# Production deployment
./scripts/deploy.sh --production

# Verify deployment
./scripts/verify.sh
```

## Deployment Options

### Option 1: Helm Chart (Recommended)

Production-ready Helm chart for Kubernetes deployment to Vultr VKE.

**Features**:
- Complete Kubernetes manifests
- Automated initialization with Jobs
- Auto-scaling with HPA
- Persistent storage with PVC
- Ingress with LoadBalancer
- Production-ready defaults

**Documentation**: [helm/plinko-pir/README.md](helm/plinko-pir/README.md)

**Guide**: [DEPLOYMENT.md](DEPLOYMENT.md)

### Option 2: Docker Compose (Development)

Docker Compose setup for local development and testing.

**Location**: Root directory `docker-compose.yml`

**Usage**:
```bash
cd ..
make build
make start
```

**Documentation**: [../IMPLEMENTATION.md](../IMPLEMENTATION.md)

## Architecture

### Kubernetes Deployment

```
Vultr LoadBalancer (Ingress)
   │
   ├─→ Rabby Wallet (3 replicas)
   ├─→ PIR Server (3-20 replicas, auto-scaling)
   └─→ CDN Mock (3-10 replicas, auto-scaling)

Cluster Services
   │
   ├─→ eth-mock (1 replica)
   ├─→ update-service (1 replica)
   ├─→ db-generator (Job, one-time)
   └─→ hint-generator (Job, one-time)

Persistent Storage
   │
   └─→ Vultr Block Storage (20-100 GB)
       └─→ database.bin, hint.bin, deltas/
```

### Service Endpoints

| Service | Port | Exposure | Purpose |
|---------|------|----------|---------|
| rabby-wallet | 80 | Ingress | User interface |
| plinko-pir-server | 3000 | Ingress | PIR queries |
| cdn-mock | 8080 | Ingress | Hint/delta files |
| eth-mock | 8545 | ClusterIP | Ethereum RPC |
| update-service | 3001 | ClusterIP | Health check |

## Resource Requirements

### Minimum (Development)

- **Cluster**: 3 nodes × "2 vCPU, 4GB RAM"
- **Storage**: 15 GB
- **Total**: 6 vCPU, 12 GB RAM
- **Cost**: ~$84/month on Vultr

### Recommended (Production)

- **Cluster**: 5 nodes × "4 vCPU, 8GB RAM"
- **Storage**: 50 GB
- **Total**: 20 vCPU, 40 GB RAM
- **Cost**: ~$255/month on Vultr

### High Availability

- **Cluster**: 10 nodes × "4 vCPU, 8GB RAM"
- **Storage**: 100 GB
- **Total**: 40 vCPU, 80 GB RAM
- **Cost**: ~$550/month on Vultr

## Prerequisites

### Required

- **Kubernetes 1.21+** (Vultr VKE or any K8s cluster)
- **Helm 3.8+** for chart deployment
- **kubectl 1.21+** for cluster management
- **Docker images** built and pushed to registry

### Optional

- **vultr-cli** for Vultr management
- **Domain name** for production ingress
- **cert-manager** for automatic TLS

### Install Tools

```bash
# macOS
brew install kubectl helm

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && sudo mv kubectl /usr/local/bin/

curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

## Deployment Workflow

### 1. Build Docker Images

```bash
# Build all services
cd ..
make build

# Tag for registry
docker tag plinko-pir/db-generator:latest your-registry/plinko-pir/db-generator:latest
docker tag plinko-pir/hint-generator:latest your-registry/plinko-pir/hint-generator:latest
docker tag plinko-pir/update-service:latest your-registry/plinko-pir/update-service:latest
docker tag plinko-pir/pir-server:latest your-registry/plinko-pir/pir-server:latest
docker tag plinko-pir/rabby-wallet:latest your-registry/plinko-pir/rabby-wallet:latest

# Push to registry
docker push your-registry/plinko-pir/db-generator:latest
# ... push all images
```

### 2. Create Kubernetes Cluster

```bash
# Using Vultr Web Console:
# 1. Go to https://my.vultr.com/kubernetes/
# 2. Deploy new cluster
# 3. Download kubeconfig

export KUBECONFIG=~/.kube/config-vultr
kubectl cluster-info
```

### 3. Install Ingress Controller

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

helm install nginx-ingress ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer
```

### 4. Deploy Plinko PIR

```bash
cd helm/plinko-pir

# Copy and customize values
cp values-vultr.yaml values-production.yaml
# Edit values-production.yaml with your configuration

# Deploy
./scripts/deploy.sh --custom values-production.yaml

# Or use helm directly
helm install plinko-pir . \
  --namespace plinko-pir \
  --create-namespace \
  --values values-production.yaml
```

### 5. Verify Deployment

```bash
# Automated verification
./scripts/verify.sh

# Manual verification
kubectl --namespace plinko-pir get all
kubectl --namespace plinko-pir get pvc
kubectl --namespace plinko-pir logs -l app.kubernetes.io/component=pir-server
```

### 6. Configure DNS (Production)

```bash
# Get LoadBalancer IP
export LB_IP=$(kubectl --namespace ingress-nginx get service nginx-ingress-ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Configure DNS A records:
# plinko-wallet.example.com -> $LB_IP
# plinko-api.example.com -> $LB_IP
# plinko-cdn.example.com -> $LB_IP
```

## Configuration

### Helm Values

The Helm chart supports extensive configuration via `values.yaml`:

```yaml
# Example: Scale PIR server
plinkoPirServer:
  replicaCount: 5
  autoscaling:
    enabled: true
    minReplicas: 5
    maxReplicas: 20

# Example: Increase storage
persistence:
  size: 50Gi

# Example: Configure ingress
ingress:
  enabled: true
  hosts:
    - host: plinko-wallet.example.com
      paths:
        - path: /
          service: rabby-wallet
```

See [helm/plinko-pir/values.yaml](helm/plinko-pir/values.yaml) for all options.

### Environment-Specific Values

- **values.yaml** - Default values
- **values-vultr.yaml** - Vultr VKE optimized values
- **values-production.yaml** - Your custom production values

## Monitoring

### View Logs

```bash
# All services
kubectl --namespace plinko-pir logs -l app.kubernetes.io/name=plinko-pir --tail=100 -f

# Specific service
kubectl --namespace plinko-pir logs -l app.kubernetes.io/component=pir-server -f

# Jobs
kubectl --namespace plinko-pir logs job/plinko-pir-db-generator
kubectl --namespace plinko-pir logs job/plinko-pir-hint-generator
```

### Resource Usage

```bash
# Pod resources
kubectl --namespace plinko-pir top pods

# Node resources
kubectl top nodes

# HPA status
kubectl --namespace plinko-pir get hpa
```

### Events

```bash
# Recent events
kubectl --namespace plinko-pir get events --sort-by='.lastTimestamp'

# Watch events
kubectl --namespace plinko-pir get events -w
```

## Scaling

### Manual Scaling

```bash
# Scale PIR server
kubectl --namespace plinko-pir scale deployment plinko-pir-pir-server --replicas=10

# Scale CDN
kubectl --namespace plinko-pir scale deployment plinko-pir-cdn-mock --replicas=5

# Scale wallet
kubectl --namespace plinko-pir scale deployment plinko-pir-rabby-wallet --replicas=5
```

### Auto-Scaling

Auto-scaling is enabled by default for production services via HPA:

```bash
# Check HPA status
kubectl --namespace plinko-pir get hpa

# Describe HPA
kubectl --namespace plinko-pir describe hpa plinko-pir-pir-server
```

## Upgrading

```bash
# Update values
vim values-production.yaml

# Dry-run upgrade
helm upgrade plinko-pir . \
  --namespace plinko-pir \
  --values values-production.yaml \
  --dry-run --debug

# Upgrade
helm upgrade plinko-pir . \
  --namespace plinko-pir \
  --values values-production.yaml

# Rollback if needed
helm rollback plinko-pir --namespace plinko-pir
```

## Troubleshooting

### Common Issues

1. **Jobs not completing**: Check logs, increase resources
2. **PVC not binding**: Verify storage class exists
3. **Ingress not working**: Check nginx controller, DNS
4. **Performance issues**: Scale replicas, increase resources
5. **OOM errors**: Increase memory limits

See [DEPLOYMENT.md](DEPLOYMENT.md) for detailed troubleshooting.

### Debug Commands

```bash
# Get all resources
kubectl --namespace plinko-pir get all

# Describe pod
kubectl --namespace plinko-pir describe pod <pod-name>

# Get events
kubectl --namespace plinko-pir get events --sort-by='.lastTimestamp'

# Shell into pod
kubectl --namespace plinko-pir exec -it <pod-name> -- sh

# Port-forward for testing
kubectl --namespace plinko-pir port-forward service/plinko-pir-pir-server 3000:3000
```

## Uninstallation

```bash
# Delete Helm release
helm uninstall plinko-pir --namespace plinko-pir

# Delete namespace
kubectl delete namespace plinko-pir

# Delete ingress controller (optional)
helm uninstall nginx-ingress --namespace ingress-nginx
kubectl delete namespace ingress-nginx
```

## Documentation

- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Complete deployment guide
- **[helm/plinko-pir/README.md](helm/plinko-pir/README.md)** - Helm chart documentation
- **[helm/plinko-pir/values.yaml](helm/plinko-pir/values.yaml)** - Configuration options
- **[../IMPLEMENTATION.md](../IMPLEMENTATION.md)** - PoC implementation details
- **[../README.md](../README.md)** - Project overview

## Support

For deployment issues:

1. Check logs: `kubectl --namespace plinko-pir logs <pod-name>`
2. Review events: `kubectl --namespace plinko-pir get events`
3. Run verification: `./scripts/verify.sh`
4. Consult [DEPLOYMENT.md](DEPLOYMENT.md)
5. Open GitHub issue

## License

MIT License - See LICENSE file for details
