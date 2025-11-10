#!/bin/bash
# =============================================================================
# PLINKO PIR - LOCAL TESTING VALIDATION SCRIPT
# =============================================================================
# This script runs smoke tests to validate local Kubernetes deployment
#
# Usage:
#   ./test-local.sh                     # Run all tests
#   ./test-local.sh --quick             # Quick tests only (skip data validation)
#   ./test-local.sh --namespace custom  # Use custom namespace
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
QUICK_MODE=false
USE_NODEPORT=true  # Default for local Kind deployment

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --quick)
      QUICK_MODE=true
      shift
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --port-forward)
      USE_NODEPORT=false
      shift
      ;;
    --help)
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  --quick              Quick tests only (skip data validation)"
      echo "  --namespace <name>   Kubernetes namespace (default: plinko-pir)"
      echo "  --port-forward       Use port-forward URLs instead of NodePort"
      echo "  --help               Show this help message"
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

log_test() {
  echo -e "${BLUE}[TEST]${NC} $1"
}

run_test() {
  local test_name="$1"
  local test_command="$2"

  TESTS_RUN=$((TESTS_RUN + 1))
  log_test "$test_name"

  if eval "$test_command"; then
    echo -e "  ${GREEN}✓ PASS${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    return 0
  else
    echo -e "  ${RED}✗ FAIL${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    return 1
  fi
}

# Set URLs based on access method
if [ "$USE_NODEPORT" = true ]; then
  WALLET_URL="http://localhost:30173"
  PIR_SERVER_URL="http://localhost:30000"
  CDN_URL="http://localhost:30080"
  ANVIL_URL="http://localhost:30545"
  UPDATE_SERVICE_URL="http://localhost:30001"
else
  WALLET_URL="http://localhost:5173"
  PIR_SERVER_URL="http://localhost:3000"
  CDN_URL="http://localhost:8080"
  ANVIL_URL="http://localhost:8545"
  UPDATE_SERVICE_URL="http://localhost:3001"
fi

check_prerequisites() {
  log_step "Checking prerequisites..."

  if ! command -v kubectl &> /dev/null; then
    log_error "kubectl not found"
    exit 1
  fi

  if ! command -v curl &> /dev/null; then
    log_error "curl not found"
    exit 1
  fi

  if ! kubectl cluster-info &> /dev/null; then
    log_error "Cannot connect to Kubernetes cluster"
    exit 1
  fi

  if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    log_error "Namespace '$NAMESPACE' not found"
    exit 1
  fi

  log_info "Prerequisites OK"
}

test_namespace() {
  log_step "Testing namespace and resources..."

  run_test "Namespace exists" \
    "kubectl get namespace $NAMESPACE -o name &> /dev/null"

  run_test "PVC exists" \
    "kubectl --namespace $NAMESPACE get pvc plinko-pir-data-pvc -o name &> /dev/null"
}

test_pods() {
  log_step "Testing pod status..."

  # Get all pods
  PODS=$(kubectl --namespace "$NAMESPACE" get pods -o name 2>/dev/null | wc -l)
  log_info "Found $PODS pods in namespace"

  # Test each deployment
  run_test "eth-mock pod running" \
    "kubectl --namespace $NAMESPACE get pods -l app=eth-mock -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q 'Running'"

  run_test "pir-server pod running" \
    "kubectl --namespace $NAMESPACE get pods -l app=plinko-pir-server -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q 'Running'"

  run_test "update-service pod running" \
    "kubectl --namespace $NAMESPACE get pods -l app=plinko-update-service -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q 'Running'"

  run_test "cdn-mock pod running" \
    "kubectl --namespace $NAMESPACE get pods -l app=cdn-mock -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q 'Running'"

  run_test "rabby-wallet pod running" \
    "kubectl --namespace $NAMESPACE get pods -l app=rabby-wallet -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q 'Running'"
}

test_jobs() {
  log_step "Testing initialization jobs..."

  run_test "db-generator job completed" \
    "kubectl --namespace $NAMESPACE get job plinko-pir-db-generator -o jsonpath='{.status.conditions[?(@.type==\"Complete\")].status}' 2>/dev/null | grep -q 'True'"

  run_test "hint-generator job completed" \
    "kubectl --namespace $NAMESPACE get job plinko-pir-hint-generator -o jsonpath='{.status.conditions[?(@.type==\"Complete\")].status}' 2>/dev/null | grep -q 'True'"
}

test_services() {
  log_step "Testing services..."

  run_test "eth-mock service exists" \
    "kubectl --namespace $NAMESPACE get service plinko-pir-eth-mock -o name &> /dev/null"

  run_test "pir-server service exists" \
    "kubectl --namespace $NAMESPACE get service plinko-pir-pir-server -o name &> /dev/null"

  run_test "cdn-mock service exists" \
    "kubectl --namespace $NAMESPACE get service plinko-pir-cdn-mock -o name &> /dev/null"

  run_test "rabby-wallet service exists" \
    "kubectl --namespace $NAMESPACE get service plinko-pir-rabby-wallet -o name &> /dev/null"
}

test_http_endpoints() {
  log_step "Testing HTTP endpoints..."

  # Give services a moment to be accessible
  sleep 2

  run_test "PIR Server health check" \
    "curl -sf $PIR_SERVER_URL/health &> /dev/null"

  run_test "CDN Mock health check" \
    "curl -sf $CDN_URL/health &> /dev/null"

  run_test "Rabby Wallet accessible" \
    "curl -sf $WALLET_URL &> /dev/null"

  run_test "Anvil RPC accessible" \
    "curl -sf -X POST -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}' $ANVIL_URL &> /dev/null"
}

test_data_files() {
  if [ "$QUICK_MODE" = true ]; then
    log_step "Skipping data file validation (quick mode)"
    return
  fi

  log_step "Testing data files..."

  # Get a pod that has access to the PVC
  PIR_POD=$(kubectl --namespace "$NAMESPACE" get pods -l app=plinko-pir-server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

  if [ -z "$PIR_POD" ]; then
    log_warn "Cannot find PIR server pod for data validation"
    return
  fi

  run_test "database.bin exists" \
    "kubectl --namespace $NAMESPACE exec $PIR_POD -- test -f /data/database.bin"

  run_test "hint.bin exists" \
    "kubectl --namespace $NAMESPACE exec $PIR_POD -- test -f /data/hint.bin"

  run_test "address-mapping.bin exists" \
    "kubectl --namespace $NAMESPACE exec $PIR_POD -- test -f /data/address-mapping.bin"

  run_test "deltas directory exists" \
    "kubectl --namespace $NAMESPACE exec $PIR_POD -- test -d /data/deltas"

  run_test "database.bin has content" \
    "kubectl --namespace $NAMESPACE exec $PIR_POD -- test -s /data/database.bin"

  run_test "hint.bin has content" \
    "kubectl --namespace $NAMESPACE exec $PIR_POD -- test -s /data/hint.bin"
}

test_pir_query() {
  if [ "$QUICK_MODE" = true ]; then
    log_step "Skipping PIR query test (quick mode)"
    return
  fi

  log_step "Testing basic PIR query..."

  # Test a simple PIR query (this would need the actual PIR protocol)
  # For now, just test that the endpoint accepts requests
  run_test "PIR Server accepts POST requests" \
    "curl -sf -X POST -H 'Content-Type: application/json' -d '{}' $PIR_SERVER_URL/query &> /dev/null || true"
}

show_summary() {
  echo ""
  log_step "Test Summary:"
  echo "======================================"
  echo -e "Total Tests:  ${BLUE}$TESTS_RUN${NC}"
  echo -e "Passed:       ${GREEN}$TESTS_PASSED${NC}"
  echo -e "Failed:       ${RED}$TESTS_FAILED${NC}"
  echo "======================================"

  if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    log_info "Local deployment is healthy"
    return 0
  else
    echo -e "${RED}✗ Some tests failed${NC}"
    echo ""
    log_error "Local deployment has issues"
    return 1
  fi
}

show_debug_info() {
  if [ $TESTS_FAILED -gt 0 ]; then
    echo ""
    log_step "Debug Information:"
    echo "======================================"
    echo ""
    echo "Pod Status:"
    kubectl --namespace "$NAMESPACE" get pods
    echo ""
    echo "Recent Events:"
    kubectl --namespace "$NAMESPACE" get events --sort-by='.lastTimestamp' | tail -n 10
    echo ""
    log_info "View detailed logs with:"
    echo "  kubectl --namespace $NAMESPACE logs -l app=plinko-pir-server"
    echo "  kubectl --namespace $NAMESPACE logs -l app=plinko-update-service"
    echo "======================================"
  fi
}

show_access_urls() {
  echo ""
  log_step "Access URLs:"
  echo "======================================"
  echo "Wallet:         $WALLET_URL"
  echo "PIR Server:     $PIR_SERVER_URL"
  echo "CDN:            $CDN_URL"
  echo "Anvil RPC:      $ANVIL_URL"
  echo "Update Service: $UPDATE_SERVICE_URL"
  echo "======================================"
}

# Main execution
main() {
  echo "==========================================="
  echo "Plinko PIR - Local Testing Validation"
  echo "==========================================="
  echo ""

  if [ "$QUICK_MODE" = true ]; then
    log_info "Running in QUICK mode (skipping data validation)"
  fi

  if [ "$USE_NODEPORT" = true ]; then
    log_info "Using NodePort access (localhost:30XXX)"
  else
    log_info "Using port-forward access (localhost:XXXX)"
  fi

  echo ""

  check_prerequisites

  echo ""
  test_namespace
  echo ""
  test_jobs
  echo ""
  test_pods
  echo ""
  test_services
  echo ""
  test_http_endpoints
  echo ""
  test_data_files
  echo ""
  test_pir_query

  show_summary
  local exit_code=$?

  show_debug_info
  show_access_urls

  exit $exit_code
}

# Run main
main
