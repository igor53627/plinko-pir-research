#!/bin/bash
# =============================================================================
# PLINKO PIR - VULTR KUBERNETES ENGINE DEPLOYMENT SCRIPT
# =============================================================================
# This script automates deployment of Plinko PIR to Vultr VKE
#
# Usage:
#   ./deploy.sh                    # Deploy with default values
#   ./deploy.sh --production       # Deploy with production values
#   ./deploy.sh --custom values.yaml  # Deploy with custom values
#
# Prerequisites:
#   - kubectl configured with Vultr VKE cluster
#   - helm 3.8+ installed
#   - nginx ingress controller installed
#   - (optional) cert-manager for TLS
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="plinko-pir"
RELEASE_NAME="plinko-pir"
CHART_PATH="."
VALUES_FILE=""
PRODUCTION_MODE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --production)
      PRODUCTION_MODE=true
      VALUES_FILE="values-vultr.yaml"
      shift
      ;;
    --custom)
      VALUES_FILE="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --release)
      RELEASE_NAME="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  --production          Use production values (values-vultr.yaml)"
      echo "  --custom <file>       Use custom values file"
      echo "  --namespace <name>    Kubernetes namespace (default: plinko-pir)"
      echo "  --release <name>      Helm release name (default: plinko-pir)"
      echo "  --help                Show this help message"
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

check_prerequisites() {
  log_info "Checking prerequisites..."

  # Check kubectl
  if ! command -v kubectl &> /dev/null; then
    log_error "kubectl not found. Please install kubectl first."
    exit 1
  fi

  # Check helm
  if ! command -v helm &> /dev/null; then
    log_error "helm not found. Please install helm first."
    exit 1
  fi

  # Check kubectl connection
  if ! kubectl cluster-info &> /dev/null; then
    log_error "Cannot connect to Kubernetes cluster. Please configure kubectl."
    exit 1
  fi

  # Check helm version
  HELM_VERSION=$(helm version --short | cut -d'v' -f2 | cut -d'.' -f1-2)
  if (( $(echo "$HELM_VERSION < 3.8" | bc -l) )); then
    log_warn "Helm version $HELM_VERSION is older than recommended 3.8+"
  fi

  log_info "Prerequisites check passed"
}

check_ingress() {
  log_info "Checking for ingress controller..."

  if kubectl get namespace ingress-nginx &> /dev/null; then
    log_info "Nginx ingress controller found"
  else
    log_warn "Nginx ingress controller not found"
    log_info "Install with: helm install nginx-ingress ingress-nginx/ingress-nginx --namespace ingress-nginx --create-namespace"
  fi
}

check_cert_manager() {
  log_info "Checking for cert-manager (TLS)..."

  if kubectl get namespace cert-manager &> /dev/null; then
    log_info "cert-manager found"
  else
    log_warn "cert-manager not found (TLS will be disabled)"
    log_info "Install with: helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set installCRDs=true"
  fi
}

create_namespace() {
  log_info "Creating namespace: $NAMESPACE"

  if kubectl get namespace "$NAMESPACE" &> /dev/null; then
    log_warn "Namespace $NAMESPACE already exists"
  else
    kubectl create namespace "$NAMESPACE"
    log_info "Namespace created"
  fi
}

deploy_chart() {
  log_info "Deploying Plinko PIR Helm chart..."

  # Build helm command
  HELM_CMD="helm upgrade --install $RELEASE_NAME $CHART_PATH"
  HELM_CMD="$HELM_CMD --namespace $NAMESPACE"
  HELM_CMD="$HELM_CMD --create-namespace"

  # Add values file if specified
  if [ -n "$VALUES_FILE" ]; then
    if [ -f "$VALUES_FILE" ]; then
      HELM_CMD="$HELM_CMD --values $VALUES_FILE"
      log_info "Using values file: $VALUES_FILE"
    else
      log_error "Values file not found: $VALUES_FILE"
      exit 1
    fi
  fi

  # Add production-specific settings
  if [ "$PRODUCTION_MODE" = true ]; then
    log_info "Production mode enabled"
  fi

  # Execute helm command
  log_info "Executing: $HELM_CMD"
  eval "$HELM_CMD"

  log_info "Deployment initiated"
}

wait_for_jobs() {
  log_info "Waiting for initialization jobs to complete..."

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
  log_info "Waiting for pods to be ready..."

  # Wait for all deployments
  kubectl --namespace "$NAMESPACE" wait --for=condition=available \
    --timeout=300s deployment --all 2>/dev/null || log_warn "Some deployments not ready"

  log_info "Pods are ready"
}

show_status() {
  log_info "Deployment status:"
  echo ""

  # Show pods
  echo "Pods:"
  kubectl --namespace "$NAMESPACE" get pods

  echo ""
  echo "Services:"
  kubectl --namespace "$NAMESPACE" get services

  echo ""
  echo "Ingress:"
  kubectl --namespace "$NAMESPACE" get ingress

  echo ""
  echo "PVC:"
  kubectl --namespace "$NAMESPACE" get pvc
}

show_access_info() {
  log_info "Access information:"
  echo ""

  # Get LoadBalancer IP
  LB_IP=$(kubectl --namespace ingress-nginx get service nginx-ingress-ingress-nginx-controller \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "Not available")

  echo "LoadBalancer IP: $LB_IP"
  echo ""

  if [ "$LB_IP" != "Not available" ]; then
    # Get ingress hosts
    INGRESS_HOSTS=$(kubectl --namespace "$NAMESPACE" get ingress plinko-pir \
      -o jsonpath='{.spec.rules[*].host}' 2>/dev/null || echo "")

    if [ -n "$INGRESS_HOSTS" ]; then
      echo "Configure DNS A records:"
      for host in $INGRESS_HOSTS; do
        echo "  $host -> $LB_IP"
      done
      echo ""
      echo "Access URLs:"
      for host in $INGRESS_HOSTS; do
        if [[ $host == *"wallet"* ]]; then
          echo "  Wallet UI: https://$host"
        elif [[ $host == *"api"* ]]; then
          echo "  PIR API: https://$host"
        elif [[ $host == *"cdn"* ]]; then
          echo "  CDN: https://$host"
        fi
      done
    else
      echo "Ingress not configured. Use port-forward for testing:"
      echo "  kubectl --namespace $NAMESPACE port-forward service/plinko-pir-rabby-wallet 5173:80"
      echo "  Then access: http://localhost:5173"
    fi
  else
    log_warn "LoadBalancer IP not available yet. Run 'kubectl --namespace ingress-nginx get service' to check."
  fi

  echo ""
  log_info "Deployment complete!"
}

# Main execution
main() {
  echo "==========================================="
  echo "Plinko PIR - Vultr VKE Deployment"
  echo "==========================================="
  echo ""

  check_prerequisites
  check_ingress
  check_cert_manager
  create_namespace
  deploy_chart

  echo ""
  log_info "Waiting for initialization (this may take 10-15 minutes)..."
  wait_for_jobs
  wait_for_pods

  echo ""
  show_status

  echo ""
  show_access_info

  echo ""
  echo "==========================================="
  log_info "Next steps:"
  echo "1. Configure DNS records (if using custom domain)"
  echo "2. Verify deployment: ./verify.sh"
  echo "3. View logs: kubectl --namespace $NAMESPACE logs -f deployment/plinko-pir-pir-server"
  echo "4. Monitor: kubectl --namespace $NAMESPACE get pods -w"
  echo "==========================================="
}

# Run main
main
