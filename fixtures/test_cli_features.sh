#!/bin/bash

echo "=== Go-Hollow CLI Feature Test Suite ==="
echo
echo "This script tests all major CLI features systematically"
echo

# Check prerequisites
if [ ! -f "hollow-cli" ]; then
    echo "❌ Error: hollow-cli binary not found in current directory"
    echo "Please run from the go-hollow root directory"
    exit 1
fi

PASS_COUNT=0
FAIL_COUNT=0

# Function to run test and check result
run_test() {
    local test_name="$1"
    local command="$2"
    local expected_pattern="$3"
    
    echo "🧪 Testing: $test_name"
    
    if eval "$command" | grep -q "$expected_pattern"; then
        echo "  ✅ PASS: $test_name"
        ((PASS_COUNT++))
    else
        echo "  ❌ FAIL: $test_name"
        ((FAIL_COUNT++))
    fi
    echo
}

# Test 1: Help command
run_test "Help command" \
    "./hollow-cli help" \
    "Hollow CLI Tool"

# Test 2: Schema validation
run_test "Schema validation" \
    "./hollow-cli schema -data=fixtures/schema/test_schema.capnp" \
    "Schema validation passed"

# Test 3: Producer with data file
echo "🧪 Testing: Producer with data file"
echo "q" | ./hollow-cli produce -data=fixtures/test_data.json -store=memory > /tmp/producer_test.out 2>&1
if grep -q "Version: 1" /tmp/producer_test.out; then
    echo "  ✅ PASS: Producer with data file"
    ((PASS_COUNT++))
else
    echo "  ❌ FAIL: Producer with data file"
    ((FAIL_COUNT++))
fi
rm -f /tmp/producer_test.out
echo

# Test 4: Interactive commands
echo "🧪 Testing: Interactive commands"
cat << 'EOF' > /tmp/test_commands.txt
l blobs
i 1
p add 2
c 2
q
EOF

./hollow-cli produce -data=fixtures/test_data.json -store=memory < /tmp/test_commands.txt > /tmp/interactive_test.out 2>&1

if grep -q "Total versions" /tmp/interactive_test.out && \
   grep -q "Snapshot blob" /tmp/interactive_test.out && \
   grep -q "New version produced" /tmp/interactive_test.out; then
    echo "  ✅ PASS: Interactive commands"
    ((PASS_COUNT++))
else
    echo "  ❌ FAIL: Interactive commands"
    ((FAIL_COUNT++))
fi

rm -f /tmp/test_commands.txt /tmp/interactive_test.out
echo

# Test 5: Schema validation with verbose output
run_test "Schema validation (verbose)" \
    "./hollow-cli schema -data=fixtures/schema/test_schema.capnp -verbose" \
    "Schema: Person"

echo "=== Test Results ==="
echo "✅ Passed: $PASS_COUNT"
echo "❌ Failed: $FAIL_COUNT"
echo "📊 Total:  $((PASS_COUNT + FAIL_COUNT))"
echo

if [ $FAIL_COUNT -eq 0 ]; then
    echo "🎉 All tests passed! The CLI is working correctly."
    exit 0
else
    echo "⚠️  Some tests failed. Please check the CLI functionality."
    exit 1
fi
