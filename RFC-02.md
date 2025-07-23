# RFC-02: Adopt Flatbuffers/Cap'n Proto for Snapshot/Delta Payloads

## Objective

This RFC evaluates replacing the current JSON-encoded `map[string]any` snapshot/delta payloads with a more performant and strongly-typed binary serialization format. The primary goals are to achieve good read latency, minimize allocations, and provide strong schema typing, all while maintaining Hollow's read-only semantics.

## Priorities

The evaluation is guided by the following priorities:

1.  **Optimize for readability and ease of operation:** The chosen solution should be easy to work with and understand.
2.  **No CGO:** The implementation must be pure Go and not rely on C bindings.
3.  **Schema versioning and transformation:** The solution must support evolving the data schema over time without breaking existing clients.

## Technical Comparison

This section compares two leading contenders: [Flatbuffers](https://google.github.io/flatbuffers/) and [Cap'n Proto](https://capnproto.org/). Both are "zero-copy" serialization libraries, meaning they access data directly from the wire format without a separate parsing or unpacking step. This makes them extremely fast for reads.

| Feature                 | Flatbuffers                                                                                             | Cap'n Proto                                                                                                 |
| ----------------------- | ------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| **Read Latency**        | Excellent. Zero-copy access.                                                                            | Excellent. Zero-copy access.                                                                                |
| **Allocations**         | Minimal. Accessors are generated to read data directly from the underlying byte buffer.                 | Minimal. Similar to Flatbuffers, it avoids allocations on read.                                             |
| **Schema Typing**       | Strong. Schemas are defined in an IDL (`.fbs`), and the compiler generates Go code.                       | Strong. Schemas are defined in an IDL (`.capnp`), and the compiler generates Go code.                       |
| **Schema Evolution**    | Good. Supports adding new fields to tables. Deprecating fields is also possible.                        | Excellent. Designed for robust schema evolution, allowing for easy addition and deprecation of fields.      |
| **CGO Usage**           | The official Go implementation is pure Go.                                                              | The official Go implementation (`go-capnproto2`) is pure Go.                                                |
| **Readability/Ease of Use** | The generated Go API is generally considered straightforward, though slightly more verbose than Cap'n Proto's. | The generated Go API is often cited as being more ergonomic and closer to native Go structs.                 |
| **Performance (Go)**    | Very fast. Benchmarks show it has a slight edge in encoding speed over Cap'n Proto.                     | Extremely fast. Benchmarks indicate it has a significant advantage in decoding speed over Flatbuffers.      |

### Performance Insights

Based on community benchmarks (e.g., `kcchu/buffer-benchmarks`), Cap'n Proto's Go implementation often shows significantly faster decoding times compared to Flatbuffers. Given that Hollow's use case is read-heavy, this gives Cap'n Proto a notable advantage.

## Implementation Examples

### Flatbuffers Example

**Schema (`schema.fbs`):**

```fbs
namespace MyProject;

table Snapshot {
  id:long;
  timestamp:long;
  data:[ubyte];
}

root_type Snapshot;
```

**Go Implementation:**

```go
package main

import (
	"fmt"

	flatbuffers "github.com/google/flatbuffers/go"
	"MyProject"
)

func main() {
	builder := flatbuffers.NewBuilder(1024)

	// Create the data payload
	data := builder.CreateByteVector([]byte("some payload"))

	// Create the snapshot
	MyProject.SnapshotStart(builder)
	MyProject.SnapshotAddId(builder, 12345)
	MyProject.SnapshotAddTimestamp(builder, 987654321)
	MyProject.SnapshotAddData(builder, data)
	snapshotOffset := MyProject.SnapshotEnd(builder)

	builder.Finish(snapshotOffset)

	buf := builder.FinishedBytes()

	// Read the snapshot
	snapshot := MyProject.GetRootAsSnapshot(buf, 0)

	fmt.Printf("ID: %d\n", snapshot.Id())
	fmt.Printf("Timestamp: %d\n", snapshot.Timestamp())
	fmt.Printf("Data: %s\n", string(snapshot.DataBytes()))
}
```

### Cap'n Proto Example

**Schema (`schema.capnp`):**

```capnp
@0x...; # Unique file ID

struct Snapshot {
  id @0 :Int64;
  timestamp @1 :Int64;
  data @2 :Data;
}
```

**Go Implementation:**

```go
package main

import (
	"context"
	"fmt"

	"capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/rpc"
	"path/to/generated/schema"
)

func main() {
	// Create a new message
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		panic(err)
	}

	// Create a new Snapshot
	snapshot, err := schema.NewRootSnapshot(seg)
	if err != nil {
		panic(err)
	}
	snapshot.SetId(12345)
	snapshot.SetTimestamp(987654321)
	snapshot.SetData([]byte("some payload"))

	// Read the snapshot
	fmt.Printf("ID: %d\n", snapshot.Id())
	fmt.Printf("Timestamp: %d\n", snapshot.Timestamp())
	data, err := snapshot.Data()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Data: %s\n", string(data))
}
```

## Recommendation

**I recommend adopting Cap'n Proto.**

While both Flatbuffers and Cap'n Proto meet the core requirements, Cap'n Proto has a few key advantages for this specific use case:

1.  **Superior Read Performance:** In a read-heavy system like Hollow, Cap'n Proto's faster decoding speeds are a significant benefit.
2.  **Ergonomic API:** The generated Go code for Cap'n Proto is generally considered more pleasant to work with, which aligns with the priority of optimizing for readability and ease of operation.
3.  **Strong Schema Evolution:** Cap'n Proto's design principles for schema evolution are robust and well-documented.

Given these factors, Cap'n Proto is the stronger choice for replacing the current JSON-based payloads.
