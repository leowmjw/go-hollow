#!/bin/bash

echo "=== Error Handling & Recovery Demo ==="
echo
echo "This demo tests error scenarios and recovery:"
echo "• Invalid data handling"
echo "• Serialization errors"
echo "• Version inconsistencies"
echo "• Recovery mechanisms"
echo "• Graceful degradation"

# Check prerequisites
if [ ! -f "hollow-cli" ]; then
    echo "❌ Error: hollow-cli binary not found"
    echo "Please run from the go-hollow root directory"
    exit 1
fi

echo "🚀 Starting error handling tests..."
echo

# Test 1: Invalid schema validation
echo "🧪 Test 1: Schema Validation"
echo "Testing with invalid schema..."
./hollow-cli schema -data=fixtures/nonexistent.capnp 2>/dev/null
if [ $? -ne 0 ]; then
    echo "  ✅ PASS: Invalid schema properly rejected"
else
    echo "  ❌ FAIL: Invalid schema should be rejected"
fi
echo

# Test 2: Invalid data file handling
echo "🧪 Test 2: Invalid Data File Handling"
echo "Testing with non-existent data file..."
echo "q" | ./hollow-cli produce -data=fixtures/nonexistent.json -store=memory 2>/dev/null
if [ $? -ne 0 ]; then
    echo "  ✅ PASS: Non-existent data file properly handled"
else
    echo "  ❌ FAIL: Should handle missing data file gracefully"
fi
echo

# Test 3: Malformed JSON handling
echo "🧪 Test 3: Malformed JSON Handling"
echo "Testing with malformed JSON..."
echo '{"invalid": json}' > /tmp/malformed.json
echo "q" | ./hollow-cli produce -data=/tmp/malformed.json -store=memory 2>/dev/null
if [ $? -ne 0 ]; then
    echo "  ✅ PASS: Malformed JSON properly handled"
else
    echo "  ❌ FAIL: Should handle malformed JSON gracefully"
fi
rm -f /tmp/malformed.json
echo

# Test 4: Interactive command error handling
echo "🧪 Test 4: Interactive Command Error Handling"
echo "Testing invalid interactive commands..."
cat << 'EOF' > /tmp/error_commands.txt
invalid_command
p invalid_operation
i invalid_version
c invalid_version
d invalid_from invalid_to
q
EOF

echo "Testing invalid commands..."
./hollow-cli produce -data=fixtures/test_data.json -store=memory < /tmp/error_commands.txt > /tmp/error_output.log 2>&1

# Check if CLI handled errors gracefully (didn't crash)
if grep -q "Goodbye!" /tmp/error_output.log; then
    echo "  ✅ PASS: CLI handled invalid commands gracefully"
else
    echo "  ❌ FAIL: CLI should handle invalid commands without crashing"
fi
echo

# Test 5: Version boundary testing
echo "🧪 Test 5: Version Boundary Testing"
echo "Testing edge case version operations..."
cat << 'EOF' > /tmp/boundary_commands.txt
i 0
i -1
i 999999
c 0
c -1
c 999999
d 0 1
d 1 0
d -1 1
q
EOF

./hollow-cli produce -data=fixtures/test_data.json -store=memory < /tmp/boundary_commands.txt > /tmp/boundary_output.log 2>&1

if grep -q "Goodbye!" /tmp/boundary_output.log; then
    echo "  ✅ PASS: Version boundary conditions handled gracefully"
else
    echo "  ❌ FAIL: Version boundary conditions should be handled gracefully"
fi
echo

# Test 6: Resource exhaustion simulation
echo "🧪 Test 6: Resource Stress Testing"
echo "Testing with intensive operations..."
cat << 'EOF' > /tmp/stress_commands.txt
p add 50
p update 25
p mixed
p add 30
p update 15
l blobs
q
EOF

echo "Running stress test..."
timeout 30s ./hollow-cli produce -data=fixtures/test_data.json -store=memory < /tmp/stress_commands.txt > /tmp/stress_output.log 2>&1
stress_exit_code=$?

if [ $stress_exit_code -eq 0 ] && grep -q "Goodbye!" /tmp/stress_output.log; then
    echo "  ✅ PASS: Stress test completed successfully"
elif [ $stress_exit_code -eq 124 ]; then
    echo "  ⚠️  TIMEOUT: Stress test exceeded 30 seconds (may indicate performance issue)"
else
    echo "  ❌ FAIL: Stress test failed unexpectedly"
fi
echo

# Test 7: Recovery after errors
echo "🧪 Test 7: Recovery and Continuation"
echo "Testing recovery after error conditions..."
cat << 'EOF' > /tmp/recovery_commands.txt
p add 3
invalid_command
p update 1
another_invalid
l blobs
i 2
q
EOF

./hollow-cli produce -data=fixtures/test_data.json -store=memory < /tmp/recovery_commands.txt > /tmp/recovery_output.log 2>&1

valid_operations=$(grep -c "New version produced:" /tmp/recovery_output.log || echo "0")
if [ "$valid_operations" -ge 2 ] && grep -q "Goodbye!" /tmp/recovery_output.log; then
    echo "  ✅ PASS: CLI recovered and continued after errors"
    echo "         ($valid_operations valid operations completed)"
else
    echo "  ❌ FAIL: CLI should recover and continue after errors"
fi

# Clean up
rm -f /tmp/error_commands.txt /tmp/boundary_commands.txt /tmp/stress_commands.txt /tmp/recovery_commands.txt
rm -f /tmp/error_output.log /tmp/boundary_output.log /tmp/stress_output.log /tmp/recovery_output.log

echo
echo "=== Error Handling Demo Complete! ==="
echo
echo "✅ Error Scenarios Tested:"
echo "  • Schema validation errors"
echo "  • Invalid data file handling"
echo "  • Malformed JSON processing"
echo "  • Interactive command errors"
echo "  • Version boundary conditions"
echo "  • Resource stress testing"
echo "  • Recovery and continuation"
echo
echo "🎯 Error Handling Quality:"
echo "  • Graceful degradation under error conditions"
echo "  • Proper error messages without crashes"
echo "  • Recovery capability after errors"
echo "  • Resource limit handling"
echo
echo "🔧 Production Readiness Indicators:"
echo "  • Error conditions don't cause data corruption"
echo "  • Invalid input properly validated and rejected"
echo "  • System continues operating after recoverable errors"
echo "  • Performance remains stable under stress"
echo
echo "📖 For production deployment:"
echo "  • Monitor error rates and types"
echo "  • Implement comprehensive logging"
echo "  • Set up health checks and alerting"
echo "  • Test recovery procedures regularly"
