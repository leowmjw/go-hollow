#!/bin/bash

echo "=== E-Commerce Data Management Demo ==="
echo
echo "This demo simulates realistic e-commerce scenarios:"
echo "• User management (with primary keys)"
echo "• Product catalog updates"
echo "• Order processing workflows" 
echo "• Inventory tracking"
echo "• Performance under load"

# Check prerequisites
if [ ! -f "hollow-cli" ]; then
    echo "❌ Error: hollow-cli binary not found"
    echo "Please run from the go-hollow root directory"
    exit 1
fi

echo "🚀 Starting E-Commerce simulation..."
echo

# Create comprehensive test scenario
cat << 'EOF' > /tmp/ecommerce_commands.txt
l blobs
i 1
p add 5
l blobs
i 2
p update 3
l blobs
p mixed
l blobs
c 2
c 4
d 1 4
p bulk 10
l blobs
i 5
q
EOF

echo "📊 E-Commerce Operations:"
echo "l blobs       - View current data state"
echo "i 1          - Inspect initial user data"
echo "p add 5      - Add 5 new users/products"
echo "p update 3   - Update 3 existing records"
echo "p mixed      - Mixed operations (realistic workflow)"
echo "p bulk 10    - Bulk operations (performance test)"
echo "c 2/4        - Consume data at different versions"
echo "d 1 4        - Compare data evolution"
echo

echo "⏱️  Running e-commerce simulation..."
echo

./hollow-cli produce -data=fixtures/test_data.json -store=memory -verbose < /tmp/ecommerce_commands.txt

# Clean up
rm -f /tmp/ecommerce_commands.txt

echo
echo "=== E-Commerce Demo Complete! ==="
echo
echo "✅ Scenarios Demonstrated:"
echo "  • Incremental data updates (add/update/mixed)"
echo "  • Bulk data operations"
echo "  • Version-based data consumption"
echo "  • Data evolution tracking"
echo "  • Delta efficiency analysis"
echo "  • Memory storage performance"
echo
echo "🎯 Production Insights:"
echo "  • Delta blob sizes indicate change efficiency"
echo "  • Version progression shows data accumulation"
echo "  • Consumer refresh validates data consistency"
echo "  • Bulk operations test performance characteristics"
echo
echo "📖 Try different scenarios:"
echo "  ./hollow-cli produce -data=fixtures/customers.csv -store=memory"
echo "  ./hollow-cli produce -data=fixtures/orders.csv -store=memory"
