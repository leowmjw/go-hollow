package hollow

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/leowmjw/go-hollow/legacy/internal/memblob"
)

// BenchmarkReadLatency tests the ≤5µs p99 requirement for primitive lookups
func BenchmarkReadLatency(b *testing.B) {
	// Setup test data
	blob := memblob.New()
	producer := NewProducer(WithBlobStager(blob))
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			latest, err := blob.Latest(context.Background())
			return latest, latest > 0, err
		}),
	)

	// Create dataset (1M records as per PRD)
	numRecords := 1000000
	batchSize := 10000

	for i := 0; i < numRecords; i += batchSize {
		end := i + batchSize
		if end > numRecords {
			end = numRecords
		}

		_, err := producer.RunCycle(func(ws WriteState) error {
			for j := i; j < end; j++ {
				key := fmt.Sprintf("key_%d", j)
				if err := ws.Add(key); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			b.Fatalf("Failed to create test data: %v", err)
		}
	}

	// Refresh consumer
	if err := consumer.Refresh(); err != nil {
		b.Fatalf("Failed to refresh consumer: %v", err)
	}

	rs := consumer.ReadState()
	b.Logf("Dataset size: %d records", rs.Size())

	// Benchmark read performance
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i%numRecords)
			_, exists := rs.Get(key)
			if !exists {
				// Some keys might not exist, that's expected
			}
			i++
		}
	})
}

// BenchmarkReadLatencySmall tests read performance on smaller dataset
func BenchmarkReadLatencySmall(b *testing.B) {
	// Setup test data
	blob := memblob.New()
	producer := NewProducer(WithBlobStager(blob))
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			latest, err := blob.Latest(context.Background())
			return latest, latest > 0, err
		}),
	)

	// Create small dataset (10K records)
	numRecords := 10000

	_, err := producer.RunCycle(func(ws WriteState) error {
		for i := 0; i < numRecords; i++ {
			key := fmt.Sprintf("key_%d", i)
			if err := ws.Add(key); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		b.Fatalf("Failed to create test data: %v", err)
	}

	// Refresh consumer
	if err := consumer.Refresh(); err != nil {
		b.Fatalf("Failed to refresh consumer: %v", err)
	}

	rs := consumer.ReadState()
	b.Logf("Dataset size: %d records", rs.Size())

	// Benchmark read performance
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%numRecords)
		_, exists := rs.Get(key)
		if !exists {
			// Some keys might not exist, that's expected
		}
	}
}

// BenchmarkProducerCycle tests producer performance
func BenchmarkProducerCycle(b *testing.B) {
	blob := memblob.New()
	producer := NewProducer(WithBlobStager(blob))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := producer.RunCycle(func(ws WriteState) error {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", i, j)
				if err := ws.Add(key); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			b.Fatalf("Producer cycle failed: %v", err)
		}
	}
}

// BenchmarkConsumerRefresh tests consumer refresh performance
func BenchmarkConsumerRefresh(b *testing.B) {
	blob := memblob.New()
	producer := NewProducer(WithBlobStager(blob))
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			latest, err := blob.Latest(context.Background())
			return latest, latest > 0, err
		}),
	)

	// Create some test data
	_, err := producer.RunCycle(func(ws WriteState) error {
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("key_%d", i)
			if err := ws.Add(key); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		b.Fatalf("Failed to create test data: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := consumer.Refresh(); err != nil {
			b.Fatalf("Consumer refresh failed: %v", err)
		}
	}
}

// BenchmarkDiffEngine tests diff computation performance
func BenchmarkDiffEngine(b *testing.B) {
	// Create test data
	oldData := make(map[string]any)
	newData := make(map[string]any)

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		oldData[key] = fmt.Sprintf("old_value_%d", i)
		newData[key] = fmt.Sprintf("new_value_%d", i)
	}

	// Add some changes
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("changed_key_%d", i)
		oldData[key] = "old_value"
		newData[key] = "new_value"
	}

	// Add some new keys
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("new_key_%d", i)
		newData[key] = "new_value"
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		diff := DiffData(oldData, newData)
		if diff.IsEmpty() {
			b.Fatal("Expected non-empty diff")
		}
	}
}

// TestReadLatencyRequirement validates the ≤5µs p99 requirement
func TestReadLatencyRequirement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping latency requirement test in short mode")
	}

	// Setup test data
	blob := memblob.New()
	producer := NewProducer(WithBlobStager(blob))
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			latest, err := blob.Latest(context.Background())
			return latest, latest > 0, err
		}),
	)

	// Create dataset (smaller for test)
	numRecords := 100000
	batchSize := 10000

	t.Logf("Creating dataset with %d records...", numRecords)

	for i := 0; i < numRecords; i += batchSize {
		end := i + batchSize
		if end > numRecords {
			end = numRecords
		}

		_, err := producer.RunCycle(func(ws WriteState) error {
			for j := i; j < end; j++ {
				key := fmt.Sprintf("key_%d", j)
				if err := ws.Add(key); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to create test data: %v", err)
		}
	}

	// Refresh consumer
	if err := consumer.Refresh(); err != nil {
		t.Fatalf("Failed to refresh consumer: %v", err)
	}

	rs := consumer.ReadState()
	t.Logf("Dataset size: %d records", rs.Size())

	// Measure read latencies
	numMeasurements := 10000
	latencies := make([]time.Duration, numMeasurements)

	for i := 0; i < numMeasurements; i++ {
		key := fmt.Sprintf("key_%d", i%numRecords)

		start := time.Now()
		_, exists := rs.Get(key)
		latency := time.Since(start)

		latencies[i] = latency

		if !exists {
			// Some keys might not exist, that's expected
		}
	}

	// Sort latencies for percentile calculation
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	// Calculate percentiles
	p50 := latencies[len(latencies)/2]
	p95 := latencies[int(float64(len(latencies))*0.95)]
	p99 := latencies[int(float64(len(latencies))*0.99)]

	t.Logf("Read latency percentiles:")
	t.Logf("  p50: %v", p50)
	t.Logf("  p95: %v", p95)
	t.Logf("  p99: %v", p99)

	// Validate requirement
	requirement := 5 * time.Microsecond
	if p99 > requirement {
		t.Errorf("Read latency requirement not met: p99=%v > %v", p99, requirement)
		t.Logf("Current implementation may need optimization for production use")
	} else {
		t.Logf("✅ Read latency requirement met: p99=%v ≤ %v", p99, requirement)
	}
}
