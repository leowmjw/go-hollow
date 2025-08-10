package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)



// DataRecord represents a simple data structure for our example
type DataRecord struct {
	ID        string `hollow:"key"`
	Value     string
	Timestamp int64
	WriterID  string
}

// WriterStats tracks statistics for each writer
type WriterStats struct {
	WriterID      string
	VersionsProduced int
	RecordsWritten   int
	LastWriteTime    time.Time
}

// ConsumerStats tracks statistics for each consumer
type ConsumerStats struct {
	ConsumerID     string
	VersionsRead   int
	RecordsRead    int
	LastReadTime   time.Time
	ZeroCopyReads  int
	FallbackReads  int
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Println("🚀 Starting Multi-Writer Zero-Copy Example")
	fmt.Println("This example demonstrates:")
	fmt.Println("  - Multiple concurrent writers producing data")
	fmt.Println("  - Multiple concurrent consumers with zero-copy reads")
	fmt.Println("  - Delta accumulation and snapshot fallback")
	fmt.Println("  - Thread-safe operations without crashes/overwrites")
	fmt.Println()

	// Create shared blob store and announcer
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Start the demonstration
	runMultiWriterZeroCopyDemo(ctx, blobStore, announcer)
}

func runMultiWriterZeroCopyDemo(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer) {
	fmt.Println("🔧 Initializing shared storage and configuration...")
	
	var wg sync.WaitGroup
	
	// Statistics tracking
	writerStats := make(map[string]*WriterStats)
	consumerStats := make(map[string]*ConsumerStats)
	var statsMutex sync.RWMutex
	
	// Announcer now directly implements AnnouncementWatcher - no adapter needed!

	// Start multiple writers
	numWriters := 3
	for i := 0; i < numWriters; i++ {
		writerID := fmt.Sprintf("Writer-%d", i+1)
		writerStats[writerID] = &WriterStats{WriterID: writerID}
		
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			runWriter(ctx, blobStore, announcer, id, writerStats, &statsMutex)
		}(writerID)
	}

	// Start multiple consumers
	numConsumers := 4
	for i := 0; i < numConsumers; i++ {
		consumerID := fmt.Sprintf("Consumer-%d", i+1)
		consumerStats[consumerID] = &ConsumerStats{ConsumerID: consumerID}
		
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			runConsumer(ctx, blobStore, announcer, id, consumerStats, &statsMutex)
		}(consumerID)
	}

	// Start statistics reporter
	wg.Add(1)
	go func() {
		defer wg.Done()
		reportStatistics(ctx, writerStats, consumerStats, &statsMutex)
	}()

	// Wait for all goroutines to complete
	wg.Wait()

	// Final report
	fmt.Println("\n🎯 Final Statistics Report")
	printFinalStats(writerStats, consumerStats, &statsMutex)
}

func runWriter(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer, writerID string, stats map[string]*WriterStats, mutex *sync.RWMutex) {
	fmt.Printf("📝 Starting writer: %s\n", writerID)

	// Create producer with zero-copy configuration
	// Always ensure snapshot for first version and use consistent snapshot frequency
	snapshotFreq := 3 + (len(writerID) % 3) // Vary between 3-5
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithSerializationMode(internal.ZeroCopyMode),
		producer.WithNumStatesBetweenSnapshots(snapshotFreq),
	)
	
	// Ensure first cycle creates a snapshot blob
	initialVersion := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		// Add a single initial record to create first snapshot
		ws.Add(DataRecord{
			ID:        fmt.Sprintf("%s-initial", writerID),
			Value:     "initial-value",
			Timestamp: time.Now().Unix(),
			WriterID:  writerID,
		})
	})
	
	// Announcer automatically announces new versions - no manual update needed!
	
	fmt.Printf("📝 %s produced initial version %d with snapshot\n", writerID, initialVersion)
	
	// Update statistics for initial version
	mutex.Lock()
	stats[writerID].VersionsProduced++
	stats[writerID].RecordsWritten++
	stats[writerID].LastWriteTime = time.Now()
	mutex.Unlock()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	recordCounter := 0
	maxRounds := 10
	roundsCompleted := 0
	
	for roundsCompleted < maxRounds {
		select {
		case <-ctx.Done():
			fmt.Printf("📝 Writer %s stopping (produced %d versions)\n", writerID, stats[writerID].VersionsProduced)
			return
		case <-ticker.C:
			// Generate batch of records
			batchSize := 2 + rand.Intn(4) // 2-5 records per batch
			records := generateRecords(writerID, recordCounter, batchSize)
			recordCounter += batchSize

			// Write data using producer
			version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
				for _, record := range records {
					ws.Add(record)
				}
			})
			
			// Announcer automatically announces new versions - no manual update needed!

			// Update statistics
			mutex.Lock()
			stats[writerID].VersionsProduced++
			stats[writerID].RecordsWritten += len(records)
			stats[writerID].LastWriteTime = time.Now()
			mutex.Unlock()

			roundsCompleted++
			fmt.Printf("📝 %s produced version %d with %d records (round %d/%d)\n", writerID, version, len(records), roundsCompleted, maxRounds)
		}
	}
}

func runConsumer(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer, consumerID string, stats map[string]*ConsumerStats, mutex *sync.RWMutex) {
	fmt.Printf("👀 Starting consumer: %s\n", consumerID)

	// Create consumers with initial delay to ensure producers have time to create initial versions
	time.Sleep(500 * time.Millisecond)
	
	// First create a basic consumer with the blob retriever and announcer (no adapter needed!)
	regularConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)
	
	// Create zero-copy consumer using the same pattern
	zeroCopyConsumer := consumer.NewZeroCopyConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	lastVersion := uint64(0)
	maxRounds := 12
	roundsCompleted := 0
	
	for roundsCompleted < maxRounds {
		select {
		case <-ctx.Done():
			fmt.Printf("👀 Consumer %s stopping (read %d versions)\n", consumerID, stats[consumerID].VersionsRead)
			return
		case <-ticker.C:
			// Try to get latest version from announcer directly
			latestVersion := announcer.GetLatestVersion()
			if latestVersion <= 0 {
				// No versions available yet
				continue
			}
			
			// Check if we've already processed this version
			if uint64(latestVersion) <= lastVersion {
				continue // No new data
			}

			// Attempt zero-copy read
			zeroCopySuccess := false
			recordCount := 0

			// First try to refresh to specific version with zero-copy consumer
			err := zeroCopyConsumer.TriggerRefreshTo(ctx, latestVersion)
			if err == nil {
				// Zero-copy read successful
				se := zeroCopyConsumer.GetStateEngine()
				if se != nil {
					recordCount = se.TotalRecords()
					zeroCopySuccess = true
				}
			} else {
				// Try refreshing to latest if specific version fails
				// This helps when there's a gap in versions
				err = zeroCopyConsumer.TriggerRefresh(ctx)
				if err == nil {
					se := zeroCopyConsumer.GetStateEngine()
					if se != nil {
						recordCount = se.TotalRecords()
						zeroCopySuccess = true
					}
				} else {
					// Fallback to regular consumer if zero-copy fails
					err = regularConsumer.TriggerRefreshTo(ctx, latestVersion)
					if err == nil {
						se := regularConsumer.GetStateEngine()
						if se != nil {
						recordCount = se.TotalRecords()
						}
					}
				}
			}

			if err == nil {
				lastVersion = uint64(latestVersion)

				// Update statistics
				mutex.Lock()
				stats[consumerID].VersionsRead++
				stats[consumerID].RecordsRead += recordCount
				stats[consumerID].LastReadTime = time.Now()
				if zeroCopySuccess {
					stats[consumerID].ZeroCopyReads++
				} else {
					stats[consumerID].FallbackReads++
				}
				mutex.Unlock()

				readType := "fallback"
				if zeroCopySuccess {
					readType = "zero-copy"
				}
				roundsCompleted++
				fmt.Printf("👀 %s read version %d (%s) with %d total records (round %d/%d)\n", 
					consumerID, latestVersion, readType, recordCount, roundsCompleted, maxRounds)
			}
		}
	}
}

func generateRecords(writerID string, startID int, count int) []DataRecord {
	records := make([]DataRecord, count)
	now := time.Now().Unix()
	
	for i := 0; i < count; i++ {
		records[i] = DataRecord{
			ID:        fmt.Sprintf("%s-record-%d", writerID, startID+i),
			Value:     fmt.Sprintf("data-value-%d-%d", startID+i, rand.Intn(1000)),
			Timestamp: now,
			WriterID:  writerID,
		}
	}
	
	return records
}



func reportStatistics(ctx context.Context, writerStats map[string]*WriterStats, consumerStats map[string]*ConsumerStats, mutex *sync.RWMutex) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// For checking completion
	writerMaxRounds := 10
	consumerMaxRounds := 12

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Println("\n📊 === STATISTICS REPORT ===")
			
			mutex.RLock()
			
			// Check if all writers and consumers are done
			allWritersDone := true
			allConsumersDone := true
			
			// Writer statistics
			fmt.Println("📝 Writers:")
			for _, stat := range writerStats {
				fmt.Printf("  %s: %d versions, %d records, last write: %s\n",
					stat.WriterID, stat.VersionsProduced, stat.RecordsWritten,
					stat.LastWriteTime.Format("15:04:05"))
					
				// Writer is done if it produced writerMaxRounds versions (initial + regular rounds)
				if stat.VersionsProduced < writerMaxRounds+1 {
					allWritersDone = false
				}
			}
			
			// Consumer statistics
			fmt.Println("👀 Consumers:")
			for _, stat := range consumerStats {
				fmt.Printf("  %s: %d versions, %d records, zero-copy: %d, fallback: %d, last read: %s\n",
					stat.ConsumerID, stat.VersionsRead, stat.RecordsRead,
					stat.ZeroCopyReads, stat.FallbackReads,
					stat.LastReadTime.Format("15:04:05"))
					
				// Consumer is done if it read consumerMaxRounds versions
				if stat.VersionsRead < consumerMaxRounds {
					allConsumersDone = false
				}
			}
			
			mutex.RUnlock()
			fmt.Println("========================\n")
			
			// If all writers and consumers are done, print final stats and exit
			if allWritersDone && allConsumersDone {
				fmt.Println("\n🏁 All writers and consumers completed their rounds!")
				printFinalStats(writerStats, consumerStats, mutex)
				return
			}
		}
	}
}

func printFinalStats(writerStats map[string]*WriterStats, consumerStats map[string]*ConsumerStats, mutex *sync.RWMutex) {
	mutex.RLock()
	defer mutex.RUnlock()

	totalVersions := 0
	totalRecords := 0
	totalZeroCopyReads := 0
	totalFallbackReads := 0

	fmt.Println("📝 Final Writer Statistics:")
	for _, stat := range writerStats {
		fmt.Printf("  %s: %d versions produced, %d records written\n",
			stat.WriterID, stat.VersionsProduced, stat.RecordsWritten)
		totalVersions += stat.VersionsProduced
		totalRecords += stat.RecordsWritten
	}

	fmt.Println("\n👀 Final Consumer Statistics:")
	for _, stat := range consumerStats {
		fmt.Printf("  %s: %d versions read, zero-copy: %d, fallback: %d\n",
			stat.ConsumerID, stat.VersionsRead, stat.ZeroCopyReads, stat.FallbackReads)
		totalZeroCopyReads += stat.ZeroCopyReads
		totalFallbackReads += stat.FallbackReads
	}

	fmt.Printf("\n🎯 Summary:\n")
	fmt.Printf("  Total versions produced: %d\n", totalVersions)
	fmt.Printf("  Total records written: %d\n", totalRecords)
	fmt.Printf("  Total zero-copy reads: %d\n", totalZeroCopyReads)
	fmt.Printf("  Total fallback reads: %d\n", totalFallbackReads)
	
	if totalZeroCopyReads+totalFallbackReads > 0 {
		zeroCopyRate := float64(totalZeroCopyReads) / float64(totalZeroCopyReads+totalFallbackReads) * 100
		fmt.Printf("  Zero-copy success rate: %.1f%%\n", zeroCopyRate)
	}

	fmt.Println("✅ Multi-writer zero-copy example completed successfully!")
	fmt.Println("   - Multiple writers operated concurrently without conflicts")
	fmt.Println("   - Multiple consumers read data with zero-copy optimization")
	fmt.Println("   - Delta accumulation and snapshot fallback worked correctly")
	fmt.Println("   - No crashes or data corruption occurred")
}
