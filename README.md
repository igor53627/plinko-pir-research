# Plinko PIR Research

**A complete, high-performance implementation of the Plinko Single-Server PIR protocol with efficient updates.**

This project implements a privacy-preserving system for querying Ethereum account balances. It allows clients to retrieve data from a server without revealing *which* address they are interested in, while supporting real-time database updates—a critical improvement over previous PIR schemes.

## 📚 Academic Context

**Plinko** is a state-of-the-art Single-Server Private Information Retrieval (PIR) scheme. Its key innovation is the use of **Invertible Pseudorandom Functions (iPRFs)** to achieve:

1.  **O(1) Hint Search**: Clients can locate the "hint" (pre-processed parity) containing their target index in constant time.
2.  **O(1) Updates**: When the database changes, clients can update their local hints in constant time without re-downloading the entire dataset.
3.  **Information-Theoretic Privacy**: The server sees only pseudorandom queries and cannot distinguish the target index from any other.

This implementation focuses on the "Warm Tier" of Ethereum state (~8.4M active accounts), demonstrating that privacy is practical at scale.

**Dataset**: The current deployment uses a snapshot of **5,575,868 real Ethereum addresses** (non-zero balances) from the Ethereum mainnet.

## 🏗️ Architecture

The system is composed of three main services:

### 1. Plinko PIR Server (`services/plinko-pir-server`)
*   **Role**: The read-path server.
*   **Function**: Stores the database and answers PIR queries.
*   **Key Tech**: Implements the server-side of the Plinko protocol. It expands client PRF keys into pseudorandom sets and computes parities.
*   **Privacy**: Does not know which index the client is querying.

### 2. Plinko Update Service (`services/plinko-update-service`)
*   **Role**: The write-path service.
*   **Function**: Monitors the Ethereum blockchain, detects balance changes, and publishes incremental "delta" files.
*   **Key Tech**: Uses a cached iPRF mapping to generate updates in **~24μs** per block (O(1) per entry).
*   **Output**: Stream of XOR deltas that clients download to stay in sync.

### 3. Client Library (`services/plinko-pir-server/pkg/client`)
*   **Role**: The logic running in the user's wallet/application.
*   **Function**:
    *   **Offline**: Generates compact hints from the database stream.
    *   **Online**: Generates privacy-preserving queries using `iPRF.Inverse`.
    *   **Updates**: Applies deltas to local hints in O(1) time.
    *   **Management**: Handles primary and backup hints to support multiple queries.

## 🚀 Quick Start

### Prerequisites
*   Docker & Docker Compose
*   Go 1.21+ (for local development)

### Running the Full Stack
The easiest way to run the system (Server, Update Service, Mock Ethereum Node) is via Docker Compose:

```bash
# Build and start all services
make start

# View logs
make logs

# Stop services
make stop
```

This will spin up:
*   **Anvil**: A simulated Ethereum blockchain.
*   **Plinko Update Service**: Generating deltas from the blockchain.
*   **Plinko PIR Server**: Ready to answer queries.
*   **CDN Mock**: Serving snapshots and deltas.

### Running Tests
We have comprehensive test suites covering cryptographic primitives, protocol logic, and system integration.

```bash
# Run all tests
make test

# Run specific component tests
cd services/plinko-pir-server
go test ./pkg/iprf/...   # Test iPRF primitives
go test ./pkg/client/... # Test Client logic
```

## 📂 Project Structure

```
.
├── services/
│   ├── plinko-pir-server/      # Main PIR Server & Client Library
│   │   ├── pkg/iprf/           # Core iPRF implementation (PMNS + PRP)
│   │   ├── pkg/client/         # Client-side logic (HintInit, Query, Update)
│   │   └── cmd/server/         # Server entrypoint
│   └── plinko-update-service/  # Update generation service
├── docs/                       # Research documentation
└── scripts/                    # Helper scripts for build/test
```

## 🛠️ Key Implementation Details

### Invertible PRF (iPRF)
Located in `services/plinko-pir-server/pkg/iprf`.
*   **PMNS**: Pseudorandom Multinomial Sampler using a tree-based construction.
*   **PRP**: Small-Domain Pseudorandom Permutation using a generalized Feistel network with cycle-walking.
*   **Composition**: `F(x) = S(P(x))` and `F^-1(y) = P^-1(S^-1(y))`.

### Efficient Updates
Located in `services/plinko-pir-server/pkg/client/update.go`.
*   Clients use `iPRF.Forward(index)` to find the *exact* hint affected by a database change.
*   They XOR the delta into that hint's parity.
*   This avoids the O(√n) cost of scanning all hints or re-downloading data.

## 📄 References

*   **Plinko: Single-Server PIR with Efficient Updates** (ePrint 2024/318)
*   **Piano: Extremely Simple, Single-Server PIR** (ePrint 2023/452)

---
*Part of the Privacy & Scaling Explorations (PSE) research.*
