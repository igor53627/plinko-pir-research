#!/usr/bin/env node
/**
 * Benchmark hint generation with different hash algorithms
 * 
 * Usage: node scripts/benchmark-hints.js [database.bin path]
 */

import { readFileSync, existsSync } from 'fs';
import { createHash } from 'crypto';
import { performance } from 'perf_hooks';

// Import our crypto modules
import { FastAes128 } from '../src/crypto/aes128-fast.js';
import { IPRF } from '../src/crypto/iprf-v2.js';

// Try to import blake3 if available
let blake3 = null;
try {
  blake3 = await import('blake3');
  console.log('✅ Blake3 available');
} catch {
  console.log('⚠️  Blake3 not installed (npm install blake3 to test)');
}

// ============= Configuration =============
const CONFIG = {
  dbSize: 5607168,      // From manifest
  chunkSize: 8192,      // From manifest  
  setSize: 684,         // From manifest
  numHints: 684 * 64,   // setSize * 64
  valueSize: 32,        // bytes per entry
};

// ============= Hash Algorithm Implementations =============

/**
 * Current: AES-based PRF (T-table optimized)
 */
class AesPRF {
  constructor(key) {
    this.aes = new FastAes128(key.slice(0, 16));
    this.input = new Uint8Array(16);
    this.output = new Uint8Array(16);
  }
  
  evaluate(data) {
    this.input.set(data.slice(0, 16));
    return this.aes.encryptBlock(this.input, this.output);
  }
}

/**
 * Alternative: Blake3-based PRF
 */
class Blake3PRF {
  constructor(key) {
    this.key = key;
  }
  
  evaluate(data) {
    if (!blake3) throw new Error('Blake3 not available');
    const input = new Uint8Array(this.key.length + data.length);
    input.set(this.key);
    input.set(data, this.key.length);
    return blake3.hash(input).slice(0, 16);
  }
}

/**
 * Alternative: SHA256-based PRF (using Node crypto)
 */
class Sha256PRF {
  constructor(key) {
    this.key = Buffer.from(key);
  }
  
  evaluate(data) {
    const hash = createHash('sha256');
    hash.update(this.key);
    hash.update(Buffer.from(data));
    return new Uint8Array(hash.digest().slice(0, 16));
  }
}

// ============= Benchmark Functions =============

function isBlockInP(hintIdx, blockIdx) {
  let h = BigInt(hintIdx) ^ (BigInt(blockIdx) << 32n);
  h ^= h >> 33n;
  h *= 0xff51afd7ed558ccdn;
  h ^= h >> 33n;
  h *= 0xc4ceb9fe1a85ec53n;
  h ^= h >> 33n;
  return (h & 1n) === 0n;
}

/**
 * Benchmark raw PRF speed
 */
function benchmarkPRF(name, prfFactory, iterations = 100000) {
  const key = new Uint8Array(32);
  for (let i = 0; i < 32; i++) key[i] = i;
  
  const prf = prfFactory(key);
  const input = new Uint8Array(16);
  
  const start = performance.now();
  for (let i = 0; i < iterations; i++) {
    input[0] = i & 0xff;
    input[1] = (i >> 8) & 0xff;
    prf.evaluate(input);
  }
  const elapsed = performance.now() - start;
  
  const opsPerSec = (iterations / elapsed) * 1000;
  const mbPerSec = (iterations * 16 / 1024 / 1024) / (elapsed / 1000);
  
  console.log(`  ${name}: ${opsPerSec.toFixed(0)} ops/sec (${mbPerSec.toFixed(1)} MB/s)`);
  return { name, opsPerSec, mbPerSec };
}

/**
 * Benchmark IPRF inverse operations (the bottleneck)
 */
function benchmarkIPRFInverse(numChunks = 10) {
  console.log(`\n📊 IPRF Inverse Benchmark (${numChunks} chunks × ${CONFIG.chunkSize} betas):`);
  
  const key = new Uint8Array(32);
  for (let i = 0; i < 32; i++) key[i] = i;
  
  const iprf = new IPRF(key, CONFIG.numHints, CONFIG.chunkSize);
  
  const start = performance.now();
  let totalPreimages = 0;
  
  for (let chunk = 0; chunk < numChunks; chunk++) {
    for (let beta = 0; beta < CONFIG.chunkSize; beta++) {
      const preimages = iprf.inverse(beta);
      totalPreimages += preimages.length;
    }
  }
  
  const elapsed = performance.now() - start;
  const inversesPerSec = (numChunks * CONFIG.chunkSize / elapsed) * 1000;
  const chunksPerSec = (numChunks / elapsed) * 1000;
  
  console.log(`  Total inverse calls: ${(numChunks * CONFIG.chunkSize).toLocaleString()}`);
  console.log(`  Total preimages found: ${totalPreimages.toLocaleString()}`);
  console.log(`  Time: ${elapsed.toFixed(0)} ms`);
  console.log(`  Speed: ${inversesPerSec.toFixed(0)} inverses/sec`);
  console.log(`  Chunks/sec: ${chunksPerSec.toFixed(2)}`);
  console.log(`  Estimated full hint gen: ${(CONFIG.setSize / chunksPerSec).toFixed(1)} sec`);
  
  return { elapsed, inversesPerSec, chunksPerSec };
}

/**
 * Benchmark full hint generation on real data
 */
function benchmarkHintGeneration(dbPath, numChunks = null) {
  if (!existsSync(dbPath)) {
    console.log(`\n⚠️  Database not found: ${dbPath}`);
    console.log('   Download from CDN or use test data');
    return null;
  }
  
  const dbBytes = readFileSync(dbPath);
  const actualDbSize = dbBytes.length / CONFIG.valueSize;
  const actualChunks = Math.ceil(actualDbSize / CONFIG.chunkSize);
  
  console.log(`\n📊 Hint Generation Benchmark:`);
  console.log(`  Database: ${dbPath}`);
  console.log(`  Size: ${(dbBytes.length / 1024 / 1024).toFixed(1)} MB`);
  console.log(`  Entries: ${actualDbSize.toLocaleString()}`);
  console.log(`  Chunks: ${actualChunks}`);
  
  const chunksToProcess = numChunks || Math.min(actualChunks, 50);
  console.log(`  Processing: ${chunksToProcess} chunks`);
  
  // Generate master key and chunk keys
  const masterKey = new Uint8Array(32);
  for (let i = 0; i < 32; i++) masterKey[i] = i;
  
  const chunkKeys = [];
  for (let i = 0; i < chunksToProcess; i++) {
    const k = new Uint8Array(32);
    for (let j = 0; j < 32; j++) k[j] = masterKey[j];
    let idx = i;
    for (let j = 0; j < 8; j++) {
      k[j] ^= idx & 0xFF;
      idx >>= 8;
    }
    chunkKeys.push(k);
  }
  
  // Create IPRFs
  const numHints = CONFIG.setSize * 64;
  const iprfs = chunkKeys.map(k => new IPRF(k, numHints, CONFIG.chunkSize));
  
  // Allocate hints buffer
  const hints = new Uint8Array(numHints * 32);
  const hintsU32 = new Uint32Array(hints.buffer);
  const dbU32 = new Uint32Array(dbBytes.buffer);
  
  console.log(`  Hints buffer: ${(hints.length / 1024 / 1024).toFixed(1)} MB`);
  
  const start = performance.now();
  
  for (let alpha = 0; alpha < chunksToProcess; alpha++) {
    const iprf = iprfs[alpha];
    
    // Pre-compute inverse table
    const inverseTable = new Array(CONFIG.chunkSize);
    for (let beta = 0; beta < CONFIG.chunkSize; beta++) {
      const indices = iprf.inverse(beta);
      inverseTable[beta] = indices
        .map(h => Number(h))
        .filter(h => isBlockInP(h, alpha));
    }
    
    // XOR values into hints
    const chunkStart = alpha * CONFIG.chunkSize;
    const chunkEnd = Math.min(chunkStart + CONFIG.chunkSize, actualDbSize);
    
    for (let i = chunkStart; i < chunkEnd; i++) {
      const beta = i - chunkStart;
      const valOffsetU32 = i * 8;
      
      if (valOffsetU32 + 8 > dbU32.length) break;
      
      const w0 = dbU32[valOffsetU32];
      const w1 = dbU32[valOffsetU32 + 1];
      const w2 = dbU32[valOffsetU32 + 2];
      const w3 = dbU32[valOffsetU32 + 3];
      const w4 = dbU32[valOffsetU32 + 4];
      const w5 = dbU32[valOffsetU32 + 5];
      const w6 = dbU32[valOffsetU32 + 6];
      const w7 = dbU32[valOffsetU32 + 7];
      
      for (const hintIdx of inverseTable[beta]) {
        const hOffsetU32 = hintIdx * 8;
        hintsU32[hOffsetU32] ^= w0;
        hintsU32[hOffsetU32 + 1] ^= w1;
        hintsU32[hOffsetU32 + 2] ^= w2;
        hintsU32[hOffsetU32 + 3] ^= w3;
        hintsU32[hOffsetU32 + 4] ^= w4;
        hintsU32[hOffsetU32 + 5] ^= w5;
        hintsU32[hOffsetU32 + 6] ^= w6;
        hintsU32[hOffsetU32 + 7] ^= w7;
      }
    }
    
    if ((alpha + 1) % 10 === 0) {
      const pct = ((alpha + 1) / chunksToProcess * 100).toFixed(0);
      process.stdout.write(`\r  Progress: ${pct}% (${alpha + 1}/${chunksToProcess})`);
    }
  }
  
  const elapsed = performance.now() - start;
  console.log(`\r  Progress: 100% (${chunksToProcess}/${chunksToProcess})`);
  
  const chunksPerSec = chunksToProcess / (elapsed / 1000);
  const estimatedFull = CONFIG.setSize / chunksPerSec;
  
  console.log(`\n  Results:`);
  console.log(`  Time: ${(elapsed / 1000).toFixed(2)} sec`);
  console.log(`  Chunks/sec: ${chunksPerSec.toFixed(2)}`);
  console.log(`  Estimated full (${CONFIG.setSize} chunks): ${estimatedFull.toFixed(1)} sec`);
  
  return { elapsed, chunksPerSec, estimatedFull };
}

// ============= Main =============

async function main() {
  console.log('🔬 Plinko PIR Hint Generation Benchmark\n');
  console.log('Configuration:');
  console.log(`  DB Size: ${CONFIG.dbSize.toLocaleString()} entries`);
  console.log(`  Chunk Size: ${CONFIG.chunkSize}`);
  console.log(`  Set Size: ${CONFIG.setSize} chunks`);
  console.log(`  Num Hints: ${CONFIG.numHints.toLocaleString()}`);
  
  // 1. Raw PRF benchmarks
  console.log('\n📊 Raw PRF Speed (100k iterations):');
  benchmarkPRF('AES-128 (T-table)', (key) => new AesPRF(key));
  benchmarkPRF('SHA-256 (Node)', (key) => new Sha256PRF(key));
  if (blake3) {
    benchmarkPRF('Blake3', (key) => new Blake3PRF(key));
  }
  
  // 2. IPRF inverse benchmark
  benchmarkIPRFInverse(10);
  
  // 3. Full hint generation on real data
  const dbPath = process.argv[2] || '../../test-data/database.bin.tmp';
  benchmarkHintGeneration(dbPath, 50);
  
  console.log('\n✅ Benchmark complete');
  
  // Summary about zipped data
  console.log('\n📝 Note on compressed data:');
  console.log('   PIR requires XOR operations on actual values.');
  console.log('   XOR(compress(A), compress(B)) ≠ compress(XOR(A, B))');
  console.log('   So hints cannot be computed on compressed data.');
}

main().catch(console.error);
