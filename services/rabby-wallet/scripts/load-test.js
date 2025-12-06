#!/usr/bin/env node
/**
 * Load test: Benchmark hint generation with synthetic or real database
 * 
 * Usage:
 *   node scripts/load-test.js                    # Use synthetic data (5.6M entries)
 *   node scripts/load-test.js /path/to/db.bin   # Use real database file
 */

import { performance } from 'perf_hooks';
import { existsSync, readFileSync } from 'fs';
import { IPRF } from '../src/crypto/iprf-v2.js';

const DB_PATH = process.argv[2];

function generateSyntheticDatabase(config) {
  const { dbSize } = config;
  const entrySize = 32; // 32 bytes per entry
  const totalBytes = dbSize * entrySize;
  
  console.log(`📦 Generating synthetic database...`);
  console.log(`   Entries: ${dbSize.toLocaleString()}`);
  console.log(`   Size: ${(totalBytes / 1024 / 1024).toFixed(1)} MB`);
  
  const buffer = new Uint8Array(totalBytes);
  
  // Fill with pseudo-random data (deterministic for reproducibility)
  let seed = 12345;
  for (let i = 0; i < buffer.length; i++) {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff;
    buffer[i] = seed & 0xff;
  }
  
  console.log(`   ✅ Generated`);
  return buffer;
}

function loadDatabase(path, config) {
  if (path && existsSync(path)) {
    console.log(`📦 Loading database from ${path}...`);
    const buffer = readFileSync(path);
    console.log(`   Size: ${(buffer.length / 1024 / 1024).toFixed(1)} MB`);
    return new Uint8Array(buffer);
  }
  
  return generateSyntheticDatabase(config);
}

function isBlockInP(hintIdx, blockIdx) {
  let h = BigInt(hintIdx) ^ (BigInt(blockIdx) << 32n);
  h ^= h >> 33n;
  h *= 0xff51afd7ed558ccdn;
  h ^= h >> 33n;
  h *= 0xc4ceb9fe1a85ec53n;
  h ^= h >> 33n;
  return (h & 1n) === 0n;
}

async function benchmarkHintGeneration(dbBytes, config) {
  const { dbSize, chunkSize, setSize } = config;
  const numHints = setSize * 64;
  const numChunks = setSize; // Use setSize as numChunks (they should match)
  
  console.log(`\n📊 Hint Generation Benchmark:`);
  console.log(`   DB entries: ${dbSize.toLocaleString()}`);
  console.log(`   Chunks: ${numChunks}`);
  console.log(`   Hints: ${numHints.toLocaleString()}`);
  
  // Generate master key
  const masterKey = new Uint8Array(32);
  for (let i = 0; i < 32; i++) masterKey[i] = i;
  
  // Derive chunk keys
  const chunkKeys = [];
  for (let i = 0; i < setSize; i++) {
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
  console.log(`   Creating ${setSize} IPRFs...`);
  const iprfStart = performance.now();
  const iprfs = chunkKeys.map(k => new IPRF(k, numHints, chunkSize));
  console.log(`   IPRFs created in ${((performance.now() - iprfStart) / 1000).toFixed(2)}s`);
  
  // Allocate hints buffer
  const hints = new Uint8Array(numHints * 32);
  const hintsU32 = new Uint32Array(hints.buffer);
  const dbU32 = new Uint32Array(dbBytes.buffer, dbBytes.byteOffset, Math.floor(dbBytes.byteLength / 4));
  
  console.log(`   Hints buffer: ${(hints.byteLength / 1024 / 1024).toFixed(1)} MB`);
  console.log(`\n   Processing chunks...`);
  
  const startTime = performance.now();
  let lastLogTime = startTime;
  
  for (let alpha = 0; alpha < numChunks; alpha++) {
    const iprf = iprfs[alpha];
    
    // Pre-compute inverse table
    const inverseTable = new Array(chunkSize);
    for (let beta = 0; beta < chunkSize; beta++) {
      const indices = iprf.inverse(beta);
      inverseTable[beta] = indices
        .map(h => Number(h))
        .filter(h => isBlockInP(h, alpha));
    }
    
    // XOR values into hints
    const chunkStart = alpha * chunkSize;
    const chunkEnd = Math.min(chunkStart + chunkSize, dbSize);
    
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
    
    // Progress
    const now = performance.now();
    if (now - lastLogTime > 5000) {
      const pct = ((alpha + 1) / numChunks * 100).toFixed(1);
      const elapsed = (now - startTime) / 1000;
      const rate = (alpha + 1) / elapsed;
      const eta = (numChunks - alpha - 1) / rate;
      console.log(`   ${pct}% (${alpha + 1}/${numChunks}) - ${rate.toFixed(1)} chunks/s - ETA: ${eta.toFixed(0)}s`);
      lastLogTime = now;
    }
  }
  
  const totalTime = (performance.now() - startTime) / 1000;
  const chunksPerSec = numChunks / totalTime;
  
  console.log(`\n   ✅ Complete!`);
  console.log(`   Total time: ${totalTime.toFixed(1)}s`);
  console.log(`   Chunks/sec: ${chunksPerSec.toFixed(1)}`);
  console.log(`   Hints size: ${(hints.byteLength / 1024 / 1024).toFixed(1)} MB`);
  
  return { totalTime, chunksPerSec, hints };
}

async function main() {
  console.log('═'.repeat(60));
  console.log('         PLINKO PIR LOAD TEST');
  console.log('═'.repeat(60));
  
  // Production config
  const config = {
    dbSize: 5607168,
    chunkSize: 8192,
    setSize: 684,
  };
  
  console.log(`\nConfig: ${config.dbSize.toLocaleString()} entries, ${config.setSize} chunks`);
  
  try {
    // Load or generate database
    const dbBytes = loadDatabase(DB_PATH, config);
    
    // Run benchmark
    await benchmarkHintGeneration(dbBytes, config);
    
    console.log('\n' + '═'.repeat(60));
    console.log('         LOAD TEST COMPLETE');
    console.log('═'.repeat(60));
    
  } catch (err) {
    console.error(`\n❌ Error: ${err.message}`);
    console.error(err.stack);
    process.exit(1);
  }
}

main();
