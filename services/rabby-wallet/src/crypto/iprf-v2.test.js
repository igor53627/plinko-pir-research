import { describe, it, expect } from 'vitest';
import { IPRF, SubsetGenerator } from './iprf-v2.js';

describe('IPRF v2', () => {
  const testKey = new Uint8Array(32);
  for (let i = 0; i < 32; i++) testKey[i] = i;

  describe('construction', () => {
    it('should create IPRF with valid parameters', () => {
      const iprf = new IPRF(testKey, 100, 16);
      expect(iprf.domainSize()).toBe(100n);
      expect(iprf.rangeSize()).toBe(16n);
    });

    it('should throw on invalid key length', () => {
      expect(() => new IPRF(new Uint8Array(16), 100, 16)).toThrow();
    });

    it('should throw on non-power-of-2 range', () => {
      expect(() => new IPRF(testKey, 100, 15)).toThrow();
    });
  });

  describe('forward evaluation', () => {
    it('should map domain to range', () => {
      const iprf = new IPRF(testKey, 100, 16);
      
      for (let x = 0; x < 100; x++) {
        const y = iprf.forward(x);
        expect(y).toBeGreaterThanOrEqual(0);
        expect(y).toBeLessThan(16);
      }
    });

    it('should be deterministic', () => {
      const iprf1 = new IPRF(testKey, 100, 16);
      const iprf2 = new IPRF(testKey, 100, 16);
      
      for (let x = 0; x < 20; x++) {
        expect(iprf1.forward(x)).toBe(iprf2.forward(x));
      }
    });
  });

  describe('inverse evaluation', () => {
    it('should return preimages that map to the given output', () => {
      const iprf = new IPRF(testKey, 100, 16);
      
      for (let y = 0; y < 16; y++) {
        const preimages = iprf.inverse(y);
        for (const x of preimages) {
          expect(iprf.forward(Number(x))).toBe(y);
        }
      }
    });

    it('should find all preimages', () => {
      const iprf = new IPRF(testKey, 50, 8);
      
      // Build forward mapping
      const forwardMap = new Map();
      for (let x = 0; x < 50; x++) {
        const y = iprf.forward(x);
        if (!forwardMap.has(y)) forwardMap.set(y, []);
        forwardMap.get(y).push(x);
      }
      
      // Check inverse returns all
      for (const [y, expected] of forwardMap) {
        const preimages = iprf.inverse(y).map(Number).sort((a, b) => a - b);
        expected.sort((a, b) => a - b);
        expect(preimages).toEqual(expected);
      }
    });

    it('should cover all domain elements across all inverse calls', () => {
      const iprf = new IPRF(testKey, 100, 16);
      
      const allPreimages = new Set();
      for (let y = 0; y < 16; y++) {
        for (const x of iprf.inverse(y)) {
          allPreimages.add(Number(x));
        }
      }
      
      expect(allPreimages.size).toBe(100);
    });
  });

  describe('expected preimage size', () => {
    it('should have O(n/m) preimages on average', () => {
      const n = 1000;
      const m = 64;
      const iprf = new IPRF(testKey, n, m);
      
      let totalPreimages = 0;
      for (let y = 0; y < m; y++) {
        totalPreimages += iprf.inverse(y).length;
      }
      
      const avgPreimages = totalPreimages / m;
      const expectedAvg = n / m;
      
      // Should be close to expected (within 50%)
      expect(avgPreimages).toBeGreaterThan(expectedAvg * 0.5);
      expect(avgPreimages).toBeLessThan(expectedAvg * 1.5);
    });
  });
});

describe('SubsetGenerator', () => {
  const testKey = new Uint8Array(16);
  for (let i = 0; i < 16; i++) testKey[i] = i;

  describe('generate', () => {
    it('should generate subset of correct size', () => {
      const gen = new SubsetGenerator(testKey);
      
      const subset = gen.generate(42, 10, 100);
      expect(subset.size).toBe(10);
    });

    it('should contain only valid indices', () => {
      const gen = new SubsetGenerator(testKey);
      
      const subset = gen.generate(0, 5, 20);
      for (const idx of subset) {
        expect(idx).toBeGreaterThanOrEqual(0);
        expect(idx).toBeLessThan(20);
      }
    });

    it('should be deterministic', () => {
      const gen1 = new SubsetGenerator(testKey);
      const gen2 = new SubsetGenerator(testKey);
      
      const subset1 = gen1.generate(123, 15, 50);
      const subset2 = gen2.generate(123, 15, 50);
      
      expect([...subset1].sort()).toEqual([...subset2].sort());
    });

    it('should produce different subsets for different seeds', () => {
      const gen = new SubsetGenerator(testKey);
      
      const subset1 = gen.generate(1, 10, 100);
      const subset2 = gen.generate(2, 10, 100);
      
      // Convert to sorted arrays for comparison
      const arr1 = [...subset1].sort((a, b) => a - b);
      const arr2 = [...subset2].sort((a, b) => a - b);
      
      // Should not be identical (very unlikely for random subsets)
      expect(arr1).not.toEqual(arr2);
    });

    it('should handle edge case of size = total', () => {
      const gen = new SubsetGenerator(testKey);
      
      const subset = gen.generate(0, 10, 10);
      expect(subset.size).toBe(10);
      
      // Should contain all elements
      for (let i = 0; i < 10; i++) {
        expect(subset.has(i)).toBe(true);
      }
    });

    it('should handle empty subset', () => {
      const gen = new SubsetGenerator(testKey);
      
      const subset = gen.generate(0, 0, 100);
      expect(subset.size).toBe(0);
    });
  });

  describe('contains', () => {
    it('should return true for elements in subset', () => {
      const gen = new SubsetGenerator(testKey);
      
      const subset = gen.generate(42, 10, 100);
      for (const idx of subset) {
        expect(gen.contains(42, 10, 100, idx)).toBe(true);
      }
    });

    it('should return false for elements not in subset', () => {
      const gen = new SubsetGenerator(testKey);
      
      const subset = gen.generate(42, 10, 100);
      for (let i = 0; i < 100; i++) {
        if (!subset.has(i)) {
          expect(gen.contains(42, 10, 100, i)).toBe(false);
        }
      }
    });
  });
});
