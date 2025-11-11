#!/bin/sh
# Simple build script compatible with all shells
# Usage: ./deploy/scripts/build-simple.sh

set -e

echo "=================================="
echo "  Building Plinko PIR Images"
echo "=================================="
echo ""

REGISTRY="ghcr.io/igor53627"

# Build each service
echo "1/6 Building db-generator..."
docker build --platform linux/amd64 -t ${REGISTRY}/plinko-db-generator:latest ./services/db-generator

echo "2/6 Building hint-generator..."
docker build --platform linux/amd64 -t ${REGISTRY}/plinko-hint-generator:latest ./services/plinko-hint-generator

echo "3/6 Building update-service..."
docker build --platform linux/amd64 -t ${REGISTRY}/plinko-update-service:latest ./services/plinko-update-service

echo "4/6 Building pir-server..."
docker build --platform linux/amd64 -t ${REGISTRY}/plinko-pir-server:latest ./services/plinko-pir-server

echo "5/6 Building cdn-mock..."
docker build --platform linux/amd64 -t ${REGISTRY}/plinko-cdn-mock:latest ./services/cdn-mock

echo "6/6 Building rabby-wallet..."
docker build --platform linux/amd64 -t ${REGISTRY}/plinko-rabby-wallet:latest ./services/rabby-wallet

echo ""
echo "=================================="
echo "  Pushing to Registry"
echo "=================================="
echo ""

docker push ${REGISTRY}/plinko-db-generator:latest
docker push ${REGISTRY}/plinko-hint-generator:latest
docker push ${REGISTRY}/plinko-update-service:latest
docker push ${REGISTRY}/plinko-pir-server:latest
docker push ${REGISTRY}/plinko-cdn-mock:latest
docker push ${REGISTRY}/plinko-rabby-wallet:latest

echo ""
echo "✓ All images built and pushed!"
echo ""
echo "Now deploy with:"
echo "  KUBECONFIG=~/pse/k8s/pir-vke-*.yaml helm upgrade plinko-pir ./deploy/helm/plinko-pir \\"
echo "    -f values-vke-simple.yaml \\"
echo "    -f values-ingress-single-ip.yaml \\"
echo "    -n plinko-pir --wait --timeout 15m"
