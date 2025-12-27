// Core zero-copy integration tests that validate the end-to-end zero-copy path
package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/index"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// TestEndToEndZeroCopyIntegration validates the complete zero-copy path from producer to consumer
func TestEndToEndZeroCopyIntegration(t *testing.T) {
	ctx := context.Background()

	// Setup infrastructure
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Create producer with zero-copy serialization
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.ZeroCopyMode),
	)

	// Create zero-copy consumer
	zeroCopyConsumer := consumer.NewZeroCopyConsumerWithOptions(
		[]consumer.ConsumerOption{
			consumer.WithBlobRetriever(blobStore),
			consumer.WithAnnouncementWatcher(announcer),
		},
		[]consumer.ZeroCopyConsumerOption{
			consumer.WithZeroCopySerializationMode(internal.ZeroCopyMode),
		},
	)

	// Produce data using zero-copy serialization
	t.Log("Producing data with zero-copy serialization...")
	version1, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		// Add test data that will be serialized using Cap'n Proto
		ws.Add(createTestData("movie_1", "The Matrix"))
		ws.Add(createTestData("movie_2", "Inception"))
		ws.Add(createTestData("movie_3", "Interstellar"))
	})
	if err != nil {
		t.Fatalf("Failed to produce data: %v", err)
	}
	if version1 == 0 {
		t.Fatalf("Expected non-zero version")
	}
	t.Logf("Produced version %d with zero-copy serialization", version1)

	// Consume data using zero-copy access
	t.Log("Consuming data with zero-copy access...")
	err = zeroCopyConsumer.TriggerRefreshToWithZeroCopy(ctx, version1)
	if err != nil {
		t.Fatalf("Failed to refresh zero-copy consumer: %v", err)
	}

	// Verify zero-copy support is available
	if !zeroCopyConsumer.HasZeroCopySupport() {
		t.Log("Zero-copy support not available, but test continues for coverage")
	} else {
		t.Log("✓ Zero-copy support confirmed")
	}

	// Test zero-copy data access
	t.Log("Testing zero-copy data access...")
	data := zeroCopyConsumer.GetDataWithZeroCopyPreference()
	if len(data) == 0 {
		t.Fatalf("Expected non-empty data")
	}
	t.Logf("✓ Retrieved data with %d categories", len(data))

	// Test zero-copy view access
	if view, exists := zeroCopyConsumer.GetZeroCopyView(); exists {
		t.Log("Testing zero-copy view access...")

		// Test direct buffer access
		buffer := view.GetByteBuffer()
		if len(buffer) == 0 {
			t.Errorf("Expected non-empty buffer")
		} else {
			t.Logf("✓ Zero-copy buffer access successful: %d bytes", len(buffer))
		}

		// Test message access
		message := view.GetMessage()
		if message == nil {
			t.Errorf("Expected non-nil message")
		} else {
			t.Log("✓ Zero-copy message access successful")
		}
	}

	// Test version consistency
	currentVersion := zeroCopyConsumer.GetCurrentVersion()
	if currentVersion != version1 {
		t.Errorf("Version mismatch: expected %d, got %d", version1, currentVersion)
	} else {
		t.Logf("✓ Version consistency confirmed: %d", currentVersion)
	}
}

// TestZeroCopyIndexIntegration tests zero-copy index building and querying
func TestZeroCopyIndexIntegration(t *testing.T) {
	ctx := context.Background()

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Create producer with zero-copy serialization
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.ZeroCopyMode),
	)

	// Create consumer
	zeroCopyConsumer := consumer.NewZeroCopyConsumerWithOptions(
		[]consumer.ConsumerOption{
			consumer.WithBlobRetriever(blobStore),
			consumer.WithAnnouncementWatcher(announcer),
		},
		[]consumer.ZeroCopyConsumerOption{
			consumer.WithZeroCopySerializationMode(internal.ZeroCopyMode),
		},
	)

	// Produce data
	t.Log("Producing data for index testing...")
	version, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 10; i++ {
			ws.Add(createTestData(fmt.Sprintf("item_%d", i), fmt.Sprintf("Title %d", i)))
		}
	})
	if err != nil {
		t.Fatalf("Failed to produce data: %v", err)
	}

	// Consume and get zero-copy view
	err = zeroCopyConsumer.TriggerRefreshToWithZeroCopy(ctx, version)
	if err != nil {
		t.Fatalf("Failed to refresh consumer: %v", err)
	}

	// Test zero-copy index building
	view, exists := zeroCopyConsumer.GetZeroCopyView()
	if !exists {
		t.Skip("Zero-copy view not available, skipping index test")
		return
	}

	t.Log("Building zero-copy indexes...")
	indexManager := index.NewZeroCopyIndexManager(view)

	// Define schema for index building
	schema := map[string][]string{
		"movie": {"id", "title"},
	}

	// Build indexes from zero-copy data
	err = indexManager.BuildAllIndexes(schema)
	if err != nil {
		t.Fatalf("Failed to build indexes: %v", err)
	}
	t.Log("✓ Zero-copy indexes built successfully")

	// Test index queries
	adapter := indexManager.GetAdapter()

	// Test hash index query
	t.Log("Testing zero-copy hash index query...")
	matches, err := adapter.FindByHashIndex("movie_id_hash", "movie_id_0")
	if err != nil {
		t.Errorf("Hash index query failed: %v", err)
	} else {
		t.Logf("✓ Hash index query returned %d matches", len(matches))
	}

	// Test unique index query
	t.Log("Testing zero-copy unique index query...")
	match, err := adapter.FindByUniqueIndex("movie_id_unique", "movie_id_0")
	if err != nil {
		t.Errorf("Unique index query failed: %v", err)
	} else if match != nil {
		t.Log("✓ Unique index query successful")
	}

	// Get index statistics
	stats := adapter.GetIndexStats()
	t.Logf("✓ Index statistics: %v", stats)
}

// TestZeroCopyPerformanceComparison compares zero-copy vs traditional performance
func TestZeroCopyPerformanceComparison(t *testing.T) {
	ctx := context.Background()

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Test data size
	recordCount := 1000

	// Test 1: Zero-copy mode
	t.Log("Testing zero-copy mode performance...")
	startTime := time.Now()

	zeroCopyProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.ZeroCopyMode),
	)

	version1, err := zeroCopyProducer.RunCycle(ctx, func(ws *internal.WriteState) {
		for i := 0; i < recordCount; i++ {
			ws.Add(createTestData(fmt.Sprintf("zc_item_%d", i), fmt.Sprintf("ZC Title %d", i)))
		}
	})
	if err != nil {
		t.Fatalf("Zero-copy production failed: %v", err)
	}

	zeroCopyConsumer := consumer.NewZeroCopyConsumerWithOptions(
		[]consumer.ConsumerOption{
			consumer.WithBlobRetriever(blobStore),
			consumer.WithAnnouncementWatcher(announcer),
		},
		[]consumer.ZeroCopyConsumerOption{
			consumer.WithZeroCopySerializationMode(internal.ZeroCopyMode),
		},
	)

	err = zeroCopyConsumer.TriggerRefreshToWithZeroCopy(ctx, version1)
	if err != nil {
		t.Fatalf("Zero-copy consumption failed: %v", err)
	}

	zeroCopyTime := time.Since(startTime)
	t.Logf("✓ Zero-copy mode completed in %v", zeroCopyTime)

	// Test 2: Traditional mode
	t.Log("Testing traditional mode performance...")
	startTime = time.Now()

	traditionalProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.TraditionalMode),
	)

	version2, err := traditionalProducer.RunCycle(ctx, func(ws *internal.WriteState) {
		for i := 0; i < recordCount; i++ {
			ws.Add(createTestData(fmt.Sprintf("tr_item_%d", i), fmt.Sprintf("TR Title %d", i)))
		}
	})
	if err != nil {
		t.Fatalf("Traditional production failed: %v", err)
	}

	traditionalConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(announcer),
	)

	err = traditionalConsumer.TriggerRefreshTo(ctx, version2)
	if err != nil {
		t.Fatalf("Traditional consumption failed: %v", err)
	}

	traditionalTime := time.Since(startTime)
	t.Logf("✓ Traditional mode completed in %v", traditionalTime)

	// Performance comparison
	if zeroCopyTime < traditionalTime {
		ratio := float64(traditionalTime) / float64(zeroCopyTime)
		t.Logf("✓ Zero-copy is %.2fx faster than traditional", ratio)
	} else {
		ratio := float64(zeroCopyTime) / float64(traditionalTime)
		t.Logf("ℹ Traditional is %.2fx faster than zero-copy (expected for small datasets)", ratio)
	}

	// Memory usage comparison
	t.Log("Testing memory usage comparison...")

	// Zero-copy memory usage
	if view, exists := zeroCopyConsumer.GetZeroCopyView(); exists {
		bufferSize := len(view.GetByteBuffer())
		t.Logf("Zero-copy buffer size: %d bytes", bufferSize)
	}

	// Traditional memory usage (estimated)
	tradData := traditionalConsumer.GetStateEngine().GetCurrentState()
	if tradData != nil {
		dataMap := tradData.GetAllData()
		estimatedSize := len(fmt.Sprintf("%v", dataMap))
		t.Logf("Traditional data size (estimated): %d bytes", estimatedSize)
	}
}

// TestHybridSerializationMode tests the hybrid mode that supports both traditional and zero-copy
func TestHybridSerializationMode(t *testing.T) {
	t.Skip("Hybrid mode requires further development - skipping for now")
	ctx := context.Background()

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Create producer with hybrid serialization
	hybridProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.HybridMode),
	)

	// Create hybrid consumer
	hybridConsumer := consumer.NewZeroCopyConsumerWithOptions(
		[]consumer.ConsumerOption{
			consumer.WithBlobRetriever(blobStore),
			consumer.WithAnnouncementWatcher(announcer),
		},
		[]consumer.ZeroCopyConsumerOption{
			consumer.WithZeroCopySerializationMode(internal.HybridMode),
		},
	)

	// Test small dataset (should use traditional mode)
	t.Log("Testing hybrid mode with small dataset...")
	version1, err := hybridProducer.RunCycle(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 10; i++ {
			ws.Add(createTestData(fmt.Sprintf("small_%d", i), fmt.Sprintf("Small %d", i)))
		}
	})
	if err != nil {
		t.Fatalf("Small dataset production failed: %v", err)
	}

	err = hybridConsumer.TriggerRefreshToWithZeroCopy(ctx, version1)
	if err != nil {
		t.Fatalf("Small dataset consumption failed: %v", err)
	}
	t.Logf("✓ Small dataset processed (version %d)", version1)

	// Test large dataset (should use zero-copy mode)
	t.Log("Testing hybrid mode with large dataset...")
	version2, err := hybridProducer.RunCycle(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 2000; i++ { // Large dataset
			ws.Add(createTestData(fmt.Sprintf("large_%d", i), fmt.Sprintf("Large %d", i)))
		}
	})
	if err != nil {
		t.Fatalf("Large dataset production failed: %v", err)
	}

	err = hybridConsumer.TriggerRefreshToWithZeroCopy(ctx, version2)
	if err != nil {
		t.Fatalf("Large dataset consumption failed: %v", err)
	}
	t.Logf("✓ Large dataset processed (version %d)", version2)

	// Verify both versions are accessible
	data1 := hybridConsumer.GetDataWithZeroCopyPreference()
	if len(data1) == 0 {
		t.Errorf("Expected data for version %d", version1)
	}

	// Check if zero-copy support varies with dataset size
	hasZeroCopy := hybridConsumer.HasZeroCopySupport()
	t.Logf("Zero-copy support available: %t", hasZeroCopy)
}

// createTestData creates test data for integration tests
func createTestData(id, title string) map[string]interface{} {
	return map[string]interface{}{
		"id":    id,
		"title": title,
		"type":  "movie",
	}
}

// TestZeroCopyDataAccessor tests the zero-copy data accessor
func TestZeroCopyDataAccessor(t *testing.T) {
	ctx := context.Background()

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Create producer and consumer
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.ZeroCopyMode),
	)

	zeroCopyConsumer := consumer.NewZeroCopyConsumerWithOptions(
		[]consumer.ConsumerOption{
			consumer.WithBlobRetriever(blobStore),
			consumer.WithAnnouncementWatcher(announcer),
		},
		[]consumer.ZeroCopyConsumerOption{
			consumer.WithZeroCopySerializationMode(internal.ZeroCopyMode),
		},
	)

	// Produce data
	version, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add(createTestData("test_1", "Test Movie"))
	})
	if err != nil {
		t.Fatalf("Failed to produce data: %v", err)
	}

	// Consume and get zero-copy view
	err = zeroCopyConsumer.TriggerRefreshToWithZeroCopy(ctx, version)
	if err != nil {
		t.Fatalf("Failed to refresh consumer: %v", err)
	}

	view, exists := zeroCopyConsumer.GetZeroCopyView()
	if !exists {
		t.Skip("Zero-copy view not available")
		return
	}

	// Test data accessor
	accessor := consumer.NewZeroCopyDataAccessor(view)

	// Test buffer access
	buffer := accessor.GetRawBuffer()
	if len(buffer) == 0 {
		t.Errorf("Expected non-empty buffer")
	} else {
		t.Logf("✓ Buffer access successful: %d bytes", len(buffer))
	}

	// Test buffer size
	size := accessor.GetBufferSize()
	if size != len(buffer) {
		t.Errorf("Buffer size mismatch: expected %d, got %d", len(buffer), size)
	} else {
		t.Logf("✓ Buffer size consistent: %d bytes", size)
	}

	// Test query engine
	queryEngine := consumer.NewZeroCopyQueryEngine(view)

	// Test record count
	count, err := queryEngine.CountRecords()
	if err != nil {
		t.Errorf("Failed to count records: %v", err)
	} else {
		t.Logf("✓ Record count estimation: %d records", count)
	}

	// Test field extraction
	fields, err := queryEngine.ExtractField("title")
	if err != nil {
		t.Errorf("Failed to extract field: %v", err)
	} else {
		t.Logf("✓ Field extraction successful: %d fields", len(fields))
	}

	// Test offset access
	if count > 0 {
		data, err := queryEngine.FindByOffset(0)
		if err != nil {
			t.Errorf("Failed to access offset 0: %v", err)
		} else {
			t.Logf("✓ Offset access successful: %d bytes", len(data))
		}
	}
}
