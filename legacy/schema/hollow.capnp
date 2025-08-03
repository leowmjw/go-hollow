@0xc52a41fc40e83404;  # Unique file ID generated for this schema

# Schema for Hollow data structures
# This defines the structure for snapshots and deltas

struct Value {
  union {
    null @0 :Void;
    boolean @1 :Bool;
    integer @2 :Int64;
    float @3 :Float64;
    string @4 :Text;
    bytes @5 :Data;
    array @6 :List(Value);
    object @7 :Map;
  }
}

struct MapEntry {
  key @0 :Text;
  value @1 :Value;
}

struct Map {
  entries @0 :List(MapEntry);
}

struct Snapshot {
  version @0 :UInt64;
  data @1 :Map;
  timestamp @2 :Int64;  # Unix timestamp in milliseconds
}

struct DeltaEntry {
  key @0 :Text;
  operation @1 :Operation;
  value @2 :Value;  # Only used for add and change operations
}

enum Operation {
  add @0;
  remove @1;
  change @2;
}

struct Delta {
  fromVersion @0 :UInt64;
  toVersion @1 :UInt64;
  timestamp @2 :Int64;  # Unix timestamp in milliseconds
  entries @3 :List(DeltaEntry);
}
