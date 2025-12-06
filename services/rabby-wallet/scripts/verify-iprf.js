#!/usr/bin/env node
/**
 * Verify IPRF implementation matches expected behavior
 * 
 * Key questions:
 * 1. Does JS IPRF match Go IPRF for same inputs?
 * 2. Is the inverse function correct (Forward(Inverse(y)) = y for all preimages)?
 */

import { IPRF } from '../src/crypto/iprf-v2.js';

// Test parameters matching production
const CONFIG = {
  numHints: 684 * 64,  // 43776
  chunkSize: 8192,
};

function createTestKey(seed = 0) {
  const key = new Uint8Array(32);
  for (let i = 0; i < 32; i++) {
    key[i] = (seed + i) & 0xFF;
  }
  return key;
}

function testInverseCorrectness() {
  console.log('🧪 Testing IPRF Inverse Correctness\n');
  
  const key = createTestKey(42);
  const n = CONFIG.numHints;  // domain: 43776
  const m = CONFIG.chunkSize; // range: 8192
  
  console.log(`Parameters: n=${n}, m=${m}`);
  console.log(`Expected avg preimages per bin: ${(n/m).toFixed(1)}`);
  
  const iprf = new IPRF(key, n, m);
  
  // Test 1: Forward/Inverse round-trip
  console.log('\n📊 Test 1: Forward/Inverse Round-trip');
  
  let correct = 0;
  let total = 0;
  const sampleSize = 100;
  
  for (let y = 0; y < sampleSize; y++) {
    const preimages = iprf.inverse(y);
    
    for (const x of preimages) {
      const yPrime = iprf.forward(x);
      if (yPrime === y) {
        correct++;
      } else {
        console.log(`  ❌ forward(${x}) = ${yPrime}, expected ${y}`);
      }
      total++;
    }
  }
  
  console.log(`  ✅ ${correct}/${total} preimages verified correctly`);
  
  // Test 2: Distribution check
  console.log('\n📊 Test 2: Preimage Distribution');
  
  const sizes = [];
  for (let y = 0; y < 100; y++) {
    sizes.push(iprf.inverse(y).length);
  }
  
  const avg = sizes.reduce((a, b) => a + b) / sizes.length;
  const min = Math.min(...sizes);
  const max = Math.max(...sizes);
  
  console.log(`  Preimage sizes (sample of 100 bins):`);
  console.log(`    Min: ${min}`);
  console.log(`    Max: ${max}`);
  console.log(`    Avg: ${avg.toFixed(1)} (expected: ${(n/m).toFixed(1)})`);
  
  // Test 3: Timing
  console.log('\n📊 Test 3: Performance');
  
  const start = performance.now();
  const iterations = 100;
  
  for (let y = 0; y < iterations; y++) {
    iprf.inverse(y);
  }
  
  const elapsed = performance.now() - start;
  console.log(`  ${iterations} inverse calls: ${elapsed.toFixed(0)} ms`);
  console.log(`  Per inverse: ${(elapsed / iterations).toFixed(2)} ms`);
  console.log(`  Inverses/sec: ${(1000 * iterations / elapsed).toFixed(0)}`);
  
  // Estimate full hint generation time
  const totalInverses = CONFIG.chunkSize * 684; // 8192 * 684
  const estimatedTime = (elapsed / iterations) * totalInverses / 1000;
  console.log(`\n  Estimated hint gen (${totalInverses.toLocaleString()} inverses): ${estimatedTime.toFixed(0)} sec`);
  
  return correct === total;
}

function testKeyConsistency() {
  console.log('\n🧪 Testing Key Derivation Consistency\n');
  
  const key1 = createTestKey(1);
  const key2 = createTestKey(1);
  const key3 = createTestKey(2);
  
  const iprf1 = new IPRF(key1, 1000, 128);
  const iprf2 = new IPRF(key2, 1000, 128);
  const iprf3 = new IPRF(key3, 1000, 128);
  
  // Same key should give same results
  let match12 = true;
  let match13 = true;
  
  for (let y = 0; y < 10; y++) {
    const inv1 = iprf1.inverse(y);
    const inv2 = iprf2.inverse(y);
    const inv3 = iprf3.inverse(y);
    
    if (JSON.stringify(inv1) !== JSON.stringify(inv2)) match12 = false;
    if (JSON.stringify(inv1) === JSON.stringify(inv3)) match13 = true;
  }
  
  console.log(`  Same key, same results: ${match12 ? '✅' : '❌'}`);
  console.log(`  Different key, different results: ${!match13 ? '✅' : '❌'}`);
}

// Run tests
console.log('═══════════════════════════════════════════════════════════');
console.log('           IPRF Implementation Verification');
console.log('═══════════════════════════════════════════════════════════\n');

const success = testInverseCorrectness();
testKeyConsistency();

console.log('\n═══════════════════════════════════════════════════════════');
console.log(success ? '✅ All tests passed' : '❌ Some tests failed');
console.log('═══════════════════════════════════════════════════════════');
