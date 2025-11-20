#!/bin/bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
    echo "Usage: $0 <user@host>"
    exit 1
fi

REMOTE=$1
BINARY="./erigon-adapter"

# Rebuild to ensure linux (Erigon usually runs on Linux)
echo "Building erigon-adapter for Linux..."
GOOS=linux GOARCH=amd64 go build -v -o erigon-adapter

echo "Deploying erigon-adapter to $REMOTE..."
# Use rsync if available for better delta transfer, or scp
rsync -avz -e "ssh -o StrictHostKeyChecking=no" $BINARY $REMOTE:~/

echo "Deployed."
echo "Run command: ssh $REMOTE './erigon-adapter -chaindata /path/to/erigon/chaindata -out output'"
