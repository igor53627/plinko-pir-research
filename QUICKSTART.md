# Quick Start Guide - Plinko PIR PoC

## Running the Proof-of-Concept

### Option 1: Using Make (Recommended)

```bash
# Build all services
make build

# Start the PoC
make start

# View logs
make logs

# Access wallet interface
open http://localhost:5173

# Run tests
make test

# Clean up
make reset
```

### Option 2: Using Docker Compose

```bash
# Build and start
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Option 3: First-time Setup Script

```bash
./scripts/init-poc.sh
```

See [IMPLEMENTATION.md](IMPLEMENTATION.md) for detailed setup instructions.

## Research Documentation

All research artifacts are in the [`research/`](research/) directory:

- **[research/findings/](research/findings/)** - Phase-by-phase research results
  - Phase 1-7 complete
  - Technical analysis
  - Performance benchmarks
  - Comparative analysis
- **[research/research-plan.md](research/research-plan.md)** - Original research plan
- **[research/POC-IMPLEMENTATION.md](research/POC-IMPLEMENTATION.md)** - FrodoPIR analysis
- **[research/POC-PLINKO-IMPLEMENTATION.md](research/POC-PLINKO-IMPLEMENTATION.md)** - Plinko implementation

## PoC Architecture

The implementation consists of 7 services:

1. **Ethereum Mock (Anvil)** - 8.4M pre-funded accounts
2. **Database Generator** - Extracts account balances
3. **Piano Hint Generator** - Creates PIR hints
4. **Plinko Update Service** - Real-time incremental updates
5. **Plinko PIR Server** - Private query endpoint
6. **CDN Mock** - Serves hint and delta files
7. **Rabby Wallet** - User interface with Privacy Mode

## Performance Metrics

- Query Latency: ~5-8ms
- Update Latency: ~24μs (with cache)
- Delta Size: ~30 KB per block
- Hint Download: ~1-2 seconds
- Information-theoretic privacy guarantee

## Development

```bash
# Build specific service
docker-compose build plinko-pir-server

# View service logs
docker-compose logs -f plinko-pir-server

# Run privacy tests
./scripts/test-privacy.sh

# Run performance tests
./scripts/test-performance.sh
```

See [IMPLEMENTATION.md](IMPLEMENTATION.md) for complete documentation.
