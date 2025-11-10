#!/bin/bash
set -e

echo "=========================================="
echo "Plinko PIR PoC - Full Reset"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Confirm reset
echo -e "${YELLOW}⚠️  WARNING: This will:${NC}"
echo "   - Stop all running services"
echo "   - Delete all Docker containers"
echo "   - Delete all Docker volumes (data will be lost)"
echo "   - Delete all generated data files"
echo "   - Rebuild all Docker images from scratch"
echo ""
read -p "Are you sure you want to continue? (yes/no): " -r
echo ""

if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Reset cancelled."
    exit 0
fi

echo -e "${BLUE}Step 1: Stopping services...${NC}"
docker-compose down 2>/dev/null || echo "  (No services running)"
echo -e "${GREEN}✓ Services stopped${NC}"
echo ""

echo -e "${BLUE}Step 2: Removing Docker volumes...${NC}"
docker-compose down -v 2>/dev/null || echo "  (No volumes to remove)"
echo -e "${GREEN}✓ Volumes removed${NC}"
echo ""

echo -e "${BLUE}Step 3: Cleaning generated data files...${NC}"
if [ -d "shared/data" ]; then
    rm -rf shared/data/*
    echo "  ✓ Removed shared/data/*"
fi
echo -e "${GREEN}✓ Data files cleaned${NC}"
echo ""

echo -e "${BLUE}Step 4: Rebuilding Docker images...${NC}"
echo -e "  ${YELLOW}This may take 5-10 minutes${NC}"
echo ""
docker-compose build --no-cache
echo ""
echo -e "${GREEN}✓ Docker images rebuilt${NC}"
echo ""

echo "=========================================="
echo -e "${GREEN}✅ RESET COMPLETE${NC}"
echo "=========================================="
echo ""
echo "Next steps:"
echo ""
echo "  1. Start the PoC:"
echo "     make start"
echo ""
echo "  2. View logs:"
echo "     make logs"
echo ""
echo "  3. Access the wallet:"
echo "     http://localhost:5173"
echo ""
echo "Run 'make help' to see all available commands."
echo ""
