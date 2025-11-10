#!/bin/bash
# =============================================================================
# PLINKO PIR - DEPLOYMENT VERIFICATION SCRIPT
# =============================================================================
# This script verifies that the Plinko PIR deployment is working correctly
#
# Usage:
#   ./verify.sh                    # Verify default namespace
#   ./verify.sh --namespace custom # Verify custom namespace
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
VERBOSE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    --help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  --namespace <name>    Kubernetes namespace (default: plinko-pir)"
      echo "  --verbose             Show detailed output"
      echo "  --help                Show this help message"
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      exit 1
      ;;
  esac
done

# Test results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Functions
log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[PASS]${NC} $1"
  ((PASSED_TESTS++))
}

log_failure() {
  echo -e "${RED}[FAIL]${NC} $1"
  ((FAILED_TESTS++))
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

run_test() {
  ((TOTAL_TESTS++))
  TEST_NAME="$1"
  TEST_CMD="$2"

  if [ "$VERBOSE" = true ]; then
    log_info "Running: $TEST_NAME"
    log_info "Command: $TEST_CMD"
  fi

  if eval "$TEST_CMD" &> /dev/null; then
    log_success "$TEST_NAME"
    return 0
  else
    log_failure "$TEST_NAME"
    return 1
  fi
}

check_namespace() {
  log_info "Checking namespace: $NAMESPACE"

  if run_test "Namespace exists" "kubectl get namespace $NAMESPACE"; then
    return 0
  else
    log_failure "Namespace $NAMESPACE does not exist"
    exit 1
  fi
}

check_pvc() {
  log_info "Checking PersistentVolumeClaim..."

  run_test "PVC exists" "kubectl --namespace $NAMESPACE get pvc plinko-pir-data"

  if kubectl --namespace "$NAMESPACE" get pvc plinko-pir-data -o jsonpath='{.status.phase}' | grep -q "Bound"; then
    log_success "PVC is bound"
  else
    log_failure "PVC is not bound"
  fi
}

check_jobs() {
  log_info "Checking initialization jobs..."

  # Check db-generator
  if kubectl --namespace "$NAMESPACE" get job plinko-pir-db-generator &> /dev/null; then
    if kubectl --namespace "$NAMESPACE" get job plinko-pir-db-generator -o jsonpath='{.status.succeeded}' | grep -q "1"; then
      log_success "db-generator job completed"
    else
      log_failure "db-generator job not completed"
    fi
  else
    log_warn "db-generator job not found (may be disabled)"
  fi

  # Check hint-generator
  if kubectl --namespace "$NAMESPACE" get job plinko-pir-hint-generator &> /dev/null; then
    if kubectl --namespace "$NAMESPACE" get job plinko-pir-hint-generator -o jsonpath='{.status.succeeded}' | grep -q "1"; then
      log_success "hint-generator job completed"
    else
      log_failure "hint-generator job not completed"
    fi
  else
    log_warn "hint-generator job not found (may be disabled)"
  fi
}

check_deployments() {
  log_info "Checking deployments..."

  DEPLOYMENTS=(
    "plinko-pir-eth-mock"
    "plinko-pir-update-service"
    "plinko-pir-pir-server"
    "plinko-pir-cdn-mock"
    "plinko-pir-rabby-wallet"
  )

  for deployment in "${DEPLOYMENTS[@]}"; do
    if kubectl --namespace "$NAMESPACE" get deployment "$deployment" &> /dev/null; then
      READY=$(kubectl --namespace "$NAMESPACE" get deployment "$deployment" -o jsonpath='{.status.readyReplicas}' || echo 0)
      DESIRED=$(kubectl --namespace "$NAMESPACE" get deployment "$deployment" -o jsonpath='{.status.replicas}' || echo 0)

      if [ "$READY" = "$DESIRED" ] && [ "$READY" -gt 0 ]; then
        log_success "$deployment is ready ($READY/$DESIRED)"
      else
        log_failure "$deployment is not ready ($READY/$DESIRED)"
      fi
    else
      log_warn "$deployment not found"
    fi
  done
}

check_services() {
  log_info "Checking services..."

  SERVICES=(
    "plinko-pir-eth-mock:8545"
    "plinko-pir-update-service:3001"
    "plinko-pir-pir-server:3000"
    "plinko-pir-cdn-mock:8080"
    "plinko-pir-rabby-wallet:80"
  )

  for service_port in "${SERVICES[@]}"; do
    service=$(echo "$service_port" | cut -d':' -f1)
    port=$(echo "$service_port" | cut -d':' -f2)

    if kubectl --namespace "$NAMESPACE" get service "$service" &> /dev/null; then
      CLUSTER_IP=$(kubectl --namespace "$NAMESPACE" get service "$service" -o jsonpath='{.spec.clusterIP}')
      if [ -n "$CLUSTER_IP" ] && [ "$CLUSTER_IP" != "None" ]; then
        log_success "$service exists ($CLUSTER_IP:$port)"
      else
        log_failure "$service has no ClusterIP"
      fi
    else
      log_warn "$service not found"
    fi
  done
}

test_service_health() {
  log_info "Testing service health endpoints..."

  # Test PIR server
  if kubectl --namespace "$NAMESPACE" run test-pir-health --rm -it --restart=Never \
    --image=curlimages/curl --timeout=30s -- \
    curl -sf http://plinko-pir-pir-server:3000/health &> /dev/null; then
    log_success "PIR server health check passed"
  else
    log_failure "PIR server health check failed"
  fi

  # Test CDN
  if kubectl --namespace "$NAMESPACE" run test-cdn-health --rm -it --restart=Never \
    --image=curlimages/curl --timeout=30s -- \
    curl -sf http://plinko-pir-cdn-mock:8080/health &> /dev/null; then
    log_success "CDN health check passed"
  else
    log_failure "CDN health check failed"
  fi

  # Test wallet
  if kubectl --namespace "$NAMESPACE" run test-wallet-health --rm -it --restart=Never \
    --image=curlimages/curl --timeout=30s -- \
    curl -sf http://plinko-pir-rabby-wallet:80/ &> /dev/null; then
    log_success "Wallet health check passed"
  else
    log_failure "Wallet health check failed"
  fi
}

check_data_files() {
  log_info "Checking data files..."

  # Create temporary pod to inspect data
  POD_NAME="data-inspector-$(date +%s)"

  kubectl --namespace "$NAMESPACE" run "$POD_NAME" --image=busybox --restart=Never -- \
    sleep 300 &> /dev/null || true

  sleep 5

  # Check if pod is running
  if ! kubectl --namespace "$NAMESPACE" get pod "$POD_NAME" &> /dev/null; then
    log_warn "Cannot create data inspector pod"
    return
  fi

  # Check database.bin
  if kubectl --namespace "$NAMESPACE" exec "$POD_NAME" -- test -f /data/database.bin &> /dev/null; then
    SIZE=$(kubectl --namespace "$NAMESPACE" exec "$POD_NAME" -- stat -c%s /data/database.bin 2>/dev/null || echo "0")
    if [ "$SIZE" -gt 1000000 ]; then
      log_success "database.bin exists ($(($SIZE / 1024 / 1024)) MB)"
    else
      log_failure "database.bin is too small"
    fi
  else
    log_failure "database.bin not found"
  fi

  # Check hint.bin
  if kubectl --namespace "$NAMESPACE" exec "$POD_NAME" -- test -f /data/hint.bin &> /dev/null; then
    SIZE=$(kubectl --namespace "$NAMESPACE" exec "$POD_NAME" -- stat -c%s /data/hint.bin 2>/dev/null || echo "0")
    if [ "$SIZE" -gt 1000000 ]; then
      log_success "hint.bin exists ($(($SIZE / 1024 / 1024)) MB)"
    else
      log_failure "hint.bin is too small"
    fi
  else
    log_failure "hint.bin not found"
  fi

  # Check deltas directory
  if kubectl --namespace "$NAMESPACE" exec "$POD_NAME" -- test -d /data/deltas &> /dev/null; then
    COUNT=$(kubectl --namespace "$NAMESPACE" exec "$POD_NAME" -- ls /data/deltas | wc -l)
    log_success "deltas directory exists ($COUNT files)"
  else
    log_failure "deltas directory not found"
  fi

  # Cleanup
  kubectl --namespace "$NAMESPACE" delete pod "$POD_NAME" --force --grace-period=0 &> /dev/null || true
}

check_ingress() {
  log_info "Checking ingress..."

  if kubectl --namespace "$NAMESPACE" get ingress plinko-pir &> /dev/null; then
    log_success "Ingress exists"

    # Get ingress hosts
    HOSTS=$(kubectl --namespace "$NAMESPACE" get ingress plinko-pir -o jsonpath='{.spec.rules[*].host}')
    if [ -n "$HOSTS" ]; then
      log_info "Configured hosts: $HOSTS"
    fi

    # Check if address is assigned
    ADDRESS=$(kubectl --namespace "$NAMESPACE" get ingress plinko-pir -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    if [ -n "$ADDRESS" ]; then
      log_success "LoadBalancer IP assigned: $ADDRESS"
    else
      log_warn "LoadBalancer IP not assigned yet"
    fi
  else
    log_warn "Ingress not configured"
  fi
}

check_resource_usage() {
  log_info "Checking resource usage..."

  if command -v kubectl-top &> /dev/null || kubectl top nodes &> /dev/null; then
    echo ""
    echo "Pod resource usage:"
    kubectl --namespace "$NAMESPACE" top pods 2>/dev/null || log_warn "kubectl top not available"
  else
    log_warn "kubectl top not available (metrics-server may not be installed)"
  fi
}

show_summary() {
  echo ""
  echo "==========================================="
  echo "Verification Summary"
  echo "==========================================="
  echo "Total tests: $TOTAL_TESTS"
  echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
  echo -e "${RED}Failed: $FAILED_TESTS${NC}"
  echo ""

  if [ "$FAILED_TESTS" -eq 0 ]; then
    log_success "All tests passed!"
    echo ""
    echo "Your Plinko PIR deployment is healthy."
    echo ""
    echo "Next steps:"
    echo "  1. Access wallet UI via ingress or port-forward"
    echo "  2. Test privacy mode functionality"
    echo "  3. Monitor logs: kubectl --namespace $NAMESPACE logs -f deployment/plinko-pir-pir-server"
    return 0
  else
    log_failure "$FAILED_TESTS test(s) failed"
    echo ""
    echo "Troubleshooting:"
    echo "  1. Check pod logs: kubectl --namespace $NAMESPACE logs <pod-name>"
    echo "  2. Describe pods: kubectl --namespace $NAMESPACE describe pod <pod-name>"
    echo "  3. Check events: kubectl --namespace $NAMESPACE get events --sort-by='.lastTimestamp'"
    return 1
  fi
}

# Main execution
main() {
  echo "==========================================="
  echo "Plinko PIR - Deployment Verification"
  echo "==========================================="
  echo ""

  check_namespace
  check_pvc
  check_jobs
  check_deployments
  check_services
  test_service_health
  check_data_files
  check_ingress
  check_resource_usage

  show_summary
}

# Run main
main
EXIT_CODE=$?
echo "==========================================="
exit $EXIT_CODE
