@0xe8f7a6b5c4d3e2f1;

using Go = import "std/go.capnp";
$Go.package("delta");
$Go.import("github.com/leowmjw/go-hollow/generated/go/delta");

# Delta serialization schema for efficient change tracking

enum DeltaOperation {
  add @0;
  update @1;
  delete @2;
}

struct DeltaRecord {
  operation @0 :DeltaOperation;
  ordinal @1 :UInt32;
  value @2 :Data;  # Serialized record data (nil for deletes)
}

struct TypeDelta {
  typeName @0 :Text;
  records @1 :List(DeltaRecord);
}

struct DeltaSet {
  version @0 :UInt64;
  fromVersion @1 :UInt64;
  deltas @2 :List(TypeDelta);
  optimized @3 :Bool = false;  # Whether this delta has been optimized
  changeCount @4 :UInt32 = 0;  # Total number of changes
}

# Delta metadata for efficiency analysis
struct DeltaMetadata {
  deltaSet @0 :DeltaSet;
  compressionRatio @1 :Float32;
  serializedSize @2 :UInt32;
  changeEfficiency @3 :Float32;  # Ratio of changes to total data
}
