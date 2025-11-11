# Plinko PIR Deployment on Vultr Kubernetes Engine (VKE)

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Deployment Options](#deployment-options)
3. [Quick Start](#quick-start)
4. [Manual Deployment](#manual-deployment)
5. [GitHub Actions CI/CD](#github-actions-cicd)
6. [Troubleshooting](#troubleshooting)
7. [Rollback Procedures](#rollback-procedures)

## Prerequisites

### Required Tools
- `kubectl` - Kubernetes CLI
- `helm` (v3+) - Kubernetes package manager
- Docker and Docker Hub account (for custom images)
- VKE kubeconfig file from Vultr

### VKE Cluster Requirements
- Kubernetes version: 1.34+
- Minimum 2 nodes
- Node resources: 4 CPU / 8GB RAM per node
- Storage: Vultr Block Storage provisioner enabled
- LoadBalancer: Vultr Load Balancer available

### Kubeconfig Setup

**CRITICAL SECURITY**: Never commit kubeconfig files to git!

```bash
# Download kubeconfig from Vultr dashboard
# Save to secure location outside repository
export KUBECONFIG=~/pse/k8s/pir-vke-<cluster-id>.yaml

# Test connectivity
kubectl cluster-info
kubectl get nodes
```

The `.gitignore` is configured to exclude:
- `*.kubeconfig`
- `*-vke-*.yaml`
- `.kube/config`

## Deployment Options

### Option 1: Using Pre-Built Binary Data (Fastest)

If you have pre-generated `database.bin`, `hint.bin`, and delta files:

```bash
# 1. Create namespace
kubectl create namespace plinko-pir

# 2. Create PVC
kubectl apply -f deploy/helm/plinko-pir/templates/pvc.yaml -n plinko-pir

# 3. Upload data files to PVC
kubectl run -it --rm data-loader \
  --image=busybox \
  --namespace=plinko-pir \
  --restart=Never \
  --overrides='
{
  "spec": {
    "containers": [{
      "name": "data-loader",
      "image": "busybox",
      "command": ["sh"],
      "volumeMounts": [{
        "name": "data",
        "mountPath": "/data"
      }]
    }],
    "volumes": [{
      "name": "data",
      "persistentVolumeClaim": {
        "claimName": "plinko-pir-data"
      }
    }]
  }
}'

# Inside the pod, use wget or transfer files
# Then deploy with data services disabled
helm install plinko-pir ./deploy/helm/plinko-pir \
  --namespace plinko-pir \
  --set dbGenerator.enabled=false \
  --set plinkoHintGenerator.enabled=false
```

### Option 2: Full Deployment with Docker Registry

Build and push images to Docker Hub or private registry:

```bash
# 1. Login to Docker Hub
docker login

# 2. Build and tag images
export REGISTRY=your-dockerhub-username
make build

# Tag each image
docker tag plinko-pir-research-db-generator:latest ${REGISTRY}/plinko-db-generator:latest
docker tag plinko-pir-research-plinko-hint-generator:latest ${REGISTRY}/plinko-hint-generator:latest
docker tag plinko-pir-research-plinko-update-service:latest ${REGISTRY}/plinko-update-service:latest
docker tag plinko-pir-research-plinko-pir-server:latest ${REGISTRY}/plinko-pir-server:latest
docker tag plinko-pir-research-cdn-mock:latest ${REGISTRY}/plinko-cdn-mock:latest
docker tag plinko-pir-research-rabby-wallet:latest ${REGISTRY}/plinko-rabby-wallet:latest

# Push to registry
docker push ${REGISTRY}/plinko-db-generator:latest
docker push ${REGISTRY}/plinko-hint-generator:latest
docker push ${REGISTRY}/plinko-update-service:latest
docker push ${REGISTRY}/plinko-pir-server:latest
docker push ${REGISTRY}/plinko-cdn-mock:latest
docker push ${REGISTRY}/plinko-rabby-wallet:latest

# 3. Update values file with your registry
cat > deploy/helm/plinko-pir/values-vke-custom.yaml <<EOF
dbGenerator:
  image:
    repository: ${REGISTRY}/plinko-db-generator
    tag: latest

plinkoHintGenerator:
  image:
    repository: ${REGISTRY}/plinko-hint-generator
    tag: latest

plinkoUpdateService:
  image:
    repository: ${REGISTRY}/plinko-update-service
    tag: latest

plinkoPirServer:
  image:
    repository: ${REGISTRY}/plinko-pir-server
    tag: latest

cdnMock:
  image:
    repository: ${REGISTRY}/plinko-cdn-mock
    tag: latest

rabbyWallet:
  image:
    repository: ${REGISTRY}/plinko-rabby-wallet
    tag: latest
EOF

# 4. Deploy
helm install plinko-pir ./deploy/helm/plinko-pir \
  --namespace plinko-pir \
  --create-namespace \
  -f deploy/helm/plinko-pir/values-vke-simple.yaml \
  -f deploy/helm/plinko-pir/values-vke-custom.yaml
```

## Quick Start

### Step 1: Set Kubeconfig
```bash
export KUBECONFIG=~/pse/k8s/pir-vke-<cluster-id>.yaml
kubectl cluster-info
```

### Step 2: Verify Cluster Resources
```bash
# Check nodes
kubectl get nodes

# Check storage classes
kubectl get storageclass

# Verify LoadBalancer availability
kubectl get svc -A | grep LoadBalancer
```

### Step 3: Deploy with Helm
```bash
cd /Users/user/pse/plinko-pir-research

# Initial deployment (only eth-mock and storage)
helm install plinko-pir ./deploy/helm/plinko-pir \
  --namespace plinko-pir \
  --create-namespace \
  -f deploy/helm/plinko-pir/values-vke-simple.yaml \
  --wait \
  --timeout 10m
```

### Step 4: Monitor Deployment
```bash
# Watch pod status
kubectl get pods -n plinko-pir -w

# Check logs
kubectl logs -n plinko-pir -l app.kubernetes.io/name=plinko-pir --tail=50

# Get service details
kubectl get svc -n plinko-pir
```

### Step 5: Get External IPs (LoadBalancer)
```bash
# Get LoadBalancer IPs
kubectl get svc -n plinko-pir -o wide

# Example output:
# NAME                SERVICE_IP      EXTERNAL_IP      PORT(S)
# plinko-pir-server   10.x.x.x       45.76.x.x        3000:30123/TCP
# cdn-mock            10.x.x.x       45.76.x.x        8080:30124/TCP
# rabby-wallet        10.x.x.x       45.76.x.x        80:30125/TCP
```

### Step 6: Update Wallet Configuration
Once you have LoadBalancer IPs, update the wallet configuration:

```bash
# Upgrade deployment with external URLs
helm upgrade plinko-pir ./deploy/helm/plinko-pir \
  --namespace plinko-pir \
  -f deploy/helm/plinko-pir/values-vke-simple.yaml \
  --set rabbyWallet.config.pirServerUrl=http://45.76.x.x:3000 \
  --set rabbyWallet.config.cdnUrl=http://45.76.x.x:8080
```

## Manual Deployment

### Phase 1: Storage Setup
```bash
# Create PVC
kubectl apply -f deploy/helm/plinko-pir/templates/pvc.yaml -n plinko-pir

# Verify PVC bound
kubectl get pvc -n plinko-pir
```

### Phase 2: Ethereum Mock
```bash
# Deploy eth-mock (Anvil)
kubectl apply -f deploy/helm/plinko-pir/templates/eth-mock-deployment.yaml -n plinko-pir
kubectl apply -f deploy/helm/plinko-pir/templates/eth-mock-service.yaml -n plinko-pir

# Wait for ready
kubectl wait --for=condition=ready pod -l app=eth-mock -n plinko-pir --timeout=5m
```

### Phase 3: Data Generation
```bash
# Deploy db-generator job
kubectl apply -f deploy/helm/plinko-pir/templates/db-generator-job.yaml -n plinko-pir

# Monitor job completion
kubectl logs -f job/db-generator -n plinko-pir

# Deploy hint-generator job (after db-generator completes)
kubectl apply -f deploy/helm/plinko-pir/templates/hint-generator-job.yaml -n plinko-pir
kubectl logs -f job/hint-generator -n plinko-pir
```

### Phase 4: Deploy Services
```bash
# PIR Server
kubectl apply -f deploy/helm/plinko-pir/templates/pir-server-deployment.yaml -n plinko-pir
kubectl apply -f deploy/helm/plinko-pir/templates/pir-server-service.yaml -n plinko-pir

# Update Service
kubectl apply -f deploy/helm/plinko-pir/templates/update-service-deployment.yaml -n plinko-pir
kubectl apply -f deploy/helm/plinko-pir/templates/update-service-service.yaml -n plinko-pir

# CDN
kubectl apply -f deploy/helm/plinko-pir/templates/cdn-configmap.yaml -n plinko-pir
kubectl apply -f deploy/helm/plinko-pir/templates/cdn-deployment.yaml -n plinko-pir
kubectl apply -f deploy/helm/plinko-pir/templates/cdn-service.yaml -n plinko-pir

# Wallet
kubectl apply -f deploy/helm/plinko-pir/templates/wallet-configmap.yaml -n plinko-pir
kubectl apply -f deploy/helm/plinko-pir/templates/wallet-deployment.yaml -n plinko-pir
kubectl apply -f deploy/helm/plinko-pir/templates/wallet-service.yaml -n plinko-pir
```

## GitHub Actions CI/CD

### Setup Instructions

#### Step 1: Encode Kubeconfig
```bash
# Encode kubeconfig to base64
cat ~/pse/k8s/pir-vke-<cluster-id>.yaml | base64 > kubeconfig-base64.txt

# Copy the output
cat kubeconfig-base64.txt
```

#### Step 2: Create GitHub Secrets
1. Go to repository Settings > Secrets and variables > Actions
2. Click "New repository secret"
3. Create the following secrets:

| Secret Name | Value | Description |
|------------|-------|-------------|
| `VKE_KUBECONFIG` | Base64-encoded kubeconfig | From Step 1 |
| `DOCKER_USERNAME` | Your Docker Hub username | For image registry |
| `DOCKER_PASSWORD` | Your Docker Hub token | For image registry |
| `REGISTRY` | Docker Hub username or registry URL | Image repository |

#### Step 3: Enable GitHub Actions
The workflow file `.github/workflows/deploy-vke.yml` is already configured.

To trigger deployment:
```bash
# Automatic: Push to main branch
git push origin main

# Manual: Use GitHub Actions UI
# Go to Actions > Deploy to Vultr VKE > Run workflow
```

### Workflow Features
- Automatic deployment on push to `main`
- Manual deployment via workflow_dispatch
- Docker image building and pushing
- Helm deployment with rollback on failure
- Deployment verification
- Slack/Discord notifications (optional)

## Troubleshooting

### Pods Not Starting
```bash
# Check pod events
kubectl describe pod <pod-name> -n plinko-pir

# Common issues:
# 1. Image pull errors - check image registry and credentials
# 2. PVC not bound - check storage class and provisioner
# 3. Insufficient resources - check node capacity
```

### Storage Issues
```bash
# Check PVC status
kubectl get pvc -n plinko-pir

# Check PV details
kubectl get pv

# If PVC stuck in Pending:
# 1. Verify storage class exists
kubectl get storageclass vultr-block-storage

# 2. Check provisioner logs
kubectl logs -n kube-system -l app=csi-vultr-controller

# 3. Try different access mode (ReadWriteOnce vs ReadWriteMany)
```

### LoadBalancer Not Getting External IP
```bash
# Check service status
kubectl get svc -n plinko-pir

# If stuck in Pending:
# 1. Verify LoadBalancer quota in Vultr
# 2. Check service annotations
kubectl describe svc <service-name> -n plinko-pir

# 3. Check LoadBalancer controller logs
kubectl logs -n kube-system -l app=vultr-cloud-controller-manager
```

### Network Connectivity Issues
```bash
# Test internal connectivity
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -n plinko-pir -- sh

# Inside pod:
curl http://eth-mock:8545
curl http://plinko-pir-server:3000/health
curl http://cdn-mock:8080/health

# Test external connectivity
curl http://<EXTERNAL_IP>:3000/health
```

### Data Generation Failures
```bash
# Check db-generator logs
kubectl logs job/db-generator -n plinko-pir

# Common issues:
# 1. Insufficient memory - increase job resources
# 2. Timeout - increase job timeout
# 3. RPC connection failed - check eth-mock availability

# Restart job
kubectl delete job db-generator -n plinko-pir
kubectl apply -f deploy/helm/plinko-pir/templates/db-generator-job.yaml -n plinko-pir
```

## Rollback Procedures

### Helm Rollback
```bash
# List releases
helm list -n plinko-pir

# View release history
helm history plinko-pir -n plinko-pir

# Rollback to previous version
helm rollback plinko-pir -n plinko-pir

# Rollback to specific revision
helm rollback plinko-pir 2 -n plinko-pir
```

### Manual Rollback
```bash
# Save current deployment
kubectl get deployment -n plinko-pir -o yaml > backup-deployment.yaml

# Restore from backup
kubectl apply -f backup-deployment.yaml
```

### Emergency Procedures

#### Full Cleanup
```bash
# Remove all resources but keep PVC (data preserved)
helm uninstall plinko-pir -n plinko-pir

# Remove namespace (includes PVC - data lost!)
kubectl delete namespace plinko-pir
```

#### Data Backup Before Cleanup
```bash
# Create volume snapshot (if supported)
kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: plinko-data-backup-$(date +%Y%m%d)
  namespace: plinko-pir
spec:
  source:
    persistentVolumeClaimName: plinko-pir-data
EOF

# Or copy data out
kubectl cp plinko-pir/<pod-name>:/data ./data-backup -n plinko-pir
```

## Monitoring and Maintenance

### Health Checks
```bash
# All pods healthy
kubectl get pods -n plinko-pir

# Service endpoints
kubectl get endpoints -n plinko-pir

# Resource usage
kubectl top pods -n plinko-pir
kubectl top nodes
```

### Log Management
```bash
# Stream logs
kubectl logs -f -n plinko-pir -l app.kubernetes.io/name=plinko-pir

# Export logs
kubectl logs -n plinko-pir <pod-name> > logs.txt

# Persistent logging (optional)
# Install Loki stack or forward to external logging service
```

### Scaling
```bash
# Manual scaling
kubectl scale deployment plinko-pir-server --replicas=5 -n plinko-pir

# Check HPA status
kubectl get hpa -n plinko-pir

# Update HPA
kubectl edit hpa plinko-pir-server -n plinko-pir
```

## Security Best Practices

1. **Kubeconfig Management**
   - Never commit kubeconfig to git
   - Use GitHub Secrets for CI/CD
   - Rotate credentials regularly
   - Use RBAC to limit access

2. **Image Security**
   - Use private registry for custom images
   - Scan images for vulnerabilities
   - Use specific image tags (not `latest`)
   - Enable image pull secrets if needed

3. **Network Security**
   - Use NetworkPolicy to restrict traffic
   - Enable TLS for external endpoints
   - Use private IPs where possible
   - Implement firewall rules

4. **Data Security**
   - Enable encryption at rest for PVCs
   - Regular backups via volume snapshots
   - Access control via PodSecurityPolicy
   - Audit logging enabled

## Cost Optimization

1. **Resource Tuning**
   - Right-size resource requests/limits
   - Use spot instances for non-critical workloads
   - Enable cluster autoscaling

2. **Storage**
   - Use appropriate storage class (SSD vs HDD)
   - Clean up unused PVCs
   - Enable compression for large files

3. **LoadBalancer**
   - Use single LoadBalancer with Ingress
   - Share LoadBalancer across services
   - Consider NodePort for internal services

## Support and Contribution

- **Documentation**: See `/deploy/helm/plinko-pir/README.md`
- **Issues**: GitHub Issues
- **Discussions**: GitHub Discussions
- **Updates**: Check CHANGELOG.md

---

**Last Updated**: 2025-11-11
**Version**: 1.0.0
