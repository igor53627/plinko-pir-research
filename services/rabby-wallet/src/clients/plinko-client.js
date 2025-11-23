/**
 * Plinko Client
 *
 * Handles:
 * - Delta discovery and download
 * - XOR delta application to local hints
 * - Block synchronization tracking
 */

export class PlinkoClient {
  constructor(cdnUrl) {
    this.cdnUrl = cdnUrl;
    this.currentBlock = 0;
    this.manifest = null;
  }

  /**
   * Get current block number (last synced)
   */
  getCurrentBlock() {
    return this.currentBlock;
  }

  async fetchManifest() {
    try {
      const response = await fetch(`${this.cdnUrl}/deltas/manifest.json?t=${Date.now()}`);
      if (response.ok) {
        this.manifest = await response.json();
        return this.manifest;
      }
    } catch (err) {
      console.warn('Failed to fetch delta manifest:', err);
    }
    return null;
  }

  /**
   * Discover latest delta block number
   * @returns {Promise<number>} - Latest block with delta file
   */
  async getLatestDeltaBlock() {
    // Try manifest first
    const manifest = await this.fetchManifest();
    if (manifest && manifest.latestBlock) {
      return manifest.latestBlock;
    }

    try {
      // Fetch delta directory listing (fallback)
      const response = await fetch(`${this.cdnUrl}/deltas/`);
      const html = await response.text();

      // Parse HTML to find delta files
      const deltaRegex = /delta-(\d{6})\.bin/g;
      const matches = [...html.matchAll(deltaRegex)];

      if (matches.length === 0) {
        return 0;
      }

      // Find highest block number
      const blockNumbers = matches.map(m => parseInt(m[1], 10));
      return Math.max(...blockNumbers);
    } catch (err) {
      console.error('Failed to get latest delta block:', err);
      return this.currentBlock;
    }
  }

  /**
   * Download delta file for specific block
   * @param {number} blockNumber - Block number
   * @returns {Promise<Uint8Array>} - Delta data
   */
  async downloadDelta(blockNumber) {
    // Check manifest for CID
    let url;
    if (this.manifest && this.manifest.deltas) {
        const deltaInfo = this.manifest.deltas.find(d => d.block === blockNumber);
        if (deltaInfo && deltaInfo.cid) {
            url = `${this.cdnUrl}/ipfs/${deltaInfo.cid}`;
            // console.log(`🌐 Using IPFS for delta ${blockNumber}: ${deltaInfo.cid}`);
        }
    }

    if (!url) {
        const filename = `delta-${blockNumber.toString().padStart(6, '0')}.bin`;
        url = `${this.cdnUrl}/deltas/${filename}`;
    }

    const CACHE_NAME = 'plinko-deltas-v1';

    // Check if Cache API is supported (requires HTTPS or localhost)
    if (typeof caches !== 'undefined') {
      try {
        const cache = await caches.open(CACHE_NAME);
        const cachedResponse = await cache.match(url);

        if (cachedResponse) {
          console.log(`📦 Served ${filename} from cache`);
          const data = await cachedResponse.arrayBuffer();
          return new Uint8Array(data);
        }

        const response = await fetch(url);
        if (!response.ok) {
          throw new Error(`Failed to download delta ${filename}: ${response.status}`);
        }

        // Cache the successful response
        try {
            cache.put(url, response.clone());
        } catch (e) {
            console.warn('Failed to cache delta:', e);
        }

        const data = await response.arrayBuffer();
        return new Uint8Array(data);
      } catch (err) {
        console.warn('Cache operation failed, falling back to network:', err);
      }
    }

    // Fallback (No Cache API or Cache Error)
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Failed to download delta ${filename}: ${response.status}`);
    }
    const data = await response.arrayBuffer();
    return new Uint8Array(data);
  }

  /**
   * Parse delta file(s)
   * Supports single delta file or concatenated bundle
   */
  parseDeltas(buffer) {
    const allDeltas = [];
    let offset = 0;
    const view = new DataView(buffer.buffer, buffer.byteOffset, buffer.byteLength);

    while (offset < buffer.byteLength) {
        if (buffer.byteLength - offset < 16) break;
        
        const count = Number(view.getBigUint64(offset, true));
        const size = 16 + count * 48;
        
        if (offset + size > buffer.byteLength) {
            console.warn("Truncated delta bundle");
            break;
        }
        
        const fileData = new Uint8Array(buffer.buffer, buffer.byteOffset + offset, size);
        const deltas = this._parseSingleDeltaFile(fileData);
        allDeltas.push(...deltas);
        
        offset += size;
    }
    return allDeltas;
  }

  _parseSingleDeltaFile(deltaData) {
    if (!deltaData || !deltaData.buffer) {
      console.warn('Invalid delta data');
      return [];
    }

    // Use byteOffset and byteLength to handle subarrays correctly
    const view = new DataView(deltaData.buffer, deltaData.byteOffset, deltaData.byteLength);

    if (view.byteLength < 16) {
      console.warn(`Delta file too short (no header). Length: ${view.byteLength}`);
      return [];
    }

    const count = Number(view.getBigUint64(0, true));
    const expectedSize = 16 + count * 48;

    if (view.byteLength < expectedSize) {
      console.warn(`Delta file truncated. Expected ${expectedSize} bytes, got ${view.byteLength} bytes. Count: ${count}`);
      return [];
    }

    const deltas = [];
    let offset = 16; // Skip header

    for (let i = 0; i < count; i++) {
      // Read 32-byte delta (4 * uint64)
      const deltaVal = new Uint8Array(32);
      for (let j = 0; j < 32; j++) {
        deltaVal[j] = view.getUint8(offset + 16 + j);
      }

      deltas.push({
        hintSetID: Number(view.getBigUint64(offset, true)),
        isBackupSet: view.getBigUint64(offset + 8, true) !== 0n,
        delta: deltaVal
      });
      offset += 48;
    }

    return deltas;
  }

  /**
   * Sync deltas from startBlock to endBlock
   * @param {number} startBlock - First block to sync
   * @param {number} endBlock - Last block to sync
   * @param {PlinkoPIRClient} pirClient - PIR client to apply deltas to
   * @returns {Promise<number>} - Number of deltas applied
   */
  async syncDeltas(startBlock, endBlock, pirClient) {
    let totalDeltas = 0;
    let current = startBlock;

    while (current <= endBlock) {
        // Check for bundle
        const bundle = this.findBundle(current, endBlock);
        
        if (bundle) {
             try {
                console.log(`📦 Downloading bundle for blocks ${bundle.startBlock}-${bundle.endBlock}...`);
                const data = await this.downloadBundle(bundle);
                const deltas = this.parseDeltas(data);
                
                for (const delta of deltas) {
                    this.applyDeltaToHint(delta, pirClient);
                    totalDeltas++;
                }
                
                console.log(`✅ Bundle applied (${deltas.length} deltas)`);
                current = bundle.endBlock + 1;
                this.currentBlock = bundle.endBlock;
                localStorage.setItem('plinko_current_block', String(bundle.endBlock));
                continue;
             } catch (err) {
                 console.warn(`Bundle download failed, falling back to individual deltas: ${err.message}`);
                 // Fallback to individual loop
             }
        }

        // Individual delta
        try {
            // Download delta
            // console.log(`📥 Downloading delta-${current.toString().padStart(6, '0')}.bin...`);
            const deltaData = await this.downloadDelta(current);

            // Parse delta
            const deltas = this.parseDeltas(deltaData);

            // Apply each delta to hint
            for (const delta of deltas) {
              this.applyDeltaToHint(delta, pirClient);
              totalDeltas++;
            }
            
            // Only log every 10 blocks to reduce noise during catchup
            if (current % 10 === 0) {
                 console.log(`✅ Synced up to block ${current}`);
            }

            // Update current block
            this.currentBlock = current;

            // Save progress to localStorage
            localStorage.setItem('plinko_current_block', String(current));

      } catch (err) {
        // console.error(`❌ Failed to sync delta for block ${current}:`, err);
        // Continue with next block (non-fatal)
      }
      current++;
    }

    return totalDeltas;
  }

  findBundle(start, end) {
      if (!this.manifest || !this.manifest.bundles) return null;
      // Find a bundle that starts at 'start' and fits within 'end'
      // Prefer largest bundle?
      return this.manifest.bundles.find(b => b.startBlock === start && b.endBlock <= end);
  }

  async downloadBundle(bundle) {
      // Prefer IPFS if available?
      let url;
      if (bundle.cid) {
          // Use CDN proxy for IPFS
          url = `${this.cdnUrl}/ipfs/${bundle.cid}`;
      } else {
          // Construct direct URL
          // Bundle filename convention: bundle-{start}-{end}.bin
          const filename = `bundle-${String(bundle.startBlock).padStart(6,'0')}-${String(bundle.endBlock).padStart(6,'0')}.bin`;
          url = `${this.cdnUrl}/deltas/${filename}`;
      }
      
      const response = await fetch(url);
      if (!response.ok) throw new Error(`Failed to download bundle: ${response.status}`);
      return new Uint8Array(await response.arrayBuffer());
  }


  /**
   * Apply single delta to hint using XOR
   *
   * Algorithm:
   * 1. Find hint set location for hintSetID
   * 2. XOR delta value at that location
   * 3. Hint is now updated for changed database entry
   *
   * @param {Object} delta - Delta object {hintSetID, isBackupSet, delta}
   * @param {PlinkoPIRClient} pirClient - PIR client with hint
   */
  applyDeltaToHint(delta, pirClient) {
    if (!pirClient.hints) {
      throw new Error('Hints not available');
    }

    // Delegate to PlinkoPIRClient which knows the internal hint structure
    pirClient.applyHintDelta(delta.hintSetID, delta.delta);
  }

  /**
   * Load current block from localStorage
   */
  loadProgress() {
    const saved = localStorage.getItem('plinko_current_block');
    if (saved) {
      this.currentBlock = parseInt(saved, 10);
    }
  }

  /**
   * Clear sync progress (for testing)
   */
  clearProgress() {
    this.currentBlock = 0;
    localStorage.removeItem('plinko_current_block');
  }
}
