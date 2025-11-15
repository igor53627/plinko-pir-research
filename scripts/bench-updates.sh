#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INPUT="${ROOT}/test-data/updates/sample_block.json"
ITERATIONS="${BENCH_ITER:-50}"
DB_SIZE="${BENCH_DB_SIZE:-8388608}"

if [[ ! -f "$INPUT" ]]; then
  echo "❌ Bench input not found at $INPUT"
  exit 1
fi

echo "🏃 Running update benchmark (${ITERATIONS} batches, db_size=${DB_SIZE})"
(
  cd "$ROOT/services/plinko-update-service"
  go run -tags bench . \
    -input "$INPUT" \
    -repeat "$ITERATIONS" \
    -db-size "$DB_SIZE"
)
