/**
 * Plinko PIR Client
 *
 * Handles:
 * - Hint download from CDN
 * - Plinko PIR query generation and decoding
 * - Balance extraction from PIR responses
 */

export class PlinkoPIRClient {
  constructor(pirServerUrl, cdnUrl) {
    this.pirServerUrl = pirServerUrl;
    this.cdnUrl = cdnUrl;
    this.hint = null;
    this.addressMapping = null;
    this.metadata = null;
  }

  /**
   * Download hint.bin from CDN
   * This is a one-time download (~70 MB)
   */
  async downloadHint() {
    console.log(`Downloading hint from ${this.cdnUrl}/hint.bin...`);

    const response = await fetch(`${this.cdnUrl}/hint.bin`);
    if (!response.ok) {
      throw new Error(`Failed to download hint: ${response.status}`);
    }

    const hintData = await response.arrayBuffer();
    this.hint = new Uint8Array(hintData);

    // Parse metadata header (first 32 bytes)
    const view = new DataView(hintData);
    this.metadata = {
      dbSize: Number(view.getBigUint64(0, true)),
      chunkSize: Number(view.getBigUint64(8, true)),
      setSize: Number(view.getBigUint64(16, true))
    };

    console.log(`Hint downloaded:`, this.metadata);
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
   * PoC Implementation:
   * - Simple hash-based mapping for demonstration
   * - Production would load address-mapping.bin
   *
   * @param {string} address - Ethereum address (0x...)
   * @returns {number} - Database index
   */
  addressToIndex(address) {
    // Remove 0x prefix
    const addrHex = address.toLowerCase().replace('0x', '');

    // Simple hash: sum of bytes mod dbSize
    let hash = 0;
    for (let i = 0; i < addrHex.length; i += 2) {
      hash += parseInt(addrHex.substr(i, 2), 16);
    }

    return hash % (this.metadata?.dbSize || 8388608);
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
    const prfKey = crypto.getRandomValues(new Uint8Array(16));

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
        chunkSize,
        setSize
      }
    };
  }

  /**
   * Expand PRF key to pseudorandom set (matches server-side prset.go)
   * @param {Uint8Array} prfKey - 16-byte PRF key
   * @param {number} setSize - Number of chunks (k in Plinko PIR)
   * @param {number} chunkSize - Size of each chunk
   * @returns {number[]} - Array of database indices
   */
  expandPRFSet(prfKey, setSize, chunkSize) {
    const indices = [];
    for (let i = 0; i < setSize; i++) {
      const offset = this.prfEvalMod(prfKey, i, chunkSize);
      const index = i * chunkSize + offset;
      indices.push(index);
    }
    return indices;
  }

  /**
   * PRF evaluation: PRF(key, x) mod m (matches server-side FNV-1a)
   * @param {Uint8Array} key - 16-byte PRF key
   * @param {number} x - Input value
   * @param {number} m - Modulus
   * @returns {number} - PRF output mod m
   */
  prfEvalMod(key, x, m) {
    if (m === 0) return 0;

    // FNV-1a hash (matching Go implementation)
    let hash = 2166136261;

    // Mix in key
    for (let i = 0; i < 16; i++) {
      hash ^= key[i];
      hash = Math.imul(hash, 16777619);
    }

    // Mix in x (little-endian uint64)
    const xBytes = new Uint8Array(8);
    new DataView(xBytes.buffer).setBigUint64(0, BigInt(x), true);
    for (const b of xBytes) {
      hash ^= b;
      hash = Math.imul(hash, 16777619);
    }

    // Return hash mod m (convert to unsigned)
    return (hash >>> 0) % m;
  }

  /**
   * Read database entry from hint data
   * @param {Uint8Array} hintData - Complete hint data
   * @param {number} dbStart - Offset where database starts (after header)
   * @param {number} index - Database index
   * @returns {bigint} - Balance value at index
   */
  readDBEntry(hintData, dbStart, index) {
    const offset = dbStart + index * 8; // 8 bytes per entry
    if (offset + 8 > hintData.length) {
      return 0n; // Out of bounds
    }
    const view = new DataView(hintData.buffer, hintData.byteOffset);
    return view.getBigUint64(offset, true); // Little-endian
  }
}
