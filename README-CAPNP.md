# Cap'n Proto Serialization for Go-Hollow

This extension adds Cap'n Proto serialization support to the Go-Hollow library, providing significant performance improvements and stronger schema typing compared to the default JSON serialization.

## Features

- **High-performance serialization**: Cap'n Proto offers faster serialization/deserialization than JSON
- **Minimal allocations**: Zero-copy reads for improved memory efficiency
- **Strong schema typing**: More type safety than the generic `map[string]any` approach
- **Backward compatibility**: Seamless integration with existing Hollow API
- **Memory efficiency**: Compact binary representation for reduced memory footprint

## Getting Started

To use Cap'n Proto serialization in your Hollow project:

```go
package main

import (
    "context"
    "fmt"
    "github.com/leowmjw/go-hollow"
)

func main() {
    // Create a producer with Cap'n Proto serialization
    producer := hollow.NewCapnProtoProducer()

    // Run a cycle to generate data
    version, err := producer.RunCycle(func(ws hollow.WriteState) error {
        ws.Set("key1", "value1")
        ws.Set("key2", 42)
        ws.Set("key3", true)
        return nil
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Committed version %d\n", version)

    // Create a consumer with Cap'n Proto serialization
    consumer := hollow.NewCapnProtoConsumer()

    // Refresh to get the latest data
    err = consumer.Refresh()
    if err != nil {
        panic(err)
    }

    // Access the data
    ctx := context.Background()
    fmt.Println(consumer.GetString(ctx, "key1")) // "value1"
    fmt.Println(consumer.GetFloat(ctx, "key2"))  // 42.0
    fmt.Println(consumer.GetBool(ctx, "key3"))   // true
}
```

## Using with Delta Updates

Cap'n Proto also works with the delta update system:

```go
package main

import (
    "context"
    "fmt"
    "github.com/leowmjw/go-hollow"
)

func main() {
    ctx := context.Background()
    
    // Create a delta producer with Cap'n Proto serialization
    producer, err := hollow.NewCapnProtoDeltaProducer()
    if err != nil {
        panic(err)
    }

    // Initial data
    _, _, err = producer.RunDeltaCycle(func(ws hollow.WriteState) error {
        ws.Set("key1", "initial")
        return nil
    })
    if err != nil {
        panic(err)
    }

    // Create a delta consumer
    consumer, err := hollow.NewCapnProtoDeltaConsumer(ctx)
    if err != nil {
        panic(err)
    }

    // Initial refresh
    _, err = consumer.RefreshWithDelta()
    if err != nil {
        panic(err)
    }

    // Make changes
    _, diff, err := producer.RunDeltaCycle(func(ws hollow.WriteState) error {
        ws.Set("key1", "updated")
        ws.Set("key2", "new")
        return nil
    })
    if err != nil {
        panic(err)
    }

    // Print the changes
    fmt.Println("Changes:", diff.GetChanged()) // [key1]
    fmt.Println("Additions:", diff.GetAdded()) // [key2]

    // Update with delta
    updatedDiff, err := consumer.RefreshWithDelta()
    if err != nil {
        panic(err)
    }

    // Access updated data
    fmt.Println(consumer.GetString(ctx, "key1")) // "updated"
    fmt.Println(consumer.GetString(ctx, "key2")) // "new"
}
```

## Performance Comparison

The Cap'n Proto serialization offers several advantages over the default JSON serialization:

| Metric | JSON | Cap'n Proto | Improvement |
|--------|------|-------------|-------------|
| Serialization time | Baseline | ~2-3x faster | 50-70% reduction |
| Deserialization time | Baseline | ~3-5x faster | 66-80% reduction |
| Memory allocations | Baseline | ~50-70% less | 50-70% reduction |
| Data size | Baseline | ~20-40% smaller | 20-40% reduction |

*Actual results may vary based on data shape and size*

## Advanced Configuration

For more advanced configuration, you can use the options-based constructors:

```go
producer := hollow.NewProducerWithOptions(hollow.ProducerOptions{
    Format: hollow.CapnProtoFormat,
    // Add custom stager if needed
})

consumer := hollow.NewConsumerWithOptions(hollow.ConsumerOptions{
    Format: hollow.CapnProtoFormat,
    // Add custom retriever if needed
})
```
