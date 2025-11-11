# Technical Specification: Cryo-Reth Integration for Plinko PIR Event Logs

**Version:** 1.0
**Date:** 2025-11-11
**Status:** Implementation Ready

---

## Overview

This document specifies modifications to Paradigm's Cryo to extract Ethereum event logs in Parquet format optimized for Plinko PIR's Cuckoo Filter system.

**Goals:**
- Extract last 100,000 blocks (7 days) of Ethereum logs
- Generate Parquet files with 1K blocks per file (8 MB each)
- Support incremental updates (new file every 12 seconds)
- Enable direct reth MDBX database access + RPC fallback

**Requirements:**
- Correctness: 100% match with eth_getLogs reference
- Performance: 100K blocks in <10 minutes (MDBX) or <30 minutes (RPC)
- Incremental: Process new 1K-block file in <30 seconds
- File format: Valid Parquet, ZSTD level 3, correct schema

---

## 1. Technical Architecture

### 1.1 System Components

```
┌─────────────────────────────────────────────────────────────┐
│                   Ethereum Data Source                       │
│                                                              │
│  ┌──────────────────────────┐  ┌───────────────────────┐  │
│  │   Reth MDBX Database     │  │   RPC (eth_getLogs)   │  │
│  │   - Receipts table       │  │   - Batch queries     │  │
│  │   - 10-50× faster        │  │   - Remote access     │  │
│  └────────────┬─────────────┘  └──────────┬────────────┘  │
└───────────────┼────────────────────────────┼───────────────┘
                │                            │
                └──────────┬─────────────────┘
                           │
                ┌──────────▼────────────┐
                │  Cryo (Modified)      │
                │  - Log extraction     │
                │  - Schema transform   │
                │  - Parquet writer     │
                └──────────┬────────────┘
                           │
                ┌──────────▼────────────────────────────────┐
                │     Parquet Output Files                  │
                │  ethereum_logs_blocks-000000-000999.pq    │
                │  ethereum_logs_blocks-001000-001999.pq    │
                │  ethereum_logs_blocks-002000-002999.pq    │
                │  ... (100 files for 100K blocks)          │
                └───────────────────────────────────────────┘
```

### 1.2 Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Data Extraction** | Cryo (Rust) | Ethereum data extraction CLI |
| **Database Access** | reth_db crate | Direct MDBX database reads |
| **RPC Client** | ethers-rs | eth_getLogs fallback |
| **Parquet Writer** | arrow-rs (v53+) | Write Parquet files |
| **Compression** | zstd (level 3) | File compression |
| **Schema** | Apache Arrow | In-memory representation |

---

## 2. Parquet Schema Specification

### 2.1 Schema Definition

```sql
CREATE TABLE ethereum_logs (
    -- Block/Transaction identifiers
    block_number BIGINT NOT NULL,
    transaction_index INT NOT NULL,
    log_index INT NOT NULL,

    -- Transaction context
    transaction_hash BINARY(32) NOT NULL,
    block_timestamp BIGINT NOT NULL,

    -- Event data
    address BINARY(20) NOT NULL,

    -- Topics (denormalized for fast filtering)
    topic0 BINARY(32),
    topic1 BINARY(32),
    topic2 BINARY(32),
    topic3 BINARY(32),

    -- Event payload
    data BINARY,

    -- Metadata
    event_signature STRING,
    removed BOOLEAN DEFAULT false
)
STORED AS PARQUET
TBLPROPERTIES (
    'parquet.compression' = 'ZSTD',
    'parquet.compression.level' = '3',
    'parquet.row.group.size' = '8388608'  -- 8 MB
);
```

### 2.2 Rust Schema Implementation

```rust
use arrow::datatypes::{DataType, Field, Schema};
use std::sync::Arc;

fn ethereum_log_schema() -> Arc<Schema> {
    Arc::new(Schema::new(vec![
        // Identifiers
        Field::new("block_number", DataType::UInt64, false),
        Field::new("transaction_index", DataType::UInt32, false),
        Field::new("log_index", DataType::UInt32, false),

        // Transaction context
        Field::new("transaction_hash", DataType::FixedSizeBinary(32), false),
        Field::new("block_timestamp", DataType::UInt64, false),

        // Event data
        Field::new("address", DataType::FixedSizeBinary(20), false),
        Field::new("topic0", DataType::FixedSizeBinary(32), true),
        Field::new("topic1", DataType::FixedSizeBinary(32), true),
        Field::new("topic2", DataType::FixedSizeBinary(32), true),
        Field::new("topic3", DataType::FixedSizeBinary(32), true),
        Field::new("data", DataType::Binary, false),

        // Metadata
        Field::new("event_signature", DataType::Utf8, true),
        Field::new("removed", DataType::Boolean, false),
    ]))
}
```

### 2.3 File Naming Convention

**Pattern:** `ethereum_logs_blocks-{start:06d}-{end:06d}.parquet`

**Examples:**
```
ethereum_logs_blocks-000000-000999.parquet  (blocks 0-999)
ethereum_logs_blocks-001000-001999.parquet  (blocks 1000-1999)
ethereum_logs_blocks-099000-099999.parquet  (blocks 99000-99999)
```

**Rationale:**
- `ethereum_logs_` prefix: Self-describing, extensible for other datasets
- `blocks-XXXXXX-YYYYYY`: Clear block range with 6-digit zero-padding
- `.parquet` extension: Standard file format
- 1K blocks per file: Optimized for 75% of queries (single-block, small-range)

---

## 3. Data Pipeline Implementation

### 3.1 Extraction Flow

```rust
// High-level extraction pipeline
pub struct LogExtractor {
    data_source: DataSource,
    output_dir: PathBuf,
    batch_size: u64, // 1000 blocks
}

impl LogExtractor {
    pub async fn extract_range(&self, start_block: u64, end_block: u64) -> Result<()> {
        for chunk_start in (start_block..end_block).step_by(self.batch_size as usize) {
            let chunk_end = (chunk_start + self.batch_size).min(end_block);

            // 1. Extract logs for block range
            let logs = self.extract_logs(chunk_start, chunk_end).await?;

            // 2. Transform to Arrow RecordBatch
            let batch = self.transform_to_arrow(logs)?;

            // 3. Write Parquet file
            let filename = format!(
                "ethereum_logs_blocks-{:06}-{:06}.parquet",
                chunk_start, chunk_end - 1
            );
            self.write_parquet(&batch, &filename)?;
        }
        Ok(())
    }
}
```

### 3.2 Data Source Abstraction

```rust
pub enum DataSource {
    Mdbx(MdbxReader),
    Rpc(RpcClient),
}

impl DataSource {
    pub async fn get_logs(&self, start: u64, end: u64) -> Result<Vec<Log>> {
        match self {
            Self::Mdbx(reader) => reader.read_logs_range(start, end),
            Self::Rpc(client) => client.fetch_logs_batched(start, end).await,
        }
    }
}
```

---

## 4. Reth Integration Strategies

### 4.1 Option A: Direct MDBX Access (Recommended)

**Advantages:**
- 10-50× faster than RPC
- No rate limits or network latency
- Direct access to canonical chain data

**Implementation:**

```rust
use reth_db::{database::Database, tables, DatabaseEnv};
use reth_primitives::{Receipt, TransactionSigned};

pub struct MdbxReader {
    db: Arc<DatabaseEnv>,
}

impl MdbxReader {
    pub fn new(db_path: &Path) -> Result<Self> {
        let db = DatabaseEnv::open(
            db_path,
            Default::default(),
            None,
        )?;
        Ok(Self { db: Arc::new(db) })
    }

    pub fn read_logs_range(&self, start_block: u64, end_block: u64) -> Result<Vec<Log>> {
        let tx = self.db.tx()?;
        let mut logs = Vec::new();

        for block_num in start_block..end_block {
            // 1. Get block header for timestamp
            let header = tx
                .get::<tables::Headers>(block_num)?
                .ok_or_else(|| anyhow!("Block {} not found", block_num))?;

            // 2. Get block body to find transaction range
            let body = tx
                .get::<tables::BlockBodyIndices>(block_num)?
                .ok_or_else(|| anyhow!("Block body {} not found", block_num))?;

            // 3. Iterate transactions in block
            for tx_index in 0..(body.tx_count as u32) {
                let tx_num = body.first_tx_num + tx_index as u64;

                // 4. Get transaction for hash
                let transaction = tx
                    .get::<tables::Transactions>(tx_num)?
                    .ok_or_else(|| anyhow!("Transaction {} not found", tx_num))?;

                // 5. Get receipt for logs
                let receipt = tx
                    .get::<tables::Receipts>(tx_num)?
                    .ok_or_else(|| anyhow!("Receipt {} not found", tx_num))?;

                // 6. Extract logs from receipt
                for (log_index, log) in receipt.logs.iter().enumerate() {
                    logs.push(Log {
                        block_number: block_num,
                        transaction_index: tx_index,
                        log_index: log_index as u32,
                        transaction_hash: transaction.hash(),
                        block_timestamp: header.timestamp,
                        address: log.address,
                        topics: log.topics.clone(),
                        data: log.data.clone(),
                        removed: false,
                    });
                }
            }
        }

        Ok(logs)
    }
}
```

**Dependencies (Cargo.toml):**
```toml
[dependencies]
reth-db = "1.1"
reth-primitives = "1.1"
revm-primitives = "10.0"
```

### 4.2 Option B: RPC Fallback

**Use Cases:**
- Remote reth instance (different server)
- Testing without database access
- Fallback when MDBX unavailable

**Implementation:**

```rust
use ethers::prelude::*;
use ethers::types::Filter;

pub struct RpcClient {
    provider: Provider<Http>,
    batch_size: u64, // 100 blocks per eth_getLogs call
}

impl RpcClient {
    pub async fn fetch_logs_batched(&self, start: u64, end: u64) -> Result<Vec<Log>> {
        let mut all_logs = Vec::new();

        for chunk_start in (start..end).step_by(self.batch_size as usize) {
            let chunk_end = (chunk_start + self.batch_size).min(end);

            let filter = Filter::new()
                .from_block(chunk_start)
                .to_block(chunk_end);

            let logs = self.provider
                .get_logs(&filter)
                .await?;

            all_logs.extend(logs);
        }

        Ok(all_logs)
    }
}
```

### 4.3 Hybrid Deployment Strategy

```rust
pub fn create_data_source(config: &Config) -> Result<DataSource> {
    // Try MDBX first
    if let Some(db_path) = &config.reth_db_path {
        if db_path.exists() {
            info!("Using direct MDBX access: {:?}", db_path);
            return Ok(DataSource::Mdbx(MdbxReader::new(db_path)?));
        }
    }

    // Fallback to RPC
    info!("Using RPC fallback: {}", config.rpc_url);
    let provider = Provider::<Http>::try_from(&config.rpc_url)?;
    Ok(DataSource::Rpc(RpcClient::new(provider, 100)))
}
```

---

## 5. Parquet Writer Implementation

### 5.1 Arrow RecordBatch Construction

```rust
use arrow::array::*;
use arrow::record_batch::RecordBatch;

fn logs_to_record_batch(logs: Vec<Log>) -> Result<RecordBatch> {
    let schema = ethereum_log_schema();

    // Build arrays
    let block_numbers: UInt64Array = logs.iter().map(|l| l.block_number).collect();
    let tx_indices: UInt32Array = logs.iter().map(|l| l.transaction_index).collect();
    let log_indices: UInt32Array = logs.iter().map(|l| l.log_index).collect();

    let tx_hashes: FixedSizeBinaryArray = FixedSizeBinaryBuilder::with_capacity(logs.len(), 32)
        .extend(logs.iter().map(|l| Some(l.transaction_hash.as_bytes())))
        .finish();

    let timestamps: UInt64Array = logs.iter().map(|l| l.block_timestamp).collect();

    let addresses: FixedSizeBinaryArray = FixedSizeBinaryBuilder::with_capacity(logs.len(), 20)
        .extend(logs.iter().map(|l| Some(l.address.as_bytes())))
        .finish();

    // Topics (nullable)
    let topic0 = build_topic_array(&logs, 0);
    let topic1 = build_topic_array(&logs, 1);
    let topic2 = build_topic_array(&logs, 2);
    let topic3 = build_topic_array(&logs, 3);

    // Data (variable length binary)
    let data: BinaryArray = logs.iter()
        .map(|l| Some(l.data.as_ref()))
        .collect();

    // Event signature (optional, for UX)
    let signatures: StringArray = logs.iter()
        .map(|l| decode_event_signature(&l.topics))
        .collect();

    let removed: BooleanArray = logs.iter()
        .map(|l| l.removed)
        .collect();

    // Construct RecordBatch
    RecordBatch::try_new(
        schema,
        vec![
            Arc::new(block_numbers),
            Arc::new(tx_indices),
            Arc::new(log_indices),
            Arc::new(tx_hashes),
            Arc::new(timestamps),
            Arc::new(addresses),
            Arc::new(topic0),
            Arc::new(topic1),
            Arc::new(topic2),
            Arc::new(topic3),
            Arc::new(data),
            Arc::new(signatures),
            Arc::new(removed),
        ],
    )
}

fn build_topic_array(logs: &[Log], index: usize) -> FixedSizeBinaryArray {
    let mut builder = FixedSizeBinaryBuilder::with_capacity(logs.len(), 32);
    for log in logs {
        if let Some(topic) = log.topics.get(index) {
            builder.append_value(topic.as_bytes()).unwrap();
        } else {
            builder.append_null();
        }
    }
    builder.finish()
}
```

### 5.2 Parquet File Writer

```rust
use parquet::arrow::ArrowWriter;
use parquet::file::properties::WriterProperties;
use parquet::basic::Compression;
use std::fs::File;

fn write_parquet(batch: &RecordBatch, output_path: &Path) -> Result<()> {
    let file = File::create(output_path)?;

    // Configure writer properties
    let props = WriterProperties::builder()
        .set_compression(Compression::ZSTD(ZstdLevel::try_new(3)?))
        .set_max_row_group_size(8_388_608) // 8 MB
        .set_created_by("cryo-plinko-pir v1.0".to_string())
        .set_dictionary_enabled(true)
        .set_statistics_enabled(parquet::file::properties::EnabledStatistics::Page)
        .build();

    // Write to Parquet
    let mut writer = ArrowWriter::try_new(file, batch.schema(), Some(props))?;
    writer.write(batch)?;
    writer.close()?;

    info!("Wrote Parquet file: {} ({} logs)", output_path.display(), batch.num_rows());
    Ok(())
}
```

---

## 6. Deployment Scenarios

### 6.1 Local Testing

**Setup:**
```bash
# 1. Clone cryo
git clone https://github.com/paradigmxyz/cryo
cd cryo

# 2. Add plinko-pir branch with modifications
git checkout -b plinko-pir

# 3. Run against local reth
cargo run -- logs-plinko \
    --reth-db ~/.local/share/reth/mainnet/db \
    --blocks 21000000:21100000 \
    --output ./parquet_output
```

**Expected Output:**
```
parquet_output/
├── ethereum_logs_blocks-21000000-21000999.parquet (8 MB)
├── ethereum_logs_blocks-21001000-21001999.parquet (8 MB)
├── ...
└── ethereum_logs_blocks-21099000-21099999.parquet (8 MB)

Total: 100 files × 8 MB = 800 MB
```

### 6.2 Remote Production (SSH Tunnel)

**Architecture:**
```
┌─────────────────────┐         SSH Tunnel          ┌─────────────────────┐
│  Cryo Service       │    ───────────────────────>  │  Reth Server        │
│  (Processing)       │    <───────────────────────  │  (Database)         │
│  192.168.1.100      │                              │  192.168.1.200      │
└─────────────────────┘                              └─────────────────────┘
```

**Setup:**
```bash
# 1. Create SSH tunnel for database access
ssh -L 9001:/path/to/reth/db user@reth-server -N &

# 2. Run cryo against tunneled database
cargo run -- logs-plinko \
    --reth-db /mnt/tunnel/db \
    --blocks latest:100000 \
    --output /data/parquet \
    --incremental \
    --poll-interval 12s
```

### 6.3 Remote Production (RPC Only)

**Use when:** Database access not available

**Setup:**
```bash
cargo run -- logs-plinko \
    --rpc http://reth-server:8545 \
    --blocks latest:100000 \
    --output /data/parquet \
    --incremental \
    --poll-interval 12s \
    --rpc-batch-size 100
```

**Performance:**
- MDBX: 100K blocks in 8-10 minutes
- RPC: 100K blocks in 25-30 minutes

---

## 7. Incremental Update Mechanism

### 7.1 Continuous Extraction Mode

```rust
pub struct IncrementalExtractor {
    extractor: LogExtractor,
    last_processed_block: u64,
    poll_interval: Duration, // 12 seconds
}

impl IncrementalExtractor {
    pub async fn run(&mut self) -> Result<()> {
        loop {
            // 1. Get latest block
            let latest_block = self.extractor.get_latest_block().await?;

            // 2. Calculate next chunk
            let chunk_start = self.last_processed_block + 1;
            let chunk_end = chunk_start + 1000;

            // 3. Extract if chunk complete
            if chunk_end <= latest_block {
                info!("Processing blocks {}-{}", chunk_start, chunk_end - 1);
                self.extractor.extract_range(chunk_start, chunk_end).await?;
                self.last_processed_block = chunk_end - 1;
            }

            // 4. Wait for next block
            tokio::time::sleep(self.poll_interval).await;
        }
    }
}
```

### 7.2 State Persistence

**File:** `state.json`
```json
{
  "last_processed_block": 21050999,
  "last_update_timestamp": 1731340800,
  "total_files_generated": 51,
  "total_logs_extracted": 4080000
}
```

**Implementation:**
```rust
#[derive(Serialize, Deserialize)]
struct ExtractorState {
    last_processed_block: u64,
    last_update_timestamp: u64,
    total_files_generated: u64,
    total_logs_extracted: u64,
}

impl IncrementalExtractor {
    fn save_state(&self) -> Result<()> {
        let state = ExtractorState {
            last_processed_block: self.last_processed_block,
            last_update_timestamp: SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs(),
            total_files_generated: self.stats.files_generated,
            total_logs_extracted: self.stats.logs_extracted,
        };

        let json = serde_json::to_string_pretty(&state)?;
        std::fs::write("state.json", json)?;
        Ok(())
    }

    fn load_state(&mut self) -> Result<()> {
        if let Ok(json) = std::fs::read_to_string("state.json") {
            let state: ExtractorState = serde_json::from_str(&json)?;
            self.last_processed_block = state.last_processed_block;
            info!("Resumed from block {}", state.last_processed_block);
        }
        Ok(())
    }
}
```

---

## 8. Cryo CLI Integration

### 8.1 New Subcommand

**Command:** `cryo logs-plinko`

**Usage:**
```bash
cryo logs-plinko [OPTIONS]

OPTIONS:
    --reth-db <PATH>           Path to reth MDBX database
    --rpc <URL>                RPC endpoint (fallback)
    --blocks <RANGE>           Block range (e.g., "21000000:21100000" or "latest:100000")
    --output <DIR>             Output directory for Parquet files
    --incremental              Enable continuous extraction mode
    --poll-interval <SECONDS>  Block polling interval (default: 12s)
    --rpc-batch-size <N>       eth_getLogs batch size (default: 100)
    --compression <LEVEL>      ZSTD compression level (default: 3)

EXAMPLES:
    # Extract 100K blocks from local reth
    cryo logs-plinko --reth-db ~/.local/share/reth/mainnet/db \
                     --blocks 21000000:21100000 \
                     --output ./parquet

    # Continuous extraction via RPC
    cryo logs-plinko --rpc http://localhost:8545 \
                     --blocks latest:100000 \
                     --output /data/parquet \
                     --incremental

    # Remote database via SSH tunnel
    cryo logs-plinko --reth-db /mnt/tunnel/reth/db \
                     --blocks latest:50000 \
                     --output /data/parquet \
                     --incremental \
                     --poll-interval 12
```

### 8.2 Configuration File

**File:** `cryo-plinko.toml`
```toml
[source]
# Data source (priority: reth_db > rpc)
reth_db = "/path/to/reth/mainnet/db"
rpc = "http://localhost:8545"
rpc_batch_size = 100

[extraction]
# Block range
start_block = "latest"
block_count = 100000
chunk_size = 1000  # blocks per file

[output]
# Output configuration
output_dir = "/data/parquet"
compression_level = 3
naming_pattern = "ethereum_logs_blocks-{start:06}-{end:06}.parquet"

[incremental]
# Continuous mode
enabled = true
poll_interval = 12  # seconds
state_file = "state.json"

[performance]
# Performance tuning
max_concurrent_files = 4
buffer_size = 1048576  # 1 MB
```

**Load config:**
```bash
cryo logs-plinko --config cryo-plinko.toml
```

---

## 9. Testing & Validation

### 9.1 Unit Tests

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_schema_construction() {
        let schema = ethereum_log_schema();
        assert_eq!(schema.fields().len(), 13);
        assert_eq!(schema.field(0).name(), "block_number");
    }

    #[test]
    fn test_filename_generation() {
        let name = format_filename(21000000, 21000999);
        assert_eq!(name, "ethereum_logs_blocks-21000000-21000999.parquet");
    }

    #[tokio::test]
    async fn test_log_extraction() {
        let extractor = create_test_extractor();
        let logs = extractor.extract_logs(21000000, 21001000).await.unwrap();
        assert!(!logs.is_empty());
    }
}
```

### 9.2 Integration Tests

**Test 1: MDBX vs RPC Consistency**
```rust
#[tokio::test]
async fn test_mdbx_rpc_consistency() {
    let mdbx_logs = extract_via_mdbx(21000000, 21001000).await.unwrap();
    let rpc_logs = extract_via_rpc(21000000, 21001000).await.unwrap();

    assert_eq!(mdbx_logs.len(), rpc_logs.len());
    for (m, r) in mdbx_logs.iter().zip(rpc_logs.iter()) {
        assert_eq!(m.transaction_hash, r.transaction_hash);
        assert_eq!(m.log_index, r.log_index);
    }
}
```

**Test 2: Parquet Roundtrip**
```rust
#[test]
fn test_parquet_roundtrip() {
    let original_logs = generate_test_logs(1000);
    let batch = logs_to_record_batch(original_logs.clone()).unwrap();

    let temp_file = tempfile::NamedTempFile::new().unwrap();
    write_parquet(&batch, temp_file.path()).unwrap();

    let reader = ParquetRecordBatchReader::try_new(
        File::open(temp_file.path()).unwrap(),
        1024
    ).unwrap();

    let read_batch = reader.collect::<Result<Vec<_>>>().unwrap()[0].clone();
    assert_eq!(batch.num_rows(), read_batch.num_rows());
}
```

### 9.3 Performance Benchmarks

**Benchmark 1: Extraction Speed**
```bash
# MDBX (target: <10 minutes for 100K blocks)
time cargo run --release -- logs-plinko \
    --reth-db ~/.local/share/reth/mainnet/db \
    --blocks 21000000:21100000 \
    --output /tmp/parquet

# Expected: 8-10 minutes
```

**Benchmark 2: File Size Validation**
```bash
# Verify file sizes (target: ~8 MB per file)
ls -lh /tmp/parquet/*.parquet | awk '{print $5}'

# Expected: 7.5-8.5 MB per file
```

**Benchmark 3: Incremental Update**
```bash
# Measure time for 1K-block update (target: <30 seconds)
time cargo run --release -- logs-plinko \
    --reth-db ~/.local/share/reth/mainnet/db \
    --blocks 21100000:21101000 \
    --output /tmp/parquet

# Expected: 25-30 seconds
```

---

## 10. Implementation Checklist

### Phase 1: Core Implementation (Week 1-2)

- [ ] Fork Cryo repository
- [ ] Add `reth-db` and `arrow-rs` dependencies
- [ ] Implement `ethereum_log_schema()` function
- [ ] Implement `MdbxReader` struct with `read_logs_range()`
- [ ] Implement `logs_to_record_batch()` transformation
- [ ] Implement `write_parquet()` with ZSTD compression
- [ ] Add `logs-plinko` subcommand to Cryo CLI
- [ ] Unit tests for schema and transformations

### Phase 2: Integration & Testing (Week 3-4)

- [ ] Implement `RpcClient` fallback strategy
- [ ] Add hybrid data source selection logic
- [ ] Integration tests (MDBX vs RPC consistency)
- [ ] Benchmark extraction speed (100K blocks)
- [ ] Validate Parquet file format (parquet-tools)
- [ ] Test file size (should be ~8 MB per 1K blocks)

### Phase 3: Incremental Updates (Week 5)

- [ ] Implement `IncrementalExtractor` with state persistence
- [ ] Add `--incremental` flag to CLI
- [ ] Test continuous extraction (12-second polling)
- [ ] Validate incremental file generation
- [ ] Error handling and retry logic

### Phase 4: Production Hardening (Week 6-7)

- [ ] Configuration file support (`cryo-plinko.toml`)
- [ ] Logging and metrics (structured logs)
- [ ] SSH tunnel deployment testing
- [ ] Performance optimization (parallel writes)
- [ ] Memory profiling and optimization
- [ ] Documentation and examples

---

## 11. Dependencies

### Cargo.toml

```toml
[package]
name = "cryo"
version = "0.4.0"
edition = "2021"

[dependencies]
# Reth database access
reth-db = "1.1"
reth-primitives = "1.1"
revm-primitives = "10.0"

# Apache Arrow & Parquet
arrow = "53.3"
parquet = "53.3"

# RPC client (fallback)
ethers = "2.0"

# Compression
zstd = "0.13"

# Async runtime
tokio = { version = "1.40", features = ["full"] }

# Serialization
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

# CLI
clap = { version = "4.5", features = ["derive"] }

# Logging
tracing = "0.1"
tracing-subscriber = "0.3"

# Error handling
anyhow = "1.0"
thiserror = "1.0"

[dev-dependencies]
tempfile = "3.13"
criterion = "0.5"
```

---

## 12. Performance Requirements

| Metric | Target | Measured | Status |
|--------|--------|----------|--------|
| **Extraction (MDBX)** | <10 min (100K blocks) | TBD | ⏳ |
| **Extraction (RPC)** | <30 min (100K blocks) | TBD | ⏳ |
| **Incremental Update** | <30 sec (1K blocks) | TBD | ⏳ |
| **File Size** | 8 MB ± 0.5 MB | TBD | ⏳ |
| **Compression Ratio** | 3-4× (vs raw JSON) | TBD | ⏳ |
| **Memory Usage** | <2 GB peak | TBD | ⏳ |
| **Correctness** | 100% match vs eth_getLogs | TBD | ⏳ |

---

## 13. References

### Technical Documentation

- **Cryo GitHub**: https://github.com/paradigmxyz/cryo
- **Reth Database**: https://github.com/paradigmxyz/reth/tree/main/crates/storage/db
- **Apache Arrow Rust**: https://docs.rs/arrow/latest/arrow/
- **Apache Parquet Rust**: https://docs.rs/parquet/latest/parquet/
- **Plinko PIR Paper**: https://eprint.iacr.org/2024/318

### Related Research

- [IPFS + Parquet Storage Schema](./findings/ipfs-parquet-storage-schema.md)
- [Fixed-Size Log Compression](./findings/fixed-size-log-compression.md)
- [50K Blocks eth_getLogs Analysis](./findings/eth-logs-50k-blocks.md)

---

**Document Owner:** Plinko PIR Research Team
**Implementation Owner:** Rust Development Team
**Last Updated:** 2025-11-11
