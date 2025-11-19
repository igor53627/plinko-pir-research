# Plinko PIR Wallet Integration

This directory contains the reference implementation of a privacy-preserving wallet client using Plinko PIR. It demonstrates how to query Ethereum balances without revealing the target address to the RPC provider.

## 🔒 Privacy Architecture

The wallet uses **Plinko PIR (Private Information Retrieval)** to achieve information-theoretic privacy. This means the server learns *absolutely nothing* about which address you are querying, even if it has infinite computational power.

### How it Works

1.  **Offline Setup (One-time)**:
    *   The client downloads a compressed snapshot of the Ethereum state (`database.bin`) and an address mapping (`address-mapping.bin`).
    *   It locally derives a compact "hint" (~70MB) from this data.
    *   This hint allows the client to decode server responses.

2.  **Online Query (Privacy-Preserving)**:
    *   To query an address, the client generates a random **PRF key**.
    *   This key expands into a pseudorandom set of database indices $S$.
    *   The client sends this key to the server.
    *   The server computes the XOR sum (parity) of all balances at indices in $S$.
    *   **Crucially**, the server sees only a random key and cannot determine which specific index in $S$ is the target.

3.  **Decoding Process**:
    *   The client receives the server's parity $P_{server}$.
    *   Using its local hint, the client computes the parity of the *same* set $S$ based on its local (possibly stale) state: $P_{hint}$.
    *   The client XORs these parities: $\Delta = P_{server} \oplus P_{hint}$.
    *   This $\Delta$ represents the *change* in the target balance since the snapshot was taken (plus noise from other changes, which are handled by the specific Plinko construction).
    *   The client reconstructs the current balance: $Balance = Hint[target] \oplus \Delta$.

4.  **Incremental Updates**:
    *   To keep the local hint fresh without re-downloading the database, the client downloads small "delta files" published by the Plinko Update Service.
    *   These updates are applied to the local hint in $O(1)$ time.

## 🛠️ Components

*   **`clients/plinko-pir-client.js`**: Core logic for hint management, query generation, and response decoding.
*   **`clients/plinko-client.js`**: Handles the download and application of incremental updates (deltas).
*   **`providers/PlinkoPIRProvider.jsx`**: React context managing the global state (privacy mode, sync status).
*   **`components/PrivacyMode.jsx`**: UI component for toggling privacy and visualizing status.

## 🚀 Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev
```

## 📚 References

*   **Plinko: Single-Server PIR with Efficient Updates** (ePrint 2024/318)
*   **Piano: Extremely Simple, Single-Server PIR** (ePrint 2023/452)
