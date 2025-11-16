#!/usr/bin/env python3
"""
Test runner for Plinko PIR reference implementation.
"""

import sys
import pytest
import logging

def main():
    """Run all tests."""
    # Configure logging for tests
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )
    
    print("🧪 Running Plinko PIR Reference Implementation Tests")
    print("=" * 60)
    
    # Run pytest with verbose output
    exit_code = pytest.main([
        'tests/',
        '-v',
        '--tb=short',
        '--strict-markers'
    ])
    
    if exit_code == 0:
        print("\n✅ All tests passed!")
    else:
        print(f"\n❌ Tests failed with exit code {exit_code}")
    
    return exit_code


if __name__ == '__main__':
    sys.exit(main())