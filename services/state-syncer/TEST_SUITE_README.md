# iPRF Test Suite - TDD RED Phase

## Overview

This comprehensive test suite implements the **RED phase** of Test-Driven Development (TDD) to expose 10 critical bugs identified in the iPRF (Invertible Pseudorandom Function) implementation.

## Test Organization

### Test Files

| File | Purpose | Bugs Tested |
|------|---------|-------------|
| `iprf_prp_test.go` | PRP correctness tests | Bugs 1, 3, 9, 10 |
| `iprf_pmns_test.go` | Base iPRF/PMNS tests | Bugs 4, 5, 8 |
| `iprf_enhanced_test.go` | Enhanced iPRF tests | Bug 2 |
| `iprf_integration_test.go` | Integration tests | Bugs 6, 7 |
| `iprf_performance_benchmark_test.go` | Performance validation | Bug 3 |

## Bug Coverage

### Priority 1: Correctness-Breaking Bugs (CRITICAL)

#### Bug 1: PRP Bijection Failure
- **Location**: `iprf_prp.go` - `permuteCycleWalking()`
- **Test**: `TestPRPBijection` in `iprf_prp_test.go`
- **Expected Result**: FAIL - PRP not a proper bijection
- **Validation**: Tests P^-1(P(x)) = x, no collisions, surjection

#### Bug 4: Binomial Sampling Parameter Error
- **Location**: `iprf.go` - `sampleBinomial()`
- **Test**: `TestPMNSCorrectness` in `iprf_pmns_test.go`
- **Expected Result**: FAIL - Forward-inverse round trips fail
- **Validation**: Tests complete forward-inverse coverage

#### Bug 5: Node Encoding Collision
- **Location**: `iprf.go` - `encodeNode()`
- **Test**: `TestNodeEncodingUniqueness` in `iprf_pmns_test.go`
- **Expected Result**: FAIL - Collisions with large values
- **Validation**: Tests uniqueness with >2^16 and >2^32 values

### Priority 2: Functionality-Breaking Bugs

#### Bug 2: InverseFixed Returns Wrong Space
- **Location**: `iprf_prp.go` - `EnhancedIPRF.Inverse()`
- **Test**: `TestEnhancedIPRFInverseSpace` in `iprf_enhanced_test.go`
- **Expected Result**: CHECK - May already be fixed
- **Validation**: Tests preimages are in original space [0,n), not permuted space

#### Bug 8: Incomplete Recursion in Bin Collection
- **Location**: `iprf_inverse.go` - `enumerateBallsInBinRecursive()`
- **Test**: `TestBinCollectionComplete` in `iprf_pmns_test.go`
- **Expected Result**: FAIL - Missing preimages
- **Validation**: Compares against brute-force forward mapping

### Priority 3: Performance-Critical Bugs

#### Bug 3: O(n) Inverse Impractical
- **Location**: `iprf_prp.go` - `inverseBruteForce()`
- **Test**: `TestPRPPerformanceReasonable`, `TestInversePerformanceComplexity`
- **Expected Result**: FAIL/TIMEOUT - Takes hours for n=8.4M
- **Validation**: Tests performance at realistic scales

#### Bug 7: Cache Mode Ineffectiveness
- **Location**: Integration layer (hypothetical)
- **Test**: `TestCacheModeEffectiveness` in `iprf_integration_test.go`
- **Expected Result**: FAIL - No speedup with caching
- **Validation**: Measures cache vs no-cache performance

### Priority 4: Safety/Robustness Bugs

#### Bug 9: Empty Slice Access Panic
- **Location**: `iprf_inverse.go` - `GetDistributionStats()`
- **Test**: `TestGetDistributionStatsEmptyHandling` in `iprf_prp_test.go`
- **Expected Result**: PANIC - Accessing sizes[0] on empty slice
- **Validation**: Tests edge cases with empty domains

#### Bug 10: Ambiguous Zero Error Signaling
- **Location**: `iprf_prp.go` - `inverseBruteForce()`
- **Test**: `TestPRPInverseCorrectness` in `iprf_prp_test.go`
- **Expected Result**: FAIL - Cannot distinguish x=0 from "not found"
- **Validation**: Tests error handling and zero-value cases

### Integration Issues

#### Bug 6: Integration Issues
- **Test**: `TestSystemIntegration` in `iprf_integration_test.go`
- **Expected Result**: Tests overall system behavior
- **Validation**: Realistic scale forward/inverse operations

## Running the Tests

### Run All Tests
```bash
cd services/state-syncer
go test -v
```

### Run Specific Test File
```bash
go test -v -run TestPRP iprf_prp_test.go iprf.go iprf_prp.go iprf_inverse.go iprf_inverse_correct.go
```

### Run Specific Test
```bash
go test -v -run TestPRPBijection
```

### Run Performance Tests
```bash
go test -v -run Performance
```

### Run Benchmarks
```bash
go test -v -bench=. -benchmem
```

### Skip Long-Running Tests
```bash
go test -v -short
```

### Run with Race Detector
```bash
go test -v -race
```

## Expected Test Results (RED Phase)

### FAIL Tests (Exposing Bugs)

| Test | Bug | Reason |
|------|-----|--------|
| `TestPRPBijection` | Bug 1 | PRP not a proper bijection |
| `TestPMNSCorrectness` | Bug 4 | Forward-inverse round trips fail |
| `TestNodeEncodingUniqueness` | Bug 5 | Encoding collisions with large values |
| `TestBinCollectionComplete` | Bug 8 | Missing preimages in bin collection |
| `TestPRPInverseCorrectness` | Bug 10 | Ambiguous zero return value |

### TIMEOUT Tests (Performance Issues)

| Test | Bug | Reason |
|------|-----|--------|
| `TestPRPPerformanceReasonable` | Bug 3 | O(n) inverse takes too long |
| `TestInversePerformanceComplexity` | Bug 3 | Realistic scale (n=8.4M) times out |

### PANIC Tests (Safety Issues)

| Test | Bug | Reason |
|------|-----|--------|
| `TestGetDistributionStatsEmptyHandling` | Bug 9 | Empty slice access |

### CHECK Tests (Possibly Fixed)

| Test | Bug | Reason |
|------|-----|--------|
| `TestEnhancedIPRFInverseSpace` | Bug 2 | May already be fixed in current code |

### PASS Tests (No Issues Found)

| Test | Bug | Reason |
|------|-----|--------|
| `TestCacheModeEffectiveness` | Bug 7 | Validates cache implementation |

## Test Categories

### Correctness Tests
- PRP bijection properties (inverse, no collisions, surjection)
- PMNS round-trip correctness
- Forward-inverse composition
- Distribution properties
- Determinism

### Performance Tests
- Forward evaluation latency/throughput
- Inverse evaluation complexity
- PRP inverse performance
- Scaling behavior (O(n) detection)
- Worst-case scenarios

### Edge Case Tests
- Empty domains (n=0, m=0)
- Single element (n=1, m=1)
- Unequal ratios (n>>m, m>>n)
- Boundary values (0, n-1, m-1)
- Power-of-2 domains

### Integration Tests
- Cache effectiveness
- Batch operations
- Concurrent access
- System-level integration
- Memory usage

## Test Metrics

### Coverage Goals
- **Line Coverage**: >90% of iPRF implementation
- **Branch Coverage**: >85% of conditional logic
- **Function Coverage**: 100% of public API

### Performance Targets
- Forward: <100µs per operation
- Inverse (n=8.4M, m=1024): <5s per operation
- Expected preimage size: ~8,200 elements
- Memory per inverse: <100KB

## Next Steps: GREEN Phase

After running these tests and confirming failures:

1. **Fix Bug 1**: Implement correct cycle-walking PRP
2. **Fix Bug 3**: Replace brute-force with efficient inverse (Feistel or table)
3. **Fix Bug 4**: Use `ballCount` instead of `n` in binomial sampling
4. **Fix Bug 5**: Use proper node encoding with sufficient bits
5. **Fix Bug 8**: Complete bin collection recursion
6. **Fix Bug 9**: Add bounds checking in GetDistributionStats
7. **Fix Bug 10**: Return error instead of 0 for "not found"
8. **Verify Bug 2**: Check if already fixed
9. **Implement Bug 7**: Add proper cache checking

## REFACTOR Phase

After all tests pass (GREEN):

1. **Optimize performance**: Ensure O(log m + k) inverse complexity
2. **Add test utilities**: Helpers for common testing patterns
3. **Enhance coverage**: Add more edge cases
4. **Improve documentation**: Add examples and usage guides
5. **Security review**: Validate cryptographic properties

## Test Execution Checklist

- [ ] Run all tests: `go test -v`
- [ ] Document failures in test results report
- [ ] Categorize failures by bug number
- [ ] Verify test coverage: `go test -cover`
- [ ] Run race detector: `go test -race`
- [ ] Run benchmarks: `go test -bench=.`
- [ ] Create bug fix plan based on test failures
- [ ] Proceed to GREEN phase (bug fixing)

## Mathematical Properties Tested

From the academic paper specification:

1. **PRP Bijection**: P: [n] → [n] is a bijection
2. **PMNS Correctness**: S^-1(k, y) = {x ∈ [n] | S(k, x) = y}
3. **iPRF Composition**: iF.F^-1((k1,k2), y) = {P^-1(k1, x) : x ∈ S^-1(k2, y)}
4. **Distribution**: Chi-squared test for multinomial distribution
5. **Determinism**: Same key produces same outputs
6. **Performance**: O(log m + k) where k is preimage size

## Research Context

These tests are based on:
- Original Plinko paper specification
- Comprehensive bug analysis identifying 10 critical issues
- QA approach with 6 testing phases
- Academic cryptographic correctness requirements

## Contact & Support

For questions about the test suite:
- See research documentation in `.taskmaster/docs/research/`
- Review bug analysis report
- Check individual test documentation in source files

---

**TDD Methodology**: RED → GREEN → REFACTOR
**Current Phase**: RED (Write failing tests)
**Next Phase**: GREEN (Fix implementations)
