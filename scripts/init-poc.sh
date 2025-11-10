#!/bin/bash
set -e

echo "=========================================="
echo "Plinko PIR PoC - First-Time Initialization"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Step 1: Check prerequisites
echo -e "${BLUE}Step 1: Checking prerequisites...${NC}"

# Check Docker
if command -v docker &> /dev/null; then
    echo "  ✓ Docker found: $(docker --version | head -1)"
else
    echo "  ✗ Docker not found. Please install Docker Desktop."
    exit 1
fi

# Check Docker Compose
if docker-compose version &> /dev/null; then
    echo "  ✓ Docker Compose found: $(docker-compose version --short)"
else
    echo "  ✗ Docker Compose not found. Please install Docker Compose."
    exit 1
fi

# Check available RAM
if command -v free &> /dev/null; then
    TOTAL_RAM=$(free -g | awk '/Mem:/ {print $2}')
    if [ "$TOTAL_RAM" -lt 16 ]; then
        echo -e "  ${YELLOW}⚠ Warning: Only ${TOTAL_RAM}GB RAM detected. Recommended: 16GB+${NC}"
    else
        echo "  ✓ RAM: ${TOTAL_RAM}GB (sufficient)"
    fi
elif command -v vm_stat &> /dev/null; then
    # macOS
    TOTAL_RAM=$(sysctl -n hw.memsize | awk '{print int($1/1024/1024/1024)}')
    if [ "$TOTAL_RAM" -lt 16 ]; then
        echo -e "  ${YELLOW}⚠ Warning: Only ${TOTAL_RAM}GB RAM detected. Recommended: 16GB+${NC}"
    else
        echo "  ✓ RAM: ${TOTAL_RAM}GB (sufficient)"
    fi
fi

# Check disk space
AVAILABLE_SPACE=$(df -h . | tail -1 | awk '{print $4}' | sed 's/G//')
if [ "${AVAILABLE_SPACE%.*}" -lt 10 ]; then
    echo -e "  ${YELLOW}⚠ Warning: Only ${AVAILABLE_SPACE} available. Recommended: 10GB+${NC}"
else
    echo "  ✓ Disk space: ${AVAILABLE_SPACE} available"
fi

echo ""

# Step 2: Create .env file
echo -e "${BLUE}Step 2: Creating environment file...${NC}"

if [ -f ".env" ]; then
    echo -e "  ${YELLOW}⚠ .env file already exists, skipping${NC}"
else
    if [ -f ".env.example" ]; then
        cp .env.example .env
        echo -e "  ${GREEN}✓ Created .env from .env.example${NC}"
    else
        echo "  Creating default .env file..."
        cat > .env << 'EOF'
# Plinko PIR PoC Environment Configuration

# Service URLs (localhost for PoC)
VITE_PIR_SERVER_URL=http://localhost:3000
VITE_CDN_URL=http://localhost:8080
VITE_FALLBACK_RPC=https://eth.llamarpc.com

# Database Configuration
DB_SIZE=8388608

# Performance Tuning
CACHE_MODE_ENABLED=true
EOF
        echo -e "  ${GREEN}✓ Created default .env file${NC}"
    fi
fi

echo ""

# Step 3: Create shared data directories
echo -e "${BLUE}Step 3: Creating data directories...${NC}"

mkdir -p shared/data/deltas
echo "  ✓ Created shared/data/"
echo "  ✓ Created shared/data/deltas/"

echo ""

# Step 4: Build Docker images
echo -e "${BLUE}Step 4: Building Docker images...${NC}"
echo -e "  ${YELLOW}This may take 5-10 minutes on first run${NC}"
echo ""

docker-compose build

echo ""
echo -e "${GREEN}✓ Docker images built successfully${NC}"
echo ""

# Step 5: Display next steps
echo "=========================================="
echo -e "${GREEN}✅ INITIALIZATION COMPLETE${NC}"
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
echo "  4. Run tests:"
echo "     make test"
echo ""
echo "Useful commands:"
echo "  make help     - Show all available commands"
echo "  make status   - Check service status"
echo "  make health   - Verify service health"
echo "  make stop     - Stop all services"
echo "  make reset    - Clean and rebuild everything"
echo ""
echo "Documentation:"
echo "  See README.md for full documentation"
echo ""
