#!/bin/bash
set -e

echo "=========================================="
echo "Plinko PIR PoC - Privacy Verification Test"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
test_start() {
    echo -n "Testing: $1... "
    TESTS_RUN=$((TESTS_RUN + 1))
}

test_pass() {
    echo -e "${GREEN}✓ PASS${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

test_fail() {
    echo -e "${RED}✗ FAIL${NC}"
    if [ -n "$1" ]; then
        echo "  Reason: $1"
    fi
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

test_warn() {
    echo -e "${YELLOW}⚠ WARNING${NC}: $1"
}

# Test 1: Check all services are running
echo "1. SERVICE HEALTH CHECKS"
echo "------------------------"

test_start "Ethereum Mock (Anvil) health"
if curl -sf -X POST -H "Content-Type: application/json" \
   --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
   http://localhost:8545 > /dev/null 2>&1; then
    test_pass
else
    test_fail "Anvil not responding on port 8545"
fi

test_start "Plinko PIR Server health"
if curl -sf http://localhost:3000/health > /dev/null 2>&1; then
    test_pass
else
    test_fail "PIR Server not responding on port 3000"
fi

test_start "Plinko Update Service health"
if curl -sf http://localhost:3001/health > /dev/null 2>&1; then
    test_pass
else
    test_fail "Plinko service not responding on port 3001"
fi

test_start "CDN Mock health"
if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    test_pass
else
    test_fail "CDN not responding on port 8080"
fi

test_start "Wallet accessibility"
if curl -sf http://localhost:5173 > /dev/null 2>&1; then
    test_pass
else
    test_fail "Wallet not accessible on port 5173"
fi

echo ""

# Test 2: Privacy verification
echo "2. PRIVACY VERIFICATION"
echo "-----------------------"

test_start "PIR Server logs contain NO addresses"
# Check last 100 lines of PIR server logs for Ethereum addresses (0x...)
if docker logs piano-pir-server 2>&1 | tail -100 | grep -i "0x[0-9a-f]\{40\}" > /dev/null; then
    test_fail "PRIVACY LEAK DETECTED! Server logs contain Ethereum addresses"
else
    test_pass
fi

test_start "Plinko logs contain NO specific indices"
# Plinko should log aggregate stats, not individual indices
if docker logs piano-pir-plinko-updates 2>&1 | tail -100 | grep -E "index [0-9]+" > /dev/null; then
    test_warn "Plinko may be logging individual indices"
else
    test_pass
fi

echo ""

# Test 3: Data files exist
echo "3. DATA FILE VERIFICATION"
echo "-------------------------"

test_start "hint.bin exists and has correct size"
if [ -f "shared/data/hint.bin" ]; then
    SIZE=$(stat -f%z shared/data/hint.bin 2>/dev/null || stat -c%s shared/data/hint.bin 2>/dev/null)
    # Expected: ~67 MB (between 60-80 MB)
    if [ "$SIZE" -gt 60000000 ] && [ "$SIZE" -lt 80000000 ]; then
        test_pass
        echo "  Size: $(echo "scale=1; $SIZE / 1024 / 1024" | bc) MB"
    else
        test_fail "hint.bin size unexpected: $SIZE bytes"
    fi
else
    test_fail "hint.bin not found"
fi

test_start "database.bin exists and has correct size"
if [ -f "shared/data/database.bin" ]; then
    SIZE=$(stat -f%z shared/data/database.bin 2>/dev/null || stat -c%s shared/data/database.bin 2>/dev/null)
    EXPECTED=67108864  # 8,388,608 × 8
    if [ "$SIZE" -eq "$EXPECTED" ]; then
        test_pass
    else
        test_fail "Expected $EXPECTED bytes, got $SIZE bytes"
    fi
else
    test_fail "database.bin not found"
fi

test_start "Delta files are being generated"
DELTA_COUNT=$(ls -1 shared/data/deltas/delta-*.bin 2>/dev/null | wc -l | tr -d ' ')
if [ "$DELTA_COUNT" -gt 0 ]; then
    test_pass
    echo "  Found: $DELTA_COUNT delta files"
else
    test_warn "No delta files found yet (Plinko may still be initializing)"
fi

echo ""

# Test 4: Query functionality
echo "4. QUERY FUNCTIONALITY"
echo "----------------------"

test_start "Plaintext query to PIR server"
RESPONSE=$(curl -s -X POST http://localhost:3000/query/plaintext \
    -H "Content-Type: application/json" \
    -d '{"index": 42}')

if echo "$RESPONSE" | grep -q "value"; then
    test_pass
    QUERY_TIME=$(echo "$RESPONSE" | grep -o '"server_time_nanos":[0-9]*' | cut -d':' -f2)
    if [ -n "$QUERY_TIME" ]; then
        QUERY_MS=$(echo "scale=2; $QUERY_TIME / 1000000" | bc)
        echo "  Query time: ${QUERY_MS} ms"
    fi
else
    test_fail "Invalid response: $RESPONSE"
fi

test_start "FullSet query to PIR server"
RESPONSE=$(curl -s -X POST http://localhost:3000/query/fullset \
    -H "Content-Type: application/json" \
    -d '{"prf_key": [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]}')

if echo "$RESPONSE" | grep -q "value"; then
    test_pass
else
    test_fail "Invalid response: $RESPONSE"
fi

echo ""

# Test 5: CDN functionality
echo "5. CDN FUNCTIONALITY"
echo "--------------------"

test_start "hint.bin downloadable from CDN"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/hint.bin)
if [ "$HTTP_CODE" = "200" ]; then
    test_pass
else
    test_fail "HTTP $HTTP_CODE"
fi

test_start "CORS headers present"
CORS=$(curl -s -I http://localhost:8080/hint.bin | grep -i "access-control-allow-origin")
if echo "$CORS" | grep -q "\*"; then
    test_pass
else
    test_fail "CORS headers not found"
fi

test_start "Delta directory browsable"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/deltas/)
if [ "$HTTP_CODE" = "200" ]; then
    test_pass
else
    test_fail "HTTP $HTTP_CODE"
fi

echo ""

# Test 6: Fallback behavior
echo "6. FALLBACK BEHAVIOR"
echo "--------------------"

test_start "Public RPC fallback accessible"
# Test that Anvil can serve as public RPC
RESPONSE=$(curl -s -X POST http://localhost:8545 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0x1000000000000000000000000000000000000042","latest"],"id":1}')

if echo "$RESPONSE" | grep -q "result"; then
    test_pass
else
    test_fail "Public RPC not working"
fi

echo ""

# Summary
echo "=========================================="
echo "TEST SUMMARY"
echo "=========================================="
echo "Total tests run: $TESTS_RUN"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
fi
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
    echo ""
    echo "Privacy Verification: ✓ Server logs clean"
    echo "Query Functionality: ✓ Working"
    echo "Data Generation: ✓ Complete"
    echo ""
    exit 0
else
    echo -e "${RED}✗ SOME TESTS FAILED${NC}"
    echo ""
    echo "Please check the failures above and review service logs:"
    echo "  docker-compose logs piano-pir-server"
    echo "  docker-compose logs piano-pir-plinko-updates"
    echo ""
    exit 1
fi
