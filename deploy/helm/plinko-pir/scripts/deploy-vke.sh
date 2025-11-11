#!/bin/bash

# =============================================================================
# Plinko PIR - Vultr VKE Deployment Script
# =============================================================================
# Deploys Plinko PIR to Vultr Kubernetes Engine
# Usage: ./deploy-vke.sh [--kubeconfig PATH] [--namespace NAME] [--registry REGISTRY]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
KUBECONFIG_PATH="${KUBECONFIG:-$HOME/.kube/config}"
NAMESPACE="plinko-pir"
REGISTRY="${REGISTRY:-}"
HELM_RELEASE="plinko-pir"
TIMEOUT="15m"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --kubeconfig)
      KUBECONFIG_PATH="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --registry)
      REGISTRY="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --kubeconfig PATH    Path to kubeconfig file (default: \$KUBECONFIG or ~/.kube/config)"
      echo "  --namespace NAME     Kubernetes namespace (default: plinko-pir)"
      echo "  --registry REGISTRY  Docker registry for custom images (optional)"
      echo "  --help               Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

# Print banner
echo -e "${BLUE}"
cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║   PLINKO PIR - VULTR VKE DEPLOYMENT                          ║
║   Private Information Retrieval for Ethereum                 ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

# Step 1: Validate prerequisites
echo -e "${BLUE}[1/8] Validating prerequisites...${NC}"

# Check kubectl
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl not found. Please install kubectl.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ kubectl found${NC}"

# Check helm
if ! command -v helm &> /dev/null; then
    echo -e "${RED}Error: helm not found. Please install Helm 3.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ helm found${NC}"

# Check kubeconfig
if [ ! -f "$KUBECONFIG_PATH" ]; then
    echo -e "${RED}Error: Kubeconfig not found at $KUBECONFIG_PATH${NC}"
    exit 1
fi
export KUBECONFIG="$KUBECONFIG_PATH"
echo -e "${GREEN}✓ kubeconfig loaded from $KUBECONFIG_PATH${NC}"

# Step 2: Verify cluster connectivity
echo -e "\n${BLUE}[2/8] Verifying cluster connectivity...${NC}"
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}Error: Cannot connect to cluster. Check your kubeconfig.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Connected to cluster${NC}"

# Get cluster info
CLUSTER_ENDPOINT=$(kubectl cluster-info | grep "Kubernetes control plane" | awk '{print $NF}')
echo "   Cluster: $CLUSTER_ENDPOINT"

# Check nodes
NODE_COUNT=$(kubectl get nodes --no-headers | wc -l | tr -d ' ')
echo "   Nodes: $NODE_COUNT"
kubectl get nodes

# Step 3: Check storage classes
echo -e "\n${BLUE}[3/8] Checking storage classes...${NC}"
if ! kubectl get storageclass vultr-block-storage &> /dev/null; then
    echo -e "${YELLOW}Warning: vultr-block-storage storage class not found${NC}"
    echo "Available storage classes:"
    kubectl get storageclass
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo -e "${GREEN}✓ vultr-block-storage found${NC}"
fi

# Step 4: Create namespace
echo -e "\n${BLUE}[4/8] Creating namespace...${NC}"
if kubectl get namespace "$NAMESPACE" &> /dev/null; then
    echo -e "${YELLOW}Namespace $NAMESPACE already exists${NC}"
else
    kubectl create namespace "$NAMESPACE"
    echo -e "${GREEN}✓ Namespace $NAMESPACE created${NC}"
fi

# Step 5: Prepare values file
echo -e "\n${BLUE}[5/8] Preparing Helm values...${NC}"
CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VALUES_FILE="$CHART_DIR/values-vke-simple.yaml"

if [ ! -f "$VALUES_FILE" ]; then
    echo -e "${RED}Error: Values file not found at $VALUES_FILE${NC}"
    exit 1
fi

# Create temporary values override if registry specified
TEMP_VALUES=""
if [ -n "$REGISTRY" ]; then
    echo -e "${YELLOW}Using custom registry: $REGISTRY${NC}"
    TEMP_VALUES=$(mktemp)
    cat > "$TEMP_VALUES" <<EOF
# Image registry override
dbGenerator:
  image:
    repository: $REGISTRY/plinko-db-generator
    tag: latest

plinkoHintGenerator:
  image:
    repository: $REGISTRY/plinko-hint-generator
    tag: latest

plinkoUpdateService:
  image:
    repository: $REGISTRY/plinko-update-service
    tag: latest

plinkoPirServer:
  image:
    repository: $REGISTRY/plinko-pir-server
    tag: latest

cdnMock:
  image:
    repository: $REGISTRY/plinko-cdn-mock
    tag: latest

rabbyWallet:
  image:
    repository: $REGISTRY/plinko-rabby-wallet
    tag: latest
EOF
fi

# Step 6: Deploy with Helm
echo -e "\n${BLUE}[6/8] Deploying with Helm...${NC}"
echo "   Release: $HELM_RELEASE"
echo "   Namespace: $NAMESPACE"
echo "   Chart: $CHART_DIR"
echo "   Timeout: $TIMEOUT"

HELM_CMD="helm upgrade --install $HELM_RELEASE $CHART_DIR \
    --namespace $NAMESPACE \
    --create-namespace \
    -f $VALUES_FILE \
    --wait \
    --timeout $TIMEOUT"

if [ -n "$TEMP_VALUES" ]; then
    HELM_CMD="$HELM_CMD -f $TEMP_VALUES"
fi

if eval $HELM_CMD; then
    echo -e "${GREEN}✓ Helm deployment successful${NC}"
else
    echo -e "${RED}✗ Helm deployment failed${NC}"

    # Show recent events
    echo -e "\n${YELLOW}Recent events:${NC}"
    kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' | tail -20

    # Cleanup temp file
    [ -n "$TEMP_VALUES" ] && rm -f "$TEMP_VALUES"
    exit 1
fi

# Cleanup temp file
[ -n "$TEMP_VALUES" ] && rm -f "$TEMP_VALUES"

# Step 7: Verify deployment
echo -e "\n${BLUE}[7/8] Verifying deployment...${NC}"

echo -e "\n${YELLOW}Pod Status:${NC}"
kubectl get pods -n "$NAMESPACE" -o wide

echo -e "\n${YELLOW}Service Status:${NC}"
kubectl get svc -n "$NAMESPACE" -o wide

echo -e "\n${YELLOW}PVC Status:${NC}"
kubectl get pvc -n "$NAMESPACE"

# Step 8: Get access information
echo -e "\n${BLUE}[8/8] Retrieving access information...${NC}"

# Wait for LoadBalancer IPs
echo "Waiting for LoadBalancer IPs (this may take 1-2 minutes)..."
sleep 30

# Get LoadBalancer IPs
PIR_SERVER_IP=$(kubectl get svc plinko-pir-pir-server -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
CDN_IP=$(kubectl get svc plinko-pir-cdn-mock -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")
WALLET_IP=$(kubectl get svc plinko-pir-rabby-wallet -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")

# Print summary
echo -e "\n${GREEN}"
cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║   DEPLOYMENT SUCCESSFUL!                                      ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

echo -e "${BLUE}Access URLs:${NC}"
echo ""

if [ "$PIR_SERVER_IP" != "pending" ]; then
    echo -e "  ${GREEN}PIR Server:${NC}  http://$PIR_SERVER_IP:3000"
    echo -e "               http://$PIR_SERVER_IP:3000/health"
else
    echo -e "  ${YELLOW}PIR Server:${NC}  LoadBalancer IP pending"
fi

if [ "$CDN_IP" != "pending" ]; then
    echo -e "  ${GREEN}CDN:${NC}         http://$CDN_IP:8080"
    echo -e "               http://$CDN_IP:8080/health"
else
    echo -e "  ${YELLOW}CDN:${NC}         LoadBalancer IP pending"
fi

if [ "$WALLET_IP" != "pending" ]; then
    echo -e "  ${GREEN}Wallet UI:${NC}   http://$WALLET_IP"
else
    echo -e "  ${YELLOW}Wallet UI:${NC}   LoadBalancer IP pending"
fi

echo ""
echo -e "${BLUE}Next Steps:${NC}"
echo "  1. Wait for all pods to be ready: kubectl get pods -n $NAMESPACE -w"
echo "  2. Monitor logs: kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=plinko-pir -f"
echo "  3. Access the wallet UI and enable Privacy Mode"
echo "  4. Test private balance queries"
echo ""
echo -e "${BLUE}Management Commands:${NC}"
echo "  View status:     kubectl get all -n $NAMESPACE"
echo "  View logs:       kubectl logs -n $NAMESPACE <pod-name>"
echo "  Scale service:   kubectl scale deployment plinko-pir-pir-server --replicas=5 -n $NAMESPACE"
echo "  Delete release:  helm uninstall $HELM_RELEASE -n $NAMESPACE"
echo ""
echo -e "${YELLOW}Note: If LoadBalancer IPs are pending, check them later with:${NC}"
echo "  kubectl get svc -n $NAMESPACE"
echo ""
