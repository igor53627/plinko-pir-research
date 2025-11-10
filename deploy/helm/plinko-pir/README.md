# Plinko PIR Helm Chart

Helm chart for deploying Plinko PIR PoC to Vultr Kubernetes Engine (VKE).

## Overview

This Helm chart deploys a complete Plinko PIR system with 7 services:

1. **eth-mock** - Simulated Ethereum blockchain (Anvil)
2. **db-generator** - Database generation job (one-time)
3. **hint-generator** - PIR hint generation job (one-time)
4. **update-service** - Real-time delta generation service
5. **pir-server** - Private query server with auto-scaling
6. **cdn-mock** - CDN for hint and delta files
7. **rabby-wallet** - User-facing wallet with Privacy Mode

## Prerequisites

### Required Tools

- **Kubernetes 1.21+** (Vultr Kubernetes Engine)
- **Helm 3.8+** - Kubernetes package manager
- **kubectl 1.21+** - Kubernetes CLI
- **vultr-cli** (optional) - Vultr command-line tool

### Install Helm

```bash
# macOS
brew install helm

# Linux
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify installation
helm version
```

### Install kubectl

```bash
# macOS
brew install kubectl

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/

# Verify installation
kubectl version --client
```

### Install Vultr CLI (Optional)

```bash
# Using go
go install github.com/vultr/vultr-cli/v2@latest

# Or download binary from GitHub
# https://github.com/vultr/vultr-cli/releases
```

## Vultr Kubernetes Engine Setup

### 1. Create VKE Cluster

```bash
# Using Vultr web console (recommended for first-time setup):
# 1. Go to https://my.vultr.com/kubernetes/
# 2. Click "Deploy New Kubernetes Cluster"
# 3. Choose region (e.g., "Atlanta" for low latency)
# 4. Select version: Kubernetes 1.28+
# 5. Configure node pool:
#    - Node count: 3 (minimum for high availability)
#    - Plan: "4 CPU, 8GB RAM" or higher
#    - Total: 12 CPU, 24GB RAM minimum
# 6. Click "Deploy Now"

# Or using vultr-cli:
vultr-cli kubernetes create \
  --label "plinko-pir-cluster" \
  --region atl \
  --version v1.28.3 \
  --node-pools="quantity=3,plan=vc2-2c-4gb,label=plinko-nodes"
```

### 2. Download Kubeconfig

```bash
# From Vultr web console:
# 1. Go to your cluster page
# 2. Click "Download Configuration"
# 3. Save to ~/.kube/config-vultr

# Or using vultr-cli:
vultr-cli kubernetes config <cluster-id> > ~/.kube/config-vultr

# Set KUBECONFIG
export KUBECONFIG=~/.kube/config-vultr

# Verify connection
kubectl cluster-info
kubectl get nodes
```

### 3. Install Nginx Ingress Controller

```bash
# Add nginx ingress helm repo
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

# Install nginx ingress with Vultr LoadBalancer
helm install nginx-ingress ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer \
  --set controller.service.annotations."service\.beta\.kubernetes\.io/vultr-loadbalancer-protocol"="tcp" \
  --set controller.service.annotations."service\.beta\.kubernetes\.io/vultr-loadbalancer-proxy-protocol"="v2"

# Wait for LoadBalancer to be provisioned
kubectl --namespace ingress-nginx get services -o wide -w nginx-ingress-ingress-nginx-controller

# Get LoadBalancer IP
export LOADBALANCER_IP=$(kubectl --namespace ingress-nginx get service nginx-ingress-ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
echo "LoadBalancer IP: $LOADBALANCER_IP"
```

### 4. Install cert-manager (Optional, for TLS)

```bash
# Add cert-manager repo
helm repo add jetstack https://charts.jetstack.io
helm repo update

# Install cert-manager
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true

# Create Let's Encrypt ClusterIssuer
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
```

## Installation

### Quick Start (Development)

```bash
# Clone repository
git clone https://github.com/yourusername/plinko-pir-research.git
cd plinko-pir-research/deploy/helm/plinko-pir

# Install with default values
helm install plinko-pir . \
  --namespace plinko-pir \
  --create-namespace

# Watch deployment progress
kubectl --namespace plinko-pir get pods -w
```

### Production Deployment

```bash
# 1. Customize values-vultr.yaml
cp values-vultr.yaml values-vultr-custom.yaml

# 2. Edit values-vultr-custom.yaml:
#    - Set your domain names
#    - Configure resource limits
#    - Enable TLS
#    - Set environment-specific configs

# 3. Install with custom values
helm install plinko-pir . \
  --namespace plinko-pir \
  --create-namespace \
  --values values-vultr-custom.yaml

# 4. Watch deployment
kubectl --namespace plinko-pir get pods -w
```

### DNS Configuration

```bash
# Get LoadBalancer IP
export LB_IP=$(kubectl --namespace ingress-nginx get service nginx-ingress-ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Add DNS A records (using your DNS provider):
# plinko-wallet.example.com -> $LB_IP
# plinko-api.example.com -> $LB_IP
# plinko-cdn.example.com -> $LB_IP

# Verify DNS resolution
dig plinko-wallet.example.com +short
dig plinko-api.example.com +short
dig plinko-cdn.example.com +short
```

## Verification

### Check Deployment Status

```bash
# Check all pods
kubectl --namespace plinko-pir get pods

# Expected output:
# NAME                                           READY   STATUS      RESTARTS   AGE
# plinko-pir-db-generator-xxxxx                 0/1     Completed   0          5m
# plinko-pir-hint-generator-xxxxx               0/1     Completed   0          8m
# plinko-pir-eth-mock-xxxxx                     1/1     Running     0          10m
# plinko-pir-update-service-xxxxx               1/1     Running     0          6m
# plinko-pir-pir-server-xxxxx                   1/1     Running     0          5m
# plinko-pir-pir-server-yyyyy                   1/1     Running     0          5m
# plinko-pir-cdn-mock-xxxxx                     1/1     Running     0          4m
# plinko-pir-cdn-mock-yyyyy                     1/1     Running     0          4m
# plinko-pir-rabby-wallet-xxxxx                 1/1     Running     0          3m
# plinko-pir-rabby-wallet-yyyyy                 1/1     Running     0          3m

# Check services
kubectl --namespace plinko-pir get services

# Check ingress
kubectl --namespace plinko-pir get ingress
```

### Test Services

```bash
# Test PIR server health
kubectl --namespace plinko-pir run test-curl --rm -it --image=curlimages/curl -- \
  curl http://plinko-pir-pir-server:3000/health

# Test CDN health
kubectl --namespace plinko-pir run test-curl --rm -it --image=curlimages/curl -- \
  curl http://plinko-pir-cdn-mock:8080/health

# Test wallet
kubectl --namespace plinko-pir run test-curl --rm -it --image=curlimages/curl -- \
  curl http://plinko-pir-rabby-wallet:80/

# Test external access (after DNS is configured)
curl https://plinko-wallet.example.com
curl https://plinko-api.example.com/health
curl https://plinko-cdn.example.com/health
```

### Verify Data Files

```bash
# Check PVC
kubectl --namespace plinko-pir get pvc

# Inspect data volume
kubectl --namespace plinko-pir run data-inspector --rm -it --image=busybox -- sh

# Inside the container:
ls -lh /data/
# Expected files:
# - database.bin (~64 MB)
# - address-mapping.bin (~192 MB)
# - hint.bin (~70 MB)
# - deltas/ (directory with delta files)

ls -lh /data/deltas/ | head -20
# Should show growing list of delta files
```

## Monitoring

### View Logs

```bash
# All pods
kubectl --namespace plinko-pir logs -l app.kubernetes.io/name=plinko-pir --tail=100

# Specific service
kubectl --namespace plinko-pir logs -l app.kubernetes.io/component=pir-server --tail=100 -f

# Job logs (db-generator, hint-generator)
kubectl --namespace plinko-pir logs job/plinko-pir-db-generator
kubectl --namespace plinko-pir logs job/plinko-pir-hint-generator
```

### Resource Usage

```bash
# Pod resource usage
kubectl --namespace plinko-pir top pods

# Node resource usage
kubectl top nodes

# Detailed pod info
kubectl --namespace plinko-pir describe pod <pod-name>
```

### Scaling

```bash
# Check HPA status
kubectl --namespace plinko-pir get hpa

# Manually scale (if HPA disabled)
kubectl --namespace plinko-pir scale deployment plinko-pir-pir-server --replicas=5

# Check scaling events
kubectl --namespace plinko-pir describe hpa plinko-pir-pir-server
```

## Troubleshooting

### Jobs Not Completing

```bash
# Check job status
kubectl --namespace plinko-pir get jobs

# Check job logs
kubectl --namespace plinko-pir logs job/plinko-pir-db-generator
kubectl --namespace plinko-pir logs job/plinko-pir-hint-generator

# Restart failed job
kubectl --namespace plinko-pir delete job plinko-pir-db-generator
helm upgrade plinko-pir . --namespace plinko-pir --reuse-values
```

### Pod Not Starting

```bash
# Describe pod to see events
kubectl --namespace plinko-pir describe pod <pod-name>

# Common issues:
# 1. Image pull errors - check image repository
# 2. Resource constraints - check node capacity
# 3. PVC not bound - check PVC status
# 4. InitContainer failures - check init logs

# Check events
kubectl --namespace plinko-pir get events --sort-by='.lastTimestamp'
```

### PVC Not Binding

```bash
# Check PVC status
kubectl --namespace plinko-pir get pvc

# Describe PVC
kubectl --namespace plinko-pir describe pvc plinko-pir-data

# Check storage class
kubectl get storageclass

# If Vultr Block Storage not available:
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: vultr-block-storage
provisioner: block.csi.vultr.com
allowVolumeExpansion: true
parameters:
  type: high_perf
volumeBindingMode: WaitForFirstConsumer
EOF
```

### Ingress Not Working

```bash
# Check ingress status
kubectl --namespace plinko-pir describe ingress plinko-pir

# Check ingress controller
kubectl --namespace ingress-nginx get pods
kubectl --namespace ingress-nginx logs -l app.kubernetes.io/component=controller

# Check LoadBalancer
kubectl --namespace ingress-nginx get service nginx-ingress-ingress-nginx-controller

# Test from inside cluster
kubectl --namespace plinko-pir run test --rm -it --image=curlimages/curl -- sh
# Inside: curl http://plinko-pir-rabby-wallet:80
```

### Performance Issues

```bash
# Check resource usage
kubectl --namespace plinko-pir top pods

# Check node capacity
kubectl describe nodes

# Increase resources
helm upgrade plinko-pir . \
  --namespace plinko-pir \
  --set plinkoPirServer.resources.limits.memory=4Gi \
  --set plinkoPirServer.resources.limits.cpu=2000m

# Scale up replicas
kubectl --namespace plinko-pir scale deployment plinko-pir-pir-server --replicas=10
```

## Upgrading

```bash
# Update values
vim values-vultr-custom.yaml

# Dry-run upgrade
helm upgrade plinko-pir . \
  --namespace plinko-pir \
  --values values-vultr-custom.yaml \
  --dry-run --debug

# Upgrade
helm upgrade plinko-pir . \
  --namespace plinko-pir \
  --values values-vultr-custom.yaml

# Rollback if needed
helm rollback plinko-pir --namespace plinko-pir
```

## Uninstallation

```bash
# Delete Helm release
helm uninstall plinko-pir --namespace plinko-pir

# Delete namespace (will delete PVC if not using "keep" policy)
kubectl delete namespace plinko-pir

# Delete PVC manually if using "keep" policy
kubectl --namespace plinko-pir delete pvc plinko-pir-data

# Delete ingress controller (optional)
helm uninstall nginx-ingress --namespace ingress-nginx
kubectl delete namespace ingress-nginx

# Delete cert-manager (optional)
helm uninstall cert-manager --namespace cert-manager
kubectl delete namespace cert-manager
```

## Cost Estimation (Vultr)

### Minimum Configuration (Development)

- **VKE Nodes**: 3 x "2 vCPU, 4GB RAM" @ $24/month = $72/month
- **Block Storage**: 20GB @ $2/month = $2/month
- **LoadBalancer**: 1 x Load Balancer @ $10/month = $10/month
- **Bandwidth**: ~1TB included, $0.01/GB overage

**Total**: ~$84/month

### Production Configuration

- **VKE Nodes**: 5 x "4 vCPU, 8GB RAM" @ $48/month = $240/month
- **Block Storage**: 50GB @ $5/month = $5/month
- **LoadBalancer**: 1 x Load Balancer @ $10/month = $10/month
- **Bandwidth**: ~2TB included, $0.01/GB overage
- **Snapshots**: Optional, ~$1/snapshot

**Total**: ~$255/month

### High Availability Configuration

- **VKE Nodes**: 10 x "4 vCPU, 8GB RAM" @ $48/month = $480/month
- **Block Storage**: 100GB @ $10/month = $10/month
- **LoadBalancer**: 1 x Load Balancer @ $10/month = $10/month
- **Bandwidth**: ~5TB included, $0.01/GB overage
- **Monitoring**: Optional add-on

**Total**: ~$500/month

## Advanced Configuration

### Custom Docker Images

If you've built custom Docker images:

```bash
# Tag and push images
docker tag plinko-pir/db-generator:latest your-registry/plinko-pir/db-generator:latest
docker push your-registry/plinko-pir/db-generator:latest

# Update values.yaml
dbGenerator:
  image:
    repository: your-registry/plinko-pir/db-generator
    tag: latest

# Or use --set flag
helm upgrade plinko-pir . \
  --namespace plinko-pir \
  --set dbGenerator.image.repository=your-registry/plinko-pir/db-generator
```

### Using Pre-generated Data

If you have pre-generated data files:

```bash
# Create PVC and upload data
kubectl --namespace plinko-pir apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: plinko-pir-data
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: vultr-block-storage
  resources:
    requests:
      storage: 20Gi
EOF

# Mount PVC and upload files
kubectl --namespace plinko-pir run data-uploader --image=busybox --rm -it -- sh

# From another terminal, copy files
kubectl --namespace plinko-pir cp ./data/database.bin data-uploader:/data/database.bin
kubectl --namespace plinko-pir cp ./data/hint.bin data-uploader:/data/hint.bin
kubectl --namespace plinko-pir cp ./data/deltas/ data-uploader:/data/deltas/

# Disable generator jobs
helm install plinko-pir . \
  --namespace plinko-pir \
  --set dbGenerator.enabled=false \
  --set plinkoHintGenerator.enabled=false
```

### Enable Monitoring

```bash
# Install Prometheus
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace

# Enable monitoring in values
monitoring:
  prometheus:
    enabled: true
    serviceMonitor:
      enabled: true

# Upgrade
helm upgrade plinko-pir . --namespace plinko-pir --values values-vultr-custom.yaml
```

## Support

- **GitHub Issues**: https://github.com/yourusername/plinko-pir-research/issues
- **Documentation**: https://github.com/yourusername/plinko-pir-research
- **Vultr Support**: https://my.vultr.com/support/

## License

MIT License - See LICENSE file for details
