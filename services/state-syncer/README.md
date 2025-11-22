# State Syncer Service

Continuously streams Ethereum mainnet (via Hypersync) and keeps the canonical `database.bin` plus public snapshot/delta artifacts fresh. This replaces the legacy Plinko hint generator entirely: clients now derive hints locally from the published snapshot package and append-only deltas.

## Responsibilities
- Load canonical `database.bin` + `address-mapping.bin` generated via `scripts/build_database_from_parquet.py`.
- Watch Hypersync RPC (or simulated mode) for touched addresses each block.
- Apply updates via the Plinko update manager and persist to `/data/database.bin`.
- Emit per-block `delta-XXXXXX.bin` files under `/public/deltas/`.
- Periodically publish versioned snapshot packages under `/public/snapshots/<version>/`.
- Copy `address-mapping.bin` into `/public` for wallet/bootstrap use.
- Expose `/health` and `/metrics` on port `3002` for monitoring.
- Pin each published snapshot to the bundled `ipfs/kubo` daemon (unless disabled) so CDN clients can download artifacts via IPFS.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PLINKO_STATE_DB_PATH` | `/data/database.bin` | Canonical database file to mutate in place. |
| `PLINKO_STATE_ADDRESS_MAPPING_PATH` | `/data/address-mapping.bin` | Input mapping file to copy into the public artifacts volume. |
| `PLINKO_STATE_PUBLIC_ROOT` | `/public` | Root directory for artifacts served by the CDN mock. |
| `PLINKO_STATE_DELTA_DIR` | `/public/deltas` | Directory for per-block delta files. |
| `PLINKO_STATE_RPC_URL` | `http://eth-mock:8545` | Ethereum RPC/Hypersync endpoint. |
| `PLINKO_STATE_RPC_TOKEN` | _empty_ | Optional bearer token for Hypersync. |
| `PLINKO_STATE_HTTP_PORT` | `3002` | Port for the embedded health/metrics server. |
| `PLINKO_STATE_START_BLOCK` | `0` | Block height that matches the seeded snapshot. |
| `PLINKO_STATE_SIMULATED` | `true` | Use deterministic fake updates instead of hitting RPC (default for Docker Compose). |
| `PLINKO_STATE_POLL_INTERVAL` | `5s` | Delay between RPC polls when the chain head is behind. |
| `PLINKO_STATE_SNAPSHOT_EVERY` | `0` | Publish a snapshot every N processed blocks (0 disables periodic snapshots). |
| `PLINKO_STATE_IPFS_API` | `http://ipfs:5001` | HTTP API for the bundled `ipfs/kubo` daemon. Set empty to skip pinning or point at your hosted pinning service. |
| `PLINKO_STATE_IPFS_GATEWAY` | `http://localhost:8080/ipfs` | Gateway base advertised inside `manifest.json` (the CDN proxies `/ipfs` to the local daemon). Override if you expose the CDN on a different hostname. |

## Running Locally

```bash
# Build only the syncer
docker compose build state-syncer

# Run against Hypersync (make sure PLINKO_STATE_RPC_TOKEN is exported)
docker compose run --rm \
  -e PLINKO_STATE_SIMULATED=false \
  -e PLINKO_STATE_RPC_URL=https://eth.rpc.hypersync.xyz \
  -e PLINKO_STATE_RPC_TOKEN=$PLINKO_RPC_TOKEN \
  state-syncer
```

> ⚠️ Run either `plinko-update-service` **or** `state-syncer`, not both against the same `/data` volume.

## Artifact Layout

```
/public
├── address-mapping.bin
├── deltas/
│   ├── delta-000123.bin
│   └── delta-000124.bin
└── snapshots/
    ├── latest -> block-000124
    └── block-000124/
        ├── database.bin
        └── manifest.json
```

Each `manifest.json` includes chunk/set sizes, DB size, and SHA-256 hash clients use before deriving hints locally.

When IPFS publishing is enabled the `files` array contains per-file `ipfs.cid` plus a fully-qualified gateway URL. Example:

```json
{
  "path": "database.bin",
  "size": 73400320,
  "sha256": "4f7f…",
  "ipfs": {
    "cid": "bafybeiaq…",
    "gateway_url": "http://localhost:8080/ipfs/bafybeiaq…"
  }
}
```

## Observability
- `GET /health` – readiness payload with last processed block and status.
- `GET /metrics` – JSON snapshot of processed blocks, update counts, and last processing duration.
