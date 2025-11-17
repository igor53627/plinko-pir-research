# Plinko PIR for Ethereum: Research Summary

**Research Focus**: Implementing Plinko PIR (Private Information Retrieval) protocol for Ethereum JSON-RPC queries, enabling users to query blockchain data without revealing their queries to RPC providers.

**Key Question**: Can Plinko PIR handle Ethereum's state size (5.6M addresses), update frequency (12-second blocks), and query patterns while maintaining acceptable performance for wallet/dApp usage?

**Research Evolution**:
1. **Phase 1-3**: Initial FrodoPIR analysis → ruled out (too slow: 108ms queries, 780 MB hints)
2. **Phase 6**: Piano PIR discovery → 22× faster (5ms), 11× smaller (70 MB hints)
3. **Current**: **Plinko PIR implementation** → real-time updates via XOR deltas

**Methodology**: Comparative analysis of PIR protocols (FrodoPIR, Piano, SimplePIR, OnionPIR), performance modeling with Ethereum-scale databases (5.6M production, 8.4M PoC), feasibility assessment for specific RPC calls (eth_getBalance: ✅ viable, eth_call: ❌ infeasible), and implementation of Plinko incremental updates for real-time blockchain synchronization.

**Final Implementation**: Plinko PIR with iPRF (invertible Pseudorandom Function) for efficient inverse queries, achieving:
- **5ms query latency** (production-ready for mobile wallets)
- **70 MB hint storage** (mobile-native feasibility)
- **O(1) incremental updates** (23.75ms per 2,000 accounts - 79× faster than SimplePIR)
- **5.6M real addresses** from Ethereum mainnet (last 100K blocks via Hypersync)
- **Information-theoretic privacy** (server cannot determine queried address)

**Practical Impact**: Enables privacy-preserving Ethereum queries without running full nodes - addressing a critical privacy gap where RPC providers currently track all user queries. Production-ready PoC demonstrates 70% RPC coverage at $0.10-0.31/user/month.

**Status**: ✅ **PoC Complete** - Docker Compose reference stack with Rabby wallet integration, iPRF implementation (Go + Python), real-time delta updates, IPFS snapshot distribution, and comprehensive testing (87 Go tests + 10 Python tests, 100% passing).

**Research Documentation**: See `findings/phase7-summary.md` for comprehensive comparative analysis (9 privacy solutions evaluated) and `findings/piano-vs-frodopir-comparison.md` for detailed Piano vs FrodoPIR comparison.

---

**Project Repository**: https://github.com/igor53627/plinko-pir-research
**Plinko Paper** (EUROCRYPT 2025): https://eprint.iacr.org/2024/318
**Piano Paper** (USENIX Security 2024): [Semantic Scholar](https://www.semanticscholar.org/paper/Piano%3A-Extremely-Simple%2C-Single-Server-PIR-with-Zhou-Park/8296729c0e5fa48c5b3229a3207c314a01214fef)
