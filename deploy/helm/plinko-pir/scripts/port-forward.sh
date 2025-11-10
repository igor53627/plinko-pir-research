#!/bin/bash
# =============================================================================
# PLINKO PIR - PORT FORWARDING HELPER
# =============================================================================
# This script sets up port forwarding for local Kubernetes access
# Alternative to NodePort when direct access is not available
#
# Usage:
#   ./port-forward.sh              # Start all port forwards
#   ./port-forward.sh --stop       # Stop all port forwards
#   ./port-forward.sh --status     # Show running port forwards
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
PID_DIR="/tmp/plinko-pir-port-forwards"

# Port mappings: LOCAL_PORT:SERVICE_NAME:SERVICE_PORT
PORT_FORWARDS=(
  "5173:plinko-pir-rabby-wallet:80"
  "3000:plinko-pir-pir-server:3000"
  "8080:plinko-pir-cdn-mock:8080"
  "8545:plinko-pir-eth-mock:8545"
  "3001:plinko-pir-update-service:3001"
)

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

# Create PID directory
mkdir -p "$PID_DIR"

check_kubectl() {
  if ! command -v kubectl &> /dev/null; then
    log_error "kubectl not found. Please install kubectl first."
    exit 1
  fi

  if ! kubectl cluster-info &> /dev/null; then
    log_error "Cannot connect to Kubernetes cluster."
    exit 1
  fi
}

check_namespace() {
  if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    log_error "Namespace '$NAMESPACE' not found. Is Plinko PIR deployed?"
    exit 1
  fi
}

start_port_forward() {
  local local_port=$1
  local service_name=$2
  local service_port=$3
  local pid_file="$PID_DIR/${service_name}.pid"

  # Check if already running
  if [ -f "$pid_file" ]; then
    local old_pid=$(cat "$pid_file")
    if ps -p "$old_pid" > /dev/null 2>&1; then
      log_warn "Port forward already running for $service_name (PID: $old_pid)"
      return
    else
      # Stale PID file, remove it
      rm -f "$pid_file"
    fi
  fi

  # Check if service exists
  if ! kubectl --namespace "$NAMESPACE" get service "$service_name" &> /dev/null; then
    log_warn "Service $service_name not found, skipping"
    return
  fi

  # Start port forward in background
  log_info "Starting port forward: localhost:$local_port -> $service_name:$service_port"
  kubectl --namespace "$NAMESPACE" port-forward "service/$service_name" "$local_port:$service_port" \
    > "$PID_DIR/${service_name}.log" 2>&1 &

  local pid=$!
  echo "$pid" > "$pid_file"

  # Wait a moment to check if it started successfully
  sleep 2
  if ps -p "$pid" > /dev/null 2>&1; then
    log_info "Port forward started successfully (PID: $pid)"
  else
    log_error "Port forward failed to start for $service_name"
    rm -f "$pid_file"
  fi
}

stop_port_forwards() {
  log_step "Stopping all port forwards..."

  local count=0
  for pid_file in "$PID_DIR"/*.pid; do
    if [ -f "$pid_file" ]; then
      local pid=$(cat "$pid_file")
      local service_name=$(basename "$pid_file" .pid)

      if ps -p "$pid" > /dev/null 2>&1; then
        log_info "Stopping port forward for $service_name (PID: $pid)"
        kill "$pid" 2>/dev/null || true
        count=$((count + 1))
      fi

      rm -f "$pid_file"
    fi
  done

  if [ $count -eq 0 ]; then
    log_warn "No port forwards were running"
  else
    log_info "Stopped $count port forward(s)"
  fi

  # Clean up log files
  rm -f "$PID_DIR"/*.log
}

show_status() {
  log_step "Port forward status:"
  echo ""

  local running_count=0

  for pid_file in "$PID_DIR"/*.pid; do
    if [ -f "$pid_file" ]; then
      local pid=$(cat "$pid_file")
      local service_name=$(basename "$pid_file" .pid)

      if ps -p "$pid" > /dev/null 2>&1; then
        echo -e "${GREEN}●${NC} $service_name (PID: $pid) - RUNNING"
        running_count=$((running_count + 1))
      else
        echo -e "${RED}●${NC} $service_name (PID: $pid) - NOT RUNNING (stale)"
        rm -f "$pid_file"
      fi
    fi
  done

  if [ $running_count -eq 0 ]; then
    echo "No port forwards running"
  fi

  echo ""
}

show_access_info() {
  cat <<EOF
${GREEN}Access URLs (via port-forward):${NC}
--------------------------------------------------
Rabby Wallet (UI):   http://localhost:5173
PIR Server API:      http://localhost:3000
CDN (hint/deltas):   http://localhost:8080
Anvil RPC:           http://localhost:8545
Update Service:      http://localhost:3001
--------------------------------------------------

${YELLOW}Quick test:${NC}
  curl http://localhost:3000/health
  curl http://localhost:8080/health
  open http://localhost:5173

${YELLOW}View logs:${NC}
  tail -f $PID_DIR/*.log

${YELLOW}Stop port forwards:${NC}
  $0 --stop
EOF
}

start_all() {
  log_step "Starting all port forwards..."
  echo ""

  check_kubectl
  check_namespace

  for mapping in "${PORT_FORWARDS[@]}"; do
    IFS=':' read -r local_port service_name service_port <<< "$mapping"
    start_port_forward "$local_port" "$service_name" "$service_port"
  done

  echo ""
  show_access_info
  echo ""
  log_info "Port forwards are running in the background"
  log_info "Press Ctrl+C or run '$0 --stop' to stop them"
}

show_help() {
  cat <<EOF
Usage: $0 [options]

Options:
  (no args)    Start all port forwards
  --stop       Stop all port forwards
  --status     Show status of port forwards
  --help       Show this help message

Port Mappings:
EOF
  for mapping in "${PORT_FORWARDS[@]}"; do
    IFS=':' read -r local_port service_name service_port <<< "$mapping"
    echo "  localhost:$local_port -> $service_name:$service_port"
  done
  echo ""
}

# Parse arguments
case "${1:-}" in
  --stop)
    stop_port_forwards
    ;;
  --status)
    show_status
    ;;
  --help)
    show_help
    ;;
  "")
    start_all
    ;;
  *)
    log_error "Unknown option: $1"
    show_help
    exit 1
    ;;
esac
