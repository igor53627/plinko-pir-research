import { sha256 } from '@noble/hashes/sha256';
import { PlinkoClientState } from '../crypto/plinko-hints.js';
import { DATASET_STATS } from '../constants/dataset.js';

const browserCrypto = typeof globalThis !== 'undefined' && globalThis.crypto
  ? globalThis.crypto
  : null;

// Plinko security parameters
const LAMBDA_HINT_SECURITY = 8;  // λ - tune for memory vs security (8 for dev, 64-128 for prod)
const MAX_QUERIES_BEFORE_REFRESH = 256;  // q - queries before hint refresh needed

// Cache format version - increment when PlinkoClientState serialization changes
const HINTS_FORMAT_VERSION = 3;

// Timestamped logging
const log = (msg) => {
  const now = new Date();
  const ts = now.toISOString().slice(11, 23);
  console.log(`[${ts}] ${msg}`);
};

export class PlinkoPIRClient {
  constructor(pirServerUrl, cdnUrl) {
    this.pirServerUrl = pirServerUrl;
    this.cdnUrl = cdnUrl;
    this.clientState = null;  // PlinkoClientState instance
    this.metadata = null;
    this.snapshotVersion = null;
    this.masterKey = null;
    this.addressMapping = null;
  }

  async tryRestoreFromCache() {
    try {
      log(`🔍 Checking for cached hints...`);

      const manifest = await this.fetchSnapshotManifest();
      this.snapshotManifest = manifest;
      this.snapshotVersion = manifest.version;

      this.metadata = {
        dbSize: Number(manifest.db_size),
        chunkSize: Number(manifest.chunk_size),
        setSize: Number(manifest.set_size)
      };

      this.initializeMasterKey();

      const databaseFile = this.findDatabaseFile(manifest);
      if (!databaseFile) return false;

      const expectedHash = databaseFile.sha256 || databaseFile.SHA256;
      const masterKeyHash = this.bufferToHex(sha256(this.masterKey)).slice(0, 16);
      const hintsCacheKey = `hints-v${HINTS_FORMAT_VERSION}-${expectedHash?.slice(0, 16)}-${masterKeyHash}`;

      const cachedHints = await this.loadFromCache('plinko-hints', hintsCacheKey);
      if (!cachedHints) {
        log(`❌ No cached hints found`);
        return false;
      }

      this.clientState = PlinkoClientState.fromBytes(cachedHints, this.masterKey);
      log(`✅ Restored hints from cache (${(cachedHints.byteLength / 1024 / 1024).toFixed(1)} MB)`);

      const cacheKey = `address-mapping-${this.snapshotVersion}`;
      const cachedMapping = await this.loadFromCache('plinko-data-v1', cacheKey);
      if (!cachedMapping) {
        await this.downloadAddressMapping(() => {});
      } else {
        this.parseAddressMapping(cachedMapping);
        log(`✅ Restored address mapping from cache`);
      }

      return true;
    } catch (e) {
      log(`⚠️ Cache restore failed: ${e.message}`);
      return false;
    }
  }

  parseAddressMapping(mappingBytes) {
    const view = new DataView(mappingBytes.buffer);
    this.addressMapping = new Map();
    const entrySize = 24;
    const entryCount = Math.floor(mappingBytes.byteLength / entrySize);

    for (let i = 0; i < entryCount; i++) {
      const offset = i * entrySize;
      const addrBytes = mappingBytes.slice(offset, offset + 20);
      const addrHex = '0x' + this.bufferToHex(addrBytes);
      const index = view.getUint32(offset + 20, true);
      this.addressMapping.set(addrHex.toLowerCase(), index);
    }
    log(`📍 Parsed ${this.addressMapping.size} address mappings`);
  }

  async downloadHint(onProgress) {
    log(`📥 Fetching snapshot manifest...`);
    const manifest = await this.fetchSnapshotManifest();
    this.snapshotManifest = manifest;
    this.snapshotVersion = manifest.version;

    this.metadata = {
      dbSize: Number(manifest.db_size),
      chunkSize: Number(manifest.chunk_size),
      setSize: Number(manifest.set_size)
    };
    
    this.initializeMasterKey();

    log(`📦 Snapshot version ${this.snapshotVersion} (db_size=${this.metadata.dbSize}, chunk=${this.metadata.chunkSize}, set=${this.metadata.setSize})`);

    const databaseFile = this.findDatabaseFile(manifest);
    if (!databaseFile) {
      throw new Error('Snapshot manifest missing database.bin entry');
    }

    const expectedHash = databaseFile.sha256 || databaseFile.SHA256;
    
    const cachedSnapshot = await this.loadFromCache('snapshot-db', expectedHash);
    let snapshotBytes;
    
    if (cachedSnapshot) {
      log(`📦 Loaded snapshot from cache (hash: ${expectedHash?.slice(0, 8)}...)`);
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
      await this.saveToCache('snapshot-db', expectedHash, snapshotBytes);
    }

    const masterKeyHash = this.bufferToHex(sha256(this.masterKey)).slice(0, 16);
    const hintsCacheKey = `hints-v${HINTS_FORMAT_VERSION}-${expectedHash?.slice(0, 16)}-${masterKeyHash}`;
    log(`🔑 Hints cache key: ${hintsCacheKey}`);
    const cachedHints = await this.loadFromCache('plinko-hints', hintsCacheKey);
    
    if (cachedHints) {
      log(`📦 Loaded hints from cache (${(cachedHints.byteLength / 1024 / 1024).toFixed(1)} MB)`);
      this.clientState = PlinkoClientState.fromBytes(cachedHints, this.masterKey);
      if (onProgress) onProgress('hint_generation', 100);
    } else {
      log(`⚙️ Generating Plinko Hints (Reference-Aligned Mode)...`);
      if (onProgress) onProgress('hint_generation', 0);
      
      await this.generateHints(snapshotBytes, onProgress);
      
      if (onProgress) onProgress('hint_generation', 100);
      const serialized = this.clientState.toBytes();
      log(`✅ Hints generated. Storage: ${(serialized.byteLength / 1024 / 1024).toFixed(1)} MB`);
      
      await this.saveToCache('plinko-hints', hintsCacheKey, serialized);
    }

    await this.downloadAddressMapping((percent) => onProgress && onProgress('address_mapping', percent));
  }

  initializeMasterKey() {
    const MASTER_KEY_STORAGE = 'plinko-master-key';
    let masterKey;
    
    try {
      const stored = localStorage.getItem(MASTER_KEY_STORAGE);
      if (stored) {
        masterKey = new Uint8Array(JSON.parse(stored));
        log(`🔑 Loaded master key from storage`);
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
      try {
        localStorage.setItem(MASTER_KEY_STORAGE, JSON.stringify(Array.from(masterKey)));
        log(`🔑 Generated and saved new master key`);
      } catch (e) {
        console.warn('Failed to persist master key:', e);
      }
    }
    this.masterKey = masterKey;
  }

  async generateHints(snapshotBytes, onProgress) {
    const { dbSize, chunkSize } = this.metadata;
    
    // Create PlinkoClientState with reference-aligned parameters
    this.clientState = new PlinkoClientState(
      dbSize,
      chunkSize,
      LAMBDA_HINT_SECURITY,
      MAX_QUERIES_BEFORE_REFRESH,
      this.masterKey
    );
    
    // Initialize hint structures (creates block subsets)
    this.clientState.initializeHints();
    
    log(`📊 Hint params: λ=${LAMBDA_HINT_SECURITY}, w=${chunkSize}, q=${MAX_QUERIES_BEFORE_REFRESH}`);
    log(`📊 Regular hints: ${this.clientState.numRegularHints}, Backup hints: ${this.clientState.numBackupHints}`);
    
    // Stream database and process entries
    const view = new DataView(snapshotBytes.buffer, snapshotBytes.byteOffset, snapshotBytes.byteLength);
    const entrySize = 32;
    const totalEntries = Math.floor(snapshotBytes.byteLength / entrySize);
    
    let lastLogTime = Date.now();
    const startTime = Date.now();
    
    for (let i = 0; i < Math.min(totalEntries, dbSize); i++) {
      // Read 256-bit value as BigInt
      const offset = i * entrySize;
      let value = 0n;
      for (let w = 0; w < 4; w++) {
        value |= view.getBigUint64(offset + w * 8, true) << BigInt(w * 64);
      }
      
      // Process entry through PlinkoClientState
      this.clientState.processEntry(i, value);
      
      // Progress reporting
      const now = Date.now();
      if (now - lastLogTime > 1000) {
        const pct = ((i / totalEntries) * 100).toFixed(1);
        const elapsed = (now - startTime) / 1000;
        const rate = (i / elapsed / 1000).toFixed(1);
        log(`⚙️ Hint generation: ${pct}% (${i.toLocaleString()}/${totalEntries.toLocaleString()} entries, ${rate}k/s)`);
        if (onProgress) onProgress('hint_generation', Number(pct));
        lastLogTime = now;
      }
    }
    
    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
    log(`✅ Processed ${totalEntries.toLocaleString()} entries in ${elapsed}s`);
  }

  async queryBalancePrivate(address) {
    if (!this.clientState) {
      throw new Error('Hints not initialized');
    }

    const targetIndex = this.addressToIndex(address);
    const { chunkSize, setSize } = this.metadata;
    
    const alpha = Math.floor(targetIndex / chunkSize);
    const beta = targetIndex % chunkSize;

    // Check client-side cache first
    const cached = this.clientState.getCached(targetIndex);
    if (cached !== null) {
      log(`📦 Cache hit for index ${targetIndex}`);
      return {
        balance: cached & ((1n << 64n) - 1n),
        saturated: false,
        visualization: { cached: true, targetIndex }
      };
    }

    // Get a hint for this entry
    const hintInfo = this.clientState.getHint(alpha, beta);
    if (!hintInfo) {
      throw new Error("No available hint for this entry - refresh hints needed");
    }

    const { hintIdx, blocks, parity, offsets, isPromoted } = hintInfo;

    // Build query: P' = blocks \ {alpha}, offsets for all blocks
    const queryP = [];
    for (const blockIdx of blocks) {
      if (blockIdx !== alpha) {
        queryP.push(blockIdx);
      }
    }

    // Send query to server
    const url = `${this.pirServerUrl}/query/plinko`;
    const body = {
      p: queryP,
      offsets: offsets
    };
    
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    
    const data = await response.json();
    const r0 = BigInt(data.r0);
    const r1 = BigInt(data.r1);
    
    // Reconstruct value: D[target] = parity XOR r0
    // parity = XOR of all entries in blocks at iPRF offsets
    // r0 = XOR of all entries in queryP (blocks \ {alpha}) at iPRF offsets
    // So: parity XOR r0 = D[target]
    const rawBalance = parity ^ r0;
    
    // Consume hint and promote backup
    this.clientState.consumeHint(hintIdx, targetIndex, rawBalance);

    // Balance is stored as 256-bit, but actual ETH balance fits in 64-bit for most accounts
    const balance = rawBalance & ((1n << 64n) - 1n);
    const saturated = rawBalance !== balance;

    if (saturated) {
      console.warn(`⚠️ Balance overflow detected: raw=${rawBalance}, masked=${balance}`);
    }

    return {
      balance: balance,
      saturated: saturated,
      visualization: {
        hintIdx: hintIdx,
        r0: r0.toString(),
        r1: r1.toString(),
        hintVal: parity.toString(),
        isPromoted: isPromoted,
        chunkSize: chunkSize,
        setSize: setSize,
        prfSetSize: blocks.size,
        targetIndex: targetIndex,
        targetChunk: alpha,
        serverParity: r0.toString(),
        hintParity: parity.toString(),
        delta: '0',
        hintValue: balance.toString(),
        dbSize: chunkSize * setSize,
        saturated: saturated,
        stats: this.clientState.getStats()
      }
    };
  }
  
  getHintSize() {
    return this.clientState ? this.clientState.toBytes().byteLength : 0;
  }

  applyAccountDelta(accountIndex, delta) {
    if (!this.clientState) return;
    
    // Convert delta bytes to BigInt
    let deltaValue = 0n;
    const view = new DataView(delta.buffer, delta.byteOffset, 32);
    for (let i = 0; i < 4; i++) {
      deltaValue |= view.getBigUint64(i * 8, true) << BigInt(i * 64);
    }
    
    // Use PlinkoClientState's updateHint method
    this.clientState.updateHint(accountIndex, deltaValue);
  }

  // Helper methods
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
    const snapshotPath = `snapshots/${this.snapshotVersion}/${fileEntry.path}`;
    candidates.push(`${this.cdnUrl}/${snapshotPath}`);
    if (fileEntry?.ipfs?.gateway_url) candidates.push(fileEntry.ipfs.gateway_url);
    if (fileEntry?.ipfs?.cid) candidates.push(`${this.cdnUrl}/ipfs/${fileEntry.ipfs.cid}`);
    return [...new Set(candidates.filter(Boolean))];
  }

  async downloadFromCandidates(urls, label, fallbackSize, onProgress) {
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

    if (cacheKey && hasCacheApi) {
      try {
        const cache = await caches.open(CACHE_NAME);
        const cachedResponse = await cache.match(cacheKey);
        if (cachedResponse) {
          log(`📦 Served ${label} from cache`);
          if (onProgress) onProgress(100);
          const buffer = await cachedResponse.arrayBuffer();
          return new Uint8Array(buffer);
        }
      } catch (err) {
        console.warn('Cache check failed:', err);
      }
    }

    log(`📥 Downloading ${label} from ${url}...`);
    const response = await fetch(url, {
      cache: 'no-store',
      headers: { 'Cache-Control': 'no-cache, no-store, must-revalidate', 'Pragma': 'no-cache' }
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
          log(`📶 ${label}: ${percent}% (${receivedMB}/${totalMB} MB) - ${speed} MB/s`);
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

    if (cacheKey && hasCacheApi) {
      try {
        const cache = await caches.open(CACHE_NAME);
        await cache.put(cacheKey, new Response(chunksAll));
        log(`💾 Cached ${label}`);
      } catch (err) {
        console.warn('Failed to write to cache:', err);
      }
    }

    log(`✅ Downloaded ${label} (${(receivedLength / 1024 / 1024).toFixed(1)} MB)`);
    return chunksAll;
  }

  async verifySnapshotHash(bytes, expectedHex) {
    if (!expectedHex) return;
    let hashBytes;
    const subtle = browserCrypto?.subtle;
    if (subtle && typeof subtle.digest === 'function') {
      const hashBuffer = await subtle.digest('SHA-256', bytes);
      hashBytes = new Uint8Array(hashBuffer);
    } else {
      hashBytes = sha256(bytes);
    }
    const actualHex = this.bufferToHex(hashBytes);
    if (actualHex.toLowerCase() !== expectedHex.toLowerCase()) {
      throw new Error(`Snapshot hash mismatch. Expected ${expectedHex}, got ${actualHex}`);
    }
    log(`✅ Snapshot hash verified (${expectedHex.slice(0, 8)}...)`);
  }

  bufferToHex(bytes) {
    return Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
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
      await cache.put(key, new Response(data));
      log(`💾 Cached ${key} (${(data.byteLength / 1024 / 1024).toFixed(1)} MB)`);
    } catch (e) {
      console.warn(`Cache save failed for ${key}:`, e);
    }
  }

  async downloadAddressMapping(onProgress) {
    const timestamp = Date.now();
    const url = `${this.cdnUrl}/address-mapping.bin?v=${timestamp}`;
    const mappingEntries = this.metadata?.dbSize || DATASET_STATS.addressCount;
    const mappingBytes = mappingEntries * 24;
    const mappingMB = Number((mappingBytes / 1024 / 1024).toFixed(1));
    const mappingLabel = `address-mapping.bin (~${mappingMB} MB)`;
    const cacheKey = `address-mapping-${this.snapshotVersion}`;

    const chunksAll = await this.downloadBinary(url, mappingLabel, mappingBytes, onProgress, cacheKey);
    this.parseAddressMapping(chunksAll);
  }

  addressToIndex(address) {
    const idx = this.addressMapping?.get(address.toLowerCase());
    if (idx === undefined) throw new Error("Address not found");
    return idx;
  }
}
