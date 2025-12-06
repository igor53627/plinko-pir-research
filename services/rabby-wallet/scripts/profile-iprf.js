#!/usr/bin/env node
/**
 * Profile IPRF implementation to identify bottlenecks
 */

import { performance } from 'perf_hooks';
import { IPRF } from '../src/crypto/iprf-v2.js';
import { FastAes128 } from '../src/crypto/aes128-fast.js';

// Production parameters
const CONFIG = {
  numHints: 684 * 64,  // 43776 (domain n)
  chunkSize: 8192,     // range m
};

function createTestKey(seed = 0) {
  const key = new Uint8Array(32);
  for (let i = 0; i < 32; i++) key[i] = (seed + i) & 0xFF;
  return key;
}

// ============= Component-level profiling =============

function profileAES() {
  console.log('\n📊 AES-128 T-table Performance:');
  
  const key = new Uint8Array(16);
  const aes = new FastAes128(key);
  const input = new Uint8Array(16);
  const output = new Uint8Array(16);
  
  const iterations = 1000000;
  const start = performance.now();
  
  for (let i = 0; i < iterations; i++) {
    input[0] = i & 0xff;
    aes.encryptBlock(input, output);
  }
  
  const elapsed = performance.now() - start;
  const opsPerSec = (iterations / elapsed) * 1000;
  
  console.log(`  ${iterations.toLocaleString()} encryptions: ${elapsed.toFixed(0)} ms`);
  console.log(`  Speed: ${(opsPerSec / 1e6).toFixed(2)} M ops/sec`);
  
  return opsPerSec;
}

function profileFeistelPRP() {
  console.log('\n📊 Feistel PRP Performance:');
  
  // Extract PRP from IPRF
  const key = createTestKey();
  const iprf = new IPRF(key, CONFIG.numHints, CONFIG.chunkSize);
  
  const iterations = 10000;
  
  // Profile forward (permute)
  let start = performance.now();
  for (let i = 0; i < iterations; i++) {
    iprf.prp.permute(BigInt(i % CONFIG.numHints));
  }
  let elapsed = performance.now() - start;
  console.log(`  Forward (permute): ${(elapsed / iterations).toFixed(3)} ms/op (${(iterations / elapsed * 1000).toFixed(0)} ops/sec)`);
  
  // Profile inverse
  start = performance.now();
  for (let i = 0; i < iterations; i++) {
    iprf.prp.inverse(BigInt(i % CONFIG.numHints));
  }
  elapsed = performance.now() - start;
  console.log(`  Inverse: ${(elapsed / iterations).toFixed(3)} ms/op (${(iterations / elapsed * 1000).toFixed(0)} ops/sec)`);
  
  // Count AES calls per operation
  console.log(`  Feistel rounds: 4 (4 AES calls per direction)`);
  console.log(`  Cycle-walking iterations: ~1-2 avg for n=${CONFIG.numHints}`);
}

function profilePMNS() {
  console.log('\n📊 PMNS (Ball-to-Bin) Performance:');
  
  const key = createTestKey();
  const iprf = new IPRF(key, CONFIG.numHints, CONFIG.chunkSize);
  
  const iterations = 1000;
  
  // Profile forward
  let start = performance.now();
  for (let i = 0; i < iterations; i++) {
    iprf.pmns.forward(BigInt(i % CONFIG.numHints));
  }
  let elapsed = performance.now() - start;
  console.log(`  Forward: ${(elapsed / iterations).toFixed(3)} ms/op`);
  
  // Profile backward (this is the expensive one!)
  start = performance.now();
  let totalPreimages = 0;
  for (let i = 0; i < iterations; i++) {
    const preimages = iprf.pmns.backward(BigInt(i % CONFIG.chunkSize));
    totalPreimages += preimages.length;
  }
  elapsed = performance.now() - start;
  const avgPreimages = totalPreimages / iterations;
  console.log(`  Backward: ${(elapsed / iterations).toFixed(3)} ms/op (avg ${avgPreimages.toFixed(1)} preimages)`);
  
  // Analyze tree depth
  const treeDepth = Math.ceil(Math.log2(CONFIG.chunkSize));
  console.log(`  Tree depth: ${treeDepth} levels (m=${CONFIG.chunkSize})`);
  console.log(`  Binomial samples per backward: ${treeDepth}`);
}

function profileBinomialSampling() {
  console.log('\n📊 Binomial Sampling Performance:');
  
  const key = createTestKey();
  const iprf = new IPRF(key, CONFIG.numHints, CONFIG.chunkSize);
  
  // The sampleBinomial is called with n = numHints at the root
  // and progressively smaller n at each level
  
  const iterations = 100;
  
  // Profile with full n (worst case - root of tree)
  let start = performance.now();
  for (let i = 0; i < iterations; i++) {
    iprf.pmns.sampleBinomial(BigInt(CONFIG.numHints), 0.5, 0n, BigInt(CONFIG.chunkSize - 1));
  }
  let elapsed = performance.now() - start;
  
  console.log(`  n=${CONFIG.numHints} (root): ${(elapsed / iterations).toFixed(2)} ms/sample`);
  
  // This is the KEY bottleneck!
  // sampleBinomial iterates through ALL n bits to count successes
  const bitsPerSample = CONFIG.numHints;
  const aesBlocksNeeded = Math.ceil(bitsPerSample / 128); // 128 bits per AES block
  console.log(`  Bits to process: ${bitsPerSample.toLocaleString()}`);
  console.log(`  AES blocks needed: ${aesBlocksNeeded}`);
  
  // Total AES calls for one PMNS backward:
  // At each level, we sample binomial with decreasing n
  // But worst case is dominated by root level
  const treeDepth = Math.ceil(Math.log2(CONFIG.chunkSize));
  console.log(`\n  ⚠️  BOTTLENECK IDENTIFIED:`);
  console.log(`  Each PMNS backward samples binomial ${treeDepth} times`);
  console.log(`  Root level processes ${CONFIG.numHints} bits = ${aesBlocksNeeded} AES blocks`);
  console.log(`  This is O(n) per inverse, not O(log m)!`);
}

function profileFullInverse() {
  console.log('\n📊 Full IPRF.inverse() Breakdown:');
  
  const key = createTestKey();
  const iprf = new IPRF(key, CONFIG.numHints, CONFIG.chunkSize);
  
  const iterations = 50;
  
  // Time breakdown
  let pmnsTime = 0;
  let prpTime = 0;
  let totalPreimages = 0;
  
  for (let y = 0; y < iterations; y++) {
    // PMNS backward
    let start = performance.now();
    const pmnsPreimages = iprf.pmns.backward(BigInt(y));
    pmnsTime += performance.now() - start;
    
    // PRP inverse for each preimage
    start = performance.now();
    for (const val of pmnsPreimages) {
      iprf.prp.inverse(val);
    }
    prpTime += performance.now() - start;
    
    totalPreimages += pmnsPreimages.length;
  }
  
  const avgPreimages = totalPreimages / iterations;
  const totalTime = pmnsTime + prpTime;
  
  console.log(`  PMNS backward: ${pmnsTime.toFixed(0)} ms (${(pmnsTime / totalTime * 100).toFixed(1)}%)`);
  console.log(`  PRP inverse: ${prpTime.toFixed(0)} ms (${(prpTime / totalTime * 100).toFixed(1)}%)`);
  console.log(`  Avg preimages: ${avgPreimages.toFixed(1)}`);
  console.log(`  Per inverse: ${(totalTime / iterations).toFixed(2)} ms`);
  
  // Estimate full hint generation
  const totalInverses = CONFIG.chunkSize * 684;
  const estimatedSec = (totalTime / iterations) * totalInverses / 1000;
  console.log(`\n  Estimated hint gen (${totalInverses.toLocaleString()} inverses): ${estimatedSec.toFixed(0)} sec`);
}

function analyzeComplexity() {
  console.log('\n' + '═'.repeat(60));
  console.log('                    COMPLEXITY ANALYSIS');
  console.log('═'.repeat(60));
  
  console.log(`
Paper claims: O(log m + k) per inverse
  where m = range size (${CONFIG.chunkSize})
  and k = avg preimages (~${(CONFIG.numHints / CONFIG.chunkSize).toFixed(1)})

Actual implementation:
  PMNS.backward() has O(log m) tree traversal
  BUT: sampleBinomial() at each level is O(n)!
  
  At root level: processes ALL ${CONFIG.numHints} domain elements
  This makes each inverse O(n), not O(log m + k)

The Problem:
  sampleBinomial(n, p, ...) iterates through n trials
  At tree root: n = ${CONFIG.numHints} (full domain)
  Each trial = 1 bit from AES output
  Total AES blocks = n/128 = ${Math.ceil(CONFIG.numHints / 128)}

Solutions:
  1. Normal approximation for large n (skip bit counting)
  2. Precompute binomial splits per tree node
  3. Use closed-form CDF instead of simulation
  4. Cache the tree structure
`);
}

// ============= Main =============

console.log('═'.repeat(60));
console.log('           IPRF PERFORMANCE PROFILING');
console.log('═'.repeat(60));
console.log(`\nParameters: n=${CONFIG.numHints}, m=${CONFIG.chunkSize}`);

profileAES();
profileFeistelPRP();
profilePMNS();
profileBinomialSampling();
profileFullInverse();
analyzeComplexity();

console.log('\n' + '═'.repeat(60));
console.log('                    RECOMMENDATIONS');
console.log('═'.repeat(60));
console.log(`
1. IMMEDIATE: Use normal approximation in sampleBinomial for n > 1000
   - Instead of counting ${CONFIG.numHints} bits, use single random + inverse CDF
   - Expected speedup: ~100x for root-level samples

2. MEDIUM: Cache the PMNS tree structure
   - Tree splits are deterministic for given (low, high, n)
   - Precompute once, reuse for all inverses

3. LONG-TERM: Port to WASM/Rust
   - Faster BigInt operations
   - Better memory layout
   - SIMD for bit counting
`);
