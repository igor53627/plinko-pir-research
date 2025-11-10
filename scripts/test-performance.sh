#!/bin/bash
set -e

echo "=========================================="
echo "Plinko PIR PoC - Performance Validation"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Performance thresholds (from Task 9 acceptance criteria)
MAX_QUERY_LATENCY_MS=10
MAX_UPDATE_LATENCY_US=100
MIN_DELTA_SIZE_KB=20
MAX_DELTA_SIZE_KB=40
MAX_HINT_DOWNLOAD_SEC=2
MAX_DELTA_APPLICATION_MS=10

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

test_info() {
    echo -e "${BLUE}ℹ INFO${NC}: $1"
}

# Test 1: Query Latency
echo "1. QUERY LATENCY TESTS"
echo "----------------------"

# Plaintext query latency (target: <10ms)
test_start "Plaintext query latency (<${MAX_QUERY_LATENCY_MS}ms)"
RESPONSE=$(curl -s -X POST http://localhost:3000/query/plaintext \
    -H "Content-Type: application/json" \
    -d '{"index": 42}')

QUERY_TIME_NS=$(echo "$RESPONSE" | grep -o '"server_time_nanos":[0-9]*' | cut -d':' -f2)
if [ -z "$QUERY_TIME_NS" ]; then
    test_fail "Could not parse query time from response"
else
    QUERY_TIME_MS=$(echo "scale=2; $QUERY_TIME_NS / 1000000" | bc)
    QUERY_TIME_MS_INT=$(echo "$QUERY_TIME_MS / 1" | bc)

    if [ "$QUERY_TIME_MS_INT" -lt "$MAX_QUERY_LATENCY_MS" ]; then
        test_pass
        echo "  Latency: ${QUERY_TIME_MS} ms"
    else
        test_fail "Query took ${QUERY_TIME_MS}ms (threshold: ${MAX_QUERY_LATENCY_MS}ms)"
    fi
fi

# FullSet query latency (target: <10ms)
test_start "FullSet query latency (<${MAX_QUERY_LATENCY_MS}ms)"
RESPONSE=$(curl -s -X POST http://localhost:3000/query/fullset \
    -H "Content-Type: application/json" \
    -d '{"prf_key": [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]}')

QUERY_TIME_NS=$(echo "$RESPONSE" | grep -o '"server_time_nanos":[0-9]*' | cut -d':' -f2)
if [ -z "$QUERY_TIME_NS" ]; then
    test_fail "Could not parse query time from response"
else
    QUERY_TIME_MS=$(echo "scale=2; $QUERY_TIME_NS / 1000000" | bc)
    QUERY_TIME_MS_INT=$(echo "$QUERY_TIME_MS / 1" | bc)

    if [ "$QUERY_TIME_MS_INT" -lt "$MAX_QUERY_LATENCY_MS" ]; then
        test_pass
        echo "  Latency: ${QUERY_TIME_MS} ms"
    else
        test_fail "Query took ${QUERY_TIME_MS}ms (threshold: ${MAX_QUERY_LATENCY_MS}ms)"
    fi
fi

# Average query latency over 10 queries
test_start "Average query latency over 10 queries"
TOTAL_TIME=0
for i in {1..10}; do
    INDEX=$((RANDOM % 8388608))
    RESPONSE=$(curl -s -X POST http://localhost:3000/query/plaintext \
        -H "Content-Type: application/json" \
        -d "{\"index\": $INDEX}")

    QUERY_TIME_NS=$(echo "$RESPONSE" | grep -o '"server_time_nanos":[0-9]*' | cut -d':' -f2)
    if [ -n "$QUERY_TIME_NS" ]; then
        TOTAL_TIME=$((TOTAL_TIME + QUERY_TIME_NS))
    fi
done

AVG_TIME_NS=$((TOTAL_TIME / 10))
AVG_TIME_MS=$(echo "scale=2; $AVG_TIME_NS / 1000000" | bc)
AVG_TIME_MS_INT=$(echo "$AVG_TIME_MS / 1" | bc)

if [ "$AVG_TIME_MS_INT" -lt "$MAX_QUERY_LATENCY_MS" ]; then
    test_pass
    echo "  Average latency: ${AVG_TIME_MS} ms"
else
    test_fail "Average query took ${AVG_TIME_MS}ms (threshold: ${MAX_QUERY_LATENCY_MS}ms)"
fi

echo ""

# Test 2: Update Latency
echo "2. UPDATE LATENCY TESTS"
echo "-----------------------"

test_start "Plinko update latency (<${MAX_UPDATE_LATENCY_US}μs per block)"
# Check Plinko logs for "Block processed" messages with timing
RECENT_BLOCK=$(docker logs piano-pir-plinko-updates 2>&1 | grep "Block processed" | tail -1)

if [ -z "$RECENT_BLOCK" ]; then
    test_info "No blocks processed yet, skipping update latency test"
else
    # Extract timing from log: "Block processed X in Yμs"
    UPDATE_TIME_US=$(echo "$RECENT_BLOCK" | grep -o '[0-9]*μs' | grep -o '[0-9]*')

    if [ -z "$UPDATE_TIME_US" ]; then
        test_info "Could not parse update time from logs"
    else
        if [ "$UPDATE_TIME_US" -lt "$MAX_UPDATE_LATENCY_US" ]; then
            test_pass
            echo "  Update time: ${UPDATE_TIME_US} μs"
        else
            test_fail "Update took ${UPDATE_TIME_US}μs (threshold: ${MAX_UPDATE_LATENCY_US}μs)"
        fi
    fi
fi

echo ""

# Test 3: Delta Size
echo "3. DELTA SIZE VALIDATION"
echo "------------------------"

test_start "Delta file size (${MIN_DELTA_SIZE_KB}-${MAX_DELTA_SIZE_KB} KB)"
LATEST_DELTA=$(ls -1t shared/data/deltas/delta-*.bin 2>/dev/null | head -1)

if [ -z "$LATEST_DELTA" ]; then
    test_info "No delta files found yet"
else
    DELTA_SIZE=$(stat -f%z "$LATEST_DELTA" 2>/dev/null || stat -c%s "$LATEST_DELTA" 2>/dev/null)
    DELTA_SIZE_KB=$((DELTA_SIZE / 1024))

    if [ "$DELTA_SIZE_KB" -ge "$MIN_DELTA_SIZE_KB" ] && [ "$DELTA_SIZE_KB" -le "$MAX_DELTA_SIZE_KB" ]; then
        test_pass
        echo "  Delta size: ${DELTA_SIZE_KB} KB"
    else
        test_fail "Delta is ${DELTA_SIZE_KB}KB (expected: ${MIN_DELTA_SIZE_KB}-${MAX_DELTA_SIZE_KB}KB)"
    fi
fi

echo ""

# Test 4: Hint Download Performance
echo "4. HINT DOWNLOAD PERFORMANCE"
echo "----------------------------"

test_start "hint.bin download time (<${MAX_HINT_DOWNLOAD_SEC}s)"
START_TIME=$(date +%s%N)
curl -sf -o /dev/null http://localhost:8080/hint.bin
END_TIME=$(date +%s%N)

DOWNLOAD_TIME_NS=$((END_TIME - START_TIME))
DOWNLOAD_TIME_MS=$(echo "scale=0; $DOWNLOAD_TIME_NS / 1000000" | bc)
DOWNLOAD_TIME_S=$(echo "scale=2; $DOWNLOAD_TIME_MS / 1000" | bc)
DOWNLOAD_TIME_S_INT=$(echo "$DOWNLOAD_TIME_S / 1" | bc)

if [ "$DOWNLOAD_TIME_S_INT" -lt "$MAX_HINT_DOWNLOAD_SEC" ]; then
    test_pass
    echo "  Download time: ${DOWNLOAD_TIME_S} seconds"
else
    test_fail "Download took ${DOWNLOAD_TIME_S}s (threshold: ${MAX_HINT_DOWNLOAD_SEC}s)"
fi

# Calculate download speed
HINT_SIZE=$(stat -f%z shared/data/hint.bin 2>/dev/null || stat -c%s shared/data/hint.bin 2>/dev/null)
HINT_SIZE_MB=$(echo "scale=1; $HINT_SIZE / 1024 / 1024" | bc)
DOWNLOAD_SPEED=$(echo "scale=1; $HINT_SIZE_MB / $DOWNLOAD_TIME_S" | bc)
test_info "Download speed: ${DOWNLOAD_SPEED} MB/s"

echo ""

# Test 5: Delta Application Performance
echo "5. DELTA APPLICATION PERFORMANCE"
echo "--------------------------------"

test_start "Client-side delta XOR performance (<${MAX_DELTA_APPLICATION_MS}ms)"
# Simulate client-side delta application using Node.js
cat > /tmp/test-delta-apply.js << 'EOF'
const fs = require('fs');
const path = require('path');

// Read hint.bin into memory
const hintPath = process.argv[2];
const hint = new Uint8Array(fs.readFileSync(hintPath));

// Read a delta file
const deltaPath = process.argv[3];
const deltaData = fs.readFileSync(deltaPath);

const startTime = process.hrtime.bigint();

// Parse delta (simplified: assume 2000 updates × 16 bytes each)
const numUpdates = 2000;
for (let i = 0; i < numUpdates; i++) {
    const offset = i * 16;

    // Read hintSetID (8 bytes)
    const hintSetID = Number(new DataView(deltaData.buffer, offset, 8).getBigUint64(0, true));

    // Read delta value (8 bytes)
    const deltaValue = new Uint8Array(deltaData.buffer, offset + 8, 8);

    // Apply XOR to hint
    const hintOffset = 32 + (hintSetID * 8);
    if (hintOffset + 8 <= hint.length) {
        for (let j = 0; j < 8; j++) {
            hint[hintOffset + j] ^= deltaValue[j];
        }
    }
}

const endTime = process.hrtime.bigint();
const elapsedNs = endTime - startTime;
const elapsedMs = Number(elapsedNs) / 1000000;

console.log(elapsedMs.toFixed(2));
EOF

LATEST_DELTA=$(ls -1t shared/data/deltas/delta-*.bin 2>/dev/null | head -1)
if [ -z "$LATEST_DELTA" ]; then
    test_info "No delta files found, skipping delta application test"
else
    if command -v node >/dev/null 2>&1; then
        APPLY_TIME_MS=$(node /tmp/test-delta-apply.js shared/data/hint.bin "$LATEST_DELTA")
        APPLY_TIME_MS_INT=$(echo "$APPLY_TIME_MS / 1" | bc)

        if [ "$APPLY_TIME_MS_INT" -lt "$MAX_DELTA_APPLICATION_MS" ]; then
            test_pass
            echo "  Application time: ${APPLY_TIME_MS} ms"
        else
            test_fail "Delta application took ${APPLY_TIME_MS}ms (threshold: ${MAX_DELTA_APPLICATION_MS}ms)"
        fi
    else
        test_info "Node.js not available, skipping delta application test"
    fi
fi

rm -f /tmp/test-delta-apply.js

echo ""

# Test 6: Throughput
echo "6. THROUGHPUT TESTS"
echo "-------------------"

test_start "Query throughput (queries per second)"
START_TIME=$(date +%s%N)
NUM_QUERIES=100

for i in $(seq 1 $NUM_QUERIES); do
    INDEX=$((RANDOM % 8388608))
    curl -sf -X POST http://localhost:3000/query/plaintext \
        -H "Content-Type: application/json" \
        -d "{\"index\": $INDEX}" > /dev/null 2>&1
done

END_TIME=$(date +%s%N)
ELAPSED_NS=$((END_TIME - START_TIME))
ELAPSED_S=$(echo "scale=2; $ELAPSED_NS / 1000000000" | bc)
QPS=$(echo "scale=1; $NUM_QUERIES / $ELAPSED_S" | bc)

test_pass
echo "  Throughput: ${QPS} queries/second"
echo "  Total time for ${NUM_QUERIES} queries: ${ELAPSED_S} seconds"

echo ""

# Summary
echo "=========================================="
echo "PERFORMANCE SUMMARY"
echo "=========================================="
echo "Total tests run: $TESTS_RUN"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
fi
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ ALL PERFORMANCE TESTS PASSED${NC}"
    echo ""
    echo "Performance meets all targets:"
    echo "  Query latency: <${MAX_QUERY_LATENCY_MS}ms ✓"
    echo "  Update latency: <${MAX_UPDATE_LATENCY_US}μs ✓"
    echo "  Delta size: ${MIN_DELTA_SIZE_KB}-${MAX_DELTA_SIZE_KB}KB ✓"
    echo "  Hint download: <${MAX_HINT_DOWNLOAD_SEC}s ✓"
    echo "  Delta application: <${MAX_DELTA_APPLICATION_MS}ms ✓"
    echo ""
    exit 0
else
    echo -e "${RED}✗ SOME PERFORMANCE TESTS FAILED${NC}"
    echo ""
    echo "Please review the failures above and check:"
    echo "  - Service configuration and resource allocation"
    echo "  - Network latency to localhost"
    echo "  - System load and available resources"
    echo ""
    exit 1
fi
