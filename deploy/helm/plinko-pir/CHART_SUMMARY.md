# Plinko PIR Helm Chart - Summary

## Chart Structure

```
plinko-pir/
├── Chart.yaml                      # Chart metadata
├── values.yaml                     # Default configuration
├── values-vultr.yaml               # Vultr VKE optimized values
├── .helmignore                     # Files to ignore when packaging
├── README.md                       # Chart documentation
│
├── templates/                      # Kubernetes manifests
│   ├── _helpers.tpl                # Template helpers
│   ├── NOTES.txt                   # Post-install notes
│   │
│   ├── pvc.yaml                    # Persistent Volume Claim (20GB)
│   │
│   ├── eth-mock-deployment.yaml    # Anvil (Ethereum mock)
│   ├── eth-mock-service.yaml
│   │
│   ├── db-generator-job.yaml      # Database generation (one-time)
│   ├── hint-generator-job.yaml    # Hint generation (one-time)
│   │
│   ├── update-service-deployment.yaml  # Plinko update service
│   ├── update-service-service.yaml
│   │
│   ├── pir-server-deployment.yaml  # PIR query server
│   ├── pir-server-service.yaml
│   ├── pir-server-hpa.yaml         # Auto-scaling config
│   │
│   ├── cdn-configmap.yaml          # CDN nginx configuration
│   ├── cdn-deployment.yaml         # CDN for hint/deltas
│   ├── cdn-service.yaml
│   ├── cdn-hpa.yaml                # Auto-scaling config
│   │
│   ├── wallet-configmap.yaml       # Wallet configuration
│   ├── wallet-deployment.yaml      # Rabby wallet UI
│   ├── wallet-service.yaml
│   ├── wallet-hpa.yaml             # Auto-scaling config
│   │
│   └── ingress.yaml                # LoadBalancer ingress
│
├── scripts/                        # Deployment scripts
│   ├── deploy.sh                   # Automated deployment
│   └── verify.sh                   # Deployment verification
│
└── charts/                         # Subchart dependencies (empty)
```

## Deployment Flow

### Phase 1: Initialization (One-Time)

1. **PVC Creation**: 20GB Vultr Block Storage volume
2. **eth-mock**: Starts Anvil with 8.4M accounts
3. **db-generator** (Job): Queries all accounts → `database.bin` (64 MB)
4. **hint-generator** (Job): Generates PIR hints → `hint.bin` (~70 MB)

**Duration**: ~10-15 minutes

### Phase 2: Runtime Services

5. **update-service**: Monitors blockchain, generates delta files
6. **pir-server**: Serves private queries (2-20 replicas, auto-scaling)
7. **cdn-mock**: Serves hint.bin and deltas (2-10 replicas, auto-scaling)
8. **rabby-wallet**: User interface (2-5 replicas, auto-scaling)

### Phase 3: External Access

9. **Ingress**: Vultr LoadBalancer routes traffic to services
10. **DNS**: Custom domains point to LoadBalancer IP
11. **TLS**: cert-manager provisions Let's Encrypt certificates (optional)

## Resource Allocation

### Per-Service Requests/Limits

| Service | Replicas | CPU Request | CPU Limit | Memory Request | Memory Limit |
|---------|----------|-------------|-----------|----------------|--------------|
| eth-mock | 1 | 1000m | 2000m | 2Gi | 4Gi |
| db-generator (job) | 1 | 500m | 1000m | 1Gi | 2Gi |
| hint-generator (job) | 1 | 1000m | 2000m | 2Gi | 4Gi |
| update-service | 1 | 250m | 500m | 512Mi | 1Gi |
| pir-server | 2-20 | 500m | 1000m | 1Gi | 2Gi |
| cdn-mock | 2-10 | 100m | 250m | 128Mi | 256Mi |
| rabby-wallet | 2-5 | 100m | 250m | 128Mi | 256Mi |

### Total Cluster Requirements

**Minimum (Development)**:
- Nodes: 3 × "2 vCPU, 4GB RAM"
- Total: 6 vCPU, 12GB RAM
- Storage: 20GB
- Cost: ~$84/month

**Production**:
- Nodes: 5 × "4 vCPU, 8GB RAM"
- Total: 20 vCPU, 40GB RAM
- Storage: 50GB
- Cost: ~$255/month

**High Availability**:
- Nodes: 10 × "4 vCPU, 8GB RAM"
- Total: 40 vCPU, 80GB RAM
- Storage: 100GB
- Cost: ~$550/month

## Key Features

### 1. Automated Initialization

- Jobs run sequentially with init containers
- Dependency management via Kubernetes primitives
- Automatic retry with backoff

### 2. High Availability

- Multiple replicas for all critical services
- Health checks and readiness probes
- Rolling updates with zero downtime
- Anti-affinity for pod distribution

### 3. Auto-Scaling

- HPA for PIR server (CPU/memory based)
- HPA for CDN (CPU based)
- HPA for wallet (CPU based)
- Scale from 2 to 20 replicas automatically

### 4. Data Management

- Shared PVC for all services
- ReadWriteMany access mode
- Vultr Block Storage high-performance SSD
- Volume snapshots for backup (optional)

### 5. Network Configuration

- ClusterIP for internal services
- LoadBalancer ingress for external access
- CORS configuration for CDN
- Gzip compression for hint.bin

### 6. Security

- Non-root containers
- Read-only root filesystem where possible
- Capability dropping
- Network policies (optional)
- Pod security policies (optional)

## Configuration Options

### values.yaml Highlights

```yaml
# Global settings
global:
  storageClass: "vultr-block-storage"
  domain: ""
  tls:
    enabled: false

# Persistent storage
persistence:
  size: 15Gi
  accessMode: ReadWriteMany

# Service scaling
plinkoPirServer:
  replicaCount: 2
  autoscaling:
    enabled: true
    minReplicas: 2
    maxReplicas: 10

# Ingress
ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: ""
      paths:
        - path: /
          service: rabby-wallet
```

### values-vultr.yaml Highlights

```yaml
# Production-ready Vultr VKE configuration
global:
  storageClass: "vultr-block-storage"

persistence:
  size: 20Gi

plinkoPirServer:
  replicaCount: 3
  autoscaling:
    minReplicas: 3
    maxReplicas: 20

ingress:
  annotations:
    # Vultr LoadBalancer specific
    service.beta.kubernetes.io/vultr-loadbalancer-protocol: "tcp"
    service.beta.kubernetes.io/vultr-loadbalancer-healthcheck-protocol: "http"
```

## Verification Checklist

After deployment, verify:

- [ ] Namespace created
- [ ] PVC bound to 20GB volume
- [ ] eth-mock running (1/1 pods)
- [ ] db-generator job completed
- [ ] hint-generator job completed
- [ ] update-service running (1/1 pods)
- [ ] pir-server running (2+ pods)
- [ ] cdn-mock running (2+ pods)
- [ ] rabby-wallet running (2+ pods)
- [ ] All services have ClusterIP
- [ ] Ingress has LoadBalancer IP
- [ ] Data files exist (database.bin, hint.bin, deltas/)
- [ ] PIR server health check passes
- [ ] CDN health check passes
- [ ] Wallet accessible via ingress

## Common Commands

```bash
# Deploy
helm install plinko-pir . --namespace plinko-pir --create-namespace

# Upgrade
helm upgrade plinko-pir . --namespace plinko-pir --values values-vultr.yaml

# Rollback
helm rollback plinko-pir --namespace plinko-pir

# Uninstall
helm uninstall plinko-pir --namespace plinko-pir

# Status
helm status plinko-pir --namespace plinko-pir

# Values
helm get values plinko-pir --namespace plinko-pir

# Template (dry-run)
helm template plinko-pir . --namespace plinko-pir

# Lint
helm lint .
```

## Dependencies

### Required

- Kubernetes 1.21+
- Helm 3.8+
- kubectl 1.21+
- Nginx Ingress Controller

### Optional

- cert-manager (for TLS)
- Prometheus (for monitoring)
- Grafana (for dashboards)
- Loki (for logging)

## Files Overview

| File | Purpose | Size |
|------|---------|------|
| Chart.yaml | Chart metadata | 480B |
| values.yaml | Default configuration | 12KB |
| values-vultr.yaml | Vultr-optimized config | 8KB |
| templates/_helpers.tpl | Template helpers | 2KB |
| templates/pvc.yaml | Storage claim | 500B |
| templates/eth-mock-* | Anvil deployment | 1.5KB |
| templates/db-generator-job.yaml | DB generation | 1.5KB |
| templates/hint-generator-job.yaml | Hint generation | 2KB |
| templates/update-service-* | Update service | 2.5KB |
| templates/pir-server-* | PIR server | 3KB |
| templates/cdn-* | CDN mock | 3.5KB |
| templates/wallet-* | Rabby wallet | 2.5KB |
| templates/ingress.yaml | LoadBalancer ingress | 1KB |
| scripts/deploy.sh | Deployment automation | 8KB |
| scripts/verify.sh | Verification script | 10KB |

## Production Readiness

### Implemented

- [x] Multi-replica deployments
- [x] Health checks and readiness probes
- [x] Rolling updates
- [x] Horizontal pod autoscaling
- [x] Resource requests and limits
- [x] Persistent storage
- [x] LoadBalancer ingress
- [x] ConfigMaps for configuration
- [x] Init containers for dependencies
- [x] Automated deployment scripts
- [x] Verification scripts

### Recommended for Production

- [ ] Enable TLS with cert-manager
- [ ] Configure custom domain names
- [ ] Enable monitoring (Prometheus)
- [ ] Enable logging (Loki/ELK)
- [ ] Enable volume snapshots
- [ ] Configure backup schedule
- [ ] Set up alerts (Alertmanager)
- [ ] Configure network policies
- [ ] Enable pod security policies
- [ ] Set up disaster recovery

## Troubleshooting

### Issue: Jobs Not Completing

**Check**: `kubectl --namespace plinko-pir logs job/<job-name>`

**Solution**: Increase resources or timeout

### Issue: PVC Not Binding

**Check**: `kubectl get storageclass`

**Solution**: Ensure Vultr CSI driver installed

### Issue: Pods Pending

**Check**: `kubectl --namespace plinko-pir describe pod <pod-name>`

**Solution**: Increase node count or resources

### Issue: Ingress Not Working

**Check**: `kubectl --namespace plinko-pir describe ingress plinko-pir`

**Solution**: Verify nginx ingress controller, DNS, LoadBalancer

### Issue: Performance Problems

**Check**: `kubectl --namespace plinko-pir top pods`

**Solution**: Scale replicas or increase resources

## References

- **Helm Documentation**: https://helm.sh/docs/
- **Kubernetes Documentation**: https://kubernetes.io/docs/
- **Vultr VKE Documentation**: https://www.vultr.com/docs/vultr-kubernetes-engine
- **Nginx Ingress**: https://kubernetes.github.io/ingress-nginx/
- **cert-manager**: https://cert-manager.io/docs/

## Support

For issues:

1. Check logs: `kubectl logs <pod-name>`
2. Check events: `kubectl get events`
3. Run verify script: `./scripts/verify.sh`
4. Consult README.md
5. Open GitHub issue
