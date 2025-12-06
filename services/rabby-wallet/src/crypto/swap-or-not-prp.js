import { FastAes128 } from './aes128-fast.js';

/**
 * Swap-or-Not Small-Domain PRP
 * 
 * Based on Morris-Rogaway (eprint 2013/560) and Plinko.v Coq implementation.
 * Achieves full security (withstands all N queries) in O(n log n) time.
 * 
 * Each round:
 * 1. Compute partner: X' = K_i - X mod N
 * 2. Compute canonical representative: X̂ = max(X, X')
 * 3. If F(i, X̂) = 1, swap X with X'
 * 
 * The key property is that each round is an involution (self-inverse),
 * so inversion runs the same rounds in reverse order.
 */
export class SwapOrNotPRP {
  constructor(key, domainSize) {
    if (key.length !== 16) throw new Error("SwapOrNotPRP key must be 16 bytes");
    if (domainSize <= 0) throw new Error("Domain size must be positive");
    
    this.block = new FastAes128(key);
    this.domainSize = BigInt(domainSize);
    this.rounds = this.computeNumRounds(domainSize);
    
    // Pre-derive round keys for efficiency
    this.roundKeys = new Array(this.rounds);
    for (let i = 0; i < this.rounds; i++) {
      this.roundKeys[i] = this.deriveRoundKey(i);
    }
  }

  /**
   * Number of rounds: ~6 * ceil(log2(N)) for security
   * Matches Plinko.v: 6 * log2_up(domain_size + 1) + 6
   */
  computeNumRounds(domainSize) {
    const log2N = Math.ceil(Math.log2(domainSize + 1));
    return 6 * log2N + 6;
  }

  /**
   * Derive round key K_i from master key and round number.
   * Uses AES(key, i || domain_size) to get a pseudorandom value.
   */
  deriveRoundKey(round) {
    const input = new Uint8Array(16);
    const view = new DataView(input.buffer);
    view.setBigUint64(0, BigInt(round), true);
    view.setBigUint64(8, this.domainSize, true);
    
    const output = new Uint8Array(16);
    this.block.encryptBlock(input, output);
    
    const outView = new DataView(output.buffer);
    // Reduce mod domainSize + 1 to get K_i
    const raw = outView.getBigUint64(0, true);
    return raw % (this.domainSize + 1n);
  }

  /**
   * PRF evaluation for swap decision.
   * Returns a single bit: 0 or 1.
   * Uses AES and takes LSB.
   */
  prfBit(round, canonical) {
    const input = new Uint8Array(16);
    const view = new DataView(input.buffer);
    view.setBigUint64(0, BigInt(round), true);
    view.setBigUint64(8, canonical, true);
    
    const output = new Uint8Array(16);
    this.block.encryptBlock(input, output);
    
    return output[0] & 1;
  }

  /**
   * Compute partner index: K_i - X mod N
   */
  computePartner(roundKey, x) {
    if (this.domainSize === 0n) return 0n;
    // (roundKey + domainSize - (x % domainSize)) % domainSize
    const xMod = x % this.domainSize;
    return (roundKey + this.domainSize - xMod) % this.domainSize;
  }

  /**
   * Single round of Swap-or-Not
   * This is an involution: applying it twice returns to original
   */
  swapOrNotRound(round, x) {
    const ki = this.roundKeys[round];
    const partner = this.computePartner(ki, x);
    const canonical = x > partner ? x : partner; // max(x, partner)
    
    if (this.prfBit(round, canonical) === 1) {
      return partner;
    }
    return x;
  }

  /**
   * Forward PRP: encrypt by running rounds 0, 1, ..., r-1
   */
  permute(x) {
    x = BigInt(x);
    if (x >= this.domainSize) return x;
    
    for (let round = 0; round < this.rounds; round++) {
      x = this.swapOrNotRound(round, x);
    }
    return x;
  }

  /**
   * Inverse PRP: decrypt by running rounds r-1, r-2, ..., 0
   * Since each round is an involution, we just run them in reverse order
   */
  inverse(y) {
    y = BigInt(y);
    if (y >= this.domainSize) return y;
    
    for (let round = this.rounds - 1; round >= 0; round--) {
      y = this.swapOrNotRound(round, y);
    }
    return y;
  }

  get n() {
    return this.domainSize;
  }
}
