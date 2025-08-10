package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/producer"
)

// DataRecord represents the data structure we'll be producing
type DataRecord struct {
	ID        string `hollow:"key"`
	Value     string
	Timestamp int64
	Source    string
}

// ProductionProducer demonstrates production-ready patterns for producers
type ProductionProducer struct {
	producerID string
	producer   *producer.Producer
	
	// Batching state
	pendingRecords []DataRecord
	pendingMutex   sync.Mutex
	batchSize      int
	batchTimeout   time.Duration
	lastFlush      time.Time
	
	// Statistics
	stats struct {
		sync.RWMutex
		versionsProduced uint64
		recordsWritten   uint64
		errors           uint64
		startTime        time.Time
	}
	
	// Resource management
	flushTicker *time.Ticker
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewProductionProducer(producerID string, blobStore blob.BlobStore, announcer blob.Announcer) *ProductionProducer {
	ctx, cancel := context.WithCancel(context.Background())
	
	pp := &ProductionProducer{
		producerID:     producerID,
		batchSize:      100,
		batchTimeout:   1 * time.Second,
		lastFlush:      time.Now(),
		pendingRecords: make([]DataRecord, 0, 100),
		ctx:           ctx,
		cancel:        cancel,
	}
	
	pp.stats.startTime = time.Now()
	
	// Create producer
	pp.producer = producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithNumStatesBetweenSnapshots(10),
	)
	
	return pp
}

func (pp *ProductionProducer) Start() error {
	log.Printf("🚀 Starting production producer: %s", pp.producerID)
	
	// Create initial snapshot
	if err := pp.createInitialSnapshot(); err != nil {
		return fmt.Errorf("failed to create initial snapshot: %w", err)
	}
	
	// Start flush ticker
	pp.flushTicker = time.NewTicker(pp.batchTimeout)
	defer pp.flushTicker.Stop()
	
	for {
		select {
		case <-pp.ctx.Done():
			return pp.gracefulShutdown()
		case <-pp.flushTicker.C:
			if err := pp.flushPendingRecords(); err != nil {
				pp.recordError()
				log.Printf("❌ Producer %s flush error: %v", pp.producerID, err)
			}
		}
	}
}

func (pp *ProductionProducer) createInitialSnapshot() error {
	// Note: In the real implementation, we would use internal.WriteState
	// This is a simplified version for the example
	log.Printf("📷 Producer %s would create initial snapshot here", pp.producerID)
	
	pp.stats.Lock()
	pp.stats.versionsProduced++
	pp.stats.recordsWritten++
	pp.stats.Unlock()
	
	return nil
}

func (pp *ProductionProducer) AddRecord(record DataRecord) error {
	pp.pendingMutex.Lock()
	defer pp.pendingMutex.Unlock()
	
	pp.pendingRecords = append(pp.pendingRecords, record)
	
	// Auto-flush if batch is full
	if len(pp.pendingRecords) >= pp.batchSize {
		go func() {
			if err := pp.flushPendingRecords(); err != nil {
				pp.recordError()
				log.Printf("❌ Producer %s auto-flush error: %v", pp.producerID, err)
			}
		}()
	}
	
	return nil
}

func (pp *ProductionProducer) flushPendingRecords() error {
	pp.pendingMutex.Lock()
	
	if len(pp.pendingRecords) == 0 {
		pp.pendingMutex.Unlock()
		return nil
	}
	
	recordsToFlush := make([]DataRecord, len(pp.pendingRecords))
	copy(recordsToFlush, pp.pendingRecords)
	pp.pendingRecords = pp.pendingRecords[:0]
	
	pp.pendingMutex.Unlock()
	
	// Simulate flushing records
	// In the real implementation, this would call producer.RunCycle
	log.Printf("✅ Producer %s flushed %d records", pp.producerID, len(recordsToFlush))
	
	pp.stats.Lock()
	pp.stats.versionsProduced++
	pp.stats.recordsWritten += uint64(len(recordsToFlush))
	pp.stats.Unlock()
	
	pp.lastFlush = time.Now()
	return nil
}

func (pp *ProductionProducer) recordError() {
	pp.stats.Lock()
	defer pp.stats.Unlock()
	pp.stats.errors++
}

func (pp *ProductionProducer) gracefulShutdown() error {
	log.Printf("📝 Producer %s starting graceful shutdown", pp.producerID)
	
	// Flush any remaining records
	if err := pp.flushPendingRecords(); err != nil {
		log.Printf("⚠️ Producer %s shutdown flush error: %v", pp.producerID, err)
	}
	
	pp.logFinalStats()
	log.Printf("✅ Producer %s shut down gracefully", pp.producerID)
	
	return nil
}

func (pp *ProductionProducer) Stop() {
	pp.cancel()
}

func (pp *ProductionProducer) GetStats() map[string]interface{} {
	pp.stats.RLock()
	defer pp.stats.RUnlock()
	
	uptime := time.Since(pp.stats.startTime)
	
	return map[string]interface{}{
		"producer_id":        pp.producerID,
		"versions_produced":  pp.stats.versionsProduced,
		"records_written":    pp.stats.recordsWritten,
		"errors":            pp.stats.errors,
		"uptime_seconds":    uptime.Seconds(),
		"pending_records":   len(pp.pendingRecords),
		"records_per_second": float64(pp.stats.recordsWritten) / uptime.Seconds(),
	}
}

func (pp *ProductionProducer) logFinalStats() {
	stats := pp.GetStats()
	log.Printf("📈 Final stats for %s: %+v", pp.producerID, stats)
}

// ProducerPool manages multiple producers
type ProducerPool struct {
	producers  []*ProductionProducer
	roundRobin uint64
}

func NewProducerPool(poolSize int, blobStore blob.BlobStore, announcer blob.Announcer) *ProducerPool {
	pool := &ProducerPool{
		producers: make([]*ProductionProducer, poolSize),
	}
	
	for i := 0; i < poolSize; i++ {
		pool.producers[i] = NewProductionProducer(
			fmt.Sprintf("Producer-%d", i+1),
			blobStore,
			announcer,
		)
	}
	
	return pool
}

func (pp *ProducerPool) Start() error {
	var wg sync.WaitGroup
	
	for _, producer := range pp.producers {
		wg.Add(1)
		go func(p *ProductionProducer) {
			defer wg.Done()
			if err := p.Start(); err != nil && err != context.Canceled {
				log.Printf("❌ Producer %s failed: %v", p.producerID, err)
			}
		}(producer)
	}
	
	wg.Wait()
	return nil
}

func (pp *ProducerPool) Stop() {
	for _, producer := range pp.producers {
		producer.Stop()
	}
}

func (pp *ProducerPool) AddRecord(record DataRecord) error {
	index := atomic.AddUint64(&pp.roundRobin, 1) % uint64(len(pp.producers))
	return pp.producers[index].AddRecord(record)
}

func generateTestData(source string, count int) []DataRecord {
	records := make([]DataRecord, count)
	now := time.Now().Unix()
	
	for i := 0; i < count; i++ {
		records[i] = DataRecord{
			ID:        fmt.Sprintf("%s-record-%d-%d", source, now, i),
			Value:     fmt.Sprintf("test-value-%d", i),
			Timestamp: now,
			Source:    source,
		}
	}
	
	return records
}

func main() {
	// Create shared infrastructure
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()
	
	// Create producer pool
	poolSize := 3
	producerPool := NewProducerPool(poolSize, blobStore, announcer)
	
	// Start producer pool in background
	go func() {
		if err := producerPool.Start(); err != nil {
			log.Printf("❌ Producer pool failed: %v", err)
		}
	}()
	
	// Simulate data production
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Generate and send test data
				records := generateTestData("DataGenerator", 25)
				for _, record := range records {
					if err := producerPool.AddRecord(record); err != nil {
						log.Printf("❌ Failed to add record: %v", err)
					}
				}
				log.Printf("📝 Generated and queued %d records", len(records))
			}
		}
	}()
	
	// Start statistics reporter
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				log.Printf("📊 Producer Pool Statistics:")
				for _, producer := range producerPool.producers {
					stats := producer.GetStats()
					log.Printf("  %s: versions=%d, records=%d, errors=%d, rps=%.2f", 
						producer.producerID,
						stats["versions_produced"],
						stats["records_written"],
						stats["errors"],
						stats["records_per_second"])
				}
			}
		}
	}()
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	log.Printf("🚀 Production producers started. Press Ctrl+C to stop.")
	log.Printf("This example demonstrates production-ready patterns:")
	log.Printf("  - Batching for efficient writes")
	log.Printf("  - Producer pooling for scaling")
	log.Printf("  - Error handling and recovery")
	log.Printf("  - Statistics and monitoring")
	log.Printf("  - Graceful shutdown")
	
	// Wait for shutdown signal
	<-sigChan
	log.Printf("📝 Shutdown signal received, stopping producers...")
	
	// Stop producer pool
	producerPool.Stop()
	
	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	
	log.Printf("✅ All producers stopped gracefully")
}
