package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// Benchmark test data size configurations
const (
	SmallDataset  = 1000     // 1K records
	MediumDataset = 100000   // 100K records  
	LargeDataset  = 1000000  // 1M records
)

// BenchmarkProducerSnapshot measures producer performance for snapshot creation
func BenchmarkProducerSnapshot(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1K", SmallDataset},
		{"100K", MediumDataset},
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Records_%s", size.name), func(b *testing.B) {
			benchmarkProducerWithSize(b, size.size)
		})
	}
}

func benchmarkProducerWithSize(b *testing.B, recordCount int) {
	ctx := context.Background()
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	// Pre-generate test data to avoid allocation overhead in benchmark
	movies := generateMovieData(recordCount)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		version, err := prod.RunCycleE(ctx, func(ws *internal.WriteState) {
			for _, movieData := range movies {
				ws.Add(movieData)
			}
		})

		if err != nil {
			b.Fatalf("Producer cycle failed: %v", err)
		}
		if version == 0 {
			b.Fatalf("Expected non-zero version")
		}
	}

	// Report performance metrics
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	recordsPerSec := opsPerSec * float64(recordCount)
	b.ReportMetric(recordsPerSec, "records/sec")
	b.ReportMetric(float64(recordCount*b.N)/1e6, "total_million_records")
}

// BenchmarkConsumerRefresh measures consumer refresh performance  
func BenchmarkConsumerRefresh(b *testing.B) {
	ctx := context.Background()
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Setup producer with data
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	movies := generateMovieData(MediumDataset)
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		for _, movieData := range movies {
			ws.Add(movieData)
		}
	})

	// Setup consumer
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(announcer),
	)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := cons.TriggerRefreshTo(ctx, int64(version))
		if err != nil {
			b.Fatalf("Consumer refresh failed: %v", err)
		}

		currentVersion := cons.GetCurrentVersion()
		if currentVersion != int64(version) {
			b.Fatalf("Expected version %d, got %d", version, currentVersion)
		}
	}

	refreshPerSec := float64(b.N) / b.Elapsed().Seconds()
	recordsPerRefresh := float64(len(movies))
	recordsPerSec := refreshPerSec * recordsPerRefresh
	b.ReportMetric(refreshPerSec, "refreshes/sec")
	b.ReportMetric(recordsPerSec, "records_consumed/sec")
}

// BenchmarkAnnouncerThroughput measures announcer performance under load
func BenchmarkAnnouncerThroughput(b *testing.B) {
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Setup multiple subscribers
	numSubscribers := 10
	subscribers := make([]chan int64, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		subscribers[i] = make(chan int64, 1000)
		announcer.Subscribe(subscribers[i])
	}
	defer func() {
		for _, sub := range subscribers {
			announcer.Unsubscribe(sub)
			close(sub)
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := announcer.Announce(int64(i))
		if err != nil {
			b.Fatalf("Announcement failed: %v", err)
		}
	}

	// Wait for all announcements to be processed
	time.Sleep(100 * time.Millisecond)

	announcementsPerSec := float64(b.N) / b.Elapsed().Seconds()
	totalDeliveries := announcementsPerSec * float64(numSubscribers)
	b.ReportMetric(announcementsPerSec, "announcements/sec")
	b.ReportMetric(totalDeliveries, "total_deliveries/sec")
}

// BenchmarkDataAccess measures latency of data access operations
func BenchmarkDataAccess(b *testing.B) {
	ctx := context.Background()
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Setup producer and consumer
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	movies := generateMovieData(SmallDataset) // Smaller dataset for access patterns
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		for _, movieData := range movies {
			ws.Add(movieData)
		}
	})

	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(announcer),
	)
	cons.TriggerRefreshTo(ctx, int64(version))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate typical data access patterns
		currentVersion := cons.GetCurrentVersion()
		if currentVersion != int64(version) {
			b.Fatalf("Unexpected version: %d", currentVersion)
		}
	}

	// Calculate access latency
	avgLatency := b.Elapsed().Nanoseconds() / int64(b.N)
	b.ReportMetric(float64(avgLatency), "ns/access")
	b.ReportMetric(float64(avgLatency)/1000.0, "μs/access")
}

// BenchmarkMemoryFootprint measures memory usage patterns
func BenchmarkMemoryFootprint(b *testing.B) {
	ctx := context.Background()
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	movieBatch := generateMovieData(1000) // 1K batch

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		version, err := prod.RunCycleE(ctx, func(ws *internal.WriteState) {
			for _, movieData := range movieBatch {
				ws.Add(movieData)
			}
		})

		if err != nil {
			b.Fatalf("Cycle failed: %v", err)
		}
		if version == 0 {
			b.Fatalf("Expected non-zero version")
		}
	}

	// Report basic memory metrics
	if b.N > 0 {
		bytesPerOp := float64(testing.AllocsPerRun(1, func() {
			version, _ := prod.RunCycleE(ctx, func(ws *internal.WriteState) {
				for _, movieData := range movieBatch {
					ws.Add(movieData)
				}
			})
			_ = version
		}))
		b.ReportMetric(bytesPerOp, "allocs/operation")
	}
}

// Helper function to generate test movie data
func generateMovieData(count int) []TestMovieData {
	movies := make([]TestMovieData, count)
	genres := []string{"Action", "Drama", "Comedy", "Thriller", "Sci-Fi", "Romance"}
	
	for i := 0; i < count; i++ {
		movies[i] = TestMovieData{
			ID:         uint32(i + 1),
			Title:      fmt.Sprintf("Test Movie %d", i+1),
			Year:       uint16(1990 + (i % 35)), // 1990-2024
			RuntimeMin: uint16(90 + (i % 90)),   // 90-180 minutes
			Genres:     []string{genres[i%len(genres)], genres[(i+1)%len(genres)]},
		}
	}
	
	return movies
}

// TestMovieData represents our test data structure
type TestMovieData struct {
	ID         uint32
	Title      string
	Year       uint16
	RuntimeMin uint16
	Genres     []string
}
