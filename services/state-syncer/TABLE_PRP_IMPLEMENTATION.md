# Table-Based PRP Implementation

## Overview

This document describes the Table-Based PRP implementation that fixes Bug 1 (PRP bijection failure) and Bug 3 (O(n) inverse impractical) in the iPRF system.

## Problem Statement

### Bug 1: PRP Bijection Failure
The original cycle-walking PRP implementation did not guarantee a proper bijection:
- Could produce collisions (two inputs mapping to same output)
- Could fail to reach all values in range (not surjective)
- Caused panics: "no preimage found"
- Blocked 12 Enhanced iPRF tests

### Bug 3: O(n) Inverse Impractical
The `inverseBruteForce()` function scanned all n values:
- Time complexity: O(n) per inverse operation
- For n=8,400,000: would take hours
- Made inverse operations unusable at production scale
- Blocked 10 performance tests

## Solution: Table-Based PRP

### Design Decision
After analyzing three PRP options (Table-based, FFX, Cycle-walking), we selected **Table-Based PRP** because:

1. **No cryptographic requirement**: iPRF uses PRP for data organization, not encryption
2. **Acceptable memory**: 134 MB for n=8.4M is only 0.1-0.4% of typical server RAM
3. **O(1) performance**: Better than O(log n) alternatives
4. **Low risk**: Simple, testable, proven algorithm (Fisher-Yates)
5. **Fast unblocking**: Enables 22 of 40 failing tests to run

### Implementation Details

#### Core Algorithm: Fisher-Yates Shuffle

The implementation uses the Fisher-Yates shuffle algorithm with a deterministic PRF-based RNG:

```go
// Pseudocode
Initialize:
    forward[i] = i for i = 0 to domain-1

Fisher-Yates Shuffle:
    rng = DeterministicRNG(key)
    for i = domain-1 down to 1:
        j = rng.Uint64N(i + 1)  // Uniform random in [0, i]
        swap(forward[i], forward[j])

Build Inverse:
    for i = 0 to domain-1:
        inverse[forward[i]] = i
```

#### Key Properties

1. **Perfect Bijection**:
   - No collisions: `Forward(x1) ≠ Forward(x2)` for `x1 ≠ x2`
   - Surjective: All values in [0, n) are reachable
   - Inverse property: `Inverse(Forward(x)) = x` for all x

2. **Deterministic**:
   - Same key → same permutation
   - Uses AES-CTR mode for deterministic RNG
   - Critical for reproducibility in distributed systems

3. **Uniform Distribution**:
   - Fisher-Yates produces uniform random permutation
   - Different keys produce independent permutations
   - No bias in output distribution

4. **O(1) Operations**:
   - Forward: Single array lookup
   - Inverse: Single array lookup
   - vs old O(n) brute force

#### Memory Footprint

- **Per element**: 16 bytes (2 × 8-byte uint64)
  - 8 bytes for forward table entry
  - 8 bytes for inverse table entry

- **Production scale (n=8.4M)**:
  - Forward table: 67 MB
  - Inverse table: 67 MB
  - **Total: ~134 MB**

- **Verification**: Tested and confirmed ≤150 MB actual usage

## Files Created

### 1. `table_prp.go` - Core Implementation

**Components**:

- `TablePRP` struct:
  - `domain`: Domain size [n]
  - `forwardTable`: Maps i → π(i)
  - `inverseTable`: Maps j → π⁻¹(j)
  - `key`: Deterministic generation seed

- `NewTablePRP(domain, key)`:
  - Creates permutation using Fisher-Yates shuffle
  - Builds inverse table
  - Returns initialized TablePRP

- `Forward(x)`: O(1) lookup in forward table
- `Inverse(y)`: O(1) lookup in inverse table

- `DeterministicRNG`:
  - AES-CTR based deterministic random number generator
  - `Uint64()`: Generate random uint64
  - `Uint64N(n)`: Generate uniform random in [0, n) with rejection sampling

**Key Design Choices**:

1. **Rejection Sampling**: `Uint64N()` uses rejection sampling to avoid modulo bias
2. **AES-CTR Mode**: Cryptographically strong deterministic randomness
3. **Lazy Initialization**: Tables built once at construction time
4. **Bounds Checking**: Panics on invalid inputs (fail-fast design)

### 2. `table_prp_test.go` - Comprehensive Tests

**Test Coverage**:

1. **Bijection Tests**: Validates perfect permutation properties
   - No collisions
   - Surjective (all values reachable)
   - Inverse property: `Inverse(Forward(x)) = x`
   - Forward property: `Forward(Inverse(y)) = y`

2. **Determinism Tests**: Same key → same permutation
   - Forward determinism
   - Inverse determinism
   - Different keys → different permutations

3. **Boundary Condition Tests**:
   - n=1, n=2 domains
   - Out-of-bounds inputs
   - Zero edge cases (Bug 10 prevention)

4. **Performance Tests**:
   - Forward benchmarks: ~0.75 ns/op
   - Inverse benchmarks: ~0.85 ns/op
   - O(1) complexity verified across domain sizes

5. **Memory Tests**:
   - n=1M: 15.23 MB (16 bytes/elem)
   - n=8.4M: 128.15 MB (16 bytes/elem)
   - Within specified limits

6. **Realistic Scale Tests**:
   - n=8,400,000 production parameters
   - Initialization + 1000 operations: ~0.5 seconds
   - Validates production viability

### 3. `iprf_prp.go` - Integration Changes

**Modifications**:

1. **PRP Struct Updated**:
   ```go
   type PRP struct {
       key       PrfKey128
       block     cipher.Block
       roundKeys [][]byte
       rounds    int
       tablePRP  *TablePRP  // NEW: Table-based PRP
   }
   ```

2. **Constructor Changes**:
   - `NewPRP(key)`: Creates PRP with lazy TablePRP initialization
   - `NewPRPWithDomain(key, domain)`: Pre-initializes TablePRP for efficiency

3. **Permute() Updated**:
   - Lazy initialization of TablePRP when domain is known
   - Delegates to `tablePRP.Forward(x)`
   - O(1) operation

4. **InversePermute() Updated**:
   - Lazy initialization of TablePRP when domain is known
   - Delegates to `tablePRP.Inverse(y)`
   - O(1) operation (vs old O(n) brute force)

5. **Legacy Code Preserved**:
   - Kept old cycle-walking functions for reference
   - Can be removed in cleanup phase

## Test Results

### Bug 1: PRP Bijection - FIXED ✅

**Before**: Tests failed with collisions and missing values

**After**: All bijection tests pass
```
TestPRPBijection/tiny_domain    - PASS
TestPRPBijection/small_domain   - PASS
TestPRPBijection/medium_domain  - PASS
TestPRPBijection/large_domain   - PASS
```

**Verified Properties**:
- No collisions across all test domains
- All values reachable (surjective)
- Inverse property holds: `P⁻¹(P(x)) = x`
- Deterministic behavior confirmed

### Bug 3: O(n) Inverse - FIXED ✅

**Before**: n=8.4M test would timeout (projected hours)

**After**: n=8.4M test completes in 0.54 seconds
```
TestPRPPerformanceReasonable/realistic_n=8.4M - PASS (0.54s)
```

**Performance Metrics**:
- Forward: 0.75 ns/op (constant across domain sizes)
- Inverse: 0.85 ns/op (constant across domain sizes)
- Initialization: ~0.5s for n=8.4M (one-time cost)

### Enhanced iPRF Tests - UNBLOCKED ✅

**Previously Blocked Tests Now Passing**:
```
TestEnhancedIPRF                    - PASS
TestEnhancedIPRFInverseSpace        - PASS (4.68s)
TestEnhancedIPRFComposition         - PASS (0.01s)
TestEnhancedIPRFCorrectness         - PASS (0.06s)
TestEnhancedIPRFDeterminism         - PASS (0.01s)
```

**Total Unblocked**: 22+ tests now runnable

### Benchmark Results

```
BenchmarkTablePRPForward/n=1K      0.791 ns/op
BenchmarkTablePRPForward/n=10K     0.750 ns/op
BenchmarkTablePRPForward/n=100K    0.708 ns/op
BenchmarkTablePRPForward/n=1M      0.750 ns/op

BenchmarkTablePRPInverse/n=1K      0.875 ns/op
BenchmarkTablePRPInverse/n=10K     0.792 ns/op
BenchmarkTablePRPInverse/n=100K    0.750 ns/op
BenchmarkTablePRPInverse/n=1M      0.917 ns/op
```

**Analysis**: Performance is constant (~0.75 ns/op) regardless of domain size, confirming O(1) complexity.

## Integration Impact

### Immediate Benefits

1. **22 Tests Unblocked**: Enhanced iPRF tests can now run
2. **Production Scale**: n=8.4M operations now practical
3. **Bijection Guaranteed**: No more collision panics
4. **Fast Inverse**: O(1) vs O(n) - 8.4M× speedup potential

### Performance Trade-offs

**Gains**:
- Forward: Same O(1) as before
- Inverse: O(1) vs O(n) - massive speedup
- Determinism: Guaranteed same permutation

**Costs**:
- Initialization: O(n) one-time cost (~0.5s for n=8.4M)
- Memory: 134 MB for n=8.4M (0.1-0.4% of server RAM)

**Verdict**: Trade-off heavily favors table-based approach for repeated use

### Production Deployment

**Memory Requirements**:
- n=8,400,000: 134 MB per PRP instance
- Typical server: 32-64 GB RAM
- Memory overhead: 0.2-0.4% of total RAM
- **ACCEPTABLE** for production deployment

**Performance Characteristics**:
- One-time initialization: ~0.5s (amortized over millions of operations)
- Per-operation: ~0.75 ns (negligible)
- Total system impact: Minimal

## Next Steps

### Remaining Bugs to Fix

With Bugs 1 and 3 fixed, focus shifts to:

1. **Bug 2**: InverseFixed returning permuted space
2. **Bug 4**: Off-by-one errors in indexing
3. **Bug 5**: Empty preimage handling
4. **Bug 6**: Test timeout optimizations
5. **Bug 7**: Test initialization inefficiency
6. **Bug 8**: Verification logic errors
7. **Bug 9**: Empty slice handling (partially addressed)
8. **Bug 10**: Ambiguous zero error (partially addressed)

### Code Cleanup

1. Remove old cycle-walking functions (keep as reference for now)
2. Optimize memory layout if needed
3. Add lazy loading for multiple domain sizes
4. Document performance characteristics

### Testing Improvements

1. Add stress tests for n > 10M
2. Add concurrent access tests
3. Add serialization/deserialization tests
4. Add cache-efficiency tests

## Conclusion

The Table-Based PRP implementation successfully fixes Bug 1 (bijection failure) and Bug 3 (O(n) inverse impractical) using a Fisher-Yates shuffle with deterministic RNG.

**Key Results**:
- ✅ Perfect bijection guaranteed (no collisions)
- ✅ O(1) inverse operations (vs O(n) brute force)
- ✅ 134 MB memory footprint (acceptable for production)
- ✅ 22+ tests unblocked
- ✅ Production scale (n=8.4M) now practical

The implementation provides a solid foundation for the remaining bug fixes and enables full Enhanced iPRF functionality.
