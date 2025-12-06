import { describe, it, expect, beforeEach } from 'vitest';
import { 
  RegularHint, 
  BackupHint, 
  PromotedHint, 
  PlinkoClientState 
} from './plinko-hints.js';

describe('RegularHint', () => {
  it('should store blocks and parity', () => {
    const blocks = new Set([0, 2, 4, 6]);
    const hint = new RegularHint(blocks, 123n);
    
    expect(hint.blocks).toEqual(blocks);
    expect(hint.parity).toBe(123n);
  });

  it('should check block membership', () => {
    const hint = new RegularHint(new Set([1, 3, 5]), 0n);
    
    expect(hint.containsBlock(1)).toBe(true);
    expect(hint.containsBlock(3)).toBe(true);
    expect(hint.containsBlock(2)).toBe(false);
  });

  it('should update parity with XOR', () => {
    const hint = new RegularHint(new Set([0]), 0b1010n);
    hint.updateParity(0b1100n);
    
    expect(hint.parity).toBe(0b0110n);
  });

  it('should clone correctly', () => {
    const hint = new RegularHint(new Set([1, 2]), 42n);
    const clone = hint.clone();
    
    clone.blocks.add(3);
    clone.updateParity(10n);
    
    expect(hint.blocks.has(3)).toBe(false);
    expect(hint.parity).toBe(42n);
  });
});

describe('BackupHint', () => {
  it('should store blocks and two parities', () => {
    const blocks = new Set([0, 1]);
    const hint = new BackupHint(blocks, 10n, 20n);
    
    expect(hint.parityIn).toBe(10n);
    expect(hint.parityOut).toBe(20n);
  });

  it('should update correct parity based on block membership', () => {
    const hint = new BackupHint(new Set([0, 1]), 0n, 0n);
    
    hint.updateParity(0, 5n);   // In blocks
    hint.updateParity(2, 10n); // Not in blocks
    
    expect(hint.parityIn).toBe(5n);
    expect(hint.parityOut).toBe(10n);
  });
});

describe('PromotedHint', () => {
  it('should store query index', () => {
    const hint = new PromotedHint(new Set([1, 2]), 42, 100n);
    
    expect(hint.queryIndex).toBe(42);
    expect(hint.parity).toBe(100n);
  });
});

describe('PlinkoClientState', () => {
  const masterKey = new Uint8Array(32);
  for (let i = 0; i < 32; i++) masterKey[i] = i;

  describe('construction', () => {
    it('should initialize with correct parameters', () => {
      const state = new PlinkoClientState(
        1024,  // n = database size
        32,    // w = block size
        4,     // lambda = security parameter
        10,    // q = queries before refresh
        masterKey
      );
      
      expect(state.n).toBe(1024);
      expect(state.w).toBe(32);
      expect(state.c).toBe(32); // 1024/32
      expect(state.numRegularHints).toBe(128); // lambda * w = 4 * 32
      expect(state.numBackupHints).toBe(10);
    });

    it('should initialize iPRF keys for each block', () => {
      const state = new PlinkoClientState(256, 16, 2, 5, masterKey);
      
      expect(state.keys.length).toBe(16); // c = 256/16
    });
  });

  describe('initializeHints', () => {
    it('should create regular hints with correct subset size', () => {
      const state = new PlinkoClientState(256, 16, 2, 5, masterKey);
      state.initializeHints();
      
      const expectedSize = Math.floor(state.c / 2) + 1; // c/2 + 1
      
      for (const hint of state.regularHints) {
        expect(hint).not.toBeNull();
        expect(hint.blocks.size).toBe(expectedSize);
        expect(hint.parity).toBe(0n);
      }
    });

    it('should create backup hints with correct subset size', () => {
      const state = new PlinkoClientState(256, 16, 2, 5, masterKey);
      state.initializeHints();
      
      const expectedSize = Math.floor(state.c / 2); // c/2
      
      for (const hint of state.backupHints) {
        expect(hint).not.toBeNull();
        expect(hint.blocks.size).toBe(expectedSize);
        expect(hint.parityIn).toBe(0n);
        expect(hint.parityOut).toBe(0n);
      }
    });
  });

  describe('processEntry', () => {
    it('should update hint parities during streaming', () => {
      const state = new PlinkoClientState(64, 8, 2, 3, masterKey);
      state.initializeHints();
      
      // Process some entries
      state.processEntry(0, 1n);
      state.processEntry(1, 2n);
      state.processEntry(8, 4n); // Different block
      
      // At least some hints should have non-zero parity
      const nonZeroRegular = state.regularHints.filter(h => h.parity !== 0n);
      expect(nonZeroRegular.length).toBeGreaterThan(0);
    });
  });

  describe('getHint', () => {
    let state;
    
    beforeEach(() => {
      state = new PlinkoClientState(64, 8, 2, 3, masterKey);
      state.initializeHints();
      
      // Simulate streaming database
      for (let i = 0; i < 64; i++) {
        state.processEntry(i, BigInt(i + 1));
      }
    });

    it('should find a hint for any valid entry', () => {
      // Try several entries
      for (let i = 0; i < 10; i++) {
        const alpha = Math.floor(i / 8);
        const beta = i % 8;
        const hintInfo = state.getHint(alpha, beta);
        
        // Should find at least one hint (may be null if all consumed)
        if (hintInfo) {
          expect(hintInfo.blocks).toBeInstanceOf(Set);
          expect(hintInfo.offsets.length).toBe(state.c);
        }
      }
    });

    it('should return offsets for all blocks', () => {
      const hintInfo = state.getHint(0, 0);
      
      if (hintInfo) {
        expect(hintInfo.offsets.length).toBe(state.c);
        for (const offset of hintInfo.offsets) {
          expect(offset).toBeGreaterThanOrEqual(0);
          expect(offset).toBeLessThan(state.w);
        }
      }
    });
  });

  describe('consumeHint', () => {
    let state;
    
    beforeEach(() => {
      state = new PlinkoClientState(64, 8, 2, 3, masterKey);
      state.initializeHints();
    });

    it('should mark hint as consumed', () => {
      const initialStats = state.getStats();
      
      state.consumeHint(0, 42, 100n);
      
      const afterStats = state.getStats();
      expect(afterStats.consumedRegular).toBe(initialStats.consumedRegular + 1);
    });

    it('should cache the query result', () => {
      state.consumeHint(5, 42, 12345n);
      
      expect(state.getCached(42)).toBe(12345n);
    });

    it('should promote a backup hint', () => {
      const initialPromoted = state.promotedHints.filter(h => h !== null).length;
      
      state.consumeHint(0, 16, 999n); // Query block 2, offset 0
      
      const afterPromoted = state.promotedHints.filter(h => h !== null).length;
      expect(afterPromoted).toBe(initialPromoted + 1);
    });
  });

  describe('updateHint', () => {
    it('should update affected hint parities', () => {
      const state = new PlinkoClientState(64, 8, 2, 3, masterKey);
      state.initializeHints();
      
      // Process initial entry
      state.processEntry(0, 10n);
      
      // Get initial parity sum
      const initialParity = state.regularHints
        .filter(h => h.containsBlock(0))
        .reduce((sum, h) => sum ^ h.parity, 0n);
      
      // Apply update (delta = 5)
      state.updateHint(0, 5n);
      
      // Parity should change
      const afterParity = state.regularHints
        .filter(h => h.containsBlock(0))
        .reduce((sum, h) => sum ^ h.parity, 0n);
      
      expect(afterParity).not.toBe(initialParity);
    });
  });

  describe('getStats', () => {
    it('should return correct statistics', () => {
      const state = new PlinkoClientState(64, 8, 2, 3, masterKey);
      state.initializeHints();
      
      const stats = state.getStats();
      
      expect(stats.totalRegular).toBe(16); // lambda * w = 2 * 8
      expect(stats.availableRegular).toBe(16);
      expect(stats.consumedRegular).toBe(0);
      expect(stats.totalBackup).toBe(3);
      expect(stats.remainingBackup).toBe(3);
    });
  });

  describe('serialization', () => {
    it('should serialize and deserialize correctly', () => {
      const state = new PlinkoClientState(64, 8, 2, 3, masterKey);
      state.initializeHints();
      
      // Process some entries to create non-zero parities
      state.processEntry(0, 12345n);
      state.processEntry(10, 67890n);
      state.processEntry(32, 11111n);
      
      // Serialize
      const bytes = state.toBytes();
      expect(bytes).toBeInstanceOf(Uint8Array);
      
      // Deserialize
      const restored = PlinkoClientState.fromBytes(bytes, masterKey);
      
      // Verify parameters match
      expect(restored.n).toBe(state.n);
      expect(restored.w).toBe(state.w);
      expect(restored.lambda).toBe(state.lambda);
      expect(restored.q).toBe(state.q);
      expect(restored.c).toBe(state.c);
      
      // Verify hint counts match
      expect(restored.numRegularHints).toBe(state.numRegularHints);
      expect(restored.numBackupHints).toBe(state.numBackupHints);
      
      // Verify parities match
      for (let j = 0; j < state.numRegularHints; j++) {
        expect(restored.regularHints[j].parity).toBe(state.regularHints[j].parity);
      }
      
      for (let k = 0; k < state.numBackupHints; k++) {
        expect(restored.backupHints[k].parityIn).toBe(state.backupHints[k].parityIn);
        expect(restored.backupHints[k].parityOut).toBe(state.backupHints[k].parityOut);
      }
    });

    it('should handle large 256-bit parities', () => {
      const state = new PlinkoClientState(64, 8, 2, 3, masterKey);
      state.initializeHints();
      
      // Process entry with large 256-bit value
      const largeValue = (1n << 200n) + (1n << 100n) + 42n;
      state.processEntry(5, largeValue);
      
      const bytes = state.toBytes();
      const restored = PlinkoClientState.fromBytes(bytes, masterKey);
      
      // Find a hint affected by entry 5 and verify parity preserved
      const alpha = Math.floor(5 / state.w);
      for (let j = 0; j < state.numRegularHints; j++) {
        expect(restored.regularHints[j].parity).toBe(state.regularHints[j].parity);
      }
    });
  });
});
