"""
Utility functions for Plinko PIR server.

Common helper functions and constants.
"""

import struct
import logging
from typing import List, Dict, Any

logger = logging.getLogger(__name__)

# Constants for Plinko PIR operations
DB_ENTRY_SIZE = 8  # uint64 = 8 bytes
DB_ENTRY_LENGTH = 1

# HTTP response codes
HTTP_OK = 200
HTTP_BAD_REQUEST = 400
HTTP_NOT_FOUND = 404
HTTP_INTERNAL_ERROR = 500


def uint64_to_bytes(value: int) -> bytes:
    """
    Convert uint64 to big-endian bytes.
    
    Args:
        value: Unsigned 64-bit integer
        
    Returns:
        8-byte big-endian representation
    """
    return struct.pack('>Q', value)


def bytes_to_uint64(data: bytes) -> int:
    """
    Convert big-endian bytes to uint64.
    
    Args:
        data: 8-byte big-endian data
        
    Returns:
        Unsigned 64-bit integer
    """
    if len(data) != 8:
        raise ValueError(f"Expected 8 bytes, got {len(data)}")
    return struct.unpack('>Q', data)[0]


def validate_index(index: int, max_index: int) -> None:
    """
    Validate a database index.
    
    Args:
        index: Index to validate
        max_index: Maximum allowed index
        
    Raises:
        ValueError: If index is invalid
    """
    if not isinstance(index, int):
        raise ValueError(f"Index must be integer, got {type(index)}")
    
    if index < 0:
        raise ValueError(f"Index cannot be negative: {index}")
    
    if index >= max_index:
        raise ValueError(f"Index {index} out of bounds [0, {max_index})")


def validate_prf_key(key_data: bytes) -> None:
    """
    Validate a PRF key.
    
    Args:
        key_data: PRF key bytes
        
    Raises:
        ValueError: If key is invalid
    """
    if not isinstance(key_data, bytes):
        raise ValueError(f"PRF key must be bytes, got {type(key_data)}")
    
    if len(key_data) != 16:
        raise ValueError(f"PRF key must be 16 bytes, got {len(key_data)}")


def compute_xor(values: List[int]) -> int:
    """
    Compute XOR of a list of uint64 values.
    
    Args:
        values: List of uint64 values
        
    Returns:
        XOR result
    """
    result = 0
    for value in values:
        result ^= value
    return result


def format_size(size_bytes: int) -> str:
    """
    Format byte size in human-readable format.
    
    Args:
        size_bytes: Size in bytes
        
    Returns:
        Formatted size string
    """
    for unit in ['B', 'KB', 'MB', 'GB']:
        if size_bytes < 1024.0:
            return f"{size_bytes:.1f} {unit}"
        size_bytes /= 1024.0
    return f"{size_bytes:.1f} TB"


def sanitize_for_logging(data: str, max_length: int = 100) -> str:
    """
    Sanitize data for safe logging (prevent log injection).
    
    Args:
        data: Data to sanitize
        max_length: Maximum length to log
        
    Returns:
        Sanitized data safe for logging
    """
    if not isinstance(data, str):
        data = str(data)
    
    # Remove control characters and truncate
    sanitized = ''.join(char for char in data if ord(char) >= 32 and ord(char) <= 126)
    
    if len(sanitized) > max_length:
        sanitized = sanitized[:max_length-3] + "..."
    
    return sanitized


class Timer:
    """Simple timer for measuring operation duration."""
    
    def __init__(self):
        self.start_time = None
        self.end_time = None
    
    def start(self):
        """Start the timer."""
        self.start_time = time.time()
        self.end_time = None
    
    def stop(self):
        """Stop the timer."""
        self.end_time = time.time()
    
    def elapsed_nanos(self) -> int:
        """
        Get elapsed time in nanoseconds.
        
        Returns:
            Elapsed time in nanoseconds
        """
        if self.start_time is None:
            return 0
        
        end_time = self.end_time or time.time()
        elapsed_seconds = end_time - self.start_time
        return int(elapsed_seconds * 1_000_000_000)
    
    def __enter__(self):
        self.start()
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        self.stop()


def setup_security_headers(response) -> None:
    """
    Add security headers to HTTP response.
    
    Args:
        response: Flask response object
    """
    response.headers['X-Content-Type-Options'] = 'nosniff'
    response.headers['X-Frame-Options'] = 'DENY'
    response.headers['X-XSS-Protection'] = '1; mode=block'
    response.headers['Strict-Transport-Security'] = 'max-age=31536000; includeSubDomains'


def log_security_event(event: str, details: Dict[str, Any] = None) -> None:
    """
    Log a security-related event.
    
    Args:
        event: Event description
        details: Additional event details (sanitized before logging)
    """
    if details:
        # Sanitize details to prevent log injection
        sanitized_details = {k: sanitize_for_logging(str(v)) for k, v in details.items()}
        logger.warning(f"SECURITY: {event} - {sanitized_details}")
    else:
        logger.warning(f"SECURITY: {event}")


# Import time for Timer class
import time