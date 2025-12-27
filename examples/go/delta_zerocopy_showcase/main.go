// Comprehensive example showcasing primary key-based delta optimization
// with zero-copy Cap'n Proto integration for maximum efficiency
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// Customer represents an e-commerce customer with primary key support
type Customer struct {
	ID    uint32 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	City  string `json:"city"`
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("=== Delta + Zero-Copy Efficiency Showcase ===")
	slog.Info("Demonstrating primary key-based delta optimization with zero-copy integration")

	// Setup infrastructure
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Step 1: Create producer with primary key support
	slog.Info("Creating producer with primary key support...")

	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		// Ensure zero-copy serialization so blobs advertise support via metadata
		producer.WithSerializationMode(internal.ZeroCopyMode),
		producer.WithPrimaryKey("Customer", "ID"), // Enable delta optimization
	)

	// Step 2: Create zero-copy consumer
	slog.Info("Creating zero-copy consumer...")

	cons := consumer.NewZeroCopyConsumerWithOptions(
		[]consumer.ConsumerOption{
			consumer.WithBlobRetriever(blobStore),
			consumer.WithAnnouncer(announcer),
		},
		[]consumer.ZeroCopyConsumerOption{
			consumer.WithZeroCopySerializationMode(internal.ZeroCopyMode),
		},
	)

	// Step 3: Initial large dataset
	slog.Info("Creating initial large dataset...")

	initialCustomers := generateCustomers(10000, 0) // 10K customers

	start := time.Now()
	version1, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		for _, customer := range initialCustomers {
			ws.Add(customer)
		}
	})
	if err != nil {
		slog.Error("Failed to create initial snapshot", "error", err)
		return
	}
	snapshotTime := time.Since(start)

	slog.Info("Initial snapshot created",
		"version", version1,
		"customers", len(initialCustomers),
		"time", snapshotTime)

	// Step 4: Consumer reads initial data with zero-copy
	slog.Info("Consumer reading initial data with zero-copy...")

	start = time.Now()
	if err := cons.TriggerRefreshToWithZeroCopy(ctx, int64(version1)); err != nil {
		slog.Error("Consumer refresh failed", "error", err)
		return
	}
	consumeTime := time.Since(start)

	data := cons.GetDataWithZeroCopyPreference()
	slog.Info("Initial data consumed",
		"records", len(data),
		"time", consumeTime,
		"zero_copy_support", cons.HasZeroCopySupport())

	// Step 5: Small delta update (0.5% change rate)
	slog.Info("Creating small delta update (0.5% change rate)...")

	// Update only 50 out of 10,000 customers (0.5% change rate)
	updatedCustomers := generateCustomers(50, 10000) // 50 new customers

	// Modify 50 existing customers
	for i := 0; i < 50; i++ {
		initialCustomers[i].Name = fmt.Sprintf("Updated_%s", initialCustomers[i].Name)
		initialCustomers[i].Email = fmt.Sprintf("updated_%s", initialCustomers[i].Email)
	}
	changedCustomers := initialCustomers[:50]

	start = time.Now()
	version2, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		// Re-add all customers, but primary key optimization will:
		// 1. Detect unchanged customers and skip them
		// 2. Store only the 50 changed + 50 new customers in delta
		// 3. Automatically detect deletions by omission

		for _, customer := range changedCustomers {
			ws.Add(customer) // Changed customers
		}
		for _, customer := range updatedCustomers {
			ws.Add(customer) // New customers
		}
		for i := 100; i < len(initialCustomers); i++ {
			ws.Add(initialCustomers[i]) // Unchanged customers (will be optimized out)
		}
	})
	if err != nil {
		slog.Error("Failed to create delta version", "error", err)
		return
	}
	deltaTime := time.Since(start)

	slog.Info("Delta created",
		"version", version2,
		"changed_customers", 50,
		"new_customers", 50,
		"total_customers", len(initialCustomers)+50,
		"change_rate", "0.5%",
		"time", deltaTime)

	// Step 6: Consumer efficiently processes delta
	slog.Info("Consumer processing delta with zero-copy...")

	start = time.Now()
	if err := cons.TriggerRefreshToWithZeroCopy(ctx, int64(version2)); err != nil {
		slog.Error("Consumer delta refresh failed", "error", err)
		return
	}
	deltaConsumeTime := time.Since(start)

	finalData := cons.GetDataWithZeroCopyPreference()
	slog.Info("Delta consumed",
		"final_records", len(finalData),
		"time", deltaConsumeTime,
		"delta_efficiency", fmt.Sprintf("%.1fx faster than snapshot", float64(consumeTime)/float64(deltaConsumeTime)))

	// Step 7: Demonstrate storage efficiency
	demonstrateStorageEfficiency(blobStore, int64(version1), int64(version2))

	// Step 8: Multiple delta cycles
	demonstrateMultipleDeltaCycles(ctx, prod, cons)

	slog.Info("=== Delta + Zero-Copy Showcase Complete ===")
	slog.Info("Key Benefits Demonstrated:")
	slog.Info("✓ Primary key-based delta optimization reduces storage and network usage")
	slog.Info("✓ Zero-copy deserialization minimizes memory overhead")
	slog.Info("✓ Delta chain traversal avoids full snapshot loading")
	slog.Info("✓ Automatic deduplication and change detection")
	slog.Info("✓ Efficient handling of large datasets with small change rates")
}

func generateCustomers(count int, startID uint32) []Customer {
	customers := make([]Customer, count)
	cities := []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose"}

	for i := 0; i < count; i++ {
		id := startID + uint32(i) + 1
		customers[i] = Customer{
			ID:    id,
			Name:  fmt.Sprintf("Customer_%d", id),
			Email: fmt.Sprintf("customer_%d@example.com", id),
			City:  cities[i%len(cities)],
		}
	}

	return customers
}

func demonstrateStorageEfficiency(blobStore blob.BlobRetriever, version1, version2 int64) {
	slog.Info("=== Storage Efficiency Analysis ===")

	// Get blob metadata for both versions
	snapshotBlob := blobStore.RetrieveSnapshotBlob(version1)
	if snapshotBlob == nil {
		slog.Error("Failed to get snapshot blob", "version", version1)
		return
	}

	// Delta is keyed by FromVersion
	deltaBlob := blobStore.RetrieveDeltaBlob(version1)
	if deltaBlob == nil {
		slog.Error("Failed to get delta blob", "fromVersion", version1, "toVersion", version2)
		return
	}

	snapshotSize := len(snapshotBlob.Data)
	deltaSize := len(deltaBlob.Data)

	slog.Info("Storage analysis",
		"snapshot_size", fmt.Sprintf("%d bytes", snapshotSize),
		"delta_size", fmt.Sprintf("%d bytes", deltaSize),
		"compression_ratio", fmt.Sprintf("%.1fx", float64(snapshotSize)/float64(deltaSize)),
		"storage_savings", fmt.Sprintf("%.1f%%", (1.0-float64(deltaSize)/float64(snapshotSize))*100))

	// Calculate theoretical savings for multiple consumers
	traditionalSize := snapshotSize * 2       // Both versions as full snapshots
	optimizedSize := snapshotSize + deltaSize // Snapshot + delta

	slog.Info("Network efficiency",
		"traditional_approach", fmt.Sprintf("%d bytes", traditionalSize),
		"delta_approach", fmt.Sprintf("%d bytes", optimizedSize),
		"network_savings", fmt.Sprintf("%.1f%%", (1.0-float64(optimizedSize)/float64(traditionalSize))*100))
}

func demonstrateMultipleDeltaCycles(ctx context.Context, prod *producer.Producer, cons *consumer.ZeroCopyConsumer) {
	slog.Info("=== Multiple Delta Cycles Demo ===")

	currentVersion := cons.GetCurrentVersion()

	// Run 5 small delta cycles
	for cycle := 1; cycle <= 5; cycle++ {
		slog.Info("Running delta cycle", "cycle", cycle)

		start := time.Now()
		version, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
			// Add a few new customers each cycle
			for i := 0; i < 10; i++ {
				customer := Customer{
					ID:    uint32(20000 + cycle*10 + i),
					Name:  fmt.Sprintf("Cycle%d_Customer_%d", cycle, i),
					Email: fmt.Sprintf("cycle%d_customer_%d@example.com", cycle, i),
					City:  "Delta City",
				}
				ws.Add(customer)
			}
		})
		if err != nil {
			slog.Error("Failed to create delta cycle", "cycle", cycle, "error", err)
			return
		}
		cycleTime := time.Since(start)

		// Consumer processes delta
		start = time.Now()
		if err := cons.TriggerRefreshToWithZeroCopy(ctx, int64(version)); err != nil {
			slog.Error("Delta cycle consumption failed", "cycle", cycle, "error", err)
			continue
		}
		consumeTime := time.Since(start)

		data := cons.GetDataWithZeroCopyPreference()

		slog.Info("Delta cycle complete",
			"cycle", cycle,
			"version", version,
			"total_records", len(data),
			"produce_time", cycleTime,
			"consume_time", consumeTime)
	}

	finalVersion := cons.GetCurrentVersion()
	deltaChainLength := finalVersion - currentVersion

	slog.Info("Multiple delta cycles complete",
		"cycles", 5,
		"delta_chain_length", deltaChainLength,
		"final_version", finalVersion,
		"note", "Consumer efficiently traversed entire delta chain")
}
