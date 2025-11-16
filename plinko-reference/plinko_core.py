"""
Core Plinko PIR server logic.

Implements the main PIR operations and query processing.
"""

import logging
import time
from typing import Dict, Any, List

from database import PlinkoDatabase
from prset import PRSet, PrfKey128
from utils import validate_index, validate_prf_key, compute_xor, Timer

logger = logging.getLogger(__name__)


class PlinkoPIRServer:
    """
    Core Plinko PIR server implementation.
    
    Provides the main PIR operations with privacy guarantees.
    """
    
    def __init__(self, database: PlinkoDatabase):
        """
        Initialize the PIR server with a loaded database.
        
        Args:
            database: Loaded PlinkoDatabase instance
        """
        self.database = database
        self.db_size = database.get_size()
        
        # Calculate chunk and set sizes (simplified version)
        # In a full implementation, these would be configurable or calculated
        # based on security parameters
        self.chunk_size = max(1, self.db_size // 1024)
        self.set_size = max(1, self.db_size // self.chunk_size)
        
        logger.info(f"Server initialized with {self.db_size} entries")
        logger.info(f"Chunk size: {self.chunk_size}, Set size: {self.set_size}")
    
    def health_check(self) -> Dict[str, Any]:
        """
        Health check endpoint.
        
        Returns:
            Health status information
        """
        return {
            "status": "healthy",
            "database_loaded": self.db_size > 0,
            "database_size": self.db_size,
            "chunk_size": self.chunk_size,
            "set_size": self.set_size
        }
    
    def plaintext_query(self, index: int) -> Dict[str, Any]:
        """
        Process a plaintext query for a specific database index.
        
        Args:
            index: Database index to query
            
        Returns:
            Query result with value and timing information
            
        Note: For security, this method does NOT log the queried index.
        """
        start_time = time.time()
        
        try:
            # Validate index (without logging it for privacy)
            validate_index(index, self.db_size)
            
            # Retrieve value
            value = self.database.get_entry(index)
            
            end_time = time.time()
            server_time_nanos = int((end_time - start_time) * 1_000_000_000)
            
            return {
                "value": value,
                "server_time_nanos": server_time_nanos
            }
            
        except Exception as e:
            logger.error(f"Plaintext query failed: {e}")
            raise
    
    def full_set_query(self, prf_key: bytes) -> Dict[str, Any]:
        """
        Process a full set query using a PRF key.
        
        Args:
            prf_key: 16-byte PRF key for set generation
            
        Returns:
            Query result with XOR of all values in the pseudorandom set
        """
        start_time = time.time()
        
        try:
            # Validate PRF key
            validate_prf_key(prf_key)
            
            # Create PRSet with the provided key
            key = PrfKey128(prf_key)
            prset = PRSet(key)
            
            # Generate pseudorandom set of indices
            indices = prset.expand(self.set_size, self.chunk_size)
            
            # Validate indices are within bounds
            for idx in indices:
                validate_index(idx, self.db_size)
            
            # Compute XOR of all values in the set
            result = compute_xor([self.database.get_entry(idx) for idx in indices])
            
            end_time = time.time()
            server_time_nanos = int((end_time - start_time) * 1_000_000_000)
            
            return {
                "value": result,
                "server_time_nanos": server_time_nanos
            }
            
        except Exception as e:
            logger.error(f"Full set query failed: {e}")
            raise
    
    def set_parity_query(self, indices: List[int]) -> Dict[str, Any]:
        """
        Process a set parity query for specific indices.
        
        Args:
            indices: List of database indices to query
            
        Returns:
            Query result with XOR of all specified values
        """
        start_time = time.time()
        
        try:
            # Validate indices
            if not indices:
                raise ValueError("Indices list cannot be empty")
            
            for idx in indices:
                validate_index(idx, self.db_size)
            
            # Compute XOR of all specified values
            result = compute_xor([self.database.get_entry(idx) for idx in indices])
            
            end_time = time.time()
            server_time_nanos = int((end_time - start_time) * 1_000_000_000)
            
            return {
                "parity": result,
                "server_time_nanos": server_time_nanos
            }
            
        except Exception as e:
            logger.error(f"Set parity query failed: {e}")
            raise
    
    def get_database_info(self) -> Dict[str, Any]:
        """
        Get database information.
        
        Returns:
            Database size and configuration information
        """
        return {
            "size": self.db_size,
            "size_mb": self.database.get_size_mb(),
            "chunk_size": self.chunk_size,
            "set_size": self.set_size
        }