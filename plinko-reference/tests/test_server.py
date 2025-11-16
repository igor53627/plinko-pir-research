"""
Tests for Plinko PIR server core functionality.
"""

import pytest
import tempfile
import struct
import os
from plinko_reference.database import PlinkoDatabase
from plinko_reference.plinko_core import PlinkoPIRServer
from plinko_reference.prset import PrfKey128


class TestPlinkoPIRServer:
    """Test cases for Plinko PIR server functionality."""
    
    @pytest.fixture
    def test_database(self):
        """Create a test database."""
        # Create temporary database file
        with tempfile.NamedTemporaryFile(delete=False) as f:
            # Write some test data (uint64 values)
            test_data = [i * 1000 for i in range(1000)]  # 1000 entries
            for value in test_data:
                f.write(struct.pack('>Q', value))  # Big-endian uint64
            
            db_path = f.name
        
        # Create database instance
        database = PlinkoDatabase(db_path)
        database.load()
        
        yield database
        
        # Cleanup
        os.unlink(db_path)
    
    @pytest.fixture
    def server(self, test_database):
        """Create a test server instance."""
        return PlinkoPIRServer(test_database)
    
    def test_server_creation(self, server):
        """Test server creation and initialization."""
        assert server.db_size == 1000
        assert server.chunk_size > 0
        assert server.set_size > 0
    
    def test_health_check(self, server):
        """Test health check functionality."""
        health = server.health_check()
        
        assert health["status"] == "healthy"
        assert health["database_loaded"] is True
        assert health["database_size"] == 1000
        assert "chunk_size" in health
        assert "set_size" in health
    
    def test_plaintext_query_valid(self, server):
        """Test valid plaintext queries."""
        # Query first entry
        result = server.plaintext_query(0)
        assert result["value"] == 0  # First entry should be 0
        assert result["server_time_nanos"] > 0
        
        # Query middle entry
        result = server.plaintext_query(500)
        assert result["value"] == 500000  # 500 * 1000
        
        # Query last entry
        result = server.plaintext_query(999)
        assert result["value"] == 999000  # 999 * 1000
    
    def test_plaintext_query_invalid(self, server):
        """Test invalid plaintext queries."""
        # Negative index
        with pytest.raises(ValueError):
            server.plaintext_query(-1)
        
        # Index too large
        with pytest.raises(ValueError):
            server.plaintext_query(1000)  # db_size is 1000, max index is 999
        
        # Non-integer (would be caught by validate_index)
        # This would be caught at the API level in practice
    
    def test_set_parity_query(self, server):
        """Test set parity queries."""
        # Test with single index
        result = server.set_parity_query([0])
        assert result["parity"] == 0  # 0 XOR nothing = 0
        assert result["server_time_nanos"] > 0
        
        # Test with two indices
        result = server.set_parity_query([0, 1])
        expected = 0 ^ 1000  # First two values: 0 and 1000
        assert result["parity"] == expected
        
        # Test with multiple indices
        indices = [0, 1, 2, 3, 4]
        result = server.set_parity_query(indices)
        expected = 0 ^ 1000 ^ 2000 ^ 3000 ^ 4000
        assert result["parity"] == expected
    
    def test_set_parity_query_empty(self, server):
        """Test set parity query with empty list."""
        with pytest.raises(ValueError, match="cannot be empty"):
            server.set_parity_query([])
    
    def test_set_parity_query_invalid_indices(self, server):
        """Test set parity query with invalid indices."""
        with pytest.raises(ValueError):
            server.set_parity_query([-1, 0, 1])
        
        with pytest.raises(ValueError):
            server.set_parity_query([0, 1, 1000])  # 1000 is out of bounds
    
    def test_full_set_query(self, server):
        """Test full set queries."""
        # Create a deterministic PRF key
        prf_key = b'\x00' * 16
        
        result = server.full_set_query(prf_key)
        
        assert "value" in result
        assert "server_time_nanos" in result
        assert result["server_time_nanos"] > 0
        
        # The result should be deterministic for the same key
        result2 = server.full_set_query(prf_key)
        assert result2["value"] == result["value"]
    
    def test_full_set_query_invalid_key(self, server):
        """Test full set query with invalid PRF key."""
        # Wrong key length
        with pytest.raises(ValueError, match="must be 16 bytes"):
            server.full_set_query(b'\x00' * 15)  # Too short
        
        with pytest.raises(ValueError, match="must be 16 bytes"):
            server.full_set_query(b'\x00' * 17)  # Too long
    
    def test_database_info(self, server):
        """Test database information retrieval."""
        info = server.get_database_info()
        
        assert info["size"] == 1000
        assert info["size_mb"] > 0
        assert "chunk_size" in info
        assert "set_size" in info
        assert info["chunk_size"] > 0
        assert info["set_size"] > 0
    
    def test_timing_consistency(self, server):
        """Test that timing measurements are reasonable."""
        # Plaintext query should be fast
        result = server.plaintext_query(500)
        assert 0 < result["server_time_nanos"] < 1_000_000_000  # Less than 1 second
        
        # Set operations might be slower but should still be reasonable
        result = server.set_parity_query([0, 1, 2, 3, 4])
        assert 0 < result["server_time_nanos"] < 1_000_000_000


class TestDatabaseOperations:
    """Test database-specific operations."""
    
    def test_database_loading(self):
        """Test database loading and validation."""
        # Create test database file
        with tempfile.NamedTemporaryFile(delete=False) as f:
            # Write invalid size (not multiple of 8)
            f.write(b'\x00' * 15)  # 15 bytes - invalid
            db_path = f.name
        
        try:
            database = PlinkoDatabase(db_path)
            with pytest.raises(ValueError, match="not a multiple of"):
                database.load()
        finally:
            os.unlink(db_path)
    
    def test_database_bounds_checking(self, test_database):
        """Test database bounds checking."""
        server = PlinkoPIRServer(test_database)
        
        # Valid indices should work
        server.plaintext_query(0)
        server.plaintext_query(999)
        
        # Invalid indices should fail
        with pytest.raises(ValueError):
            server.plaintext_query(-1)
        
        with pytest.raises(ValueError):
            server.plaintext_query(1000)


if __name__ == '__main__':
    pytest.main([__file__, '-v'])