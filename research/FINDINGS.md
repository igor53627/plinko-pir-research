# Plinko PIR Research Findings

Comprehensive analysis of Plinko PIR's viability for Ethereum JSON-RPC privacy.

## Key Findings Summary

| Research Area | Finding | Status |
|---------------|---------|--------|
| **eth_getBalance** | ✅ **VIABLE** - 5.6M recent addresses, ~5 ms queries | PoC Implemented |
| **eth_call** | ❌ **NOT VIABLE** - Storage explosion (10B+ slots) | [Analysis](findings/phase7-summary.md) |
| **eth_getLogs (Full)** | ❌ **NOT VIABLE** - 500B logs, 150 TB database | [Analysis](findings/phase7-summary.md) |
| **eth_getLogs (Per-User)** | ✅ **HIGHLY VIABLE** - 30K logs/user, 7.7 MB database | [Analysis](findings/phase7-summary.md) |
| **eth_getLogs (50K Blocks)** | ✅ **FEASIBLE** - 200M logs, 6.4-51 GB (with compression) | [Analysis](archive/fixed-size-log-compression.md) |
| **Fixed-Size Compression** | ✅ **VIABLE** - 4 approaches analyzed, 8-62× reduction | [Analysis](archive/fixed-size-log-compression.md) |

---

## Balance Queries (eth_getBalance)

### Production Configuration

**Verdict**: ✅ **PRODUCTION VIABLE**

```
Database: 5,575,868 addresses (balances from the last 100K Ethereum blocks)
Entry size: 8 bytes (uint64 balance)
Snapshot artifacts: ~43 MB (database.bin) + ~128 MB (address-mapping.bin)
Query latency: ~5ms
Update latency: 23.75ms per 2,000 accounts (Plinko cache mode)
```

> **Note**: The PoC/development environment (see IMPLEMENTATION.md) uses 8.4M simulated accounts via Anvil for scalability testing, while production deployment uses 5.6M real mainnet addresses.

### Use Cases

**Privacy-focused wallets:**
- MetaMask alternative with "Privacy Mode"
- DeFi portfolio trackers
- Tax reporting tools
- Whale watchers

**Economics:**
- Cost: $0.09-0.14/user/month for 10K users
- 70 MB one-time snapshot download
- 60 KB delta updates per block

---

## Key Innovations

### 1. Real-Time Blockchain Synchronization

**First PIR system** to achieve real-time sync with 12-second Ethereum blocks using Plinko's O(1) updates.

**Performance comparison:**
```
Traditional PIR (SimplePIR):
  Update 2,000 accounts: 1,875ms (database regeneration)

Plinko PIR:
  Update 2,000 accounts: 23.75ms (XOR deltas)

Speedup: 79× faster ⚡
```

This makes Plinko the first PIR system viable for real-time blockchain synchronization.

### 2. Smart Event Log Compression

[Template-based compression](archive/fixed-size-log-compression.md) reduces logs to fixed 256-byte entries:
- 85% coverage with 50 event templates
- ERC20/721 transfers, Uniswap swaps, DeFi events
- 8× database size reduction
- Lossless for common patterns

### 3. Hybrid Architecture

Combines three storage tiers for different deployment scenarios:
- **Cuckoo Filters** (6.4 GB): Mobile-friendly references
- **Smart Compression** (51 GB): Desktop/server deployment
- **IPFS Fallback**: Complex events

---

## Use Cases

### Privacy Wallets

**Problem**: MetaMask reveals every address you query to Infura

**Solution**: Plinko PIR wallet with Privacy Mode

**How it works:**
1. Download 70 MB snapshot package (one-time)
2. Derive hint locally (client-side computation)
3. Query any balance in 5ms (information-theoretic privacy)
4. Update with 60 KB deltas every block

**Cost**: $0.09-0.14/user/month

**Target users:**
- Privacy-conscious crypto users
- High-net-worth individuals
- Professional traders
- DeFi power users

### DeFi Analytics

**Problem**: Querying DeFi positions reveals trading strategies to RPC providers

**Solution**: Private log queries for recent activity (50K blocks)

**Configuration:**
- 7-day rolling window
- 6.4-51 GB database (depending on compression)
- Track Uniswap swaps, Aave positions, etc.
- 40-60ms query latency

**Use cases:**
- Private portfolio tracking
- Strategy backtesting without revealing positions
- Competitive trading firms
- Market makers

### Tax Reporting

**Problem**: Tax reporting tools see all wallet addresses and transaction history

**Solution**: Per-user log database with private query execution

**Configuration:**
- 30K logs/user = 7.7 MB database
- Complete transaction history
- Private query execution
- Export to tax software (TurboTax, etc.)

**Benefits:**
- Users maintain transaction privacy
- Tax compliance without data leakage
- Self-hosted or trusted service provider
- Audit trail without RPC surveillance

### Research & Development

**Academic research:**
- PIR protocol benchmarking
- Privacy-preserving blockchain analytics
- Cryptographic protocol evaluation

**Blockchain analytics:**
- Private on-chain research
- MEV strategy analysis
- Network health monitoring

**Compliance & auditing:**
- Private regulatory compliance checks
- Smart contract security auditing
- Forensic analysis with privacy guarantees

---

## Performance Summary

### iPRF Inverse (Production)

```
Domain size: 5.6M accounts
Range size: 1024 bins
Inverse time: 60µs (O(log m + k))
Speedup: 1046× faster than brute force
```

### TablePRP

```
Forward/Inverse: O(1) with 0.54ns per operation
Memory: 16 bytes per element (~90MB for 5.6M)
```

### Plinko Update Performance

```
Plinko Cache Mode:
  Update 2,000 accounts: 23.75ms
  Throughput: 84,000 updates/second

Comparison to SimplePIR:
  SimplePIR: 1,875ms (database regeneration)
  Plinko: 23.75ms (XOR deltas)
  Speedup: 79×
```

---

## Economic Analysis

### Cost per User (10K users)

**Infrastructure costs:**
- CDN bandwidth: $0.05-0.10/user/month
- Storage: $0.02/user/month
- Compute: $0.02-0.04/user/month

**Total**: $0.09-0.14/user/month

**Scaling:**
- 100K users: $0.05-0.08/user/month
- 1M users: $0.03-0.05/user/month

### Client Requirements

**One-time download:**
- Snapshot package: 70 MB
- Address mapping: 128 MB
- Total: ~200 MB

**Ongoing updates:**
- Delta per block: ~60 KB
- Per day (7,200 blocks): ~430 MB
- Per month: ~13 GB

**Compute requirements:**
- Hint derivation: ~100ms (one-time)
- Query computation: <1ms client-side
- Update application: ~10ms per block

---

## Limitations & Future Work

### Current Limitations

1. **RPC Coverage**: 70% of common wallet operations (eth_getBalance + eth_getLogs)
2. **Storage Overhead**: 128 MB address mapping (could be optimized)
3. **Hint Size**: 70 MB (larger than some light clients)
4. **Update Bandwidth**: 430 MB/day (manageable but non-trivial)

### Future Enhancements

1. **eth_call Support**:
   - Investigate state slot compression techniques
   - Per-contract state snapshots
   - Client-side state reconstruction

2. **Proof of Retrievability**:
   - Verify snapshot integrity without full download
   - Cryptographic commitments to database state

3. **Multi-Server PIR**:
   - Reduce hint size via distribution
   - Improved security against server collusion

4. **Incremental Hints**:
   - Update hints without full re-derivation
   - Reduce client compute requirements

---

## Comparison to Alternatives

See [Phase 7 Summary](findings/phase7-summary.md) for comprehensive comparison:

- SimplePIR: 79× slower updates
- FrodoPIR: 22× slower queries, 11× larger hints
- OnionPIR: Not suitable for frequent updates
- TOR + RPC: No information-theoretic privacy
- Light clients: Require full block headers
- Trusted RPC: No privacy guarantees

**Plinko PIR uniquely enables:**
- Information-theoretic privacy
- Real-time blockchain synchronization
- Mobile-friendly client requirements
- Production-viable economics

---

## References

- **Plinko Paper**: [eprint.iacr.org/2024/318](https://eprint.iacr.org/2024/318)
- **Piano Paper**: [USENIX Security 2024](https://www.semanticscholar.org/paper/Piano%3A-Extremely-Simple%2C-Single-Server-PIR-with-Zhou-Park/8296729c0e5fa48c5b3229a3207c314a01214fef)
- **Implementation**: [github.com/igor53627/plinko-pir-research](https://github.com/igor53627/plinko-pir-research)
- **Research Plan**: [research-plan.md](research-plan.md)
- **Methodology**: [_summary.md](_summary.md)
