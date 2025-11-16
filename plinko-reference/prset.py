"""
PRSet (Pseudorandom Set) implementation for Plinko PIR.

This module provides AES-128 based pseudorandom set expansion for
generating deterministic sets of database indices.
"""

import struct
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.backends import default_backend


class PrfKey128:
    """A 16-byte PRF key for AES-128 operations."""
    
    def __init__(self, key_bytes=None):
        if key_bytes is None:
            # Generate random key if not provided
            import os
            key_bytes = os.urandom(16)
        
        if len(key_bytes) != 16:
            raise ValueError(f"PRF key must be 16 bytes, got {len(key_bytes)}")
        
        self.key_bytes = key_bytes
    
    def __bytes__(self):
        return self.key_bytes
    
    def __eq__(self, other):
        if not isinstance(other, PrfKey128):
            return False
        return self.key_bytes == other.key_bytes


class PRSet:
    """
    Pseudorandom Set for Plinko PIR.
    
    Uses AES-128 to generate deterministic sets of database indices.
    Provides cryptographically secure pseudorandom set expansion.
    """
    
    def __init__(self, key: PrfKey128):
        """
        Create a new PRSet with the given key.
        
        Args:
            key: 16-byte PRF key for AES-128
        """
        self.key = key
        # Create AES-128 cipher in ECB mode (industry standard)
        self.cipher = Cipher(
            algorithms.AES(bytes(key)),
            modes.ECB(),
            backend=default_backend()
        )
        self.encryptor = self.cipher.encryptor()
    
    def expand(self, set_size: int, chunk_size: int) -> list[int]:
        """
        Generate a pseudorandom set of database indices.
        
        Args:
            set_size: Number of chunks (k in Plinko PIR)
            chunk_size: Size of each chunk
            
        Returns:
            Array of set_size indices, one per chunk
        """
        indices = []
        
        for i in range(set_size):
            offset = self.prf_eval_mod(i, chunk_size)
            indices.append(i * chunk_size + offset)
        
        return indices
    
    def prf_eval_mod(self, x: int, m: int) -> int:
        """
        Evaluate PRF(key, x) mod m using AES-128.
        
        Args:
            x: Input value
            m: Modulus
            
        Returns:
            PRF output mod m
        """
        if m == 0:
            return 0
        
        # Create input block: 16 bytes, with x in the last 8 bytes
        # Standard format for AES block processing
        input_block = bytearray(16)
        struct.pack_into('>Q', input_block, 8, x)  # Big-endian uint64 at offset 8
        
        # Encrypt using AES-128
        output_block = self.encryptor.update(bytes(input_block))
        
        # Extract first 8 bytes as uint64 (like Go: output[:8])
        value = struct.unpack('>Q', output_block[:8])[0]
        
        return value % m
    
    def __repr__(self):
        return f"PRSet(key_hash={hash(bytes(self.key)) & 0xFFFF:04x})"