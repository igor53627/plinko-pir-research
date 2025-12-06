import { FastAes128 } from './aes128-fast.js';
import { SwapOrNotPRP } from './swap-or-not-prp.js';

/**
 * Invertible PRF (iPRF) v2 - Aligned with Plinko.v reference implementation
 * 
 * Built from Swap-or-Not PRP + PMNS:
 * - Forward: iF.F(k, x) = S(k_pmns, P(k_prp, x))
 * - Inverse: iF.F^{-1}(k, y) = {P^{-1}(k_prp, z) : z ∈ S^{-1}(k_pmns, y)}
 * 
 * Security follows from:
 * 1. Swap-or-Not provides a secure small-domain PRP (Morris-Rogaway 2013)
 * 2. PMNS simulates the preimage-size distribution of a random function
 * 3. Composition yields an iPRF indistinguishable from a random function
 */
export class IPRF {
  constructor(key, n, m) {
    if (key.length !== 32) throw new Error("IPRF key must be 32 bytes");
    
    const key1 = key.slice(0, 16);
    const key2 = key.slice(16, 32);
    
    this.prp = new SwapOrNotPRP(key1, n);
    this.pmns = new PMNS(key2, n, m);
  }

  forward(x) {
    const permuted = this.prp.permute(BigInt(x));
    return this.pmns.forward(permuted);
  }

  inverse(y) {
    const preimages = this.pmns.backward(BigInt(y));
    return preimages.map(val => this.prp.inverse(val));
  }
  
  domainSize() {
    return this.prp.n;
  }
  
  rangeSize() {
    return this.pmns.m;
  }
}

/**
 * PMNS (Pseudorandom Multinomial Sampler)
 * 
 * Binary tree sampling that simulates throwing n balls into m bins.
 * Named after the Plinko game where balls bounce left/right at pegs.
 * 
 * Complexity: O(log m) for both forward and inverse
 */
class PMNS {
  constructor(key, n, m) {
    if (n <= 0 || m <= 0) throw new Error("PMNS requires n > 0 and m > 0");
    
    const mBig = BigInt(m);
    if ((mBig & (mBig - 1n)) !== 0n) {
      throw new Error("PMNS currently assumes m is a power of two for Plinko parameters");
    }

    this.block = new FastAes128(key);
    this.n = BigInt(n);
    this.m = mBig;
  }

  forward(x) {
    x = BigInt(x);
    let node = { start: 0n, count: this.n, low: 0n, high: this.m - 1n };
    
    while (node.low < node.high) {
      const { left: leftNode, right: rightNode } = this.children(node);
      
      if (x < node.start + leftNode.count) {
        node = leftNode;
      } else {
        node = rightNode;
      }
    }
    return Number(node.low);
  }

  backward(y) {
    y = BigInt(y);
    let node = { start: 0n, count: this.n, low: 0n, high: this.m - 1n };
    
    while (node.low < node.high) {
      const { left: leftNode, right: rightNode } = this.children(node);
      const mid = (node.high + node.low) / 2n;
      
      if (y <= mid) {
        node = leftNode;
      } else {
        node = rightNode;
      }
    }
    
    const result = [];
    for (let i = 0n; i < node.count; i++) {
      result.push(node.start + i);
    }
    return result;
  }

  children(node) {
    const mid = (node.high + node.low) / 2n;
    const leftBins = mid - node.low + 1n;
    const totalBins = node.high - node.low + 1n;
    
    const p = Number(leftBins) / Number(totalBins);
    const leftCount = this.sampleBinomial(node.count, p, node.low, node.high);
    
    const left = {
      start: node.start,
      count: leftCount,
      low: node.low,
      high: mid
    };
    
    const right = {
      start: node.start + leftCount,
      count: node.count - leftCount,
      low: mid + 1n,
      high: node.high
    };
    
    return { left, right };
  }

  sampleBinomial(n, p, low, high) {
    if (n === 0n) return 0n;
    
    const nNum = Number(n);
    
    if (nNum > 100) {
      return this.sampleBinomialNormal(n, p, low, high);
    }
    
    return this.sampleBinomialExact(n, p, low, high);
  }
  
  sampleBinomialNormal(n, p, low, high) {
    const nNum = Number(n);
    const mean = nNum * p;
    const variance = nNum * p * (1 - p);
    const stddev = Math.sqrt(variance);
    
    const seed = new Uint8Array(16);
    const seedView = new DataView(seed.buffer);
    seedView.setBigUint64(0, low, true);
    seedView.setBigUint64(8, high, true);
    
    const output = new Uint8Array(16);
    this.block.encryptBlock(seed, output);
    
    const outView = new DataView(output.buffer);
    const u1 = (outView.getUint32(0, true) >>> 0) / 4294967296;
    const u2 = (outView.getUint32(4, true) >>> 0) / 4294967296;
    
    const z = Math.sqrt(-2 * Math.log(u1 + 1e-10)) * Math.cos(2 * Math.PI * u2);
    
    let result = Math.round(mean + stddev * z);
    result = Math.max(0, Math.min(nNum, result));
    
    return BigInt(result);
  }
  
  sampleBinomialExact(n, p, low, high) {
    const seed = new Uint8Array(16);
    const seedView = new DataView(seed.buffer);
    seedView.setBigUint64(0, low, true);
    seedView.setBigUint64(8, high, true);
    
    const input = new Uint8Array(16);
    input.set(seed);
    const inputView = new DataView(input.buffer);
    
    const output = new Uint8Array(16);
    
    let successes = 0n;
    const isHalf = Math.abs(p - 0.5) < 0.000001;
    
    let processed = 0n;
    let counter = 0n;
    
    while (processed < n) {
      inputView.setBigUint64(0, counter, true);
      this.block.encryptBlock(input, output);
      counter++;
      
      for (let i = 0; i < 16 && processed < n; i++) {
        if (isHalf) {
          const val = output[i];
          for (let b = 0; b < 8 && processed < n; b++) {
            if ((val & (1 << b)) !== 0) {
              successes++;
            }
            processed++;
          }
        } else {
          const outView = new DataView(output.buffer);
          if (i + 4 <= 16) {
            const rndVal = outView.getUint32(i, true);
            const rndFloat = rndVal / 4294967296.0;
            if (rndFloat < p) {
              successes++;
            }
            processed++;
            i += 3;
          }
        }
      }
    }
    
    return successes;
  }
}

/**
 * Deterministic Random Subset Generator
 * 
 * Generates a random subset of exactly `size` elements from [0, total-1]
 * using a PRF for deterministic randomness.
 * 
 * Matches Plinko.v random_subset implementation.
 */
export class SubsetGenerator {
  constructor(key) {
    if (key.length !== 16) throw new Error("SubsetGenerator key must be 16 bytes");
    this.block = new FastAes128(key);
  }

  /**
   * Generate a deterministic random subset of `size` elements from [0, total-1]
   * @param {number} seed - Seed for this specific subset (e.g., hint index)
   * @param {number} size - Number of elements to select
   * @param {number} total - Total number of elements to choose from
   * @returns {Set<number>} - Set of selected indices
   */
  generate(seed, size, total) {
    if (size > total) throw new Error("Subset size cannot exceed total");
    if (size === 0) return new Set();
    
    const result = new Set();
    let counter = 0;
    
    const input = new Uint8Array(16);
    const inputView = new DataView(input.buffer);
    inputView.setBigUint64(8, BigInt(seed), true);
    
    const output = new Uint8Array(16);
    const outputView = new DataView(output.buffer);
    
    while (result.size < size) {
      inputView.setBigUint64(0, BigInt(counter), true);
      this.block.encryptBlock(input, output);
      counter++;
      
      // Extract up to 4 candidate indices per AES block
      for (let i = 0; i < 4 && result.size < size; i++) {
        const raw = outputView.getUint32(i * 4, true);
        const idx = raw % total;
        result.add(idx);
      }
    }
    
    return result;
  }

  /**
   * Check if a block index is in the subset for a given hint index
   * Uses same logic as generate() but checks a single element
   */
  contains(seed, size, total, blockIdx) {
    const subset = this.generate(seed, size, total);
    return subset.has(blockIdx);
  }
}
