package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
)

// InstrumentedBlobStore tracks performance metrics
type InstrumentedBlobStore struct {
	*blob.InMemoryBlobStore
	listCalls    int64
	getCalls     int64
	totalLatency time.Duration
}

func NewInstrumentedBlobStore() *InstrumentedBlobStore {
	return &InstrumentedBlobStore{
		InMemoryBlobStore: blob.NewInMemoryBlobStore(),
	}
}

func (s *InstrumentedBlobStore) ListVersions() []int64 {
	start := time.Now()
	atomic.AddInt64(&s.listCalls, 1)
	
	// Simulate S3-like latency for list operations (expensive!)
	time.Sleep(50 * time.Millisecond)
	
	result := s.InMemoryBlobStore.ListVersions()
	s.totalLatency += time.Since(start)
	return result
}

func (s *InstrumentedBlobStore) RetrieveSnapshotBlob(version int64) *blob.Blob {
	start := time.Now()
	atomic.AddInt64(&s.getCalls, 1)
	
	// Simulate faster GET operations
	time.Sleep(2 * time.Millisecond)
	
	result := s.InMemoryBlobStore.RetrieveSnapshotBlob(version)
	s.totalLatency += time.Since(start)
	return result
}

func (s *InstrumentedBlobStore) RetrieveDeltaBlob(fromVersion int64) *blob.Blob {
	start := time.Now()
	atomic.AddInt64(&s.getCalls, 1)
	
	// Simulate GET latency
	time.Sleep(2 * time.Millisecond)
	
	result := s.InMemoryBlobStore.RetrieveDeltaBlob(fromVersion)
	s.totalLatency += time.Since(start)
	return result
}

func (s *InstrumentedBlobStore) GetStats() (listCalls, getCalls int64, latency time.Duration) {
	return atomic.LoadInt64(&s.listCalls), atomic.LoadInt64(&s.getCalls), s.totalLatency
}

func (s *InstrumentedBlobStore) ResetStats() {
	atomic.StoreInt64(&s.listCalls, 0)
	atomic.StoreInt64(&s.getCalls, 0)
	s.totalLatency = 0
}

// setupLargeStore creates a store with many versions and configurable snapshot frequency
func setupLargeStore(totalVersions int, snapshotInterval int) *InstrumentedBlobStore {
	store := NewInstrumentedBlobStore()
	
	for version := int64(1); version <= int64(totalVersions); version++ {
		// Create snapshot every snapshotInterval versions
		if version%int64(snapshotInterval) == 0 || version == 1 {
			snapshotBlob := &blob.Blob{
				Type:    blob.SnapshotBlob,
				Version: version,
				Data:    []byte(fmt.Sprintf("snapshot_data_%d", version)),
			}
			store.Store(context.Background(), snapshotBlob)
		} else {
			// Create delta blob
			deltaBlob := &blob.Blob{
				Type:        blob.DeltaBlob,
				FromVersion: version - 1,
				ToVersion:   version,
				Data:        []byte(fmt.Sprintf("delta_data_%d", version)),
			}
			store.Store(context.Background(), deltaBlob)
		}
	}
	
	store.ResetStats() // Reset stats after setup
	return store
}

// Benchmark different snapshot intervals with large version counts
func BenchmarkConsumerTraversal(b *testing.B) {
	scenarios := []struct {
		totalVersions    int
		snapshotInterval int
		traversalSize    int
	}{
		{100000, 10, 1000},    // Frequent snapshots, medium traversal
		{100000, 100, 5000},   // Moderate snapshots, large traversal  
		{100000, 1000, 10000}, // Sparse snapshots, very large traversal
		{500000, 500, 25000},  // Huge scale test
	}
	
	for _, scenario := range scenarios {
		name := fmt.Sprintf("versions_%d_snapshots_every_%d_traverse_%d", 
			scenario.totalVersions, scenario.snapshotInterval, scenario.traversalSize)
		
		b.Run(name, func(b *testing.B) {
			// Setup store once per scenario
			store := setupLargeStore(scenario.totalVersions, scenario.snapshotInterval)
			
			consumer := consumer.NewConsumer(
				consumer.WithBlobRetriever(store),
			)
			
			// Random starting points for more realistic testing
			startVersions := make([]int64, b.N)
			for i := 0; i < b.N; i++ {
				maxStart := int64(scenario.totalVersions - scenario.traversalSize)
				if maxStart <= 0 {
					maxStart = 1
				}
				startVersions[i] = rand.Int63n(maxStart) + 1
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			var totalListCalls, totalGetCalls int64
			var totalLatency time.Duration
			
			for i := 0; i < b.N; i++ {
				startVersion := startVersions[i]
				targetVersion := startVersion + int64(scenario.traversalSize)
				
				store.ResetStats()
				
				err := consumer.TriggerRefreshTo(context.Background(), targetVersion)
				if err != nil {
					b.Fatalf("TriggerRefreshTo failed: %v", err)
				}
				
				listCalls, getCalls, latency := store.GetStats()
				totalListCalls += listCalls
				totalGetCalls += getCalls
				totalLatency += latency
			}
			
			// Report custom metrics
			avgListCalls := float64(totalListCalls) / float64(b.N)
			avgGetCalls := float64(totalGetCalls) / float64(b.N)
			avgLatency := totalLatency / time.Duration(b.N)
			
			b.ReportMetric(avgListCalls, "list_calls_per_op")
			b.ReportMetric(avgGetCalls, "get_calls_per_op")
			b.ReportMetric(float64(avgLatency.Milliseconds()), "latency_ms_per_op")
		})
	}
}

// Benchmark comparing old vs new approach directly
func BenchmarkTraversalComparison(b *testing.B) {
	const totalVersions = 50000
	const snapshotInterval = 200
	const traversalSize = 5000
	
	store := setupLargeStore(totalVersions, snapshotInterval)
	
	b.Run("new_planBlobs_approach", func(b *testing.B) {
		consumer := consumer.NewConsumer(
			consumer.WithBlobRetriever(store),
		)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			store.ResetStats()
			
			startVersion := rand.Int63n(int64(totalVersions-traversalSize)) + 1
			targetVersion := startVersion + traversalSize
			
			err := consumer.TriggerRefreshTo(context.Background(), targetVersion)
			if err != nil {
				b.Fatalf("TriggerRefreshTo failed: %v", err)
			}
		}
	})
	
	b.Run("simulate_old_ListVersions_approach", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			store.ResetStats()
			
			startVersion := rand.Int63n(int64(totalVersions-traversalSize)) + 1
			targetVersion := startVersion + traversalSize
			
			// Simulate old approach: expensive ListVersions() call
			versions := store.ListVersions()
			
			// Find nearest snapshot (old way)
			var snapshotVersion int64
			for j := len(versions) - 1; j >= 0; j-- {
				v := versions[j]
				if v <= targetVersion {
					if store.RetrieveSnapshotBlob(v) != nil {
						snapshotVersion = v
						break
					}
				}
			}
			
			// Simulate fetching deltas from snapshot to target
			for v := snapshotVersion + 1; v <= targetVersion; v++ {
				store.RetrieveDeltaBlob(v - 1)
			}
		}
	})
}

// Memory usage benchmark
func BenchmarkMemoryUsage(b *testing.B) {
	const totalVersions = 100000
	const snapshotInterval = 500
	
	store := setupLargeStore(totalVersions, snapshotInterval)
	consumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(store),
	)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		targetVersion := rand.Int63n(totalVersions) + 1
		err := consumer.TriggerRefreshTo(context.Background(), targetVersion)
		if err != nil {
			b.Fatalf("TriggerRefreshTo failed: %v", err)
		}
	}
}

// Stress test with concurrent consumers
func BenchmarkConcurrentTraversal(b *testing.B) {
	const totalVersions = 200000
	const snapshotInterval = 300
	
	store := setupLargeStore(totalVersions, snapshotInterval)
	
	b.RunParallel(func(pb *testing.PB) {
		consumer := consumer.NewConsumer(
			consumer.WithBlobRetriever(store),
		)
		
		for pb.Next() {
			targetVersion := rand.Int63n(totalVersions) + 1
			err := consumer.TriggerRefreshTo(context.Background(), targetVersion)
			if err != nil {
				b.Fatalf("TriggerRefreshTo failed: %v", err)
			}
		}
	})
}
