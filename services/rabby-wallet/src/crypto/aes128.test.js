import { describe, it, expect } from 'vitest';
import { Aes128 } from './aes128.js';
import prfVectors from '../testdata/prf_vectors.json';
import { PlinkoPIRClient } from '../clients/plinko-pir-client.js';

const hexToUint8Array = (hex) => {
  if (hex.length % 2 !== 0) {
    throw new Error('Hex string must have even length');
  }
  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
};

const hexToBigInt = (hex) => BigInt(`0x${hex}`);

const formatHex = (value) => value.toString(16).padStart(16, '0');

describe('AES-128 PRF vectors', () => {
  it('matches known ciphertext outputs', () => {
    const keyBytes = hexToUint8Array(prfVectors.key_hex);
    const aes = new Aes128(keyBytes);

    const block = new Uint8Array(16);
    const encrypted = new Uint8Array(16);
    const blockView = new DataView(block.buffer);
    const encryptedView = new DataView(encrypted.buffer);

    for (const vector of prfVectors.indices) {
      block.fill(0);
      blockView.setBigUint64(8, BigInt(vector.index));
      aes.encryptBlock(block, encrypted);
      const raw = encryptedView.getBigUint64(0, false);
      expect(formatHex(raw)).toBe(vector.raw_hex);
    }
  });
});

describe('PlinkoPIRClient PRF helpers', () => {
  it('produces offsets and indices consistent with AES vectors', () => {
    const keyBytes = hexToUint8Array(prfVectors.key_hex);
    const aes = new Aes128(keyBytes);
    const client = new PlinkoPIRClient('http://localhost:3000', 'http://localhost:3000');
    const chunkSize = 8192;
    const chunkSizeBig = BigInt(chunkSize);
    const scratch = client.getPrfScratch();

    prfVectors.indices.forEach((vector) => {
      const expectedRaw = hexToBigInt(vector.raw_hex);
      const expectedOffset = Number(expectedRaw % chunkSizeBig);
      const offset = client.prfEvalMod(aes, vector.index, chunkSize, scratch);
      expect(offset).toBe(expectedOffset);
    });

    const setSize = 32;
    const indices = client.expandPRFSet(keyBytes, setSize, chunkSize);
    expect(indices).toHaveLength(setSize);

    for (let i = 0; i < setSize; i++) {
      const expectedOffset = client.prfEvalMod(aes, i, chunkSize, scratch);
      const expectedIndex = i * chunkSize + expectedOffset;
      expect(indices[i]).toBe(expectedIndex);
    }
  });
});
