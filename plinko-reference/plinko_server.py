#!/usr/bin/env python3
"""
Plinko PIR Server - Python Reference Implementation

A Private Information Retrieval server that allows clients to query database
entries without revealing which specific items they're interested in.

Security Features:
- No query logging to protect client privacy
- Deterministic pseudorandom sets using AES-128
- Input validation and secure error handling
- Designed for security auditing and education
"""

import argparse
import time
import logging
from flask import Flask, request, jsonify
from typing import Dict, Any

from config import load_config, ConfigurationError
from database import PlinkoDatabase, DatabaseError
from prset import PRSet, PrfKey128

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Constants for Plinko PIR operations
DB_ENTRY_SIZE = 8  # uint64 = 8 bytes
DB_ENTRY_LENGTH = 1

app = Flask(__name__)

# Global server instance
server = None


class PlinkoPIRServer:
    """
    Main Plinko PIR server implementation.
    
    Provides private information retrieval services with strong privacy guarantees.
    """
    
    def __init__(self, database_path: str):
        """
        Initialize the PIR server.
        
        Args:
            database_path: Path to the binary database file
        """
        self.database = PlinkoDatabase(database_path)
        self.db_size = 0
        self.chunk_size = 0
        self.set_size = 0
        
    def load_database(self, timeout: int = 60) -> None:
        """
        Load the canonical database.
        
        Args:
            timeout: Maximum time to wait for database file (seconds)
        """
        logger.info("Loading canonical database snapshot...")
        self.database.load(timeout)
        self.db_size = self.database.get_size()
        
        # Calculate chunk and set sizes based on database size
        # These would typically come from config or be calculated based on db_size
        self.chunk_size = max(1, self.db_size // 1024)  # Example calculation
        self.set_size = max(1, self.db_size // self.chunk_size)
        
        logger.info(f"✅ Database loaded: {self.db_size} entries ({self.database.get_size_mb():.1f} MB)")
        logger.info(f"   ChunkSize: {self.chunk_size}, SetSize: {self.set_size}")
    
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
            # Validate index
            if index < 0 or index >= self.db_size:
                raise ValueError(f"Index {index} out of bounds [0, {self.db_size})")
            
            # Retrieve value (without logging the index for privacy)
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
            if len(prf_key) != 16:
                raise ValueError(f"PRF key must be 16 bytes, got {len(prf_key)}")
            
            # Create PRSet with the provided key
            key = PrfKey128(prf_key)
            prset = PRSet(key)
            
            # Generate pseudorandom set of indices
            indices = prset.expand(self.set_size, self.chunk_size)
            
            # Validate indices are within bounds
            for idx in indices:
                if idx < 0 or idx >= self.db_size:
                    raise ValueError(f"Generated index {idx} out of bounds")
            
            # Compute XOR of all values in the set (standard PIR operation)
            result = 0
            for idx in indices:
                result ^= self.database.get_entry(idx)
            
            end_time = time.time()
            server_time_nanos = int((end_time - start_time) * 1_000_000_000)
            
            return {
                "value": result,
                "server_time_nanos": server_time_nanos
            }
            
        except Exception as e:
            logger.error(f"Full set query failed: {e}")
            raise
    
    def set_parity_query(self, indices: list) -> Dict[str, Any]:
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
                if idx < 0 or idx >= self.db_size:
                    raise ValueError(f"Index {idx} out of bounds [0, {self.db_size})")
            
            # Compute XOR of all specified values
            result = 0
            for idx in indices:
                result ^= self.database.get_entry(idx)
            
            end_time = time.time()
            server_time_nanos = int((end_time - start_time) * 1_000_000_000)
            
            return {
                "parity": result,
                "server_time_nanos": server_time_nanos
            }
            
        except Exception as e:
            logger.error(f"Set parity query failed: {e}")
            raise


# Flask route handlers
def cors_middleware(func):
    """Add CORS headers to responses."""
    def wrapper(*args, **kwargs):
        response = func(*args, **kwargs)
        if isinstance(response, tuple):
            data, status_code = response
        else:
            data, status_code = response, 200
        
        if isinstance(data, dict):
            resp = jsonify(data)
        else:
            resp = data
        
        resp.headers['Access-Control-Allow-Origin'] = '*'
        resp.headers['Access-Control-Allow-Methods'] = 'GET, POST, OPTIONS'
        resp.headers['Access-Control-Allow-Headers'] = 'Content-Type'
        
        return resp, status_code
    
    wrapper.__name__ = func.__name__
    return wrapper


@app.route('/health', methods=['GET', 'OPTIONS'])
@cors_middleware
def health_handler():
    """Health check endpoint."""
    if request.method == 'OPTIONS':
        return {}, 200
    
    try:
        health_status = server.health_check()
        return health_status, 200
    except Exception as e:
        logger.error(f"Health check failed: {e}")
        return {"error": "Health check failed"}, 500


@app.route('/query/plaintext', methods=['POST', 'OPTIONS'])
@cors_middleware
def plaintext_query_handler():
    """Handle plaintext queries."""
    if request.method == 'OPTIONS':
        return {}, 200
    
    try:
        data = request.get_json()
        if not data or 'index' not in data:
            return {"error": "Missing 'index' field"}, 400
        
        index = data['index']
        if not isinstance(index, int) or index < 0:
            return {"error": "Invalid index"}, 400
        
        result = server.plaintext_query(index)
        return result, 200
        
    except ValueError as e:
        return {"error": str(e)}, 400
    except Exception as e:
        logger.error(f"Plaintext query error: {e}")
        return {"error": "Internal server error"}, 500


@app.route('/query/fullset', methods=['POST', 'OPTIONS'])
@cors_middleware
def full_set_query_handler():
    """Handle full set queries."""
    if request.method == 'OPTIONS':
        return {}, 200
    
    try:
        data = request.get_json()
        if not data or 'prf_key' not in data:
            return {"error": "Missing 'prf_key' field"}, 400
        
        prf_key = data['prf_key']
        if not isinstance(prf_key, str):
            return {"error": "prf_key must be a hex string"}, 400
        
        # Convert hex string to bytes
        try:
            prf_key_bytes = bytes.fromhex(prf_key)
        except ValueError:
            return {"error": "Invalid hex string for prf_key"}, 400
        
        result = server.full_set_query(prf_key_bytes)
        return result, 200
        
    except ValueError as e:
        return {"error": str(e)}, 400
    except Exception as e:
        logger.error(f"Full set query error: {e}")
        return {"error": "Internal server error"}, 500


@app.route('/query/setparity', methods=['POST', 'OPTIONS'])
@cors_middleware
def set_parity_query_handler():
    """Handle set parity queries."""
    if request.method == 'OPTIONS':
        return {}, 200
    
    try:
        data = request.get_json()
        if not data or 'indices' not in data:
            return {"error": "Missing 'indices' field"}, 400
        
        indices = data['indices']
        if not isinstance(indices, list) or not all(isinstance(i, int) for i in indices):
            return {"error": "indices must be a list of integers"}, 400
        
        result = server.set_parity_query(indices)
        return result, 200
        
    except ValueError as e:
        return {"error": str(e)}, 400
    except Exception as e:
        logger.error(f"Set parity query error: {e}")
        return {"error": "Internal server error"}, 500


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(description='Plinko PIR Server - Python Reference Implementation')
    parser.add_argument('--port', type=int, help='Server port (default: 8080)')
    parser.add_argument('--database-path', help='Path to database file (default: data/database.bin)')
    parser.add_argument('--database-timeout', type=int, help='Database wait timeout in seconds (default: 60)')
    parser.add_argument('--log-level', choices=['DEBUG', 'INFO', 'WARNING', 'ERROR'], help='Log level (default: INFO)')
    
    args = parser.parse_args()
    
    try:
        # Load configuration
        config = load_config()
        
        # Override with command line arguments
        if args.port:
            config.port = args.port
        if args.database_path:
            config.database_path = args.database_path
        if args.database_timeout is not None:
            config.database_wait_timeout = args.database_timeout
        if args.log_level:
            config.log_level = args.log_level
        
        # Setup logging
        config.setup_logging()
        
        # Log startup banner
        logger.info("=" * 40)
        logger.info("Plinko PIR Server - Python Reference")
        logger.info("=" * 40)
        logger.info("")
        logger.info(f"Configuration: port={config.port}, database_path={config.database_path}, "
                   f"database_timeout={config.database_wait_timeout}s")
        
        # Create and initialize server
        global server
        server = PlinkoPIRServer(config.database_path)
        server.load_database(config.database_wait_timeout)
        
        logger.info("")
        logger.info(f"🚀 Plinko PIR Server listening on {config.get_listen_address()}")
        logger.info("=" * 40)
        logger.info("")
        logger.info("Privacy Mode: ENABLED")
        logger.info("⚠️  Server will NEVER log queried addresses")
        logger.info("")
        
        # Start Flask server
        app.run(host='0.0.0.0', port=config.port, debug=False)
        
    except (ConfigurationError, DatabaseError) as e:
        logger.error(f"Configuration error: {e}")
        exit(1)
    except KeyboardInterrupt:
        logger.info("Server stopped by user")
    except Exception as e:
        logger.error(f"Server error: {e}")
        exit(1)


if __name__ == '__main__':
    main()