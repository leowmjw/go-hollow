package main

import (
	"context"
	"testing"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// TestProducerConsumerWorkflow tests the exact scenario that was failing:
// 1. Consumer version 0 should show nothing
// 2. Produce data
// 3. Consumer version 1 should now show the new data added
func TestProducerConsumerWorkflow(t *testing.T) {
	// Use in-memory store for reliable testing
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Create producer
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithNumStatesBetweenSnapshots(1), // Create snapshot for every version
	)

	// Create consumer
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		// Disable auto-refresh for controlled testing
	)

	ctx := context.Background()

	// Test 1: Consumer version 0 should show nothing (no data available)
	t.Run("Consumer version 0 shows nothing", func(t *testing.T) {
		// Try to refresh to version 0 (latest, but nothing exists yet)
		err := cons.TriggerRefresh(ctx)

		// Consumer should handle empty store gracefully (no error, stays at version 0)
		if err != nil {
			t.Logf("Consumer refresh from empty store returned error: %v", err)
		}

		// Consumer version should be 0 (no data consumed)
		if cons.GetCurrentVersion() != 0 {
			t.Errorf("Expected consumer version 0, got %d", cons.GetCurrentVersion())
		}

		// Verify no versions exist in store
		versions := blobStore.ListVersions()
		if len(versions) != 0 {
			t.Errorf("Expected no versions in store, got %v", versions)
		}

		// State engine should not have any types
		stateEngine := cons.GetStateEngine()
		if stateEngine.HasType("String") {
			t.Error("State engine should not have String type when no data consumed")
		}
	})

	// Test 2: Produce data
	var version1 int64
	t.Run("Produce data", func(t *testing.T) {
		version1, _ = prod.RunCycle(ctx, func(ws *internal.WriteState) {
			ws.Add("test_data_1")
			ws.Add("test_data_2")
			ws.Add("test_data_3")
		})

		if version1 == 0 {
			t.Fatal("Producer should return non-zero version")
		}

		// Verify version exists in store
		versions := blobStore.ListVersions()
		if len(versions) != 1 || versions[0] != version1 {
			t.Errorf("Expected versions [%d], got %v", version1, versions)
		}

		// Verify snapshot blob exists
		snapshot := blobStore.RetrieveSnapshotBlob(version1)
		if snapshot == nil {
			t.Fatal("Snapshot blob should exist for version 1")
		}

		if snapshot.Version != version1 {
			t.Errorf("Expected snapshot version %d, got %d", version1, snapshot.Version)
		}
	})

	// Test 3: Consumer version 1 should now show the new data added
	t.Run("Consumer version 1 shows new data", func(t *testing.T) {
		// Refresh consumer to version 1
		err := cons.TriggerRefreshTo(ctx, version1)
		if err != nil {
			t.Fatalf("Consumer refresh to version %d failed: %v", version1, err)
		}

		// Consumer should now be at version 1
		if cons.GetCurrentVersion() != version1 {
			t.Errorf("Expected consumer version %d, got %d", version1, cons.GetCurrentVersion())
		}

		// Verify state engine has data
		stateEngine := cons.GetStateEngine()
		if stateEngine == nil {
			t.Fatal("State engine should not be nil")
		}

		// Check that String type exists (our test data is strings)
		if !stateEngine.HasType("String") {
			t.Error("State engine should have String type after consuming data")
		}

		// Verify the state engine version matches
		if stateEngine.GetCurrentVersion() != version1 {
			t.Errorf("Expected state engine version %d, got %d", version1, stateEngine.GetCurrentVersion())
		}
	})

	// Test 4: Consumer can refresh to latest (should be same as version 1)
	t.Run("Consumer refresh to latest", func(t *testing.T) {
		err := cons.TriggerRefresh(ctx)
		if err != nil {
			t.Fatalf("Consumer refresh to latest failed: %v", err)
		}

		if cons.GetCurrentVersion() != version1 {
			t.Errorf("Expected consumer at latest version %d, got %d", version1, cons.GetCurrentVersion())
		}
	})
}

// TestProducerConsumerMultipleVersions tests the workflow with multiple versions
func TestProducerConsumerMultipleVersions(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithNumStatesBetweenSnapshots(1), // Create snapshot for every version
	)

	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		// Disable auto-refresh for controlled testing
	)

	ctx := context.Background()

	// Produce version 1
	version1, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add("data_v1_1")
		ws.Add("data_v1_2")
	})
	t.Logf("Produced version 1: %d", version1)

	// Produce version 2 with completely different data
	version2, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		// Clear previous data and add new data
		ws.Add("completely_different_data_1")
		ws.Add("completely_different_data_2")
		ws.Add("completely_different_data_3")
		ws.Add("extra_data_to_ensure_difference")
	})
	t.Logf("Produced version 2: %d", version2)

	// Debug: Check if versions are different
	if version1 == version2 {
		t.Errorf("Version 1 and 2 are the same: %d. This indicates a producer issue.", version1)
	}

	// Test consuming version 1
	t.Run("Consume version 1", func(t *testing.T) {
		err := cons.TriggerRefreshTo(ctx, version1)
		if err != nil {
			t.Fatalf("Failed to refresh to version 1: %v", err)
		}

		if cons.GetCurrentVersion() != version1 {
			t.Errorf("Expected version %d, got %d", version1, cons.GetCurrentVersion())
		}

		stateEngine := cons.GetStateEngine()
		if !stateEngine.HasType("String") {
			t.Error("Version 1 should have String type")
		}
	})

	// Test consuming version 2
	t.Run("Consume version 2", func(t *testing.T) {
		err := cons.TriggerRefreshTo(ctx, version2)
		if err != nil {
			t.Fatalf("Failed to refresh to version 2: %v", err)
		}

		if cons.GetCurrentVersion() != version2 {
			t.Errorf("Expected version %d, got %d", version2, cons.GetCurrentVersion())
		}

		stateEngine := cons.GetStateEngine()
		if !stateEngine.HasType("String") {
			t.Error("Version 2 should have String type")
		}
	})

	// Test consuming latest (should be version 2)
	t.Run("Consume latest", func(t *testing.T) {
		err := cons.TriggerRefresh(ctx)
		if err != nil {
			t.Fatalf("Failed to refresh to latest: %v", err)
		}

		if cons.GetCurrentVersion() != version2 {
			t.Errorf("Expected latest version %d, got %d", version2, cons.GetCurrentVersion())
		}
	})

	// Verify both versions exist in store
	versions := blobStore.ListVersions()
	t.Logf("Versions in store: %v", versions)
	expectedVersions := []int64{version1, version2}
	if len(versions) != len(expectedVersions) {
		t.Errorf("Expected %d versions, got %d. Versions in store: %v", len(expectedVersions), len(versions), versions)
	}

	// Check that both versions have snapshot blobs
	for _, v := range expectedVersions {
		snapshot := blobStore.RetrieveSnapshotBlob(v)
		if snapshot == nil {
			t.Errorf("Missing snapshot blob for version %d", v)
		}
	}
}

// TestConsumerErrorHandling tests error cases
func TestConsumerErrorHandling(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		// Disable auto-refresh for controlled testing
	)

	ctx := context.Background()

	// Test consuming non-existent version
	t.Run("Consume non-existent version", func(t *testing.T) {
		err := cons.TriggerRefreshTo(ctx, 999)
		if err == nil {
			t.Error("Expected error when consuming non-existent version, but got none")
		}
	})

	// Test consuming from empty store
	t.Run("Consume from empty store", func(t *testing.T) {
		err := cons.TriggerRefresh(ctx)
		// Consumer handles empty store gracefully (no error, stays at version 0)
		if err != nil {
			t.Logf("Consumer refresh from empty store returned error: %v", err)
		}
		// Consumer should stay at version 0
		if cons.GetCurrentVersion() != 0 {
			t.Errorf("Expected consumer to stay at version 0, got %d", cons.GetCurrentVersion())
		}
	})
}
