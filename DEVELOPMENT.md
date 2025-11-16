# Development Guide

Detailed development instructions for contributors to the Plinko PIR Research project.

## Table of Contents

- [Development Setup](#development-setup)
- [Architecture Details](#architecture-details)
- [iPRF Implementation](#iprf-implementation)
- [Bug Fixes Applied](#bug-fixes-applied)
- [Testing Strategy](#testing-strategy)
- [Code Quality](#code-quality)
- [Deployment](#deployment)
- [Contributing](#contributing)

## Development Setup

### Prerequisites

```bash
# Required
docker --version        # >= 20.10
docker compose version  # >= 2.0
go version             # >= 1.21
python3 --version      # >= 3.9
node --version         # >= 18.0

# Optional (for advanced features)
git lfs install        # For large file storage
```

### Local Environment

```bash
# Clone repository
git clone https://github.com/igor53627/plinko-pir-research.git
cd plinko-pir-research

# Install dependencies
make install-deps

# Run tests
make test

# Start development environment
make dev
```

### Environment Variables

Create a `.env` file in the project root:

```bash
# Ethereum Node
RPC_URL=http://eth-mock:8545
DB_SIZE=1000  # Number of accounts (local testing)

# Plinko Configuration
CHUNK_SIZE=256
ENTRY_SIZE=8

# IPFS Configuration
PLINKO_STATE_IPFS_API=http://ipfs:5001
PLINKO_STATE_IPFS_GATEWAY=http://localhost:8080/ipfs

# Development
DEBUG=true
LOG_LEVEL=debug
```

## Architecture Details

### System Components

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client Layer                             │
│  ┌──────────────────┐                                           │
│  │  Rabby Wallet    │  Privacy Mode Toggle                      │
│  │  (React + Vite)  │  → Download snapshot + deltas             │
│  └────────┬─────────┘  → Derive hint locally                    │
│           │            → Generate PIR query                      │
└───────────┼─────────────────────────────────────────────────────┘
            │
            ▼ PIR Query (encrypted)
┌───────────┼─────────────────────────────────────────────────────┐
│           │              Server Layer                            │
│  ┌────────▼─────────┐   ┌──────────────┐   ┌─────────────┐    │
│  │  PIR Server      │◀──│  CDN Mock    │◀──│  IPFS Node  │    │
│  │  (Query Handler) │   │  (Nginx)     │   │  (Kubo)     │    │
│  └────────┬─────────┘   └──────────────┘   └─────────────┘    │
│           │                                                      │
│           ▼                                                      │
│  ┌────────────────────────────────────────────────────────┐    │
│  │  State Syncer (Go)                                     │    │
│  │  - iPRF operations (iprf.go)                          │    │
│  │  - Inverse computation (iprf_inverse.go)              │    │
│  │  - PRP composition (iprf_prp.go)                      │    │
│  │  - TablePRP (table_prp.go)                            │    │
│  │  - Snapshot generation                                 │    │
│  └────────┬───────────────────────────────────────────────┘    │
└───────────┼─────────────────────────────────────────────────────┘
            │
            ▼ Block events
┌───────────┼─────────────────────────────────────────────────────┐
│           │              Data Layer                              │
│  ┌────────▼─────────┐   ┌──────────────┐                       │
│  │  Ethereum Node   │   │  Hypersync   │                       │
│  │  (Anvil/Geth)    │   │  (Optional)  │                       │
│  └──────────────────┘   └──────────────┘                       │
└─────────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **Snapshot Creation**:
   - State Syncer reads Ethereum state (via Hypersync or RPC)
   - Applies iPRF to create `database.bin`
   - Generates `address-mapping.bin` for lookups
   - Pins to IPFS, publishes manifest to CDN

2. **Client Query**:
   - Wallet downloads snapshot + recent deltas
   - Derives hint locally (no server interaction)
   - Generates PIR query (encrypted)
   - Sends to PIR Server

3. **Server Response**:
   - PIR Server performs matrix multiplication
   - Returns encrypted response
   - Client decrypts → balance

4. **Updates**:
   - New block arrives every 12s
   - State Syncer generates XOR delta
   - Publishes to CDN
   - Clients apply delta (O(1) time)

## iPRF Implementation

The invertible Pseudorandom Function (iPRF) implementation is the core of Plinko PIR:

### Core Files

- **`iprf.go`**: Core iPRF with binomial sampling
- **`iprf_inverse.go`**: Tree-based inverse (O(log m + k))
- **`iprf_inverse_correct.go`**: Mathematically correct inverse implementation
- **`iprf_prp.go`**: PRP composition with TablePRP
- **`table_prp.go`**: Fisher-Yates deterministic shuffle

### Key Algorithms

#### 1. Forward iPRF (iprf.go)

```go
// F(key, x) → bin
// Maps domain element x to a bin using binomial sampling
func (i *IPRF) Evaluate(key []byte, x int) int {
    // Sample k random values uniformly from [0, m)
    // Return index of sorted position
}
```

**Complexity**: O(k log k) for sorting samples

#### 2. Inverse iPRF (iprf_inverse.go)

```go
// F^{-1}(key, bin) → {x₁, x₂, ..., xₙ}
// Returns all domain elements that map to bin
func (i *IPRF) Inverse(key []byte, bin int) []int {
    // Tree enumeration algorithm
    // O(log m + k) expected time
}
```

**Optimization**: Tree-based enumeration instead of brute force (1046× speedup)

#### 3. TablePRP (table_prp.go)

```go
// Deterministic Fisher-Yates shuffle
// Ensures bijection: each input maps to unique output
func (t *TablePRP) Forward(x int) int {
    // O(1) lookup after O(n) precomputation
}

func (t *TablePRP) Inverse(y int) int {
    // O(1) lookup using reverse table
}
```

**Memory**: 16 bytes per element (~134MB for 8.4M elements)

### Parameter Configuration

```go
// Default parameters for Ethereum state
const (
    DomainSize = 8_400_000  // Number of Ethereum accounts
    RangeSize  = 1024       // Number of bins
    BallCount  = 32         // Samples per domain element
)
```

## Bug Fixes Applied

The production implementation includes 15 critical bug fixes from the research phase:

### High-Priority Bugs (Fixed)

1. **Inverse Performance** (Bug #1)
   - **Issue**: O(n) brute force inverse
   - **Fix**: Tree enumeration → O(log m + k)
   - **Impact**: 1046× speedup (60µs vs 62ms)

2. **PRP Bijection** (Bug #2)
   - **Issue**: Hash-based PRP not injective
   - **Fix**: TablePRP with Fisher-Yates shuffle
   - **Impact**: Perfect permutation guarantee

3. **Key Persistence** (Bug #3)
   - **Issue**: Random keys invalidate hints
   - **Fix**: Deterministic key derivation from master seed
   - **Impact**: Clients can reuse hints across restarts

4. **Node Encoding** (Bug #4)
   - **Issue**: 16-bit node IDs cause collisions for n > 65536
   - **Fix**: SHA-256 hash for node encoding
   - **Impact**: Supports 8.4M+ accounts

5. **Parameter Separation** (Bug #5)
   - **Issue**: Confusion between originalN and ballCount
   - **Fix**: Clear parameter naming and validation
   - **Impact**: Correct binomial distribution

### Additional Fixes (6-15)

- Space conversion bugs in inverse computation
- Zero-handling in ambiguous cases
- Cache invalidation issues
- Distribution uniformity
- Memory leaks in long-running servers
- [See archived research docs for complete list]

### Testing Coverage

All bug fixes are validated by comprehensive test suites:

```bash
cd services/state-syncer
go test -v ./...
# 87/87 tests passing (100%)
```

## Testing Strategy

### Test Organization

```
services/state-syncer/
├── iprf_test.go                    # Core iPRF tests
├── iprf_inverse_test.go            # Inverse correctness tests
├── iprf_prp_test.go                # PRP composition tests
├── table_prp_test.go               # TablePRP unit tests
├── iprf_integration_test.go        # End-to-end integration
├── iprf_performance_benchmark_test.go  # Performance benchmarks
└── iprf_enhanced_test.go           # Edge case coverage
```

### Unit Tests

```bash
# Run all tests
cd services/state-syncer
go test -v ./...

# Run specific test suite
go test -v -run TestIPRF

# Run with coverage
go test -cover ./...
```

### Integration Tests

```bash
# Full system integration test
go test -v -run TestSystemIntegration

# Tests:
# - Forward iPRF correctness
# - Inverse completeness
# - PRP bijection
# - Snapshot generation
# - Query/response flow
```

### Performance Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem

# Key benchmarks:
# - BenchmarkIPRFForward: ~10µs per operation
# - BenchmarkIPRFInverse: ~60µs per operation
# - BenchmarkTablePRPForward: ~0.5ns per operation
# - BenchmarkTablePRPInverse: ~0.5ns per operation
```

### Python Reference Tests

```bash
cd plinko-reference
python3 test_iprf_simple.py

# Tests:
# - Binomial sampling correctness
# - Inverse correctness
# - Distribution uniformity
# - Edge cases (empty bins, overflow)
```

## Code Quality

### Pre-commit Checks

```bash
# Run linter
golangci-lint run

# Format code
gofmt -w .

# Run all tests
go test ./...

# Check test coverage
go test -cover ./... | grep -E "coverage: [0-9]+\.[0-9]+%"
```

### Linting Configuration

Create `.golangci.yml`:

```yaml
linters:
  enable:
    - gofmt
    - govet
    - staticcheck
    - errcheck
    - gosimple
    - ineffassign
    - unused

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

### Code Style Guidelines

- **Naming**: Use descriptive variable names (no single-letter except i, j, k in loops)
- **Comments**: Document all exported functions with godoc-style comments
- **Error Handling**: Always check errors, never use `_` for error returns
- **Testing**: Aim for 100% coverage on critical paths
- **Performance**: Use benchmarks to validate optimizations

### Test Coverage Goals

Target: **100% coverage for critical paths**

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Critical paths requiring 100% coverage:
# - iPRF forward/inverse
# - TablePRP forward/inverse
# - Snapshot generation
# - Query/response handling
```

## Deployment

See [DEPLOYMENT.md](docs/DEPLOYMENT.md) for detailed production deployment instructions.

### Local Development

```bash
# Start all services
make build && make start

# Tail logs
make logs

# Stop services
make stop

# Clean up volumes
make clean
```

### Docker Compose Services

```yaml
services:
  eth-mock:         # Simulated Ethereum node
  state-syncer:     # iPRF state updates
  plinko-pir-server: # PIR query handler
  cdn:              # Snapshot distribution
  rabby-wallet:     # Privacy-enabled wallet UI
  ipfs:             # Snapshot pinning
```

### Remote Deployment (Vultr)

```bash
# Set environment variables
export VULTR_API_KEY=your_key_here
export VULTR_TAG=plinko-pir-production
export SSH_KEY=~/.ssh/id_rsa

# Bootstrap remote server
./scripts/vultr-deploy.sh bootstrap

# Sync code
./scripts/vultr-deploy.sh sync

# Start services
./scripts/vultr-deploy.sh up

# View logs
./scripts/vultr-deploy.sh logs
```

## Contributing

### Development Workflow

1. **Fork the repository**

```bash
git clone https://github.com/YOUR_USERNAME/plinko-pir-research.git
cd plinko-pir-research
git remote add upstream https://github.com/igor53627/plinko-pir-research.git
```

2. **Create feature branch**

```bash
git checkout -b feature/amazing-feature
```

3. **Make changes**

- Write tests first (TDD approach)
- Implement feature
- Run tests and linters
- Update documentation

4. **Commit changes**

```bash
git add .
git commit -m "feat: add amazing feature"
```

5. **Push to fork**

```bash
git push origin feature/amazing-feature
```

6. **Open Pull Request**

- Describe changes clearly
- Reference related issues
- Include test results
- Update CHANGELOG.md

### Commit Message Convention

```
<type>: <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `test`: Adding/updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvement
- `chore`: Maintenance tasks

### Pull Request Checklist

- [ ] Tests pass locally (`make test`)
- [ ] Code is formatted (`gofmt -w .`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] No breaking changes (or clearly documented)

## Research Archive

Research phase documentation has been archived to `/tmp/plinko-research-archive-*`.

This includes:
- TDD methodology reports
- Bug fix delivery reports
- Test execution logs
- Debug helper files

To access archived research:

```bash
# Find latest archive
ls -lh /tmp/plinko-research-archive-*

# View manifest
cat /tmp/plinko-research-archive-*/ARCHIVE_MANIFEST.md

# Recover specific file
cp /tmp/plinko-research-archive-*/state-syncer/BUG_4_FIX_REPORT.md .
```

## Troubleshooting

### Common Issues

**Issue**: Tests fail with "database.bin not found"

```bash
# Solution: Generate test database
cd services/state-syncer
go test -v -run TestSnapshotGeneration
```

**Issue**: IPFS connection refused

```bash
# Solution: Ensure IPFS container is running
docker compose up ipfs -d
```

**Issue**: Go build fails with module errors

```bash
# Solution: Update dependencies
go mod tidy
go mod download
```

**Issue**: Docker Compose services won't start

```bash
# Solution: Clean volumes and rebuild
make clean
make build
make start
```

## Additional Resources

- **[README.md](README.md)**: Project overview and quick start
- **[IMPLEMENTATION.md](IMPLEMENTATION.md)**: Technical deep-dive
- **[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)**: Production deployment
- **[services/state-syncer/README.md](services/state-syncer/README.md)**: iPRF implementation details
- **[plinko-reference/IPRF_IMPLEMENTATION.md](plinko-reference/IPRF_IMPLEMENTATION.md)**: Python reference

## Contact

For development questions or issues:
- **GitHub Issues**: https://github.com/igor53627/plinko-pir-research/issues
- **Pull Requests**: https://github.com/igor53627/plinko-pir-research/pulls

---

*Happy coding! Let's bring privacy to Ethereum.* 🔐
