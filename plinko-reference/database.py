"""
Database management for Plinko PIR server.

Handles loading and managing the canonical database snapshot
for private information retrieval operations.
"""

import struct
import os
import time
import logging

logger = logging.getLogger(__name__)


class DatabaseError(Exception):
    """Exception raised for database-related errors."""
    pass


class PlinkoDatabase:
    """
    Manages the canonical database for Plinko PIR operations.
    
    Loads binary database files and provides access to entries.
    Provides secure and efficient database access.
    """
    
    def __init__(self, db_path: str):
        """
        Initialize database manager.
        
        Args:
            db_path: Path to the binary database file
        """
        self.db_path = db_path
        self.data = []
        self.size = 0
        self.entry_size = 8  # uint64 = 8 bytes
        
    def load(self, timeout: int = 0) -> None:
        """
        Load the database from file with optional timeout.
        
        Args:
            timeout: Maximum time to wait for database file (seconds)
                     0 means no timeout (wait indefinitely)
                     
        Raises:
            DatabaseError: If database cannot be loaded
        """
        logger.info(f"Loading database from {self.db_path}")
        
        # Wait for database file if needed
        self._wait_for_database(timeout)
        
        try:
            with open(self.db_path, 'rb') as f:
                # Read entire file
                binary_data = f.read()
                
            if len(binary_data) % self.entry_size != 0:
                raise DatabaseError(
                    f"Database file size {len(binary_data)} is not a multiple of "
                    f"entry size {self.entry_size}"
                )
            
            # Parse uint64 entries (big-endian for consistency)
            self.size = len(binary_data) // self.entry_size
            self.data = []
            
            for i in range(self.size):
                offset = i * self.entry_size
                entry_bytes = binary_data[offset:offset + self.entry_size]
                entry = struct.unpack('>Q', entry_bytes)[0]  # Big-endian uint64 for consistency
                self.data.append(entry)
            
            logger.info(f"Database loaded successfully: {self.size} entries")
            
        except FileNotFoundError:
            raise DatabaseError(f"Database file not found: {self.db_path}")
        except Exception as e:
            raise DatabaseError(f"Failed to load database: {e}")
    
    def _wait_for_database(self, timeout: int) -> None:
        """
        Wait for database file to exist with optional timeout.
        
        Args:
            timeout: Maximum time to wait in seconds (0 = wait indefinitely)
        """
        if timeout <= 0:
            # No timeout - just check once
            if not os.path.exists(self.db_path):
                raise DatabaseError(f"Database file {self.db_path} not found")
            return
        
        start_time = time.time()
        attempts = 0
        
        while True:
            if os.path.exists(self.db_path):
                logger.info("Database file found")
                return
            
            attempts += 1
            if attempts % 10 == 0:
                elapsed = int(time.time() - start_time)
                logger.info(f"Still waiting for database... ({elapsed}s/{timeout}s)")
            
            if time.time() - start_time >= timeout:
                raise DatabaseError(f"Timeout waiting for database file at {self.db_path}")
            
            time.sleep(1)
    
    def get_entry(self, index: int) -> int:
        """
        Get a database entry by index.
        
        Args:
            index: Database index (0-based)
            
        Returns:
            The uint64 value at the specified index
            
        Raises:
            IndexError: If index is out of bounds
        """
        if index < 0 or index >= self.size:
            raise IndexError(f"Database index {index} out of bounds [0, {self.size})")
        
        return self.data[index]
    
    def get_size(self) -> int:
        """Get the number of entries in the database."""
        return self.size
    
    def get_size_mb(self) -> float:
        """Get the database size in megabytes."""
        return (self.size * self.entry_size) / (1024 * 1024)
    
    def __len__(self):
        return self.size
    
    def __getitem__(self, index):
        return self.get_entry(index)
    
    def __repr__(self):
        return f"PlinkoDatabase(path='{self.db_path}', size={self.size})"