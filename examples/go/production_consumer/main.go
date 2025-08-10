package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
)

// ProductionConsumer represents a production-ready consumer
type ProductionConsumer struct {
	consumerID       string
	zeroCopyConsumer *consumer.ZeroCopyConsumer
	regularConsumer  *consumer.Consumer
	announcer        blob.Announcer
	
	// State management
	lastVersion      uint64
	processedRecords uint64
	startTime        time.Time
	
	// Resource management
	ticker           *time.Ticker
	backoffDuration  time.Duration
	maxBackoffDuration time.Duration
	
	// Statistics
	stats struct {
		sync.RWMutex
		versionsProcessed uint64
		zeroCopyReads     uint64
		fallbackReads     uint64
		errors            uint64
		lastActivity      time.Time
	}
}

func NewProductionConsumer(consumerID string, blobStore blob.BlobStore, announcer blob.Announcer) *ProductionConsumer {
	pc := &ProductionConsumer{
		consumerID:         consumerID,
		announcer:          announcer,
		startTime:          time.Now(),
		backoffDuration:    100 * time.Millisecond,
		maxBackoffDuration: 5 * time.Second,
	}
	
	// Create consumers once during initialization
	pc.regularConsumer = consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
		consumer.WithSerializer(internal.NewCapnProtoSerializer()),
	)
	
	pc.zeroCopyConsumer = consumer.NewZeroCopyConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)
	
	return pc
}

func (pc *ProductionConsumer) Start(ctx context.Context) error {
	log.Printf("🚀 Starting production consumer: %s", pc.consumerID)
	
	// Use adaptive ticker that adjusts based on data availability
	pc.ticker = time.NewTicker(pc.backoffDuration)
	defer pc.ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("📝 Consumer %s shutting down gracefully", pc.consumerID)
			pc.printFinalStats()
			return ctx.Err()
		case <-pc.ticker.C:
			if err := pc.processNewData(ctx); err != nil {
				pc.recordError()
				log.Printf("❌ Consumer %s error: %v", pc.consumerID, err)
				
				// Implement exponential backoff on errors
				pc.increaseBackoff()
			} else {
				// Reset backoff on successful processing
				pc.resetBackoff()
			}
		}
	}
}

func (pc *ProductionConsumer) processNewData(ctx context.Context) error {
	// Check for new versions
	goroutineAnnouncer, ok := pc.announcer.(*blob.GoroutineAnnouncer)
	if !ok {
		return fmt.Errorf("unsupported announcer type")
	}
	
	latestVersion := goroutineAnnouncer.Latest()
	if latestVersion <= 0 || uint64(latestVersion) <= pc.lastVersion {
		// No new data - this is normal, not an error
		return nil
	}
	
	// Update activity timestamp
	pc.stats.Lock()
	pc.stats.lastActivity = time.Now()
	pc.stats.Unlock()
	
	// Try zero-copy first
	recordCount, err := pc.tryZeroCopyRead(ctx, latestVersion)
	if err != nil {
		// Fallback to regular consumer
		recordCount, err = pc.tryRegularRead(ctx, latestVersion)
		if err != nil {
			return fmt.Errorf("both zero-copy and regular read failed: %w", err)
		}
		pc.recordFallbackRead()
	} else {
		pc.recordZeroCopyRead()
	}
	
	// Update state
	pc.lastVersion = uint64(latestVersion)
	pc.processedRecords += uint64(recordCount)
	
	log.Printf("✅ Consumer %s processed version %d (%d records)", 
		pc.consumerID, latestVersion, recordCount)
	
	return nil
}

func (pc *ProductionConsumer) tryZeroCopyRead(ctx context.Context, version int) (int, error) {
	err := pc.zeroCopyConsumer.TriggerRefreshTo(ctx, version)
	if err != nil {
		return 0, err
	}
	
	se := pc.zeroCopyConsumer.GetStateEngine()
	if se == nil {
		return 0, fmt.Errorf("no state engine available")
	}
	
	return se.TotalRecords(), nil
}

func (pc *ProductionConsumer) tryRegularRead(ctx context.Context, version int) (int, error) {
	err := pc.regularConsumer.TriggerRefreshTo(ctx, version)
	if err != nil {
		return 0, err
	}
	
	se := pc.regularConsumer.GetStateEngine()
	if se == nil {
		return 0, fmt.Errorf("no state engine available")
	}
	
	return se.TotalRecords(), nil
}

func (pc *ProductionConsumer) increaseBackoff() {
	pc.backoffDuration = time.Duration(float64(pc.backoffDuration) * 1.5)
	if pc.backoffDuration > pc.maxBackoffDuration {
		pc.backoffDuration = pc.maxBackoffDuration
	}
	
	// Update ticker
	pc.ticker.Stop()
	pc.ticker = time.NewTicker(pc.backoffDuration)
}

func (pc *ProductionConsumer) resetBackoff() {
	pc.backoffDuration = 100 * time.Millisecond
	
	// Update ticker
	pc.ticker.Stop()
	pc.ticker = time.NewTicker(pc.backoffDuration)
}

func (pc *ProductionConsumer) recordZeroCopyRead() {
	pc.stats.Lock()
	defer pc.stats.Unlock()
	pc.stats.versionsProcessed++
	pc.stats.zeroCopyReads++
}

func (pc *ProductionConsumer) recordFallbackRead() {
	pc.stats.Lock()
	defer pc.stats.Unlock()
	pc.stats.versionsProcessed++
	pc.stats.fallbackReads++
}

func (pc *ProductionConsumer) recordError() {
	pc.stats.Lock()
	defer pc.stats.Unlock()
	pc.stats.errors++
}

func (pc *ProductionConsumer) GetStats() map[string]interface{} {
	pc.stats.RLock()
	defer pc.stats.RUnlock()
	
	uptime := time.Since(pc.startTime)
	
	return map[string]interface{}{
		"consumer_id":         pc.consumerID,
		"versions_processed":  pc.stats.versionsProcessed,
		"records_processed":   pc.processedRecords,
		"zero_copy_reads":     pc.stats.zeroCopyReads,
		"fallback_reads":      pc.stats.fallbackReads,
		"errors":              pc.stats.errors,
		"uptime_seconds":      uptime.Seconds(),
		"last_version":        pc.lastVersion,
		"last_activity":       pc.stats.lastActivity,
		"current_backoff_ms":  pc.backoffDuration.Milliseconds(),
	}
}

func (pc *ProductionConsumer) printFinalStats() {
	stats := pc.GetStats()
	log.Printf("📊 Final stats for %s: %+v", pc.consumerID, stats)
}

// HealthChecker provides health monitoring for production consumers
type HealthChecker struct {
	consumers []*ProductionConsumer
	threshold time.Duration
}

func NewHealthChecker(consumers []*ProductionConsumer) *HealthChecker {
	return &HealthChecker{
		consumers: consumers,
		threshold: 30 * time.Second, // Alert if no activity for 30 seconds
	}
}

func (hc *HealthChecker) CheckHealth() []string {
	var issues []string
	now := time.Now()
	
	for _, consumer := range hc.consumers {
		stats := consumer.GetStats()
		
		lastActivity, ok := stats["last_activity"].(time.Time)
		if !ok || lastActivity.IsZero() {
			issues = append(issues, fmt.Sprintf("Consumer %s: no activity recorded", consumer.consumerID))
			continue
		}
		
		if now.Sub(lastActivity) > hc.threshold {
			issues = append(issues, fmt.Sprintf("Consumer %s: inactive for %v", 
				consumer.consumerID, now.Sub(lastActivity)))
		}
		
		errors, _ := stats["errors"].(uint64)
		if errors > 10 { // Threshold for error count
			issues = append(issues, fmt.Sprintf("Consumer %s: high error count %d", 
				consumer.consumerID, errors))
		}
	}
	
	return issues
}

func main() {
	// Create shared infrastructure
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()
	
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create production consumers
	var consumers []*ProductionConsumer
	for i := 1; i <= 3; i++ {
		consumer := NewProductionConsumer(
			fmt.Sprintf("Consumer-%d", i),
			blobStore,
			announcer,
		)
		consumers = append(consumers, consumer)
	}
	
	// Start health checker
	healthChecker := NewHealthChecker(consumers)
	
	// Start consumers in separate goroutines
	var wg sync.WaitGroup
	for _, consumer := range consumers {
		wg.Add(1)
		go func(c *ProductionConsumer) {
			defer wg.Done()
			if err := c.Start(ctx); err != nil && err != context.Canceled {
				log.Printf("❌ Consumer %s failed: %v", c.consumerID, err)
			}
		}(consumer)
	}
	
	// Start health monitoring
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				issues := healthChecker.CheckHealth()
				if len(issues) > 0 {
					log.Printf("🚨 Health issues detected:")
					for _, issue := range issues {
						log.Printf("  - %s", issue)
					}
				} else {
					log.Printf("💚 All consumers healthy")
				}
				
				// Print consumer stats
				for _, consumer := range consumers {
					stats := consumer.GetStats()
					log.Printf("📊 %s: processed=%d, zero-copy=%d, fallback=%d, errors=%d", 
						consumer.consumerID,
						stats["versions_processed"],
						stats["zero_copy_reads"], 
						stats["fallback_reads"],
						stats["errors"])
				}
			}
		}
	}()
	
	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	log.Printf("🚀 Production consumers started. Press Ctrl+C to stop.")
	
	// Wait for shutdown signal
	<-sigChan
	log.Printf("📝 Shutdown signal received, stopping consumers...")
	
	// Cancel context to signal shutdown
	cancel()
	
	// Wait for all goroutines to finish
	wg.Wait()
	
	log.Printf("✅ All consumers stopped gracefully")
}
