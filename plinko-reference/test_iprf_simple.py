"""
Simple test runner for iPRF implementation validation.

Tests core algorithmic properties without pytest dependency.
Validates iPRF construction per Plinko paper specification.
"""

import sys
import time
import traceback


def test_iprf_import():
    """Test iPRF can be imported."""
    try:
        from iprf import IPRF, encode_node, derive_iprf_key
        return True, "Import successful"
    except Exception as e:
        return False, f"Import failed: {e}\n{traceback.format_exc()}"


def test_iprf_creation():
    """Test iPRF creation."""
    try:
        from iprf import IPRF
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=1000, range_size=100)

        assert iprf.domain == 1000
        assert iprf.range_size == 100
        return True, "iPRF created successfully"
    except Exception as e:
        return False, f"Creation failed: {e}\n{traceback.format_exc()}"


def test_forward_basic():
    """Test forward evaluation."""
    try:
        from iprf import IPRF
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=1000, range_size=100)

        for x in [0, 1, 10, 100, 500, 999]:
            y = iprf.forward(x)
            assert 0 <= y < 100, f"forward({x}) = {y} out of range"

        return True, "Forward evaluation works"
    except Exception as e:
        return False, f"Forward failed: {e}\n{traceback.format_exc()}"


def test_inverse_correctness():
    """Test inverse returns correct preimages (correctness property)."""
    try:
        from iprf import IPRF
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=1000, range_size=100)

        # Test inverse for a few bins
        for y in [0, 10, 50, 99]:
            preimages = iprf.inverse(y)

            # All preimages should map back to y
            for x in preimages:
                y_computed = iprf.forward(x)
                assert y_computed == y, \
                    f"Inverse correctness violated: x={x} in inverse({y}) but forward({x}) = {y_computed}"

        return True, f"Inverse correctness verified ({len(preimages)} preimages in last bin)"
    except Exception as e:
        return False, f"Inverse failed: {e}\n{traceback.format_exc()}"


def test_inverse_performance():
    """Test inverse achieves O(log m + k) complexity via tree enumeration."""
    try:
        from iprf import IPRF
        key = b'0123456789abcdef'
        iprf = IPRF(key=key, domain=100000, range_size=1000)

        start = time.time()
        preimages = iprf.inverse(500)
        elapsed = time.time() - start

        # Should be < 10ms for O(log m + k) tree traversal
        assert elapsed < 0.01, f"Performance not O(log m + k): {elapsed*1000:.2f}ms"

        return True, f"Inverse O(log m + k): {elapsed*1000:.2f}ms for domain=100000"
    except Exception as e:
        return False, f"Performance test failed: {e}\n{traceback.format_exc()}"


def test_node_encoding_no_collisions():
    """Test SHA-256 node encoding provides collision-free identifiers."""
    try:
        from iprf import encode_node

        # Test large n values > 2^16
        seen_ids = set()
        for n in [100000, 1000000]:
            for low in [0, 1, 100]:
                for high in [low + 10, low + 100]:
                    node_id = encode_node(low, high, n)

                    assert node_id not in seen_ids, \
                        f"Collision detected for n={n}"

                    seen_ids.add(node_id)

        return True, f"No collisions detected ({len(seen_ids)} unique node IDs)"
    except Exception as e:
        return False, f"Node encoding test failed: {e}\n{traceback.format_exc()}"


def test_deterministic_key_derivation():
    """Test PRF-based deterministic key derivation (Section 5.2)."""
    try:
        from iprf import derive_iprf_key

        master_secret = b'my-master-secret'
        context = 'plinko-iprf-v1'

        key1 = derive_iprf_key(master_secret, context)
        key2 = derive_iprf_key(master_secret, context)

        assert key1 == key2, "Key derivation not deterministic"
        assert len(key1) == 16, f"Wrong key length: {len(key1)}"

        # Different context should give different key
        key3 = derive_iprf_key(master_secret, 'different')
        assert key1 != key3, "Different contexts should produce different keys"

        return True, "Key derivation is deterministic"
    except Exception as e:
        return False, f"Key derivation failed: {e}\n{traceback.format_exc()}"


def test_table_prp_import():
    """Test TablePRP can be imported."""
    try:
        from table_prp import TablePRP
        return True, "TablePRP import successful"
    except Exception as e:
        return False, f"TablePRP import failed: {e}\n{traceback.format_exc()}"


def test_table_prp_bijection():
    """Test TablePRP implements perfect bijection via Fisher-Yates."""
    try:
        from table_prp import TablePRP

        key = b'0123456789abcdef'
        prp = TablePRP(domain=1000, key=key)

        # Test no collisions
        outputs = set()
        for x in range(1000):
            y = prp.forward(x)
            assert y not in outputs, f"Collision at output {y}"
            outputs.add(y)

        # Test all outputs reachable
        assert len(outputs) == 1000, "Not all outputs reachable"

        # Test round-trip
        for x in [0, 1, 100, 500, 999]:
            y = prp.forward(x)
            x_recovered = prp.inverse(y)
            assert x == x_recovered, f"Round-trip failed: {x} → {y} → {x_recovered}"

        return True, "TablePRP bijection verified"
    except Exception as e:
        return False, f"TablePRP bijection test failed: {e}\n{traceback.format_exc()}"


def test_table_prp_inverse_performance():
    """Test TablePRP inverse achieves O(1) complexity via table lookup."""
    try:
        from table_prp import TablePRP

        key = b'0123456789abcdef'
        prp = TablePRP(domain=10000, key=key)

        start = time.time()
        for y in [0, 100, 1000, 5000, 9999]:
            prp.inverse(y)
        elapsed = time.time() - start

        # Should be < 1ms for O(1) lookups
        assert elapsed < 0.001, f"Too slow: {elapsed*1000:.2f}ms"

        return True, f"TablePRP inverse fast: {elapsed*1000:.2f}ms for 5 lookups"
    except Exception as e:
        return False, f"TablePRP performance test failed: {e}\n{traceback.format_exc()}"


def run_tests():
    """Run all tests and report results."""
    tests = [
        ("Import iPRF", test_iprf_import),
        ("Create iPRF", test_iprf_creation),
        ("Forward evaluation", test_forward_basic),
        ("Inverse correctness", test_inverse_correctness),
        ("Inverse performance O(log m + k)", test_inverse_performance),
        ("Node encoding collision-free", test_node_encoding_no_collisions),
        ("Key derivation deterministic", test_deterministic_key_derivation),
        ("Import TablePRP", test_table_prp_import),
        ("TablePRP bijection property", test_table_prp_bijection),
        ("TablePRP inverse O(1)", test_table_prp_inverse_performance),
    ]

    print("=" * 70)
    print("iPRF AND TablePRP CORRECTNESS TESTS")
    print("=" * 70)
    print()

    passed = 0
    failed = 0

    for name, test_func in tests:
        print(f"Testing: {name}...", end=" ")
        try:
            success, message = test_func()
            if success:
                print(f"✓ PASS - {message}")
                passed += 1
            else:
                print(f"✗ FAIL")
                print(f"  Error: {message}")
                failed += 1
        except Exception as e:
            print(f"✗ FAIL (exception)")
            print(f"  {e}")
            traceback.print_exc()
            failed += 1

        print()

    print("=" * 70)
    print(f"Results: {passed} passed, {failed} failed")
    print("=" * 70)

    return failed == 0


if __name__ == "__main__":
    success = run_tests()
    sys.exit(0 if success else 1)
