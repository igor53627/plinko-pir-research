#!/usr/bin/env bash
# Build all Docker images locally and deploy to VKE
# Usage: ./deploy/scripts/build-local-and-deploy.sh

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Plinko PIR - Local Build & Deploy${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo ""

# Configuration
REGISTRY="ghcr.io/igor53627"
CACHE_DIR="/tmp/plinko-docker-cache"
mkdir -p "$CACHE_DIR"

# Check Docker login
echo -e "${BLUE}1. Checking Docker Registry authentication...${NC}"
if ! docker login ghcr.io --help > /dev/null 2>&1; then
  echo -e "${YELLOW}Please login to GitHub Container Registry:${NC}"
  echo "  docker login ghcr.io"
  echo "  Username: igor53627"
  echo "  Password: [Your GitHub Personal Access Token]"
  echo ""
  echo "Create token at: https://github.com/settings/tokens"
  echo "Required scopes: write:packages, read:packages"
  exit 1
fi

# Services to build
declare -A SERVICES=(
  ["db-generator"]="plinko-db-generator"
  ["plinko-hint-generator"]="plinko-hint-generator"
  ["plinko-update-service"]="plinko-update-service"
  ["plinko-pir-server"]="plinko-pir-server"
  ["cdn-mock"]="plinko-cdn-mock"
  ["rabby-wallet"]="plinko-rabby-wallet"
)

echo -e "${BLUE}2. Building Docker images...${NC}"
echo ""

for DIR in "${!SERVICES[@]}"; do
  NAME="${SERVICES[$DIR]}"
  echo -e "${BLUE}Building ${NAME}...${NC}"

  docker buildx build \
    --platform linux/amd64 \
    --cache-to "type=local,dest=${CACHE_DIR}/${NAME}" \
    --cache-from "type=local,src=${CACHE_DIR}/${NAME}" \
    --load \
    -t "${REGISTRY}/${NAME}:latest" \
    "./services/${DIR}"

  echo -e "${GREEN}✓ ${NAME} built${NC}"
  echo ""
done

echo -e "${BLUE}3. Pushing images to registry...${NC}"
echo ""

for DIR in "${!SERVICES[@]}"; do
  NAME="${SERVICES[$DIR]}"
  echo -e "${BLUE}Pushing ${NAME}...${NC}"

  docker push "${REGISTRY}/${NAME}:latest"

  echo -e "${GREEN}✓ ${NAME} pushed${NC}"
  echo ""
done

echo -e "${BLUE}4. Deploying to VKE cluster...${NC}"
echo ""

export KUBECONFIG=~/pse/k8s/pir-vke-cdedc8ce-ce47-4242-bbd7-16ef74a88a99.yaml

cd "$(dirname "$0")/../helm/plinko-pir"

helm upgrade plinko-pir . \
  -f values-vke-simple.yaml \
  -f values-ingress-single-ip.yaml \
  -n plinko-pir \
  --wait \
  --timeout 15m

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  Deployment Complete!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
echo ""

# Get Ingress IP
INGRESS_IP=$(kubectl get service -n ingress-nginx ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "pending")

echo -e "${GREEN}Access your services at:${NC}"
echo ""
echo "  Wallet UI:   http://${INGRESS_IP}/"
echo "  PIR Server:  http://${INGRESS_IP}/api"
echo "  CDN:         http://${INGRESS_IP}/cdn"
echo ""
echo -e "${BLUE}Verify deployment:${NC}"
echo "  kubectl get pods -n plinko-pir"
echo "  kubectl get ingress -n plinko-pir"
echo ""
