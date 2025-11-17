"""
Table-based Pseudorandom Permutation with O(1) forward and inverse operations.

Implements a perfect bijection using Fisher-Yates shuffle with pre-computed
lookup tables. This provides the permutation component needed for full iPRF
construction as described in the Plinko paper.

Algorithm: Deterministic Fisher-Yates shuffle seeded by cryptographic PRF
Complexity: O(n) initialization, O(1) forward/inverse queries
Memory: 16 bytes per element (2 * 8 bytes for uint64 tables)
For n=8,400,000: ~134 MB total (67 MB forward + 67 MB inverse)
"""

import struct
from typing import Dict
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.backends import default_backend


class TablePRP:
    """
    Table-based Pseudorandom Permutation using deterministic Fisher-Yates shuffle.

    Provides perfect bijection with O(1) forward and inverse operations.

    Properties:
    - Deterministic: Same key + domain → same permutation
    - Bijective: Perfect 1-to-1 mapping, no collisions
    - Fast: O(1) forward and inverse after O(n) initialization
    - Space: O(n) - stores forward and inverse tables
    """

    def __init__(self, domain: int, key: bytes):
        """
        Initialize TablePRP with deterministic shuffle.

        Args:
            domain: Size of domain [0, n)
            key: 16-byte key for deterministic shuffle

        Raises:
            ValueError: If domain is 0 or key is not 16 bytes
        """
        if domain == 0:
            raise ValueError("Domain size cannot be zero")

        if len(key) != 16:
            raise ValueError(f"Key must be 16 bytes, got {len(key)}")

        self.domain = domain
        self.key = key

        # Create forward and inverse tables
        self.forward_table: Dict[int, int] = {}
        self.inverse_table: Dict[int, int] = {}

        # Generate permutation using deterministic Fisher-Yates
        self._generate_permutation()

    def _generate_permutation(self) -> None:
        """
        Generate permutation using Fisher-Yates shuffle with PRF-seeded RNG.

        Guarantees uniform random permutation with perfect bijection.
        """
        # Initialize identity permutation
        perm = list(range(self.domain))

        # Create deterministic RNG from key
        rng = DeterministicRNG(self.key)

        # Fisher-Yates shuffle
        for i in range(self.domain - 1, 0, -1):
            j = rng.uint64_n(i + 1)
            perm[i], perm[j] = perm[j], perm[i]

        # Build forward and inverse tables
        for x in range(self.domain):
            y = perm[x]
            self.forward_table[x] = y
            self.inverse_table[y] = x

    def forward(self, x: int) -> int:
        """
        Apply PRP: y = P(x).

        Complexity: O(1) - single dictionary lookup

        Args:
            x: Input in [0, domain)

        Returns:
            Permuted output in [0, domain)

        Raises:
            ValueError: If x is out of domain
        """
        if x >= self.domain or x < 0:
            raise ValueError(f"Input {x} out of domain [0, {self.domain})")

        return self.forward_table[x]

    def inverse(self, y: int) -> int:
        """
        Apply inverse PRP: x = P^-1(y).

        Achieves O(1) complexity via pre-computed inverse table lookup.
        Guarantees exact inverse due to perfect bijection property.

        Complexity: O(1) - single dictionary lookup

        Args:
            y: Output in [0, domain)

        Returns:
            Preimage x such that forward(x) = y

        Raises:
            ValueError: If y is out of domain
        """
        if y >= self.domain or y < 0:
            raise ValueError(f"Input {y} out of domain [0, {self.domain})")

        return self.inverse_table[y]

    def verify_bijection(self) -> bool:
        """
        Verify that permutation is a valid bijection.

        Checks:
        1. Forward maps all inputs to distinct outputs
        2. Inverse maps all outputs to distinct inputs
        3. Round-trip consistency: inverse(forward(x)) = x

        Returns:
            True if bijection is valid
        """
        # Check forward table is complete
        if len(self.forward_table) != self.domain:
            return False

        # Check inverse table is complete
        if len(self.inverse_table) != self.domain:
            return False

        # Check round-trip consistency
        for x in range(self.domain):
            y = self.forward_table[x]
            x_recovered = self.inverse_table[y]
            if x != x_recovered:
                return False

        return True


class DeterministicRNG:
    """
    Cryptographically strong deterministic RNG using AES in counter mode.

    Used for Fisher-Yates shuffle to ensure cryptographic quality randomness
    while maintaining determinism.
    """

    def __init__(self, key: bytes):
        """
        Create deterministic RNG from key.

        Args:
            key: 16-byte AES key
        """
        if len(key) < 16:
            # Pad key if too short
            key = key + b'\x00' * (16 - len(key))

        self.key = key[:16]
        self.counter = 0

        # Initialize AES cipher
        self.cipher = Cipher(
            algorithms.AES(self.key),
            modes.ECB(),
            backend=default_backend()
        )
        self._encryptor = self.cipher.encryptor()

    def uint64(self) -> int:
        """
        Generate next deterministic random uint64.

        Uses AES-CTR mode: encrypt incrementing counter to get random bytes.

        Returns:
            Random 64-bit integer
        """
        # Create input block with counter
        input_block = bytearray(16)
        struct.pack_into('>Q', input_block, 0, self.counter)
        struct.pack_into('>Q', input_block, 8, self.counter >> 32)

        # Encrypt to get random bytes
        output_block = self._encryptor.update(bytes(input_block))
        self.counter += 1

        # Extract 64-bit random value
        return struct.unpack('>Q', output_block[:8])[0]

    def uint64_n(self, n: int) -> int:
        """
        Generate uniform random uint64 in [0, n).

        Uses rejection sampling to avoid modulo bias, which is critical
        for Fisher-Yates shuffle correctness.

        Args:
            n: Upper bound (exclusive)

        Returns:
            Random integer in [0, n)
        """
        if n == 0:
            return 0
        if n == 1:
            return 0

        # For power-of-2, simple mask works without bias
        if n & (n - 1) == 0:
            return self.uint64() & (n - 1)

        # For non-power-of-2, use rejection sampling
        max_val = (1 << 64) - 1
        threshold = max_val - (max_val % n)

        while True:
            r = self.uint64()
            if r < threshold:
                return r % n
            # Reject and retry to avoid bias
