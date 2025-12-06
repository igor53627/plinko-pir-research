import { IPRF, SubsetGenerator } from './iprf-v2.js';
import { FastAes128 } from './aes128-fast.js';

/**
 * Plinko Hint Structures - Aligned with Plinko.v Coq implementation
 * 
 * Key components:
 * - Regular hints: (P_j, p_j) where P_j is a subset of blocks (size = c/2 + 1)
 * - Backup hints: (B_j, ℓ_j, r_j) where B_j is subset (size = c/2), with two parities
 * - Promoted hints: Backup hints that have been used, include query index
 */

/**
 * Regular hint: (P_j, p_j)
 * P_j is a subset of block indices, size = c/2 + 1
 * p_j is parity of entries at iPRF-chosen offsets
 */
export class RegularHint {
  constructor(blocks, parity) {
    this.blocks = blocks;  // Set<number> - subset of block indices
    this.parity = parity;  // BigInt - XOR parity of entries (256-bit)
  }

  containsBlock(blockIdx) {
    return this.blocks.has(blockIdx);
  }

  updateParity(delta) {
    this.parity ^= delta;
  }

  clone() {
    return new RegularHint(new Set(this.blocks), this.parity);
  }
}

/**
 * Backup hint: (B_j, ℓ_j, r_j)
 * B_j is subset of block indices, size = c/2
 * ℓ_j is parity of entries in B_j at iPRF-chosen offsets
 * r_j is parity of entries outside B_j at iPRF-chosen offsets
 */
export class BackupHint {
  constructor(blocks, parityIn, parityOut) {
    this.blocks = blocks;      // Set<number> - subset of block indices
    this.parityIn = parityIn;  // BigInt - parity for blocks in B_j
    this.parityOut = parityOut; // BigInt - parity for blocks not in B_j
  }

  containsBlock(blockIdx) {
    return this.blocks.has(blockIdx);
  }

  updateParity(blockIdx, delta) {
    if (this.blocks.has(blockIdx)) {
      this.parityIn ^= delta;
    } else {
      this.parityOut ^= delta;
    }
  }

  clone() {
    return new BackupHint(new Set(this.blocks), this.parityIn, this.parityOut);
  }
}

/**
 * Promoted hint: (P_j, x, p_j)
 * A backup hint that has been promoted after a query
 * Includes the query index x that caused promotion
 */
export class PromotedHint {
  constructor(blocks, queryIndex, parity) {
    this.blocks = blocks;       // Set<number> - subset of block indices
    this.queryIndex = queryIndex; // number - the database index that was queried
    this.parity = parity;       // BigInt - XOR parity
  }

  containsBlock(blockIdx) {
    return this.blocks.has(blockIdx);
  }

  updateParity(delta) {
    this.parity ^= delta;
  }

  clone() {
    return new PromotedHint(new Set(this.blocks), this.queryIndex, this.parity);
  }
}

/**
 * Cache entry for previously queried values
 */
export class CacheEntry {
  constructor(value, hintIdx) {
    this.value = value;    // BigInt - retrieved value
    this.hintIdx = hintIdx; // number - hint index that was used
  }
}

/**
 * Plinko Client State
 * 
 * Manages all client-side state for Plinko PIR:
 * - iPRF keys (one per block)
 * - Regular hints H
 * - Backup hints T
 * - Promoted hints (former backup hints that were used)
 * - Query cache Q
 */
export class PlinkoClientState {
  /**
   * @param {number} n - Database size (total entries)
   * @param {number} w - Block size (entries per block)
   * @param {number} lambda - Security parameter
   * @param {number} q - Number of queries before refresh
   * @param {Uint8Array} masterKey - 32-byte master key
   */
  constructor(n, w, lambda, q, masterKey) {
    this.n = n;
    this.w = w;
    this.c = Math.floor(n / w);  // Number of blocks
    this.lambda = lambda;
    this.q = q;
    
    // Hint counts per paper: λw regular hints, q backup hints
    this.numRegularHints = lambda * w;
    this.numBackupHints = q;
    
    // Initialize keys - one iPRF key per block
    this.keys = [];
    for (let i = 0; i < this.c; i++) {
      const key = this.deriveBlockKey(masterKey, i);
      // Domain: λw + q (total hint indices), Range: w (offsets within block)
      this.keys.push(new IPRF(key, this.numRegularHints + this.numBackupHints, w));
    }
    
    // Initialize subset generator for block selection
    this.subsetGen = new SubsetGenerator(masterKey.slice(0, 16));
    
    // Regular hints: H[0..λw-1]
    this.regularHints = new Array(this.numRegularHints).fill(null);
    
    // Backup hints: T[0..q-1]
    this.backupHints = new Array(this.numBackupHints).fill(null);
    
    // Promoted hints (indexed by original backup hint index)
    this.promotedHints = new Array(this.numBackupHints).fill(null);
    
    // Query cache: stores (value, hintIdx) for each queried index
    this.cache = new Map();
    
    // Track which regular hints have been consumed
    this.consumedRegular = new Set();
    
    // Track next backup hint to promote
    this.nextBackupIdx = 0;
  }

  /**
   * Derive a block-specific key from master key using AES-based key derivation
   * for proper domain separation (addresses CodeRabbit review feedback)
   */
  deriveBlockKey(masterKey, blockIdx) {
    const input = new Uint8Array(16);
    const view = new DataView(input.buffer);
    view.setBigUint64(0, BigInt(blockIdx), true);
    view.setUint32(8, 0x504C4E4B, true); // "PLNK" domain tag
    
    const block = new FastAes128(masterKey.slice(0, 16));
    const derived1 = new Uint8Array(16);
    const derived2 = new Uint8Array(16);
    
    // Generate two 16-byte blocks for 32-byte key
    view.setUint32(12, 0, true);
    block.encryptBlock(input, derived1);
    view.setUint32(12, 1, true);
    block.encryptBlock(input, derived2);
    
    const key = new Uint8Array(32);
    key.set(derived1, 0);
    key.set(derived2, 16);
    return key;
  }

  /**
   * Initialize hints with empty parities
   * Called before streaming the database
   */
  initializeHints() {
    const regularSubsetSize = Math.floor(this.c / 2) + 1;
    const backupSubsetSize = Math.floor(this.c / 2);
    
    // Initialize regular hints
    for (let j = 0; j < this.numRegularHints; j++) {
      const blocks = this.subsetGen.generate(j, regularSubsetSize, this.c);
      this.regularHints[j] = new RegularHint(blocks, 0n);
    }
    
    // Initialize backup hints
    for (let j = 0; j < this.numBackupHints; j++) {
      const blocks = this.subsetGen.generate(
        this.numRegularHints + j,
        backupSubsetSize,
        this.c
      );
      this.backupHints[j] = new BackupHint(blocks, 0n, 0n);
    }
  }

  /**
   * Process a single database entry during HintInit streaming phase
   * Updates all relevant hint parities using iPRF inversion
   * 
   * @param {number} i - Database index
   * @param {BigInt} value - Entry value (256-bit)
   */
  processEntry(i, value) {
    const alpha = Math.floor(i / this.w);  // Block index
    const beta = i % this.w;               // Offset within block
    
    if (alpha >= this.c) return;
    
    // Find all hints that include this entry via iPRF inversion
    const hintIndices = this.keys[alpha].inverse(beta);
    
    for (const j of hintIndices) {
      const jNum = Number(j);
      
      if (jNum < this.numRegularHints) {
        // Regular hint
        const hint = this.regularHints[jNum];
        if (hint && hint.containsBlock(alpha)) {
          hint.updateParity(value);
        }
      } else {
        // Backup hint
        const backupIdx = jNum - this.numRegularHints;
        if (backupIdx < this.numBackupHints) {
          const hint = this.backupHints[backupIdx];
          if (hint) {
            hint.updateParity(alpha, value);
          }
        }
      }
    }
  }

  /**
   * Get a hint containing the entry at (block, offset)
   * Returns the hint info needed for query generation
   * 
   * @param {number} alpha - Block index
   * @param {number} beta - Offset within block
   * @returns {{hintIdx: number, blocks: Set<number>, parity: BigInt, offsets: number[]} | null}
   */
  getHint(alpha, beta) {
    // Find all hint indices that map to this offset
    const candidates = this.keys[alpha].inverse(beta);
    
    // Shuffle candidates using crypto RNG for privacy (addresses CodeRabbit review)
    const shuffled = [...candidates].map(x => Number(x));
    const randomBytes = new Uint8Array(shuffled.length * 4);
    crypto.getRandomValues(randomBytes);
    const randomView = new DataView(randomBytes.buffer);
    for (let i = shuffled.length - 1; i > 0; i--) {
      const randomVal = randomView.getUint32(i * 4, true);
      const j = randomVal % (i + 1);
      [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
    }
    
    // Try regular hints first
    for (const j of shuffled) {
      if (j < this.numRegularHints) {
        const hint = this.regularHints[j];
        if (hint && !this.consumedRegular.has(j) && hint.containsBlock(alpha)) {
          // Compute offsets for all blocks
          const offsets = this.computeOffsets(j);
          return {
            hintIdx: j,
            blocks: hint.blocks,
            parity: hint.parity,
            offsets,
            isPromoted: false
          };
        }
      }
    }
    
    // Try promoted hints
    for (const j of shuffled) {
      if (j >= this.numRegularHints) {
        const backupIdx = j - this.numRegularHints;
        const hint = this.promotedHints[backupIdx];
        if (hint && hint.containsBlock(alpha)) {
          // For promoted hints, check if this is the same entry or compatible
          const queryAlpha = Math.floor(hint.queryIndex / this.w);
          const queryBeta = hint.queryIndex % this.w;
          
          if (alpha === queryAlpha && beta !== queryBeta) {
            continue; // Same block but different offset - not usable
          }
          
          const offsets = this.computeOffsets(j);
          // Override offset for the query block
          offsets[queryAlpha] = queryBeta;
          
          return {
            hintIdx: j,
            blocks: new Set([...hint.blocks, queryAlpha]),
            parity: hint.parity,
            offsets,
            isPromoted: true
          };
        }
      }
    }
    
    return null; // No available hint
  }

  /**
   * Compute iPRF offsets for all blocks given a hint index
   */
  computeOffsets(hintIdx) {
    const offsets = new Array(this.c);
    for (let k = 0; k < this.c; k++) {
      offsets[k] = this.keys[k].forward(hintIdx);
    }
    return offsets;
  }

  /**
   * Mark a hint as consumed and promote a backup hint
   * Called after a successful query
   * 
   * @param {number} hintIdx - The hint that was used
   * @param {number} queryIdx - The database index that was queried
   * @param {BigInt} value - The retrieved value
   */
  consumeHint(hintIdx, queryIdx, value) {
    // Cache the query result
    this.cache.set(queryIdx, new CacheEntry(value, hintIdx));
    
    if (hintIdx < this.numRegularHints) {
      // Mark regular hint as consumed
      this.consumedRegular.add(hintIdx);
      
      // Promote next backup hint if available
      if (this.nextBackupIdx < this.numBackupHints) {
        const backup = this.backupHints[this.nextBackupIdx];
        if (backup) {
          const alpha = Math.floor(queryIdx / this.w);
          
          // Determine which parity to use based on whether alpha is in backup's blocks
          let promotedBlocks, promotedParity;
          if (backup.containsBlock(alpha)) {
            // Use blocks as-is, parity = parityOut XOR value
            promotedBlocks = backup.blocks;
            promotedParity = backup.parityOut ^ value;
          } else {
            // Use complement of blocks, parity = parityIn XOR value
            promotedBlocks = new Set();
            for (let k = 0; k < this.c; k++) {
              if (!backup.containsBlock(k)) {
                promotedBlocks.add(k);
              }
            }
            promotedParity = backup.parityIn ^ value;
          }
          
          this.promotedHints[this.nextBackupIdx] = new PromotedHint(
            promotedBlocks,
            queryIdx,
            promotedParity
          );
          
          // Clear the backup hint
          this.backupHints[this.nextBackupIdx] = null;
          this.nextBackupIdx++;
        }
      }
    }
  }

  /**
   * Update hints after a database change
   * Uses iPRF inversion to find all affected hints in O(1) time
   * 
   * @param {number} i - Database index that changed
   * @param {BigInt} delta - XOR of old and new value
   */
  updateHint(i, delta) {
    const alpha = Math.floor(i / this.w);
    const beta = i % this.w;
    
    if (alpha >= this.c) return;
    
    // Find all hints affected by this entry
    const hintIndices = this.keys[alpha].inverse(beta);
    
    for (const j of hintIndices) {
      const jNum = Number(j);
      
      if (jNum < this.numRegularHints) {
        // Update regular hint
        const hint = this.regularHints[jNum];
        if (hint && hint.containsBlock(alpha)) {
          hint.updateParity(delta);
        }
      } else {
        const backupIdx = jNum - this.numRegularHints;
        
        // Update promoted hint if exists
        const promoted = this.promotedHints[backupIdx];
        if (promoted && promoted.containsBlock(alpha)) {
          promoted.updateParity(delta);
        }
        
        // Update backup hint if exists
        const backup = this.backupHints[backupIdx];
        if (backup) {
          backup.updateParity(alpha, delta);
        }
      }
    }
    
    // Update cache if this index was previously queried
    const cached = this.cache.get(i);
    if (cached) {
      const hintIdx = cached.hintIdx;
      if (hintIdx >= this.numRegularHints) {
        const promoted = this.promotedHints[hintIdx - this.numRegularHints];
        if (promoted) {
          promoted.updateParity(delta);
        }
      }
    }
  }

  /**
   * Check if we have a cached result for an index
   */
  getCached(i) {
    return this.cache.get(i)?.value ?? null;
  }

  /**
   * Get statistics about hint availability
   */
  getStats() {
    const availableRegular = this.numRegularHints - this.consumedRegular.size;
    const availablePromoted = this.promotedHints.filter(h => h !== null).length;
    const remainingBackup = this.numBackupHints - this.nextBackupIdx;
    
    return {
      totalRegular: this.numRegularHints,
      availableRegular,
      consumedRegular: this.consumedRegular.size,
      totalBackup: this.numBackupHints,
      availablePromoted,
      remainingBackup,
      cachedQueries: this.cache.size,
      queriesBeforeRefresh: availableRegular + availablePromoted
    };
  }

  /**
   * Serialize hint parities to bytes for caching
   * Only stores parities (not lifecycle state) for v1
   */
  toBytes() {
    const MAGIC = 0x504C484E; // "PLHN"
    const VERSION = 1;
    
    // Header: 32 bytes
    // Regular hints: numRegularHints * 32 bytes
    // Backup parityIn: numBackupHints * 32 bytes
    // Backup parityOut: numBackupHints * 32 bytes
    const headerSize = 32;
    const regularSize = this.numRegularHints * 32;
    const backupSize = this.numBackupHints * 32 * 2;
    const totalSize = headerSize + regularSize + backupSize;
    
    const buffer = new ArrayBuffer(totalSize);
    const view = new DataView(buffer);
    const bytes = new Uint8Array(buffer);
    
    // Header
    view.setUint32(0, MAGIC, true);
    view.setUint32(4, VERSION, true);
    view.setUint32(8, this.n & 0xFFFFFFFF, true);
    view.setUint32(12, Math.floor(this.n / 0x100000000), true);
    view.setUint32(16, this.w, true);
    view.setUint32(20, this.lambda, true);
    view.setUint32(24, this.q, true);
    view.setUint32(28, this.c, true);
    
    // Regular hint parities
    let offset = headerSize;
    for (let j = 0; j < this.numRegularHints; j++) {
      const parity = this.regularHints[j]?.parity || 0n;
      this.writeBigInt256(bytes, offset, parity);
      offset += 32;
    }
    
    // Backup parityIn
    for (let k = 0; k < this.numBackupHints; k++) {
      const parity = this.backupHints[k]?.parityIn || 0n;
      this.writeBigInt256(bytes, offset, parity);
      offset += 32;
    }
    
    // Backup parityOut
    for (let k = 0; k < this.numBackupHints; k++) {
      const parity = this.backupHints[k]?.parityOut || 0n;
      this.writeBigInt256(bytes, offset, parity);
      offset += 32;
    }
    
    return bytes;
  }

  /**
   * Restore hint parities from cached bytes
   * Regenerates block subsets deterministically from masterKey
   */
  static fromBytes(bytes, masterKey) {
    const MAGIC = 0x504C484E;
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    
    // Validate header
    if (view.getUint32(0, true) !== MAGIC) {
      throw new Error('Invalid hints cache magic');
    }
    const version = view.getUint32(4, true);
    if (version !== 1) {
      throw new Error(`Unsupported hints cache version: ${version}`);
    }
    
    // Read params
    const nLow = view.getUint32(8, true);
    const nHigh = view.getUint32(12, true);
    const n = nLow + nHigh * 0x100000000;
    const w = view.getUint32(16, true);
    const lambda = view.getUint32(20, true);
    const q = view.getUint32(24, true);
    const c = view.getUint32(28, true);
    
    // Create state (this initializes keys and subsetGen)
    const state = new PlinkoClientState(n, w, lambda, q, masterKey);
    
    // Verify params match
    if (state.c !== c) {
      throw new Error(`Block count mismatch: expected ${c}, got ${state.c}`);
    }
    
    // Initialize hints structure (creates empty hints with correct block subsets)
    state.initializeHints();
    
    // Read regular hint parities
    const headerSize = 32;
    let offset = headerSize;
    for (let j = 0; j < state.numRegularHints; j++) {
      state.regularHints[j].parity = state.readBigInt256(bytes, offset);
      offset += 32;
    }
    
    // Read backup parityIn
    for (let k = 0; k < state.numBackupHints; k++) {
      state.backupHints[k].parityIn = state.readBigInt256(bytes, offset);
      offset += 32;
    }
    
    // Read backup parityOut
    for (let k = 0; k < state.numBackupHints; k++) {
      state.backupHints[k].parityOut = state.readBigInt256(bytes, offset);
      offset += 32;
    }
    
    return state;
  }

  writeBigInt256(bytes, offset, value) {
    for (let i = 0; i < 4; i++) {
      const word = value & 0xFFFFFFFFFFFFFFFFn;
      const wordOffset = offset + i * 8;
      for (let j = 0; j < 8; j++) {
        bytes[wordOffset + j] = Number((word >> BigInt(j * 8)) & 0xFFn);
      }
      value >>= 64n;
    }
  }

  readBigInt256(bytes, offset) {
    let value = 0n;
    for (let i = 0; i < 4; i++) {
      let word = 0n;
      const wordOffset = offset + i * 8;
      for (let j = 0; j < 8; j++) {
        word |= BigInt(bytes[wordOffset + j]) << BigInt(j * 8);
      }
      value |= word << BigInt(i * 64);
    }
    return value;
  }
}
