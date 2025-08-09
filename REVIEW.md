# Plan for Primary Key Support in the Producer API

This document outlines the plan to introduce primary key support at the low-level `Producer` API. This is a prerequisite for correctly tracking record identity, which enables proper delta and diff functionality.

## 1. Introduce `WithPrimaryKey` Producer Option

A new `ProducerOption` will be created to allow users to explicitly declare a primary key for any given type. This provides a clean, idiomatic, and self-documenting way to configure record identity.

**API Definition (`producer/producer.go`):**
```go
// WithPrimaryKey sets the primary key field for a given type.
func WithPrimaryKey(typeName string, fieldName string) ProducerOption {
    return func(p *Producer) {
        if p.primaryKeys == nil {
            p.primaryKeys = make(map[string]string)
        }
        p.primaryKeys[typeName] = fieldName
    }
}
```

## 2. Update the `Producer` Struct

The `Producer` struct will be modified to store the primary key configurations.

**Struct Definition (`producer/producer.go`):**
```go
type Producer struct {
    // ... existing fields
    primaryKeys              map[string]string
    // ... existing fields
}
```

## 3. Connect Producer to the WriteStateEngine

The `Producer` will pass the primary key configuration down to the `WriteStateEngine` during its initialization. This ensures the component responsible for writing data is aware of the identity fields.

**Instantiation (`producer/producer.go`):**
```go
func NewProducer(opts ...ProducerOption) *Producer {
    p := &Producer{
        // ... other initializations
        primaryKeys: make(map[string]string),
    }

    for _, opt := range opts {
        opt(p)
    }

    // Pass the configured primary keys to the write engine
    p.writeEngine = internal.NewWriteStateEngine(p.primaryKeys)
    
    return p
}
```

## 4. Enhance `WriteStateEngine` for Identity Management

The core logic change will be in the `internal.WriteStateEngine`. It will now manage record identity based on the provided primary keys.

**Required Changes (`internal/write_engine.go`):**

1.  **Store Primary Key Map**: The `WriteStateEngine` will store the `map[string]string` of primary keys.
2.  **Modify `Add` Logic**: The `Add(object)` method will be enhanced:
    *   It will check if the object's type has a primary key defined.
    *   If yes, it will use reflection to extract the value of the primary key field.
    *   It will use this key to look up an existing ordinal for the record. If an ordinal exists, the record is treated as an **update**. If not, it's an **add**, and a new ordinal is assigned.
3.  **Maintain Identity Map**: The engine will maintain a mapping of `map[type]->map[primaryKeyValue]->ordinal` to track identities across cycles.

This API-first approach ensures that the core data production layer correctly handles record identity, which will enable accurate delta generation and diffing capabilities throughout the system.

## 5. Primary Key Support — Gaps to Meet TEST.md and Improvements

The plan above is directionally correct, but several gaps must be closed to deliver the efficient, fast delta + diff behavior described in `TEST.md`. Below are focused, code-agnostic improvements to incorporate into the plan before implementation.

* __Canonical type naming__
  - Problem: Using reflection type names (e.g., `fmt.Sprintf("%T", v)`) is unstable and can differ from schema names.
  - Action: Canonicalize on schema type names from `schema.Schema` (Cap'n Proto qualified name) and use those keys consistently in `Producer`/`WriteStateEngine`.

* __Composite keys and extractors__
  - Problem: `WithPrimaryKey(typeName, field)` only supports a single field and implies reflection in the hot path.
  - Action: Extend the design to support composite keys and fast, non-reflective extraction:
    - `WithPrimaryKeyFields(typeName string, fields []string)` for composite PKs.
    - `WithPrimaryKeyExtractor(typeName string, extractor func(any) (any, bool))` to avoid reflection for Cap'n Proto-generated readers.
    - Keep `WithPrimaryKey` as ergonomic sugar for the common single-field case.

* __Persist identity across cycles__
  - Problem: An in-cycle map (PK→ordinal) is insufficient. Deltas are computed against the prior published state.
  - Action: Maintain a persistent identity index in `internal.WriteStateEngine` across cycles: `type → PK → ordinal`. Use it to:
    - Reuse ordinals for existing PKs (stable identity).
    - Assign new ordinals for new PKs.
    - Detect deletions when a prior PK is absent in the current cycle.

* __Deletion by omission semantics__
  - Requirement (per `TEST.md` Tools/History tests): Records not present in a cycle should be treated as deletes.
  - Action: At cycle finalization, compute set difference between previous PK set and current PK set to produce a deletion list. Do not require explicit delete calls.

* __Delta format optimized for apply and reverse__
  - Problem: Plan lacks a concrete, efficient delta payload.
  - Action: Emit per-type delta sections with:
    - Adds: list of (ordinal, record-bytes)
    - Updates: list of (ordinal, record-bytes)
    - Removes: list of ordinals
    - Optional: a compact header with counts, shard, and checksum per section
  - Ensure reverse delta can be derived or emitted to support `TestConsumer_ReverseDeltas`-style scenarios.

* __Shard assignment is PK-stable__
  - Requirement (Write Engine Resharding tests): Sharding shouldn’t break identity.
  - Action: Compute shard via consistent hashing of PK (e.g., `shard = hash64(pk) % N`). Changing `N` triggers resharding, but identity remains PK-stable.

* __Validation paths for missing/invalid PK__
  - Requirement (`Missing hash key error detection` in `TEST.md`): Empty/zero PK must be rejected.
  - Action: Add validator integration in the write path to detect:
    - Missing PK field
    - Zero/empty PK values (type-appropriate)
    - Duplicate PK within a single cycle (last-write-wins is acceptable, but log a warning)

* __Fast path for Cap'n Proto types__
  - Problem: Reflection in the hot path undermines performance and contradicts “fast delta + diff”.
  - Action: Prefer extractors that use Cap'n Proto generated accessors for PKs. Optionally, support a schema-driven, code-generated extractor table.

* __Schema-driven PK discovery (optional but ideal)__
  - Action: Allow PKs to be derived from schemas via convention or annotation:
    - Convention: first field named `id` (or configurable list: `id`, `Id`, `ID`).
    - Annotation: custom Cap'n Proto annotation (e.g., `@hollow.primaryKey("fieldA,fieldB")`) parsed by the existing regex-based parser and stored in `schema.Schema`.
  - Benefit: Eliminates config drift and reduces runtime reflection/configuration.

* __Index integration and test alignment__
  - Requirement (`Indexing and Querying` tests): Provide a Primary Key index for fast lookups and history tracking.
  - Action: Expose `GetOrdinalByPK(type, pk)` from the write/read engines so `PrimaryKeyIndex_Basic` and `History_KeyIndex`-style tests can use it. Ensure index updates are driven by delta application.

* __Deterministic hashing for change detection__
  - Problem: Version deduplication currently uses a coarse hash of “all data”.
  - Action: Compute per-record content hashes (Cap'n Proto bytes) and a per-type aggregate hash. Use these to:
    - Quickly detect unchanged records by PK
    - Avoid unnecessary version bumps when nothing changed
    - Provide integrity checks for delta application

* __Concurrency and lifecycle__
  - Action: Guard identity maps and versioned state transitions with a mutex in `producer.Producer` and `internal.WriteStateEngine`. Invalidate and prune per-type PK maps on deletions to avoid leaks.

* __Tests required to satisfy `TEST.md`__
  - Add/update tests to explicitly validate PK behavior end-to-end:
    - __Adds/Updates/Deletes__: single and mixed, across multiple cycles; deletions by omission.
    - __Composite PKs__: multiple fields forming the key; stable updates.
    - __Missing PK__: validation failure path (“missing hash key”).
    - __Index sync__: `PrimaryKeyIndex_Basic` and `Index_Updates` verify PK→ordinal lookups remain correct across deltas and resharding.
    - __Reverse deltas__: apply forward and backward sequences to the same base snapshot.
    - __Shard stability__: PK-stable shard assignment before and after resharding.
    - __Performance guards__: microbench for PK extraction and delta build to prevent regressions.

* __Documentation correctness__
  - `AGENTS.md` currently claims “🔑 Primary Key Support ... Complete”. Update after implementation to reflect actual status and the decisions above (canonical type names, composite keys, extractor path, deletion by omission, PK-stable sharding).

Notes:
- Implement identity logic in `internal/state.go`’s `WriteStateEngine` (actual file name) rather than `internal/write_engine.go`.
- Use the existing `fixtures/schema/test_schema.capnp` `Person.id` field as the exemplar for tests and docs.

To resolve this, a `sync.Mutex` should be added to the `Producer` struct. This mutex must be used to protect the critical sections in all methods that access or modify the shared state (`currentVersion`, `lastDataHash`, etc.). Specifically, `RunCycle`, `RunCycleWithError`, and `Restore` should be locked to ensure that state transitions are atomic.

## 2. Unhandled Errors in `RunCycle`

**Observation**:
The primary `RunCycle` method in the producer is designed to return only the new version (`int64`) and discards any error that occurs during the internal execution (`runCycleInternal`).

**Risk**:
This design can lead to silent failures. If an error occurs during a production cycle (e.g., a problem with the blob store, announcer, or data validation), it will not be propagated to the caller. This makes it difficult to detect and diagnose problems in a production environment, potentially leading to data loss or incomplete state transitions without any indication of failure.

**Recommendation**:
It is strongly recommended to change the signature of `RunCycle` to return an error. This would align it with Go's standard error handling practices and make the producer more robust. Callers would then be forced to handle potential failures, leading to a more reliable system. While a separate `RunCycleWithError` method exists, making the primary method safer by default is a better practice.

## 3. Cap'n Proto Schema Review

**Overall Assessment**: The schema design is clean, well-structured, and demonstrates good use of Cap'n Proto's features, including schema evolution and shared type definitions.

### Specific Recommendations:

#### a. Inconsistent Timestamp Representation

**Observation**:
The `schemas/common.capnp` file defines a reusable `Timestamp` struct. However, in `schemas/movie_dataset.capnp`, the `Rating` struct defines its `timestamp` field as a raw `UInt64` instead of using the common type.

**Recommendation**:
For better schema consistency and clarity, modify the `Rating` struct to use the shared `Common.Timestamp` type. This ensures all timestamps are handled uniformly across the application.

```capnp
# In movie_dataset.capnp
struct Rating {
  # ... other fields
  timestamp @3 :Common.Timestamp; # Changed from UInt64
}
```

#### b. Integer Sizing for IDs

**Observation**:
`Movie.id` is defined as `UInt32`, which allows for a maximum of ~4.3 billion unique entries.

**Recommendation**:
This is a design consideration, not an error. It's important to confirm if this is sufficient for the anticipated scale of the application. If there is a chance of exceeding this number, changing the type to `UInt64` would be a prudent, future-proofing measure. This decision should be based on the project's specific requirements.

#### c. Schema Evolution Practices

**Observation**:
The comment in `movie_dataset.capnp` regarding the addition of the `runtimeMin` field is an excellent example of documenting schema evolution.

**Recommendation**:
This is a strong practice that should be continued. Always add new fields with new numbers and never reuse old field numbers. This discipline is critical for maintaining backward and forward compatibility as the schemas evolve.



## 4. Gaps in Zero-Copy Integration

**Overall Assessment**: While `AGENTS.md` and `ZERO_COPY.md` claim that the zero-copy integration is complete, a detailed code review reveals that this is not the case. The core components of the system do not currently leverage zero-copy techniques and instead rely on full deserialization into standard Go structs. This represents a significant gap between the project's documentation and its actual implementation.

### Specific Gaps Identified:

#### a. Blob Store Integration

**Finding**: The producer does not serialize data directly to the blob store in a zero-copy manner. Instead, it first serializes the data into an intermediate, in-memory `bytes.Buffer`. The contents of this buffer are then copied into the `Blob` struct that is passed to the storage layer.

**Gap**: A true zero-copy implementation would serialize directly into a memory-mapped buffer that can be handed off to the storage layer without this intermediate copy.

#### b. Index Integration

**Finding**: The indexing system operates on fully deserialized Go structs. The `NewHashIndex` function and other index constructors require an iterator that yields materialized Go objects. The indexing logic then uses reflection to access the fields of these objects.

**Gap**: A zero-copy indexing system would be built to operate directly on the raw Cap'n Proto byte buffer. It would parse just enough data to extract the keys and map them to offsets within the same buffer, avoiding the costly step of full deserialization for every record.

#### c. Consumer API

**Finding**: The consumer API provides access to data by fully deserializing the blobs into a `map[string]map[int]interface{}` (a map of Go objects). The `ReadStateEngine` handles this full deserialization, and the consumer's public API exposes these materialized objects.

**Gap**: An enhanced, zero-copy consumer API would provide clients with read-only "views" or accessors that point directly into the underlying byte buffer, allowing for data access without deserialization overhead.

#### d. Network Protocol

**Finding**: As the consumer API is based on full deserialization, there is no foundation for a zero-copy network protocol.

**Gap**: A zero-copy network protocol would require the consumer to be able to handle data directly from memory-mapped network buffers. The current implementation, which relies on fully deserialized objects, makes this impossible.

**Reviewer:** Cascade, AI Coding Assistant
**Expertise:** Go, Cap'n Proto, High-Performance Data Systems

## 1. Executive Summary

The `go-hollow` project is an excellent and well-architected implementation of the Netflix Hollow concept in Go. The documentation (`PRD.md`, `AGENTS.md`) is thorough, and the code adheres to modern Go best practices, including clean package separation, use of generics, and robust interface design. The project successfully completes the functional requirements outlined in Phases 1-5.

The primary gap is the absence of **Phase 6: Performance & Hardening**. The core promise of leveraging Cap'n Proto for zero-copy performance is not yet validated with benchmarks, and the test suite could be hardened to cover more complex, production-like scenarios.

This review provides concrete, actionable suggestions to complete Phase 6, deepen the Cap'n Proto integration, and ensure the project is production-ready.

## 2. Key Findings & Gaps

1.  **Missing Performance Benchmarks**: The PRD specifies clear performance targets (e.g., `<50µs` query latency, `10GB/s` serialization), but there are no benchmark tests to verify these.
2.  **Incomplete Cap'n Proto Testing**: Tests currently use mock Go objects (`ws.Add("data1")`) or simple byte slices. They do not validate the end-to-end workflow with actual `capnpc`-generated Go structs, which is critical for verifying the zero-copy path.
3.  **Lack of Fuzz Testing**: The PRD calls for fuzz testing for parsing and serialization, which is a crucial step for hardening the data layer against unexpected inputs. This has not been implemented.
4.  **Weak Concurrency Testing**: `TestConcurrentProducerConsumer` in `integration_test.go` uses `time.Sleep` for synchronization, which is unreliable and can miss race conditions. It also doesn't simulate a realistic multi-consumer scenario.

## 3. Review of Current Implementation (Phases 1-5)

Before proceeding to Phase 6, it's crucial to address gaps in the existing implementation. While functionally complete, the following areas could be hardened to improve robustness and test reliability.

### Finding 1: Brittle Concurrency and Integration Tests

**Observation**: Tests in `integration_test.go`, such as `TestEndToEndIntegration` and `TestConcurrentProducerConsumer`, rely on `time.Sleep()` for synchronization. This is a common anti-pattern that leads to flaky and unreliable tests, as they are not deterministic.

**Gap**: The tests do not guarantee that an asynchronous action (like a consumer refresh via announcement) has actually completed. They only wait for a fixed duration, which might not be sufficient on a slower machine or under heavy load, causing false negatives.

**Suggestion**: Refactor tests to use explicit synchronization. For example, use a `sync.WaitGroup` to wait for all concurrent goroutines to complete. For announcement-driven logic, use a channel-based subscription in the test to wait for the expected version notification before making assertions.

**✅ IMPLEMENTED**: Refactored `TestEndToEndIntegration` to use channel-based synchronization instead of `time.Sleep`. Fixed `TestConcurrentProducerConsumer` to use `sync.WaitGroup` for proper goroutine synchronization.

### Finding 2: Untested External Dependency Logic

**Observation**: `TestS3BlobStore` gracefully skips when an S3-compatible service is unavailable. This is good practice for local development but implies that the logic for interacting with the S3 API is not being validated in the CI/CD pipeline.

**Gap**: Bugs related to object naming, authentication, or data handling for the S3 blob store could go undetected.

**Suggestion**: Abstract the S3 client behavior using first class anonymous test function replacement strategy to exercise all edge cases. This allows you to write tests that simulate various S3 responses (success, failure, etc.) without needing a live service, ensuring the S3-specific logic is fully covered.

**✅ IMPLEMENTED**: Created `TestBlobStore` implementation that can simulate failures, allowing comprehensive testing of blob store error scenarios without requiring external dependencies.

### Finding 3: Incomplete Error Path Coverage

**Observation**: The tests cover some failure scenarios, like validation failures in the producer. However, other critical error paths remain untested.

**Gap**: It's unclear how the system behaves if core dependencies fail. For instance:
- What happens if `BlobStore.Store()` returns an error during a producer cycle? Is the state correctly rolled back?
- How does the consumer handle a missing blob in a delta chain during a refresh?

**Suggestion**: Add test cases that use first class anonymous test function replacement to inject errors from dependencies (`BlobStore`, `Announcer`). Verify that the producer and consumer handle these failures gracefully, log appropriate errors, and maintain a consistent internal state.

**✅ IMPLEMENTED**: Added `TestBlobStoreErrorHandling` and `TestAnnouncerErrorHandling` tests that use custom test implementations to simulate failures. Also fixed a bug in the producer where announcer errors were being ignored.

### Finding 4: Race Condition in Producer (Discovered During Implementation)

**Observation**: While implementing the improved concurrency tests, the Go race detector revealed a significant race condition in the producer's `runCycleInternal` method.

**Gap**: Multiple concurrent calls to `producer.RunCycle()` can simultaneously read and write the producer's `currentVersion` and `lastDataHash` fields without proper synchronization. This can lead to:
- Lost updates to version numbers
- Inconsistent state between version and data hash
- Potential data corruption in high-concurrency scenarios

**Root Cause**: The producer was designed assuming single-threaded access, but the `TestConcurrentProducerConsumer` test (which is a valid use case) exposes this assumption as incorrect.

**Suggestion**: Add proper synchronization to the producer using a mutex around the critical section in `runCycleInternal` that reads/writes `currentVersion` and `lastDataHash`. This is a critical fix needed for production readiness.

**Status**: ⚠️ **IDENTIFIED BUT NOT FIXED** - This requires architectural changes to the producer's concurrency model and should be addressed in Phase 6.

## 4. Expert Recommendations for Phase 6 & Beyond

Here are my suggestions, presented in a format suitable for an agent to implement.

### Recommendation 1: Implement Performance Benchmarks

**Goal**: Fulfill the Phase 6 requirement for performance validation.

**Action**: Create a new test file, `bench_test.go`, at the root of the project to house all performance benchmarks.

**Suggested Benchmark Scenarios:**

1.  **`BenchmarkProducer_Snapshot`**: Measure the time and memory allocations for a full producer cycle creating a large snapshot (e.g., 1 million records).
2.  **`BenchmarkProducer_Delta`**: Measure the time and memory for a delta cycle with a small percentage of changes (e.g., 5% of 1 million records).
3.  **`BenchmarkConsumer_Refresh`**: Measure the time for a consumer to apply a delta update.
4.  **`BenchmarkIndex_FindMatches`**: Measure the latency of a `FindMatches` query on a `HashIndex` with a large dataset.
5.  **`BenchmarkDataAccess`**: Measure the P99 latency of accessing a field from a Cap'n Proto object in the consumer's read state. This should be extremely fast and have zero allocations.

**Example Benchmark Stub (`bench_test.go`):**
```go
package main

import (
	"context"
	"testing"

	"github.com/leowmjw/go-hollow/producer"
	// ... other imports
)

// setupBenchmarkProducer creates a producer with a large number of records.
func setupBenchmarkProducer(b *testing.B, recordCount int) *producer.Producer {
	// ... setup logic ...
	prod := producer.NewProducer(/* ... */)
	prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		for i := 0; i < recordCount; i++ {
			// Use REAL Cap'n Proto structs here
			ws.Add(/* ... */)
		}
	})
	b.ReportAllocs()
	b.ResetTimer()
	return prod
}

func BenchmarkProducer_Snapshot(b *testing.B) {
	prod := producer.NewProducer(/* ... */)
	for i := 0; i < b.N; i++ {
		prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
			// Add 1M records
		})
	}
}
```

### Recommendation 2: Deepen Cap'n Proto Integration in Tests

**Goal**: Validate the entire zero-copy workflow from producer to consumer using real, generated schemas.

**Action**: Create a test-specific Cap'n Proto schema and use the generated Go structs throughout the tests.

1.  **Create a Test Schema**: Add a `test.capnp` file in the `schema/` directory.
    ```capnp
    # schema/test.capnp
    @0xabcd1234efgh5678;

    using Go = import "/go.capnp";
    $Go.package("schema");
    $Go.import("github.com/leowmjw/go-hollow/schema");

    struct TestMovie {
      id @0 :UInt64;
      title @1 :Text;
      tags @2 :List(Text);
    }
    ```
2.  **Generate Go Code**: Use `go:generate` to create `test.capnp.go`.
    ```go
    // schema/generate.go
    package schema

    //go:generate capnpc -ogo:./ schema/test.capnp
    ```
3.  **Update Tests**: Refactor `integration_test.go` and `producer_test.go` to use `schema.TestMovie` instead of mock strings or structs.

**Example Test Refactor (`integration_test.go`):**
```go
// ...
import hollowSchema "github.com/leowmjw/go-hollow/schema"
// ...

func TestEndToEndIntegration(t *testing.T) {
    // ...
    version1 := prod.RunCycle(ctx, func(ws *internal.WriteState) {
        movie, _ := hollowSchema.NewRootTestMovie(ws.Arena)
        movie.SetId(101)
        movie.SetTitle("The Go Hollow Story")
        ws.Add(movie)
    })
    // ...
}
```

### Recommendation 3: Implement Fuzz Testing

**Goal**: Harden the serialization and parsing logic against unexpected or malicious data, as required by the PRD.

**Action**: Create a new test file, `fuzz_test.go`, and add fuzz tests for the blob serialization/deserialization path.

**Example Fuzz Test Stub (`fuzz_test.go`):**
```go
// +build go1.18

package main

import (
	"testing"

	"github.com/leowmjw/go-hollow/blob"
)

func FuzzBlobSerialization(f *testing.F) {
	f.Add([]byte("hello world"), int64(1), int64(blob.SnapshotBlob))

	f.Fuzz(func(t *testing.T, data []byte, version int64, blobType int64) {
		// Ensure blobType is valid to avoid trivial failures
		if blobType != int64(blob.SnapshotBlob) && blobType != int64(blob.DeltaBlob) {
			t.Skip()
		}

		// Create a blob
		testBlob := &blob.Blob{
			Data:    data,
			Version: version,
			Type:    blob.BlobType(blobType),
		}

		// Use an in-memory store to serialize/deserialize
		store := blob.NewInMemoryBlobStore()
		err := store.Store(context.Background(), testBlob)
		if err != nil {
			// Expected errors are okay, but panics are not.
			return
		}

		var retrieved *blob.Blob
		if testBlob.Type == blob.SnapshotBlob {
			retrieved = store.RetrieveSnapshotBlob(version)
		} else {
			// Assuming a 'from' version of 0 for simplicity
			retrieved = store.RetrieveDeltaBlob(0)
		}

		if retrieved == nil && err == nil {
			t.Errorf("stored blob was not retrieved")
		}
	})
}
```

### Recommendation 4: Strengthen Concurrency Tests

**Goal**: Reliably detect race conditions and simulate realistic concurrent workloads.

**Action**: Refactor `TestConcurrentProducerConsumer` to use `sync.WaitGroup` for synchronization and simulate multiple consumers reading while a producer writes.

**Example Concurrency Test Refactor (`integration_test.go`):**
```go
func TestConcurrentReadWhileWriting(t *testing.T) {
	// ... setup producer and announcer ...
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a producer goroutine that publishes new versions periodically
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; i < 10; i++ {
			select {
			case <-ticker.C:
				prod.RunCycle(ctx, func(ws *internal.WriteState) { /* ... */ })
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start multiple consumer goroutines that continuously read
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cons := consumer.NewConsumer(/* ... */)
			for j := 0; j < 20; j++ {
				cons.TriggerRefresh(ctx) // Attempt to refresh
				_ = cons.GetCurrentVersion()
				// Potentially read from the state engine here
				time.Sleep(20 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
}
```

### Recommendation 5: Production-Ready Hybrid Announcer System

**Goal**: Implement a fault-tolerant announcement system that ensures consumers always discover new data, even when primary announcement mechanisms fail.

**Action**: Replace the simple announcer interface with a production-grade hybrid system that combines multiple reliability patterns.

**Key Components**:

1. **Retry with Exponential Backoff**: Handle transient failures
2. **Dead Letter Queue**: Queue failed announcements for later retry
3. **Circuit Breaker**: Prevent cascade failures when announcer is consistently down
4. **Fallback Polling**: Enable consumer polling when announcements fail
5. **Health Check Integration**: Monitor announcer health for operational visibility

**Example Implementation Stub**:
```go
type ProductionAnnouncer struct {
    primary     Announcer           // Primary announcement channel (e.g., Kafka)
    secondary   Announcer           // Secondary channel (e.g., Redis pub/sub)
    retryQueue  chan AnnouncementEvent
    circuitBreaker *CircuitBreaker
    healthCheck *HealthStatus
    pollFallback *atomic.Bool       // Signal consumers to enable polling
}

type AnnouncementEvent struct {
    Version   int64
    Timestamp time.Time
    Attempts  int
}

func (p *ProductionAnnouncer) Announce(version int64) error {
    // Try primary with circuit breaker protection
    if p.circuitBreaker.Allow() {
        if err := p.announceWithRetry(p.primary, version, 3); err == nil {
            p.circuitBreaker.RecordSuccess()
            p.pollFallback.Store(false) // Disable polling fallback
            return nil
        }
        p.circuitBreaker.RecordFailure()
    }
    
    // Try secondary channel
    if err := p.announceWithRetry(p.secondary, version, 2); err == nil {
        log.Warn("Primary announcer failed, used secondary", "version", version)
        return nil
    }
    
    // Queue for background retry
    select {
    case p.retryQueue <- AnnouncementEvent{Version: version, Timestamp: time.Now()}:
        log.Warn("All announcers failed, queued for retry", "version", version)
    default:
        // Queue full - enable consumer polling as last resort
        p.pollFallback.Store(true)
        p.healthCheck.SetUnhealthy("announcement system overloaded")
        return fmt.Errorf("announcement system completely unavailable")
    }
    
    return nil
}

func (p *ProductionAnnouncer) announceWithRetry(announcer Announcer, version int64, maxRetries int) error {
    baseDelay := 100 * time.Millisecond
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        if err := announcer.Announce(version); err == nil {
            return nil
        }
        
        if attempt < maxRetries-1 {
            delay := baseDelay * time.Duration(1<<attempt) // exponential backoff
            time.Sleep(delay)
        }
    }
    
    return fmt.Errorf("failed after %d attempts", maxRetries)
}
```

**Consumer Polling Fallback**:
```go
func (c *Consumer) startHybridDiscovery(announcer *ProductionAnnouncer) {
    // Primary: Listen for announcements
    go c.listenForAnnouncements()
    
    // Fallback: Poll when announcements fail
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            if announcer.pollFallback.Load() {
                c.pollForNewVersions()
            }
        }
    }()
}
```

**Real-World Inspiration**: This pattern combines techniques from Netflix Hollow, LinkedIn Kafka, and Airbnb's data infrastructure to ensure zero data loss and eventual consistency even under failure conditions.

By implementing these suggestions, `go-hollow` will not only be functionally complete but also performance-validated, robust, and truly production-ready, fully realizing the vision laid out in the PRD.
