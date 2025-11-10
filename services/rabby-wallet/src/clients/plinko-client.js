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
  }

  /**
   * Get current block number (last synced)
   */
  getCurrentBlock() {
    return this.currentBlock;
  }

  /**
   * Discover latest delta block number
   * @returns {Promise<number>} - Latest block with delta file
   */
  async getLatestDeltaBlock() {
    try {
      // Fetch delta directory listing
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
    const filename = `delta-${blockNumber.toString().padStart(6, '0')}.bin`;
    const url = `${this.cdnUrl}/deltas/${filename}`;

    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Failed to download delta ${filename}: ${response.status}`);
    }

    const data = await response.arrayBuffer();
    return new Uint8Array(data);
  }

  /**
   * Parse delta file format
   *
   * Format:
   * [0:8]   Delta count (uint64)
   * [8:16]  Reserved (uint64)
   * Then for each delta (24 bytes):
   *   [0:8]   HintSetID (uint64)
   *   [8:16]  IsBackupSet (uint64)
   *   [16:24] Delta value (uint64)
   */
  parseDelta(deltaData) {
    const view = new DataView(deltaData.buffer);
    const count = Number(view.getBigUint64(0, true));

    const deltas = [];
    let offset = 16; // Skip header

    for (let i = 0; i < count; i++) {
      deltas.push({
        hintSetID: Number(view.getBigUint64(offset, true)),
        isBackupSet: view.getBigUint64(offset + 8, true) !== 0n,
        delta: view.getBigUint64(offset + 16, true)
      });
      offset += 24;
    }

    return deltas;
  }

  /**
   * Sync deltas from startBlock to endBlock
   * @param {number} startBlock - First block to sync
   * @param {number} endBlock - Last block to sync
   * @param {PianoPIRClient} pirClient - PIR client to apply deltas to
   * @returns {Promise<number>} - Number of deltas applied
   */
  async syncDeltas(startBlock, endBlock, pirClient) {
    let totalDeltas = 0;

    for (let block = startBlock; block <= endBlock; block++) {
      try {
        // Download delta
        console.log(`📥 Downloading delta-${block.toString().padStart(6, '0')}.bin...`);
        const deltaData = await this.downloadDelta(block);

        // Parse delta
        const deltas = this.parseDelta(deltaData);

        // Apply each delta to hint
        for (const delta of deltas) {
          this.applyDeltaToHint(delta, pirClient);
          totalDeltas++;
        }

        // Log successful application
        if (deltas.length > 0) {
          console.log(`✅ Block ${block}: Applied ${deltas.length} delta(s)`);
        } else {
          console.log(`⏭️  Block ${block}: Empty (0 deltas)`);
        }

        // Update current block
        this.currentBlock = block;

        // Save progress to localStorage
        localStorage.setItem('plinko_current_block', String(block));

      } catch (err) {
        console.error(`❌ Failed to sync delta for block ${block}:`, err);
        // Continue with next block (non-fatal)
      }
    }

    return totalDeltas;
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
   * @param {PianoPIRClient} pirClient - PIR client with hint
   */
  applyDeltaToHint(delta, pirClient) {
    if (!pirClient.hint) {
      throw new Error('Hint not available');
    }

    // Calculate offset in hint for this hint set
    // Offset = header (32 bytes) + hintSetID * chunkSize * 8 bytes
    const metadata = pirClient.metadata;
    const offset = 32 + (delta.hintSetID * metadata.chunkSize * 8);

    // Convert delta value to bytes
    const deltaBytes = new Uint8Array(8);
    const view = new DataView(deltaBytes.buffer);
    view.setBigUint64(0, delta.delta, true);

    // Apply XOR delta
    pirClient.applyDelta(deltaBytes, offset);
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
