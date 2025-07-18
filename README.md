# Hollow-Go

A Go implementation of the core Hollow producer/consumer libraries, providing ultra-fast, read-only in-memory datasets that are synchronised by snapshots and deltas.

## Features

- **Ultra-fast reads**: ≤ 5 µs p99 read for primitive lookups on 1M-record datasets
- **Zero external dependencies**: Uses only Go standard library with optional adapters for S3/GCS
- **Atomic updates**: Thread-safe producer/consumer pattern with double-snapshot atomic swaps
- **Pluggable storage**: In-memory blob store with extensible interface for cloud storage
- **Comprehensive metrics**: Built-in metrics collection with pluggable collectors
- **Diff engine**: Efficient snapshot/delta diffing with idempotence guarantees

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                Producer                                     │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌──────────────┐ │
│  │ WriteState  │───▶│ BlobStager  │───▶│ BlobStore   │    │ Metrics      │ │
│  │             │    │             │    │             │    │ Collector    │ │
│  └─────────────┘    └─────────────┘    └─────────────┘    └──────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                Consumer                                     │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌──────────────┐ │
│  │Announcement │───▶│BlobRetriever│───▶│ ReadState   │───▶│ Index APIs   │ │
│  │ Watcher     │    │             │    │             │    │              │ │
│  └─────────────┘    └─────────────┘    └─────────────┘    └──────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Quick Start

```go
package main

import (
    "log"
    "github.com/leow/go-raw-hollow"
    "github.com/leow/go-raw-hollow/internal/memblob"
)

func main() {
    // Create in-memory blob store
    blob := memblob.New()
    
    // Create producer
    producer := hollow.NewProducer(hollow.WithBlobStager(blob))
    
    // Run a production cycle
    version, err := producer.RunCycle(func(ws hollow.WriteState) error {
        if err := ws.Add("user1"); err != nil {
            return err
        }
        if err := ws.Add("user2"); err != nil {
            return err
        }
        return ws.Add("user3")
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Create consumer
    consumer := hollow.NewConsumer(
        hollow.WithBlobRetriever(blob),
        hollow.WithAnnouncementWatcher(func() (uint64, bool, error) {
            return version, true, nil
        }),
    )
    
    // Refresh consumer to latest version
    if err := consumer.Refresh(); err != nil {
        log.Fatal(err)
    }
    
    // Read data
    rs := consumer.ReadState()
    if val, ok := rs.Get("user1"); ok {
        log.Printf("Found user: %v", val)
    }
    
    log.Printf("Dataset size: %d records", rs.Size())
}
```

## Examples

The `cmd/` directory contains comprehensive examples demonstrating all Hollow-Go capabilities:

### 🚀 Complete Demo (`cmd/demo/`)
A comprehensive demonstration of all features with 10,000 events distributed using a normal distribution curve over 10 minutes.

```bash
go run ./cmd/demo
```

**Features demonstrated:**
- Normal distribution event scheduling
- Producer/consumer patterns
- Real-time metrics collection
- Performance analysis (5µs p99 read latency target)
- Diff engine functionality
- Comprehensive reporting

### 📊 Benchmark Suite (`cmd/benchmark/`)
Performance benchmarking with detailed metrics and distribution analysis.

```bash
go run ./cmd/benchmark
```

**Features:**
- 10k events with configurable distribution
- Multiple concurrent producers/consumers
- Latency percentile analysis
- Throughput measurement
- Memory usage tracking
- Event timeline visualization

### 🔥 Stress Testing (`cmd/stress/`)
High-load concurrent testing with error injection and performance monitoring.

```bash
go run ./cmd/stress
```

**Features:**
- Multiple concurrent producers/consumers
- Error injection and recovery
- Concurrency stress testing
- Performance degradation detection
- State consistency validation

### 📈 Metrics Showcase (`cmd/metrics/`)
Advanced metrics collection with custom collectors and business intelligence.

```bash
go run ./cmd/metrics
```

**Features:**
- Custom metrics collectors
- Business metrics tracking
- Performance trend analysis
- JSON export functionality
- Real-time dashboards

### 🛒 E-commerce Simulation (`cmd/ecommerce/`)
Real-world simulation of e-commerce events with business metrics.

```bash
go run ./cmd/ecommerce
```

**Features:**
- Realistic user behavior simulation
- Shopping cart and purchase flows
- Revenue and conversion tracking
- Session management
- Business intelligence reporting

### ⚡ Performance Testing (`cmd/performance/`)
Focused performance testing with system monitoring and latency analysis.

```bash
go run ./cmd/performance
```

**Features:**
- Ultra-fast read latency testing (≤5µs p99)
- Memory profiling and GC analysis
- CPU usage monitoring
- Throughput optimization
- Performance regression detection

### 🎯 Comprehensive Demo (`cmd/comprehensive/`)
Full-featured demonstration combining all capabilities in realistic scenarios.

```bash
go run ./cmd/comprehensive
```

**Features:**
- All Hollow-Go features in one demo
- Multiple test scenarios
- Real-time monitoring
- Performance assessment
- Complete reporting suite

## Running Examples

All examples support the normal distribution pattern for the 10,000 events over 10 minutes:

```bash
# Run individual examples
go run ./cmd/demo           # Complete demo
go run ./cmd/benchmark      # Performance benchmarks
go run ./cmd/stress         # Stress testing
go run ./cmd/metrics        # Metrics showcase
go run ./cmd/ecommerce      # E-commerce simulation
go run ./cmd/performance    # Performance testing
go run ./cmd/comprehensive  # Full demonstration

# Build all examples
go build ./cmd/...
```

## API Reference

### Producer

```go
// Create a producer with options
producer := hollow.NewProducer(
    hollow.WithBlobStager(stager),
    hollow.WithLogger(logger),
    hollow.WithMetricsCollector(collector),
)

// Run a production cycle
version, err := producer.RunCycle(func(ws hollow.WriteState) error {
    return ws.Add("data")
})
```

### Consumer

```go
// Create a consumer with options
consumer := hollow.NewConsumer(
    hollow.WithBlobRetriever(retriever),
    hollow.WithAnnouncementWatcher(watcher),
    hollow.WithConsumerLogger(logger),
)

// Refresh to latest version
err := consumer.Refresh()

// Get current version
version := consumer.CurrentVersion()

// Access read state
rs := consumer.ReadState()
value, exists := rs.Get("key")
size := rs.Size()
```

### Blob Storage

#### In-Memory Store

```go
import "github.com/leow/go-raw-hollow/internal/memblob"

store := memblob.New()
```

#### Custom Storage

Implement the `BlobStager` and `BlobRetriever` interfaces:

```go
type BlobStager interface {
    Stage(ctx context.Context, version uint64, data []byte) error
    Commit(ctx context.Context, version uint64) error
}

type BlobRetriever interface {
    Retrieve(ctx context.Context, version uint64) ([]byte, error)
    Latest(ctx context.Context) (uint64, error)
}
```

### Metrics

```go
type MetricsCollector func(m Metrics)

type Metrics struct {
    Version     uint64
    RecordCount int
    ByteSize    int64
}

producer := hollow.NewProducer(
    hollow.WithMetricsCollector(func(m hollow.Metrics) {
        log.Printf("Version %d: %d records, %d bytes", 
            m.Version, m.RecordCount, m.ByteSize)
    }),
)
```

### Diff Engine

```go
// Compare byte slices
diff := hollow.Diff(oldData, newData)
isEmpty := diff.IsEmpty()

// Compare data maps
diff := hollow.DiffData(oldMap, newMap)
diff.Apply(targetMap, sourceMap)
```

## Testing

Run all tests:
```bash
go test -v ./...
```

Run specific test suites:
```bash
go test -v ./...producer_test.go        # Producer tests
go test -v ./...consumer_test.go        # Consumer tests
go test -v ./...integration_test.go     # Integration tests
go test -v ./...diff_test.go            # Diff engine tests
go test -v ./internal/memblob/...       # Blob storage tests
```

Run fuzz tests:
```bash
go test -fuzz=FuzzDiffIsIdempotent
go test -fuzz=FuzzDiffDetectsChanges
```

## Performance

- **Read latency**: ≤ 5 µs p99 for primitive lookups
- **Memory efficiency**: Atomic pointer swaps for zero-copy updates
- **Concurrency**: Lock-free reads, write-locked updates
- **Serialization**: JSON-based with extensible formats

## Design Principles

1. **Interface-based design**: All components implement interfaces for testability and extensibility
2. **Composition over inheritance**: Concrete implementations live under `internal/`
3. **Thread safety**: Producer uses `sync.RWMutex`, consumer uses `atomic.Value`
4. **Zero dependencies**: Only Go standard library required
5. **Pluggable architecture**: Metrics, logging, and storage are all pluggable

## Comparison with Java Hollow

| Feature | Java Hollow | Hollow-Go |
|---------|-------------|-----------|
| Language | Java | Go |
| Dependencies | Many | Zero (stdlib only) |
| Memory Model | JVM GC | Go GC |
| Concurrency | Java threads | Goroutines |
| Serialization | Custom binary | JSON (extensible) |
| UI Tools | Full suite | Out of scope |

## Roadmap

- [x] M1: In-memory blob store + snapshot writer
- [x] M2: Consumer & refresh loop
- [ ] M3: S3 blob adapter
- [ ] M4: Advanced metrics & slog hooks
- [ ] M5: Diff/diagnostics enhancements
- [ ] M6: Code generation for strongly-typed accessors

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `go test -v ./...`
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
