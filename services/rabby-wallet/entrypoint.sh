#!/bin/sh
# =============================================================================
# Plinko PIR Wallet - Runtime Environment Variable Injection
# =============================================================================
# Injects runtime environment variables into built Vite assets
# This solves the issue of Vite baking env vars at build time

set -e

echo "→ Injecting runtime environment variables..."

# Find the main JS bundle (usually in /usr/share/nginx/html/assets/)
ASSETS_DIR="/usr/share/nginx/html/assets"

# Default values if not provided
FALLBACK_RPC="${VITE_FALLBACK_RPC:-https://eth.llamarpc.com}"
PIR_SERVER_URL="${VITE_PIR_SERVER_URL:-/api}"
CDN_URL="${VITE_CDN_URL:-/cdn}"

echo "  VITE_FALLBACK_RPC: ${FALLBACK_RPC}"
echo "  VITE_PIR_SERVER_URL: ${PIR_SERVER_URL}"
echo "  VITE_CDN_URL: ${CDN_URL}"

# Replace hardcoded localhost:8545 with the actual fallback RPC
# We need to handle both the default and any variations
if [ -d "$ASSETS_DIR" ]; then
    for file in "$ASSETS_DIR"/*.js; do
        if [ -f "$file" ]; then
            # Replace https://eth.drpc.org with the configured fallback RPC
            sed -i "s|https://eth.drpc.org|${FALLBACK_RPC}|g" "$file"
        fi
    done
    echo "✓ Environment variables injected"
else
    echo "⚠️  Assets directory not found: $ASSETS_DIR"
fi

echo "→ Starting nginx..."
exec nginx -g "daemon off;"
