# Production Deployment (Vultr)

This repo now ships with a reproducible deployment workflow that targets the
existing Vultr environment (`tag=plinko-pir`). Everything is driven by
`scripts/vultr-deploy.sh`, which wraps the Vultr API, SSH, and Docker Compose
commands so you can push new revisions in a single command.

## 1. Prerequisites

1. Copy the shared secrets into the repo-local `.env` (ignored by git):
   ```bash
   cp /Users/user/pse/.env .env
   ```
   Required variables:
   - `VULTR_API_KEY` – API token with read/write access
   - `SSH_KEY` – path to the Ed25519 key that can log into the target VM
   - `VULTR_TAG` – optional override (defaults to `plinko-pir`)
   - Optional: `VULTR_REMOTE_DIR` (default `/opt/plinko-pir`), `VULTR_SSH_USER` (default `root`)

2. Ensure `curl`, `ssh`, `rsync`, and `python3` are installed locally.

3. Confirm the target instance has the `plinko-pir` tag (`./scripts/vultr-deploy.sh info` will print all details).

4. (One-time per dataset) copy the parquet balance diffs and build canonical artifacts:

```bash
rsync -avz reth-onion-dev:~/plinko-balances/balance_diffs_blocks-*.parquet raw_balances/
python3 scripts/build_database_from_parquet.py --input raw_balances --output data
```

## 2. First-Time Bootstrap

Run once per VM to install Docker + rsync:

```bash
./scripts/vultr-deploy.sh bootstrap
```

This performs:
- `apt-get update`
- installs `docker-ce`, `docker compose` plugin, `rsync`, `git`
- adds the SSH user to the `docker` group

## 3. Deploying

```bash
# Preview instance metadata + SSH command
./scripts/vultr-deploy.sh info

# Sync repo (rsync --delete with safe excludes) + docker compose up --build
./scripts/vultr-deploy.sh up

# Tail remote logs
./scripts/vultr-deploy.sh logs
```

### Commands

| Command    | Description |
|------------|-------------|
| `info`     | Prints instance ID, IP, plan, tags, and suggested SSH command |
| `ssh`      | Opens an interactive shell (`ssh -i $SSH_KEY root@<ip>`) |
| `bootstrap` | Installs Docker + dependencies (idempotent) |
| `sync`     | Rsyncs repo to `$VULTR_REMOTE_DIR` (excludes `.git`, data blobs, node_modules, etc.) |
| `up`       | Runs `sync`, `docker compose pull`, `docker compose up -d --build` |
| `down`     | `docker compose down` in the remote directory |
| `logs`     | `docker compose logs -f` for quick triage |

All commands derive the target IP via the Vultr API (`instances?tag=$VULTR_TAG`). To deploy to a different machine, set `VULTR_TAG` before invoking the script.

## 4. Remote Layout

- Code lives under `$VULTR_REMOTE_DIR` (default `/opt/plinko-pir`).
- Docker volumes persist under `/var/lib/docker`.
- The CDN container exposes `/ipfs/<cid>` (proxied to the bundled `ipfs/kubo` gateway).

## 5. Rolling Updates

1. `./scripts/vultr-deploy.sh up` – pushes code + restarts containers in-place.
2. Watch `./scripts/vultr-deploy.sh logs` for healthy startup (state-syncer should announce snapshot/metrics endpoints).
3. Verify CDN endpoints via:
   ```bash
   # Get your instance IP
   VULTR_IP=$(./scripts/vultr-deploy.sh info | grep "IP:" | awk '{print $2}')

   curl http://${VULTR_IP}:8080/health
   curl http://${VULTR_IP}:8080/ipfs/<latest-cid>
   ```

## 6. Pinning Provider Overrides

The state-syncer defaults to the bundled `ipfs/kubo` daemon. To use a managed pinning provider instead (web3.storage, Pinata, Infura IPFS, Estuary):

```bash
export PLINKO_STATE_IPFS_API=https://api.web3.storage/upload
export PLINKO_STATE_IPFS_GATEWAY=https://cdn.example.com/ipfs
./scripts/vultr-deploy.sh up
```

The CDN should continue to proxy `/ipfs/<cid>` so browsers only talk to a single origin.
