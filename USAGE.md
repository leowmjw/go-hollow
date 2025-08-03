# go-hollow: Multi-language Usage Guide

go-hollow is a versioned, in-memory data system inspired by Netflix Hollow. This guide demonstrates realistic scenarios that exercise all capabilities across multiple programming languages using identical Cap'n Proto schemas.

## Quick Start

| Language   | Tech Stack          | Example Location    |
|------------|---------------------|---------------------|
| Go         | v1.24.5, stdlib, slog | `examples/go/`     |
| Python     | uv/uvx standalone   | `examples/python/`  |
| TypeScript | bun                 | `examples/typescript/` |
| Java       | Quarkus            | `examples/java/`    |
| Rust       | stdlib             | `examples/rust/`    |

## Installing Codegen Toolchain

1. Install Cap'n Proto compiler:
   ```bash
   # macOS
   brew install capnp
   
   # Ubuntu/Debian  
   apt-get install capnproto libcapnp-dev
   ```

2. Generate language bindings:
   ```bash
   ./tools/gen-schema.sh
   ```

## Core Concepts

go-hollow provides:
- **Versioned snapshots** with delta chains for efficient updates
- **Schema evolution** with backward/forward compatibility
- **Three index types**: Hash, Unique, Primary Key
- **Blob storage** with S3/MinIO support
- **Async announcements** for real-time updates
- **CLI tools** for inspection and debugging

## Scenario A: Movie Catalog

**Use Case**: Movie streaming service managing catalog with user ratings. Demonstrates schema evolution and index usage.

**Data Scale**: 100 movies, 1,000 ratings (small for testing)

### Schema v1
```capnp
# schemas/movie_dataset.capnp
struct Movie {
  id        @0 :UInt32;     # Unique identifier
  title     @1 :Text;       # Movie title
  year      @2 :UInt16;     # Release year
  genres    @3 :List(Text); # Genre tags
}

struct Rating {
  movieId   @0 :UInt32;     # References Movie.id
  userId    @1 :UInt32;     # User identifier  
  score     @2 :Float32;    # Rating 1.0-5.0
  timestamp @3 :UInt64;     # Unix timestamp
}
```

### Workflow

1. **Bootstrap Schema**
   ```bash
   # Generate language bindings
   ./tools/gen-schema.sh
   ```

2. **Producer Cycle**
   ```pseudocode
   producer = Producer(
       blobStore = S3("localhost:9000"),
       announcer = GoroutineAnnouncer()
   )
   
   version = producer.RunCycle() { writeState =>
       for movie in loadCSV("fixtures/movies.csv"):
           writeState.Add("Movie", movie)
       for rating in loadCSV("fixtures/ratings.csv"):
           writeState.Add("Rating", rating)
   }
   ```

3. **Consumer Setup**
   ```pseudocode
   consumer = Consumer(
       retriever = blobStore,
       watcher = announcer,
       typeFilter = ["Movie", "Rating"]
   )
   consumer.TriggerRefresh()
   
   # Create indexes
   movieIndex = UniqueKeyIndex(consumer.Engine, "Movie", ["id"])
   genreIndex = HashIndex(consumer.Engine, "Movie", ["genres"])
   ratingIndex = PrimaryKeyIndex(consumer.Engine, "Rating", ["movieId", "userId"])
   ```

4. **Query Examples**
   ```pseudocode
   # Find specific movie
   movie = movieIndex.GetMatch(12345)
   
   # Find all action movies
   actionMovies = genreIndex.GetMatches("Action")
   
   # Find user's rating for movie
   userRating = ratingIndex.GetMatch([12345, 67890])
   ```

5. **Schema Evolution to v2**
   ```capnp
   # Add new field (backward compatible)
   struct Movie {
     id        @0 :UInt32;
     title     @1 :Text;
     year      @2 :UInt16;
     genres    @3 :List(Text);
     runtimeMin @4 :UInt16;    # NEW: Runtime in minutes
   }
   ```

6. **CLI Inspection**
   ```bash
   # Inspect current state
   hollow-cli inspect -store s3 -version $version
   
   # Compare versions (shows new runtimeMin field)
   hollow-cli diff -store s3 -version $version -target $(($version-1))
   ```

## Scenario B: Commerce Orders

**Use Case**: E-commerce platform tracking customers and orders. Demonstrates multi-producer setup and type filtering.

**Data Scale**: 200 customers, 500 orders

### Schema v1
```capnp
# schemas/commerce_dataset.capnp
struct Customer {
  id       @0 :UInt32;     # Unique customer ID
  email    @1 :Text;       # Email (unique)
  name     @2 :Text;       # Full name
  city     @3 :Text;       # City for shipping
  age      @4 :UInt16;     # Age for analytics
}

struct Order {
  id         @0 :UInt32;   # Unique order ID
  customerId @1 :UInt32;   # References Customer.id
  amount     @2 :Float64;  # Order total
  status     @3 :Text;     # "pending", "shipped", "delivered"
  timestamp  @4 :UInt64;   # Order creation time
}
```

### Workflow

1. **Multi-Producer Setup**
   ```pseudocode
   # Primary producer (handles customers)
   primaryProducer = Producer(blobStore, announcer)
   
   # Secondary producer (handles orders)
   secondaryProducer = Producer(blobStore, announcer)
   ```

2. **Type-Filtered Consumer**
   ```pseudocode
   # Consumer only interested in customers
   customerConsumer = Consumer(
       retriever = blobStore,
       typeFilter = ["Customer"]  # Ignores Order updates
   )
   
   # Analytics consumer needs everything
   analyticsConsumer = Consumer(
       retriever = blobStore,
       typeFilter = ["Customer", "Order"]
   )
   ```

3. **Index Usage**
   ```pseudocode
   # Unique email lookup
   emailIndex = UniqueKeyIndex(consumer.Engine, "Customer", ["email"])
   customer = emailIndex.GetMatch("user@example.com")
   
   # Orders by city (for shipping analytics)
   cityIndex = HashIndex(consumer.Engine, "Customer", ["city"])
   nycCustomers = cityIndex.GetMatches("New York")
   
   # Customer-Order relationship
   orderIndex = PrimaryKeyIndex(consumer.Engine, "Order", ["customerId", "id"])
   customerOrders = orderIndex.GetMatches([customerId])
   ```

## Scenario C: IoT Metrics

**Use Case**: IoT platform collecting device metrics. Demonstrates high-throughput scenarios and memory management.

**Data Scale**: 50 devices, 10,000 metrics (1 day of data)

### Schema v1
```capnp
# schemas/iot_dataset.capnp
struct Device {
  id         @0 :UInt32;   # Device ID
  serial     @1 :Text;     # Serial number (unique)
  type       @2 :Text;     # "sensor", "actuator", "gateway"
  location   @3 :Text;     # Physical location
}

struct Metric {
  deviceId   @0 :UInt32;   # References Device.id
  type       @1 :Text;     # "temperature", "humidity", "pressure"
  value      @2 :Float64;  # Measured value
  timestamp  @3 :UInt64;   # Measurement time
  quality    @4 :UInt8;    # Data quality score 0-100
}
```

### Workflow

1. **High-Throughput Producer**
   ```pseudocode
   producer = Producer(
       blobStore = S3("production-bucket"),
       announcer = GoroutineAnnouncer()
   )
   
   # Batch metrics for efficiency
   version = producer.RunCycle() { writeState =>
       for device in loadCSV("fixtures/devices.csv"):
           writeState.Add("Device", device)
       
       # Process metrics in batches
       for metricBatch in loadMetricsInBatches("fixtures/metrics.csv", batchSize=1000):
           for metric in metricBatch:
               writeState.Add("Metric", metric)
   }
   ```

2. **Memory-Optimized Consumer**
   ```pseudocode
   consumer = Consumer(
       retriever = blobStore,
       memoryMode = SharedMmap,  # Memory-mapped for large datasets
       pinning = true            # Pin frequently accessed data
   )
   
   # Index for device lookup
   deviceIndex = UniqueKeyIndex(consumer.Engine, "Device", ["serial"])
   
   # Hash index for metric queries
   metricIndex = HashIndex(consumer.Engine, "Metric", ["deviceId", "type"])
   ```

3. **Real-time Queries**
   ```pseudocode
   # Find device by serial
   device = deviceIndex.GetMatch("DEV-12345")
   
   # Get latest temperature readings for device
   tempMetrics = metricIndex.GetMatches([deviceId, "temperature"])
   recentMetrics = filter(tempMetrics, timestamp > (now() - 3600))
   ```

4. **Blob Lifecycle Management**
   ```pseudocode
   # Pin current hour data in memory
   consumer.Pin(currentVersion)
   
   # Unpin old data to free memory
   consumer.Unpin(oldVersion)
   
   # Blob storage automatically tiers cold data
   ```

## Common CLI Commands

```bash
# List all available versions
hollow-cli versions -store s3

# Inspect specific version
hollow-cli inspect -store s3 -version 42 -verbose

# Compare two versions
hollow-cli diff -store s3 -version 42 -target 41

# Validate schema compatibility
hollow-cli validate -schema schemas/movie_dataset.capnp

# Export data for debugging
hollow-cli export -store s3 -version 42 -format json -output data.json
```

## Schema Evolution Best Practices

1. **Never renumber field IDs** - Always append new fields
2. **Reserve field ranges** - Use ID >= 128 for extensibility  
3. **Deprecate, don't delete** - Leave comments for removed fields
4. **Test compatibility** - Verify old consumers work with new schemas
5. **Version schemas** - Tag schemas with semantic versions

## Performance Tuning

### Memory Usage
- Use `SharedMmap` for large datasets
- Pin frequently accessed versions
- Unpin old versions to free memory
- Configure blob cache size appropriately

### Network Efficiency  
- Batch producer cycles for fewer snapshots
- Use delta chains to minimize transfer size
- Configure async announcer buffer sizes
- Enable blob compression in production

### Index Performance
- Choose appropriate index types for query patterns
- Hash indexes for equality lookups
- Unique indexes for single-value constraints
- Primary key indexes for composite lookups

## Phase 6 TODOs (Production Scale)

- **Realistic data volumes**: 1M+ records per scenario
- **Programmatic data generators**: Replace CSV files with generators
- **Performance benchmarks**: Measure throughput/latency across languages
- **Production deployment**: Kubernetes, monitoring, alerting
- **Advanced features**: Custom serialization, compression, encryption
