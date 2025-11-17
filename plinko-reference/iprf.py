"""
Invertible Pseudorandom Function (iPRF) Implementation for Plinko PIR.

Based on Plinko paper: https://eprint.iacr.org/2022/1483

Implementation follows the iPRF construction from Section 4 and implementation
details from Section 5. Key algorithmic components:

- Tree-based inverse enumeration: O(log m + k) complexity per Theorem 4.4
- SHA-256 node encoding: Collision-free identifier generation for tree nodes
- Deterministic key derivation: PRF-based key hierarchy (Section 5.2)
- PMNS binomial sampling: Balls-into-bins with pseudorandom binomial splits
- Parameter consistency: Maintains separate tracking for node encoding vs sampling

This is a simplified iPRF using binomial tree sampling without full PMNS+PRP composition.
"""

import hashlib
import math
import struct
from typing import List, Optional
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.backends import default_backend


class IPRF:
    """
    Invertible Pseudorandom Function for Plinko PIR.

    Maps domain [0, n) to range [0, m) with efficient forward and inverse operations.

    Performance:
    - Forward: O(log m) - trace ball through binary tree
    - Inverse: O(log m + k) - enumerate k balls in target bin

    Memory: O(1) - no tables needed for basic version
    """

    def __init__(self, key: bytes, domain: int, range_size: int):
        """
        Create new iPRF instance.

        Args:
            key: 16-byte AES key for PRF operations
            domain: Domain size n (e.g., database size)
            range_size: Range size m (e.g., hint set size)

        Raises:
            ValueError: If parameters are invalid
        """
        if len(key) != 16:
            raise ValueError(f"Key must be 16 bytes, got {len(key)}")

        if domain == 0 or range_size == 0:
            raise ValueError(f"Domain and range must be > 0, got domain={domain}, range={range_size}")

        self.key = key
        self.domain = domain
        self.range_size = range_size
        self.tree_depth = int(math.ceil(math.log2(range_size))) if range_size > 1 else 0

        # Initialize AES cipher for PRF operations
        self.cipher = Cipher(
            algorithms.AES(key),
            modes.ECB(),
            backend=default_backend()
        )
        self._encryptor = self.cipher.encryptor()

    def forward(self, x: int) -> int:
        """
        Evaluate iPRF: maps x ∈ [0, n) to y ∈ [0, m).

        Uses binomial tree sampling to deterministically assign balls to bins.

        Args:
            x: Input value in domain [0, n)

        Returns:
            Output value in range [0, m)
        """
        if x >= self.domain:
            return 0

        return self._trace_ball(x, self.domain, self.range_size)

    def inverse(self, y: int) -> List[int]:
        """
        Compute iPRF inverse: returns all x such that forward(x) = y.

        This is the key innovation - efficient enumeration of preimages
        using tree traversal instead of brute force scanning.

        Args:
            y: Target bin in range [0, m)

        Returns:
            List of all x ∈ [0, n) that map to y (sorted)
        """
        if y >= self.range_size:
            return []

        return self._enumerate_balls_in_bin(y, self.domain, self.range_size)

    def get_expected_preimage_size(self) -> int:
        """
        Returns expected size of inverse(y) for any y.

        Returns:
            Expected preimage size ≈ n/m
        """
        return int(math.ceil(self.domain / self.range_size))

    # --- Internal Implementation ---

    def _trace_ball(self, x_prime: int, n: int, m: int) -> int:
        """
        Trace ball x_prime through binary tree to find its bin.

        Args:
            x_prime: Ball index
            n: Total balls in current subtree
            m: Total bins in current subtree

        Returns:
            Bin index that ball lands in
        """
        if m == 1:
            return 0  # Only one bin

        # Tree traversal state
        low = 0
        high = m - 1
        ball_count = n
        ball_index = x_prime

        while low < high:
            mid = (low + high) // 2
            left_bins = mid - low + 1
            total_bins = high - low + 1

            # Probability ball goes left
            p = left_bins / total_bins

            # Sample binomial split using PRF
            # Use SHA-256 hash-based node encoding for collision-free identifiers
            node_id = encode_node(low, high, n)
            left_count = self._sample_binomial(node_id, ball_count, p)

            # Determine if ball goes left or right
            if ball_index < left_count:
                # Ball goes left
                high = mid
                ball_count = left_count
            else:
                # Ball goes right
                low = mid + 1
                ball_index = ball_index - left_count
                ball_count = ball_count - left_count

        return low

    def _enumerate_balls_in_bin(self, target_bin: int, n: int, m: int) -> List[int]:
        """
        Enumerate all balls in target bin using efficient tree traversal.

        Implements Algorithm 2 from Plinko paper (inverse function).
        Achieves O(log m + k) complexity where k is the number of balls in the bin,
        as proven in Theorem 4.4.

        The algorithm recursively traverses the binomial tree, following only paths
        that lead to the target bin, avoiding full domain scan.

        Args:
            target_bin: Bin to find balls for
            n: Total balls
            m: Total bins

        Returns:
            Sorted list of ball indices mapping to target_bin
        """
        if m == 1:
            # All balls map to bin 0
            return list(range(n))

        result = []
        self._enumerate_recursive(
            target_bin=target_bin,
            low=0, high=m - 1,
            original_n=n,  # For consistent node encoding across tree
            ball_count=n,  # For binomial sampling in current subtree
            start_idx=0, end_idx=n - 1,
            result=result
        )

        return sorted(result)

    def _enumerate_recursive(
        self,
        target_bin: int,
        low: int, high: int,
        original_n: int,  # For deterministic node encoding
        ball_count: int,  # For binomial sampling in subtree
        start_idx: int, end_idx: int,
        result: List[int]
    ) -> None:
        """
        Recursive helper for tree-based bin enumeration.

        Maintains two separate parameters for correctness:
        - original_n: Fixed domain size for consistent node ID generation
        - ball_count: Dynamic count for binomial sampling in current subtree

        Args:
            target_bin: Bin we're searching for
            low, high: Current bin range
            original_n: Original domain size (for node encoding consistency)
            ball_count: Current number of balls in subtree
            start_idx, end_idx: Current ball index range
            result: Accumulated result list (modified in place)
        """
        # Base cases
        if low > high or start_idx > end_idx or ball_count == 0:
            return

        # Leaf node - add all balls if this is target bin
        if low == high:
            if low == target_bin:
                result.extend(range(start_idx, end_idx + 1))
            return

        # Internal node - recurse into appropriate subtree
        mid = (low + high) // 2
        left_bins = mid - low + 1
        total_bins = high - low + 1
        p = left_bins / total_bins

        # Sample binomial split
        # Use original_n for deterministic encoding, ball_count for sampling distribution
        node_id = encode_node(low, high, original_n)
        left_count = self._sample_binomial(node_id, ball_count, p)
        right_count = ball_count - left_count

        split_idx = start_idx + left_count

        # Recurse into appropriate subtree
        if target_bin <= mid:
            # Target in left subtree
            if left_count > 0 and split_idx > start_idx:
                self._enumerate_recursive(
                    target_bin, low, mid,
                    original_n, left_count,
                    start_idx, split_idx - 1,
                    result
                )
        else:
            # Target in right subtree
            if right_count > 0 and split_idx <= end_idx:
                self._enumerate_recursive(
                    target_bin, mid + 1, high,
                    original_n, right_count,
                    split_idx, end_idx,
                    result
                )

    def _sample_binomial(self, node_id: int, n: int, p: float) -> int:
        """
        Sample from Binomial(n, p) using PRF-based randomness.

        Args:
            node_id: Unique node identifier for deterministic sampling
            n: Number of trials
            p: Success probability

        Returns:
            Sample from Binomial(n, p)
        """
        # Generate PRF output
        prf_output = self._prf_eval(node_id)

        # Map to uniform [0, 1)
        # Use upper 53 bits for precision (IEEE 754 double mantissa)
        INV_TWO_TO_53 = 1.0 / (1 << 53)
        uniform = ((prf_output >> 11) + 0.5) * INV_TWO_TO_53

        # Use inverse CDF to convert to binomial sample
        return self._binomial_inverse_cdf(n, p, uniform)

    def _binomial_inverse_cdf(self, n: int, p: float, u: float) -> int:
        """
        Compute inverse CDF of Binomial(n, p) at point u.

        Returns k such that P(X ≤ k) ≥ u and P(X ≤ k-1) < u.

        Args:
            n: Number of trials
            p: Success probability
            u: Uniform random value in [0, 1]

        Returns:
            Binomial sample
        """
        # Handle edge cases
        if u <= 0.0:
            return 0
        if u >= 1.0:
            return n
        if p == 0:
            return 0
        if p == 1:
            return n
        if n == 0:
            return 0

        # For large n, use normal approximation
        if n > 100:
            return self._normal_approx_binomial(n, p, u)

        # For small n, use exact CDF computation
        cum_prob = 0.0
        q = 1.0 - p

        # Start with P(X = 0) = q^n
        prob = math.pow(q, n)
        cum_prob += prob

        if u <= cum_prob:
            return 0

        # Compute remaining probabilities using recurrence relation
        for k in range(n):
            prob = prob * (n - k) / (k + 1) * p / q
            cum_prob += prob

            if u <= cum_prob:
                return k + 1

        return n

    def _normal_approx_binomial(self, n: int, p: float, u: float) -> int:
        """
        Normal approximation for Binomial inverse CDF (large n).

        Uses N(np, np(1-p)) approximation.

        Args:
            n: Number of trials
            p: Success probability
            u: Uniform value

        Returns:
            Approximate binomial sample
        """
        mean = n * p
        variance = n * p * (1 - p)
        stddev = math.sqrt(variance)

        # Clamp u to safe range for inverse normal CDF
        u_clamped = max(0.001, min(0.999, u))

        # Inverse normal CDF
        z = inv_normal_cdf(u_clamped)
        result = mean + z * stddev

        # Clamp to valid range
        return int(max(0, min(n, round(result))))

    def _prf_eval(self, x: int) -> int:
        """
        Evaluate PRF using AES-128: returns first 64 bits of AES(key, x).

        Args:
            x: Input value

        Returns:
            64-bit PRF output
        """
        # Create 16-byte input block with x in last 8 bytes
        input_block = bytearray(16)
        struct.pack_into('>Q', input_block, 8, x & 0xFFFFFFFFFFFFFFFF)

        # Encrypt with AES
        output_block = self._encryptor.update(bytes(input_block))

        # Extract first 8 bytes as uint64
        return struct.unpack('>Q', output_block[:8])[0]


# --- Helper Functions ---

def encode_node(low: int, high: int, n: int) -> int:
    """
    Create unique node identifier using SHA-256 hash.

    Generates collision-free identifiers for tree nodes across the entire
    parameter space. Essential for deterministic binomial sampling in the
    iPRF tree structure (Section 5.2).

    Uses cryptographic hash to avoid collisions that can occur with simple
    bit-packing schemes when parameters exceed certain bounds.

    Args:
        low: Lower bin range
        high: Upper bin range
        n: Ball count (domain size)

    Returns:
        64-bit node ID from hash
    """
    # Pack parameters as big-endian 64-bit integers
    buf = struct.pack('>QQQ', low, high, n)

    # Hash and extract first 8 bytes as node ID
    h = hashlib.sha256(buf)
    return struct.unpack('>Q', h.digest()[:8])[0]


def derive_iprf_key(master_secret: bytes, context: str) -> bytes:
    """
    Derive deterministic iPRF key from master secret using PRF.

    Implements key hierarchy as described in Section 5.2 of Plinko paper:
    "The n/r keys for each of the iPRFs can also be pseudorandomly
    generated using a PRF. Therefore, this only requires storing
    a single PRF key."

    Ensures consistent key generation for reproducible iPRF outputs,
    which is critical for hint validity across server sessions.

    Args:
        master_secret: Long-term secret (from config/KMS)
        context: Context string for domain separation

    Returns:
        16-byte deterministic key
    """
    h = hashlib.sha256()
    h.update(master_secret)
    h.update(b"iprf-key-derivation-v1")  # Domain separator
    h.update(context.encode('utf-8'))

    return h.digest()[:16]


def inv_normal_cdf(p: float) -> float:
    """
    Approximate inverse normal CDF (quantile function).

    Uses rational approximation for the central region.

    Args:
        p: Probability in (0, 1)

    Returns:
        z such that Φ(z) ≈ p
    """
    if p <= 0:
        return -10.0
    if p >= 1:
        return 10.0

    # Rational approximation coefficients
    a0 = 2.50662823884
    a1 = -18.61500062529
    a2 = 41.39119773534
    a3 = -25.44106049637

    b0 = -8.47351093090
    b1 = 23.08336743743
    b2 = -21.06224101826
    b3 = 3.13082909833

    y = p - 0.5

    if abs(y) < 0.42:
        # Central region - accurate approximation
        r = y * y
        return y * (((a3*r + a2)*r + a1)*r + a0) / ((((b3*r + b2)*r + b1)*r + b0)*r + 1)

    # Tail region - simplified
    return 2.0 if y > 0 else -2.0
