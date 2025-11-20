# Plinko PIR Server

A high-performance, privacy-preserving server implementation of the **Plinko** Single-Server PIR protocol. This service allows clients to query a database (e.g., Ethereum account balances) without revealing *which* item they are querying to the server.

## Features

-   **Information-Theoretic Privacy**: The server sees only pseudorandom queries and cannot determine the target index.
-   **Plinko Protocol**: Implements the full Plinko system with **Invertible PRFs (iPRF)** for optimal efficiency.
    -   **O(1) Hint Search**: Clients can find relevant hints in constant time.
    -   **O(1) Updates**: Clients can update their local state efficiently when the database changes.
-   **High Performance**: In-memory database for low-latency query processing.

## Architecture

The system consists of three main components:

1.  **Server (`cmd/server`)**:
    -   Stores the database (flat array of 256-bit (32-byte) entries).
    -   Responds to PIR queries by computing parities over pseudorandom sets.
    -   Stateless and horizontally scalable.

2.  **Client Library (`pkg/client`)**:
    -   **Offline Phase**: Streams the database to generate compact "hints".
    -   **Online Phase**: Generates privacy-preserving queries using iPRF inversion.
    -   **Hint Management**: Manages primary and backup hints to support multiple queries.
    -   **Updates**: Applies database diffs to local hints in O(1) time.

3.  **Core Primitives (`pkg/iprf`)**:
    -   **PMNS**: Pseudorandom Multinomial Sampler.
    -   **PRP**: Small-Domain Pseudorandom Permutation.
    -   **iPRF**: Composition of PMNS and PRP providing efficient forward `F(x)` and inverse `F^-1(y)` evaluation.

## API Endpoints

### `POST /query/fullset`
Performs a standard PIR query using a PRF key.
-   **Input**: `{"prf_key": "hex_encoded_16_bytes"}`
-   **Output**: `{"value": string (decimal representation of 256-bit parity)}`
-   **Description**: The server expands the PRF key to a set of indices and computes their XOR parity.

### `POST /query/setparity`
Performs a PIR query using an explicit set of indices.
-   **Input**: `{"indices": [id1, id2, ...]}`
-   **Output**: `{"parity": string (decimal representation of 256-bit parity)}`
-   **Description**: Used by the client for "punctured set" queries where specific indices need to be included/excluded.

### `GET /health`
Returns service health and configuration.

## Usage

### Prerequisites
-   Go 1.21+
-   A database file (flat binary of `uint64` entries)

### Running the Server

```bash
# Set environment variables
export PLINKO_PIR_DATABASE_PATH="./data/database.bin"
export PLINKO_PIR_SERVER_PORT="3000"

# Run
go run main.go
```

### Client Example (Go)

```go
import "plinko-pir-server/pkg/client"

// Initialize
c := client.NewClient(dbSize, numHints, keyAlpha, keyBeta)
c.HintInit(dbStream)

// Query
req, hint, err := c.Query(targetIndex)
// Send req to server...
// Receive response...
value := c.Reconstruct(response, hint)
```
