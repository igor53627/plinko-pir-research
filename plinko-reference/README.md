# Plinko PIR Reference Implementation (Python)

This is a clean Python reference implementation of the Plinko PIR (Private Information Retrieval) server designed for security auditing and educational purposes. It prioritizes code clarity, security best practices, and mathematical correctness.

## Overview

The Plinko PIR (Private Information Retrieval) system allows clients to query a database without revealing which specific item they're interested in. This Python implementation provides:

- **Pseudorandom Set Expansion**: Generate consistent query sets using AES-based PRF
- **Private Query Processing**: Handle PIR queries without logging sensitive information  
- **HTTP API**: RESTful endpoints for client interaction
- **Security-Focused Design**: Clear, auditable code for security review

## Architecture

```
plkno-reference/
├── plinko_server.py      # Main server implementation
├── prset.py              # Pseudorandom set expansion (AES-PRF)
├── config.py             # Configuration management
├── database.py           # Database loading and management
├── handlers.py           # HTTP request handlers
├── utils.py              # Utility functions
└── tests/                # Test suite
    ├── test_prset.py
    ├── test_server.py
    └── test_integration.py
```

## Key Components

### 1. PRSet (Pseudorandom Set)
- AES-128 based PRF for deterministic set generation
- Expands a key into a set of database indices
- Provides cryptographically secure pseudorandom set expansion

### 2. PlinkoPIRServer
- Loads and manages the canonical database
- Processes PIR queries without revealing client interests
- Provides health monitoring and metrics

### 3. HTTP API Endpoints
- `GET /health` - Health check
- `POST /query/plaintext` - Plaintext query
- `POST /query/fullset` - Full set query  
- `POST /query/setparity` - Set parity query

## Security Features

- **No Query Logging**: Server never logs queried indices to protect privacy
- **Deterministic PRF**: Same key produces same sets for consistency
- **Input Validation**: All inputs are validated before processing
- **Error Handling**: Secure error handling without information leakage

## Installation

```bash
pip install -r requirements.txt
```

## Usage

```bash
python plinko_server.py --database-path /path/to/database.bin --port 8080
```

## Testing

```bash
python -m pytest tests/ -v
```

## Technical Specifications

This implementation provides:
- AES-128 ECB mode for pseudorandom functions
- Big-endian encoding for binary data
- RESTful HTTP API endpoints
- JSON request/response formats
- Binary database file format

## Security Audit Notes

This implementation prioritizes security and clarity:
- All cryptographic operations use well-audited standard libraries
- Clear separation of concerns between components
- Extensive input validation and secure error handling
- Comprehensive documentation and test coverage
- Designed for educational and audit purposes

## Quick Start for Security Auditing

1. **Review Core Components**:
   ```bash
   # Read the main security-critical files
   cat prset.py      # Cryptographic PRF implementation
   cat database.py   # Database access with privacy protection
   cat plinko_core.py # PIR query processing
   cat handlers.py   # HTTP request handling
   ```

2. **Run Security Tests**:
   ```bash
   # Test basic functionality
   python test_structure.py
   
   # Run comprehensive tests
   python -m pytest tests/ -v
   
   # Interactive demo
   python demo.py
   ```

3. **Manual Security Testing**:
   ```bash
   # Start server
   python plinko_server.py --port 8080 &
   
   # Test with invalid inputs
   curl -X POST http://localhost:8080/query/plaintext -d '{"index": -1}'
   curl -X POST http://localhost:8080/query/fullset -d '{"prf_key": "invalid"}'
   ```

## Files to Review for Security Audit

### Core Security Components
1. **`prset.py`** - Cryptographic PRF implementation using AES-128
2. **`database.py`** - Database access with privacy protection (no logging)
3. **`plinko_core.py`** - PIR query processing with input validation
4. **`handlers.py`** - HTTP request handling with security middleware

### Configuration and Utilities
5. **`config.py`** - Configuration validation
6. **`utils.py`** - Security utilities and validation functions

### Tests (for understanding expected behavior)
7. **`tests/test_prset.py`** - Cryptographic component tests
8. **`tests/test_server.py`** - Integration tests

## Privacy Guarantees

This implementation maintains strong privacy properties:

1. **Query Privacy**: Server cannot determine which specific index was queried
2. **Deterministic Operation**: Same inputs produce same outputs (no state needed)
3. **No Side Channels**: No timing or logging side channels that reveal queries
4. **Input Sanitization**: All inputs are validated to prevent injection attacks

## Mathematical Foundation

The Plinko PIR system is based on:
- **Pseudorandom Functions**: AES-128 for deterministic set generation
- **XOR Properties**: Using XOR for private aggregation
- **Database Partitioning**: Dividing database into chunks for efficiency
- **Probabilistic Sets**: Random-looking but deterministic index selection

## Performance Characteristics

- **Query Latency**: O(1) for plaintext queries, O(set_size) for set queries
- **Memory Usage**: O(database_size) for full database loading
- **Network Overhead**: Minimal - only query parameters sent over network
- **CPU Usage**: Primarily AES operations and XOR computations

## Conclusion

This reference implementation provides a clean, auditable version of the Plinko PIR server that prioritizes security understanding over performance optimization. Every security-critical function is well-documented and tested, making it ideal for security review and educational purposes.