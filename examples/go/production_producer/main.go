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
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// DataRecord represents the data structure we'll be producing
type DataRecord struct {
	ID        string `hollow:"key"`
	Value     string
	Timestamp int64
	Source    string
}

// ProductionProducer represents a production-ready producer with proper resource management
type ProductionProducer struct {
	producerID string
	producer   *producer.Producer
	announcer  blob.Announcer

	// Batching configuration
	batchSize      int
	batchTimeout   time.Duration
	pendingRecords []DataRecord
	pendingMutex   sync.Mutex
	lastFlush      time.Time

	// Performance tracking
	stats struct {
		sync.RWMutex
		versionsProduced uint64
		recordsWritten   uint64
		batchesProcessed uint64
		errors           uint64
		lastWrite        time.Time
		avgBatchSize     float64
		avgWriteLatency  time.Duration
	}

	// Resource management
	flushTicker   *time.Ticker
	metricsTicker *time.Ticker
	startTime     time.Time

	// Configuration
	config ProductionConfig
}

type ProductionConfig struct {
	BatchSize               int           `json:"batch_size"`
	BatchTimeout            time.Duration `json:"batch_timeout"`
	SnapshotFrequency       int           `json:"snapshot_frequency"`
	MaxMemoryUsage          int64         `json:"max_memory_usage_mb"`
	MetricsInterval         time.Duration `json:"metrics_interval"`
	HealthCheckInterval     time.Duration `json:"health_check_interval"`
	GracefulShutdownTimeout time.Duration `json:"graceful_shutdown_timeout"`
	ErrorRetryAttempts      int           `json:"error_retry_attempts"`
	ErrorRetryBackoff       time.Duration `json:"error_retry_backoff"`
}

// DefaultProductionConfig returns sensible defaults for production
func DefaultProductionConfig() ProductionConfig {
	return ProductionConfig{
		BatchSize:               100,
		BatchTimeout:            1 * time.Second,
		SnapshotFrequency:       10,
		MaxMemoryUsage:          512, // MB
		MetricsInterval:         30 * time.Second,
		HealthCheckInterval:     10 * time.Second,
		GracefulShutdownTimeout: 30 * time.Second,
		ErrorRetryAttempts:      3,
		ErrorRetryBackoff:       1 * time.Second,
	}
}

func NewProductionProducer(producerID string, blobStore blob.BlobStore, announcer blob.Announcer, config ProductionConfig) *ProductionProducer {
	pp := &ProductionProducer{
		producerID:     producerID,
		announcer:      announcer,
		config:         config,
		startTime:      time.Now(),
		lastFlush:      time.Now(),
		pendingRecords: make([]DataRecord, 0, config.BatchSize),
	}

	// Create producer with optimized settings
	pp.producer = producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
		producer.WithNumStatesBetweenSnapshots(config.SnapshotFrequency),
	)

	return pp
}

func (pp *ProductionProducer) Start(ctx context.Context) error {
	log.Printf("🚀 Starting production producer: %s", pp.producerID)

	// Initialize tickers
	pp.flushTicker = time.NewTicker(pp.config.BatchTimeout)
	pp.metricsTicker = time.NewTicker(pp.config.MetricsInterval)

	defer func() {
		pp.flushTicker.Stop()
		pp.metricsTicker.Stop()
	}()

	// Create initial snapshot
	if err := pp.createInitialSnapshot(ctx); err != nil {
		return fmt.Errorf("failed to create initial snapshot: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return pp.gracefulShutdown(ctx)
		case <-pp.flushTicker.C:
			if err := pp.flushPendingRecords(ctx); err != nil {
				pp.recordError()
				log.Printf("❌ Producer %s flush error: %v", pp.producerID, err)
			}
		case <-pp.metricsTicker.C:
			pp.logMetrics()
		}
	}
}

func (pp *ProductionProducer) createInitialSnapshot(ctx context.Context) error {
	start := time.Now()

	version, err := pp.producer.RunCycle(ctx, func(ws *internal.WriteState) {
		// Add initial marker record
		ws.Add(DataRecord{
			ID:        fmt.Sprintf("%s-initial", pp.producerID),
			Value:     "initial-snapshot",
			Timestamp: time.Now().Unix(),
			Source:    pp.producerID,
		})
	})
	if err != nil {
		return fmt.Errorf("failed to create initial snapshot: %w", err)
	}

	latency := time.Since(start)

	pp.stats.Lock()
	pp.stats.versionsProduced++
	pp.stats.recordsWritten++
	pp.stats.lastWrite = time.Now()
	pp.stats.avgWriteLatency = latency
	pp.stats.Unlock()

	log.Printf("📷 Producer %s created initial snapshot (version %d) in %v",
		pp.producerID, version, latency)

	return nil
}

// AddRecord adds a record to the pending batch (thread-safe)
func (pp *ProductionProducer) AddRecord(record DataRecord) error {
	pp.pendingMutex.Lock()
	defer pp.pendingMutex.Unlock()

	// Check memory pressure
	if err := pp.checkMemoryPressure(); err != nil {
		return err
	}

	pp.pendingRecords = append(pp.pendingRecords, record)

	// Auto-flush if batch is full
	if len(pp.pendingRecords) >= pp.config.BatchSize {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := pp.flushPendingRecords(ctx); err != nil {
				pp.recordError()
				log.Printf("❌ Producer %s auto-flush error: %v", pp.producerID, err)
			}
		}()
	}

	return nil
}

// AddRecords adds multiple records efficiently (thread-safe)
func (pp *ProductionProducer) AddRecords(records []DataRecord) error {
	pp.pendingMutex.Lock()
	defer pp.pendingMutex.Unlock()

	// Check memory pressure
	if err := pp.checkMemoryPressure(); err != nil {
		return err
	}

	pp.pendingRecords = append(pp.pendingRecords, records...)

	// Auto-flush if batch is full
	if len(pp.pendingRecords) >= pp.config.BatchSize {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := pp.flushPendingRecords(ctx); err != nil {
				pp.recordError()
				log.Printf("❌ Producer %s batch auto-flush error: %v", pp.producerID, err)
			}
		}()
	}

	return nil
}

func (pp *ProductionProducer) flushPendingRecords(ctx context.Context) error {
	pp.pendingMutex.Lock()

	if len(pp.pendingRecords) == 0 {
		pp.pendingMutex.Unlock()
		return nil // Nothing to flush
	}

	// Move pending records to local slice
	recordsToFlush := make([]DataRecord, len(pp.pendingRecords))
	copy(recordsToFlush, pp.pendingRecords)
	pp.pendingRecords = pp.pendingRecords[:0] // Reset slice but keep capacity

	pp.pendingMutex.Unlock()

	// Perform the flush operation
	return pp.flushRecordsBatch(ctx, recordsToFlush)
}

func (pp *ProductionProducer) flushRecordsBatch(ctx context.Context, records []DataRecord) error {
	if len(records) == 0 {
		return nil
	}

	start := time.Now()
	retryCount := 0

	for retryCount <= pp.config.ErrorRetryAttempts {
		version, err := pp.producer.RunCycle(ctx, func(ws *internal.WriteState) {
			for _, record := range records {
				ws.Add(record)
			}
		})
		if err != nil {
			retryCount++
			if retryCount > pp.config.ErrorRetryAttempts {
				return fmt.Errorf("failed to flush records batch after %d retries: %w", retryCount, err)
			}
			time.Sleep(pp.config.ErrorRetryBackoff)
			continue
		}

		if version > 0 {
			// Success
			latency := time.Since(start)

			pp.stats.Lock()
			pp.stats.versionsProduced++
			pp.stats.recordsWritten += uint64(len(records))
			pp.stats.batchesProcessed++
			pp.stats.lastWrite = time.Now()

			// Update rolling averages
			pp.stats.avgBatchSize = (pp.stats.avgBatchSize + float64(len(records))) / 2
			pp.stats.avgWriteLatency = (pp.stats.avgWriteLatency + latency) / 2

			pp.stats.Unlock()

			pp.lastFlush = time.Now()

			log.Printf("✅ Producer %s flushed %d records (version %d) in %v",
				pp.producerID, len(records), version, latency)

			return nil
		}

		// Retry logic
		retryCount++
		if retryCount <= pp.config.ErrorRetryAttempts {
			backoff := time.Duration(retryCount) * pp.config.ErrorRetryBackoff
			log.Printf("⏳ Producer %s retrying flush in %v (attempt %d/%d)",
				pp.producerID, backoff, retryCount, pp.config.ErrorRetryAttempts)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
	}

	return fmt.Errorf("failed to flush batch after %d attempts", pp.config.ErrorRetryAttempts)
}

func (pp *ProductionProducer) checkMemoryPressure() error {
	// Simple memory pressure check
	if len(pp.pendingRecords) > pp.config.BatchSize*10 {
		return fmt.Errorf("memory pressure: too many pending records (%d)", len(pp.pendingRecords))
	}
	return nil
}

func (pp *ProductionProducer) recordError() {
	pp.stats.Lock()
	defer pp.stats.Unlock()
	pp.stats.errors++
}

func (pp *ProductionProducer) gracefulShutdown(ctx context.Context) error {
	log.Printf("📝 Producer %s starting graceful shutdown", pp.producerID)

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), pp.config.GracefulShutdownTimeout)
	defer cancel()

	// Flush any remaining records
	if err := pp.flushPendingRecords(shutdownCtx); err != nil {
		log.Printf("⚠️ Producer %s shutdown flush error: %v", pp.producerID, err)
	}

	pp.logFinalMetrics()
	log.Printf("✅ Producer %s shut down gracefully", pp.producerID)

	return nil
}

func (pp *ProductionProducer) GetStats() map[string]interface{} {
	pp.stats.RLock()
	defer pp.stats.RUnlock()

	uptime := time.Since(pp.startTime)

	return map[string]interface{}{
		"producer_id":          pp.producerID,
		"versions_produced":    pp.stats.versionsProduced,
		"records_written":      pp.stats.recordsWritten,
		"batches_processed":    pp.stats.batchesProcessed,
		"errors":               pp.stats.errors,
		"uptime_seconds":       uptime.Seconds(),
		"last_write":           pp.stats.lastWrite,
		"avg_batch_size":       pp.stats.avgBatchSize,
		"avg_write_latency_ms": pp.stats.avgWriteLatency.Milliseconds(),
		"pending_records":      len(pp.pendingRecords),
		"records_per_second":   float64(pp.stats.recordsWritten) / uptime.Seconds(),
	}
}

func (pp *ProductionProducer) logMetrics() {
	stats := pp.GetStats()
	log.Printf("📊 Producer %s: versions=%d, records=%d, batches=%d, errors=%d, rps=%.2f, pending=%d",
		pp.producerID,
		stats["versions_produced"],
		stats["records_written"],
		stats["batches_processed"],
		stats["errors"],
		stats["records_per_second"],
		stats["pending_records"])
}

func (pp *ProductionProducer) logFinalMetrics() {
	stats := pp.GetStats()
	log.Printf("📈 Final metrics for %s: %+v", pp.producerID, stats)
}

// ProducerPool manages multiple producers for high-throughput scenarios
type ProducerPool struct {
	producers     []*ProductionProducer
	roundRobin    uint64
	healthChecker *ProducerHealthChecker
}

func NewProducerPool(poolSize int, blobStore blob.BlobStore, announcer blob.Announcer, config ProductionConfig) *ProducerPool {
	pool := &ProducerPool{
		producers: make([]*ProductionProducer, poolSize),
	}

	// Create producers
	for i := 0; i < poolSize; i++ {
		pool.producers[i] = NewProductionProducer(
			fmt.Sprintf("Producer-%d", i+1),
			blobStore,
			announcer,
			config,
		)
	}

	pool.healthChecker = NewProducerHealthChecker(pool.producers)
	return pool
}

func (pp *ProducerPool) Start(ctx context.Context) error {
	var wg sync.WaitGroup

	// Start all producers
	for _, producer := range pp.producers {
		wg.Add(1)
		go func(p *ProductionProducer) {
			defer wg.Done()
			if err := p.Start(ctx); err != nil && err != context.Canceled {
				log.Printf("❌ Producer %s failed: %v", p.producerID, err)
			}
		}(producer)
	}

	// Start health checker
	wg.Add(1)
	go func() {
		defer wg.Done()
		pp.healthChecker.Start(ctx)
	}()

	wg.Wait()
	return nil
}

// AddRecord adds a record using round-robin load balancing
func (pp *ProducerPool) AddRecord(record DataRecord) error {
	index := atomic.AddUint64(&pp.roundRobin, 1) % uint64(len(pp.producers))
	return pp.producers[index].AddRecord(record)
}

// AddRecords distributes records across producers
func (pp *ProducerPool) AddRecords(records []DataRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Distribute records across producers
	chunkSize := len(records) / len(pp.producers)
	if chunkSize == 0 {
		chunkSize = 1
	}

	var wg sync.WaitGroup
	var errors []error
	var errorMutex sync.Mutex

	for i, producer := range pp.producers {
		start := i * chunkSize
		end := start + chunkSize
		if i == len(pp.producers)-1 {
			end = len(records) // Last producer gets remaining records
		}

		if start >= len(records) {
			break
		}

		chunk := records[start:end]

		wg.Add(1)
		go func(p *ProductionProducer, chunk []DataRecord) {
			defer wg.Done()
			if err := p.AddRecords(chunk); err != nil {
				errorMutex.Lock()
				errors = append(errors, err)
				errorMutex.Unlock()
			}
		}(producer, chunk)
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("failed to add records to %d producers: %v", len(errors), errors[0])
	}

	return nil
}

// ProducerHealthChecker monitors producer health
type ProducerHealthChecker struct {
	producers []*ProductionProducer
	threshold time.Duration
}

func NewProducerHealthChecker(producers []*ProductionProducer) *ProducerHealthChecker {
	return &ProducerHealthChecker{
		producers: producers,
		threshold: 60 * time.Second, // Alert if no writes for 60 seconds
	}
}

func (phc *ProducerHealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			issues := phc.CheckHealth()
			if len(issues) > 0 {
				log.Printf("🚨 Producer health issues detected:")
				for _, issue := range issues {
					log.Printf("  - %s", issue)
				}
			} else {
				log.Printf("💚 All producers healthy")
			}
		}
	}
}

func (phc *ProducerHealthChecker) CheckHealth() []string {
	var issues []string
	now := time.Now()

	for _, producer := range phc.producers {
		stats := producer.GetStats()

		lastWrite, ok := stats["last_write"].(time.Time)
		if !ok || lastWrite.IsZero() {
			issues = append(issues, fmt.Sprintf("Producer %s: no writes recorded", producer.producerID))
			continue
		}

		if now.Sub(lastWrite) > phc.threshold {
			issues = append(issues, fmt.Sprintf("Producer %s: inactive for %v",
				producer.producerID, now.Sub(lastWrite)))
		}

		errors, _ := stats["errors"].(uint64)
		if errors > 5 { // Threshold for error count
			issues = append(issues, fmt.Sprintf("Producer %s: high error count %d",
				producer.producerID, errors))
		}

		pending, _ := stats["pending_records"].(int)
		if pending > 1000 { // Threshold for pending records
			issues = append(issues, fmt.Sprintf("Producer %s: high pending records %d",
				producer.producerID, pending))
		}
	}

	return issues
}

// Example data generator for testing
func generateTestData(producerID string, batchSize int) []DataRecord {
	records := make([]DataRecord, batchSize)
	now := time.Now().Unix()

	for i := 0; i < batchSize; i++ {
		records[i] = DataRecord{
			ID:        fmt.Sprintf("%s-record-%d-%d", producerID, now, i),
			Value:     fmt.Sprintf("test-value-%d", i),
			Timestamp: now,
			Source:    producerID,
		}
	}

	return records
}

func main() {
	// Create shared infrastructure
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use production configuration
	config := DefaultProductionConfig()
	config.BatchSize = 50
	config.BatchTimeout = 500 * time.Millisecond

	// Create producer pool
	poolSize := 3
	producerPool := NewProducerPool(poolSize, blobStore, announcer, config)

	// Start producer pool
	go func() {
		if err := producerPool.Start(ctx); err != nil {
			log.Printf("❌ Producer pool failed: %v", err)
		}
	}()

	// Simulate data production
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Generate and send test data
				records := generateTestData("DataGenerator", 25)
				if err := producerPool.AddRecords(records); err != nil {
					log.Printf("❌ Failed to add records: %v", err)
				} else {
					log.Printf("📝 Generated and queued %d records", len(records))
				}
			}
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("🚀 Production producers started. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-sigChan
	log.Printf("📝 Shutdown signal received, stopping producers...")

	// Cancel context to signal shutdown
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)

	log.Printf("✅ All producers stopped gracefully")
}
