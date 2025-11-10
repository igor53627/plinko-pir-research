#!/bin/bash
# =============================================================================
# PLINKO PIR - LOCAL KUBERNETES DEPLOYMENT SCRIPT
# =============================================================================
# This script automates deployment of Plinko PIR to local Kubernetes (Kind/Minikube)
#
# Usage:
#   ./deploy-local.sh                    # Deploy with Kind (default)
#   ./deploy-local.sh --minikube         # Deploy with Minikube
#   ./deploy-local.sh --cluster-name my-cluster  # Custom cluster name
#
# Prerequisites:
#   - Docker installed and running
#   - kubectl installed
#   - helm 3.8+ installed
#   - Kind or Minikube installed (will check)
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="plinko-pir"
RELEASE_NAME="plinko-pir"
CHART_PATH="$(dirname "$0")/.."
VALUES_FILE="values-local.yaml"
CLUSTER_NAME="plinko-pir-local"
USE_KIND=true
USE_MINIKUBE=false
SKIP_CLUSTER_CREATE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --minikube)
      USE_KIND=false
      USE_MINIKUBE=true
      shift
      ;;
    --kind)
      USE_KIND=true
      USE_MINIKUBE=false
      shift
      ;;
    --cluster-name)
      CLUSTER_NAME="$2"
      shift 2
      ;;
    --skip-cluster-create)
      SKIP_CLUSTER_CREATE=true
      shift
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  --kind                      Use Kind (default)"
      echo "  --minikube                  Use Minikube instead of Kind"
      echo "  --cluster-name <name>       Cluster name (default: plinko-pir-local)"
      echo "  --skip-cluster-create       Skip cluster creation (use existing)"
      echo "  --namespace <name>          Kubernetes namespace (default: plinko-pir)"
      echo "  --help                      Show this help message"
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      exit 1
      ;;
  esac
done

# Functions
log_info() {
  echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
  echo -e "${BLUE}==>${NC} $1"
}

check_prerequisites() {
  log_step "Checking prerequisites..."

  # Check Docker
  if ! command -v docker &> /dev/null; then
    log_error "Docker not found. Please install Docker Desktop or Docker Engine."
    exit 1
  fi

  # Check if Docker is running
  if ! docker info &> /dev/null; then
    log_error "Docker is not running. Please start Docker."
    exit 1
  fi

  # Check kubectl
  if ! command -v kubectl &> /dev/null; then
    log_error "kubectl not found. Please install kubectl first."
    echo "  macOS: brew install kubectl"
    echo "  Linux: See https://kubernetes.io/docs/tasks/tools/"
    exit 1
  fi

  # Check helm
  if ! command -v helm &> /dev/null; then
    log_error "helm not found. Please install helm first."
    echo "  macOS: brew install helm"
    echo "  Linux: See https://helm.sh/docs/intro/install/"
    exit 1
  fi

  # Check Kind or Minikube
  if [ "$USE_KIND" = true ]; then
    if ! command -v kind &> /dev/null; then
      log_error "Kind not found. Please install Kind first."
      echo "  macOS: brew install kind"
      echo "  Linux: curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64"
      echo "         chmod +x ./kind && sudo mv ./kind /usr/local/bin/"
      exit 1
    fi
  fi

  if [ "$USE_MINIKUBE" = true ]; then
    if ! command -v minikube &> /dev/null; then
      log_error "Minikube not found. Please install Minikube first."
      echo "  macOS: brew install minikube"
      echo "  Linux: See https://minikube.sigs.k8s.io/docs/start/"
      exit 1
    fi
  fi

  log_info "Prerequisites check passed"
}

create_kind_cluster() {
  log_step "Creating Kind cluster: $CLUSTER_NAME"

  # Check if cluster already exists
  if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    log_warn "Kind cluster '$CLUSTER_NAME' already exists"
    read -p "Delete and recreate? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      log_info "Deleting existing cluster..."
      kind delete cluster --name "$CLUSTER_NAME"
    else
      log_info "Using existing cluster"
      return
    fi
  fi

  # Create Kind cluster config with extra port mappings
  cat > /tmp/kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  # Rabby Wallet
  - containerPort: 30173
    hostPort: 30173
    protocol: TCP
  # PIR Server
  - containerPort: 30000
    hostPort: 30000
    protocol: TCP
  # CDN Mock
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
  # Eth Mock (Anvil)
  - containerPort: 30545
    hostPort: 30545
    protocol: TCP
EOF

  log_info "Creating cluster with port mappings..."
  kind create cluster --name "$CLUSTER_NAME" --config /tmp/kind-config.yaml

  # Wait for cluster to be ready
  log_info "Waiting for cluster to be ready..."
  kubectl wait --for=condition=Ready nodes --all --timeout=300s

  log_info "Kind cluster created successfully"
}

create_minikube_cluster() {
  log_step "Creating Minikube cluster: $CLUSTER_NAME"

  # Check if cluster already exists
  if minikube profile list 2>/dev/null | grep -q "$CLUSTER_NAME"; then
    log_warn "Minikube cluster '$CLUSTER_NAME' already exists"
    read -p "Delete and recreate? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      log_info "Deleting existing cluster..."
      minikube delete -p "$CLUSTER_NAME"
    else
      log_info "Using existing cluster"
      minikube profile "$CLUSTER_NAME"
      return
    fi
  fi

  # Create Minikube cluster with sufficient resources
  log_info "Creating cluster (this may take a few minutes)..."
  minikube start \
    --profile "$CLUSTER_NAME" \
    --memory 8192 \
    --cpus 4 \
    --disk-size 20g \
    --driver docker

  log_info "Minikube cluster created successfully"
}

setup_cluster() {
  if [ "$SKIP_CLUSTER_CREATE" = true ]; then
    log_info "Skipping cluster creation (using existing cluster)"
    return
  fi

  if [ "$USE_KIND" = true ]; then
    create_kind_cluster
  elif [ "$USE_MINIKUBE" = true ]; then
    create_minikube_cluster
  fi
}

setup_ingress_controller() {
  log_step "Setting up Nginx Ingress Controller..."

  # For local testing, we use NodePort instead of Ingress
  # But we still set up the controller for completeness
  if [ "$USE_KIND" = true ]; then
    log_info "Installing Nginx Ingress for Kind..."
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

    log_info "Waiting for ingress controller to be ready..."
    kubectl wait --namespace ingress-nginx \
      --for=condition=ready pod \
      --selector=app.kubernetes.io/component=controller \
      --timeout=300s 2>/dev/null || log_warn "Ingress controller not ready (this is OK for NodePort mode)"
  elif [ "$USE_MINIKUBE" = true ]; then
    log_info "Enabling Ingress addon for Minikube..."
    minikube addons enable ingress -p "$CLUSTER_NAME"
  fi

  log_info "Ingress setup complete (optional for local NodePort mode)"
}

create_namespace() {
  log_step "Creating namespace: $NAMESPACE"

  if kubectl get namespace "$NAMESPACE" &> /dev/null; then
    log_warn "Namespace $NAMESPACE already exists"
  else
    kubectl create namespace "$NAMESPACE"
    log_info "Namespace created"
  fi
}

deploy_chart() {
  log_step "Deploying Plinko PIR Helm chart..."

  # Navigate to chart directory
  cd "$CHART_PATH"

  # Build helm command
  log_info "Using values file: $VALUES_FILE"

  # Deploy with helm
  helm upgrade --install "$RELEASE_NAME" . \
    --namespace "$NAMESPACE" \
    --create-namespace \
    --values "$VALUES_FILE" \
    --wait \
    --timeout 15m

  log_info "Deployment initiated"
}

wait_for_initialization() {
  log_step "Waiting for initialization jobs (this may take 10-15 minutes)..."

  # Wait for db-generator
  log_info "Waiting for db-generator job..."
  kubectl --namespace "$NAMESPACE" wait --for=condition=complete \
    --timeout=600s job/plinko-pir-db-generator 2>/dev/null || log_warn "db-generator job not found or timeout"

  # Wait for hint-generator
  log_info "Waiting for hint-generator job..."
  kubectl --namespace "$NAMESPACE" wait --for=condition=complete \
    --timeout=900s job/plinko-pir-hint-generator 2>/dev/null || log_warn "hint-generator job not found or timeout"

  log_info "Initialization jobs completed"
}

wait_for_pods() {
  log_step "Waiting for pods to be ready..."

  # Wait for all deployments
  kubectl --namespace "$NAMESPACE" wait --for=condition=available \
    --timeout=300s deployment --all 2>/dev/null || log_warn "Some deployments not ready"

  log_info "Pods are ready"
}

show_status() {
  log_step "Deployment status:"
  echo ""

  # Show pods
  echo "Pods:"
  kubectl --namespace "$NAMESPACE" get pods
  echo ""

  # Show services
  echo "Services:"
  kubectl --namespace "$NAMESPACE" get services
  echo ""

  # Show PVC
  echo "Persistent Volumes:"
  kubectl --namespace "$NAMESPACE" get pvc
}

show_access_info() {
  log_step "Access information:"
  echo ""

  cat <<EOF
${GREEN}Local Access URLs:${NC}
--------------------------------------------------
Rabby Wallet (UI):   http://localhost:30173
PIR Server API:      http://localhost:30000
CDN (hint/deltas):   http://localhost:30080
Anvil RPC:           http://localhost:30545
--------------------------------------------------

${YELLOW}Note:${NC} These URLs work directly with Kind (port-forwarded).
For Minikube, you may need to use: minikube service -n $NAMESPACE <service-name>

${GREEN}Quick test:${NC}
  # Check if services are responding
  curl http://localhost:30000/health
  curl http://localhost:30080/health

${GREEN}View logs:${NC}
  kubectl --namespace $NAMESPACE logs -f deployment/plinko-pir-pir-server
  kubectl --namespace $NAMESPACE logs -f deployment/plinko-pir-update-service
  kubectl --namespace $NAMESPACE logs -f deployment/plinko-pir-rabby-wallet

${GREEN}Run validation:${NC}
  cd $(dirname "$0")
  ./test-local.sh

${GREEN}Port forwarding (alternative):${NC}
  cd $(dirname "$0")
  ./port-forward.sh
EOF
}

cleanup_instructions() {
  echo ""
  log_step "Cleanup commands:"
  cat <<EOF
${YELLOW}To uninstall:${NC}
  helm uninstall $RELEASE_NAME --namespace $NAMESPACE

${YELLOW}To delete cluster:${NC}
EOF

  if [ "$USE_KIND" = true ]; then
    echo "  kind delete cluster --name $CLUSTER_NAME"
  elif [ "$USE_MINIKUBE" = true ]; then
    echo "  minikube delete -p $CLUSTER_NAME"
  fi

  echo ""
}

# Main execution
main() {
  echo "==========================================="
  echo "Plinko PIR - Local Kubernetes Deployment"
  echo "==========================================="
  echo ""

  if [ "$USE_KIND" = true ]; then
    log_info "Using Kind for local Kubernetes"
  elif [ "$USE_MINIKUBE" = true ]; then
    log_info "Using Minikube for local Kubernetes"
  fi

  echo ""

  check_prerequisites
  setup_cluster
  setup_ingress_controller
  create_namespace
  deploy_chart

  echo ""
  wait_for_initialization
  wait_for_pods

  echo ""
  show_status

  echo ""
  show_access_info

  cleanup_instructions

  echo "==========================================="
  log_info "Deployment complete!"
  echo "==========================================="
}

# Run main
main
