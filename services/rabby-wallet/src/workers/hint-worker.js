/**
 * Hint Generation Web Worker
 * 
 * Processes a range of chunks in parallel to generate Plinko hints.
 * Each worker handles independent chunks, then results are XORed together.
 */

import { IPRF } from '../crypto/iprf-v2.js';

// Worker state
let iprfs = null;
let numHints = 0;
let chunkSize = 0;
let chunkStart = 0;

/**
 * MurmurHash3-based block partition check
 */
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
 * Initialize worker with IPRF keys
 */
function initialize(data) {
  const { chunkKeys, metadata, chunkStartIdx } = data;
  
  numHints = metadata.setSize * 64;
  chunkSize = metadata.chunkSize;
  chunkStart = chunkStartIdx;
  
  // Create IPRFs for assigned chunk range
  // Keys are indexed 0..N for this worker's chunks
  iprfs = chunkKeys.map(keyBytes => {
    const key = new Uint8Array(keyBytes);
    return new IPRF(key, numHints, chunkSize);
  });
  
  self.postMessage({ type: 'initialized' });
}

/**
 * Process assigned chunks and generate partial hints
 */
function processChunks(data) {
  const { chunkEnd, snapshotBytes, dbSize } = data;
  
  // Create partial hints buffer
  const partialHints = new Uint8Array(numHints * 32);
  const hintsU32 = new Uint32Array(partialHints.buffer);
  
  // Create view for snapshot
  const snapshot = new Uint8Array(snapshotBytes);
  const dbU32 = new Uint32Array(snapshot.buffer, snapshot.byteOffset, Math.floor(snapshot.byteLength / 4));
  
  let processedChunks = 0;
  const totalChunks = chunkEnd - chunkStart;
  
  for (let alpha = chunkStart; alpha < chunkEnd; alpha++) {
    // Local index into iprfs array (0-based for this worker)
    const localIdx = alpha - chunkStart;
    const iprf = iprfs[localIdx];
    
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
    const chunkEndIdx = Math.min(chunkStartIdx + chunkSize, dbSize);
    
    for (let i = chunkStartIdx; i < chunkEndIdx; i++) {
      const beta = i - chunkStartIdx;
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
    
    processedChunks++;
    
    // Report progress every chunk for responsive UI
    self.postMessage({
      type: 'progress',
      processed: processedChunks,
      total: totalChunks
    });
  }
  
  // Transfer the buffer back to main thread
  self.postMessage(
    { type: 'complete', partialHints },
    [partialHints.buffer]
  );
}

// Message handler
self.onmessage = (e) => {
  const { type, ...data } = e.data;
  
  switch (type) {
    case 'initialize':
      initialize(data);
      break;
    case 'process':
      processChunks(data);
      break;
    default:
      console.error('Unknown message type:', type);
  }
};
