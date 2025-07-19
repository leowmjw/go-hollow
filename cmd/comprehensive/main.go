package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leow/go-raw-hollow"
	"github.com/leow/go-raw-hollow/internal/memblob"
)

// ComprehensiveDemo demonstrates all Hollow-Go capabilities
type ComprehensiveDemo struct {
	config      DemoConfig
	blob        *memblob.Store
	producers   []*DemoProducer
	consumers   []*DemoConsumer
	metrics     *ComprehensiveMetrics
	logger      *slog.Logger
	
	// Event scheduling
	eventScheduler *EventScheduler
	eventQueue     chan ScheduledEvent
	
	// State management
	mu           sync.RWMutex
	currentPhase string
	isRunning    bool
}

type DemoConfig struct {
	TotalEvents      int
	Duration         time.Duration
	NumProducers     int
	NumConsumers     int
	EventTypes       []string
	UseNormalDist    bool
	EnableMetrics    bool
	EnableDiff       bool
	EnablePerf       bool
	TestScenarios    []string
}

type DemoProducer struct {
	id       int
	producer *hollow.Producer
	metrics  *ComprehensiveMetrics
	logger   *slog.Logger
}

type DemoConsumer struct {
	id       int
	consumer *hollow.Consumer
	metrics  *ComprehensiveMetrics
	logger   *slog.Logger
}

type ScheduledEvent struct {
	ID        int
	Type      string
	Data      map[string]interface{}
	Timestamp time.Time
}

type EventScheduler struct {
	events    []ScheduledEvent
	startTime time.Time
	config    DemoConfig
}

type ComprehensiveMetrics struct {
	mu sync.RWMutex
	
	// Basic metrics
	totalEvents      int64
	totalCycles      int64
	totalRefreshes   int64
	totalReads       int64
	
	// Performance metrics
	cycleLatencies   []time.Duration
	refreshLatencies []time.Duration
	readLatencies    []time.Duration
	
	// Business metrics
	eventTypeCounts  map[string]int64
	hourlyStats      map[int]map[string]int64
	
	// System metrics
	memoryUsage      []float64
	cpuUsage         []float64
	goroutineCount   []int
	
	// Error tracking
	errorsByType     map[string]int64
	errorsByPhase    map[string]int64
	
	// Diff tracking
	diffHistory      []DiffResult
	
	startTime        time.Time
}

type DiffResult struct {
	Timestamp time.Time
	Added     int
	Removed   int
	Changed   int
	Impact    float64
}

func NewComprehensiveDemo(config DemoConfig) *ComprehensiveDemo {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	
	return &ComprehensiveDemo{
		config:         config,
		blob:           memblob.New(),
		metrics:        NewComprehensiveMetrics(),
		logger:         logger,
		eventScheduler: NewEventScheduler(config),
		eventQueue:     make(chan ScheduledEvent, 1000),
		currentPhase:   "initialization",
	}
}

func NewEventScheduler(config DemoConfig) *EventScheduler {
	return &EventScheduler{
		events:    make([]ScheduledEvent, 0, config.TotalEvents),
		startTime: time.Now(),
		config:    config,
	}
}

func NewComprehensiveMetrics() *ComprehensiveMetrics {
	return &ComprehensiveMetrics{
		cycleLatencies:   make([]time.Duration, 0),
		refreshLatencies: make([]time.Duration, 0),
		readLatencies:    make([]time.Duration, 0),
		eventTypeCounts:  make(map[string]int64),
		hourlyStats:      make(map[int]map[string]int64),
		memoryUsage:      make([]float64, 0),
		cpuUsage:         make([]float64, 0),
		goroutineCount:   make([]int, 0),
		errorsByType:     make(map[string]int64),
		errorsByPhase:    make(map[string]int64),
		diffHistory:      make([]DiffResult, 0),
		startTime:        time.Now(),
	}
}

func (cd *ComprehensiveDemo) Initialize() error {
	cd.mu.Lock()
	cd.currentPhase = "initialization"
	cd.mu.Unlock()
	
	fmt.Printf("🚀 Initializing Comprehensive Hollow-Go Demo\n")
	fmt.Print(strings.Repeat("=", 60) + "\n")
	
	// Create producers
	cd.producers = make([]*DemoProducer, cd.config.NumProducers)
	for i := 0; i < cd.config.NumProducers; i++ {
		producer := hollow.NewProducer(
			hollow.WithBlobStager(cd.blob),
			hollow.WithLogger(cd.logger),
			hollow.WithMetricsCollector(cd.createMetricsCollector(i)),
		)
		
		cd.producers[i] = &DemoProducer{
			id:       i,
			producer: producer,
			metrics:  cd.metrics,
			logger:   cd.logger,
		}
	}
	
	// Create consumers
	cd.consumers = make([]*DemoConsumer, cd.config.NumConsumers)
	for i := 0; i < cd.config.NumConsumers; i++ {
		consumer := hollow.NewConsumer(
			hollow.WithBlobRetriever(cd.blob),
			hollow.WithAnnouncementWatcher(func() (uint64, bool, error) {
				latest, err := cd.blob.Latest(context.Background())
				return latest, latest > 0, err
			}),
			hollow.WithConsumerLogger(cd.logger),
		)
		
		cd.consumers[i] = &DemoConsumer{
			id:       i,
			consumer: consumer,
			metrics:  cd.metrics,
			logger:   cd.logger,
		}
	}
	
	// Generate event schedule
	cd.eventScheduler.GenerateSchedule()
	
	fmt.Printf("✓ Configuration:\n")
	fmt.Printf("  Total Events:     %d\n", cd.config.TotalEvents)
	fmt.Printf("  Duration:         %v\n", cd.config.Duration)
	fmt.Printf("  Producers:        %d\n", cd.config.NumProducers)
	fmt.Printf("  Consumers:        %d\n", cd.config.NumConsumers)
	fmt.Printf("  Event Types:      %v\n", cd.config.EventTypes)
	fmt.Printf("  Normal Distribution: %v\n", cd.config.UseNormalDist)
	fmt.Printf("  Test Scenarios:   %v\n", cd.config.TestScenarios)
	fmt.Printf("\n")
	
	return nil
}

func (cd *ComprehensiveDemo) createMetricsCollector(producerID int) hollow.MetricsCollector {
	return func(m hollow.Metrics) {
		cd.metrics.mu.Lock()
		defer cd.metrics.mu.Unlock()
		
		atomic.AddInt64(&cd.metrics.totalEvents, int64(m.RecordCount))
		atomic.AddInt64(&cd.metrics.totalCycles, 1)
		
		// Track by hour
		hour := time.Now().Hour()
		if cd.metrics.hourlyStats[hour] == nil {
			cd.metrics.hourlyStats[hour] = make(map[string]int64)
		}
		cd.metrics.hourlyStats[hour]["events"] += int64(m.RecordCount)
		cd.metrics.hourlyStats[hour]["bytes"] += m.ByteSize
		
		cd.logger.Debug("Producer metrics", 
			"producer", producerID,
			"version", m.Version,
			"records", m.RecordCount,
			"bytes", m.ByteSize)
	}
}

func (es *EventScheduler) GenerateSchedule() {
	fmt.Printf("📅 Generating event schedule...\n")
	
	eventTypes := es.config.EventTypes
	if len(eventTypes) == 0 {
		eventTypes = []string{"user_action", "system_event", "business_event", "analytics_event"}
	}
	
	for i := 0; i < es.config.TotalEvents; i++ {
		var scheduledTime time.Time
		
		if es.config.UseNormalDist {
			// Generate normal distribution over duration
			progress := generateNormalDistribution(0.5, 0.2) // Mean=0.5, StdDev=0.2
			if progress < 0 {
				progress = 0
			}
			if progress > 1 {
				progress = 1
			}
			offset := time.Duration(float64(es.config.Duration) * progress)
			scheduledTime = es.startTime.Add(offset)
		} else {
			// Uniform distribution
			offset := time.Duration(rand.Int63n(int64(es.config.Duration)))
			scheduledTime = es.startTime.Add(offset)
		}
		
		event := ScheduledEvent{
			ID:        i + 1,
			Type:      eventTypes[rand.Intn(len(eventTypes))],
			Timestamp: scheduledTime,
			Data: map[string]interface{}{
				"id":          i + 1,
				"source":      "demo",
				"priority":    rand.Intn(5) + 1,
				"correlation": fmt.Sprintf("corr_%d", rand.Intn(100)),
				"value":       rand.Float64() * 1000,
			},
		}
		
		es.events = append(es.events, event)
	}
	
	fmt.Printf("✓ Generated %d events with %s distribution\n", 
		len(es.events), 
		map[bool]string{true: "normal", false: "uniform"}[es.config.UseNormalDist])
}

func generateNormalDistribution(mean, stdDev float64) float64 {
	// Box-Muller transform
	u1 := rand.Float64()
	u2 := rand.Float64()
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	return mean + stdDev*z
}

func (cd *ComprehensiveDemo) RunDemo() error {
	cd.mu.Lock()
	cd.currentPhase = "running"
	cd.isRunning = true
	cd.mu.Unlock()
	
	fmt.Printf("🎯 Starting Comprehensive Demo\n")
	fmt.Print(strings.Repeat("=", 60) + "\n")
	
	ctx, cancel := context.WithTimeout(context.Background(), cd.config.Duration+time.Minute)
	defer cancel()
	
	var wg sync.WaitGroup
	
	// Start system monitoring
	wg.Add(1)
	go cd.monitorSystem(ctx, &wg)
	
	// Start event scheduler
	wg.Add(1)
	go cd.scheduleEvents(ctx, &wg)
	
	// Start event processors
	for i, producer := range cd.producers {
		wg.Add(1)
		go cd.runProducer(ctx, &wg, producer, i)
	}
	
	// Start consumers
	for i, consumer := range cd.consumers {
		wg.Add(1)
		go cd.runConsumer(ctx, &wg, consumer, i)
	}
	
	// Start test scenarios
	for _, scenario := range cd.config.TestScenarios {
		wg.Add(1)
		go cd.runScenario(ctx, &wg, scenario)
	}
	
	// Start progress reporter
	wg.Add(1)
	go cd.reportProgress(ctx, &wg)
	
	// Wait for completion
	wg.Wait()
	
	cd.mu.Lock()
	cd.currentPhase = "completed"
	cd.isRunning = false
	cd.mu.Unlock()
	
	fmt.Printf("\n✅ Demo completed successfully!\n")
	
	return nil
}

func (cd *ComprehensiveDemo) scheduleEvents(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(cd.eventQueue)
	
	cd.eventScheduler.startTime = time.Now()
	
	for _, event := range cd.eventScheduler.events {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		// Wait until it's time for this event
		now := time.Now()
		eventTime := cd.eventScheduler.startTime.Add(event.Timestamp.Sub(cd.eventScheduler.startTime))
		
		if sleepDuration := eventTime.Sub(now); sleepDuration > 0 {
			time.Sleep(sleepDuration)
		}
		
		cd.eventQueue <- event
	}
}

func (cd *ComprehensiveDemo) runProducer(ctx context.Context, wg *sync.WaitGroup, producer *DemoProducer, producerID int) {
	defer wg.Done()
	
	batchSize := 0
	batch := make([]ScheduledEvent, 0, 50)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				cd.processBatch(producer, batch)
			}
			return
			
		case event, ok := <-cd.eventQueue:
			if !ok {
				if len(batch) > 0 {
					cd.processBatch(producer, batch)
				}
				return
			}
			
			batch = append(batch, event)
			batchSize++
			
			// Process batch when it reaches optimal size
			if batchSize >= 10 {
				cd.processBatch(producer, batch)
				batch = batch[:0]
				batchSize = 0
			}
			
		case <-ticker.C:
			if len(batch) > 0 {
				cd.processBatch(producer, batch)
				batch = batch[:0]
				batchSize = 0
			}
		}
	}
}

func (cd *ComprehensiveDemo) processBatch(producer *DemoProducer, batch []ScheduledEvent) {
	start := time.Now()
	
	_, err := producer.producer.RunCycle(func(ws hollow.WriteState) error {
		for _, event := range batch {
			key := fmt.Sprintf("event_%d_%s", event.ID, event.Type)
			if err := ws.Add(key); err != nil {
				return err
			}
			
			// Track event type
			cd.metrics.mu.Lock()
			cd.metrics.eventTypeCounts[event.Type]++
			cd.metrics.mu.Unlock()
		}
		return nil
	})
	
	latency := time.Since(start)
	
	if err != nil {
		cd.metrics.mu.Lock()
		cd.metrics.errorsByType["producer"]++
		cd.metrics.errorsByPhase[cd.currentPhase]++
		cd.metrics.mu.Unlock()
		
		cd.logger.Error("Producer batch failed", 
			"producer", producer.id,
			"batch_size", len(batch),
			"error", err)
		return
	}
	
	cd.metrics.mu.Lock()
	cd.metrics.cycleLatencies = append(cd.metrics.cycleLatencies, latency)
	cd.metrics.mu.Unlock()
	
	cd.logger.Debug("Producer batch processed",
		"producer", producer.id,
		"batch_size", len(batch),
		"latency", latency)
}

func (cd *ComprehensiveDemo) runConsumer(ctx context.Context, wg *sync.WaitGroup, consumer *DemoConsumer, consumerID int) {
	defer wg.Done()
	
	// Stagger consumer start times
	time.Sleep(time.Duration(consumerID*100) * time.Millisecond)
	
	ticker := time.NewTicker(time.Duration(200+rand.Intn(300)) * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			
			err := consumer.consumer.Refresh()
			latency := time.Since(start)
			
			if err != nil {
				cd.metrics.mu.Lock()
				cd.metrics.errorsByType["consumer"]++
				cd.metrics.errorsByPhase[cd.currentPhase]++
				cd.metrics.mu.Unlock()
				
				cd.logger.Error("Consumer refresh failed",
					"consumer", consumerID,
					"error", err)
				continue
			}
			
			cd.metrics.mu.Lock()
			cd.metrics.refreshLatencies = append(cd.metrics.refreshLatencies, latency)
			cd.metrics.mu.Unlock()
			
			atomic.AddInt64(&cd.metrics.totalRefreshes, 1)
			
			// Occasionally perform reads
			if rand.Float64() < 0.3 {
				cd.performReads(consumer)
			}
		}
	}
}

func (cd *ComprehensiveDemo) performReads(consumer *DemoConsumer) {
	rs := consumer.consumer.ReadState()
	
	// Perform multiple reads
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("event_%d_%s", 
			rand.Intn(cd.config.TotalEvents)+1,
			cd.config.EventTypes[rand.Intn(len(cd.config.EventTypes))])
		
		start := time.Now()
		_, exists := rs.Get(key)
		latency := time.Since(start)
		
		cd.metrics.mu.Lock()
		cd.metrics.readLatencies = append(cd.metrics.readLatencies, latency)
		cd.metrics.mu.Unlock()
		
		atomic.AddInt64(&cd.metrics.totalReads, 1)
		
		if !exists {
			// This is expected for some random keys
		}
	}
}

func (cd *ComprehensiveDemo) runScenario(ctx context.Context, wg *sync.WaitGroup, scenario string) {
	defer wg.Done()
	
	switch scenario {
	case "diff_testing":
		cd.runDiffTesting(ctx)
	case "performance_testing":
		cd.runPerformanceTesting(ctx)
	case "error_injection":
		cd.runErrorInjection(ctx)
	case "load_testing":
		cd.runLoadTesting(ctx)
	default:
		cd.logger.Info("Unknown scenario", "scenario", scenario)
	}
}

func (cd *ComprehensiveDemo) runDiffTesting(ctx context.Context) {
	cd.logger.Info("🔍 Starting diff testing scenario")
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	var previousData map[string]any
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Generate synthetic data changes
			currentData := cd.generateSyntheticData()
			
			if previousData != nil {
				diff := hollow.DiffData(previousData, currentData)
				
				result := DiffResult{
					Timestamp: time.Now(),
					Added:     len(diff.GetAdded()),
					Removed:   len(diff.GetRemoved()),
					Changed:   len(diff.GetChanged()),
					Impact:    float64(len(diff.GetAdded())+len(diff.GetRemoved())+len(diff.GetChanged())) / float64(len(currentData)) * 100,
				}
				
				cd.metrics.mu.Lock()
				cd.metrics.diffHistory = append(cd.metrics.diffHistory, result)
				cd.metrics.mu.Unlock()
				
				cd.logger.Info("Diff analysis",
					"added", result.Added,
					"removed", result.Removed,
					"changed", result.Changed,
					"impact", fmt.Sprintf("%.1f%%", result.Impact))
			}
			
			previousData = currentData
		}
	}
}

func (cd *ComprehensiveDemo) generateSyntheticData() map[string]any {
	data := make(map[string]any)
	
	// Add some stable data
	data["total_events"] = atomic.LoadInt64(&cd.metrics.totalEvents)
	data["total_cycles"] = atomic.LoadInt64(&cd.metrics.totalCycles)
	
	// Add some changing data
	for _, eventType := range cd.config.EventTypes {
		cd.metrics.mu.RLock()
		count := cd.metrics.eventTypeCounts[eventType]
		cd.metrics.mu.RUnlock()
		data[fmt.Sprintf("count_%s", eventType)] = count
	}
	
	// Add some random data that changes
	data["random_metric"] = rand.Float64() * 1000
	data["timestamp"] = time.Now().Unix()
	
	return data
}

func (cd *ComprehensiveDemo) runPerformanceTesting(ctx context.Context) {
	cd.logger.Info("⚡ Starting performance testing scenario")
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cd.analyzePerformance()
		}
	}
}

func (cd *ComprehensiveDemo) analyzePerformance() {
	cd.metrics.mu.RLock()
	defer cd.metrics.mu.RUnlock()
	
	// Analyze read latencies
	if len(cd.metrics.readLatencies) > 100 {
		// Get recent latencies
		recent := cd.metrics.readLatencies[len(cd.metrics.readLatencies)-100:]
		
		// Calculate percentiles
		sorted := make([]time.Duration, len(recent))
		copy(sorted, recent)
		
		// Simple sort
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		
		p99 := sorted[int(float64(len(sorted))*0.99)]
		target := 5 * time.Microsecond
		
		if p99 > target {
			cd.logger.Warn("Read latency degradation",
				"p99", p99,
				"target", target,
				"ratio", float64(p99)/float64(target))
		}
	}
}

func (cd *ComprehensiveDemo) runErrorInjection(ctx context.Context) {
	cd.logger.Info("💥 Starting error injection scenario")
	
	// This would simulate various error conditions
	// For demo purposes, we'll just log that it's running
	<-ctx.Done()
}

func (cd *ComprehensiveDemo) runLoadTesting(ctx context.Context) {
	cd.logger.Info("🏋️ Starting load testing scenario")
	
	// This would ramp up load over time
	// For demo purposes, we'll just log that it's running
	<-ctx.Done()
}

func (cd *ComprehensiveDemo) monitorSystem(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			
			cd.metrics.mu.Lock()
			cd.metrics.memoryUsage = append(cd.metrics.memoryUsage, float64(m.Alloc)/(1024*1024))
			cd.metrics.goroutineCount = append(cd.metrics.goroutineCount, runtime.NumGoroutine())
			cd.metrics.mu.Unlock()
		}
	}
}

func (cd *ComprehensiveDemo) reportProgress(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cd.printProgressReport()
		}
	}
}

func (cd *ComprehensiveDemo) printProgressReport() {
	elapsed := time.Since(cd.metrics.startTime)
	events := atomic.LoadInt64(&cd.metrics.totalEvents)
	cycles := atomic.LoadInt64(&cd.metrics.totalCycles)
	refreshes := atomic.LoadInt64(&cd.metrics.totalRefreshes)
	reads := atomic.LoadInt64(&cd.metrics.totalReads)
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	cd.mu.RLock()
	phase := cd.currentPhase
	cd.mu.RUnlock()
	
	fmt.Printf("📊 Progress [%s] [%v]: Events=%d, Cycles=%d, Refreshes=%d, Reads=%d, Mem=%.1fMB, Goroutines=%d\n",
		phase, elapsed, events, cycles, refreshes, reads, 
		float64(m.Alloc)/(1024*1024), runtime.NumGoroutine())
}

func (cd *ComprehensiveDemo) GenerateReport() {
	cd.mu.Lock()
	cd.currentPhase = "reporting"
	cd.mu.Unlock()
	
	fmt.Print("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("📈 COMPREHENSIVE DEMO REPORT\n")
	fmt.Print(strings.Repeat("=", 80) + "\n")
	
	totalDuration := time.Since(cd.metrics.startTime)
	
	// Executive Summary
	fmt.Printf("Executive Summary:\n")
	fmt.Printf("  Total Duration:       %v\n", totalDuration)
	fmt.Printf("  Total Events:         %d\n", atomic.LoadInt64(&cd.metrics.totalEvents))
	fmt.Printf("  Total Cycles:         %d\n", atomic.LoadInt64(&cd.metrics.totalCycles))
	fmt.Printf("  Total Refreshes:      %d\n", atomic.LoadInt64(&cd.metrics.totalRefreshes))
	fmt.Printf("  Total Reads:          %d\n", atomic.LoadInt64(&cd.metrics.totalReads))
	fmt.Printf("  Event Types:          %d\n", len(cd.config.EventTypes))
	fmt.Printf("  Producers:            %d\n", cd.config.NumProducers)
	fmt.Printf("  Consumers:            %d\n", cd.config.NumConsumers)
	fmt.Printf("\n")
	
	// Throughput Analysis
	cd.analyzeThroughput(totalDuration)
	
	// Latency Analysis
	cd.analyzeLatencies()
	
	// Event Type Distribution
	cd.analyzeEventTypes()
	
	// Diff Analysis
	cd.analyzeDiffs()
	
	// Error Analysis
	cd.analyzeErrors()
	
	// System Performance
	cd.analyzeSystemPerformance()
	
	// Final Assessment
	cd.performFinalAssessment()
	
	fmt.Print(strings.Repeat("=", 80) + "\n")
	fmt.Printf("✅ Comprehensive Demo Report Complete\n")
}

func (cd *ComprehensiveDemo) analyzeThroughput(totalDuration time.Duration) {
	events := atomic.LoadInt64(&cd.metrics.totalEvents)
	cycles := atomic.LoadInt64(&cd.metrics.totalCycles)
	refreshes := atomic.LoadInt64(&cd.metrics.totalRefreshes)
	reads := atomic.LoadInt64(&cd.metrics.totalReads)
	
	fmt.Printf("Throughput Analysis:\n")
	fmt.Printf("  Events/Second:        %.2f\n", float64(events)/totalDuration.Seconds())
	fmt.Printf("  Cycles/Second:        %.2f\n", float64(cycles)/totalDuration.Seconds())
	fmt.Printf("  Refreshes/Second:     %.2f\n", float64(refreshes)/totalDuration.Seconds())
	fmt.Printf("  Reads/Second:         %.2f\n", float64(reads)/totalDuration.Seconds())
	
	if cycles > 0 {
		fmt.Printf("  Avg Events/Cycle:     %.2f\n", float64(events)/float64(cycles))
	}
	
	fmt.Printf("\n")
}

func (cd *ComprehensiveDemo) analyzeLatencies() {
	cd.metrics.mu.RLock()
	defer cd.metrics.mu.RUnlock()
	
	fmt.Printf("Latency Analysis:\n")
	
	// Cycle latencies
	if len(cd.metrics.cycleLatencies) > 0 {
		avg := cd.calculateAverageLatency(cd.metrics.cycleLatencies)
		p99 := cd.calculatePercentile(cd.metrics.cycleLatencies, 0.99)
		
		fmt.Printf("  Cycle Latencies:\n")
		fmt.Printf("    Average:            %v\n", avg)
		fmt.Printf("    99th Percentile:    %v\n", p99)
	}
	
	// Read latencies
	if len(cd.metrics.readLatencies) > 0 {
		avg := cd.calculateAverageLatency(cd.metrics.readLatencies)
		p99 := cd.calculatePercentile(cd.metrics.readLatencies, 0.99)
		
		fmt.Printf("  Read Latencies:\n")
		fmt.Printf("    Average:            %v\n", avg)
		fmt.Printf("    99th Percentile:    %v", p99)
		
		if p99 <= 5*time.Microsecond {
			fmt.Printf(" ✅ PASS\n")
		} else {
			fmt.Printf(" ❌ FAIL\n")
		}
	}
	
	fmt.Printf("\n")
}

func (cd *ComprehensiveDemo) calculateAverageLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	return total / time.Duration(len(latencies))
}

func (cd *ComprehensiveDemo) calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	
	// Simple sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	index := int(float64(len(sorted)) * percentile)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	
	return sorted[index]
}

func (cd *ComprehensiveDemo) analyzeEventTypes() {
	cd.metrics.mu.RLock()
	defer cd.metrics.mu.RUnlock()
	
	fmt.Printf("Event Type Distribution:\n")
	
	var total int64
	for _, count := range cd.metrics.eventTypeCounts {
		total += count
	}
	
	for eventType, count := range cd.metrics.eventTypeCounts {
		percentage := float64(count) / float64(total) * 100
		fmt.Printf("  %-20s: %6d (%.1f%%)\n", eventType, count, percentage)
	}
	
	fmt.Printf("\n")
}

func (cd *ComprehensiveDemo) analyzeDiffs() {
	cd.metrics.mu.RLock()
	defer cd.metrics.mu.RUnlock()
	
	if len(cd.metrics.diffHistory) == 0 {
		return
	}
	
	fmt.Printf("Diff Analysis:\n")
	fmt.Printf("  Total Diffs:          %d\n", len(cd.metrics.diffHistory))
	
	var totalAdded, totalRemoved, totalChanged int
	var totalImpact float64
	
	for _, diff := range cd.metrics.diffHistory {
		totalAdded += diff.Added
		totalRemoved += diff.Removed
		totalChanged += diff.Changed
		totalImpact += diff.Impact
	}
	
	fmt.Printf("  Total Added:          %d\n", totalAdded)
	fmt.Printf("  Total Removed:        %d\n", totalRemoved)
	fmt.Printf("  Total Changed:        %d\n", totalChanged)
	fmt.Printf("  Average Impact:       %.1f%%\n", totalImpact/float64(len(cd.metrics.diffHistory)))
	
	fmt.Printf("\n")
}

func (cd *ComprehensiveDemo) analyzeErrors() {
	cd.metrics.mu.RLock()
	defer cd.metrics.mu.RUnlock()
	
	fmt.Printf("Error Analysis:\n")
	
	var totalErrors int64
	for _, count := range cd.metrics.errorsByType {
		totalErrors += count
	}
	
	fmt.Printf("  Total Errors:         %d\n", totalErrors)
	
	for errorType, count := range cd.metrics.errorsByType {
		fmt.Printf("  %-20s: %d\n", errorType, count)
	}
	
	fmt.Printf("\n")
}

func (cd *ComprehensiveDemo) analyzeSystemPerformance() {
	cd.metrics.mu.RLock()
	defer cd.metrics.mu.RUnlock()
	
	fmt.Printf("System Performance:\n")
	
	if len(cd.metrics.memoryUsage) > 0 {
		maxMem := cd.metrics.memoryUsage[0]
		for _, mem := range cd.metrics.memoryUsage {
			if mem > maxMem {
				maxMem = mem
			}
		}
		
		fmt.Printf("  Peak Memory:          %.1f MB\n", maxMem)
	}
	
	if len(cd.metrics.goroutineCount) > 0 {
		maxGoroutines := cd.metrics.goroutineCount[0]
		for _, count := range cd.metrics.goroutineCount {
			if count > maxGoroutines {
				maxGoroutines = count
			}
		}
		
		fmt.Printf("  Peak Goroutines:      %d\n", maxGoroutines)
	}
	
	fmt.Printf("\n")
}

func (cd *ComprehensiveDemo) performFinalAssessment() {
	fmt.Printf("Final Assessment:\n")
	
	// Check read latency target
	cd.metrics.mu.RLock()
	readLatencies := cd.metrics.readLatencies
	cd.metrics.mu.RUnlock()
	
	if len(readLatencies) > 0 {
		p99 := cd.calculatePercentile(readLatencies, 0.99)
		if p99 <= 5*time.Microsecond {
			fmt.Printf("  ✅ Read Performance: PASS (p99: %v)\n", p99)
		} else {
			fmt.Printf("  ❌ Read Performance: FAIL (p99: %v)\n", p99)
		}
	}
	
	// Check error rate
	cd.metrics.mu.RLock()
	totalErrors := int64(0)
	for _, count := range cd.metrics.errorsByType {
		totalErrors += count
	}
	cd.metrics.mu.RUnlock()
	
	totalOps := atomic.LoadInt64(&cd.metrics.totalEvents) + 
		atomic.LoadInt64(&cd.metrics.totalRefreshes) + 
		atomic.LoadInt64(&cd.metrics.totalReads)
	
	if totalOps > 0 {
		errorRate := float64(totalErrors) / float64(totalOps) * 100
		if errorRate < 1.0 {
			fmt.Printf("  ✅ Error Rate: PASS (%.3f%%)\n", errorRate)
		} else {
			fmt.Printf("  ❌ Error Rate: FAIL (%.3f%%)\n", errorRate)
		}
	}
	
	// Check throughput
	totalDuration := time.Since(cd.metrics.startTime)
	eventsPerSecond := float64(atomic.LoadInt64(&cd.metrics.totalEvents)) / totalDuration.Seconds()
	
	if eventsPerSecond >= 100 {
		fmt.Printf("  ✅ Throughput: PASS (%.1f events/s)\n", eventsPerSecond)
	} else {
		fmt.Printf("  ⚠️  Throughput: LOW (%.1f events/s)\n", eventsPerSecond)
	}
	
	fmt.Printf("\n")
}

func main() {
	fmt.Printf("🌟 Hollow-Go Comprehensive Demo\n")
	fmt.Printf("Showcasing all capabilities with 10,000 events over 10 minutes\n\n")
	
	config := DemoConfig{
		TotalEvents:   10000,
		Duration:      10 * time.Minute,
		NumProducers:  2,
		NumConsumers:  3,
		EventTypes:    []string{"user_login", "page_view", "purchase", "search", "logout"},
		UseNormalDist: true,
		EnableMetrics: true,
		EnableDiff:    true,
		EnablePerf:    true,
		TestScenarios: []string{"diff_testing", "performance_testing"},
	}
	
	demo := NewComprehensiveDemo(config)
	
	if err := demo.Initialize(); err != nil {
		fmt.Printf("❌ Initialization failed: %v\n", err)
		return
	}
	
	if err := demo.RunDemo(); err != nil {
		fmt.Printf("❌ Demo failed: %v\n", err)
		return
	}
	
	demo.GenerateReport()
	
	fmt.Printf("\n🎉 Comprehensive Demo completed successfully!\n")
	fmt.Printf("All Hollow-Go capabilities have been demonstrated.\n")
}
