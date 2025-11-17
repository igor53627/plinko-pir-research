"""
TDD Test Suite for TablePRP Implementation

Tests verify perfect bijection property with O(1) forward and inverse operations.
Validates Fisher-Yates shuffle correctness for PRP construction.
"""

import pytest
import time
from table_prp import TablePRP, DeterministicRNG


class TestTablePRPCore:
    """Test core TablePRP functionality."""

    def test_table_prp_creation(self):
        """Test TablePRP can be created with valid parameters."""
        key = b'0123456789abcdef'
        prp = TablePRP(domain=1000, key=key)

        assert prp.domain == 1000
        assert prp.key == key

    def test_table_prp_invalid_domain(self):
        """Test TablePRP rejects zero domain."""
        key = b'0123456789abcdef'

        with pytest.raises(ValueError, match="Domain size cannot be zero"):
            TablePRP(domain=0, key=key)

    def test_table_prp_invalid_key(self):
        """Test TablePRP rejects invalid key."""
        with pytest.raises(ValueError, match="Key must be 16 bytes"):
            TablePRP(domain=1000, key=b'short')

    def test_forward_in_range(self):
        """Test forward produces outputs in valid range."""
        key = b'0123456789abcdef'
        prp = TablePRP(domain=100, key=key)

        for x in range(100):
            y = prp.forward(x)
            assert 0 <= y < 100, f"forward({x}) = {y} out of range [0, 100)"

    def test_forward_deterministic(self):
        """Test forward is deterministic."""
        key = b'0123456789abcdef'
        prp1 = TablePRP(domain=1000, key=key)
        prp2 = TablePRP(domain=1000, key=key)

        for x in [0, 1, 10, 100, 500, 999]:
            y1 = prp1.forward(x)
            y2 = prp2.forward(x)
            assert y1 == y2, f"forward({x}) not deterministic: {y1} != {y2}"


class TestTablePRPBijection:
    """
    Test TablePRP implements perfect bijection via Fisher-Yates shuffle.

    Validates injectivity (no collisions), surjectivity (all outputs reachable),
    and round-trip consistency (inverse correctness).
    """

    def test_bijection_no_collisions(self):
        """Test forward is injective (no collisions)."""
        key = b'0123456789abcdef'
        prp = TablePRP(domain=1000, key=key)

        outputs = set()
        for x in range(1000):
            y = prp.forward(x)
            assert y not in outputs, f"Collision detected: multiple inputs map to {y}"
            outputs.add(y)

    def test_bijection_surjective(self):
        """Test forward is surjective (all outputs reachable)."""
        key = b'0123456789abcdef'
        prp = TablePRP(domain=1000, key=key)

        outputs = set()
        for x in range(1000):
            y = prp.forward(x)
            outputs.add(y)

        # All values [0, 1000) should be reachable
        assert outputs == set(range(1000)), "Not all outputs reachable (not surjective)"

    def test_inverse_correctness(self):
        """
        Test inverse correctness (inverse(forward(x)) = x).

        O(1) table lookup ensures exact inverse via pre-computed mapping.
        """
        key = b'0123456789abcdef'
        prp = TablePRP(domain=1000, key=key)

        # Test round-trip for all inputs
        for x in range(1000):
            y = prp.forward(x)
            x_recovered = prp.inverse(y)
            assert x == x_recovered, f"Round-trip failed: {x} → {y} → {x_recovered}"

    def test_verify_bijection(self):
        """Test bijection verification method."""
        key = b'0123456789abcdef'
        prp = TablePRP(domain=1000, key=key)

        assert prp.verify_bijection(), "Bijection verification failed"


class TestTablePRPPerformance:
    """
    Test inverse achieves O(1) complexity via pre-computed table lookup.
    """

    def test_inverse_performance(self):
        """Test inverse achieves O(1) complexity (constant-time lookup)."""
        key = b'0123456789abcdef'
        prp = TablePRP(domain=10000, key=key)

        # Inverse should be instant (< 1ms for O(1) lookup)
        start = time.time()
        for y in [0, 100, 1000, 5000, 9999]:
            prp.inverse(y)
        elapsed = time.time() - start

        assert elapsed < 0.001, f"Inverse too slow: {elapsed*1000:.2f}ms (expected < 1ms)"

    def test_inverse_constant_time(self):
        """Test inverse time doesn't grow with domain size."""
        key = b'0123456789abcdef'

        times = []
        for domain in [1000, 5000, 10000]:
            prp = TablePRP(domain=domain, key=key)

            start = time.time()
            prp.inverse(0)
            elapsed = time.time() - start

            times.append(elapsed)

        # O(1) lookup should not scale with domain size
        # Allow 2x variance due to system noise
        assert times[-1] < times[0] * 2, \
            f"Inverse time scaling suggests O(n) not O(1): {times}"


class TestDeterministicRNG:
    """Test deterministic RNG for Fisher-Yates shuffle."""

    def test_rng_deterministic(self):
        """Test RNG produces same sequence for same key."""
        key = b'0123456789abcdef'

        rng1 = DeterministicRNG(key)
        rng2 = DeterministicRNG(key)

        for _ in range(100):
            r1 = rng1.uint64()
            r2 = rng2.uint64()
            assert r1 == r2, "RNG not deterministic"

    def test_rng_different_keys(self):
        """Test different keys produce different sequences."""
        key1 = b'0123456789abcdef'
        key2 = b'fedcba9876543210'

        rng1 = DeterministicRNG(key1)
        rng2 = DeterministicRNG(key2)

        # Sequences should differ
        same_count = 0
        for _ in range(100):
            r1 = rng1.uint64()
            r2 = rng2.uint64()
            if r1 == r2:
                same_count += 1

        assert same_count < 10, "Different keys producing too similar sequences"

    def test_rng_uint64_n_uniform(self):
        """Test uint64_n produces uniform distribution."""
        key = b'0123456789abcdef'
        rng = DeterministicRNG(key)

        n = 10
        counts = {i: 0 for i in range(n)}

        # Generate many samples
        for _ in range(10000):
            r = rng.uint64_n(n)
            assert 0 <= r < n, f"uint64_n({n}) returned {r} out of range"
            counts[r] += 1

        # Check distribution is roughly uniform
        expected = 10000 / n
        for i in range(n):
            # Allow 30% deviation from expected
            assert abs(counts[i] - expected) < expected * 0.3, \
                f"Distribution not uniform: bucket {i} has {counts[i]} (expected ~{expected})"

    def test_rng_uint64_n_edge_cases(self):
        """Test uint64_n handles edge cases."""
        key = b'0123456789abcdef'
        rng = DeterministicRNG(key)

        # n = 0 should return 0
        assert rng.uint64_n(0) == 0

        # n = 1 should return 0
        assert rng.uint64_n(1) == 0

        # Power of 2 should work
        for _ in range(10):
            r = rng.uint64_n(256)
            assert 0 <= r < 256


class TestTablePRPDifferentKeys:
    """Test different keys produce different permutations."""

    def test_different_keys_different_permutations(self):
        """Test different keys produce different permutations."""
        key1 = b'0123456789abcdef'
        key2 = b'fedcba9876543210'

        prp1 = TablePRP(domain=100, key=key1)
        prp2 = TablePRP(domain=100, key=key2)

        # Count how many outputs differ
        different = 0
        for x in range(100):
            y1 = prp1.forward(x)
            y2 = prp2.forward(x)
            if y1 != y2:
                different += 1

        # Most outputs should differ (> 90%)
        assert different > 90, \
            f"Different keys producing too similar permutations: only {different}/100 differ"
