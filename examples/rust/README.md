# go-hollow Rust Examples

This directory contains Rust implementations of the three realistic scenarios described in [USAGE.md](../../USAGE.md).

## Prerequisites

- Rust 1.70 or later
- Cargo for dependency management
- Generated Cap'n Proto bindings (run `../../tools/gen-schema.sh rust`)

## Tech Stack

- **Rust**: Standard library (no async runtime dependencies like tokio)
- **Dependencies**: capnp crate for Cap'n Proto bindings
- **Storage**: aws-sdk-s3 with rustls for S3 integration
- **Concurrency**: Standard library threads and channels

## Installation

```bash
# Install Rust if not already installed
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Build project
cargo build
```

## Running Examples

### Scenario A: Movie Catalog

```bash
# Generate schemas first
cd ../../
./tools/gen-schema.sh rust

# Run movie catalog example
cd examples/rust
cargo run --bin movie
```

### Scenario B: Commerce Orders

```bash
cargo run --bin commerce
```

### Scenario C: IoT Metrics

```bash
cargo run --bin iot
```

## Example Structure

Each scenario demonstrates:

1. **Schema Loading**: Using capnp with generated bindings
2. **Producer Setup**: Rust wrapper for go-hollow producer
3. **Consumer Setup**: Thread-based consumer with channels
4. **Index Queries**: HashMap and BTreeMap for index operations
5. **CLI Integration**: std::process::Command for hollow-cli

## Files

- `src/bin/movie.rs` - Movie catalog scenario implementation
- `src/bin/commerce.rs` - Commerce orders scenario implementation  
- `src/bin/iot.rs` - IoT metrics scenario implementation
- `src/lib.rs` - Rust wrapper library for go-hollow
- `src/hollow/` - go-hollow Rust integration modules
- `Cargo.toml` - Rust project configuration
- `build.rs` - Build script for Cap'n Proto compilation

## Features

- **Memory Safety**: Zero-cost abstractions with compile-time guarantees
- **Performance**: Optimized for low-latency operations
- **Concurrency**: Standard library threads and channels (no async)
- **Error Handling**: Comprehensive Result types and error propagation

## Dependencies

```toml
[dependencies]
capnp = "0.18"
aws-sdk-s3 = { version = "1.0", features = ["rustls"] }
serde = { version = "1.0", features = ["derive"] }
csv = "1.3"

[build-dependencies]
capnpc = "0.18"
```

## TODO (Phase 6)

- Implement capnpc-rust schema generation
- Create Rust wrapper for go-hollow APIs
- Add thread-based announcer implementation
- Implement blob storage with aws-sdk-s3
- Add comprehensive error handling
- Performance optimizations for high-throughput scenarios
