# Go-Hollow

A high-performance Go implementation of Netflix's Hollow framework with Cap'n Proto integration for zero-copy data transfer.

## What is Go-Hollow?

Go-Hollow is a Go port of Netflix's Hollow framework designed for efficiently managing, distributing, and consuming in-memory datasets. It enables applications to share a consistent in-memory dataset across multiple instances with minimal latency and memory overhead.

Key features:

- **Zero-copy data transfer** using Cap'n Proto serialization for minimal memory overhead
- **Primary key-based delta optimization** - only changed records are transmitted (up to 37.5% storage savings)
- **Intelligent delta chains** with automatic deduplication and efficient compression
- **True incremental updates** - consumers efficiently traverse delta chains without full snapshots
- **Type-safe generic collections** for easy data access
- **Concurrent-safe by design** with proper synchronization primitives
- **Flexible storage backends** including memory and S3
- **Schema evolution support** for backward compatibility

## Quick Start

### Installation

```bash
go get github.com/leowmjw/go-hollow
```

### Basic Usage

1. Define your data schema (using Cap'n Proto):

```capnp
@0xabcdef1234567890;

struct Person {
  id @0 :UInt32;
  name @1 :Text;
  email @2 :Text;
}
```

2. Create a producer with primary key support for efficient deltas:

```go
package main

import (
    "context"
    "github.com/leowmjw/go-hollow/producer"
    "github.com/leowmjw/go-hollow/blob"
    "github.com/leowmjw/go-hollow/internal"
)

func main() {
    // Initialize a blob store and announcer
    store := blob.NewMemoryBlobStore()
    announcer := blob.NewGoroutineAnnouncer()
    defer announcer.Close()
    
    // Create a producer with primary key support for efficient deltas
    p := producer.NewProducer(
        producer.WithBlobStore(store),
        producer.WithAnnouncer(announcer),
        producer.WithPrimaryKey("Person", "id"), // Enable delta efficiency
    )
    
    // Run initial cycle to publish data
    version1, err := p.RunCycle(context.Background(), func(state *internal.WriteState) {
        // Add initial data
        state.Add(Person{ID: 1, Name: "Alice", Email: "alice@example.com"})
        state.Add(Person{ID: 2, Name: "Bob", Email: "bob@example.com"})
    })
    
    // Run delta cycle - only changed data is stored!
    version2, err := p.RunCycle(context.Background(), func(state *internal.WriteState) {
        // Update existing person (delta will contain only the change)
        state.Add(Person{ID: 1, Name: "Alice Smith", Email: "alice.smith@example.com"})
        // Add new person
        state.Add(Person{ID: 3, Name: "Charlie", Email: "charlie@example.com"})
        // Person ID 2 removed by omission (delta optimization)
    })
    
    fmt.Printf("Published version %d (snapshot) and %d (efficient delta)\n", version1, version2)
}
```

3. Create a consumer with zero-copy support:

```go
package main

import (
    "context"
    "github.com/leowmjw/go-hollow/consumer"
    "github.com/leowmjw/go-hollow/blob"
    "github.com/leowmjw/go-hollow/internal"
)

func main() {
    // Initialize a blob store
    store := blob.NewMemoryBlobStore()
    announcer := blob.NewGoroutineAnnouncer()
    defer announcer.Close()
    
    // Create a consumer with zero-copy support
    c := consumer.NewZeroCopyConsumerWithOptions(
        []consumer.ConsumerOption{
            consumer.WithBlobRetriever(store),
            consumer.WithAnnouncementWatcher(announcer),
        },
        []consumer.ZeroCopyConsumerOption{
            consumer.WithZeroCopySerializationMode(internal.ZeroCopyMode),
        },
    )
    
    // Efficiently refresh to latest version (delta chain traversal)
    // Note: version2 comes from the producer example above
    err := c.TriggerRefreshToWithZeroCopy(context.Background(), version2)
    if err != nil {
        panic(err)
    }
    
    // Access data with zero-copy efficiency
    data := c.GetDataWithZeroCopyPreference()
    fmt.Printf("Consumed %d records with zero-copy efficiency\n", len(data))
}
```

## CLI Usage

Go-Hollow includes a command-line tool for schema validation, data inspection, and interactive data operations.

### Basic Commands

```bash
# Show help
./hollow-cli help

# Validate a schema file
./hollow-cli schema -data=fixtures/schema/test_schema.capnp -verbose

# Start interactive producer session with test data
./hollow-cli produce -data=fixtures/test_data.json -store=memory
```

### Interactive Mode

The `produce` command opens an interactive session where you can perform multiple operations:

```bash
# Start interactive session
./hollow-cli produce -data=fixtures/test_data.json -store=memory

# Available interactive commands:
# c <version>      - Consume data from specific version
# i <version>      - Inspect specific version  
# d <from> <to>    - Diff between two versions
# p [type] [count] - Produce additional data
# l [blobs]        - List versions or detailed blob info
# q                - Quit
```

Example interactive session:
```
Version: 1

=== Interactive Mode (Memory Storage) ===
Data is now available in memory. Current version: 1

hollow> c 1
Consumer successfully refreshed to version: 1
State engine ready with version: 1

hollow> i 1  
Snapshot blob for version 1:
  Type: 0
  Version: 1
  Data size: 172 bytes

hollow> p String 3
New version produced: 2

hollow> d 1 2
Diff from version 1 to 2:
Changes: 1

hollow> q
Goodbye!
```

### Schema Validation

The CLI can validate Cap'n Proto schema files:

```bash
# Validate a schema file
./hollow-cli schema -data=fixtures/schema/test_schema.capnp -verbose
```

Example output:
```
Validating schema file: fixtures/schema/test_schema.capnp
Schema Person validated successfully
Schema Address validated successfully
Schema Role validated successfully
Schema validation passed for 3 schemas

Schema: Address (Type: 0)
  Fields: 5
    street (primitive)
    city (primitive)
    state (primitive)
    zipCode (primitive)
    country (primitive)
```

### Testing the CLI

You can test all CLI features using the provided test scripts:

```bash
# Run a comprehensive interactive demo
./fixtures/demo_cli.sh

# Run automated feature tests
./fixtures/test_cli_features.sh
```

The demo script showcases:
- Producer-consumer workflow with delta evolution
- Data inspection and version comparison
- Zero-copy integration with graceful fallback
- Realistic data operations (add, update, delete, mixed)

## Contributing

We welcome contributions to Go-Hollow! Here's how to get started:

### Setup Development Environment

1. **Fork and clone the repository**:
   ```bash
   git clone https://github.com/yourusername/go-hollow.git
   cd go-hollow
   ```

2. **Install dependencies**:
   ```bash
   go mod download
   ```

3. **Run tests**:
   ```bash
   go test ./...
   ```

### Development Workflow

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** and add tests for new functionality

3. **Run tests and linters**:
   ```bash
   go test ./...
   golangci-lint run
   ```

4. **Submit a pull request** with a clear description of your changes

### Contribution Guidelines

- Write clear, documented code with proper error handling
- Add tests for new functionality
- Follow Go best practices and code style
- Keep pull requests focused on a single topic

## License

MIT License - See LICENSE file for details.

## Acknowledgments

- Inspired by [Netflix Hollow](https://github.com/Netflix/hollow)
- Uses [Cap'n Proto](https://capnproto.org/) for efficient serialization
