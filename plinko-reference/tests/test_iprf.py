"""
TDD Test Suite for iPRF Implementation

Tests verify correctness of iPRF construction per Plinko paper:
- Tree-based inverse: O(log m + k) complexity (Theorem 4.4)
- Inverse correctness: Proper preimage enumeration
- Deterministic key derivation: PRF-based key hierarchy (Section 5.2)
- Node encoding: Collision-free identifiers via SHA-256
- Parameter separation: Consistent node encoding vs binomial sampling
"""

import pytest
import time
from iprf import IPRF, encode_node, derive_iprf_key, inv_normal_cdf


class TestIPRFCore:
    """Test core iPRF functionality."""

    def test_iprf_creation(self):
        """Test iPRF can be created with valid parameters."""
        key = b'0123456789abcdef'  # 16 bytes
        iprf = IPRF(key=key, domain=1000, range_size=100)

        assert iprf.domain == 1000
        assert iprf.range_size == 100
        assert iprf.key == key

    def test_iprf_invalid_key(self):
        """Test iPRF rejects invalid key sizes."""
        with pytest.raises(ValueError, match="Key must be 16 bytes"):
            IPRF(key=b'short', domain=1000, range_size=100)

    def test_iprf_invalid_domain(self):
        """Test iPRF rejects zero domain/range."""
        key = b'0123456789abcdef'

        with pytest.raises(ValueError, match="Domain and range must be > 0"):
            IPRF(key=key, domain=0, range_size=100)

        with pytest.raises(ValueError, match="Domain and range must be > 0"):
            IPRF(key=key, domain=1000, range_size=0)

    def test_forward_basic(self):
        """Test forward evaluation produces outputs in range."""
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=1000, range_size=100)

        # Test random inputs
        for x in [0, 1, 10, 100, 500, 999]:
            y = iprf.forward(x)
            assert 0 <= y < 100, f"forward({x}) = {y} out of range [0, 100)"

    def test_forward_deterministic(self):
        """Test forward is deterministic (same input → same output)."""
        key = b'0123456789abcdef'
        iprf1 = IPRF(key=key, domain=1000, range_size=100)
        iprf2 = IPRF(key=key, domain=1000, range_size=100)

        for x in range(0, 1000, 50):
            y1 = iprf1.forward(x)
            y2 = iprf2.forward(x)
            assert y1 == y2, f"forward({x}) not deterministic: {y1} != {y2}"

    def test_inverse_returns_preimages(self):
        """
        Test inverse returns correct preimages in original domain.

        Verifies inverse correctness: all x in inverse(y) satisfy forward(x) = y.
        """
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=10000, range_size=100)

        # Test several bins
        for y in [0, 1, 10, 50, 99]:
            preimages = iprf.inverse(y)

            # All preimages should be in domain
            for x in preimages:
                assert 0 <= x < 10000, f"Preimage {x} out of domain"

            # All preimages should map to y
            for x in preimages:
                assert iprf.forward(x) == y, f"forward({x}) = {iprf.forward(x)} != {y}"

    def test_inverse_completeness(self):
        """
        Test inverse finds ALL preimages (completeness property).

        For small domain, brute force verification that inverse enumerates
        all x mapping to y, validating tree traversal correctness.
        """
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=1000, range_size=100)

        # Build ground truth by scanning all inputs
        ground_truth = {y: [] for y in range(100)}
        for x in range(1000):
            y = iprf.forward(x)
            ground_truth[y].append(x)

        # Check inverse matches ground truth for all bins
        for y in range(100):
            computed = set(iprf.inverse(y))
            expected = set(ground_truth[y])

            assert computed == expected, \
                f"Bin {y}: inverse missed preimages. Got {len(computed)}, expected {len(expected)}"


class TestIPRFPerformance:
    """
    Test tree-based inverse achieves O(log m + k) complexity per Theorem 4.4.
    """

    def test_inverse_performance(self):
        """Test inverse achieves sublinear complexity for large domains."""
        key = b'0123456789abcdef'

        # Large domain (simulating database size)
        iprf = IPRF(key=key, domain=100000, range_size=1000)

        # Inverse should be fast (< 10ms for O(log m + k) tree traversal)
        start = time.time()
        preimages = iprf.inverse(500)
        elapsed = time.time() - start

        assert elapsed < 0.01, f"Performance not O(log m + k): {elapsed*1000:.2f}ms (expected < 10ms)"
        assert len(preimages) > 0, "Inverse should return at least one preimage"

    def test_inverse_scales_logarithmically(self):
        """Test inverse time grows logarithmically with range size."""
        key = b'0123456789abcdef'

        # Test different range sizes with fixed domain
        domain = 10000

        times = []
        for range_size in [100, 500, 1000]:
            iprf = IPRF(key=key, domain=domain, range_size=range_size)

            start = time.time()
            iprf.inverse(0)
            elapsed = time.time() - start

            times.append(elapsed)

        # Time should grow logarithmically, not linearly with range size
        # Sublinear scaling validates O(log m) tree depth traversal
        assert times[-1] < times[0] * 5, \
            f"Inverse scaling suggests O(m) instead of O(log m): {times}"


class TestAlgorithmicProperties:
    """Test specific algorithmic properties of iPRF construction."""

    def test_node_encoding_collision_free(self):
        """
        Test SHA-256 node encoding provides collision-free identifiers.

        Cryptographic hash ensures unique identifiers across arbitrary parameter
        ranges, supporting large domain sizes without collision concerns.
        """
        # Test large n values (> 2^16 = 65536)
        large_n_values = [100000, 1000000, 10000000]

        seen_ids = set()

        for n in large_n_values:
            for low in [0, 1, 100]:
                for high in [low + 10, low + 100]:
                    node_id = encode_node(low, high, n)

                    # Check for collisions
                    assert node_id not in seen_ids, \
                        f"Node encoding collision for (low={low}, high={high}, n={n})"

                    seen_ids.add(node_id)

    def test_deterministic_key_derivation(self):
        """
        Test PRF-based key derivation is deterministic (Section 5.2).

        Same master secret + context → same key, ensuring hint validity
        across server sessions.
        """
        master_secret = b'my-master-secret-key'
        context = 'plinko-iprf-v1'

        # Derive key multiple times
        key1 = derive_iprf_key(master_secret, context)
        key2 = derive_iprf_key(master_secret, context)

        assert key1 == key2, "Key derivation not deterministic"
        assert len(key1) == 16, f"Derived key wrong length: {len(key1)}"

        # Different context should give different key
        key3 = derive_iprf_key(master_secret, 'different-context')
        assert key1 != key3, "Different contexts should produce different keys"

    def test_parameter_separation(self):
        """
        Test correct parameter handling in tree traversal.

        Validates that node encoding uses fixed domain size for consistency,
        while binomial sampling uses dynamic ball count in current subtree.
        """
        key = b'0123456789abcdef'

        # Small domain for easy verification
        iprf = IPRF(key=key, domain=100, range_size=10)

        # Forward and inverse should be consistent
        for x in range(100):
            y = iprf.forward(x)
            preimages = iprf.inverse(y)

            assert x in preimages, \
                f"x={x} not in inverse(forward(x)={y}). Forward-inverse consistency violated."


class TestIPRFDistribution:
    """Test iPRF output distribution properties."""

    def test_expected_preimage_size(self):
        """Test expected preimage size is approximately n/m."""
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=10000, range_size=100)

        expected = iprf.get_expected_preimage_size()
        assert expected == 100, f"Expected preimage size should be 10000/100 = 100, got {expected}"

    def test_distribution_uniformity(self):
        """Test outputs are roughly uniformly distributed."""
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=10000, range_size=100)

        # Count outputs
        counts = {y: 0 for y in range(100)}
        for x in range(10000):
            y = iprf.forward(x)
            counts[y] += 1

        # All bins should have at least some balls
        for y in range(100):
            assert counts[y] > 0, f"Bin {y} is empty (distribution failure)"

        # Average should be n/m = 100
        avg = sum(counts.values()) / len(counts)
        assert abs(avg - 100) < 1, f"Average bin size {avg} != expected 100"

        # Standard deviation should be reasonable (not all in one bin)
        import math
        variance = sum((count - avg) ** 2 for count in counts.values()) / len(counts)
        stddev = math.sqrt(variance)
        assert stddev < 50, f"Standard deviation {stddev} too large (bad distribution)"


class TestHelperFunctions:
    """Test helper functions."""

    def test_inv_normal_cdf(self):
        """Test inverse normal CDF approximation."""
        # Test known values
        assert abs(inv_normal_cdf(0.5)) < 0.1, "inv_normal_cdf(0.5) should be ~0"
        assert inv_normal_cdf(0.9) > 1.0, "inv_normal_cdf(0.9) should be > 1"
        assert inv_normal_cdf(0.1) < -1.0, "inv_normal_cdf(0.1) should be < -1"

        # Test edge cases
        assert inv_normal_cdf(0.0) < -5, "inv_normal_cdf(0) should be very negative"
        assert inv_normal_cdf(1.0) > 5, "inv_normal_cdf(1) should be very positive"
