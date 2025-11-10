# Plinko PIR for Ethereum: Research & Implementation

**Research Question**: Can Plinko PIR (with incremental Plinko updates) be practically applied to provide Private Information Retrieval for Ethereum public JSON-RPC endpoints, enabling users to query blockchain data without revealing their queries?

**Started**: 2025-11-09
**Status**: Complete - PoC Implemented

## Project Structure

```
plinko-pir-research/
├── services/              # PoC service implementations
├── data/                  # Binary database files
├── scripts/               # Utility scripts
├── docs/                  # Implementation documentation
├── shared/                # Shared data volumes
├── research/              # Research artifacts
│   ├── findings/          # Research findings by phase
│   ├── POC-IMPLEMENTATION.md
│   ├── POC-PLINKO-IMPLEMENTATION.md
│   ├── research-plan.md
│   └── _summary.md
├── docker-compose.yml     # Service orchestration
├── Makefile              # Convenience commands
├── IMPLEMENTATION.md     # PoC implementation guide
└── README.md             # This file
```

## Hypothesis

FrodoPIR's single-server, LWE-based PIR scheme could enable privacy-preserving Ethereum RPC queries by treating blockchain state/history as a database, potentially protecting user query patterns from RPC providers while maintaining reasonable performance.

## Background

### What is FrodoPIR?

FrodoPIR is a stateful, single-server Private Information Retrieval (PIR) scheme developed by Brave that allows clients to query databases without revealing what they're searching for. Named after Frodo from *The Lord of the Rings*, it enables "hidden queries" to servers.

**Key Innovation**: Uses pure Learning With Errors (LWE) cryptography without FHE, achieving:
- 170x database compression during offline phase
- Sub-second query responses for million-record databases
- Post-quantum security
- Simple modular arithmetic (no complex FHE operations)

**Two-Phase Protocol**:
1. **Offline Phase**: Server compresses database into public parameters; clients download and precompute query parameters (client-independent)
2. **Online Phase**: Clients send encrypted query vectors; server multiplies against database matrix and returns results

### Ethereum JSON-RPC Context

Ethereum JSON-RPC is the standard interface for querying blockchain data:
- `eth_getBalance`, `eth_call`, `eth_getTransactionReceipt`, etc.
- Users typically connect to public RPC providers (Infura, Alchemy, QuickNode)
- **Privacy Problem**: RPC providers can track which addresses users query, revealing user interests and holdings

**Current Privacy Limitations**:
- Running full nodes is expensive (1+ TB storage, high bandwidth)
- Light clients still reveal query patterns to serving nodes
- VPNs/Tor only hide IP, not query content

## Research Objectives

### Primary Questions

1. **Feasibility**: Can FrodoPIR's database model map to Ethereum state/history queries?
2. **Performance**: Would query latency be acceptable for wallet/dApp usage?
3. **Scalability**: Can it handle Ethereum's data size and update frequency?
4. **Cost**: What are the computational/bandwidth costs for clients and servers?
5. **Practicality**: What implementation challenges would arise?

### Secondary Questions

6. How do Rust vs C++ implementations compare for this use case?
7. What specific Ethereum RPC calls are viable with FrodoPIR?
8. How often would offline phase need regeneration (blockchain updates)?
9. What privacy guarantees actually achieved in practice?
10. Could this be integrated into existing wallets/libraries (ethers.js, web3.py)?

## Methodology

### Phase 1: Technical Analysis

**FrodoPIR Protocol Deep Dive**:
- Analyze official Brave blog post and academic paper
- Study Rust implementation (brave-experiments/frodo-pir)
- Study C++ implementation (itzmeanjan/frodoPIR)
- Identify performance characteristics and constraints

**Ethereum RPC Analysis**:
- Document common RPC call patterns (wallets, dApps)
- Analyze data structures returned by key endpoints
- Measure typical response sizes and query frequencies
- Identify state access patterns

### Phase 2: Mapping Analysis

**Database Model Mapping**:
- How to represent Ethereum state as FrodoPIR database?
  - Account balances → database records?
  - Transaction history → indexed entries?
  - Contract storage → key-value mappings?
- What granularity of queries is feasible?
  - Full address balance queries?
  - Individual storage slot reads?
  - Transaction receipt lookups?

**Parameter Selection**:
- Estimate database sizes for different Ethereum data types
- Calculate compression ratios and download sizes
- Determine optimal LWE parameters for use case

### Phase 3: Performance Modeling

**Benchmarking**:
- Extract performance metrics from both implementations
- Model query latency for typical Ethereum RPC calls
- Estimate bandwidth requirements (offline + online phases)
- Calculate server computational costs

**Comparison**:
- Compare with current Ethereum light client protocols
- Compare with full node query performance
- Assess tradeoffs vs privacy gains

### Phase 4: Practical Implementation Analysis

**Integration Challenges**:
- How to handle blockchain updates (new blocks every 12s)?
- Offline phase regeneration frequency and cost
- Client-side computation requirements (mobile wallets?)
- Network latency considerations

**Implementation Variants**:
- Rust implementation evaluation for production use
- C++ implementation performance advantages
- Potential optimizations for Ethereum-specific patterns

### Phase 5: Use Case Validation

**Viable Scenarios**:
- Wallet balance queries (high privacy value)
- DeFi portfolio tracking
- NFT ownership verification
- Historical transaction lookups

**Non-Viable Scenarios**:
- Real-time mempool queries (too dynamic)
- Complex multi-step contract calls
- Full chain syncing

## Initial Findings

### FrodoPIR Characteristics (from sources)

**Performance** (Rust implementation, 1M KB database):
- Query response: <1 second
- Server response size: >3.6x blow-up factor
- Financial cost: ~$1 for answering client queries
- Database compression: ~170x smaller than original

**Performance** (C++ implementation, 1GB database - 2^20 entries × 1KB):
- Server setup: 46.7s (ARM) / 67.7s (x86)
- Client query: 146 microseconds (ARM) / 454 microseconds (x86)
- Database encoding: <3.5x blow-up
- Server response bandwidth: 55.24 GB/s (ARM) / 32.46 GB/s (x86)

**Tested Configurations**:
- Database sizes: 2^16 to 2^20 items
- Element sizes: 8KB per record
- LWE dimension: 1572
- Plaintext bits: 9-10 bits depending on log₂(m)

**Limitations**:
- Requires honest-but-curious server assumption
- Database must be public (Ethereum state is public ✓)
- Stateful design with offline phase requirement
- Single-server (no multi-server security)
- Research prototype ("do not use in production")

### Ethereum State/RPC Characteristics

**State Size** (as of 2025):
- Full archive node: >12 TB
- Full node (pruned): ~1 TB
- Number of accounts: ~250 million
- Active accounts: ~50 million

**Common RPC Calls**:
- `eth_getBalance(address)` - 32 bytes response
- `eth_call(to, data)` - variable response
- `eth_getTransactionReceipt(hash)` - ~500 bytes
- `eth_getCode(address)` - variable (contracts)
- `eth_getStorageAt(address, slot)` - 32 bytes

**Query Patterns**:
- Wallet: Check 5-10 addresses periodically
- DeFi dashboard: Query 20-50 addresses + storage slots
- NFT viewer: Query ownership + metadata URIs
- Block updates: Every 12 seconds

## Preliminary Analysis

### Potential Viability

**Promising Aspects**:
✓ Ethereum state is public (fits FrodoPIR's database publicity requirement)
✓ Many queries are simple lookups (address → balance)
✓ Response sizes often small (32 bytes for balance)
✓ Sub-second queries could be acceptable for non-real-time use
✓ Post-quantum security aligns with blockchain's long-term value

**Challenges Identified**:
❌ State updates every 12 seconds (offline phase regeneration?)
❌ Database size (250M accounts = massive offline download)
❌ Variable-size responses (contract calls)
❌ Real-time requirements for some dApps
❌ Client computation on mobile devices
❌ Multi-query patterns (wallet checks multiple addresses)

### Critical Questions

1. **Database Scope**: Full state vs subset?
   - Option A: Full state (250M accounts) → massive offline phase
   - Option B: Popular/active accounts only → reduces privacy set
   - Option C: Time-windowed data (recent blocks) → smaller, needs updates

2. **Update Frequency**:
   - Regenerate offline phase every block? (12s) → too expensive
   - Regenerate daily? → stale data
   - Differential updates? → complex protocol extension

3. **Query Granularity**:
   - Per-account queries → manageable
   - Per-storage-slot queries → state explosion
   - Contract calls with computation → doesn't fit PIR model

### Implementation Comparison

**Rust (brave-experiments/frodo-pir)**:
- Official implementation by authors
- Simpler API, clearer code structure
- Better for research and prototyping
- Research prototype warning
- Docker support for testing

**C++ (itzmeanjan/frodoPIR)**:
- Header-only, zero dependencies
- Significantly faster (ARM: 3.1x faster queries)
- Production-optimization friendly
- More complex integration
- Better for performance-critical deployment

## Next Steps

### Experiments to Run

1. **Benchmark with Ethereum-sized data**:
   - Test FrodoPIR with 2^20+ database sizes
   - Measure offline phase time/size with realistic parameters
   - Calculate bandwidth for different account set sizes

2. **Prototype simple balance query**:
   - Implement address → balance lookup using FrodoPIR
   - Measure end-to-end latency
   - Compare with direct RPC call

3. **Model update strategies**:
   - Calculate cost of regenerating per block vs batched
   - Explore incremental update possibilities
   - Estimate server costs at scale

4. **Integration analysis**:
   - Study ethers.js/web3.py provider architecture
   - Design FrodoPIR provider interface
   - Identify integration points

### Quick Start

#### Option 1: Docker Compose (Fastest)

```bash
# Build and start all services
make build
make start

# Access wallet interface
open http://localhost:5173

# Run tests
make test
```

#### Option 2: Local Kubernetes (Production-like)

Test the full Kubernetes deployment locally before deploying to Vultr:

```bash
# Navigate to Helm chart directory
cd deploy/helm/plinko-pir

# Deploy to local Kind cluster (automated)
./scripts/deploy-local.sh

# Run validation tests
./scripts/test-local.sh

# Access wallet
open http://localhost:30173
```

See [IMPLEMENTATION.md](IMPLEMENTATION.md) for Docker Compose setup or [deploy/LOCAL_TESTING.md](deploy/LOCAL_TESTING.md) for local Kubernetes testing.

## Deployment

The PoC can be deployed in multiple ways:

### Development (Docker Compose)
- **Best for**: Local development and testing
- **Setup time**: 5 minutes
- **Resources**: Minimal (runs on laptop)
- **Guide**: [IMPLEMENTATION.md](IMPLEMENTATION.md)

### Local Kubernetes (Kind/Minikube)
- **Best for**: Testing Helm charts before production deployment
- **Setup time**: 15 minutes (including initialization)
- **Resources**: 8GB RAM, 4 CPU cores
- **Guide**: [deploy/LOCAL_TESTING.md](deploy/LOCAL_TESTING.md)

```bash
cd deploy/helm/plinko-pir
./scripts/deploy-local.sh
```

### Production Kubernetes (Vultr VKE)
- **Best for**: Production deployment with auto-scaling and high availability
- **Setup time**: 20 minutes
- **Resources**: 3-node cluster (4GB RAM per node minimum)
- **Guide**: [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md)

```bash
cd deploy/helm/plinko-pir
./scripts/deploy.sh --production
```

## Research Artifacts

All research findings are located in the [`research/`](research/) directory:

- **[research/findings/](research/findings/)** - Phase-by-phase research results
- **[research/POC-IMPLEMENTATION.md](research/POC-IMPLEMENTATION.md)** - Original FrodoPIR analysis
- **[research/POC-PLINKO-IMPLEMENTATION.md](research/POC-PLINKO-IMPLEMENTATION.md)** - Plinko PIR implementation details
- **[research/research-plan.md](research/research-plan.md)** - Original research plan
- **[research/_summary.md](research/_summary.md)** - Executive summary

## Implementation

The proof-of-concept demonstrates:

- **Plinko PIR**: Information-theoretic private queries
- **Plinko Updates**: Incremental O(1) updates (79× speedup)
- **Ethereum Scale**: 8.4M accounts (Warm Tier)
- **Real-time Updates**: 12-second block synchronization
- **Rabby Wallet Integration**: Privacy Mode toggle

See [IMPLEMENTATION.md](IMPLEMENTATION.md) for architecture and deployment.

## Resources

### Official FrodoPIR

- **Blog Post**: https://brave.com/blog/frodopir/
- **Rust Implementation**: https://github.com/brave-experiments/frodo-pir
- **C++ Implementation**: https://github.com/itzmeanjan/frodoPIR
- **Academic Paper**: [Stateful Single-Server PIR from LWE]

### Ethereum Documentation

- **JSON-RPC Specification**: https://ethereum.org/en/developers/docs/apis/json-rpc/
- **State Structure**: https://ethereum.org/en/developers/docs/data-structures-and-encoding/patricia-merkle-trie/
- **Light Clients**: https://ethereum.org/en/developers/docs/nodes-and-clients/light-clients/

### Related Work

- **Other PIR Schemes**: OnionPIR, PSIR, SimplePIR
- **Ethereum Privacy**: Tornado Cash, Aztec, Railgun
- **Light Client Protocols**: Portal Network, Helios

## Expected Outcomes

### Best Case

FrodoPIR proves viable for specific Ethereum RPC use cases:
- Balance/storage queries for known account sets
- Historical data lookups (non-real-time)
- Privacy-focused wallets willing to trade latency for privacy
- Potential integration into next-gen light clients

### Realistic Case

FrodoPIR works but with significant limitations:
- Only viable for subset of RPC calls
- Requires hybrid approach (PIR + direct queries)
- High offline phase overhead limits update frequency
- Suitable for specific high-privacy scenarios only

### Worst Case

FrodoPIR fundamentally incompatible with Ethereum RPC:
- State size too large for practical offline phase
- Update frequency incompatible with 12s blocks
- Query patterns don't map to database index model
- Performance unacceptable for user experience

**Even negative results are valuable** - documenting what doesn't work guides future privacy research.

## Meta Notes

**Research Transparency**: This investigation is conducted entirely by Claude Code, demonstrating LLM capability for technical cryptography/blockchain analysis. All findings, code, and benchmarks will be documented with full transparency.

**Practical Impact**: If viable, this could significantly improve Ethereum user privacy without requiring full node operation - a real-world application of advanced cryptography.

---

*Exploring the intersection of post-quantum cryptography and blockchain privacy.*
