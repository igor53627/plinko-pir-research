#!/bin/bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
    echo "Usage: $0 <user@host> <remote_db_path>"
    exit 1
fi

REMOTE=$1
DB_PATH=$2

echo "Syncing reth-adapter source to $REMOTE..."
rsync -avz --exclude target --exclude .git ./ $REMOTE:~/plinko-bench/reth-adapter/

echo "Building reth-adapter on remote (requires Cargo)..."
ssh -o StrictHostKeyChecking=no $REMOTE "cd ~/plinko-bench/reth-adapter && cargo build --release"

echo "Running extraction on remote..."
echo "Reading DB from: $DB_PATH"
ssh -o StrictHostKeyChecking=no $REMOTE "RUST_LOG=info ~/plinko-bench/reth-adapter/target/release/reth-adapter --db-path $DB_PATH --out-dir ~/plinko-bench/output"

echo "Extraction complete. Artifacts in ~/plinko-bench/output"