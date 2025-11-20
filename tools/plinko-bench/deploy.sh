#!/bin/bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
    echo "Usage: $0 <user@host>"
    exit 1
fi

REMOTE=$1
BINARY="./plinko-bench"

# Rebuild to ensure linux
echo "Building plinko-bench for Linux..."
GOOS=linux GOARCH=amd64 go build -v -o plinko-bench

echo "Deploying plinko-bench to $REMOTE..."
scp -o StrictHostKeyChecking=no $BINARY $REMOTE:~/plinko-bench/

echo "Deployed."
echo "Run command: ssh $REMOTE '~/plinko-bench/plinko-bench -db ~/plinko-bench/output/database.bin -hints 100000'"
