#!/bin/bash
set -e

echo "=========================================="
echo "Plinko PIR PoC - Ethereum Mock (Anvil)"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  Accounts: 10 (minimal for PoC - db-generator creates its own)"
echo "  Balance per account: 10 ETH"
echo "  Block time: 12 seconds"
echo "  RPC Port: 8545"
echo ""
echo "Starting Anvil..."
echo ""

# Start Anvil with minimal accounts
# - Only 10 accounts (db-generator creates its own 8M addresses)
# - 10 ETH per account
# - 12-second block time (matches Ethereum mainnet)
# - Listen on all interfaces for Docker networking
exec anvil \
  --accounts 10 \
  --balance 10000000000000000000 \
  --block-time 12 \
  --host 0.0.0.0 \
  --port 8545 \
  --chain-id 31337
