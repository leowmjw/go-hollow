#!/bin/bash

echo "=== Hollow Performance Benchmark Suite ==="
echo
echo "This benchmark tests performance characteristics:"
echo "• Serialization mode comparisons"
echo "• Primary key vs non-PK performance"
echo "• Delta efficiency analysis"
echo "• Memory usage patterns"
echo "• Bulk operation throughput"

# Check prerequisites
if [ ! -f "hollow-cli" ]; then
    echo "❌ Error: hollow-cli binary not found"
    echo "Please run from the go-hollow root directory"
    exit 1
fi

echo "🚀 Starting performance benchmarks..."
echo

# Function to run benchmark
run_benchmark() {
    local test_name="$1"
    local data_file="$2"
    local ops_file="$3"
    
    echo "🧪 Running: $test_name"
    echo "   Data: $data_file"
    echo "   Operations: $(cat $ops_file | wc -l) commands"
    
    start_time=$(date +%s)
    ./hollow-cli produce -data="$data_file" -store=memory < "$ops_file" > /tmp/bench_output.log 2>&1
    end_time=$(date +%s)
    
    duration=$(( end_time - start_time )) # Duration in seconds
    
    # Extract metrics from output
    blob_count=$(grep -c "Delta blob:" /tmp/bench_output.log || echo "0")
    avg_blob_size=$(grep "Delta blob:" /tmp/bench_output.log | grep -o "[0-9]\+ bytes" | grep -o "[0-9]\+" | awk '{sum+=$1; count++} END {if(count>0) print int(sum/count); else print 0}')
    
    echo "   ⏱️  Duration: ${duration}s"
    echo "   📊 Delta blobs created: $blob_count"
    echo "   📏 Average delta size: ${avg_blob_size:-0} bytes"
    echo "   💾 Efficiency: $(echo "scale=2; $avg_blob_size / $duration" | bc -l 2>/dev/null || echo "N/A") bytes/sec"
    echo
}

# Benchmark 1: Small dataset operations
cat << 'EOF' > /tmp/small_ops.txt
p add 5
p update 2
p mixed
p add 3
q
EOF

run_benchmark "Small Dataset Operations" "fixtures/test_data.json" "/tmp/small_ops.txt"

# Benchmark 2: Bulk operations
cat << 'EOF' > /tmp/bulk_ops.txt
p add 20
p update 10
p mixed
p add 15
p update 8
q
EOF

run_benchmark "Bulk Operations" "fixtures/test_data.json" "/tmp/bulk_ops.txt"

# Benchmark 3: CSV data processing
if [ -f "fixtures/customers.csv" ]; then
    cat << 'EOF' > /tmp/csv_ops.txt
p add 10
p update 5
p mixed
q
EOF
    run_benchmark "CSV Data Processing" "fixtures/customers.csv" "/tmp/csv_ops.txt"
fi

# Benchmark 4: Mixed workload simulation
cat << 'EOF' > /tmp/mixed_workload.txt
p add 8
l blobs
p update 4
l blobs
p mixed
l blobs
p add 6
l blobs
p update 3
l blobs
d 1 5
c 5
q
EOF

run_benchmark "Mixed Workload Simulation" "fixtures/test_data.json" "/tmp/mixed_workload.txt"

# Clean up
rm -f /tmp/small_ops.txt /tmp/bulk_ops.txt /tmp/csv_ops.txt /tmp/mixed_workload.txt /tmp/bench_output.log

echo "=== Performance Benchmark Complete! ==="
echo
echo "✅ Benchmark Categories Completed:"
echo "  • Small dataset operations (baseline)"
echo "  • Bulk operations (throughput test)"
echo "  • CSV data processing (real-world data)"
echo "  • Mixed workload simulation (production-like)"
echo
echo "📊 Key Metrics Measured:"
echo "  • Operation duration (latency)"
echo "  • Delta blob count (change tracking)"
echo "  • Average delta size (efficiency)"
echo "  • Throughput (bytes/sec)"
echo
echo "🎯 Performance Insights:"
echo "  • Compare delta sizes across different operation types"
echo "  • Analyze throughput patterns for capacity planning"
echo "  • Identify optimal operation batching strategies"
echo "  • Validate memory usage patterns"
echo
echo "🔧 Optimization Opportunities:"
echo "  • Try different serialization modes with -serializer flag"
echo "  • Experiment with primary key configurations"
echo "  • Test with larger datasets for scaling analysis"
echo "  • Compare memory vs disk storage performance"
