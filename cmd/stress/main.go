package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leow/go-raw-hollow"
	"github.com/leow/go-raw-hollow/internal/memblob"
)

const (
	NumProducers     = 3
	NumConsumers     = 5
	EventsPerProducer = 3334 // ~10k total events
	TestDuration     = 10 * time.Minute
)

type StressMetrics struct {
	mu                sync.RWMutex
	producerCycles    int64
	consumerRefreshes int64
	totalErrors       int64
	producerErrors    int64
	consumerErrors    int64
	startTime         time.Time
	
	// Performance metrics
	cycleLatencies    []time.Duration
	refreshLatencies  []time.Duration
	
	// Throughput metrics
	eventsPerSecond   []int64
	bytesPerSecond    []int64
}

func NewStressMetrics() *StressMetrics {
	return &StressMetrics{
		startTime:         time.Now(),
		cycleLatencies:    make([]time.Duration, 0, 10000),
		refreshLatencies:  make([]time.Duration, 0, 10000),
		eventsPerSecond:   make([]int64, 0, 600), // 10 minutes worth of seconds
		bytesPerSecond:    make([]int64, 0, 600),
	}
}

func (sm *StressMetrics) RecordProducerCycle(duration time.Duration, eventCount int64, byteCount int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	atomic.AddInt64(&sm.producerCycles, 1)
	sm.cycleLatencies = append(sm.cycleLatencies, duration)
	
	// Track throughput
	second := int64(time.Since(sm.startTime).Seconds())
	sm.ensureSliceSize(&sm.eventsPerSecond, second)
	sm.ensureSliceSize(&sm.bytesPerSecond, second)
	
	if second >= 0 && second < int64(len(sm.eventsPerSecond)) {
		sm.eventsPerSecond[second] += eventCount
		sm.bytesPerSecond[second] += byteCount
	}
}

func (sm *StressMetrics) RecordConsumerRefresh(duration time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	atomic.AddInt64(&sm.consumerRefreshes, 1)
	sm.refreshLatencies = append(sm.refreshLatencies, duration)
}

func (sm *StressMetrics) RecordProducerError() {
	atomic.AddInt64(&sm.producerErrors, 1)
	atomic.AddInt64(&sm.totalErrors, 1)
}

func (sm *StressMetrics) RecordConsumerError() {
	atomic.AddInt64(&sm.consumerErrors, 1)
	atomic.AddInt64(&sm.totalErrors, 1)
}

func (sm *StressMetrics) ensureSliceSize(slice *[]int64, minSize int64) {
	for int64(len(*slice)) <= minSize {
		*slice = append(*slice, 0)
	}
}

func (sm *StressMetrics) PrintReport() {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	totalDuration := time.Since(sm.startTime)
	
	fmt.Print("\n" + strings.Repeat("=", 70) + "\n")
	fmt.Printf("STRESS TEST REPORT\n")
	fmt.Print(strings.Repeat("=", 70) + "\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Producers:        %d\n", NumProducers)
	fmt.Printf("  Consumers:        %d\n", NumConsumers)
	fmt.Printf("  Test Duration:    %v\n", totalDuration)
	fmt.Printf("  Target Events:    %d\n", NumProducers*EventsPerProducer)
	fmt.Printf("\n")
	
	fmt.Printf("Operations:\n")
	fmt.Printf("  Producer Cycles:  %d\n", atomic.LoadInt64(&sm.producerCycles))
	fmt.Printf("  Consumer Refresh: %d\n", atomic.LoadInt64(&sm.consumerRefreshes))
	fmt.Printf("  Total Errors:     %d\n", atomic.LoadInt64(&sm.totalErrors))
	fmt.Printf("  Producer Errors:  %d\n", atomic.LoadInt64(&sm.producerErrors))
	fmt.Printf("  Consumer Errors:  %d\n", atomic.LoadInt64(&sm.consumerErrors))
	fmt.Printf("\n")
	
	// Calculate latency statistics
	if len(sm.cycleLatencies) > 0 {
		var totalCycleTime time.Duration
		var minCycle, maxCycle time.Duration = time.Hour, 0
		
		for _, lat := range sm.cycleLatencies {
			totalCycleTime += lat
			if lat < minCycle {
				minCycle = lat
			}
			if lat > maxCycle {
				maxCycle = lat
			}
		}
		
		avgCycle := totalCycleTime / time.Duration(len(sm.cycleLatencies))
		fmt.Printf("Producer Cycle Latency:\n")
		fmt.Printf("  Average:          %v\n", avgCycle)
		fmt.Printf("  Minimum:          %v\n", minCycle)
		fmt.Printf("  Maximum:          %v\n", maxCycle)
		fmt.Printf("  Total Cycles:     %d\n", len(sm.cycleLatencies))
		fmt.Printf("\n")
	}
	
	if len(sm.refreshLatencies) > 0 {
		var totalRefreshTime time.Duration
		var minRefresh, maxRefresh time.Duration = time.Hour, 0
		
		for _, lat := range sm.refreshLatencies {
			totalRefreshTime += lat
			if lat < minRefresh {
				minRefresh = lat
			}
			if lat > maxRefresh {
				maxRefresh = lat
			}
		}
		
		avgRefresh := totalRefreshTime / time.Duration(len(sm.refreshLatencies))
		fmt.Printf("Consumer Refresh Latency:\n")
		fmt.Printf("  Average:          %v\n", avgRefresh)
		fmt.Printf("  Minimum:          %v\n", minRefresh)
		fmt.Printf("  Maximum:          %v\n", maxRefresh)
		fmt.Printf("  Total Refreshes:  %d\n", len(sm.refreshLatencies))
		fmt.Printf("\n")
	}
	
	// Calculate throughput
	var totalEvents, totalBytes int64
	for _, events := range sm.eventsPerSecond {
		totalEvents += events
	}
	for _, bytes := range sm.bytesPerSecond {
		totalBytes += bytes
	}
	
	fmt.Printf("Throughput:\n")
	fmt.Printf("  Total Events:     %d\n", totalEvents)
	fmt.Printf("  Total Bytes:      %d (%.2f MB)\n", totalBytes, float64(totalBytes)/(1024*1024))
	fmt.Printf("  Events/Second:    %.2f\n", float64(totalEvents)/totalDuration.Seconds())
	fmt.Printf("  Bytes/Second:     %.2f (%.2f KB/s)\n", 
		float64(totalBytes)/totalDuration.Seconds(),
		float64(totalBytes)/(1024*totalDuration.Seconds()))
	fmt.Printf("  Cycles/Second:    %.2f\n", float64(atomic.LoadInt64(&sm.producerCycles))/totalDuration.Seconds())
	fmt.Printf("  Refreshes/Second: %.2f\n", float64(atomic.LoadInt64(&sm.consumerRefreshes))/totalDuration.Seconds())
	
	fmt.Print(strings.Repeat("=", 70) + "\n")
}

// StressProducer manages a single producer's stress testing
type StressProducer struct {
	id       int
	producer *hollow.Producer
	metrics  *StressMetrics
	logger   *slog.Logger
}

func NewStressProducer(id int, blob *memblob.Store, metrics *StressMetrics, logger *slog.Logger) *StressProducer {
	producer := hollow.NewProducer(
		hollow.WithBlobStager(blob),
		hollow.WithLogger(logger),
		hollow.WithMetricsCollector(func(m hollow.Metrics) {
			// Metrics are recorded in the RunStress method
		}),
	)
	
	return &StressProducer{
		id:       id,
		producer: producer,
		metrics:  metrics,
		logger:   logger,
	}
}

func (sp *StressProducer) RunStress(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	
	for i := 0; i < EventsPerProducer; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		// Generate batch of events
		batchSize := rand.Intn(10) + 1 // 1-10 events per cycle
		
		start := time.Now()
		version, err := sp.producer.RunCycle(func(ws hollow.WriteState) error {
			for j := 0; j < batchSize; j++ {
				eventID := fmt.Sprintf("producer_%d_event_%d_%d", sp.id, i, j)
				
				if err := ws.Add(eventID); err != nil {
					return fmt.Errorf("failed to add event %s: %w", eventID, err)
				}
			}
			return nil
		})
		
		duration := time.Since(start)
		
		if err != nil {
			sp.metrics.RecordProducerError()
			sp.logger.Error("Producer cycle failed", "producer", sp.id, "error", err)
			continue
		}
		
		// Estimate byte size (rough approximation)
		estimatedBytes := int64(batchSize * 200) // ~200 bytes per event
		sp.metrics.RecordProducerCycle(duration, int64(batchSize), estimatedBytes)
		
		if i%100 == 0 {
			sp.logger.Info("Producer progress", "producer", sp.id, "events", i, "version", version)
		}
		
		// Random delay between cycles (0-100ms)
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	}
	
	sp.logger.Info("Producer completed", "producer", sp.id, "total_events", EventsPerProducer)
}

// StressConsumer manages a single consumer's stress testing
type StressConsumer struct {
	id       int
	consumer *hollow.Consumer
	metrics  *StressMetrics
	logger   *slog.Logger
}

func NewStressConsumer(id int, blob *memblob.Store, metrics *StressMetrics, logger *slog.Logger) *StressConsumer {
	consumer := hollow.NewConsumer(
		hollow.WithBlobRetriever(blob),
		hollow.WithAnnouncementWatcher(func() (uint64, bool, error) {
			latest, err := blob.Latest(context.Background())
			return latest, latest > 0, err
		}),
		hollow.WithConsumerLogger(logger),
	)
	
	return &StressConsumer{
		id:       id,
		consumer: consumer,
		metrics:  metrics,
		logger:   logger,
	}
}

func (sc *StressConsumer) RunStress(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	
	ticker := time.NewTicker(time.Duration(rand.Intn(500)+100) * time.Millisecond) // 100-600ms
	defer ticker.Stop()
	
	refreshCount := 0
	
	for {
		select {
		case <-ctx.Done():
			sc.logger.Info("Consumer stopped", "consumer", sc.id, "refreshes", refreshCount)
			return
		case <-ticker.C:
			start := time.Now()
			err := sc.consumer.Refresh()
			duration := time.Since(start)
			
			if err != nil {
				sc.metrics.RecordConsumerError()
				sc.logger.Error("Consumer refresh failed", "consumer", sc.id, "error", err)
				continue
			}
			
			sc.metrics.RecordConsumerRefresh(duration)
			refreshCount++
			
			// Periodically perform reads to test read performance
			if refreshCount%10 == 0 {
				sc.performRandomReads()
			}
			
			if refreshCount%50 == 0 {
				rs := sc.consumer.ReadState()
				sc.logger.Info("Consumer progress", "consumer", sc.id, "refreshes", refreshCount, 
					"version", sc.consumer.CurrentVersion(), "size", rs.Size())
			}
		}
	}
}

func (sc *StressConsumer) performRandomReads() {
	rs := sc.consumer.ReadState()
	
	// Perform random reads
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("producer_%d_event_%d_%d", 
			rand.Intn(NumProducers), 
			rand.Intn(EventsPerProducer),
			rand.Intn(10))
		
		start := time.Now()
		_, exists := rs.Get(key)
		readDuration := time.Since(start)
		
		if readDuration > 5*time.Microsecond {
			sc.logger.Warn("Slow read detected", "consumer", sc.id, "key", key, 
				"duration", readDuration, "exists", exists)
		}
	}
}

func main() {
	fmt.Printf("Hollow-Go Stress Test\n")
	fmt.Printf("Producers: %d, Consumers: %d\n", NumProducers, NumConsumers)
	fmt.Printf("Target Events: %d over %v\n", NumProducers*EventsPerProducer, TestDuration)
	
	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce log noise during stress test
	}))
	
	// Create shared blob store
	blob := memblob.New()
	
	// Create metrics collector
	metrics := NewStressMetrics()
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), TestDuration)
	defer cancel()
	
	var wg sync.WaitGroup
	
	// Start producers
	producers := make([]*StressProducer, NumProducers)
	for i := 0; i < NumProducers; i++ {
		producers[i] = NewStressProducer(i, blob, metrics, logger)
		wg.Add(1)
		go producers[i].RunStress(ctx, &wg)
	}
	
	// Start consumers
	consumers := make([]*StressConsumer, NumConsumers)
	for i := 0; i < NumConsumers; i++ {
		consumers[i] = NewStressConsumer(i, blob, metrics, logger)
		wg.Add(1)
		go consumers[i].RunStress(ctx, &wg)
	}
	
	// Start progress monitor
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cycles := atomic.LoadInt64(&metrics.producerCycles)
				refreshes := atomic.LoadInt64(&metrics.consumerRefreshes)
				errors := atomic.LoadInt64(&metrics.totalErrors)
				elapsed := time.Since(metrics.startTime)
				
				fmt.Printf("Progress: %v elapsed - Cycles: %d, Refreshes: %d, Errors: %d\n",
					elapsed, cycles, refreshes, errors)
			}
		}
	}()
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	fmt.Printf("\nStress test completed!\n")
	
	// Print final metrics
	metrics.PrintReport()
	
	// Test final state consistency
	fmt.Printf("\nTesting final state consistency...\n")
	testStateConsistency(consumers, logger)
	
	fmt.Printf("\nStress test finished successfully!\n")
}

func testStateConsistency(consumers []*StressConsumer, logger *slog.Logger) {
	// All consumers should have the same final state
	if len(consumers) == 0 {
		return
	}
	
	// Force final refresh for all consumers
	for _, consumer := range consumers {
		consumer.consumer.Refresh()
	}
	
	// Check that all consumers have the same version and size
	firstVersion := consumers[0].consumer.CurrentVersion()
	firstSize := consumers[0].consumer.ReadState().Size()
	
	allConsistent := true
	for i, consumer := range consumers {
		version := consumer.consumer.CurrentVersion()
		size := consumer.consumer.ReadState().Size()
		
		if version != firstVersion || size != firstSize {
			logger.Error("State inconsistency detected",
				"consumer", i,
				"version", version,
				"expected_version", firstVersion,
				"size", size,
				"expected_size", firstSize)
			allConsistent = false
		}
	}
	
	if allConsistent {
		fmt.Printf("✓ All consumers have consistent state (version: %d, size: %d)\n", 
			firstVersion, firstSize)
	} else {
		fmt.Printf("✗ State inconsistencies detected!\n")
	}
}
