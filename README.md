# Plinko PIR Research Implementation

> Private Information Retrieval using invertible Pseudorandom Functions (iPRF) based on the Plinko paper (EUROCRYPT 2025)

## Overview

Plinko is a single-server Private Information Retrieval (PIR) protocol with efficient updates. This implementation provides a production-ready system for private blockchain state queries.

**Key Features:**
- 🔒 **Privacy-Preserving**: Query blockchain state without revealing query contents
- ⚡ **High Performance**: O(log m + k) query complexity with iPRF inverse
- 🔄 **Efficient Updates**: Incremental state updates without full reconstruction
- 🎯 **Production-Ready**: Comprehensive test coverage, deployment guides
- 🐍 **Multi-Language**: Go (production) and Python (reference) implementations

## Quick Start

### Docker Compose (Fastest - 5 minutes)

```bash
git clone https://github.com/igor53627/plinko-pir-research.git
cd plinko-pir-research

# Start all services
make build && make start

# Access the demo wallet
open http://localhost:5173
```

**What you get:**
- Rabby wallet fork with "Privacy Mode" toggle
- 1,000 test accounts with balances
- Live Plinko PIR decoding visualization
- Real-time delta updates every 12 seconds

## Architecture

```
┌─────────────────┐      ┌──────────────────┐      ┌─────────────────┐
│  Wallet Client  │ ───▶ │  PIR Server      │ ───▶ │  State Syncer   │
│  (Privacy Mode) │ ◀─── │  (Query Handler) │ ◀─── │  (iPRF Updates) │
└─────────────────┘      └──────────────────┘      └─────────────────┘
                                                            │
                                                            ▼
                                                    ┌───────────────┐
                                                    │ Ethereum Node │
                                                    └───────────────┘
```

### Core Components

| Service | Purpose | Technology |
|---------|---------|------------|
| **eth-mock** | Simulated Ethereum node | Anvil (Foundry) |
| **plinko-update-service** | Monitors blocks, generates deltas | Go + WebSocket |
| **state-syncer** | Streams Hypersync blocks → snapshots/deltas | Go + Hypersync RPC |
| **plinko-pir-server** | Handles PIR queries | Plinko protocol |
| **cdn** | Distributes snapshot packages/deltas + proxies IPFS | Nginx / CloudFlare R2 |
| **rabby-wallet** | Privacy-enhanced wallet UI | React + Vite |
| **ipfs** | Local Kubo daemon (pin snapshots) | ipfs/kubo |

## Performance

**iPRF Inverse (Production):**
- Domain size: 5.6M accounts
- Range size: 1024 bins
- Inverse time: **60µs** (O(log m + k))
- Speedup: **1046× faster** than brute force

**TablePRP:**
- Forward/Inverse: **O(1)** with 0.54ns per operation
- Memory: 16 bytes per element (~90MB for 5.6M)

**Plinko Update Performance:**
```
Traditional PIR (SimplePIR):
  Update 2,000 accounts: 1,875ms (database regeneration)

Plinko PIR:
  Update 2,000 accounts: 23.75ms (XOR deltas)

Speedup: 79× faster ⚡
```

This makes Plinko the **first PIR system viable for real-time blockchain synchronization** (12-second Ethereum blocks).

## Testing

### Go Tests

```bash
cd services/state-syncer
go test -v ./...
# 87/87 tests passing (100%)
```

### Python Tests

```bash
cd plinko-reference
python3 test_iprf_simple.py
# 10/10 tests passing (100%)
```

## Research Summary

### Key Findings

| Research Area | Finding | Status |
|---------------|---------|--------|
| **eth_getBalance** | ✅ **VIABLE** - 5.6M recent addresses, ~5 ms queries | PoC Implemented |
| **eth_call** | ❌ **NOT VIABLE** - Storage explosion (10B+ slots) | [Analysis](research/findings/phase-4-eth-call-analysis.md) |
| **eth_getLogs (Full)** | ❌ **NOT VIABLE** - 500B logs, 150 TB database | [Analysis](research/findings/phase-5-eth-logs-analysis.md) |
| **eth_getLogs (Per-User)** | ✅ **HIGHLY VIABLE** - 30K logs/user, 7.7 MB database | [Analysis](research/findings/phase-5-eth-logs-analysis.md) |
| **eth_getLogs (50K Blocks)** | ✅ **FEASIBLE** - 200M logs, 6.4-51 GB (with compression) | [Analysis](research/findings/eth-logs-50k-blocks.md) |
| **Fixed-Size Compression** | ✅ **VIABLE** - 4 approaches analyzed, 8-62× reduction | [Analysis](research/findings/fixed-size-log-compression.md) |

**External Summary**: [Plinko PIR Analysis](https://www.kimi.com/share/19a6fcb1-3f92-8c58-8000-0000f106bbd7)

### Balance Queries (eth_getBalance)

**Verdict**: ✅ **PRODUCTION VIABLE**

```
Configuration:
  - Database: 5,575,868 addresses (balances from the last 100K Ethereum blocks)
  - Entry size: 8 bytes (uint64 balance)
  - Snapshot artifacts: ~43 MB (database.bin) + ~128 MB (address-mapping.bin)
  - Query latency: ~5ms
  - Update latency: 23.75ms per 2,000 accounts (Plinko cache mode)
```

**Use Cases:**
- Privacy-focused wallets (MetaMask alternative)
- DeFi portfolio trackers
- Tax reporting tools
- Whale watchers

**Cost**: $0.09-0.14/user/month for 10K users

## Documentation

- **[Deployment Guide](docs/DEPLOYMENT.md)**: Production deployment instructions
- **[Development Guide](DEVELOPMENT.md)**: Detailed development setup and contribution guide
- **[Implementation Details](IMPLEMENTATION.md)**: Technical deep-dive
- **[State Syncer README](services/state-syncer/README.md)**: iPRF implementation details
- **[Python Implementation](plinko-reference/IPRF_IMPLEMENTATION.md)**: Python reference guide

## Research Paper

Implementation based on:
> **Plinko: Single-Server PIR with Efficient Updates via Invertible PRFs**
> Alexander Hoover, Sarvar Patel, Giuseppe Persiano, Kevin Yeo
> EUROCRYPT 2025
> [eprint.iacr.org/2024/318](https://eprint.iacr.org/2024/318)

Paper included: [`docs/research/plinko-pir-paper.pdf`](docs/research/plinko-pir-paper.pdf)

## Deployment

Plinko PIR ships as a Docker Compose reference stack:

```bash
make build && make start    # builds services + starts docker compose
make logs                   # tail logs per service
make clean                  # tear down containers + volumes
```

**Resources**: 4 GB RAM, 2 CPU cores

### Remote Deployment

See [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) for the fully scripted Vultr deployment workflow powered by `scripts/vultr-deploy.sh`.

### Preparing Canonical Database

Production datasets arrive as Parquet diffs. Convert them into `database.bin` + `address-mapping.bin`:

```bash
# 1. Copy raw diffs from reth-onion-dev
rsync -avz reth-onion-dev:~/plinko-balances/balance_diffs_blocks-*.parquet raw_balances/

# 2. Build the artifacts (writes into ./data/)
python3 scripts/build_database_from_parquet.py --input raw_balances --output data
```

## Key Innovations

### 1. Real-Time Blockchain Synchronization

**First PIR system** to achieve real-time sync with 12-second Ethereum blocks using Plinko's O(1) updates.

### 2. Smart Event Log Compression

[Template-based compression](research/findings/fixed-size-log-compression.md) reduces logs to fixed 256-byte entries:
- 85% coverage with 50 event templates
- ERC20/721 transfers, Uniswap swaps, DeFi events
- 8× database size reduction
- Lossless for common patterns

### 3. Hybrid Architecture

Combines three storage tiers:
- **Cuckoo Filters** (6.4 GB): Mobile-friendly references
- **Smart Compression** (51 GB): Desktop/server deployment
- **IPFS Fallback**: Complex events

## Use Cases

### Privacy Wallets

**Problem**: MetaMask reveals every address you query to Infura
**Solution**: Plinko PIR wallet with Privacy Mode

- Download 70 MB snapshot package (derive hint locally, one-time)
- Query any balance in 5ms (private!)
- Update with 60 KB deltas every block
- Cost: $0.09-0.14/user/month

### DeFi Analytics

**Problem**: Querying DeFi positions reveals trading strategies
**Solution**: Private log queries for recent activity (50K blocks)

- 7-day rolling window
- 6.4-51 GB database (depending on compression)
- Track Uniswap swaps, Aave positions privately
- 40-60ms query latency

### Tax Reporting

**Problem**: Tax tools see all wallet addresses
**Solution**: Per-user log database

- 30K logs/user = 7.7 MB database
- Complete transaction history
- Private query execution
- Export to tax software

## Development

For detailed development instructions, see [DEVELOPMENT.md](DEVELOPMENT.md).

### Prerequisites

```bash
# Docker & Docker Compose
docker --version  # >= 20.10
docker compose version  # >= 2.0
```

### Build and Test

```bash
# Clone repository
git clone https://github.com/igor53627/plinko-pir-research.git
cd plinko-pir-research

# Start services
make build
make start

# Run tests
make test

# View logs
make logs

# Stop services
make stop
```

## License

MIT License - see [LICENSE](LICENSE) file

## Contact & Links

- **GitHub**: https://github.com/igor53627/plinko-pir-research
- **Plinko Paper**: https://eprint.iacr.org/2024/318
- **Plinko Summary**: https://www.kimi.com/share/19a6fcb1-3f92-8c58-8000-0000f106bbd7
- **Issues**: https://github.com/igor53627/plinko-pir-research/issues

---

*Bringing information-theoretic privacy to Ethereum, one query at a time.* 🔐
