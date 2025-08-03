# Cap'n Proto Schemas

This directory contains Cap'n Proto schema definitions for go-hollow's data structures and protocols.

## Schema Files

- **`common.capnp`** - Common types and utilities shared across all schemas
- **`delta.capnp`** - 🆕 Delta serialization schema for efficient change tracking
- **`movie_dataset.capnp`** - Movie catalog schema for examples
- **`commerce_dataset.capnp`** - E-commerce schema for examples
- **`iot_dataset.capnp`** - IoT metrics schema for examples

## Delta Schema

The `delta.capnp` schema enables efficient delta serialization with Cap'n Proto integration:

### Key Features
- **Typed Operations**: Add, Update, Delete operations for each record
- **Compressed Storage**: Cap'n Proto packed encoding for network efficiency
- **Metadata Tracking**: Change counts and optimization flags
- **Zero-Copy Access**: Direct buffer access without deserialization overhead

### Schema Structure
```capnp
struct DeltaSet {
  version @0 :UInt64;           # Target version
  fromVersion @1 :UInt64;       # Source version  
  deltas @2 :List(TypeDelta);   # Changes per type
  optimized @3 :Bool;           # Whether optimized
  changeCount @4 :UInt32;       # Total changes
}

struct TypeDelta {
  typeName @0 :Text;            # Type name (e.g., "Customer")
  records @1 :List(DeltaRecord); # Change records
}

struct DeltaRecord {
  operation @0 :DeltaOperation; # Add/Update/Delete
  ordinal @1 :UInt32;           # Record position
  value @2 :Data;               # Serialized data (nil for deletes)
}
```

## Generating Bindings

To regenerate language bindings after schema changes:

```bash
# Generate all language bindings
./tools/gen-schema.sh all

# Generate only Go bindings
./tools/gen-schema.sh go
```

## Schema Evolution Guidelines

1. **Never renumber field IDs** - Always append new fields
2. **Reserve field ranges** - Use ID >= 128 for extensibility  
3. **Deprecate, don't delete** - Leave comments for removed fields
4. **Test compatibility** - Verify old consumers work with new schemas
5. **Version schemas** - Tag schemas with semantic versions

## Integration Points

- **Producer**: Uses delta schema for efficient incremental updates
- **Consumer**: Deserializes delta chains for zero-copy access
- **Blob Store**: Stores both snapshots and delta blobs
- **CLI Tools**: Inspect and validate delta structures
