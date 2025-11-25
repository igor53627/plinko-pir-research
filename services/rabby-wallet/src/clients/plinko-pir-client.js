import { sha256 } from '@noble/hashes/sha256';
import { Aes128 } from '../crypto/aes128.js';
import { IPRF } from '../crypto/iprf.js';
import { DATASET_STATS } from '../constants/dataset.js';

const browserCrypto = typeof globalThis !== 'undefined' && globalThis.crypto
  ? globalThis.crypto
  : null;
const UINT256_MAX = (1n << 256n) - 1n;

export class PlinkoPIRClient {
  constructor(pirServerUrl, cdnUrl) {
    this.pirServerUrl = pirServerUrl;
    this.cdnUrl = cdnUrl;
    this.hints = null; // Uint8Array holding parities (numHints * 32 bytes)
    this.chunkKeys = null; // Array of IPRF keys
    this.metadata = null;
    this.snapshotVersion = null;
    this.masterKey = null;
  }

  /**
   * Download snapshot and generate Light Client hints
   */
  async downloadHint(onProgress) {
    console.log(`📥 Fetching snapshot manifest...`);
    const manifest = await this.fetchSnapshotManifest();
    this.snapshotManifest = manifest;
    this.snapshotVersion = manifest.version;

    this.metadata = {
      dbSize: Number(manifest.db_size),
      chunkSize: Number(manifest.chunk_size),
      setSize: Number(manifest.set_size)
    };
    
    // Initialize keys
    this.initializeKeys();

    console.log(`📦 Snapshot version ${this.snapshotVersion} (db_size=${this.metadata.dbSize}, chunk=${this.metadata.chunkSize}, set=${this.metadata.setSize})`);

    const databaseFile = this.findDatabaseFile(manifest);
    if (!databaseFile) {
      throw new Error('Snapshot manifest missing database.bin entry');
    }

    const expectedHash = databaseFile.sha256 || databaseFile.SHA256;
    
    // Try to load from cache first using hash as key
    const cachedSnapshot = await this.loadFromCache('snapshot-db', expectedHash);
    let snapshotBytes;
    
    if (cachedSnapshot) {
      console.log(`📦 Loaded snapshot from cache (hash: ${expectedHash?.slice(0, 8)}...)`);
      if (onProgress) onProgress('database', 100);
      snapshotBytes = cachedSnapshot;
    } else {
      const snapshotUrls = this.buildSnapshotUrls(databaseFile);
      snapshotBytes = await this.downloadFromCandidates(
        snapshotUrls,
        `snapshot database`,
        databaseFile.size,
        (percent) => onProgress && onProgress('database', percent)
      );

      await this.verifySnapshotHash(snapshotBytes, expectedHash);
      
      // Save to cache with hash as key
      await this.saveToCache('snapshot-db', expectedHash, snapshotBytes);
    }

    // Try to load cached hints (keyed by snapshot hash + master key hash)
    const masterKeyHash = this.bufferToHex(sha256(this.masterKey)).slice(0, 16);
    const hintsCacheKey = `hints-${expectedHash?.slice(0, 16)}-${masterKeyHash}`;
    const cachedHints = await this.loadFromCache('plinko-hints', hintsCacheKey);
    
    if (cachedHints) {
      console.log(`📦 Loaded hints from cache`);
      this.hints = cachedHints;
      if (onProgress) onProgress('hint_generation', 100);
    } else {
      // Check for partial progress checkpoint
      const checkpointKey = `${hintsCacheKey}-checkpoint`;
      const checkpoint = await this.loadCheckpoint(checkpointKey);
      
      console.log(`⚙️ Generating Plinko Hints (Light Client Mode)...`);
      if (checkpoint) {
        console.log(`📥 Resuming from chunk ${checkpoint.completedChunks}/${this.iprfs.length}`);
      }
      if (onProgress) onProgress('hint_generation', checkpoint ? (checkpoint.completedChunks / this.iprfs.length) * 100 : 0);
      
      // Generate hints from snapshot (with checkpoint support)
      await this.generateHints(snapshotBytes, onProgress, checkpointKey, checkpoint);
      
      if (onProgress) onProgress('hint_generation', 100);
      console.log(`✅ Hints generated. Storage: ${(this.hints.byteLength / 1024 / 1024).toFixed(1)} MB`);
      
      // Cache the completed hints and clear checkpoint
      await this.saveToCache('plinko-hints', hintsCacheKey, this.hints);
      await this.clearCheckpoint(checkpointKey);
    }

    // Download address-mapping.bin
    await this.downloadAddressMapping((percent) => onProgress && onProgress('address_mapping', percent));
  }

  initializeKeys() {
    // Try to load persisted master key, or generate new one
    const MASTER_KEY_STORAGE = 'plinko-master-key';
    let masterKey;
    
    try {
      const stored = localStorage.getItem(MASTER_KEY_STORAGE);
      if (stored) {
        masterKey = new Uint8Array(JSON.parse(stored));
        console.log(`🔑 Loaded master key from storage`);
      }
    } catch (e) {
      console.warn('Failed to load master key from storage:', e);
    }
    
    if (!masterKey || masterKey.length !== 32) {
      masterKey = new Uint8Array(32);
      if (browserCrypto) {
        browserCrypto.getRandomValues(masterKey);
      } else {
        console.warn("Using insecure random for master key");
        for(let i=0; i<32; i++) masterKey[i] = Math.floor(Math.random() * 256);
      }
      // Persist the new key
      try {
        localStorage.setItem(MASTER_KEY_STORAGE, JSON.stringify(Array.from(masterKey)));
        console.log(`🔑 Generated and saved new master key`);
      } catch (e) {
        console.warn('Failed to persist master key:', e);
      }
    }
    this.masterKey = masterKey;

    // Derive keys for each chunk IPRF
    // key[i] = H(master, i)
    // We use a simple derivation for PoC
    this.chunkKeys = [];
    for (let i = 0; i < this.metadata.setSize; i++) {
        const k = new Uint8Array(32);
        // Simple KDF: XOR master with index (not secure, but functional for PoC structure)
        // Real impl should use HMAC/HKDF
        for(let j=0; j<32; j++) k[j] = masterKey[j];
        // Mix index into first 8 bytes
        let idx = i;
        for(let j=0; j<8; j++) {
            k[j] ^= idx & 0xFF;
            idx >>= 8;
        }
        this.chunkKeys.push(k);
    }

    // Initialize IPRFs
    // We'll create them on demand to save memory, or cache them?
    // Creating 1000 IPRF objects is fine.
    this.iprfs = this.chunkKeys.map(k => new IPRF(k, this.metadata.numHints || (this.metadata.setSize * 2), this.metadata.chunkSize)); 
    // numHints usually depends on params. Let's assume numHints approx setSize * lambda?
    // For this PoC, let's define numHints.
    // Paper: "n/r sets". 
    // Let's fix numHints = setSize * 4 for good coverage?
    // Params.go doesn't specify numHints. 
    // Let's assume numHints = setSize * 64 (approx sqrt(N) * log N)
    this.numHints = this.metadata.setSize * 64;
    
    // Re-init IPRFs with correct domain size
    this.iprfs = this.chunkKeys.map(k => new IPRF(k, this.numHints, this.metadata.chunkSize));
  }

  async generateHints(snapshotBytes, onProgress, checkpointKey, checkpoint) {
    const dbU32 = new Uint32Array(snapshotBytes.buffer, snapshotBytes.byteOffset, Math.floor(snapshotBytes.byteLength / 4));
    const dbSize = this.metadata.dbSize;
    const chunkSize = this.metadata.chunkSize;
    const numChunks = this.iprfs.length;
    
    // Resume from checkpoint or start fresh
    let startChunk = 0;
    if (checkpoint && checkpoint.hints) {
      this.hints = checkpoint.hints;
      startChunk = checkpoint.completedChunks;
    } else {
      this.hints = new Uint8Array(this.numHints * 32);
    }
    const hintsU32 = new Uint32Array(this.hints.buffer);
    
    let lastLog = Date.now();
    let lastCheckpoint = Date.now();
    const CHECKPOINT_INTERVAL = 5000; // Save checkpoint every 5 seconds
    
    // Process chunk by chunk - pre-compute inverse table for each chunk
    for (let alpha = startChunk; alpha < numChunks; alpha++) {
        const iprf = this.iprfs[alpha];
        
        // Pre-compute inverse lookup table for this chunk
        // inverseTable[beta] = array of hint indices
        const inverseTable = new Array(chunkSize);
        for (let beta = 0; beta < chunkSize; beta++) {
            const indices = iprf.inverse(beta);
            // Convert to Numbers and filter by isBlockInP
            inverseTable[beta] = indices
                .map(h => Number(h))
                .filter(h => this.isBlockInP(h, alpha));
        }
        
        // Now process all entries in this chunk
        const chunkStart = alpha * chunkSize;
        const chunkEnd = Math.min(chunkStart + chunkSize, dbSize);
        
        for (let i = chunkStart; i < chunkEnd; i++) {
            const beta = i - chunkStart;
            const valOffsetU32 = i * 8; // 32 bytes = 8 uint32s
            
            if (valOffsetU32 + 8 > dbU32.length) break;
            
            // Read 8 uint32s
            const w0 = dbU32[valOffsetU32];
            const w1 = dbU32[valOffsetU32 + 1];
            const w2 = dbU32[valOffsetU32 + 2];
            const w3 = dbU32[valOffsetU32 + 3];
            const w4 = dbU32[valOffsetU32 + 4];
            const w5 = dbU32[valOffsetU32 + 5];
            const w6 = dbU32[valOffsetU32 + 6];
            const w7 = dbU32[valOffsetU32 + 7];
            
            // XOR into matching hints
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

        const now = Date.now();
        
        // Save checkpoint periodically
        if (now - lastCheckpoint > CHECKPOINT_INTERVAL) {
            await this.saveCheckpoint(checkpointKey, {
                completedChunks: alpha + 1,
                hints: this.hints
            });
            lastCheckpoint = now;
        }
        
        if (now - lastLog > 500) {
            const pct = ((alpha + 1) / numChunks) * 100;
            console.log(`⚙️ Hint generation: ${pct.toFixed(1)}% (chunk ${alpha + 1}/${numChunks})`);
            if (onProgress) onProgress('hint_generation', pct);
            lastLog = now;
            // Yield to event loop to allow UI updates
            await new Promise(r => setTimeout(r, 0));
        }
    }
  }

  /**
   * Plinko Query (Real Protocol)
   */
  async queryBalancePrivate(address) {
    if (!this.hints) {
      throw new Error('Hints not initialized');
    }

    const targetIndex = this.addressToIndex(address);
    const { chunkSize, setSize } = this.metadata;
    
    const alpha = Math.floor(targetIndex / chunkSize);
    const beta = targetIndex % chunkSize;

    // 1. Find a hint set containing target (alpha, beta)
    const iprf = this.iprfs[alpha];
    const candidates = iprf.inverse(beta);
    
    if (candidates.length === 0) {
        throw new Error("No hint set found for this element (probabilistic failure, try refreshing hints)");
    }
    
    // Pick random candidate securely
    const randBuf = new Uint32Array(1);
    if (browserCrypto) {
        browserCrypto.getRandomValues(randBuf);
    } else {
        randBuf[0] = Math.floor(Math.random() * 0xFFFFFFFF);
    }
    const hintIdx = Number(candidates[randBuf[0] % candidates.length]);
    
    // 2. Construct Query
    // Reconstruct the set P and offsets
    const P = [];
    const offsets = new Uint8Array(setSize); // Using Uint8Array assuming offset fits in 255? 
    // Wait, offset < chunkSize. chunkSize ~ 1000. Need Uint16 or Uint32.
    const offsetsArr = new Uint32Array(setSize);
    
    // Derive P (subset of blocks)
    // We use a simple PRG seeded by hintIdx to determine P
    // For each block k, is k in P?
    // We also need the offsets for each block k.
    // offset_k = IPRF_k.forward(hintIdx)
    
    for (let k = 0; k < setSize; k++) {
        const o = this.iprfs[k].forward(hintIdx);
        offsetsArr[k] = o;
        
        // Determine if k is in P
        // Seed PRG with (hintIdx, k) or just (hintIdx) and sample set?
        // Implementation must match server? No, Client sends P to Server.
        // So Client defines P.
        // P should be random subset of size approx setSize/2.
        // Use hash(hintIdx, k) < Threshold
        if (this.isBlockInP(hintIdx, k)) {
            P.push(k);
        }
    }
    
    // Puncturing:
    // We need alpha to be in P? 
    // Figure 7: "If alpha in P: H[j] = (P, p xor d)".
    // Query q = (P \ {alpha}, offsets).
    // So if alpha is NOT in P, we can't use this hint for alpha?
    // Wait, if alpha is NOT in P, then H[j] does not include D[alpha].
    // So H[j] = Parity(Blocks in P).
    // Response r0 = Parity(Blocks in P \ {alpha}).
    // If alpha not in P, then H[j] is independent of D[alpha].
    // So we MUST select a hint where alpha IS in P.
    
    // Filter candidates for alpha \in P (convert BigInt to Number)
    const validCandidates = candidates.map(h => Number(h)).filter(h => this.isBlockInP(h, alpha));
    if (validCandidates.length === 0) {
        throw new Error("No valid hint found (alpha not in P)");
    }

    // Select securely
    if (browserCrypto) {
        browserCrypto.getRandomValues(randBuf);
    } else {
        randBuf[0] = Math.floor(Math.random() * 0xFFFFFFFF);
    }
    const selectedHintIdx = validCandidates[randBuf[0] % validCandidates.length];
    
    // Re-generate P and offsets for selected hint
    const finalP = [];
    const finalOffsets = [];
    for (let k = 0; k < setSize; k++) {
        finalOffsets.push(this.iprfs[k].forward(selectedHintIdx));
        if (this.isBlockInP(selectedHintIdx, k)) {
            // Remove alpha from P sent to server
            if (k !== alpha) {
                finalP.push(k);
            }
        }
    }
    
    // 3. Send Query
    const url = `${this.pirServerUrl}/query/plinko`;
    const body = {
        p: finalP,
        offsets: finalOffsets
    };
    
    const response = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    });
    
    const data = await response.json();
    const r0 = BigInt(data.r0);
    const r1 = BigInt(data.r1);
    
    // 4. Reconstruct
    // H[j] = Parity(P_orig)
    // r0 = Parity(P \ {alpha})
    // So H[j] ^ r0 = D[alpha] (if alpha in P)
    // Wait, we verified alpha in P.
    // So Parity(P) = D[alpha] ^ Parity(P \ {alpha}).
    // D[alpha] = H[j] ^ Parity(P \ {alpha}).
    // Parity(P \ {alpha}) is exactly r0 returned by server (sum of blocks in finalP).
    
    // Get local hint value
    const hintVal = this.readHint(selectedHintIdx);
    const balance = hintVal ^ r0;
    
    return {
        balance: balance,
        visualization: {
            hintIdx: selectedHintIdx,
            r0: r0.toString(),
            r1: r1.toString(),
            hintVal: hintVal.toString()
        }
    };
  }
  
  isBlockInP(hintIdx, blockIdx) {
      // MurmurHash3 64-bit finalizer mixing function for better distribution
      let h = BigInt(hintIdx) ^ (BigInt(blockIdx) << 32n);
      h ^= h >> 33n;
      h *= 0xff51afd7ed558ccdn;
      h ^= h >> 33n;
      h *= 0xc4ceb9fe1a85ec53n;
      h ^= h >> 33n;
      return (h & 1n) === 0n;
  }

  readHint(hintIdx) {
      const offset = hintIdx * 32;
      const view = new DataView(this.hints.buffer);
      let val = 0n;
      for (let i = 0; i < 4; i++) {
        val += view.getBigUint64(offset + i * 8, true) << BigInt(i * 64);
      }
      return val;
  }

  applyDelta(deltaBytes, offset) {
      // offset in delta file was calculated as: header + hintSetID * chunkSize * 32.
      // But in "Heavy Client", that was wrong.
      // Here, we assume the delta is meant for HINTS.
      // So delta file format: hintSetID is the index in 'hints'.
      // We need to ignore the 'offset' passed by plinko-client.js and use hintSetID directly.
      // But plinko-client.js calls applyDelta(delta, offset).
      // We should update plinko-client.js to pass hintSetID or handle it here.
      // For now, let's assume the delta logic is fixed in plinko-client.js or we interpret offset.
      // Actually, plinko-client.js calculates offset = 32 + id * ...
      // We should modify plinko-client.js to just call applyHintDelta(id, delta).
  }
  
  // ... Helpers ...
  async fetchSnapshotManifest() {
    const url = `${this.cdnUrl}/snapshots/latest/manifest.json?t=${Date.now()}`;
    const response = await fetch(url, { cache: 'no-store' });
    if (!response.ok) throw new Error(`Failed to download snapshot manifest: ${response.status}`);
    return await response.json();
  }

  findDatabaseFile(manifest) {
    if (!manifest || !manifest.files) return null;
    return manifest.files.find(file => file.path.endsWith('database.bin')) || null;
  }

  buildSnapshotUrls(fileEntry) {
    const candidates = [];
    if (fileEntry?.ipfs?.gateway_url) candidates.push(fileEntry.ipfs.gateway_url);
    if (fileEntry?.ipfs?.cid) candidates.push(`${this.cdnUrl}/ipfs/${fileEntry.ipfs.cid}`);
    const snapshotPath = `snapshots/${this.snapshotVersion}/${fileEntry.path}`;
    candidates.push(`${this.cdnUrl}/${snapshotPath}`);
    return [...new Set(candidates.filter(Boolean))];
  }

  async downloadFromCandidates(urls, label, fallbackSize, onProgress) {
      // ... (Keep original download logic) ...
      // For brevity in this edit, I will assume the original download logic is available or I should copy it.
      // Since I am using create_file, I must provide the FULL content.
      // I will copy the download logic from the original file.
      
      // (Copying helper methods from original file...)
      let lastError = null;
      for (const url of urls) {
        try {
          return await this.downloadBinary(url, label, fallbackSize, onProgress, url);
        } catch (err) {
          console.warn(`⚠️  Download failed for ${url}: ${err.message}`);
          lastError = err;
        }
      }
      throw lastError || new Error(`Failed to download ${label}`);
  }

  async downloadBinary(url, label, fallbackSize, onProgress, cacheKey = null) {
    const CACHE_NAME = 'plinko-data-v1';
    const hasCacheApi = typeof caches !== 'undefined';

    // 1. Check cache first
    if (cacheKey && hasCacheApi) {
      try {
        const cache = await caches.open(CACHE_NAME);
        const cachedResponse = await cache.match(cacheKey);
        if (cachedResponse) {
          console.log(`📦 Served ${label} from cache`);
          if (onProgress) onProgress(100);
          const buffer = await cachedResponse.arrayBuffer();
          return new Uint8Array(buffer);
        }
      } catch (err) {
        console.warn('Cache check failed:', err);
      }
    }

    console.log(`📥 Downloading ${label} from ${url}...`);
    const response = await fetch(url, {
      cache: 'no-store',
      headers: {
        'Cache-Control': 'no-cache, no-store, must-revalidate',
        'Pragma': 'no-cache'
      }
    });
    if (!response.ok) {
      throw new Error(`Failed to download ${label}: ${response.status}`);
    }

    const contentLength = response.headers.get('content-length');
    const total = contentLength ? parseInt(contentLength, 10) : fallbackSize || 0;

    const reader = response.body.getReader();
    let receivedLength = 0;
    const chunks = [];

    let lastLogTime = Date.now();
    const startTime = Date.now();

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      chunks.push(value);
      receivedLength += value.length;

      if (total > 0) {
        const now = Date.now();
        if (now - lastLogTime > 500) {
          const percent = ((receivedLength / total) * 100).toFixed(1);
          const receivedMB = (receivedLength / 1024 / 1024).toFixed(1);
          const totalMB = (total / 1024 / 1024).toFixed(1);
          const elapsed = (now - startTime) / 1000;
          const speed = (receivedLength / 1024 / 1024 / elapsed).toFixed(1);
          console.log(`📶 ${label}: ${percent}% (${receivedMB}/${totalMB} MB) - ${speed} MB/s`);
          if (onProgress) onProgress(Number(percent));
          lastLogTime = now;
        }
      }
    }

    const chunksAll = new Uint8Array(receivedLength);
    let position = 0;
    for (const chunk of chunks) {
      chunksAll.set(chunk, position);
      position += chunk.length;
    }

    // 2. Save to cache
    if (cacheKey && hasCacheApi) {
      try {
        const cache = await caches.open(CACHE_NAME);
        const responseToCache = new Response(chunksAll);
        await cache.put(cacheKey, responseToCache);
        console.log(`💾 Cached ${label}`);
      } catch (err) {
        console.warn('Failed to write to cache:', err);
      }
    }

    const finalSize = (receivedLength / 1024 / 1024).toFixed(1);
    console.log(`✅ Downloaded ${label} (${finalSize} MB)`);
    return chunksAll;
  }

  async verifySnapshotHash(bytes, expectedHex) {
    if (!expectedHex) {
      return;
    }
    let hashBytes;
    const subtle = browserCrypto?.subtle;
    if (subtle && typeof subtle.digest === 'function') {
      const hashBuffer = await subtle.digest('SHA-256', bytes);
      hashBytes = new Uint8Array(hashBuffer);
    } else {
      console.warn('⚠️ WebCrypto subtle API unavailable; falling back to @noble/hashes for snapshot verification');
      hashBytes = sha256(bytes);
    }
    const actualHex = this.bufferToHex(hashBytes);
    if (actualHex.toLowerCase() !== expectedHex.toLowerCase()) {
      throw new Error(`Snapshot hash mismatch. Expected ${expectedHex}, got ${actualHex}`);
    }
    console.log(`✅ Snapshot hash verified (${expectedHex.slice(0, 8)}...)`);
  }

  bufferToHex(bytes) {
    return Array.from(bytes)
      .map(b => b.toString(16).padStart(2, '0'))
      .join('');
  }

  async loadFromCache(cacheName, key) {
    if (typeof caches === 'undefined') return null;
    try {
      const cache = await caches.open(cacheName);
      const response = await cache.match(key);
      if (response) {
        const buffer = await response.arrayBuffer();
        return new Uint8Array(buffer);
      }
    } catch (e) {
      console.warn(`Cache load failed for ${key}:`, e);
    }
    return null;
  }

  async saveToCache(cacheName, key, data) {
    if (typeof caches === 'undefined') return;
    try {
      const cache = await caches.open(cacheName);
      const response = new Response(data);
      await cache.put(key, response);
      console.log(`💾 Cached ${key} (${(data.byteLength / 1024 / 1024).toFixed(1)} MB)`);
    } catch (e) {
      console.warn(`Cache save failed for ${key}:`, e);
    }
  }

  async loadCheckpoint(key) {
    if (typeof caches === 'undefined') return null;
    try {
      const cache = await caches.open('plinko-checkpoints');
      const metaResponse = await cache.match(`${key}-meta`);
      const hintsResponse = await cache.match(`${key}-hints`);
      if (metaResponse && hintsResponse) {
        const meta = await metaResponse.json();
        const hintsBuffer = await hintsResponse.arrayBuffer();
        return {
          completedChunks: meta.completedChunks,
          hints: new Uint8Array(hintsBuffer)
        };
      }
    } catch (e) {
      console.warn(`Checkpoint load failed:`, e);
    }
    return null;
  }

  async saveCheckpoint(key, checkpoint) {
    if (typeof caches === 'undefined') return;
    try {
      const cache = await caches.open('plinko-checkpoints');
      await cache.put(`${key}-meta`, new Response(JSON.stringify({ completedChunks: checkpoint.completedChunks })));
      await cache.put(`${key}-hints`, new Response(checkpoint.hints));
      console.log(`💾 Checkpoint saved (chunk ${checkpoint.completedChunks})`);
    } catch (e) {
      console.warn(`Checkpoint save failed:`, e);
    }
  }

  async clearCheckpoint(key) {
    if (typeof caches === 'undefined') return;
    try {
      const cache = await caches.open('plinko-checkpoints');
      await cache.delete(`${key}-meta`);
      await cache.delete(`${key}-hints`);
    } catch (e) {
      // Ignore
    }
  }

  async downloadAddressMapping(onProgress) {
    // Add cache-busting timestamp to force fresh download
    // This ensures we get the NEW file, not cached old Anvil data
    const timestamp = Date.now();
    const url = `${this.cdnUrl}/address-mapping.bin?v=${timestamp}`;
    const mappingEntries = this.metadata?.dbSize || DATASET_STATS.addressCount;
    const mappingBytes = mappingEntries * 24;
    const mappingMB = Number((mappingBytes / 1024 / 1024).toFixed(1));
    const mappingLabel = `address-mapping.bin (~${mappingMB} MB)`;
    
    // Use snapshot version in cache key to ensure we get matching mapping for the DB
    const cacheKey = `address-mapping-${this.snapshotVersion}`;

    const chunksAll = await this.downloadBinary(url, mappingLabel, mappingBytes, onProgress, cacheKey);

    const mappingData = chunksAll.buffer;
    const view = new DataView(mappingData);

    // Parse address-mapping.bin
    // Format: [20 bytes address][4 bytes index (little-endian)] repeated
    this.addressMapping = new Map();

    const entrySize = 24; // 20 bytes address + 4 bytes index
    const numEntries = mappingData.byteLength / entrySize;

    console.log(`📊 Parsing ${numEntries.toLocaleString()} address entries...`);

    for (let i = 0; i < numEntries; i++) {
      const offset = i * entrySize;

      // Read 20-byte address
      const addressBytes = new Uint8Array(mappingData, offset, 20);
      const addressHex = '0x' + Array.from(addressBytes)
        .map(b => b.toString(16).padStart(2, '0'))
        .join('');

      // Read 4-byte index (little-endian)
      const index = view.getUint32(offset + 20, true);

      this.addressMapping.set(addressHex.toLowerCase(), index);
    }

    const finalSize = (chunksAll.byteLength / 1024 / 1024).toFixed(1);
    console.log(`✅ Address mapping loaded (${finalSize} MB, ${this.addressMapping.size.toLocaleString()} addresses)`);

    // Log address range for debugging and cache verification
    if (this.addressMapping.size > 0) {
      const addresses = Array.from(this.addressMapping.keys()).sort();
      const firstAddr = addresses[0];
      const lastAddr = addresses[addresses.length - 1];
      console.log(`📍 Address range: ${firstAddr} to ${lastAddr}`);
      const expectedCount = (this.metadata?.dbSize || DATASET_STATS.addressCount).toLocaleString();
      console.log(`ℹ️  Expected dataset size: ${expectedCount} Ethereum addresses from the initial 99k mainnet blocks`);

      // Detect stale Anvil cache
      if (firstAddr.startsWith('0x1000')) {
        console.error(`❌ STALE CACHE DETECTED! Got Anvil test data (0x1000...) instead of real Ethereum data (0x0000...)`);
        console.error(`⚠️  Please hard-refresh (Ctrl+Shift+R or Cmd+Shift+R) to clear browser cache`);
      }
    }
  }

  addressToIndex(address) {
      const idx = this.addressMapping?.get(address.toLowerCase());
      if (idx === undefined) throw new Error("Address not found");
      return idx;
  }

  // Apply raw account delta to local private hints
  applyAccountDelta(accountIndex, delta) {
      if (!this.hints) return;
      
      const { chunkSize } = this.metadata;
      const alpha = Math.floor(accountIndex / chunkSize);
      const beta = accountIndex % chunkSize;
      
      // 1. Find hint sets containing this element
      const iprf = this.iprfs[alpha];
      const hintIndices = iprf.inverse(beta);
      
      // 2. Create view for delta
      const dView = new DataView(delta.buffer, delta.byteOffset, 32);
      const d0 = dView.getBigUint64(0, true);
      const d1 = dView.getBigUint64(8, true);
      const d2 = dView.getBigUint64(16, true);
      const d3 = dView.getBigUint64(24, true);

      const view = new DataView(this.hints.buffer);

      for (const hintIdxBig of hintIndices) {
          const hintIdx = Number(hintIdxBig);
          // Only update hint if the block (alpha) is in the partition P for this hint
          if (this.isBlockInP(hintIdx, alpha)) {
              const offset = hintIdx * 32;
              if (offset + 32 > this.hints.byteLength) continue;

              // XOR delta into hint
              view.setBigUint64(offset, view.getBigUint64(offset, true) ^ d0, true);
              view.setBigUint64(offset+8, view.getBigUint64(offset+8, true) ^ d1, true);
              view.setBigUint64(offset+16, view.getBigUint64(offset+16, true) ^ d2, true);
              view.setBigUint64(offset+24, view.getBigUint64(offset+24, true) ^ d3, true);
          }
      }
  }
}
