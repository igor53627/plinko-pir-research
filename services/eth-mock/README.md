# Ethereum Mock Service (Foundry Anvil)

**Purpose**: Simulated Ethereum blockchain with 8.4M pre-funded accounts for Plinko PIR testing

## Configuration

- **Accounts**: 8,388,608 (2^23 - Ethereum Warm Tier scale)
- **Balance per account**: 1000 ETH
- **Block time**: 12 seconds (matches Ethereum mainnet)
- **Chain ID**: 31337 (Anvil default)
- **RPC Port**: 8545

## Usage

### Start with Docker Compose
```bash
docker-compose up eth-mock
```

### Manual Testing
```bash
# Build service
docker-compose build eth-mock

# Start service
docker-compose up -d eth-mock

# Check logs
docker-compose logs -f eth-mock

# Health check
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545

# Query account balance
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_getBalance","params":["0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266","latest"],"id":1}' \
  http://localhost:8545
```

## Files

- `Dockerfile` - Container definition using Foundry image
- `init-anvil.sh` - Startup script with Anvil configuration
- `README.md` - This file

## Performance

**Expected startup time**: 30-60 seconds
- Account generation: ~20-40s (8.4M accounts)
- First block: ~12s after startup

**Memory usage**: ~2-4 GB (for 8.4M accounts)

## Verification

After startup, verify:
1. RPC responds to `eth_blockNumber`
2. Account count matches expected (8,388,608)
3. Blocks mine every 12 seconds
4. Each account has 1000 ETH balance

## Troubleshooting

**Problem**: Anvil fails to start
- Check Docker memory allocation (needs 4+ GB)
- Verify port 8545 is not in use

**Problem**: Slow account generation
- Normal for 8.4M accounts - wait 60s
- Check Docker resource limits

**Problem**: Health check failing
- Wait for account generation to complete
- Check Anvil logs for errors
