"""
Tests for PRSet (Pseudorandom Set) implementation.
"""

import pytest
import os
from plinko_reference.prset import PRSet, PrfKey128


class TestPRSet:
    """Test cases for PRSet functionality."""
    
    def test_prf_key_creation(self):
        """Test PrfKey128 creation and validation."""
        # Test with random key
        key1 = PrfKey128()
        assert len(key1.key_bytes) == 16
        
        # Test with specific key
        test_bytes = b'\x00' * 16
        key2 = PrfKey128(test_bytes)
        assert key2.key_bytes == test_bytes
        
        # Test invalid key length
        with pytest.raises(ValueError):
            PrfKey128(b'\x00' * 15)  # Too short
        
        with pytest.raises(ValueError):
            PrfKey128(b'\x00' * 17)  # Too long
    
    def test_prset_creation(self):
        """Test PRSet creation."""
        key = PrfKey128()
        prset = PRSet(key)
        assert prset.key == key
        assert prset.cipher is not None
        assert prset.encryptor is not None
    
    def test_prf_eval_mod(self):
        """Test PRF evaluation with modulus."""
        key = PrfKey128(b'\x00' * 16)  # Deterministic key
        prset = PRSet(key)
        
        # Test basic PRF evaluation
        result1 = prset.prf_eval_mod(0, 100)
        assert 0 <= result1 < 100
        
        result2 = prset.prf_eval_mod(1, 100)
        assert 0 <= result2 < 100
        assert result1 != result2  # Different inputs should give different outputs
        
        # Test with modulus 0
        assert prset.prf_eval_mod(5, 0) == 0
        
        # Test determinism
        result3 = prset.prf_eval_mod(0, 100)
        assert result3 == result1  # Same input should give same output
    
    def test_expand_basic(self):
        """Test basic set expansion."""
        key = PrfKey128(b'\x00' * 16)  # Deterministic key
        prset = PRSet(key)
        
        set_size = 10
        chunk_size = 100
        
        indices = prset.expand(set_size, chunk_size)
        
        assert len(indices) == set_size
        
        # Check that indices are in expected range
        for i, idx in enumerate(indices):
            expected_min = i * chunk_size
            expected_max = (i + 1) * chunk_size - 1
            assert expected_min <= idx <= expected_max
    
    def test_expand_determinism(self):
        """Test that expansion is deterministic."""
        key_bytes = os.urandom(16)
        key1 = PrfKey128(key_bytes)
        key2 = PrfKey128(key_bytes)
        
        prset1 = PRSet(key1)
        prset2 = PRSet(key2)
        
        set_size = 20
        chunk_size = 50
        
        indices1 = prset1.expand(set_size, chunk_size)
        indices2 = prset2.expand(set_size, chunk_size)
        
        assert indices1 == indices2  # Same key should give same results
    
    def test_expand_different_keys(self):
        """Test that different keys give different results."""
        key1 = PrfKey128(os.urandom(16))
        key2 = PrfKey128(os.urandom(16))
        
        prset1 = PRSet(key1)
        prset2 = PRSet(key2)
        
        set_size = 15
        chunk_size = 30
        
        indices1 = prset1.expand(set_size, chunk_size)
        indices2 = prset2.expand(set_size, chunk_size)
        
        # Different keys should give different results (with high probability)
        assert indices1 != indices2
    
    def test_expand_edge_cases(self):
        """Test edge cases for expansion."""
        key = PrfKey128(b'\x00' * 16)
        prset = PRSet(key)
        
        # Test with set_size = 1
        indices = prset.expand(1, 100)
        assert len(indices) == 1
        assert 0 <= indices[0] < 100
        
        # Test with chunk_size = 1
        indices = prset.expand(10, 1)
        assert len(indices) == 10
        for i, idx in enumerate(indices):
            assert idx == i  # Should be exactly i*1 + offset where offset is 0 or 1
    
    def test_repr(self):
        """Test string representation."""
        key = PrfKey128()
        prset = PRSet(key)
        
        repr_str = repr(prset)
        assert "PRSet" in repr_str
        assert "key_hash=" in repr_str


if __name__ == '__main__':
    pytest.main([__file__, '-v'])