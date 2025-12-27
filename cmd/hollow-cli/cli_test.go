package main

import (
	"context"
	"testing"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// TestCLIProducerConsumerWorkflow tests the CLI's producer-consumer workflow
// This test verifies the exact scenario that was failing in the CLI:
// 1. Consumer version 0 should show nothing
// 2. Produce data
// 3. Consumer version 1 should now show the new data added
func TestCLIProducerConsumerWorkflow(t *testing.T) {
	// Use the same setup as the CLI
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Create producer with CLI configuration
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithNumStatesBetweenSnapshots(1), // Ensure snapshots for every version
	)

	// Create consumer with CLI configuration
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	ctx := context.Background()

	// Test 1: Consumer version 0 should show nothing (no data available)
	t.Run("CLI Consumer version 0 shows nothing", func(t *testing.T) {
		// Try to refresh to version 0 (latest, but nothing exists yet)
		err := cons.TriggerRefresh(ctx)

		// Consumer should handle empty store gracefully
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

	// Test 2: Produce data (simulating CLI producer)
	var version1 int64
	t.Run("CLI Produce data", func(t *testing.T) {
		// Simulate the CLI producer cycle with test data
		version1, _ = prod.RunCycle(ctx, func(ws *internal.WriteState) {
			// Add some test data like the CLI does
			for i := 0; i < 10; i++ {
				ws.Add("test_data_" + string(rune('0'+i)))
			}
			ws.Add("data_from_file:fixtures/simple_test.json")
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

		t.Logf("CLI Producer created version %d with snapshot blob", version1)
	})

	// Test 3: Consumer version 1 should now show the new data added
	t.Run("CLI Consumer version 1 shows new data", func(t *testing.T) {
		// Refresh consumer to version 1 (simulating CLI interactive mode)
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

		t.Logf("CLI Consumer successfully consumed version %d with String data", version1)
	})

	// Test 4: Test interactive mode scenario - produce another version
	t.Run("CLI Interactive mode - produce version 2", func(t *testing.T) {
		// Simulate interactive mode producing another version
		version2, _ := prod.RunCycle(ctx, func(ws *internal.WriteState) {
			// Add some new test data
			for i := 0; i < 5; i++ {
				ws.Add("new_data_v2_" + string(rune('0'+i)))
			}
		})

		if version2 <= version1 {
			t.Errorf("Version 2 (%d) should be greater than version 1 (%d)", version2, version1)
		}

		// Consumer should be able to consume version 2
		err := cons.TriggerRefreshTo(ctx, version2)
		if err != nil {
			t.Fatalf("Consumer refresh to version %d failed: %v", version2, err)
		}

		if cons.GetCurrentVersion() != version2 {
			t.Errorf("Expected consumer version %d, got %d", version2, cons.GetCurrentVersion())
		}

		// Verify both versions exist in store
		versions := blobStore.ListVersions()
		if len(versions) != 2 {
			t.Errorf("Expected 2 versions in store, got %d: %v", len(versions), versions)
		}

		t.Logf("CLI Interactive mode successfully created and consumed version %d", version2)
	})
}

// TestCLIMemoryStorageIssue tests the specific issue that was reported:
// "no delta blob found from version 0" when using separate processes
func TestCLIMemoryStorageIssue(t *testing.T) {
	t.Run("Separate blob stores simulate separate processes", func(t *testing.T) {
		// Simulate the original issue: separate blob stores (like separate processes)
		producerBlobStore := blob.NewInMemoryBlobStore()
		consumerBlobStore := blob.NewInMemoryBlobStore()

		// Producer creates data in its own blob store
		prod := producer.NewProducer(
			producer.WithBlobStore(producerBlobStore),
			producer.WithNumStatesBetweenSnapshots(1),
		)

		version1, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
			ws.Add("test_data")
		})

		// Consumer tries to consume from its own (empty) blob store
		cons := consumer.NewConsumer(
			consumer.WithBlobRetriever(consumerBlobStore),
		)

		// This should fail because the consumer's blob store is empty
		err := cons.TriggerRefreshTo(context.Background(), version1)
		if err == nil {
			t.Error("Expected error when consumer uses different blob store, but got none")
		}

		t.Logf("Confirmed: separate blob stores cause the original issue: %v", err)
	})

	t.Run("Shared blob store solves the issue", func(t *testing.T) {
		// Solution: shared blob store (same process)
		sharedBlobStore := blob.NewInMemoryBlobStore()

		// Producer and consumer share the same blob store
		prod := producer.NewProducer(
			producer.WithBlobStore(sharedBlobStore),
			producer.WithNumStatesBetweenSnapshots(1),
		)

		cons := consumer.NewConsumer(
			consumer.WithBlobRetriever(sharedBlobStore),
		)

		version1, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
			ws.Add("test_data")
		})

		// This should succeed because they share the same blob store
		err := cons.TriggerRefreshTo(context.Background(), version1)
		if err != nil {
			t.Errorf("Expected success with shared blob store, but got error: %v", err)
		}

		if cons.GetCurrentVersion() != version1 {
			t.Errorf("Expected consumer version %d, got %d", version1, cons.GetCurrentVersion())
		}

		t.Logf("Confirmed: shared blob store solves the issue - consumer at version %d", cons.GetCurrentVersion())
	})
}
