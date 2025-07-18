# AGENTS.md - Hollow-Go Development Guide

This document contains essential learnings and patterns for agents working on the Hollow-Go project.

## Project Overview

Hollow-Go is a Go implementation of the core Hollow producer/consumer libraries, providing ultra-fast, read-only in-memory datasets synchronized by snapshots and deltas. It follows the original Java Hollow architecture but leverages Go's concurrency primitives and standard library.

## Architecture & Design Patterns

### Core Components
- **Producer**: Thread-safe data ingestion with versioned snapshots
- **Consumer**: Lock-free data consumption with atomic state swaps
- **WriteState/ReadState**: Safe data access interfaces during operations
- **BlobStager/BlobRetriever**: Pluggable storage abstraction
- **Metrics**: Pluggable metrics collection system
- **Diff Engine**: Snapshot/delta comparison with idempotence guarantees

### Key Design Principles
1. **Interface-first design**: All major components are interfaces with concrete implementations in `internal/`
2. **Composition over inheritance**: Leverage Go's embedding and composition
3. **Thread safety**: Producer uses `sync.RWMutex`, Consumer uses `atomic.Value`
4. **Zero dependencies**: Only Go standard library (except optional cloud adapters)
5. **Functional options pattern**: Clean API configuration with `WithXXX()` functions

## Project Structure

```
.
├── go.mod                     # Module definition with Go 1.24.5
├── README.md                  # User documentation
├── AGENTS.md                  # This file - agent guidance
├── PRD.md                     # Product Requirements Document
├── hollow.go                  # Core interfaces and types
├── producer.go                # Producer implementation
├── consumer.go                # Consumer implementation
├── diff.go                    # Diff engine implementation
├── *_test.go                  # Test files (unit, integration, fuzz)
├── cmd/                       # Comprehensive examples
│   ├── demo/                  # Complete demo (10k events, normal dist)
│   ├── benchmark/             # Performance benchmarking
│   ├── stress/                # Stress testing
│   ├── metrics/               # Metrics showcase
│   ├── ecommerce/             # E-commerce simulation
│   ├── performance/           # Performance testing
│   ├── comprehensive/         # Full-featured demo
│   └── example/               # Simple example
└── internal/
    └── memblob/
        ├── memblob.go         # In-memory blob store implementation
        └── memblob_test.go    # Blob store tests
```

## Implementation Learnings

### 1. Data Serialization Challenge
**Problem**: Go's `json.Marshal` doesn't support `map[interface{}]interface{}`
**Solution**: Use `map[string]any` for data storage and convert keys using `fmt.Sprintf("%v", key)`
**Pattern**: Always prefer JSON-serializable types for data interchange

### 2. Atomic State Management
**Problem**: Consumer needs lock-free atomic updates
**Solution**: Use `atomic.Value` to store `ReadState` implementations
**Pattern**: Store concrete types implementing interfaces in `atomic.Value`

### 3. Thread Safety Patterns
- **Producer**: `sync.RWMutex` for write operations (staging, committing)
- **Consumer**: `atomic.Value` for state swaps, `sync.RWMutex` for individual ReadState access
- **Blob Store**: `sync.RWMutex` for concurrent access to internal maps

### 4. Functional Options Pattern
```go
// Good pattern for configuration
func WithBlobStager(stager BlobStager) ProducerOpt {
    return func(p *Producer) {
        p.stager = stager
    }
}

producer := NewProducer(
    WithBlobStager(blob),
    WithLogger(logger),
    WithMetricsCollector(collector),
)
```

### 5. Interface Design
- Keep interfaces small and focused (single responsibility)
- Use `context.Context` for cancellation and timeouts
- Return errors for all operations that can fail
- Prefer `any` over `interface{}` for Go 1.18+

## Testing Strategy

### Test Organization
- **Unit tests**: Same package as implementation (`package hollow`)
- **Integration tests**: Full producer/consumer workflows
- **Fuzz tests**: Diff engine correctness and idempotence
- **Internal tests**: Separate packages for internal components

### Testing Patterns
```go
// Table-driven tests for multiple scenarios
func TestProducerCycles(t *testing.T) {
    tests := []struct {
        name string
        data []any
        want int
    }{
        {"single item", []any{"item1"}, 1},
        {"multiple items", []any{"item1", "item2"}, 2},
    }
    // ... test implementation
}

// Fuzz tests for property verification
func FuzzDiffIsIdempotent(f *testing.F) {
    f.Fuzz(func(t *testing.T, b []byte) {
        diff := Diff(b, b)
        if !diff.IsEmpty() {
            t.Fatalf("expected empty diff for identical data")
        }
    })
}
```

### Key Testing Commands
```bash
# Run all tests
go test -v ./...

# Run specific test suites
go test -v ./producer_test.go
go test -v ./consumer_test.go
go test -v ./integration_test.go
go test -v ./internal/memblob/...

# Run fuzz tests
go test -fuzz=FuzzDiffIsIdempotent
go test -fuzz=FuzzDiffDetectsChanges

# Build verification
go build ./...

# Run comprehensive examples
go run ./cmd/demo           # Complete demo with 10k events
go run ./cmd/benchmark      # Performance benchmarking
go run ./cmd/stress         # Stress testing
go run ./cmd/metrics        # Metrics showcase
go run ./cmd/ecommerce      # E-commerce simulation
go run ./cmd/performance    # Performance testing
go run ./cmd/comprehensive  # Full-featured demo
```

## Performance Considerations

### Memory Management
- **Zero-copy updates**: Use `atomic.Value` swaps instead of data copying
- **Data isolation**: Always copy data in/out of blob store to prevent mutations
- **String keys**: Use string keys for JSON serialization efficiency

### Concurrency
- **Lock-free reads**: Consumer reads don't block each other
- **Write serialization**: Producer writes are serialized but don't block reads
- **Atomic operations**: Version counters use `atomic.Uint64`

### Optimization Patterns
```go
// Good: Atomic pointer swap
c.state.Store(newReadState)

// Good: Copy data to prevent external mutations
result := make([]byte, len(data))
copy(result, data)
return result

// Good: Use RWMutex for read-heavy workloads
func (rs *readState) Get(key any) (any, bool) {
    rs.mu.RLock()
    defer rs.mu.RUnlock()
    // ... read operation
}
```

## Common Patterns & Anti-Patterns

### ✅ Good Patterns
```go
// Functional options for configuration
func WithLogger(logger *slog.Logger) ProducerOpt

// Interface-based design
type BlobStager interface {
    Stage(ctx context.Context, version uint64, data []byte) error
    Commit(ctx context.Context, version uint64) error
}

// Atomic state management
state := atomic.Value{}
state.Store(newReadState)

// Proper error handling
if err := producer.RunCycle(fn); err != nil {
    return fmt.Errorf("cycle failed: %w", err)
}
```

### ❌ Anti-Patterns
```go
// Don't use interface{} in modern Go
func badFunc(data interface{}) // Use 'any' instead

// Don't use map[interface{}]interface{} with JSON
data := make(map[interface{}]interface{}) // Use map[string]any

// Don't block reads during writes
func (p *Producer) Read() { // This would block unnecessarily
    p.mu.Lock() // Should use RLock for reads
    defer p.mu.Unlock()
}
```

## Development Workflow

### Adding New Features
1. **Design interfaces first**: Define clean, focused interfaces
2. **Implement with tests**: TDD approach with comprehensive coverage
3. **Consider concurrency**: Thread safety from the start
4. **Add integration tests**: End-to-end workflow verification
5. **Update documentation**: Keep README and examples current

### Debugging Tips
- Use `go test -v` for detailed test output
- Check for race conditions with `go test -race`
- Use `go run -race ./cmd/example` for race detection in examples
- Monitor memory usage with `go tool pprof`

## Future Development Guidelines

### Planned Milestones
- **M3**: S3 blob adapter (implement BlobStager/BlobRetriever for S3)
- **M4**: Advanced metrics & slog hooks
- **M5**: Enhanced diff algorithms
- **M6**: Code generation for strongly-typed accessors

### Extension Points
- **Storage backends**: Implement BlobStager/BlobRetriever interfaces
- **Serialization formats**: Replace JSON with binary formats
- **Metrics collectors**: Implement custom MetricsCollector functions
- **Logging**: Replace slog with custom logging interfaces

### API Stability
- **Core interfaces**: Stable, avoid breaking changes
- **Internal packages**: Can change without notice
- **Functional options**: Extensible, prefer adding new options over changing existing ones

## Go-Specific Considerations

### Version Requirements
- **Go 1.24.5**: Uses modern Go features like `any` type and improved atomic operations
- **No external dependencies**: Leverage rich standard library
- **Module structure**: Clean module organization with internal packages

### Idiomatic Go
- Use `context.Context` for cancellation
- Prefer composition over inheritance
- Return errors, don't panic
- Use interfaces for testing and extensibility
- Follow Go naming conventions

## Performance Targets

- **Read latency**: ≤ 5 µs p99 for primitive lookups
- **Memory efficiency**: Atomic pointer swaps for zero-copy updates
- **Concurrency**: Lock-free reads, minimal lock contention
- **Throughput**: Support high-frequency producer cycles

## Troubleshooting Common Issues

### JSON Serialization Errors
```
Error: json: unsupported type: map[interface {}]interface {}
Solution: Use map[string]any instead of map[any]any
```

### Race Conditions
```
Error: race detected during execution
Solution: Use atomic operations or proper mutex patterns
```

### Build Failures
```
Error: found packages hollow and main in same directory
Solution: Move main.go to cmd/example/main.go
```

## Testing Checklist

Before committing changes:
- [ ] All tests pass: `go test -v ./...`
- [ ] No race conditions: `go test -race ./...`
- [ ] Builds successfully: `go build ./...`
- [ ] Example runs: `go run ./cmd/example`
- [ ] Fuzz tests pass: `go test -fuzz=FuzzDiffIsIdempotent`
- [ ] Documentation updated: README.md reflects changes

## Key Metrics & Monitoring

Track these metrics in production:
- **Version progression**: Monitor version increments
- **Record counts**: Track dataset sizes
- **Byte sizes**: Monitor memory usage
- **Cycle duration**: Producer cycle performance
- **Refresh latency**: Consumer refresh times

## Example Patterns

The project includes comprehensive examples demonstrating different aspects of Hollow-Go:

### Normal Distribution Event Generation
All examples use a normal distribution curve to generate 10,000 events over 10 minutes, simulating realistic load patterns:

```go
// Generate normal distribution for event timing
func generateNormalDistribution(mean, stdDev float64) float64 {
    // Box-Muller transform
    u1, u2 := rand.Float64(), rand.Float64()
    z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
    return mean + stdDev*z
}

// Schedule events with peak activity in the middle
mean := duration.Seconds() / 2.0      // Peak at 5 minutes
stdDev := duration.Seconds() / 6.0    // 99.7% within 10 minutes
```

### Example Categories
- **`cmd/demo/`**: Complete functional demonstration
- **`cmd/benchmark/`**: Performance measurement and analysis
- **`cmd/stress/`**: Concurrency and reliability testing
- **`cmd/metrics/`**: Advanced metrics collection
- **`cmd/ecommerce/`**: Real-world business simulation
- **`cmd/performance/`**: Ultra-fast read latency testing
- **`cmd/comprehensive/`**: All features combined

### Common Example Patterns
```go
// Event generation with realistic distribution
type Event struct {
    ID        int                    `json:"id"`
    Type      string                 `json:"type"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
}

// Metrics collection with comprehensive tracking
type Metrics struct {
    TotalEvents      int64
    TotalCycles      int64
    CycleLatencies   []time.Duration
    ReadLatencies    []time.Duration
    EventTypeCounts  map[string]int64
}

// Concurrent producer/consumer patterns
var wg sync.WaitGroup
ctx, cancel := context.WithTimeout(context.Background(), duration)
defer cancel()

// Multiple goroutines for producers, consumers, monitors
for i := 0; i < numProducers; i++ {
    wg.Add(1)
    go runProducer(ctx, &wg, i)
}
```

### Performance Testing Patterns
- **Target**: ≤5µs p99 read latency
- **Measurement**: Percentile analysis with sorted latencies
- **Validation**: Automated pass/fail criteria
- **Reporting**: Comprehensive performance reports

This document should be updated as the project evolves to capture new learnings and patterns.
