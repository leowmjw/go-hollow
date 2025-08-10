# Fixtures Directory

This directory contains test data and comprehensive demonstration scripts for the go-hollow CLI.

## Test Data Files

### Core Test Data
- `test_data.json` - Sample JSON data for CLI demonstrations
- `simple_test.json` - Simple test data for basic operations
- `realistic_dataset.json` - **NEW** Comprehensive dataset with users, products, orders, metrics
- `person_data.json` / `person_data_v2.json` - Person data for schema testing

### CSV Datasets
- `customers.csv` - Customer data for CSV processing examples
- `devices.csv` - Device/IoT data
- `metrics.csv` - Performance metrics data
- `movies.csv` - Movie catalog data
- `orders.csv` - Order processing data
- `ratings.csv` - Rating/review data

## Schema Files

- `schema/test_schema.capnp` - Cap'n Proto schema for validation testing
- `schema/invalid_schema.txt` - Invalid schema for error testing

## Demo Scripts

### Core Testing Scripts
#### `demo_cli.sh` 🚀
**Primary demo script** - Showcases all major CLI features:
- Producer-consumer workflow with delta evolution
- Data inspection and version comparison  
- Zero-copy integration with graceful fallback
- Realistic data operations (add, update, delete, mixed)

#### `test_cli_features.sh` 🧪
**Automated test suite** - Validates CLI functionality:
- Help command functionality
- Schema validation (normal and verbose)
- Producer operations with data files
- Interactive command processing

### **NEW** Enhanced Demo Scripts

#### `ecommerce_demo.sh` 🏪
**Realistic E-Commerce Scenarios**:
- User management workflows
- Product catalog updates
- Order processing simulation
- Inventory tracking
- Bulk operations testing

#### `performance_benchmark.sh` ⚡
**Performance Analysis Suite**:
- Throughput measurement across different operation types
- Delta efficiency analysis
- Memory usage pattern testing
- Operation latency benchmarking
- Scaling characteristics validation

#### `error_recovery_demo.sh` 🛡️
**Error Handling & Recovery Testing**:
- Invalid schema and data file handling
- Malformed JSON processing
- Interactive command error recovery
- Version boundary testing
- Resource stress testing
- Graceful degradation validation

#### `serialization_comparison.sh` 🔄
**Serialization Mode Analysis**:
- Traditional vs CapnProto comparison
- Zero-copy performance characteristics
- Memory efficiency patterns
- Production trade-off analysis
- Detailed serialization metrics

#### `comprehensive_demo.sh` 🎯
**Master Demo Script** - Interactive menu system:
- Guided selection of demo scenarios
- Sequential execution of test suites
- Comprehensive feature validation
- Production readiness assessment

## Key Features Demonstrated

### ✅ **Recently Fixed Issues**
- **Delta Serialization Bug**: Non-PK types now generate proper delta blobs (not 0 bytes)
- **Zero-Copy Integration**: Graceful fallback when zero-copy unavailable
- **Error Handling**: Robust error recovery and continuation
- **CLI Feedback**: Meaningful delta blob size reporting

### 🎯 **Production Readiness Validation**
- Realistic data management scenarios
- Performance characteristics under load
- Error boundary testing and recovery
- Serialization mode selection guidance
- Resource usage analysis

### 📊 **Comprehensive Metrics**
- Delta blob size tracking and efficiency
- Operation throughput measurement
- Memory usage pattern analysis
- Zero-copy success rate monitoring
- Error recovery validation

## Usage

### Quick Start - Interactive Demo Menu
```bash
./fixtures/comprehensive_demo.sh
```

### Individual Demo Categories
```bash
# 1. Core functionality (recommended first run)
./fixtures/test_cli_features.sh
./fixtures/demo_cli.sh

# 2. Realistic scenarios
./fixtures/ecommerce_demo.sh

# 3. Performance analysis
./fixtures/performance_benchmark.sh

# 4. Error handling validation
./fixtures/error_recovery_demo.sh

# 5. Serialization comparison
./fixtures/serialization_comparison.sh
```

### Direct CLI Usage
```bash
# Schema validation
./hollow-cli schema -data=fixtures/schema/test_schema.capnp -verbose

# Interactive exploration
./hollow-cli produce -data=fixtures/test_data.json -store=memory

# Realistic dataset testing
./hollow-cli produce -data=fixtures/realistic_dataset.json -store=memory
```

## Demo Categories & Validation Matrix

| **Category** | **Script** | **Focus** | **Validates** |
|--------------|------------|-----------|----------------|
| **Core Testing** | `test_cli_features.sh` | Basic functionality | CLI commands, error handling |
| **Interactive Demo** | `demo_cli.sh` | Feature showcase | Producer-consumer workflow |
| **E-Commerce** | `ecommerce_demo.sh` | Realistic scenarios | Data management workflows |
| **Performance** | `performance_benchmark.sh` | Throughput analysis | Scaling and efficiency |
| **Error Handling** | `error_recovery_demo.sh` | Robustness testing | Recovery mechanisms |
| **Serialization** | `serialization_comparison.sh` | Mode comparison | Performance trade-offs |
| **Comprehensive** | `comprehensive_demo.sh` | Full validation | Production readiness |

## Removed Scripts (Historical)

The following redundant scripts were consolidated during cleanup:
- **Update Testing Scripts (5)**: `test_update_behavior.sh`, `prove_update_identity.sh`, etc.
- **Workflow Testing Scripts (3)**: `test_producer_consumer_workflow.sh`, etc.
- **Zero-Copy Testing Scripts (1)**: `test_zero_copy_interaction.sh`

All functionality preserved in the enhanced demo scripts above.

## 🎉 Getting Started

1. **First Time**: Run `./fixtures/comprehensive_demo.sh` and select option 1 (Basic CLI Features)
2. **Explore Features**: Try option 6 (Comprehensive Suite) for full validation
3. **Production Planning**: Focus on performance and error handling demos
4. **Development**: Use individual scripts for specific feature testing

The demo suite validates that the recent **delta serialization bug fix** is working correctly - you should now see meaningful delta blob sizes instead of 0 bytes!
