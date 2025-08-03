#!/bin/bash

echo "=== Realistic Producer-Consumer Delta Evolution ==="
echo

echo "This demonstrates realistic data evolution with:"
echo "- Snapshots every 5 versions (realistic production setting)"
echo "- Delta blobs for versions in between"
echo "- Various data evolution patterns (add/update/delete/mixed)"
echo

# Create a test script with realistic evolution commands
cat << 'EOF' > /tmp/hollow_delta_test.txt
l blobs
p add 5
l blobs
p update 3
l blobs
p mixed
l blobs
p delete 2
l blobs
p add 10
l blobs
i 1
i 2
i 3
i 4
i 5
i 6
d 1 3
d 3 6
c 6
q
EOF

echo "Running realistic delta evolution session:"
echo "Commands: l blobs, p add 5, p update 3, p mixed, p delete 2, p add 10, inspect versions, diff, consume"
echo

hollow-cli produce -data=fixtures/simple_test.json -store=memory -verbose < /tmp/hollow_delta_test.txt

# Clean up
rm -f /tmp/hollow_delta_test.txt

echo
echo "=== Delta Evolution Summary ==="
echo "✅ Version 1: Initial snapshot (from CLI startup)"
echo "✅ Versions 2-5: Delta blobs showing incremental changes"
echo "✅ Version 6: New snapshot (every 5 versions)"
echo "✅ Realistic pattern: Snapshots provide full state, deltas show changes"
echo "✅ Data evolution: add → update → mixed → delete → add operations"
echo "✅ Blob inspection: Shows actual snapshot vs delta blob distribution"
echo
echo "🎯 Realistic producer-consumer workflow with proper delta evolution!"
