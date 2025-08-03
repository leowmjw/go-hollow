# Zero-Copy Integration with go-hollow

This document outlines the zero-copy capabilities implemented in go-hollow using Cap'n Proto serialization.

## Overview

Zero-copy data access allows reading and processing data without copying it from its original location in memory. This provides significant benefits for:

- **Memory efficiency**: Multiple readers can share the same data
- **Performance**: Faster deserialization and reduced memory allocations
- **Scalability**: Lower memory pressure in high-throughput scenarios
- **Network efficiency**: Direct access to memory-mapped network buffers

## Implementation

### Zero-Copy Components

The zero-copy implementation includes both demonstration layers and **core system integration**:

#### **Core System Integration** (Production-Ready)
1. **Pluggable Serialization** ([`internal/serialization.go`](internal/serialization.go)): Support for Traditional, ZeroCopy, and Hybrid modes
2. **Enhanced Producer** ([`producer/producer.go`](producer/producer.go)): Integrated zero-copy blob serialization
3. **Zero-Copy Consumer** ([`consumer/zero_copy_consumer.go`](consumer/zero_copy_consumer.go)): Direct Cap'n Proto data access
4. **Zero-Copy Indexing** ([`index/zero_copy_index.go`](index/zero_copy_index.go)): Indexes built directly from buffers
5. **Integration Tests** ([`core_zero_copy_integration_test.go`](core_zero_copy_integration_test.go)): End-to-end validation

#### **Demonstration Layer** (Examples)
1. **Zero-Copy Reader** (`zero_copy/zero_copy.go`): Provides zero-copy access to Cap'n Proto data
2. **Zero-Copy Writer** (`zero_copy/zero_copy.go`): Efficient serialization using Cap'n Proto
3. **Zero-Copy Iterator** (`zero_copy/zero_copy.go`): Batch processing without data copying
4. **Zero-Copy Aggregator** (`zero_copy/zero_copy.go`): Statistical operations on large datasets

### Schema Definition

Cap'n Proto schemas are defined in `schemas/`:

```capnp
struct Movie {
  id @0 :UInt32;           # Unique identifier
  title @1 :Text;          # Movie title  
  year @2 :UInt16;         # Release year
  genres @3 :List(Text);   # Genre tags
  runtimeMin @4 :UInt16;   # Runtime in minutes
}
```

### Generated Code

Cap'n Proto generates Go bindings automatically:

```bash
./tools/gen-schema.sh go
```

This creates type-safe Go structs in `generated/go/` that support zero-copy access.

## Performance Benchmarks

### Benchmark Results

Based on our benchmarks on Apple M2 Pro:

| Operation | Zero-Copy | Traditional | Winner |
|-----------|-----------|-------------|---------|
| **Deserialization** | 351 ns/op | N/A | ✅ Zero-Copy (instant) |
| **Data Access** | 16.99ms | 76µs | ⚠️ Traditional (access overhead) |
| **Serialization** | 168µs | 79µs | ⚠️ Traditional (creation overhead) |
| **Memory Usage** | 5x sharing | 1x per copy | ✅ Zero-Copy (shared) |

### Key Insights

1. **Zero-copy excels in**:
   - Deserialization (instant access to data)
   - Memory sharing across multiple readers
   - Network/disk I/O scenarios
   - Large dataset processing

2. **Traditional structs excel in**:
   - Simple field access (no indirection)
   - Small datasets with frequent access
   - CPU-intensive operations

3. **Trade-offs**:
   - Zero-copy has accessor overhead for field access
   - Traditional copying has memory overhead for large data
   - Choice depends on access patterns and dataset size

## Usage Patterns

### 1. Core System Zero-Copy Integration

#### Enable Zero-Copy Mode in Producer
```go
prod := producer.NewProducer(
    producer.WithBlobStore(blobStore),
    producer.WithAnnouncer(announcer),
    producer.WithSerializationMode(internal.ZeroCopyMode), // Enable zero-copy
)
```

#### Zero-Copy Consumer with Direct Buffer Access
```go
zeroCopyConsumer := consumer.NewZeroCopyConsumerWithOptions(
    []consumer.ConsumerOption{
        consumer.WithBlobRetriever(blobStore),
        consumer.WithAnnouncementWatcher(announcer),
    },
    []consumer.ZeroCopyConsumerOption{
        consumer.WithZeroCopySerializationMode(internal.ZeroCopyMode),
    },
)

// Refresh with zero-copy support
err := zeroCopyConsumer.TriggerRefreshToWithZeroCopy(ctx, version)

// Get direct buffer access
if view, exists := zeroCopyConsumer.GetZeroCopyView(); exists {
    buffer := view.GetByteBuffer() // Direct buffer access
    message := view.GetMessage()   // Cap'n Proto message
}
```

#### Zero-Copy Index Building
```go
// Build indexes directly from Cap'n Proto buffers
view, _ := zeroCopyConsumer.GetZeroCopyView()
indexManager := index.NewZeroCopyIndexManager(view)

schema := map[string][]string{
    "movie": {"id", "title"},
}
indexManager.BuildAllIndexes(schema)

// Query using zero-copy indexes
adapter := indexManager.GetAdapter()
matches, _ := adapter.FindByHashIndex("movie_id_hash", "movie_123")
```

### 2. Hybrid Mode (Automatic Selection)

```go
// Automatically choose serialization based on data characteristics
prod := producer.NewProducer(
    producer.WithSerializationMode(internal.HybridMode),
)
// Small datasets use traditional mode, large datasets use zero-copy
```

### 3. Demonstration Layer Usage

```go
reader := zerocopy.NewZeroCopyReader(blobStore, announcer)
movies, err := reader.GetMovies()

// Direct access without copying
for i := 0; i < movies.Len(); i++ {
    movie := movies.At(i)
    title, _ := movie.Title()
    year := movie.Year()
    // Data is accessed directly from serialized buffer
}
```

### 2. Zero-Copy Filtering

```go
// Filter without copying data
moviesFrom2010 := []movie.Movie{}
for i := 0; i < movies.Len(); i++ {
    m := movies.At(i)
    if m.Year() == 2010 {
        moviesFrom2010 = append(moviesFrom2010, m) // Only store reference
    }
}
```

### 3. Zero-Copy Aggregation

```go
aggregator := zerocopy.NewZeroCopyAggregator()

// Calculate statistics without copying data
avgRuntime := aggregator.AverageRuntimeByYear(movies)
genreCounts, _ := aggregator.CountByGenre(movies)
```

### 4. Batch Processing

```go
iterator := zerocopy.NewZeroCopyIterator(movies, 1000)

for iterator.HasNext() {
    batch := iterator.Next() // Reference-only batch
    // Process batch without copying
}
```

## Best Practices

### When to Use Zero-Copy

✅ **Recommended for**:
- Large datasets (>10MB)
- Network/distributed systems
- Memory-constrained environments
- Read-heavy workloads
- Multiple consumers of same data
- Schema evolution requirements

⚠️ **Consider traditional structs for**:
- Small datasets (<1MB)
- Frequent field access patterns
- Simple data structures
- CPU-intensive computations
- Single-threaded access

### Memory Management

1. **Lifecycle**: Keep the original `*capnp.Message` alive while accessing data
2. **Sharing**: Multiple readers can safely share the same message
3. **Cleanup**: Messages are garbage collected automatically

### Schema Evolution

Cap'n Proto supports backward and forward compatible schema evolution:

```capnp
struct Movie {
  id @0 :UInt32;
  title @1 :Text;
  year @2 :UInt16;
  genres @3 :List(Text);
  runtimeMin @4 :UInt16;   # Added in v2 - old readers ignore
  # Future fields can be added without breaking compatibility
}
```

### Error Handling

```go
movies, err := dataset.Movies()
if err != nil {
    return fmt.Errorf("failed to access movies: %w", err)
}

for i := 0; i < movies.Len(); i++ {
    movie := movies.At(i)
    title, err := movie.Title()
    if err != nil {
        continue // Handle gracefully
    }
}
```

## Integration with go-hollow

### Current Status

The zero-copy implementation demonstrates:

1. ✅ **Cap'n Proto Schema Generation**: Working schemas for movie, commerce, and IoT domains
2. ✅ **Zero-Copy Data Access**: Efficient reading patterns with proper interfaces
3. ✅ **Performance Benchmarks**: Comprehensive benchmarking of zero-copy vs traditional approaches
4. ✅ **Example Applications**: Working examples showing real-world usage patterns

### Future Integration Points

1. **Blob Store Integration**: Direct Cap'n Proto serialization in blob storage
2. **Index Integration**: Zero-copy index building from Cap'n Proto data
3. **Network Protocol**: Zero-copy network message handling
4. **Consumer API**: Enhanced consumer with zero-copy data views

### Production Considerations

1. **Validation**: Always validate data integrity after deserialization
2. **Monitoring**: Track memory usage and access patterns
3. **Fallback**: Implement fallback to traditional structs if needed
4. **Testing**: Extensive testing with various data sizes and access patterns

## Examples

Run the zero-copy examples to see the implementation in action:

```bash
# Simple zero-copy demonstration
go run examples/go/zero_copy_simple/main.go

# Run zero-copy benchmarks
go test -bench=BenchmarkZeroCopyVsCopy -benchtime=1s
go test -bench=BenchmarkDeserializationZeroCopy -benchtime=1s
go test -bench=BenchmarkMemoryFootprintZeroCopy -benchtime=1s
```

## Conclusion

The zero-copy integration provides a powerful foundation for high-performance data processing in go-hollow. While it requires careful consideration of access patterns and memory management, it offers significant benefits for large-scale data processing scenarios.

The implementation demonstrates that zero-copy patterns can be successfully integrated with the go-hollow architecture, providing both performance benefits and memory efficiency for appropriate use cases.
