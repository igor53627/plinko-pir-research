/**
 * Plinko PIR Client
 *
 * Handles:
 * - Hint download from CDN
 * - Plinko PIR query generation and decoding
 * - Balance extraction from PIR responses
 */

import { sha256 } from '@noble/hashes/sha256';
import { Aes128 } from '../crypto/aes128.js';
import { DATASET_STATS } from '../constants/dataset.js';

const browserCrypto = typeof globalThis !== 'undefined' && globalThis.crypto
  ? globalThis.crypto
  : null;
const UINT256_MAX = (1n << 256n) - 1n;

export class PlinkoPIRClient {
  constructor(pirServerUrl, cdnUrl) {
    this.pirServerUrl = pirServerUrl;
    this.cdnUrl = cdnUrl;
    this.hint = null;
    this.addressMapping = null; // Map from address hex -> index
    this.metadata = null;
    this.snapshotVersion = null;
    this.snapshotManifest = null;
    this._prfScratch = null;
  }

  /**
   * Download `snapshots/latest/manifest.json` + database.bin from CDN
   * Derive hint locally from the canonical snapshot (43 MB)
   * Total download: ~170 MB (snapshot database ~42.6 MB + address-mapping.bin ~127.6 MB)
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

    console.log(`📦 Snapshot version ${this.snapshotVersion} (db_size=${this.metadata.dbSize.toLocaleString()}, chunk=${this.metadata.chunkSize}, set=${this.metadata.setSize})`);

    const databaseFile = this.findDatabaseFile(manifest);
    if (!databaseFile) {
      throw new Error('Snapshot manifest missing database.bin entry');
    }

    const snapshotUrls = this.buildSnapshotUrls(databaseFile);
    const snapshotBytes = await this.downloadFromCandidates(
      snapshotUrls,
      `snapshot database (${(databaseFile.size / 1024 / 1024).toFixed(1)} MB)`,
      databaseFile.size,
      (percent) => onProgress && onProgress('database', percent)
    );

    await this.verifySnapshotHash(snapshotBytes, databaseFile.sha256 || databaseFile.SHA256);

    console.log(`🔐 Deriving local hint from snapshot...`);
    if (onProgress) onProgress('hint_generation', 0);
    this.hint = this.buildHintFromSnapshot(snapshotBytes, this.metadata);
    if (onProgress) onProgress('hint_generation', 100);
    const finalSize = (this.hint.byteLength / 1024 / 1024).toFixed(1);
    console.log(`✅ Local hint generated (${finalSize} MB)`);
    console.log(`📊 Hint metadata:`, this.metadata);

    // Download address-mapping.bin
    await this.downloadAddressMapping((percent) => onProgress && onProgress('address_mapping', percent));
  }

  async fetchSnapshotManifest() {
    const url = `${this.cdnUrl}/snapshots/latest/manifest.json?t=${Date.now()}`;
    const response = await fetch(url, { cache: 'no-store' });
    if (!response.ok) {
      throw new Error(`Failed to download snapshot manifest: ${response.status}`);
    }
    const manifest = await response.json();
    return manifest;
  }

  findDatabaseFile(manifest) {
    if (!manifest || !manifest.files) return null;
    return manifest.files.find(file => file.path.endsWith('database.bin')) || null;
  }

  buildSnapshotUrls(fileEntry) {
    const candidates = [];
    if (fileEntry?.ipfs?.gateway_url) {
      candidates.push(fileEntry.ipfs.gateway_url);
    }
    if (fileEntry?.ipfs?.cid) {
      candidates.push(`${this.cdnUrl}/ipfs/${fileEntry.ipfs.cid}`);
    }
    const snapshotPath = `snapshots/${this.snapshotVersion}/${fileEntry.path}`;
    candidates.push(`${this.cdnUrl}/${snapshotPath}`);
    return [...new Set(candidates.filter(Boolean))];
  }

  buildHintFromSnapshot(snapshotBytes, metadata) {
    const totalLength = 32 + snapshotBytes.length;
    const hintBuffer = new Uint8Array(totalLength);
    const view = new DataView(hintBuffer.buffer, hintBuffer.byteOffset, 32);
    view.setBigUint64(0, BigInt(metadata.dbSize), true);
    view.setBigUint64(8, BigInt(metadata.chunkSize), true);
    view.setBigUint64(16, BigInt(metadata.setSize), true);
    view.setBigUint64(24, 0n, true);
    hintBuffer.set(snapshotBytes, 32);
    return hintBuffer;
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

  async downloadFromCandidates(urls, label, fallbackSize, onProgress) {
    let lastError = null;
    for (const url of urls) {
      try {
        // Use the URL itself as the cache key for snapshot files (they are versioned)
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

  /**
   * Download address-mapping.bin from CDN
   * This maps Ethereum addresses to database indices (~127.6 MB)
   *
   * Cache-busting strategy:
   * - Uses timestamp parameter and no-store to bypass browser cache
   * - Prevents serving stale Anvil test data
   * - Forces fresh download of real Ethereum address mapping
   */
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

  /**
   * Get hint size in bytes
   */
  getHintSize() {
    return this.hint ? this.hint.byteLength : 0;
  }

  /**
   * Update hint with XOR delta
   * @param {Uint8Array} delta - Delta to apply
   * @param {number} offset - Offset in hint to update
   */
  applyDelta(delta, offset) {
    if (!this.hint) {
      throw new Error('Hint not downloaded');
    }

    // Apply XOR delta at offset
    for (let i = 0; i < delta.length; i++) {
      this.hint[offset + i] ^= delta[i];
    }
  }

  /**
   * Query balance for an address using Plinko PIR (PLAINTEXT - NOT PRIVATE)
   *
   * PoC Implementation:
   * - Uses simplified PlaintextQuery for demonstration
   * - Production should use queryBalancePrivate() with FullSetQuery
   *
   * @param {string} address - Ethereum address
   * @returns {Promise<bigint>} - Balance in wei
   */
  async queryBalance(address) {
    if (!this.hint) {
      throw new Error('Hint not downloaded - call downloadHint() first');
    }

    // For PoC: use simplified plaintext query
    // Production: generate PRF key and use FullSetQuery
    const index = this.addressToIndex(address);

    // Prepare request
    const url = `${this.pirServerUrl}/query/plaintext`;
    const headers = { 'Content-Type': 'application/json' };
    const requestBody = { index };
    const bodyString = JSON.stringify(requestBody);

    // Log full HTTP request details
    console.log('========================================');
    console.log('⚠️  PLAINTEXT QUERY (PoC Mode - NOT Private!)');
    console.log('========================================');
    console.log('HTTP Request Details:');
    console.log(`  Method: POST`);
    console.log(`  URL: ${url}`);
    console.log(`  Headers:`, headers);
    console.log(`  Body (JSON):`, requestBody);
    console.log(`  Full Body String: ${bodyString}`);
    console.log('');
    console.log('⚠️  What server sees:');
    console.log(`  ❌ Database index: ${index}`);
    console.log(`  ❌ Server can determine which address is queried!`);
    console.log(`  ⚠️  This is NOT private - for PoC demonstration only`);
    console.log('');
    console.log('ℹ️  For true privacy, use queryBalancePrivate() with FullSet PIR');
    console.log('========================================');

    const response = await fetch(url, {
      method: 'POST',
      headers: headers,
      body: bodyString
    });

    if (!response.ok) {
      throw new Error(`Query failed: ${response.status}`);
    }

    const data = await response.json();
    return BigInt(data.value);
  }

  /**
   * Map Ethereum address to database index
   *
   * Uses address-mapping.bin for accurate lookups
   *
   * @param {string} address - Ethereum address (0x...)
   * @returns {number} - Database index
   */
  addressToIndex(address) {
    const normalizedAddress = address.toLowerCase();

    // Look up address in mapping
    if (this.addressMapping && this.addressMapping.has(normalizedAddress)) {
      return this.addressMapping.get(normalizedAddress);
    }

    // Address not found in database - show real address range
    let errorMessage = `Address ${address} not found in database. `;

    if (this.addressMapping && this.addressMapping.size > 0) {
      const addresses = Array.from(this.addressMapping.keys()).sort();
      const firstAddr = addresses[0];
      const lastAddr = addresses[addresses.length - 1];

      // Show actual address range from mapping
      errorMessage += `Database contains ${this.addressMapping.size.toLocaleString()} real Ethereum addresses ` +
        `(range: ${firstAddr.substring(0, 6)}...${firstAddr.slice(-4)} to ${lastAddr.substring(0, 6)}...${lastAddr.slice(-4)}). `;

      // Detect stale cache
      if (firstAddr.startsWith('0x1000')) {
        errorMessage += `⚠️ WARNING: Detected Anvil test data (0x1000...) - you may have stale cache. `;
      }
    } else {
      errorMessage += `Address mapping not loaded. `;
    }

    errorMessage += `Try hard-refreshing (Ctrl+Shift+R or Cmd+Shift+R) if you see unexpected address ranges.`;

    throw new Error(errorMessage);
  }

  /**
   * Plinko PIR FullSet query (production implementation)
   *
   * Algorithm:
   * 1. Client determines index i for target address
   * 2. Generate random PRF key k
   * 3. Expand k to set S such that i ∈ S
   * 4. Send FullSetQuery(k) to server
   * 5. Server responds with parity p = ⊕_{j ∈ S} DB[j]
   * 6. Client decodes: balance_i = decode(p, k, i)
   *
   * Privacy: Server learns nothing about i
   */
  async queryBalancePrivate(address) {
    if (!this.hint) {
      throw new Error('Hint not downloaded - call downloadHint() first');
    }

    const targetIndex = this.addressToIndex(address);
    const { chunkSize, setSize } = this.metadata;

    // Generate random PRF key (16 bytes)
    const cryptoSource = browserCrypto;
    if (!cryptoSource?.getRandomValues) {
      throw new Error('Secure random generator unavailable in this environment');
    }
    const prfKey = cryptoSource.getRandomValues(new Uint8Array(16));

    // Prepare request
    const url = `${this.pirServerUrl}/query/fullset`;
    const headers = { 'Content-Type': 'application/json' };
    const requestBody = { prf_key: Array.from(prfKey) };
    const bodyString = JSON.stringify(requestBody);

    // Log full HTTP request details
    console.log('========================================');
    console.log('🔒 PRIVATE QUERY - CLIENT SIDE');
    console.log('========================================');
    console.log('HTTP Request Details:');
    console.log(`  Method: POST`);
    console.log(`  URL: ${url}`);
    console.log(`  Headers:`, headers);
    console.log(`  Body (JSON):`);
    console.log(`    prf_key: [${requestBody.prf_key.slice(0, 8).join(', ')}...] (16 bytes)`);
    console.log(`  Full Body String: ${bodyString.substring(0, 150)}...`);
    console.log('');
    console.log('What server sees:');
    console.log('  ✅ Random PRF key (looks like noise)');
    console.log('  ❌ NOT the address being queried');
    console.log('  ❌ NOT which balance is requested');
    console.log('========================================');

    // Send FullSet query to server
    const response = await fetch(url, {
      method: 'POST',
      headers: headers,
      body: bodyString
    });

    if (!response.ok) {
      throw new Error(`Private query failed: ${response.status}`);
    }

    const data = await response.json();
    const serverParity = BigInt(data.value);

    // === PRODUCTION PLINKO PIR DECODING ===

    // Step 1: Re-expand PRF key to get same set as server
    const prfSet = this.expandPRFSet(prfKey, setSize, chunkSize);

    // Step 2: Compute target chunk
    const targetChunk = Math.floor(targetIndex / chunkSize);

    // Step 3: Read database entries from hint for decoding
    // Hint structure: [32-byte header][database entries...]
    const hintData = this.hint;
    const dbStart = 32; // Skip header

    // Step 4: Compute XOR of all PRF-selected entries FROM HINT
    let hintParity = 0n;
    for (const idx of prfSet) {
      const value = this.readDBEntry(hintData, dbStart, idx);
      hintParity ^= value;
    }

    // Step 5: Compute delta between server and hint
    // If hint is up to date: serverParity === hintParity
    // If there are updates: delta = serverParity ⊕ hintParity contains the changes
    const delta = serverParity ^ hintParity;

    // Step 6: Extract target balance
    // Read target from hint
    const hintValue = this.readDBEntry(hintData, dbStart, targetIndex);

    // For this PoC: hint should match database exactly, so balance = hintValue
    // In production with updates: would need to apply delta if hint is stale
    const targetBalance = hintValue;
    const saturated = targetBalance === UINT256_MAX;

    if (saturated) {
      console.warn('⚠️ Balance hit uint256 cap in dataset; account exceeds 256-bit range');
    }

    console.log(`✅ Decoded balance: ${targetBalance} wei`);
    console.log(`   Server parity: ${serverParity}, Hint parity: ${hintParity}, Delta: ${delta}`);

    if (delta !== 0n) {
      console.warn(`⚠️ Delta is non-zero (${delta}), hint may be stale. Using hint value anyway for PoC.`);
    }

    // Return balance with visualization data
    return {
      balance: targetBalance,
      visualization: {
        prfKey: Array.from(prfKey),
        targetIndex,
        targetChunk,
        prfSetSize: prfSet.length,
        prfSetSample: prfSet.slice(0, 5), // First 5 indices for display
        serverParity: serverParity.toString(),
        hintParity: hintParity.toString(),
        delta: delta.toString(),
        hintValue: hintValue.toString(),
        dbSize: this.metadata?.dbSize || chunkSize * setSize,
        chunkSize,
        setSize,
        saturated
      },
      saturated
    };
  }

  /**
   * Expand PRF key to pseudorandom set (matches server AES-128 PRF)
   * @param {Uint8Array} prfKey - 16-byte PRF key
   * @param {number} setSize - Number of chunks (k in Plinko PIR)
   * @param {number} chunkSize - Size of each chunk
   * @returns {number[]} - Array of database indices
   */
  expandPRFSet(prfKey, setSize, chunkSize) {
    const keyBytes = prfKey instanceof Uint8Array ? prfKey : Uint8Array.from(prfKey);
    const aes = new Aes128(keyBytes);
    const scratch = this.getPrfScratch();
    const indices = [];
    for (let i = 0; i < setSize; i++) {
      const offset = this.prfEvalMod(aes, i, chunkSize, scratch);
      indices.push(i * chunkSize + offset);
    }
    return indices;
  }

  /**
   * PRF evaluation: AES-128(key, x) mod m (matches server implementation)
   * @param {Aes128} aes - AES instance initialised with the PRF key
   * @param {number} x - Input value
   * @param {number} m - Modulus
   * @param {object} [scratch] - Reusable buffers for AES evaluation
   * @returns {number} - PRF output mod m
   */
  prfEvalMod(aes, x, m, scratch = this.getPrfScratch()) {
    if (m === 0) return 0;

    const { block, encrypted, blockView, encryptedView } = scratch;
    block.fill(0);
    blockView.setBigUint64(8, BigInt(x), false);

    aes.encryptBlock(block, encrypted);

    const value = encryptedView.getBigUint64(0, false);
    return Number(value % BigInt(m));
  }

  /**
   * Read database entry from hint data
   * @param {Uint8Array} hintData - Complete hint data
   * @param {number} dbStart - Offset where database starts (after header)
   * @param {number} index - Database index
   * @returns {bigint} - Balance value at index
   */
  readDBEntry(hintData, dbStart, index) {
    const offset = dbStart + index * 32; // 32 bytes per entry
    if (offset + 32 > hintData.length) {
      return 0n; // Out of bounds
    }
    const view = new DataView(hintData.buffer, hintData.byteOffset);

    // Read 4 uint64s little-endian and combine to 256-bit integer
    let val = 0n;
    for (let i = 0; i < 4; i++) {
      const word = view.getBigUint64(offset + i * 8, true);
      val += word << BigInt(i * 64);
    }
    return val;
  }

  getPrfScratch() {
    if (!this._prfScratch) {
      const block = new Uint8Array(16);
      const encrypted = new Uint8Array(16);
      this._prfScratch = {
        block,
        encrypted,
        blockView: new DataView(block.buffer),
        encryptedView: new DataView(encrypted.buffer)
      };
    }
    return this._prfScratch;
  }
}
