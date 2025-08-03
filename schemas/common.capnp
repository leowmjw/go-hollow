@0xd1a2b3c4e5f60718;

using Go = import "std/go.capnp";
$Go.package("common");
$Go.import("github.com/leowmjw/go-hollow/generated/go/common");

# Common types and utilities shared across all schemas

# Standard timestamp type
struct Timestamp {
  unixSeconds @0 :UInt64;
  nanos @1 :UInt32;
}

# Common status enums
enum Status {
  active @0;
  inactive @1;
  pending @2;
  deleted @3;
}

# Quality score (0-100)
struct QualityScore {
  value @0 :UInt8;  # 0-100
}

# Geographic location
struct Location {
  latitude @0 :Float64;
  longitude @1 :Float64;
  address @2 :Text;
}

# Money representation (to avoid floating point issues)
struct Money {
  amountCents @0 :UInt64;  # Amount in smallest currency unit
  currency @1 :Text;       # ISO 4217 currency code
}
