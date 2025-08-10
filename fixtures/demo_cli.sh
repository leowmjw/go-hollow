#!/bin/bash

echo "=== Go-Hollow CLI Interactive Demo ==="
echo
echo "This script demonstrates the key features of the Go-Hollow CLI:"
echo "1. Producer-Consumer Workflow"
echo "2. Delta Evolution and Blob Management"
echo "3. Data Inspection and Comparison"
echo "4. Zero-Copy Integration"
echo

# Check if we're in the right directory
if [ ! -f "hollow-cli" ]; then
    echo "❌ Error: hollow-cli binary not found"
    echo "Please run this script from the go-hollow root directory where hollow-cli is located"
    exit 1
fi

if [ ! -f "fixtures/test_data.json" ]; then
    echo "❌ Error: fixtures/test_data.json not found"
    echo "Please ensure the fixtures directory contains test_data.json"
    exit 1
fi

echo "🚀 Starting comprehensive CLI demo..."
echo

# Create interactive command sequence
cat << 'EOF' > /tmp/hollow_demo_commands.txt
l blobs
c 1
i 1
p add 3
l blobs
i 2
p update 2
l blobs
i 3
p mixed
l blobs
i 4
d 1 4
c 4
q
EOF

echo "📋 Demo Commands:"
echo "l blobs       - List all blob versions"
echo "c 1          - Consume data from version 1"
echo "i 1          - Inspect version 1 details"
echo "p add 3      - Produce 3 new records"
echo "p update 2   - Update 2 existing records"
echo "p mixed      - Mixed operations (add/update/delete)"
echo "d 1 4        - Compare differences between versions 1 and 4"
echo
echo "⏱️  Running demo session..."
echo

./hollow-cli produce -data=fixtures/test_data.json -store=memory -verbose < /tmp/hollow_demo_commands.txt

# Clean up
rm -f /tmp/hollow_demo_commands.txt

echo
echo "=== Demo Complete! ==="
echo
echo "✅ Key Features Demonstrated:"
echo "  • Producer-Consumer workflow with multiple versions"
echo "  • Delta blob creation and management"
echo "  • Snapshot vs Delta blob patterns (snapshots every 5 versions)"
echo "  • Data inspection across versions"
echo "  • Version comparison and diff analysis"
echo "  • Zero-copy integration with graceful fallback"
echo "  • Real data evolution (add, update, delete, mixed operations)"
echo
echo "🎯 Try it yourself:"
echo "  ./hollow-cli help"
echo "  ./hollow-cli schema -data=fixtures/schema/test_schema.capnp -verbose"
echo "  ./hollow-cli produce -data=fixtures/test_data.json -store=memory"
echo
echo "📖 For more information, see the README.md CLI section"
