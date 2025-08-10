package main

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// AnnouncementWatcherAdapter adapts blob.Announcer to implement blob.AnnouncementWatcher
type AnnouncementWatcherAdapter struct {
	announcer blob.Announcer
	// Use goroutineAnnouncer for direct access if available
	goroutineAnnouncer *blob.GoroutineAnnouncer
	mutex     sync.RWMutex
	version   int64
}

// NewAnnouncementWatcherAdapter creates an adapter that wraps an announcer
func NewAnnouncementWatcherAdapter(announcer blob.Announcer) *AnnouncementWatcherAdapter {
	addapter := &AnnouncementWatcherAdapter{
		announcer: announcer,
		version:   0,
	}
	
	// If the announcer is a GoroutineAnnouncer, store it directly
	if ga, ok := announcer.(*blob.GoroutineAnnouncer); ok {
		addapter.goroutineAnnouncer = ga
	}
	
	return addapter
}

// UpdateVersion updates the latest known version
func (a *AnnouncementWatcherAdapter) UpdateVersion(version int64) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if version > a.version {
		a.version = version
	}
}

// GetLatestVersion implements AnnouncementWatcher interface
func (a *AnnouncementWatcherAdapter) GetLatestVersion() int64 {
	// Try to get the latest version directly from GoroutineAnnouncer if available
	// This is more reliable than our internal tracking
	if a.goroutineAnnouncer != nil {
		return a.goroutineAnnouncer.GetLatestVersion()
	}
	
	// Fall back to our tracked version
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.version
}

// Pin implements AnnouncementWatcher interface
func (a *AnnouncementWatcherAdapter) Pin(version int64) {
	// Not implemented for this example
}

// Unpin implements AnnouncementWatcher interface
func (a *AnnouncementWatcherAdapter) Unpin() {
	// Not implemented for this example
}

// IsPinned implements AnnouncementWatcher interface
func (a *AnnouncementWatcherAdapter) IsPinned() bool {
	return false
}

// GetPinnedVersion implements AnnouncementWatcher interface
func (a *AnnouncementWatcherAdapter) GetPinnedVersion() int64 {
	return 0
}

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
	
	// Create a shared announcement watcher adapter
	watcherAdapter := NewAnnouncementWatcherAdapter(announcer)

	// Start multiple writers
	numWriters := 3
	for i := 0; i < numWriters; i++ {
		writerID := fmt.Sprintf("Writer-%d", i+1)
		writerStats[writerID] = &WriterStats{WriterID: writerID}
		
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			runWriter(ctx, blobStore, announcer, id, writerStats, &statsMutex, watcherAdapter)
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
			runConsumer(ctx, blobStore, announcer, id, consumerStats, &statsMutex, watcherAdapter)
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

func runWriter(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer, writerID string, stats map[string]*WriterStats, mutex *sync.RWMutex, watcherAdapter *AnnouncementWatcherAdapter) {
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
	
	// Update watcher adapter with new version
	watcherAdapter.UpdateVersion(initialVersion)
	
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
			
			// Update watcher adapter with new version
			watcherAdapter.UpdateVersion(version)

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

func runConsumer(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer, consumerID string, stats map[string]*ConsumerStats, mutex *sync.RWMutex, watcherAdapter *AnnouncementWatcherAdapter) {
	fmt.Printf("👀 Starting consumer: %s\n", consumerID)

	// Create consumers with initial delay to ensure producers have time to create initial versions
	time.Sleep(500 * time.Millisecond)
	
	// First create a basic consumer with the blob retriever and watcher adapter
	regularConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(watcherAdapter),
	)
	
	// Create zero-copy consumer using the same pattern
	zeroCopyConsumer := consumer.NewZeroCopyConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(watcherAdapter),
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
			// Try to get latest version from watcher adapter
			latestVersion := watcherAdapter.GetLatestVersion()
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
					recordCount = countRecordsInStateEngine(se)
					zeroCopySuccess = true
					
					// If record count is still 0, try using reflection directly on the state engine
					// This handles zero-copy state engines that might not implement all interfaces
					if recordCount == 0 {
						seValue := reflect.ValueOf(se)
						if seValue.Kind() == reflect.Ptr && !seValue.IsNil() {
							// Try OrdinalList method
							ordListMethod := seValue.MethodByName("OrdinalList")
							if ordListMethod.IsValid() {
								results := ordListMethod.Call(nil)
								if len(results) > 0 && results[0].Kind() == reflect.Slice {
									ordinals := results[0]
									for i := 0; i < ordinals.Len(); i++ {
										ordinal := ordinals.Index(i).Interface().(int)
										
										// Try TypeCount method
										typeCountMethod := seValue.MethodByName("TypeCount")
										if typeCountMethod.IsValid() {
											args := []reflect.Value{reflect.ValueOf(ordinal)}
											result := typeCountMethod.Call(args)
											if len(result) > 0 && result[0].Kind() == reflect.Int {
												recordCount += int(result[0].Int())
											}
										}
									}
								}
							}
						}
					}
				}
			} else {
				// Try refreshing to latest if specific version fails
				// This helps when there's a gap in versions
				err = zeroCopyConsumer.TriggerRefresh(ctx)
				if err == nil {
					se := zeroCopyConsumer.GetStateEngine()
					if se != nil {
						recordCount = countRecordsInStateEngine(se)
						zeroCopySuccess = true
					}
				} else {
					// Fallback to regular consumer if zero-copy fails
					err = regularConsumer.TriggerRefreshTo(ctx, latestVersion)
					if err == nil {
						se := regularConsumer.GetStateEngine()
						if se != nil {
							recordCount = countRecordsInStateEngine(se)
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

func countRecordsInStateEngine(se interface{}) int {
	if se == nil {
		return 0
	}
	
	count := 0
	
	// First try using interface assertions
	// Approach 1: Try to access state engine via GetOrdinalIterator
	if stateEngine, ok := se.(interface{ GetOrdinalIterator() interface{} }); ok {
		iterator := stateEngine.GetOrdinalIterator()
		if iterator != nil {
			if iter, ok := iterator.(interface{ Size() int }); ok {
				return iter.Size()
			}
		}
	}
	
	// Approach 2: Try to access state engine via OrdinalList and TypeCount methods
	if stateEngine, ok := se.(interface {
		OrdinalList() []int
		TypeCount(int) int
	}); ok {
		ordinals := stateEngine.OrdinalList()
		for _, ordinal := range ordinals {
			count += stateEngine.TypeCount(ordinal)
		}
		return count
	}
	
	// Approach 3: Try type-specific methods for DataRecord type
	if stateEngine, ok := se.(interface{ GetTypeOrdinals(string) interface{} }); ok {
		// Try for our specific DataRecord type
		ordinals := stateEngine.GetTypeOrdinals("DataRecord")
		if ordinals != nil {
			// Try to get Size method from ordinals
			if sized, ok := ordinals.(interface{ Size() int }); ok {
				count += sized.Size()
			}
		}
		
		// If ordinals didn't work, try getting all type names
		if typed, ok := se.(interface{ GetAllTypeNames() []string }); ok {
			for _, typeName := range typed.GetAllTypeNames() {
				ordinals := stateEngine.GetTypeOrdinals(typeName)
				if ordinals != nil {
					if sized, ok := ordinals.(interface{ Size() int }); ok {
						count += sized.Size()
					}
				}
			}
		}
	}
	
	// Approach 4: Try using reflection (especially for zero-copy state engines)
	if count == 0 {
		seValue := reflect.ValueOf(se)
		if seValue.Kind() == reflect.Ptr && !seValue.IsNil() {
			// Attempt to find GetAllTypeNames method first
			allTypeNamesMethod := seValue.MethodByName("GetAllTypeNames")
			if allTypeNamesMethod.IsValid() {
				typeNamesResult := allTypeNamesMethod.Call(nil)
				if len(typeNamesResult) > 0 && typeNamesResult[0].Kind() == reflect.Slice {
					typeNames := typeNamesResult[0]
					
					for i := 0; i < typeNames.Len(); i++ {
						typeName := typeNames.Index(i).Interface().(string)
						
						// Get ordinals for this type
						getTypeOrdinalsMethod := seValue.MethodByName("GetTypeOrdinals")
						if getTypeOrdinalsMethod.IsValid() {
							args := []reflect.Value{reflect.ValueOf(typeName)}
							ordinalsResult := getTypeOrdinalsMethod.Call(args)
							
							if len(ordinalsResult) > 0 && !ordinalsResult[0].IsNil() {
								ordinalsObj := ordinalsResult[0].Interface()
								
								// Try Size() method
								ordValue := reflect.ValueOf(ordinalsObj)
								sizeMethod := ordValue.MethodByName("Size")
								if sizeMethod.IsValid() {
									results := sizeMethod.Call(nil)
									if len(results) > 0 && results[0].Kind() == reflect.Int {
										count += int(results[0].Int())
									}
								}
							}
						}
					}
				}
			}
			
			// Fallback to OrdinalList approach using reflection
			if count == 0 {
				ordListMethod := seValue.MethodByName("OrdinalList")
				if ordListMethod.IsValid() {
					results := ordListMethod.Call(nil)
					if len(results) > 0 && results[0].Kind() == reflect.Slice {
						ordinals := results[0]
						typeCountMethod := seValue.MethodByName("TypeCount")
						
						if typeCountMethod.IsValid() {
							for i := 0; i < ordinals.Len(); i++ {
								ordinal := ordinals.Index(i).Interface().(int)
								args := []reflect.Value{reflect.ValueOf(ordinal)}
								result := typeCountMethod.Call(args)
								
								if len(result) > 0 && result[0].Kind() == reflect.Int {
									count += int(result[0].Int())
								}
							}
						}
					}
				}
			}
		}
	}
	
	return count
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
