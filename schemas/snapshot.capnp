@0x9eb32e19f86ee174;

using Go = import "std/go.capnp";
$Go.package("snapshot");
$Go.import("github.com/leowmjw/go-hollow/generated/go/snapshot");

# Simplified snapshot schema for serialized data
struct Snapshot {
  types @0 :List(TypeData);
}

struct TypeData {
  typeName @0 :Text;
  records  @1 :List(Data);  # Raw serialized record data
}
