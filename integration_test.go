package hollow

import (
	"sync"
	"testing"
	"time"

	"github.com/leowmjw/go-hollow/internal/memblob"
)

func TestIntegrationProducerConsumerFlow(t *testing.T) {
	blob := memblob.New()

	// Create producer
	producer := NewProducer(WithBlobStager(blob))

	// Create consumer with announcement watcher
	var currentVersion uint64
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			return currentVersion, currentVersion > 0, nil
		}),
	)

	// Producer creates first version
	v1, err := producer.RunCycle(func(ws WriteState) error {
		return ws.Add("test-data")
	})
	if err != nil {
		t.Fatalf("producer cycle failed: %v", err)
	}

	// Update announcement
	currentVersion = v1

	// Consumer refreshes to first version
	if err := consumer.Refresh(); err != nil {
		t.Fatalf("consumer refresh failed: %v", err)
	}

	if consumer.CurrentVersion() != v1 {
		t.Fatalf("expected version %d, got %d", v1, consumer.CurrentVersion())
	}

	// Verify data is accessible
	rs := consumer.ReadState()
	if val, ok := rs.Get("test-data"); !ok || val != "test-data" {
		t.Fatalf("expected test-data to exist, got %v, %v", val, ok)
	}

	// Producer creates second version
	v2, err := producer.RunCycle(func(ws WriteState) error {
		if err := ws.Add("data-v2"); err != nil {
			return err
		}
		return ws.Add("more-data")
	})
	if err != nil {
		t.Fatalf("producer second cycle failed: %v", err)
	}

	// Update announcement
	currentVersion = v2

	// Consumer refreshes to second version
	if err := consumer.Refresh(); err != nil {
		t.Fatalf("consumer refresh to v2 failed: %v", err)
	}

	if consumer.CurrentVersion() != v2 {
		t.Fatalf("expected version %d, got %d", v2, consumer.CurrentVersion())
	}

	// Verify new data is accessible
	rs = consumer.ReadState()
	if rs.Size() != 2 {
		t.Fatalf("expected size 2, got %d", rs.Size())
	}

	if val, ok := rs.Get("data-v2"); !ok || val != "data-v2" {
		t.Fatalf("expected data-v2 to exist, got %v, %v", val, ok)
	}

	if val, ok := rs.Get("more-data"); !ok || val != "more-data" {
		t.Fatalf("expected more-data to exist, got %v, %v", val, ok)
	}
}

func TestIntegrationConcurrentProducerConsumer(t *testing.T) {
	blob := memblob.New()

	// Create producer
	producer := NewProducer(WithBlobStager(blob))

	// Shared state for announcements
	var mu sync.RWMutex
	var latestVersion uint64

	// Create consumer
	consumer := NewConsumer(
		WithBlobRetriever(blob),
		WithAnnouncementWatcher(func() (uint64, bool, error) {
			mu.RLock()
			defer mu.RUnlock()
			return latestVersion, latestVersion > 0, nil
		}),
	)

	// Producer goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			version, err := producer.RunCycle(func(ws WriteState) error {
				return ws.Add(i)
			})
			if err != nil {
				t.Errorf("producer cycle %d failed: %v", i, err)
				return
			}

			// Update announcement
			mu.Lock()
			latestVersion = version
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Consumer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			if err := consumer.Refresh(); err != nil {
				t.Errorf("consumer refresh %d failed: %v", i, err)
				return
			}
			time.Sleep(15 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Verify final state
	finalVersion := consumer.CurrentVersion()
	if finalVersion == 0 {
		t.Fatal("expected non-zero final version")
	}

	rs := consumer.ReadState()
	if rs.Size() != 1 {
		t.Fatalf("expected size 1, got %d", rs.Size())
	}
}

func TestIntegrationMultipleConsumers(t *testing.T) {
	blob := memblob.New()

	// Create producer
	producer := NewProducer(WithBlobStager(blob))

	// Create test data
	version, err := producer.RunCycle(func(ws WriteState) error {
		if err := ws.Add("shared-data"); err != nil {
			return err
		}
		return ws.Add("more-shared-data")
	})
	if err != nil {
		t.Fatalf("producer cycle failed: %v", err)
	}

	// Create multiple consumers
	consumers := make([]*Consumer, 3)
	for i := 0; i < 3; i++ {
		consumers[i] = NewConsumer(
			WithBlobRetriever(blob),
			WithAnnouncementWatcher(func() (uint64, bool, error) {
				return version, true, nil
			}),
		)
	}

	// All consumers refresh
	for i, consumer := range consumers {
		if err := consumer.Refresh(); err != nil {
			t.Fatalf("consumer %d refresh failed: %v", i, err)
		}

		if consumer.CurrentVersion() != version {
			t.Fatalf("consumer %d expected version %d, got %d", i, version, consumer.CurrentVersion())
		}

		// Verify all have the same data
		rs := consumer.ReadState()
		if rs.Size() != 2 {
			t.Fatalf("consumer %d expected size 2, got %d", i, rs.Size())
		}

		if val, ok := rs.Get("shared-data"); !ok || val != "shared-data" {
			t.Fatalf("consumer %d expected shared-data, got %v, %v", i, val, ok)
		}
	}
}

func TestIntegrationMetricsCollection(t *testing.T) {
	blob := memblob.New()

	var collectedMetrics []Metrics
	var mu sync.Mutex

	producer := NewProducer(
		WithBlobStager(blob),
		WithMetricsCollector(func(m Metrics) {
			mu.Lock()
			collectedMetrics = append(collectedMetrics, m)
			mu.Unlock()
		}),
	)

	// Run multiple cycles
	for i := 0; i < 5; i++ {
		_, err := producer.RunCycle(func(ws WriteState) error {
			return ws.Add(i)
		})
		if err != nil {
			t.Fatalf("cycle %d failed: %v", i, err)
		}
	}

	// Check metrics
	mu.Lock()
	defer mu.Unlock()

	if len(collectedMetrics) != 5 {
		t.Fatalf("expected 5 metrics collections, got %d", len(collectedMetrics))
	}

	// Verify metrics are in ascending order
	for i := 1; i < len(collectedMetrics); i++ {
		if collectedMetrics[i].Version <= collectedMetrics[i-1].Version {
			t.Fatalf("expected ascending versions, got %d <= %d",
				collectedMetrics[i].Version, collectedMetrics[i-1].Version)
		}
	}
}
