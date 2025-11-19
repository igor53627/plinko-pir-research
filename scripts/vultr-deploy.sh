#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Load local environment overrides if present (ignored by git)
if [[ -f "$ROOT_DIR/.env" ]]; then
  # shellcheck disable=SC1090
  set -a && source "$ROOT_DIR/.env" && set +a
fi
if [[ -f "$ROOT_DIR/.env.deploy" ]]; then
  # shellcheck disable=SC1090
  set -a && source "$ROOT_DIR/.env.deploy" && set +a
fi

API_KEY="${VULTR_API_KEY:-}"
TAG="${VULTR_TAG:-plinko-pir}"
SSH_KEY_PATH="${SSH_KEY:-}"
SSH_USER="${VULTR_SSH_USER:-root}"
REMOTE_DIR="${VULTR_REMOTE_DIR:-/opt/plinko-pir}"

REQUIRED_TOOLS=(curl ssh rsync python3)

usage() {
  cat <<'EOF'
Usage: scripts/vultr-deploy.sh <command> [args]

Commands
  info            Show instance metadata (tag-derived)
  ssh             Open SSH shell (root by default)
  bootstrap       Install Docker + rsync on the remote host
  sync            rsync repo (minus large/generated dirs) to remote host
  up              Sync + run `docker compose up -d --build`
  down            Run `docker compose down` on remote host
  logs            Tail remote `docker compose logs -f`

Environment
  VULTR_API_KEY       (required) API token used for instance lookup
  VULTR_TAG           Instance tag selector (default: plinko-pir)
  SSH_KEY             Path to SSH private key (default: empty -> required)
  VULTR_SSH_USER      Remote SSH username (default: root)
  VULTR_REMOTE_DIR    Target workspace directory (default: /opt/plinko-pir)

Examples
  ./scripts/vultr-deploy.sh info
  ./scripts/vultr-deploy.sh up
  ./scripts/vultr-deploy.sh logs
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_env() {
  local name=$1 value=${!1:-}
  [[ -n "$value" ]] || die "Environment variable $name is required"
}

check_tools() {
  for tool in "${REQUIRED_TOOLS[@]}"; do
    command -v "$tool" >/dev/null 2>&1 || die "Missing required tool: $tool"
  done
}

fetch_instance_summary() {
  require_env API_KEY

  local resp
  resp=$(curl -sS -H "Authorization: Bearer ${API_KEY}" \
    "https://api.vultr.com/v2/instances?tag=${TAG}")
  if [[ -z "$resp" ]]; then
    die "Empty response from Vultr API (check VULTR_API_KEY / network)"
  fi

  RESP_JSON="$resp" python3 - "$TAG" <<'PY' || return 1
import json, os, sys
tag = sys.argv[1]
data = json.loads(os.environ["RESP_JSON"])
instances = data.get("instances", [])
if not instances:
    sys.exit("No Vultr instances found for tag=%s" % tag)
inst = instances[0]
summary = {
    "id": inst.get("id"),
    "label": inst.get("label"),
    "ip": inst.get("main_ip"),
    "region": inst.get("region"),
    "plan": inst.get("plan"),
    "tags": ",".join(inst.get("tags", [])),
}
print(f"{summary['ip']}|{summary['id']}|{summary['label']}|{summary['region']}|{summary['plan']}|{summary['tags']}")
PY
}

ensure_instance() {
  if [[ -n "${INSTANCE_IP:-}" ]]; then
    return
  fi
  local summary
  summary=$(fetch_instance_summary)
  IFS='|' read -r INSTANCE_IP INSTANCE_ID INSTANCE_LABEL INSTANCE_REGION INSTANCE_PLAN INSTANCE_TAGS <<<"$summary"
  [[ -n "$INSTANCE_IP" ]] || die "Unable to resolve IP for tag ${TAG}"
}

remote_cmd() {
  [[ -n "${INSTANCE_IP:-}" ]] || ensure_instance
  require_env SSH_KEY_PATH
  ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    "${SSH_USER}@${INSTANCE_IP}" "$@"
}

remote_shell() {
  [[ -n "${INSTANCE_IP:-}" ]] || ensure_instance
  require_env SSH_KEY_PATH
  ssh -i "$SSH_KEY_PATH" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
    "${SSH_USER}@${INSTANCE_IP}"
}

bootstrap_remote() {
  remote_cmd "export DEBIAN_FRONTEND=noninteractive && \
sudo apt-get update && \
sudo apt-get install -y ca-certificates curl gnupg lsb-release rsync git && \
if ! command -v docker >/dev/null 2>&1; then \
  sudo install -m 0755 -d /etc/apt/keyrings && \
  curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg && \
  echo \"deb [arch=\$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \$(lsb_release -cs) stable\" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null && \
  sudo apt-get update && \
  sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin; \
fi && sudo usermod -aG docker ${SSH_USER}"
}

sync_repo() {
  ensure_instance
  remote_cmd "sudo mkdir -p ${REMOTE_DIR} ${REMOTE_DIR}/data ${REMOTE_DIR}/public-data && sudo chown -R ${SSH_USER}:${SSH_USER} ${REMOTE_DIR}"
  local rsync_excludes=(
    "--exclude=.git"
    "--exclude=.env"
    "--exclude=node_modules"
    "--exclude=public-data"
    "--exclude=raw_balances"
    "--exclude=test-data"
    "--exclude=services/rabby-wallet/node_modules"
    "--exclude=services/rabby-wallet/dist"
  )
  rsync -az --delete "${rsync_excludes[@]}" \
    -e "ssh -i ${SSH_KEY_PATH} -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null" \
    "$ROOT_DIR/" "${SSH_USER}@${INSTANCE_IP}:${REMOTE_DIR}/"
}

compose_run() {
  ensure_instance
  remote_cmd "cd ${REMOTE_DIR} && $*"
}

cmd=${1:-}
case "$cmd" in
  info)
    check_tools
    summary=$(fetch_instance_summary)
    IFS='|' read -r ip id label region plan tags <<<"$summary"
    printf "Tag:          %s\n" "$TAG"
    printf "Instance ID:  %s\n" "$id"
    printf "Label:        %s\n" "$label"
    printf "Region:       %s\n" "$region"
    printf "Plan:         %s\n" "$plan"
    printf "Tags:         %s\n" "${tags:-<none>}"
    printf "IPv4:         %s\n" "$ip"
    printf "SSH Command:  ssh -i %s %s@%s\n" "${SSH_KEY_PATH:-<unset>}" "$SSH_USER" "$ip"
    ;;
  ssh)
    check_tools
    remote_shell
    ;;
  bootstrap)
    check_tools
    ensure_instance
    echo "Bootstrapping remote host ${INSTANCE_IP}..."
    bootstrap_remote
    ;;
  sync)
    check_tools
    require_env SSH_KEY_PATH
    sync_repo
    ;;
  up)
    check_tools
    require_env SSH_KEY_PATH
    sync_repo
    compose_run "docker compose pull"
    # Ensure clean slate to avoid name conflicts
    compose_run "docker compose down --remove-orphans || true"
    # Aggressively remove potential zombie containers that block deployment
    # Names must match docker-compose.yml container_name fields exactly
    compose_run "docker rm -f plinko-pir-server plinko-pir-updates plinko-pir-cdn plinko-wallet plinko-state-syncer plinko-ipfs plinko-nginx-proxy || true"
    compose_run "docker compose up -d --build --force-recreate --remove-orphans"
    ;;
  down)
    check_tools
    require_env SSH_KEY_PATH
    compose_run "docker compose down"
    ;;
  logs)
    check_tools
    require_env SSH_KEY_PATH
    compose_run "docker compose logs -f"
    ;;
  ""|-h|--help|help)
    usage
    ;;
  *)
    usage
    die "Unknown command: $cmd"
    ;;
esac
