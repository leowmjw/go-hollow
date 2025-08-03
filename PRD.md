# Product Requirements Document: go-hollow

**Project:** go-hollow - Go implementation of Netflix Hollow with Cap'n Proto serialization  
**Version:** 1.0  
**Target Audience:** Go backend engineers & performance-oriented data platform teams  
**Last Updated:** January 2025

## Executive Summary

go-hollow is a high-performance, in-memory data system for Go that provides read-optimized datasets with snapshot/delta versioning, producer-consumer decoupling, and schema evolution. This implementation leverages Cap'n Proto for zero-copy serialization and maintains API compatibility with Netflix Hollow while following Go idioms.

## 1. System Overview

### Vision
Provide a production-ready Go library that delivers the same semantic guarantees as Netflix Hollow (Java) with superior performance through Cap'n Proto's zero-copy architecture.

### Core Principles
- **Zero-Copy Performance**: Direct memory access without deserialization overhead
- **Schema Evolution**: Backward-compatible data structure changes
- **Producer-Consumer Decoupling**: Independent data publication and consumption cycles
- **Read Optimization**: Optimized for high-frequency read workloads with minimal write overhead
- **Go Idioms**: Context-aware, error-returning, channel-based concurrency

### High-Level Architecture

```
┌─────────────┐    Snapshots/Deltas    ┌─────────────┐
│  Producer   │ ════════════════════▶ │  Consumer   │
│             │     (Cap'n Proto)      │             │
└─────────────┘                        └─────────────┘
       │                                      │
       ▼                                      ▼
┌─────────────┐                        ┌─────────────┐
│WriteStateEng│                        │ReadStateEng │
│+ Cap'n Proto│                        │+ mmap access│
│  Schema     │                        │+ Zero-copy  │
└─────────────┘                        └─────────────┘
       │                                      │
       └──────────▶ BlobStore ◀──────────────┘
                   + Announcer
```

## 2. Core Requirements

### 2.1 Schema Management

**Functional Requirements:**
- Define schemas using Cap'n Proto `.capnp` files
- Support all primitive types: Int8/16/32/64, UInt8/16/32/64, Float32/64, Bool, Text, Data
- Support collection types: List, Map (through Cap'n Proto unions)
- Support reference types with cross-schema dependencies
- Schema parsing and validation with topological sorting
- Schema evolution with backward compatibility
- Schema equality comparison (order-insensitive)

**Cap'n Proto Integration:**
```capnp
# Example schema file: movie.capnp
@0x85d3acc39d94e0f8;

struct Movie {
  id @0 :UInt32;
  title @1 :Text;
  year @2 :UInt16;
  genres @3 :List(Text);
  ratings @4 :List(Rating);
}

struct Rating {
  userId @0 :UInt32;
  score @1 :Float32;
  timestamp @2 :UInt64;
}
```

**API Design:**
```go
// Schema definition and management
type Schema interface {
    GetName() string
    GetSchemaType() SchemaType
    Validate() error
    Equals(other Schema) bool
    HashCode() uint64
}

// Schema compilation workflow
func CompileSchema(capnpFile string) ([]Schema, error)
func ParseSchemaCollection(dsl string) ([]Schema, error)
```

### 2.2 Producer System

**Functional Requirements:**
- Cycle-based data publishing with version management
- Empty cycles return version 0 (no publish)
- Identical data cycles return same version number
- Single producer enforcement with primary/secondary modes
- State restoration from previous versions
- Validation pipeline with rollback on failure
- Type resharding based on data volume
- Header tag management across versions

**Performance Requirements:**
- Full snapshot build: ≤ 3 min for 5M records on 8-core, 16GB RAM
- Delta build (<5% change): 10× faster than snapshot
- Zero-copy writes using Cap'n Proto builders
- Parallel serialization by shard

**API Design:**
```go
type Producer struct {
    writeEngine *WriteStateEngine
    blobStore   BlobStore
    announcer   Announcer
    validators  []Validator
}

// Functional options pattern
func NewProducer(opts ...ProducerOption) *Producer

func WithBlobStore(store BlobStore) ProducerOption
func WithValidator(v Validator) ProducerOption
func WithTypeResharding(enabled bool) ProducerOption
func WithTargetMaxTypeShardSize(size int) ProducerOption

// Core producer cycle
func (p *Producer) RunCycle(ctx context.Context, populate func(*WriteState)) (version int64, err error)
func (p *Producer) Restore(ctx context.Context, version int64, retriever BlobRetriever) error
```

### 2.3 Consumer System

**Functional Requirements:**
- Version-based data consumption with refresh triggers
- Delta chain traversal for incremental updates
- Reverse delta navigation for version rollback
- Automatic updates via announcement mechanisms
- Version pinning for controlled updates
- Type filtering for selective data loading
- Memory mode support (shared memory via mmap, private heap)

**Performance Requirements:**
- Consumer refresh (delta): < 150ms P99
- Query latency (index lookup): < 50µs median
- Concurrent readers: 1000 goroutines without contention
- Heap overhead: ≤ 30% in shared memory mode

**API Design:**
```go
type Consumer struct {
    readEngine *ReadStateEngine
    retriever  BlobRetriever
    watcher    AnnouncementWatcher
}

func NewConsumer(opts ...ConsumerOption) *Consumer

func WithBlobRetriever(retriever BlobRetriever) ConsumerOption
func WithAnnouncementWatcher(watcher AnnouncementWatcher) ConsumerOption
func WithTypeFilter(filter TypeFilter) ConsumerOption
func WithMemoryMode(mode MemoryMode) ConsumerOption

func (c *Consumer) TriggerRefreshTo(ctx context.Context, version int64) error
func (c *Consumer) TriggerRefresh(ctx context.Context) error
func (c *Consumer) GetCurrentVersion() int64
```

### 2.4 Collection Types with Generics

**Functional Requirements:**
- HollowSet[T]: Unordered unique element collections with containment checks
- HollowMap[K,V]: Key-value mappings with entry iteration and equality
- HollowList[T]: Ordered element sequences with indexed access
- Zero ordinal handling for all collection types
- State invalidation exception handling
- Iterator interfaces with proper resource management

**Cap'n Proto Mapping:**
- Sets implemented as sorted Lists in Cap'n Proto
- Maps implemented as List of key-value structs
- Lists map directly to Cap'n Proto Lists

**API Design:**
```go
type HollowSet[T any] interface {
    Contains(element T) bool
    Size() int
    Iterator() Iterator[T]
    IsEmpty() bool
}

type HollowMap[K comparable, V any] interface {
    Get(key K) (V, bool)
    Size() int
    EntrySet() []Entry[K, V]
    Keys() Iterator[K]
    Values() Iterator[V]
    Equals(other HollowMap[K, V]) bool
}

type HollowList[T any] interface {
    Get(index int) T
    Size() int
    Iterator() Iterator[T]
    IsEmpty() bool
}
```

### 2.5 Indexing System

**Functional Requirements:**
- Hash indexes for multi-field lookups with collision handling
- Unique key indexes for single-result queries
- Primary key indexes for composite key lookups
- Null value handling in indexed fields
- Index updates during state transitions
- Byte array field indexing
- Generic type safety

**API Design:**
```go
type HashIndex[T any] interface {
    FindMatches(ctx context.Context, values ...interface{}) Iterator[T]
    DetectUpdates(ctx context.Context) error
}

type UniqueKeyIndex[T any] interface {
    GetMatch(ctx context.Context, key interface{}) (T, bool)
    DetectUpdates(ctx context.Context) error
}

func NewHashIndex[T any](engine *ReadStateEngine, typeName string, matchFields []string) HashIndex[T]
func NewUniqueKeyIndex[T any](engine *ReadStateEngine, typeName string, keyFields []string) UniqueKeyIndex[T]
```

### 2.6 Cap'n Proto Integration Details

**Serialization Format:**
- Use Cap'n Proto's packed format for bandwidth efficiency
- Little-endian byte order for CPU efficiency
- Memory-mapped file support for large datasets
- Zero-copy deserialization for read operations

**Schema Evolution Strategy:**
- Cap'n Proto fields numbered in addition order
- New fields added to end of structs
- Removed fields leave gaps (handled by bounds checking)
- Union types for polymorphic data

**Code Generation Workflow:**
```bash
# Schema compilation
capnpc -ogo:./generated schema/*.capnp

# Integration with go generate
//go:generate capnpc -ogo:./generated schema/movie.capnp
```

**Memory Management:**
- Arena allocation for Cap'n Proto messages
- sync.Pool for buffer reuse
- Automatic garbage collection integration
- mmap support for shared memory mode

## 3. Non-Functional Requirements

### 3.1 Performance
- Zero allocation in read hot paths
- Sub-microsecond field access
- Minimal serialization overhead
- Efficient memory layout via Cap'n Proto

### 3.2 Compatibility
- Wire format compatibility with Cap'n Proto specification
- API compatibility with test scenarios in TEST.md
- Go version compatibility: 1.21+ (for generics)

### 3.3 Reliability
- 95%+ test coverage on core packages
- Race condition detection via go test -race
- Fuzz testing for parser and serialization
- Integration tests with real-world data sizes

### 3.4 Usability
- Comprehensive documentation with examples
- CLI tools for debugging and introspection
- Clear error messages with actionable guidance
- Idiomatic Go patterns throughout

## 4. Architecture Components

### 4.1 Core Packages

```
go-hollow/
├── schema/          # Cap'n Proto schema management
├── producer/        # Data production and publishing
├── consumer/        # Data consumption and reading
├── collections/     # Generic collection types
├── index/          # Indexing and query system
├── blob/           # Serialization and storage
├── tools/          # CLI utilities and debugging
└── internal/       # Private implementation details
```

### 4.2 External Dependencies

**Required:**
- `capnproto.org/go/capnp/v3` - Cap'n Proto Go bindings
- `golang.org/x/sync/errgroup` - Concurrent operations
- `golang.org/x/exp/mmap` - Memory mapping support

**Optional:**
- Cloud storage SDKs (AWS, GCP, Azure) for BlobStore implementations
- Message queue clients (Kafka, NATS) for Announcer implementations
- Monitoring libraries (Prometheus, OpenTelemetry) for metrics

## 5. Implementation Phases

### Phase 1: Foundation (3 weeks)
- Cap'n Proto schema definition and compilation
- Basic serialization/deserialization
- Core interfaces and types
- Test framework setup

### Phase 2: Write Path (4 weeks)
- WriteStateEngine with Cap'n Proto builders
- Producer cycle management
- Snapshot and delta generation
- Validation pipeline

### Phase 3: Read Path (4 weeks)
- ReadStateEngine with zero-copy access
- Consumer refresh logic
- Collection type implementations
- Memory management

### Phase 4: Indexing (3 weeks)
- Hash index implementation
- Unique and primary key indexes
- Index update mechanisms
- Query performance optimization

### Phase 5: Tools & Integration (2 weeks)
- CLI utilities
- BlobStore implementations
- Announcer implementations
- Documentation and examples

### Phase 6: Performance & Hardening (3 weeks)
- Benchmark suite
- Performance tuning
- Security review
- Production readiness checklist

## 6. Success Metrics

### 6.1 Performance Benchmarks
- Serialization: 10GB/s throughput
- Deserialization: Zero-copy (infinite speed)
- Index queries: <50µs P50, <500µs P99
- Memory efficiency: <30% overhead vs raw data

### 6.2 Quality Gates
- 95% test coverage
- Zero race conditions in CI
- All linting tools pass
- Documentation coverage >90%

### 6.3 Adoption Metrics
- API stability (semver compatibility)
- Community contributions
- Production usage examples
- Performance comparisons vs alternatives

## 7. Risk Mitigation

### 7.1 Technical Risks
- **Cap'n Proto API changes**: Pin specific versions, contribute to upstream
- **Performance regressions**: Continuous benchmarking in CI
- **Memory leaks**: Comprehensive testing with race detector and memory profilers
- **Schema evolution bugs**: Extensive backward compatibility tests

### 7.2 Project Risks
- **Scope creep**: Strict adherence to MVP requirements
- **Timeline pressure**: Prioritize core functionality over nice-to-have features
- **Resource constraints**: Clear milestone definitions and progress tracking

## 8. Future Considerations

### 8.1 Advanced Features
- Distributed consensus for multi-producer scenarios
- Real-time streaming updates
- Cross-language RPC integration
- Advanced compression algorithms

### 8.2 Ecosystem Integration
- Kubernetes operators
- Service mesh integration
- Observability platform connectors
- Data pipeline tool integrations

---

This PRD provides a comprehensive specification for implementing go-hollow with Cap'n Proto integration while maintaining compatibility with the behaviors defined in TEST.md. The zero-copy architecture of Cap'n Proto aligns perfectly with Hollow's read-optimization goals, potentially delivering significant performance improvements over the Java implementation.
