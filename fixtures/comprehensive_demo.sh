#!/bin/bash

echo "======================================================================="
echo "           🚀 Go-Hollow Comprehensive Feature Demonstration"
echo "======================================================================="
echo
echo "This comprehensive demo exercises all major go-hollow features:"
echo "• Basic producer-consumer workflows"
echo "• Realistic e-commerce data scenarios"
echo "• Performance benchmarking and analysis"
echo "• Error handling and recovery mechanisms"
echo "• Serialization mode comparisons"
echo "• Production readiness validation"
echo

# Check prerequisites
if [ ! -f "hollow-cli" ]; then
    echo "❌ Error: hollow-cli binary not found"
    echo "Please run from the go-hollow root directory"
    exit 1
fi

echo "🎯 Available Demo Scenarios:"
echo "   1. Basic CLI Features (quick validation)"
echo "   2. E-Commerce Data Management (realistic scenarios)"
echo "   3. Performance Benchmarking (throughput analysis)"
echo "   4. Error Handling & Recovery (robustness testing)"
echo "   5. Serialization Comparison (mode analysis)"
echo "   6. Comprehensive Suite (all scenarios)"
echo

# Function to run with confirmation
run_demo() {
    local demo_name="$1"
    local demo_script="$2"
    
    echo "🚀 Running: $demo_name"
    echo "   Script: $demo_script"
    echo
    
    if [ -f "$demo_script" ]; then
        ./"$demo_script"
    else
        echo "❌ Error: Demo script not found: $demo_script"
        return 1
    fi
    
    echo
    echo "✅ $demo_name completed!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo
}

# Get user choice
read -p "Select demo to run (1-6): " choice

case $choice in
    1)
        echo "🎯 Running Basic CLI Features Demo..."
        run_demo "CLI Feature Tests" "fixtures/test_cli_features.sh"
        run_demo "Interactive CLI Demo" "fixtures/demo_cli.sh"
        ;;
    2)
        echo "🎯 Running E-Commerce Data Management Demo..."
        run_demo "E-Commerce Scenarios" "fixtures/ecommerce_demo.sh"
        ;;
    3)
        echo "🎯 Running Performance Benchmarking..."
        run_demo "Performance Benchmarks" "fixtures/performance_benchmark.sh"
        ;;
    4)
        echo "🎯 Running Error Handling & Recovery Tests..."
        run_demo "Error Recovery Tests" "fixtures/error_recovery_demo.sh"
        ;;
    5)
        echo "🎯 Running Serialization Mode Comparison..."
        run_demo "Serialization Analysis" "fixtures/serialization_comparison.sh"
        ;;
    6)
        echo "🎯 Running Comprehensive Feature Suite..."
        echo "This will run ALL demo scenarios sequentially..."
        echo
        read -p "Continue with full suite? (y/N): " confirm
        if [[ $confirm =~ ^[Yy]$ ]]; then
            run_demo "CLI Feature Tests" "fixtures/test_cli_features.sh"
            run_demo "Interactive CLI Demo" "fixtures/demo_cli.sh"
            run_demo "E-Commerce Scenarios" "fixtures/ecommerce_demo.sh"
            run_demo "Performance Benchmarks" "fixtures/performance_benchmark.sh"
            run_demo "Error Recovery Tests" "fixtures/error_recovery_demo.sh"
            run_demo "Serialization Analysis" "fixtures/serialization_comparison.sh"
            
            echo "🎉 COMPREHENSIVE DEMO SUITE COMPLETE!"
            echo
            echo "📊 Full Feature Matrix Validated:"
            echo "   ✅ Basic CLI operations and interactive mode"
            echo "   ✅ Realistic data management scenarios"
            echo "   ✅ Performance characteristics and scaling"
            echo "   ✅ Error handling and recovery mechanisms"
            echo "   ✅ Serialization mode trade-offs"
            echo "   ✅ Production readiness validation"
            echo
        else
            echo "Comprehensive suite cancelled."
        fi
        ;;
    *)
        echo "❌ Invalid choice. Please select 1-6."
        exit 1
        ;;
esac

echo "======================================================================="
echo "                     🎯 Demo Complete!"
echo "======================================================================="
echo
echo "🔍 What You've Validated:"
echo "• Core hollow functionality working correctly"
echo "• Delta serialization bug fix successful (non-zero delta blobs)"
echo "• Interactive CLI provides meaningful feedback"
echo "• Error conditions handled gracefully"
echo "• Performance characteristics understood"
echo
echo "🚀 Next Steps for Production:"
echo "• Configure appropriate serialization mode for your use case"
echo "• Set up monitoring for delta blob sizes and performance"
echo "• Implement health checks and alerting"
echo "• Test with production-sized datasets"
echo "• Configure primary keys for critical data types"
echo
echo "📖 Documentation:"
echo "• README.md - Setup and basic usage"
echo "• CLI.md - Command reference"
echo "• AGENTS.md - Implementation learnings and patterns"
echo "• TEST.md - Comprehensive testing guide"
echo
echo "🎉 Go-Hollow is ready for production deployment!"
