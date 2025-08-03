# Hollow Go Implementation - Test Scenarios

This document summarizes the key behaviors and test scenarios extracted from the Netflix Hollow Java codebase, converted to equivalent Go test cases.

## Overview

The test suite covers the core functionality of the Hollow data system with Cap'n Proto integration, including:
- Cap'n Proto schema compilation and management with validation
- Producer-consumer cycles with zero-copy versioning
- Collection types (Sets, Maps, Lists) backed by Cap'n Proto structures
- Indexing and querying capabilities with memory-mapped access
- State engine management with arena allocation
- Data serialization and blob handling using Cap'n Proto's packed format
- History tracking and diff operations with zero-copy comparisons
- Validation and error handling with schema evolution support

## Core Test Categories

### 1. Schema and Dataset Management (`dataset_test.go`, `schema_test.go`)

**Key Behaviors:**
- Cap'n Proto schema compilation from `.capnp` files with field numbering
- Schema equality comparison ignoring field order but respecting Cap'n Proto evolution rules
- Schema validation for required fields, naming rules, and Cap'n Proto constraints
- Dataset identity checking across different schema arrangements
- Schema parsing from both Cap'n Proto definitions and legacy DSL for backward compatibility
- Topological sorting of schema dependencies considering Cap'n Proto imports

**Test Scenarios:**
- `TestDataset_IdenticalSchemas`: Validates that datasets with same schemas in different orders are considered identical
- `TestSchema_Validation`: Ensures empty/null schema names are rejected and Cap'n Proto rules are enforced
- `TestSchemaParser_ParseCapnProto`: Tests Cap'n Proto schema compilation with field numbering and evolution
- `TestSchemaParser_ParseCollection`: Tests legacy schema parsing for backward compatibility
- `TestSchemaSorter_TopologicalSort`: Verifies dependency-based schema ordering with Cap'n Proto imports

### 2. Producer Functionality (`producer_test.go`)

**Key Behaviors:**
- Cycle-based data publishing with version management
- Empty cycles return version 0 (no publish)
- Identical data cycles return same version number
- Single producer enforcement with primary/secondary modes
- State restoration from previous versions
- Validation pipeline with rollback on failure
- Type resharding based on data volume

**Test Scenarios:**
- `TestProducer_BasicCycle`: Core producer cycle behavior
- `TestProducer_SingleProducerEnforcement`: Primary producer validation
- `TestProducer_ValidationFailure`: Error handling and state rollback
- `TestProducer_TypeResharding`: Automatic sharding based on data size

### 3. Consumer Functionality (`consumer_test.go`)

**Key Behaviors:**
- Version-based data consumption with refresh triggers
- Delta chain traversal for incremental updates
- Reverse delta navigation for version rollback
- Automatic updates via announcement mechanisms
- Version pinning for controlled updates
- Type filtering for selective data loading
- Memory mode compatibility constraints

**Test Scenarios:**
- `TestConsumer_DeltaTraversal`: Multi-step delta chain following
- `TestConsumer_ReverseDeltas`: Backward navigation through versions
- `TestConsumer_AnnouncementWatcher`: Automatic update subscriptions
- `TestConsumer_PinnedAnnouncement`: Version pinning and unpinning
- `TestConsumer_TypeFiltering`: Selective type loading

### 4. Collection Types (`collections_test.go`)

**Key Behaviors:**
- HollowSet: Unordered unique element collections with containment checks
- HollowMap: Key-value mappings with entry iteration and equality
- HollowList: Ordered element sequences with indexed access
- Zero ordinal handling for all collection types
- State invalidation exception handling

**Test Scenarios:**
- `TestHollowSet_Basic`: Set operations (add, contains, iterate)
- `TestHollowMap_Equals`: Map equality and hash code consistency
- `TestHollowList_Basic`: List access patterns and ordering
- `TestCollections_StateInvalidation`: Stale reference exception handling

### 5. Indexing and Querying (`index_test.go`)

**Key Behaviors:**
- Hash indexes for multi-field lookups with collision handling
- Unique key indexes for single-result queries
- Primary key indexes for composite key lookups
- Null value handling in indexed fields
- Index updates during state transitions
- Byte array field indexing

**Test Scenarios:**
- `TestHashIndex_BasicFunctionality`: Multi-field hash index queries
- `TestHashIndex_NullValues`: Null value indexing and retrieval
- `TestUniqueKeyIndex_Basic`: Single-result unique lookups
- `TestPrimaryKeyIndex_Basic`: Composite key indexing
- `TestIndex_Updates`: Index maintenance during state changes

### 6. Write State Engine (`write_engine_test.go`)

**Key Behaviors:**
- Header tag management across versions and deltas
- Type resharding with shard count tracking
- Empty type snapshot handling
- State reset and rollback mechanisms
- Missing hash key error detection
- Negative number serialization support

**Test Scenarios:**
- `TestWriteStateEngine_HeaderTags`: Tag propagation through deltas
- `TestWriteStateEngine_Resharding`: Automatic shard count adjustment
- `TestWriteStateEngine_ResetToLastPrepared`: State rollback functionality
- `TestWriteStateEngine_NegativeFloat`: Special value handling

### 7. Tools and Utilities (`tools_test.go`)

**Key Behaviors:**
- Data diffing between state versions
- Schema change detection
- Checksum calculation for data integrity
- History tracking with operation logging
- Data combination from multiple sources
- Blob filtering by type
- Record stringification (JSON and pretty print)
- Field-based query matching

**Test Scenarios:**
- `TestHollowDiff_Basic`: Version-to-version data comparison
- `TestHollowHistory_KeyIndex`: Record lifecycle tracking
- `TestHollowCombiner_Basic`: Multi-source data merging
- `TestStringifier_Basic`: Record serialization formats

## Error Handling Patterns

The test suite validates several critical error conditions:

1. **Validation Failures**: Producers must rollback state when validation fails
2. **State Invalidation**: Collections must throw exceptions when accessing stale state
3. **Schema Violations**: Empty names and invalid references are rejected
4. **Primary Key Conflicts**: Missing hash keys in write operations are detected
5. **Memory Mode Conflicts**: Type filtering is incompatible with shared memory mode

## Performance and Scalability Features

The tests verify key performance-related behaviors:

1. **Type Resharding**: Automatic shard count adjustment based on data volume
2. **Delta Chains**: Efficient incremental updates without full snapshots
3. **Indexing**: Fast lookups through hash and unique key indexes
4. **Blob Filtering**: Selective type loading to reduce memory usage
5. **Concurrent Access**: Thread-safe operations across producer/consumer pairs

## Data Integrity Guarantees

Several tests ensure data consistency:

1. **Versioning**: Monotonic version numbers with deterministic behavior
2. **Checksums**: Data integrity verification across serialization
3. **Schema Evolution**: Backward compatibility with field additions/removals
4. **Transactional Updates**: All-or-nothing cycle completion

## Integration Patterns

The test suite demonstrates several integration patterns:

1. **Producer-Consumer**: Decoupled data publication and consumption
2. **Multi-Version**: Concurrent access to different data versions
3. **Announcement-Driven**: Event-based update propagation
4. **Validation Pipeline**: Pluggable validation with failure handling
5. **History Tracking**: Audit trail for data changes over time

## Memory Management

Memory-related test behaviors include:

1. **Shared vs Private Memory**: Mode-specific feature availability
2. **State Cleanup**: Proper resource deallocation on state transitions
3. **Large Object Handling**: Support for variable-length fields and collections
4. **Ordinal Management**: Efficient sparse data representation

## Type System Features

The schema and type system tests cover:

1. **Primitive Types**: Int, Float, String, Boolean, Bytes
2. **Reference Types**: Cross-type relationships and dependencies  
3. **Collection Types**: Sets, Lists, Maps with generic element types
4. **Schema Evolution**: Field addition, removal, and type changes
5. **Validation Rules**: Name constraints and structural requirements

## Cap'n Proto Integration Benefits

The integration with Cap'n Proto provides several key advantages over the original Java implementation:

### Performance Improvements
1. **Zero-Copy Deserialization**: Direct memory access without parsing overhead, delivering theoretically infinite deserialization speed
2. **Efficient Memory Layout**: Cap'n Proto's struct-like arrangement optimizes CPU cache usage
3. **Arena Allocation**: Batch memory management reduces GC pressure and improves locality
4. **Memory Mapping**: Large datasets can be accessed via mmap without loading entire files into memory

### Schema Evolution
1. **Field Numbering**: Cap'n Proto's explicit field numbering ensures stable schema evolution
2. **Backward Compatibility**: New fields are added to struct endings, maintaining compatibility
3. **Bounds Checking**: Automatic handling of missing fields in older schema versions
4. **Type Safety**: Compile-time validation of schema changes

### Serialization Advantages
1. **Platform Independence**: Consistent byte-for-byte encoding across architectures
2. **Packed Format**: Efficient compression removing zero bytes while maintaining speed
3. **Position Independence**: Offset-based pointers enable message sharing across processes
4. **Little-Endian**: Optimized for modern CPU architectures

### Development Experience
1. **Code Generation**: Automatic generation of type-safe Go structs from schemas
2. **Tooling Integration**: Standard Cap'n Proto toolchain and ecosystem
3. **Cross-Language Support**: Compatibility with other Cap'n Proto implementations
4. **Incremental Reads**: Access specific fields without parsing entire messages

These benefits make go-hollow not just a port of Hollow to Go, but a next-generation implementation that leverages modern serialization technology for superior performance and developer experience.

## Test Modifications Rationale

The following test modifications were made to incorporate Cap'n Proto integration:

1. **Schema Tests**: Added `TestSchemaParser_ParseCapnProto` to test Cap'n Proto schema compilation alongside legacy DSL parsing for backward compatibility.

2. **Type Definitions**: Updated type definitions to reflect Cap'n Proto field numbering and evolution constraints.

3. **Performance Expectations**: Updated performance benchmarks to reflect zero-copy capabilities, with deserialization speed targets adjusted for zero-copy access patterns.

4. **Memory Management**: Enhanced memory management tests to validate arena allocation and mmap support.

5. **Error Handling**: Added tests for Cap'n Proto specific error conditions like schema evolution conflicts and field bounds checking.

These modifications ensure the test suite validates both the core Hollow behaviors and the additional capabilities provided by Cap'n Proto integration.

## Conclusion

This comprehensive test suite ensures the Go implementation maintains feature parity with the Java version while following idiomatic Go patterns and conventions. The Cap'n Proto integration elevates performance and provides a superior foundation for high-performance, read-optimized data systems.
