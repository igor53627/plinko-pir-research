#!/usr/bin/env python3
"""
Demo script for Plinko PIR reference implementation.

Shows how to use the core components for security auditing.
"""

import tempfile
import struct
import os
import logging

from database import PlinkoDatabase
from prset import PRSet, PrfKey128
from plinko_core import PlinkoPIRServer

def setup_logging():
    """Setup logging for the demo."""
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(levelname)s - %(message)s'
    )

def create_test_database():
    """Create a test database for demonstration."""
    # Create temporary database file
    with tempfile.NamedTemporaryFile(delete=False, suffix='.bin') as f:
        # Write test data: values 0, 1000, 2000, ..., 999000
        for i in range(1000):
            value = i * 1000
            f.write(struct.pack('>Q', value))  # Big-endian uint64
        
        db_path = f.name
    
    print(f"📁 Created test database: {db_path}")
    return db_path

def demo_database_operations(db_path):
    """Demonstrate database operations."""
    print("\n📊 Database Operations Demo")
    print("-" * 30)
    
    # Load database
    database = PlinkoDatabase(db_path)
    database.load()
    
    print(f"Database size: {database.get_size()} entries")
    print(f"Database size: {database.get_size_mb():.2f} MB")
    
    # Query some entries
    for i in [0, 100, 500, 999]:
        value = database.get_entry(i)
        print(f"  Entry {i}: {value}")

def demo_prset_operations():
    """Demonstrate PRSet (Pseudorandom Set) operations."""
    print("\n🔐 PRSet Operations Demo")
    print("-" * 30)
    
    # Create PRF keys
    key1 = PrfKey128(b'\x00' * 16)  # Deterministic key
    key2 = PrfKey128(b'\x01' * 16)  # Different deterministic key
    
    # Create PRSets
    prset1 = PRSet(key1)
    prset2 = PRSet(key2)
    prset3 = PRSet(key1)  # Same key as prset1
    
    # Demonstrate set expansion
    set_size = 10
    chunk_size = 100
    
    indices1 = prset1.expand(set_size, chunk_size)
    indices2 = prset2.expand(set_size, chunk_size)
    indices3 = prset3.expand(set_size, chunk_size)
    
    print(f"PRSet1 indices: {indices1}")
    print(f"PRSet2 indices: {indices2}")
    print(f"PRSet3 indices: {indices3}")
    
    # Show determinism
    print(f"PRSet1 == PRSet3 (same key): {indices1 == indices3}")
    print(f"PRSet1 == PRSet2 (different keys): {indices1 == indices2}")

def demo_pir_operations(db_path):
    """Demonstrate PIR operations."""
    print("\n🔍 PIR Operations Demo")
    print("-" * 30)
    
    # Create server
    database = PlinkoDatabase(db_path)
    database.load()
    server = PlinkoPIRServer(database)
    
    print(f"Server health: {server.health_check()}")
    
    # Demo plaintext query
    print("\n📋 Plaintext Query:")
    result = server.plaintext_query(500)
    print(f"  Index 500: value={result['value']}, time={result['server_time_nanos']}ns")
    
    # Demo set parity query
    print("\n🔄 Set Parity Query:")
    indices = [0, 1, 2, 3, 4]
    result = server.set_parity_query(indices)
    print(f"  Indices {indices}: parity={result['parity']}, time={result['server_time_nanos']}ns")
    
    # Demo full set query
    print("\n🎯 Full Set Query:")
    prf_key = b'\x42' * 16  # Test PRF key
    result = server.full_set_query(prf_key)
    print(f"  PRF result: value={result['value']}, time={result['server_time_nanos']}ns")

def demo_security_features():
    """Demonstrate security features."""
    print("\n🛡️ Security Features Demo")
    print("-" * 30)
    
    print("✅ No query logging - protects client privacy")
    print("✅ Input validation on all operations")
    print("✅ Deterministic PRF with AES-128")
    print("✅ Proper error handling without information leakage")
    print("✅ Clean, auditable implementation")

def main():
    """Main demo function."""
    setup_logging()
    
    print("🚀 Plinko PIR Reference Implementation Demo")
    print("=" * 50)
    print("This demo shows the core functionality for security auditing.")
    print()
    
    try:
        # Create test database
        db_path = create_test_database()
        
        # Run demos
        demo_database_operations(db_path)
        demo_prset_operations()
        demo_pir_operations(db_path)
        demo_security_features()
        
        print("\n" + "=" * 50)
        print("✅ Demo completed successfully!")
        print("\nFor security auditing:")
        print("- Review the source code in this directory")
        print("- Run tests: python run_tests.py")
        print("- Start server: python plinko_server.py --help")
        
    except Exception as e:
        print(f"\n❌ Demo failed: {e}")
        raise
    
    finally:
        # Cleanup
        if 'db_path' in locals():
            try:
                os.unlink(db_path)
                print(f"\n🗑️  Cleaned up test database: {db_path}")
            except:
                pass


if __name__ == '__main__':
    main()