#!/bin/sh
set -e

echo "Plinko PIR Hint Generator - Wrapper Script"
echo "=========================================="
echo ""

# Wait for database.bin to exist
echo "Checking for database.bin..."
while [ ! -f /data/database.bin ]; do
    echo "  Waiting for database.bin to be generated..."
    sleep 2
done

# Check database.bin size
DB_SIZE=$(stat -c%s /data/database.bin 2>/dev/null || stat -f%z /data/database.bin 2>/dev/null)
EXPECTED_SIZE=67108864  # 8,388,608 × 8 bytes

echo "✅ database.bin found"
echo "  Size: $DB_SIZE bytes (expected: $EXPECTED_SIZE)"
echo ""

# Run hint generator
echo "Starting hint generation..."
/app/hint-generator

echo ""
echo "Hint generation complete!"
