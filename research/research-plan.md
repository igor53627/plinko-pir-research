# Research Plan: FrodoPIR for Ethereum JSON-RPC

## Project Overview

Investigate the feasibility of applying FrodoPIR (Brave's Private Information Retrieval protocol) to Ethereum JSON-RPC queries, enabling privacy-preserving blockchain queries without running full nodes.

## Research Timeline

**Estimated Duration**: 2-3 weeks
**Complexity**: Advanced (cryptography + blockchain + performance analysis)

## Research Phases

### Phase 1: Deep Technical Analysis (Days 1-3)

**Goal**: Comprehensive understanding of FrodoPIR protocol and implementations

**Tasks**:

1.1. **Protocol Analysis**
   - Study FrodoPIR academic paper in detail
   - Document two-phase protocol (offline/online)
   - Understand LWE parameters and security implications
   - Identify protocol constraints and assumptions

1.2. **Rust Implementation Analysis** (brave-experiments/frodo-pir)
   - Clone and build repository
   - Study API design (api.rs, db.rs, util.rs)
   - Run existing benchmarks
   - Document parameter ranges (LWE dim, db size, element size)
   - Analyze compression ratios achieved

1.3. **C++ Implementation Analysis** (itzmeanjan/frodoPIR)
   - Clone and build repository
   - Compare architecture with Rust version
   - Run benchmarks on available hardware
   - Document performance characteristics
   - Identify optimization techniques used

1.4. **Performance Baseline**
   - Extract all benchmark data from both implementations
   - Create comparison table (query time, setup time, bandwidth)
   - Identify performance scaling patterns
   - Document hardware requirements

**Deliverables**:
- Technical summary document
- Performance comparison table
- Protocol flow diagrams
- Implementation notes

### Phase 2: Ethereum RPC Characterization (Days 4-5)

**Goal**: Understand Ethereum query patterns and data characteristics

**Tasks**:

2.1. **RPC Call Analysis**
   - Document all common JSON-RPC methods
   - Categorize by response size and complexity:
     - Simple lookups (getBalance, getCode)
     - Medium complexity (getTransactionReceipt)
     - Complex/variable (eth_call, getLogs)
   - Identify which fit PIR model

2.2. **State Size Analysis**
   - Research current Ethereum state size
   - Estimate number of active accounts
   - Calculate storage requirements for different data types:
     - Account balances only
     - + Contract code
     - + Storage slots (infeasible)
   - Analyze historical growth rates

2.3. **Query Pattern Research**
   - Study typical wallet query patterns
   - Analyze DeFi dashboard requirements
   - Document block explorer access patterns
   - Identify privacy-critical queries

2.4. **Update Frequency Analysis**
   - Calculate state change rate (per block)
   - Estimate percentage of accounts modified per block
   - Analyze implications for offline phase regeneration

**Deliverables**:
- Ethereum RPC categorization matrix
- State size estimates and projections
- Query pattern documentation
- Update frequency analysis

### Phase 3: Feasibility Mapping (Days 6-8)

**Goal**: Map Ethereum use cases to FrodoPIR capabilities

**Tasks**:

3.1. **Database Model Design**
   - **Option A**: Full state snapshot
     - Pros: Complete privacy set
     - Cons: Massive size, frequent updates
   - **Option B**: Active account subset
     - Pros: Smaller, more practical
     - Cons: Reduced anonymity set
   - **Option C**: Time-windowed data
     - Pros: Balance size/freshness
     - Cons: Complex update logic
   - **Option D**: Hybrid approach
     - Pros: Flexibility
     - Cons: Implementation complexity

3.2. **Parameter Selection**
   - For each database option:
     - Calculate required FrodoPIR parameters
     - Estimate offline phase download size
     - Calculate query response sizes
     - Determine regeneration frequency

3.3. **Use Case Viability Assessment**

   **High Priority** (check first):
   - Single address balance queries
   - Multiple address balance queries (wallet)
   - Transaction receipt lookups by hash
   - Contract code retrieval
   - Simple storage slot reads

   **Medium Priority**:
   - Historical balance queries
   - Token balance queries (ERC-20)
   - NFT ownership verification
   - Event log queries (filtered)

   **Low Priority** (likely non-viable):
   - Real-time mempool queries
   - Complex multi-call queries
   - Unbounded log queries
   - State across multiple blocks

3.4. **Performance Modeling**
   - Model end-to-end latency for viable use cases
   - Calculate bandwidth requirements
   - Estimate client computation (mobile devices?)
   - Project server costs at scale

**Deliverables**:
- Database design document with options
- Parameter selection matrix
- Use case viability table
- Performance models and estimates

### Phase 4: Proof of Concept (Days 9-12)

**Goal**: Implement and test a minimal working prototype

**Tasks**:

4.1. **Scope Definition**
   - Select simplest viable use case (likely: single balance query)
   - Choose implementation (Rust vs C++)
   - Define minimal feature set

4.2. **Ethereum State Snapshot**
   - Create test dataset with realistic properties:
     - Option 1: Random synthetic data (fastest)
     - Option 2: Real mainnet snapshot (most realistic)
     - Start small (1000-10000 accounts) for testing

4.3. **FrodoPIR Integration**
   - Adapt FrodoPIR to Ethereum data format
   - Implement offline phase with Ethereum state
   - Create client query for address → balance
   - Implement response decoding

4.4. **Testing & Measurement**
   - Measure offline phase generation time
   - Measure offline phase download size
   - Measure query time (client + server)
   - Measure response size
   - Test with varying database sizes (2^10 → 2^20 if possible)

4.5. **Scaling Analysis**
   - Extrapolate measurements to full Ethereum state
   - Calculate costs for realistic deployment
   - Identify performance bottlenecks
   - Document limitations encountered

**Deliverables**:
- Working proof-of-concept code
- Benchmark results
- Scaling analysis document
- Implementation challenges documentation

### Phase 5: Update Strategy Analysis (Days 13-14)

**Goal**: Solve the "blockchain updates every 12s" problem

**Tasks**:

5.1. **Update Models**
   - **Full Regeneration**: Cost of regenerating offline phase
     - Per block (12s) → compute/bandwidth requirements
     - Hourly (300 blocks) → staleness implications
     - Daily → practicality assessment
   - **Incremental Updates**: Explore possibilities
     - Can FrodoPIR support differential updates?
     - Complexity of maintaining consistency
     - Protocol modifications needed

5.2. **State Diff Analysis**
   - Analyze typical block state changes
   - Calculate percentage of database modified per block
   - Identify patterns (hot accounts vs cold accounts)
   - Explore "active set" strategies

5.3. **Hybrid Approaches**
   - PIR for historical/stable data
   - Direct queries for recent blocks
   - Privacy implications of hybrid model

5.4. **Cost-Benefit Analysis**
   - Server costs for different update frequencies
   - Client bandwidth for different strategies
   - Privacy degradation vs staleness tradeoffs

**Deliverables**:
- Update strategy comparison matrix
- Cost analysis for each approach
- Hybrid architecture proposal
- Recommendations

### Phase 6: Integration & Deployment Analysis (Days 15-17)

**Goal**: Assess real-world deployment feasibility

**Tasks**:

6.1. **Wallet Integration**
   - Study ethers.js provider architecture
   - Design FrodoPIRProvider interface
   - Identify integration points and challenges
   - Assess mobile wallet viability

6.2. **Infrastructure Requirements**
   - Server hardware requirements
   - Network bandwidth requirements
   - CDN considerations for offline phase distribution
   - Geographic distribution for latency

6.3. **Security Analysis**
   - Honest-but-curious server assumption validity
   - Potential attacks and mitigations
   - Privacy guarantees in practice
   - Comparison with alternatives (VPN, Tor)

6.4. **User Experience**
   - Latency impact on dApp usage
   - Initial setup time (offline phase download)
   - Update frequency UX implications
   - Fallback strategies for failures

**Deliverables**:
- Integration architecture document
- Infrastructure requirements specification
- Security analysis report
- UX impact assessment

### Phase 7: Comparative Analysis & Conclusions (Days 18-21)

**Goal**: Compare with alternatives and synthesize findings

**Tasks**:

7.1. **Alternative Comparison**
   - **Full Nodes**: Cost, latency, privacy (baseline)
   - **Light Clients**: Portal Network, Helios
   - **VPN/Tor**: Privacy level, latency, cost
   - **Trusted RPC**: Convenience vs privacy
   - **Other PIR Schemes**: OnionPIR, SimplePIR for Ethereum

7.2. **Tradeoff Analysis**
   - Privacy vs Performance matrix
   - Cost vs Convenience analysis
   - Security vs Usability tradeoffs
   - Create decision tree for when to use what

7.3. **Recommendations**
   - Scenarios where FrodoPIR makes sense
   - Scenarios where alternatives better
   - Implementation priorities if proceeding
   - Research gaps to address

7.4. **Future Work**
   - Protocol improvements needed
   - Optimization opportunities
   - Integration with Ethereum roadmap
   - Academic research questions

**Deliverables**:
- Comprehensive comparison table
- Tradeoff analysis document
- Final recommendations
- Future research directions

## Success Metrics

### Research Success
- [ ] Clear answer to: "Can FrodoPIR work for Ethereum RPC?"
- [ ] Quantitative performance data
- [ ] Documented limitations and workarounds
- [ ] Working proof-of-concept (even if limited)
- [ ] Actionable recommendations

### Documentation Quality
- [ ] Reproducible experiments
- [ ] Complete methodology documentation
- [ ] Performance benchmarks with real data
- [ ] Clear limitations noted
- [ ] Code and data published

### Practical Impact
- [ ] Inform Ethereum privacy research
- [ ] Guide RPC privacy tool development
- [ ] Contribute to PIR application understanding
- [ ] Provide foundation for follow-up work

## Risk Mitigation

**Risk**: FrodoPIR fundamentally incompatible with Ethereum
- **Mitigation**: Document findings thoroughly; negative results are valuable

**Risk**: Cannot obtain large enough Ethereum state snapshot
- **Mitigation**: Use synthetic data with realistic properties; extrapolate

**Risk**: Performance too poor to measure meaningful results
- **Mitigation**: Start small, scale gradually, focus on scaling analysis

**Risk**: Implementations don't build/work as documented
- **Mitigation**: Document setup issues; contribute fixes if time permits

**Risk**: Complexity exceeds time budget
- **Mitigation**: Prioritize core questions; document "out of scope" items clearly

## Resource Requirements

### Computational
- System capable of building Rust/C++ projects
- Sufficient RAM for FrodoPIR benchmarks (8GB+ recommended)
- Storage for Ethereum state snapshots (100GB+ for full experiments)

### Data
- Ethereum mainnet RPC access (Infura, Alchemy, or local node)
- FrodoPIR benchmark datasets
- Ethereum state snapshots (can generate synthetic)

### Tools
- Rust toolchain
- C++ compiler (C++20 support)
- Python for data analysis
- Benchmarking tools (Criterion, Google Benchmark)

## Expected Outcomes

### Technical Outputs
- Comprehensive research report (this README + findings)
- Proof-of-concept implementation
- Performance benchmark suite
- Integration architecture design

### Knowledge Contribution
- First systematic analysis of PIR for Ethereum RPC
- Performance data for FrodoPIR at blockchain scale
- Practical deployment guidance
- Open questions for future research

### Decision Support
- Clear go/no-go recommendation
- Cost-benefit analysis for deployment
- Integration roadmap if viable
- Alternative suggestions if not

## Notes

**Iterative Approach**: Research will adapt based on findings. If Phase 2 reveals fundamental incompatibility, pivot to comparative analysis of alternatives.

**Open Science**: All findings, code, and data will be published. Even negative results contribute to the field.

**Practical Focus**: While cryptographically rigorous, emphasis on real-world applicability and deployment feasibility.

---

*Research conducted entirely by Claude Code, demonstrating LLM capability for complex technical analysis spanning cryptography, blockchain, and systems performance.*
