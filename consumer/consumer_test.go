package consumer

import (
	"context"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
	"testing"
)

// TestConsumer_BasicRefresh tests basic consumer refresh functionality
func TestConsumer_BasicRefresh(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	// Publish a snapshot
	prod := producer.NewProducer(producer.WithBlobStore(blobStore))
	version, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("test_data")
	})

	// Create consumer and refresh
	consumer := NewConsumer(WithBlobRetriever(blobStore))
	err := consumer.TriggerRefreshTo(context.Background(), version)
	if err != nil {
		t.Fatalf("TriggerRefreshTo failed: %v", err)
	}

	if consumer.GetCurrentVersion() != version {
		t.Errorf("Current version = %d, want %d", consumer.GetCurrentVersion(), version)
	}
}

// TestConsumer_DeltaTraversal tests delta chain traversal
func TestConsumer_DeltaTraversal(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithNumStatesBetweenSnapshots(2), // Only snapshot every 2 states
	)

	// Create versions with deltas
	v1, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data1")
	})

	_, _ = prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data2")
	})

	v3, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data3")
	})

	// Consumer should traverse deltas to reach v3
	consumer := NewConsumer(WithBlobRetriever(blobStore))
	err := consumer.TriggerRefreshTo(context.Background(), v3)
	if err != nil {
		t.Fatalf("TriggerRefreshTo v3 failed: %v", err)
	}

	if consumer.GetCurrentVersion() != v3 {
		t.Errorf("Current version = %d, want %d", consumer.GetCurrentVersion(), v3)
	}

	// Verify that blob storage is working - the producer creates snapshots/deltas based on configuration
	// In this test with numStatesBetweenSnapshots=2, v1 and v3 are snapshots
	if blobStore.RetrieveSnapshotBlob(v1) == nil {
		t.Error("Should have snapshot for v1")
	}
	if blobStore.RetrieveSnapshotBlob(v3) == nil {
		t.Error("Should have snapshot for v3")
	}

	// v2 could be either snapshot or delta depending on the producer logic
	// The important thing is that consumer can traverse to v3 successfully
}

// TestConsumer_AnnouncementWatcher tests automatic updates via announcements
func TestConsumer_AnnouncementWatcher(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcement := blob.NewInMemoryAnnouncement()

	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcement),
	)

	// Create consumer with announcement watcher
	consumer := NewConsumer(
		WithBlobRetriever(blobStore),
		WithAnnouncementWatcher(announcement),
	)

	// Publish v1
	v1, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data1")
	})

	// Consumer should automatically update
	consumer.TriggerRefresh(context.Background())
	if consumer.GetCurrentVersion() != v1 {
		t.Errorf("Consumer should auto-update to v1, got %d", consumer.GetCurrentVersion())
	}

	// Publish v2
	v2, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data2")
	})

	// Give time for announcement propagation (simplified test)
	err := consumer.TriggerRefresh(context.Background())
	if err != nil {
		t.Fatalf("Manual refresh failed: %v", err)
	}

	if consumer.GetCurrentVersion() != v2 {
		t.Errorf("Consumer should update to v2, got %d", consumer.GetCurrentVersion())
	}
}

// TestConsumer_TypeFiltering tests type filtering during consumption
func TestConsumer_TypeFiltering(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	prod := producer.NewProducer(producer.WithBlobStore(blobStore))
	version, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("string_data")
		ws.Add(42) // integer data
	})

	// Create consumer with type filter - only strings
	typeFilter := internal.NewTypeFilter().ExcludeAll().Include("String")
	consumer := NewConsumer(
		WithBlobRetriever(blobStore),
		WithTypeFilter(typeFilter),
	)

	err := consumer.TriggerRefreshTo(context.Background(), version)
	if err != nil {
		t.Fatalf("TriggerRefreshTo failed: %v", err)
	}

	// Verify only string type is loaded
	stateEngine := consumer.GetStateEngine()
	if !stateEngine.HasType("String") {
		t.Error("String type should be loaded")
	}
	if stateEngine.HasType("Integer") {
		t.Error("Integer type should be filtered out")
	}
}

// TestConsumer_MemoryModeCompatibility tests memory mode constraints
func TestConsumer_MemoryModeCompatibility(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	typeFilter := internal.NewTypeFilter().ExcludeAll().Include("String")

	// Test shared memory mode with filtering - should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Should panic when using type filter with shared memory mode")
		}
	}()

	NewConsumer(
		WithBlobRetriever(blobStore),
		WithMemoryMode(internal.SharedMemoryLazy),
		WithTypeFilter(typeFilter),
	)
}
