package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
)

// Test the key performance difference: ListVersions vs direct traversal
func TestTraversalPerformanceComparison(t *testing.T) {
	const totalVersions = 50000
	const snapshotInterval = 1000 // Sparse snapshots to show the difference
	const targetVersion = 45000
	
	// Setup store with sparse snapshots
	store := blob.NewInMemoryBlobStore()
	
	for version := int64(1); version <= totalVersions; version++ {
		if version%snapshotInterval == 0 {
			// Create snapshot
			snapshotBlob := &blob.Blob{
				Type:    blob.SnapshotBlob,
				Version: version,
				Data:    []byte(fmt.Sprintf("snapshot_%d", version)),
			}
			store.Store(context.Background(), snapshotBlob)
		} else {
			// Create delta
			deltaBlob := &blob.Blob{
				Type:        blob.DeltaBlob,
				FromVersion: version - 1,
				ToVersion:   version,
				Data:        []byte(fmt.Sprintf("delta_%d", version)),
			}
			store.Store(context.Background(), deltaBlob)
		}
	}
	
	fmt.Printf("Created store with %d versions, snapshots every %d versions\n", totalVersions, snapshotInterval)
	fmt.Printf("Target: refresh to version %d\n\n", targetVersion)
	
	// Test 1: Simulate OLD approach (ListVersions + iterate)
	t.Run("Old ListVersions Approach", func(t *testing.T) {
		start := time.Now()
		
		// Step 1: Expensive ListVersions call
		versions := store.ListVersions()
		listTime := time.Since(start)
		
		// Step 2: Find nearest snapshot by iterating through ALL versions
		var nearestSnapshot int64
		searchStart := time.Now()
		for i := len(versions) - 1; i >= 0; i-- {
			v := versions[i]
			if v <= targetVersion {
				if snapshot := store.RetrieveSnapshotBlob(v); snapshot != nil {
					nearestSnapshot = v
					break
				}
			}
		}
		searchTime := time.Since(searchStart)
		
		// Step 3: Load deltas from snapshot to target
		deltaStart := time.Now()
		deltasLoaded := 0
		for v := nearestSnapshot + 1; v <= targetVersion; v++ {
			if store.RetrieveDeltaBlob(v-1) != nil {
				deltasLoaded++
			}
		}
		deltaTime := time.Since(deltaStart)
		
		totalTime := time.Since(start)
		
		fmt.Printf("OLD APPROACH:\n")
		fmt.Printf("  ListVersions(): %v (returned %d versions)\n", listTime, len(versions))
		fmt.Printf("  Search for snapshot: %v (found snapshot at version %d)\n", searchTime, nearestSnapshot)
		fmt.Printf("  Load %d deltas: %v\n", deltasLoaded, deltaTime)
		fmt.Printf("  TOTAL TIME: %v\n\n", totalTime)
	})
	
	// Test 2: NEW approach (planBlobs - direct traversal)
	t.Run("New planBlobs Approach", func(t *testing.T) {
		consumer := consumer.NewConsumer(
			consumer.WithBlobRetriever(store),
		)
		
		start := time.Now()
		
		err := consumer.TriggerRefreshTo(context.Background(), targetVersion)
		if err != nil {
			t.Fatalf("TriggerRefreshTo failed: %v", err)
		}
		
		totalTime := time.Since(start)
		
		fmt.Printf("NEW APPROACH:\n")
		fmt.Printf("  planBlobs() direct traversal: %v\n", totalTime)
		fmt.Printf("  (No expensive ListVersions call!)\n")
		fmt.Printf("  (Walks backwards until snapshot found)\n\n")
	})
	
	// Test 3: Show scaling behavior with different snapshot frequencies
	fmt.Printf("SCALING TEST - Effect of snapshot frequency:\n")
	
	for _, interval := range []int{100, 500, 1000, 5000} {
		t.Run(fmt.Sprintf("Snapshots_every_%d", interval), func(t *testing.T) {
			// Create store with this snapshot frequency
			testStore := blob.NewInMemoryBlobStore()
			for version := int64(1); version <= 10000; version++ {
				if version%int64(interval) == 0 {
					snapshot := &blob.Blob{
						Type:    blob.SnapshotBlob,
						Version: version,
						Data:    []byte(fmt.Sprintf("snapshot_%d", version)),
					}
					testStore.Store(context.Background(), snapshot)
				} else {
					delta := &blob.Blob{
						Type:        blob.DeltaBlob,
						FromVersion: version - 1,
						ToVersion:   version,
						Data:        []byte(fmt.Sprintf("delta_%d", version)),
					}
					testStore.Store(context.Background(), delta)
				}
			}
			
			consumer := consumer.NewConsumer(
				consumer.WithBlobRetriever(testStore),
			)
			
			start := time.Now()
			err := consumer.TriggerRefreshTo(context.Background(), 9500)
			if err != nil {
				t.Fatalf("TriggerRefreshTo failed: %v", err)
			}
			elapsed := time.Since(start)
			
			fmt.Printf("  Snapshot every %4d versions: %v\n", interval, elapsed)
		})
	}
}

// Demonstrate the planBlobs algorithm step by step
func TestPlanBlobsVisualization(t *testing.T) {
	const totalVersions = 20
	const snapshotInterval = 7
	const targetVersion = 18
	
	store := blob.NewInMemoryBlobStore()
	
	// Create small store for visualization
	for version := int64(1); version <= totalVersions; version++ {
		if version%snapshotInterval == 0 {
			snapshot := &blob.Blob{
				Type:    blob.SnapshotBlob,
				Version: version,
				Data:    []byte(fmt.Sprintf("snapshot_%d", version)),
			}
			store.Store(context.Background(), snapshot)
		} else {
			delta := &blob.Blob{
				Type:        blob.DeltaBlob,
				FromVersion: version - 1,
				ToVersion:   version,
				Data:        []byte(fmt.Sprintf("delta_%d", version)),
			}
			store.Store(context.Background(), delta)
		}
	}
	
	fmt.Printf("VISUALIZATION: How planBlobs works\n")
	fmt.Printf("Store: versions 1-%d, snapshots at [7, 14]\n", totalVersions)
	fmt.Printf("Goal: refresh to version %d\n\n", targetVersion)
	
	// Show what planBlobs algorithm does step by step
	fmt.Printf("planBlobs algorithm walkthrough:\n")
	cur := int64(targetVersion)
	steps := 0
	
	for cur > 0 {
		steps++
		if snapshot := store.RetrieveSnapshotBlob(cur); snapshot != nil {
			fmt.Printf("Step %d: Check version %d for snapshot → FOUND! (anchor)\n", steps, cur)
			break
		} else {
			fmt.Printf("Step %d: Check version %d for snapshot → not found\n", steps, cur)
		}
		
		if delta := store.RetrieveDeltaBlob(cur - 1); delta != nil {
			fmt.Printf("        Check version %d for delta → found, continue from version %d\n", cur-1, delta.FromVersion)
			cur = delta.FromVersion
		} else {
			fmt.Printf("        Check version %d for delta → not found, stop\n", cur-1)
			break
		}
		
		if steps > 10 {
			fmt.Printf("        (stopping after 10 steps for demo)\n")
			break
		}
	}
	
	fmt.Printf("\nTotal steps: %d\n", steps)
	fmt.Printf("This is O(distance_to_nearest_snapshot), NOT O(total_versions)!\n")
}
