#!/bin/bash

# Setup script for FrodoPIR + Ethereum RPC Research
# This demonstrates reproducible research setup

set -e

echo "🔬 Setting up FrodoPIR Ethereum RPC Analysis Project..."
echo ""

# Check prerequisites
echo "📋 Checking prerequisites..."

if ! command -v git &> /dev/null; then
    echo "❌ git is required but not installed"
    exit 1
fi

if ! command -v rustc &> /dev/null; then
    echo "⚠️  Rust not found. Install from: https://rustup.rs"
    MISSING_RUST=true
else
    echo "✅ Rust $(rustc --version)"
fi

if ! command -v cargo &> /dev/null && [ -z "$MISSING_RUST" ]; then
    echo "❌ cargo is required"
    exit 1
fi

if ! command -v g++ &> /dev/null && ! command -v clang++ &> /dev/null; then
    echo "⚠️  C++ compiler not found (needed for C++ implementation)"
    MISSING_CPP=true
else
    if command -v g++ &> /dev/null; then
        echo "✅ g++ $(g++ --version | head -1)"
    else
        echo "✅ clang++ $(clang++ --version | head -1)"
    fi
fi

if ! command -v python3 &> /dev/null; then
    echo "⚠️  Python3 not found (needed for analysis scripts)"
    MISSING_PYTHON=true
else
    echo "✅ Python3 $(python3 --version)"
fi

echo ""

# Create directory structure
echo "📁 Creating project structure..."
mkdir -p src/{rust-analysis,cpp-analysis,ethereum-data}
mkdir -p tests
mkdir -p findings
mkdir -p benchmarks
echo "✅ Directory structure created"
echo ""

# Clone FrodoPIR implementations
echo "📦 Cloning FrodoPIR implementations..."

if [ ! -d "src/rust-analysis/frodo-pir" ]; then
    echo "  Cloning Rust implementation (brave-experiments)..."
    git clone https://github.com/brave-experiments/frodo-pir.git \
        src/rust-analysis/frodo-pir
    echo "  ✅ Rust implementation cloned"
else
    echo "  ✅ Rust implementation already exists"
fi

if [ ! -d "src/cpp-analysis/frodoPIR" ]; then
    echo "  Cloning C++ implementation (itzmeanjan)..."
    git clone --recurse-submodules \
        https://github.com/itzmeanjan/frodoPIR.git \
        src/cpp-analysis/frodoPIR
    echo "  ✅ C++ implementation cloned"
else
    echo "  ✅ C++ implementation already exists"
fi

echo ""

# Build Rust implementation (if Rust available)
if [ -z "$MISSING_RUST" ]; then
    echo "🔨 Building Rust implementation..."
    cd src/rust-analysis/frodo-pir
    if cargo build --release 2>&1 | tail -5; then
        echo "✅ Rust implementation built successfully"
    else
        echo "⚠️  Rust build encountered issues (check output above)"
    fi
    cd ../../..
    echo ""
fi

# Build C++ implementation (if C++ available)
if [ -z "$MISSING_CPP" ]; then
    echo "🔨 Building C++ implementation..."
    cd src/cpp-analysis/frodoPIR
    if make lib -j 2>&1 | tail -10; then
        echo "✅ C++ implementation built successfully"
    else
        echo "⚠️  C++ build encountered issues (check output above)"
    fi
    cd ../../..
    echo ""
fi

# Create analysis templates
echo "📝 Creating analysis templates..."

cat > findings/performance-comparison.md << 'EOF'
# Performance Comparison: Rust vs C++ FrodoPIR

## Test Configuration

**Hardware**:
- CPU:
- RAM:
- OS:

**FrodoPIR Parameters**:
- Database size:
- Element size:
- LWE dimension:

## Benchmark Results

### Rust Implementation

| Operation | Time | Notes |
|-----------|------|-------|
| Server Setup | | |
| Client Setup | | |
| Client Query | | |
| Server Response | | |
| Client Decode | | |

### C++ Implementation

| Operation | Time | Notes |
|-----------|------|-------|
| Server Setup | | |
| Client Setup | | |
| Client Query | | |
| Server Response | | |
| Client Decode | | |

## Analysis

### Performance Differences

### Memory Usage

### Bandwidth

## Conclusions
EOF

cat > findings/ethereum-mapping.md << 'EOF'
# Ethereum State to FrodoPIR Database Mapping

## Database Design Options

### Option A: Full State Snapshot

**Scope**: All Ethereum accounts
**Database Size**: ~250M entries
**Element Size**: TBD based on data included

**Pros**:
- Complete privacy set
- All queries supported

**Cons**:
- Massive offline phase
- Frequent updates needed

### Option B: Active Account Subset

### Option C: Time-Windowed Data

### Option D: Hybrid Approach

## Parameter Calculations

## Privacy Analysis

## Conclusion
EOF

cat > setup-notes.md << 'EOF'
# Setup Notes and Issues

## Build Environment

Date:
System:

## Rust Implementation

Build command used:
Issues encountered:
Workarounds:

## C++ Implementation

Build command used:
Issues encountered:
Workarounds:

## Dependencies

Additional packages installed:

## Notes
EOF

echo "✅ Analysis templates created"
echo ""

# Create Python analysis scripts directory
if [ -z "$MISSING_PYTHON" ]; then
    echo "🐍 Setting up Python analysis environment..."
    mkdir -p scripts

    cat > scripts/analyze_benchmarks.py << 'EOF'
#!/usr/bin/env python3
"""
Analyze FrodoPIR benchmark results for Ethereum use case
"""

import json
import sys

def analyze_rust_benchmarks(benchmark_file):
    """Parse Criterion benchmark results"""
    # TODO: Implement parsing
    pass

def analyze_cpp_benchmarks(benchmark_file):
    """Parse Google Benchmark results"""
    # TODO: Implement parsing
    pass

def calculate_ethereum_projections(results):
    """Project performance to Ethereum scale"""
    # TODO: Implement projections
    pass

if __name__ == "__main__":
    print("Benchmark analysis script - TODO: Implement")
EOF

    chmod +x scripts/analyze_benchmarks.py
    echo "✅ Python scripts created"
    echo ""
fi

# Summary
echo ""
echo "✨ Setup complete!"
echo ""
echo "📚 Next steps:"
echo "  1. Review README.md for research overview"
echo "  2. Review research-plan.md for detailed methodology"
echo "  3. Start with Phase 1: Deep Technical Analysis"
echo "  4. Run benchmarks: cd src/rust-analysis/frodo-pir && cargo bench"
echo "  5. Document findings in findings/ directory"
echo ""

if [ ! -z "$MISSING_RUST" ]; then
    echo "⚠️  Install Rust to build and test implementations"
fi

if [ ! -z "$MISSING_CPP" ]; then
    echo "⚠️  Install C++ compiler for C++ implementation"
fi

if [ ! -z "$MISSING_PYTHON" ]; then
    echo "⚠️  Install Python3 for analysis scripts"
fi

echo ""
echo "🔬 Happy researching!"
