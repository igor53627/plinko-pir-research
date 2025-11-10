#!/bin/bash

# Test Service Addressing Configuration
# Validates that all services are accessible at the expected addresses

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

echo -e "${BLUE}=====================================${NC}"
echo -e "${BLUE}Service Addressing Configuration Test${NC}"
echo -e "${BLUE}=====================================${NC}"
echo ""

# Function to test HTTP endpoint
test_http_endpoint() {
    local name=$1
    local url=$2
    local expected_status=${3:-200}

    TESTS_TOTAL=$((TESTS_TOTAL + 1))

    echo -n "Testing $name at $url ... "

    if response=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "$url" 2>/dev/null); then
        if [ "$response" -eq "$expected_status" ]; then
            echo -e "${GREEN}✓ PASS${NC} (HTTP $response)"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            return 0
        else
            echo -e "${RED}✗ FAIL${NC} (Expected HTTP $expected_status, got HTTP $response)"
            TESTS_FAILED=$((TESTS_FAILED + 1))
            return 1
        fi
    else
        echo -e "${RED}✗ FAIL${NC} (Connection failed)"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Function to test Docker internal connectivity
test_docker_internal() {
    local container=$1
    local target_url=$2
    local description=$3

    TESTS_TOTAL=$((TESTS_TOTAL + 1))

    echo -n "Testing $description ... "

    if docker exec "$container" wget -q -O /dev/null --timeout=5 "$target_url" 2>/dev/null; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Check if services are running
echo -e "${YELLOW}Checking service status...${NC}"
if ! docker-compose ps | grep -q "Up"; then
    echo -e "${RED}ERROR: Services are not running!${NC}"
    echo "Please start services with: docker-compose up -d"
    exit 1
fi
echo ""

# Test 1: External Access (from host machine)
echo -e "${YELLOW}=== External Access Tests (localhost) ===${NC}"
test_http_endpoint "Wallet UI" "http://localhost:5173" "200"
test_http_endpoint "PIR Server Health" "http://localhost:3000/health" "200"
test_http_endpoint "CDN Health" "http://localhost:8080/health" "200"
test_http_endpoint "CDN Hint File" "http://localhost:8080/hint.bin" "200"
test_http_endpoint "CDN Deltas Directory" "http://localhost:8080/deltas/" "200"
echo ""

# Test 2: Docker Internal Communication
echo -e "${YELLOW}=== Docker Internal Communication Tests ===${NC}"

# Check if containers are running before testing
if docker ps | grep -q "plinko-pir-server"; then
    test_docker_internal "plinko-pir-server" "http://cdn-mock:8080/health" "PIR Server → CDN"
else
    echo -e "${YELLOW}⚠ SKIP${NC} PIR Server container not running"
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
fi

if docker ps | grep -q "plinko-pir-updates"; then
    test_docker_internal "plinko-pir-updates" "http://eth-mock:8545" "Update Service → Anvil"
else
    echo -e "${YELLOW}⚠ SKIP${NC} Update Service container not running"
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
fi

if docker ps | grep -q "plinko-pir-db-generator"; then
    test_docker_internal "plinko-pir-db-generator" "http://eth-mock:8545" "DB Generator → Anvil"
else
    echo -e "${YELLOW}⚠ SKIP${NC} DB Generator container completed (expected)"
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
fi

if docker ps | grep -q "plinko-wallet"; then
    # Wallet container runs nginx, can't use wget. Test from another container.
    echo -n "Testing Wallet accessibility from Docker network ... "
    if docker exec plinko-pir-server wget -q -O /dev/null --timeout=5 "http://ambire-wallet:80" 2>/dev/null; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
else
    echo -e "${YELLOW}⚠ SKIP${NC} Wallet container not running"
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
fi

echo ""

# Test 3: Network Configuration
echo -e "${YELLOW}=== Network Configuration Tests ===${NC}"

# Check if network exists
TESTS_TOTAL=$((TESTS_TOTAL + 1))
echo -n "Testing plinko-pir-network exists ... "
if docker network ls | grep -q "plinko-pir-network"; then
    echo -e "${GREEN}✓ PASS${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ FAIL${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Check if services are on the network
TESTS_TOTAL=$((TESTS_TOTAL + 1))
echo -n "Testing services connected to network ... "
connected_services=$(docker network inspect plinko-pir-network -f '{{range .Containers}}{{.Name}} {{end}}' 2>/dev/null | wc -w)
if [ "$connected_services" -gt 0 ]; then
    echo -e "${GREEN}✓ PASS${NC} ($connected_services services connected)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ FAIL${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo ""

# Test 4: Port Mappings
echo -e "${YELLOW}=== Port Mapping Tests ===${NC}"

check_port_mapping() {
    local service=$1
    local port=$2

    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo -n "Testing $service port mapping ($port) ... "

    if docker-compose port "$service" "$port" 2>/dev/null | grep -q "$port"; then
        echo -e "${GREEN}✓ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

check_port_mapping "plinko-pir-server" "3000"
check_port_mapping "cdn-mock" "8080"
check_port_mapping "ambire-wallet" "80"

# Check Anvil is NOT exposed
TESTS_TOTAL=$((TESTS_TOTAL + 1))
echo -n "Testing Anvil NOT exposed externally ... "
if ! docker-compose port eth-mock 8545 2>/dev/null | grep -q "8545"; then
    echo -e "${GREEN}✓ PASS${NC} (Correctly not exposed)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ FAIL${NC} (Should not be exposed)"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo ""

# Test 5: Environment Variables
echo -e "${YELLOW}=== Environment Variable Tests ===${NC}"

check_env_var() {
    local container=$1
    local var_name=$2
    local expected_value=$3

    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo -n "Testing $container has $var_name ... "

    if docker exec "$container" env 2>/dev/null | grep -q "^$var_name="; then
        actual_value=$(docker exec "$container" env 2>/dev/null | grep "^$var_name=" | cut -d'=' -f2-)
        if [ -n "$expected_value" ] && [ "$actual_value" != "$expected_value" ]; then
            echo -e "${YELLOW}⚠ WARNING${NC} (Expected: $expected_value, Got: $actual_value)"
        else
            echo -e "${GREEN}✓ PASS${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            return 0
        fi
    else
        echo -e "${RED}✗ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

if docker ps | grep -q "plinko-wallet"; then
    check_env_var "plinko-wallet" "VITE_PIR_SERVER_URL" ""
    check_env_var "plinko-wallet" "VITE_CDN_URL" ""
else
    echo -e "${YELLOW}⚠ SKIP${NC} Wallet container not running"
    TESTS_TOTAL=$((TESTS_TOTAL + 2))
fi

echo ""

# Summary
echo -e "${BLUE}=====================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}=====================================${NC}"
echo ""
echo "Total tests: $TESTS_TOTAL"
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "Service addressing is correctly configured:"
    echo "  • External access via localhost ports"
    echo "  • Internal communication via Docker network"
    echo "  • Proper network isolation"
    exit 0
else
    echo -e "${RED}✗ Some tests failed!${NC}"
    echo ""
    echo "Troubleshooting tips:"
    echo "  1. Check service status: docker-compose ps"
    echo "  2. View logs: docker-compose logs"
    echo "  3. Restart services: docker-compose restart"
    echo "  4. See docs/SERVICE_ADDRESSING.md for detailed help"
    exit 1
fi
