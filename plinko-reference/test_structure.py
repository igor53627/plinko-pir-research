#!/usr/bin/env python3
"""
Basic structure test for Plinko PIR reference implementation.
"""

import sys
import os

def test_imports():
    """Test that all modules can be imported."""
    print("Testing module imports...")
    
    # Test basic imports (without external dependencies)
    try:
        # These should work without external dependencies
        from config import PlinkoConfig, load_config
        from database import PlinkoDatabase, DatabaseError
        from utils import uint64_to_bytes, bytes_to_uint64, validate_index
        from handlers import cors_middleware, validate_json_request
        
        print("✅ Basic modules imported successfully")
        return True
        
    except ImportError as e:
        print(f"❌ Import error: {e}")
        return False

def test_constants():
    """Test that constants are defined correctly."""
    print("Testing constants...")
    
    from utils import DB_ENTRY_SIZE, DB_ENTRY_LENGTH
    
    assert DB_ENTRY_SIZE == 8, f"Expected DB_ENTRY_SIZE=8, got {DB_ENTRY_SIZE}"
    assert DB_ENTRY_LENGTH == 1, f"Expected DB_ENTRY_LENGTH=1, got {DB_ENTRY_LENGTH}"
    
    print("✅ Constants are correct")
    return True

def test_basic_functions():
    """Test basic utility functions."""
    print("Testing basic functions...")
    
    from utils import uint64_to_bytes, bytes_to_uint64, compute_xor
    
    # Test uint64 conversion
    test_value = 0x1234567890ABCDEF
    bytes_result = uint64_to_bytes(test_value)
    assert len(bytes_result) == 8, "uint64_to_bytes should return 8 bytes"
    
    # Test round-trip conversion
    back_to_int = bytes_to_uint64(bytes_result)
    assert back_to_int == test_value, "Round-trip conversion failed"
    
    # Test XOR computation
    values = [0xFF, 0x0F, 0xF0]
    result = compute_xor(values)
    expected = 0xFF ^ 0x0F ^ 0xF0
    assert result == expected, f"XOR computation failed: {result} != {expected}"
    
    print("✅ Basic functions work correctly")
    return True

def test_file_structure():
    """Test that all expected files exist."""
    print("Testing file structure...")
    
    expected_files = [
        'README.md',
        'requirements.txt',
        'config.py',
        'database.py',
        'prset.py',
        'utils.py',
        'handlers.py',
        'plinko_core.py',
        'plinko_server.py',
        'plinko_server_simple.py',
        'demo.py',
        'run_tests.py',
        'tests/test_prset.py',
        'tests/test_server.py',
        'tests/__init__.py'
    ]
    
    missing_files = []
    for file_path in expected_files:
        if not os.path.exists(file_path):
            missing_files.append(file_path)
    
    if missing_files:
        print(f"❌ Missing files: {missing_files}")
        return False
    
    print("✅ All expected files exist")
    return True

def main():
    """Run all structure tests."""
    print("🔍 Plinko PIR Reference Implementation - Structure Test")
    print("=" * 60)
    
    # Add current directory to path
    sys.path.insert(0, '.')
    
    tests = [
        test_file_structure,
        test_imports,
        test_constants,
        test_basic_functions,
    ]
    
    passed = 0
    failed = 0
    
    for test in tests:
        try:
            if test():
                passed += 1
            else:
                failed += 1
        except Exception as e:
            print(f"❌ Test {test.__name__} failed with exception: {e}")
            failed += 1
        print()
    
    print("=" * 60)
    print(f"Results: {passed} passed, {failed} failed")
    
    if failed == 0:
        print("\n✅ All structure tests passed!")
        print("\nNext steps:")
        print("1. Install dependencies: pip install -r requirements.txt")
        print("2. Run demo: python demo.py")
        print("3. Run tests: python run_tests.py")
        print("4. Start server: python plinko_server.py --help")
    else:
        print(f"\n❌ {failed} tests failed")
    
    return failed == 0

if __name__ == '__main__':
    import sys
    sys.exit(0 if main() else 1)