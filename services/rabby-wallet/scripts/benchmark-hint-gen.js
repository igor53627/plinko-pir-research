/**
 * Benchmark: Hint Generation with Different Block Sizes
 *
 * Compares 32B vs 4KB block sizes for Plinko PIR hint generation.
 * Run with: node scripts/benchmark-hint-gen.js
 */

import { IPRF } from '../src/crypto/iprf.js';
import { createHash } from 'crypto';

// Configuration - Real world scenario
// Same total data size, different block sizes
const TOTAL_BALANCES = 4194304; // ~4.2M balances (power of 2, close to 5.5M real)
const BALANCE_SIZE = 32; // 32 bytes per balance (256-bit)
const CHUNK_SIZE = 1024; // Must be power of 2 for IPRF
const NUM_RUNS = 1; // Single run for large scale

// Block configurations to test
const CONFIGS = [
  { name: '32B (1 balance/block)', blockSize: 32, balancesPerBlock: 1 },
  { name: '128B (4 balances/block) [PAPER]', blockSize: 128, balancesPerBlock: 4 },
  { name: '512B (16 balances/block)', blockSize: 512, balancesPerBlock: 16 },
  { name: '4KB (128 balances/block)', blockSize: 4096, balancesPerBlock: 128 },
];

// MurmurHash3-based block partition check (same as in worker)
function isBlockInP(hintIdx, blockIdx) {
  let h = BigInt(hintIdx) ^ (BigInt(blockIdx) << 32n);
  h ^= h >> 33n;
  h *= 0xff51afd7ed558ccdn;
  h ^= h >> 33n;
  h *= 0xc4ceb9fe1a85ec53n;
  h ^= h >> 33n;
  return (h & 1n) === 0n;
}

// Generate deterministic test data
function generateTestData(numEntries, blockSize) {
  const data = new Uint8Array(numEntries * blockSize);

  // Fill with pseudo-random data
  for (let i = 0; i < data.length; i += 32) {
    const hash = createHash('sha256').update(Buffer.from([i & 0xff, (i >> 8) & 0xff, (i >> 16) & 0xff, (i >> 24) & 0xff])).digest();
    for (let j = 0; j < 32 && i + j < data.length; j++) {
      data[i + j] = hash[j];
    }
  }

  return data;
}

// Generate master key and derive chunk keys
function generateKeys(setSize) {
  const masterKey = new Uint8Array(32);
  for (let i = 0; i < 32; i++) masterKey[i] = i * 7 + 13; // Deterministic

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

  return { masterKey, chunkKeys };
}

// Benchmark hint generation for a given block size
async function benchmarkHintGeneration(blockSize, numEntries, chunkSize) {
  const data = generateTestData(numEntries, blockSize);
  const dbSizeMB = (numEntries * blockSize) / 1024 / 1024;
  const setSize = Math.ceil(numEntries / chunkSize);
  const numHints = setSize * 64;

  console.log(`\n📊 Block Size: ${blockSize} bytes`);
  console.log(`   Database: ${dbSizeMB.toFixed(1)} MB (${numEntries.toLocaleString()} entries)`);
  console.log(`   Chunk size: ${chunkSize.toLocaleString()}`);
  console.log(`   Num hints: ${numHints.toLocaleString()}`);
  console.log(`   Hints size: ${((numHints * blockSize) / 1024 / 1024).toFixed(2)} MB`);

  const { chunkKeys } = generateKeys(setSize);

  // Create IPRFs
  const iprfs = chunkKeys.map(k => new IPRF(k, numHints, chunkSize));

  // Create hints buffer
  const hints = new Uint8Array(numHints * blockSize);
  const wordsPerBlock = blockSize / 4;
  const hintsU32 = new Uint32Array(hints.buffer);
  const dataU32 = new Uint32Array(data.buffer);

  const startTime = performance.now();

  // Generate hints (simplified version - processes subset for speed)
  const chunksToProcess = Math.min(setSize, 100); // Process first 100 chunks for benchmark

  for (let alpha = 0; alpha < chunksToProcess; alpha++) {
    const iprf = iprfs[alpha];

    // Pre-compute inverse table for this chunk
    const inverseTable = new Array(chunkSize);
    for (let beta = 0; beta < chunkSize; beta++) {
      const indices = iprf.inverse(beta);
      inverseTable[beta] = indices
        .map(h => Number(h))
        .filter(h => isBlockInP(h, alpha));
    }

    // Process all entries in this chunk
    const chunkStartIdx = alpha * chunkSize;
    const chunkEndIdx = Math.min(chunkStartIdx + chunkSize, numEntries);

    for (let i = chunkStartIdx; i < chunkEndIdx; i++) {
      const beta = i - chunkStartIdx;
      const valOffsetU32 = i * wordsPerBlock;

      if (valOffsetU32 + wordsPerBlock > dataU32.length) break;

      for (const hintIdx of inverseTable[beta]) {
        const hOffsetU32 = hintIdx * wordsPerBlock;

        // XOR all words in the block
        for (let w = 0; w < wordsPerBlock; w++) {
          hintsU32[hOffsetU32 + w] ^= dataU32[valOffsetU32 + w];
        }
      }
    }

    if ((alpha + 1) % 20 === 0) {
      process.stdout.write(`\r   Processing chunk ${alpha + 1}/${chunksToProcess}...`);
    }
  }

  const elapsed = performance.now() - startTime;
  const chunksPerSec = chunksToProcess / (elapsed / 1000);
  const estimatedTotalTime = (setSize / chunksPerSec);

  console.log(`\r   ✅ Processed ${chunksToProcess} chunks in ${elapsed.toFixed(0)}ms`);
  console.log(`   ⏱️  Rate: ${chunksPerSec.toFixed(1)} chunks/sec`);
  console.log(`   📈 Estimated full generation: ${estimatedTotalTime.toFixed(1)}s for ${setSize} chunks`);

  return {
    blockSize,
    numEntries,
    elapsed,
    chunksProcessed: chunksToProcess,
    chunksPerSec,
    estimatedTotalTime,
    hintsSize: numHints * blockSize
  };
}

// Main benchmark
async function main() {
  console.log('═══════════════════════════════════════════════════════════');
  console.log('  Plinko PIR Hint Generation Benchmark');
  console.log('  Real-world scenario: Same total balances, different blocking');
  console.log('═══════════════════════════════════════════════════════════');
  console.log(`Total balances: ${TOTAL_BALANCES.toLocaleString()}`);
  console.log(`Balance size: ${BALANCE_SIZE} bytes`);
  console.log(`Total data: ${((TOTAL_BALANCES * BALANCE_SIZE) / 1024 / 1024).toFixed(1)} MB`);
  console.log(`Chunk size: ${CHUNK_SIZE} (power of 2 for IPRF)`);
  console.log(`Runs per config: ${NUM_RUNS}`);

  const results = {};

  for (const config of CONFIGS) {
    const numEntries = Math.ceil(TOTAL_BALANCES / config.balancesPerBlock);
    results[config.name] = [];

    console.log(`\n${'═'.repeat(60)}`);
    console.log(`  ${config.name}`);
    console.log(`  Entries: ${numEntries.toLocaleString()} (${config.balancesPerBlock} balance(s) per block)`);
    console.log(`${'═'.repeat(60)}`);

    for (let run = 0; run < NUM_RUNS; run++) {
      console.log(`\n--- Run ${run + 1}/${NUM_RUNS} ---`);
      const result = await benchmarkHintGeneration(config.blockSize, numEntries, CHUNK_SIZE);
      result.config = config;
      results[config.name].push(result);
    }
  }

  // Summary
  console.log('\n═══════════════════════════════════════════════════════════');
  console.log('  SUMMARY');
  console.log('═══════════════════════════════════════════════════════════');

  for (const config of CONFIGS) {
    const runs = results[config.name];
    const avgTime = runs.reduce((s, r) => s + r.estimatedTotalTime, 0) / runs.length;
    const avgRate = runs.reduce((s, r) => s + r.chunksPerSec, 0) / runs.length;
    const hintsSize = runs[0].hintsSize;

    console.log(`\n${config.name}:`);
    console.log(`  Entries: ${runs[0].numEntries?.toLocaleString() || 'N/A'}`);
    console.log(`  Avg rate: ${avgRate.toFixed(1)} chunks/sec`);
    console.log(`  Avg estimated time: ${avgTime.toFixed(1)}s`);
    console.log(`  Hints storage: ${(hintsSize / 1024 / 1024).toFixed(2)} MB`);
  }

  // Comparison
  const config32 = CONFIGS[0];
  const config4k = CONFIGS[1];
  const avg32 = results[config32.name].reduce((s, r) => s + r.estimatedTotalTime, 0) / NUM_RUNS;
  const avg4k = results[config4k.name].reduce((s, r) => s + r.estimatedTotalTime, 0) / NUM_RUNS;
  const speedup = avg32 / avg4k;

  console.log('\n───────────────────────────────────────────────────────────');
  console.log(`SPEEDUP: ${speedup.toFixed(1)}x ${speedup > 1 ? '(4KB FASTER)' : '(32B faster)'}`);
  console.log(`Hints size: ${(results[config4k.name][0].hintsSize / 1024 / 1024).toFixed(2)} MB vs ${(results[config32.name][0].hintsSize / 1024 / 1024).toFixed(2)} MB`);
  console.log(`IPRF operations reduced: ${config4k.balancesPerBlock}x fewer with 4KB blocks`);
}

main().catch(console.error);
