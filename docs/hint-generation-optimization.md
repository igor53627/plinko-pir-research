# Hint Generation Optimization

## Current Bottleneck Analysis

The hint generation processes 684 chunks × 8192 betas = **5.6M IPRF inverse operations**.

Each IPRF inverse involves:
1. **PMNS backward** - AES-based binomial sampling (~16 AES blocks per call)
2. **Feistel PRP inverse** - 4 rounds of AES (~4 AES blocks)

**Total: ~100M+ AES block encryptions** 

Current JS AES: ~2-5 MB/s → **Minutes of computation**

## Optimization Strategies

### 1. Web Workers (Parallel Chunks) ✅ Recommended

Split chunks across CPU cores. Each worker processes independent chunks.

```
Main Thread                    Workers (4-8)
     │                              │
     │── chunk 0-85 ──────────────► Worker 1
     │── chunk 86-171 ────────────► Worker 2  
     │── chunk 172-257 ───────────► Worker 3
     │── chunk 258-341 ───────────► Worker 4
     │── chunk 342-427 ───────────► Worker 5
     │── chunk 428-513 ───────────► Worker 6
     │── chunk 514-599 ───────────► Worker 7
     │── chunk 600-683 ───────────► Worker 8
     │                              │
     │◄─── partial hints XOR ───────│
     │                              │
```

**Expected speedup: 4-8x** (limited by cores)

### 2. Hardware-Accelerated AES

#### Option A: WebCrypto API (AES-NI)
```javascript
// Uses native AES-NI when available
// BUT: async API adds overhead for small blocks
const key = await crypto.subtle.importKey('raw', keyBytes, 'AES-CBC', false, ['encrypt']);
const encrypted = await crypto.subtle.encrypt({ name: 'AES-CBC', iv }, key, data);
```

**Problem:** WebCrypto is async, adds ~0.1ms overhead per call. Not good for millions of small blocks.

#### Option B: WASM with SIMD (Recommended)
```javascript
// noble-ciphers uses optimized WASM
import { aes } from '@noble/ciphers/aes';
const cipher = aes(key, nonce);
cipher.encrypt(data);  // ~100-200 MB/s
```

**Expected speedup: 10-50x over pure JS**

### 3. Batch AES Operations

Instead of encrypting one 16-byte block at a time, batch them:

```javascript
// Before: 1 block at a time
for (let i = 0; i < 8192; i++) {
  aes.encryptBlock(input[i], output[i]);  // overhead per call
}

// After: batch all blocks
const allInputs = new Uint8Array(8192 * 16);
const allOutputs = new Uint8Array(8192 * 16);
aes.encryptBlocks(allInputs, allOutputs);  // single call, vectorized
```

**Expected speedup: 2-5x**

### 4. Alternative Hash Functions

#### Blake3 vs AES for PRF

| Property | AES-128 | Blake3 |
|----------|---------|--------|
| Speed (WASM) | ~200 MB/s | ~500 MB/s |
| HW Accel | AES-NI | AVX2/NEON |
| Security | Block cipher | Hash function |
| WASM size | ~10 KB | ~50 KB |

**Blake3 advantages:**
- 2-3x faster in WASM
- Native SIMD support
- Variable output length
- Tree hashing (parallelizable)

**Consideration:** Changing from AES to Blake3 requires updating both client and server IPRF implementations. The cryptographic security properties are different but both are suitable for PRF construction.

```javascript
// Blake3-based PRF (conceptual)
import { blake3 } from 'blake3/browser';

function prf(key, input) {
  return blake3.hash(concat(key, input), { length: 16 });
}
```

## Implementation Plan

### Phase 1: Web Workers (Immediate, 4-8x speedup)

```javascript
// hint-worker.js
self.onmessage = async ({ data }) => {
  const { chunkRange, snapshotBytes, iprfKeys, metadata } = data;
  const partialHints = new Uint8Array(numHints * 32);
  
  for (let alpha = chunkRange.start; alpha < chunkRange.end; alpha++) {
    // Process chunk...
  }
  
  self.postMessage({ partialHints }, [partialHints.buffer]);
};
```

### Phase 2: WASM AES (10-50x speedup)

```javascript
// Use @noble/ciphers for optimized AES
import { aes } from '@noble/ciphers/aes';

class FastAes128 {
  constructor(key) {
    this.cipher = aes(key);
  }
  
  encryptBlock(block) {
    return this.cipher.encrypt(block);
  }
  
  encryptBlocks(blocks) {
    // Batch encryption
    return this.cipher.encrypt(blocks);
  }
}
```

### Phase 3: Blake3 Migration (Optional, additional 2-3x)

```javascript
// Requires coordinated client + server change
import { createHash } from 'blake3/browser';

class Blake3PRF {
  constructor(key) {
    this.key = key;
  }
  
  evaluate(input) {
    const h = createHash();
    h.update(this.key);
    h.update(input);
    return h.digest().slice(0, 16);
  }
}
```

## Benchmark Expectations

| Optimization | Current | Optimized | Speedup |
|--------------|---------|-----------|---------|
| Baseline (JS AES) | 180s | - | 1x |
| + Web Workers (8) | 180s | 25s | 7x |
| + WASM AES | 25s | 2.5s | 70x |
| + Blake3 | 2.5s | 1s | 180x |

## Code Changes Required

### Client-side
1. `hint-worker.js` - New worker for parallel processing
2. `aes128.js` → `aes128-fast.js` - WASM-based AES
3. `plinko-pir-client.js` - Orchestrate workers

### Server-side (for Blake3 migration only)
1. `iprf.go` - Update PRF implementation
2. Coordinated deployment required

## Compatibility

| Browser | Web Workers | WASM SIMD | WebCrypto |
|---------|-------------|-----------|-----------|
| Chrome 91+ | ✅ | ✅ | ✅ |
| Firefox 89+ | ✅ | ✅ | ✅ |
| Safari 16.4+ | ✅ | ✅ | ✅ |
| Edge 91+ | ✅ | ✅ | ✅ |

## Risks & Mitigations

1. **Worker memory**: Each worker needs snapshot copy
   - Mitigation: Use SharedArrayBuffer if available
   
2. **WASM loading time**: ~50-100ms initial load
   - Mitigation: Lazy load, cache in IndexedDB
   
3. **Blake3 compatibility**: Requires server changes
   - Mitigation: Keep AES as fallback, feature flag
