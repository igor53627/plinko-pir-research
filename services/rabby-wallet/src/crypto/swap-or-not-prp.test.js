import { describe, it, expect } from 'vitest';
import { SwapOrNotPRP } from './swap-or-not-prp.js';

describe('SwapOrNotPRP', () => {
  const testKey = new Uint8Array(16);
  for (let i = 0; i < 16; i++) testKey[i] = i;

  describe('construction', () => {
    it('should create PRP with valid parameters', () => {
      const prp = new SwapOrNotPRP(testKey, 100);
      expect(prp.domainSize).toBe(100n);
      expect(prp.rounds).toBeGreaterThan(0);
    });

    it('should throw on invalid key length', () => {
      expect(() => new SwapOrNotPRP(new Uint8Array(15), 100)).toThrow();
      expect(() => new SwapOrNotPRP(new Uint8Array(17), 100)).toThrow();
    });

    it('should throw on invalid domain size', () => {
      expect(() => new SwapOrNotPRP(testKey, 0)).toThrow();
      expect(() => new SwapOrNotPRP(testKey, -1)).toThrow();
    });
  });

  describe('permutation properties', () => {
    it('should be a bijection (permute then inverse returns original)', () => {
      const prp = new SwapOrNotPRP(testKey, 100);
      
      for (let x = 0; x < 100; x++) {
        const y = prp.permute(BigInt(x));
        const xBack = prp.inverse(y);
        expect(xBack).toBe(BigInt(x));
      }
    });

    it('should map domain to domain', () => {
      const prp = new SwapOrNotPRP(testKey, 50);
      
      const outputs = new Set();
      for (let x = 0; x < 50; x++) {
        const y = prp.permute(BigInt(x));
        expect(y).toBeGreaterThanOrEqual(0n);
        expect(y).toBeLessThan(50n);
        outputs.add(Number(y));
      }
      
      // Should cover all outputs (bijection)
      expect(outputs.size).toBe(50);
    });

    it('should handle small domains', () => {
      const prp = new SwapOrNotPRP(testKey, 2);
      
      const y0 = prp.permute(0n);
      const y1 = prp.permute(1n);
      
      expect(y0).not.toBe(y1);
      expect(prp.inverse(y0)).toBe(0n);
      expect(prp.inverse(y1)).toBe(1n);
    });

    it('should handle values outside domain', () => {
      const prp = new SwapOrNotPRP(testKey, 50);
      
      // Values >= domainSize should pass through unchanged
      expect(prp.permute(100n)).toBe(100n);
      expect(prp.inverse(100n)).toBe(100n);
    });
  });

  describe('determinism', () => {
    it('should produce same output for same input', () => {
      const prp1 = new SwapOrNotPRP(testKey, 100);
      const prp2 = new SwapOrNotPRP(testKey, 100);
      
      for (let x = 0; x < 10; x++) {
        expect(prp1.permute(BigInt(x))).toBe(prp2.permute(BigInt(x)));
      }
    });

    it('should produce different output for different keys', () => {
      const key2 = new Uint8Array(16);
      for (let i = 0; i < 16; i++) key2[i] = i + 1;
      
      const prp1 = new SwapOrNotPRP(testKey, 100);
      const prp2 = new SwapOrNotPRP(key2, 100);
      
      // At least some outputs should differ
      let diffCount = 0;
      for (let x = 0; x < 20; x++) {
        if (prp1.permute(BigInt(x)) !== prp2.permute(BigInt(x))) {
          diffCount++;
        }
      }
      expect(diffCount).toBeGreaterThan(0);
    });
  });

  describe('round count', () => {
    it('should use 6*log2(N)+6 rounds', () => {
      const prp = new SwapOrNotPRP(testKey, 100);
      const expectedRounds = 6 * Math.ceil(Math.log2(101)) + 6;
      expect(prp.rounds).toBe(expectedRounds);
    });

    it('should scale rounds with domain size', () => {
      const prp10 = new SwapOrNotPRP(testKey, 10);
      const prp1000 = new SwapOrNotPRP(testKey, 1000);
      
      expect(prp1000.rounds).toBeGreaterThan(prp10.rounds);
    });
  });
});
