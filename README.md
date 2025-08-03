# Go-Hollow

A high-performance Go implementation of Netflix's Hollow framework with Cap'n Proto integration for zero-copy data transfer.

## What is Go-Hollow?

Go-Hollow is a Go port of Netflix's Hollow framework designed for efficiently managing, distributing, and consuming in-memory datasets. It enables applications to share a consistent in-memory dataset across multiple instances with minimal latency and memory overhead.

Key features:

- **Zero-copy data transfer** using Cap'n Proto serialization
- **Efficient delta-based updates** to minimize network and memory usage
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

2. Create a producer to populate and publish data:

```go
package main

import (
    "context"
    "github.com/leowmjw/go-hollow/producer"
    "github.com/leowmjw/go-hollow/blob"
)

func main() {
    // Initialize a blob store
    store := blob.NewMemoryBlobStore()
    
    // Create a producer
    p := producer.NewProducer(store)
    
    // Run a cycle to publish data
    version, err := p.RunCycle(context.Background(), func(state *producer.WriteState) {
        // Add data to the state
        state.Add("Person", personData)
    })
    
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Published version: %d\n", version)
}
```

3. Create a consumer to read the data:

```go
package main

import (
    "context"
    "github.com/leowmjw/go-hollow/consumer"
    "github.com/leowmjw/go-hollow/blob"
)

func main() {
    // Initialize a blob store
    store := blob.NewMemoryBlobStore()
    
    // Create a consumer
    c := consumer.NewConsumer(store)
    
    // Initialize the consumer with the latest version
    err := c.RefreshTo(context.Background(), consumer.LatestVersion)
    if err != nil {
        panic(err)
    }
    
    // Access the data
    state := c.GetCurrentState()
    people := state.GetAll("Person")
    
    // Process the data
    for _, person := range people {
        // Use the data
        fmt.Printf("Person: %v\n", person)
    }
}
```

## CLI Usage

Go-Hollow includes a command-line tool for schema validation, data inspection, and more.

### Basic Commands

```bash
# Show help
./hollow-cli help

# Validate a schema file
./hollow-cli schema -data=fixtures/schema/test_schema.capnp -verbose

# Run a producer cycle with test data
./hollow-cli produce -data=data.json -store=memory

# Consume data from a specific version
./hollow-cli consume -version=1 -verbose

# Inspect a specific version
./hollow-cli inspect -store=memory -version=1

# Compare two versions
./hollow-cli diff -store=memory -version=1 -target=2
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

Schema: Person (Type: 0)
  Fields: 7
    id (reference to @0)
    name (reference to @1)
    email (reference to @2)
    ...
```

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
