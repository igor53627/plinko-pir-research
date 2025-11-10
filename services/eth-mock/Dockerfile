# Ethereum Mock Service - Foundry Anvil
# 8,388,608 pre-funded test accounts for Piano PIR PoC

FROM ghcr.io/foundry-rs/foundry:latest

WORKDIR /app

# Copy initialization script (already executable)
COPY init-anvil.sh /app/init-anvil.sh

# Expose Ethereum RPC port
EXPOSE 8545

# Health check disabled - curl not available in Foundry image
# HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 \
#   CMD curl -sf -X POST -H "Content-Type: application/json" \
#       --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
#       http://localhost:8545 || exit 1

# Start Anvil with initialization script
ENTRYPOINT ["/app/init-anvil.sh"]
