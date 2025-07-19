package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leowmjw/go-hollow"
	"github.com/leowmjw/go-hollow/internal/memblob"
)

// PerformanceTest manages comprehensive performance testing
type PerformanceTest struct {
	config    PerformanceConfig
	metrics   *PerformanceMetrics
	blob      *memblob.Store
	producer  *hollow.Producer
	consumers []*hollow.Consumer
	logger    *slog.Logger
}

type PerformanceConfig struct {
	TotalEvents       int
	Duration          time.Duration
	NumProducers      int
	NumConsumers      int
	BatchSizeMin      int
	BatchSizeMax      int
	ReadTestInterval  time.Duration
	MemoryProfiler    bool
	CPUProfiler       bool
	TargetReadLatency time.Duration // Target: 5µs p99
}

type PerformanceMetrics struct {
	mu sync.RWMutex

	// Throughput metrics
	totalEvents    int64
	totalCycles    int64
	totalRefreshes int64
	totalReads     int64

	// Latency metrics
	cycleLatencies   []time.Duration
	refreshLatencies []time.Duration
	readLatencies    []time.Duration

	// Memory metrics
	memoryUsage []MemorySnapshot
	gcStats     []GCSnapshot

	// CPU metrics
	cpuUsage []CPUSnapshot

	// Error metrics
	producerErrors int64
	consumerErrors int64
	readErrors     int64

	// Concurrency metrics
	goroutineCount []int

	startTime  time.Time
	lastGCTime time.Time
}

type MemorySnapshot struct {
	Timestamp       time.Time
	AllocBytes      uint64
	TotalAllocBytes uint64
	HeapBytes       uint64
	StackBytes      uint64
	NumGC           uint32
}

type GCSnapshot struct {
	Timestamp  time.Time
	NumGC      uint32
	PauseTotal time.Duration
	PauseNs    uint64
}

type CPUSnapshot struct {
	Timestamp  time.Time
	Goroutines int
	CPUUsage   float64
}

func NewPerformanceTest(config PerformanceConfig) *PerformanceTest {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Reduce noise during performance testing
	}))

	return &PerformanceTest{
		config:  config,
		metrics: NewPerformanceMetrics(),
		blob:    memblob.New(),
		logger:  logger,
	}
}

func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		cycleLatencies:   make([]time.Duration, 0, 10000),
		refreshLatencies: make([]time.Duration, 0, 10000),
		readLatencies:    make([]time.Duration, 0, 100000),
		memoryUsage:      make([]MemorySnapshot, 0, 1000),
		gcStats:          make([]GCSnapshot, 0, 1000),
		cpuUsage:         make([]CPUSnapshot, 0, 1000),
		goroutineCount:   make([]int, 0, 1000),
		startTime:        time.Now(),
	}
}

func (pt *PerformanceTest) Setup() {
	// Create producer
	pt.producer = hollow.NewProducer(
		hollow.WithBlobStager(pt.blob),
		hollow.WithLogger(pt.logger),
		hollow.WithMetricsCollector(func(m hollow.Metrics) {
			pt.metrics.recordCycle(m.RecordCount, m.ByteSize)
		}),
	)

	// Create consumers
	pt.consumers = make([]*hollow.Consumer, pt.config.NumConsumers)
	for i := 0; i < pt.config.NumConsumers; i++ {
		pt.consumers[i] = hollow.NewConsumer(
			hollow.WithBlobRetriever(pt.blob),
			hollow.WithAnnouncementWatcher(func() (uint64, bool, error) {
				latest, err := pt.blob.Latest(context.Background())
				return latest, latest > 0, err
			}),
			hollow.WithConsumerLogger(pt.logger),
		)
	}

	fmt.Printf("Performance test setup complete:\n")
	fmt.Printf("  Target Events:        %d\n", pt.config.TotalEvents)
	fmt.Printf("  Duration:             %v\n", pt.config.Duration)
	fmt.Printf("  Producers:            %d\n", pt.config.NumProducers)
	fmt.Printf("  Consumers:            %d\n", pt.config.NumConsumers)
	fmt.Printf("  Target Read Latency:  %v p99\n", pt.config.TargetReadLatency)
	fmt.Printf("  Memory Profiling:     %v\n", pt.config.MemoryProfiler)
	fmt.Printf("  CPU Profiling:        %v\n", pt.config.CPUProfiler)
	fmt.Printf("\n")
}

func (pt *PerformanceTest) Run() {
	fmt.Printf("Starting performance test...\n")

	ctx, cancel := context.WithTimeout(context.Background(), pt.config.Duration)
	defer cancel()

	var wg sync.WaitGroup

	// Start system monitoring
	wg.Add(1)
	go pt.monitorSystem(ctx, &wg)

	// Start producers
	for i := 0; i < pt.config.NumProducers; i++ {
		wg.Add(1)
		go pt.runProducer(ctx, &wg, i)
	}

	// Start consumers
	for i, consumer := range pt.consumers {
		wg.Add(1)
		go pt.runConsumer(ctx, &wg, consumer, i)
	}

	// Start read performance tests
	wg.Add(1)
	go pt.runReadPerformanceTest(ctx, &wg)

	// Start progress reporter
	wg.Add(1)
	go pt.reportProgress(ctx, &wg)

	// Wait for completion
	wg.Wait()

	fmt.Printf("\nPerformance test completed!\n")
}

func (pt *PerformanceTest) runProducer(ctx context.Context, wg *sync.WaitGroup, producerID int) {
	defer wg.Done()

	eventCount := 0
	eventsPerProducer := pt.config.TotalEvents / pt.config.NumProducers

	for eventCount < eventsPerProducer {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Variable batch size
		batchSize := rand.Intn(pt.config.BatchSizeMax-pt.config.BatchSizeMin) + pt.config.BatchSizeMin

		// Measure cycle latency
		start := time.Now()

		_, err := pt.producer.RunCycle(func(ws hollow.WriteState) error {
			for i := 0; i < batchSize && eventCount < eventsPerProducer; i++ {
				key := fmt.Sprintf("producer_%d_event_%d", producerID, eventCount)
				if err := ws.Add(key); err != nil {
					return err
				}
				eventCount++
			}
			return nil
		})

		latency := time.Since(start)

		if err != nil {
			atomic.AddInt64(&pt.metrics.producerErrors, 1)
			pt.logger.Error("Producer cycle failed", "producer", producerID, "error", err)
			continue
		}

		pt.metrics.recordCycleLatency(latency)

		// Small delay to avoid overwhelming the system
		time.Sleep(time.Millisecond)
	}
}

func (pt *PerformanceTest) runConsumer(ctx context.Context, wg *sync.WaitGroup, consumer *hollow.Consumer, consumerID int) {
	defer wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()

			err := consumer.Refresh()
			latency := time.Since(start)

			if err != nil {
				atomic.AddInt64(&pt.metrics.consumerErrors, 1)
				pt.logger.Error("Consumer refresh failed", "consumer", consumerID, "error", err)
				continue
			}

			pt.metrics.recordRefreshLatency(latency)
			atomic.AddInt64(&pt.metrics.totalRefreshes, 1)
		}
	}
}

func (pt *PerformanceTest) runReadPerformanceTest(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(pt.config.ReadTestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pt.performReadTest()
		}
	}
}

func (pt *PerformanceTest) performReadTest() {
	// Use first consumer for read tests
	if len(pt.consumers) == 0 {
		return
	}

	rs := pt.consumers[0].ReadState()
	if rs.Size() == 0 {
		return
	}

	// Perform multiple reads to get statistically significant results
	numReads := 1000
	keys := make([]string, numReads)

	// Generate random keys
	for i := 0; i < numReads; i++ {
		producerID := rand.Intn(pt.config.NumProducers)
		eventID := rand.Intn(pt.config.TotalEvents / pt.config.NumProducers)
		keys[i] = fmt.Sprintf("producer_%d_event_%d", producerID, eventID)
	}

	// Measure read latencies
	for _, key := range keys {
		start := time.Now()
		_, exists := rs.Get(key)
		latency := time.Since(start)

		pt.metrics.recordReadLatency(latency)
		atomic.AddInt64(&pt.metrics.totalReads, 1)

		if !exists {
			// This is expected for some keys
		}
	}
}

func (pt *PerformanceTest) monitorSystem(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pt.captureMemorySnapshot()
			pt.captureGCSnapshot()
			pt.captureCPUSnapshot()
		}
	}
}

func (pt *PerformanceTest) captureMemorySnapshot() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	snapshot := MemorySnapshot{
		Timestamp:       time.Now(),
		AllocBytes:      m.Alloc,
		TotalAllocBytes: m.TotalAlloc,
		HeapBytes:       m.HeapAlloc,
		StackBytes:      m.StackInuse,
		NumGC:           m.NumGC,
	}

	pt.metrics.mu.Lock()
	pt.metrics.memoryUsage = append(pt.metrics.memoryUsage, snapshot)
	pt.metrics.mu.Unlock()
}

func (pt *PerformanceTest) captureGCSnapshot() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	snapshot := GCSnapshot{
		Timestamp:  time.Now(),
		NumGC:      m.NumGC,
		PauseTotal: time.Duration(m.PauseTotalNs),
		PauseNs:    m.PauseNs[(m.NumGC+255)%256],
	}

	pt.metrics.mu.Lock()
	pt.metrics.gcStats = append(pt.metrics.gcStats, snapshot)
	pt.metrics.mu.Unlock()
}

func (pt *PerformanceTest) captureCPUSnapshot() {
	snapshot := CPUSnapshot{
		Timestamp:  time.Now(),
		Goroutines: runtime.NumGoroutine(),
		CPUUsage:   0.0, // Would need additional CPU monitoring
	}

	pt.metrics.mu.Lock()
	pt.metrics.cpuUsage = append(pt.metrics.cpuUsage, snapshot)
	pt.metrics.goroutineCount = append(pt.metrics.goroutineCount, snapshot.Goroutines)
	pt.metrics.mu.Unlock()
}

func (pt *PerformanceTest) reportProgress(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pt.printProgressReport()
		}
	}
}

func (pt *PerformanceTest) printProgressReport() {
	elapsed := time.Since(pt.metrics.startTime)
	events := atomic.LoadInt64(&pt.metrics.totalEvents)
	cycles := atomic.LoadInt64(&pt.metrics.totalCycles)
	refreshes := atomic.LoadInt64(&pt.metrics.totalRefreshes)
	reads := atomic.LoadInt64(&pt.metrics.totalReads)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("Progress [%v]: Events=%d, Cycles=%d, Refreshes=%d, Reads=%d, Mem=%.1fMB, GC=%d\n",
		elapsed, events, cycles, refreshes, reads,
		float64(m.Alloc)/(1024*1024), m.NumGC)
}

func (pt *PerformanceTest) GenerateReport() {
	fmt.Print("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("PERFORMANCE TEST REPORT\n")
	fmt.Print(strings.Repeat("=", 80) + "\n")

	totalDuration := time.Since(pt.metrics.startTime)

	// Basic statistics
	fmt.Printf("Test Configuration:\n")
	fmt.Printf("  Duration:             %v\n", totalDuration)
	fmt.Printf("  Target Events:        %d\n", pt.config.TotalEvents)
	fmt.Printf("  Producers:            %d\n", pt.config.NumProducers)
	fmt.Printf("  Consumers:            %d\n", pt.config.NumConsumers)
	fmt.Printf("\n")

	// Throughput metrics
	events := atomic.LoadInt64(&pt.metrics.totalEvents)
	cycles := atomic.LoadInt64(&pt.metrics.totalCycles)
	refreshes := atomic.LoadInt64(&pt.metrics.totalRefreshes)
	reads := atomic.LoadInt64(&pt.metrics.totalReads)

	fmt.Printf("Throughput Metrics:\n")
	fmt.Printf("  Total Events:         %d\n", events)
	fmt.Printf("  Total Cycles:         %d\n", cycles)
	fmt.Printf("  Total Refreshes:      %d\n", refreshes)
	fmt.Printf("  Total Reads:          %d\n", reads)
	fmt.Printf("  Events/Second:        %.2f\n", float64(events)/totalDuration.Seconds())
	fmt.Printf("  Cycles/Second:        %.2f\n", float64(cycles)/totalDuration.Seconds())
	fmt.Printf("  Refreshes/Second:     %.2f\n", float64(refreshes)/totalDuration.Seconds())
	fmt.Printf("  Reads/Second:         %.2f\n", float64(reads)/totalDuration.Seconds())
	fmt.Printf("\n")

	// Latency analysis
	pt.analyzeLatencies()

	// Memory analysis
	pt.analyzeMemory()

	// Error analysis
	pt.analyzeErrors()

	// Performance assessment
	pt.assessPerformance()

	fmt.Print(strings.Repeat("=", 80) + "\n")
}

func (pt *PerformanceTest) analyzeLatencies() {
	pt.metrics.mu.RLock()
	defer pt.metrics.mu.RUnlock()

	fmt.Printf("Latency Analysis:\n")

	// Cycle latencies
	if len(pt.metrics.cycleLatencies) > 0 {
		latencies := make([]time.Duration, len(pt.metrics.cycleLatencies))
		copy(latencies, pt.metrics.cycleLatencies)

		// Sort for percentiles
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}

		avg := pt.calculateAverage(latencies)
		p50 := latencies[len(latencies)/2]
		p95 := latencies[int(float64(len(latencies))*0.95)]
		p99 := latencies[int(float64(len(latencies))*0.99)]

		fmt.Printf("  Cycle Latencies:\n")
		fmt.Printf("    Average:            %v\n", avg)
		fmt.Printf("    50th Percentile:    %v\n", p50)
		fmt.Printf("    95th Percentile:    %v\n", p95)
		fmt.Printf("    99th Percentile:    %v\n", p99)
		fmt.Printf("    Min:                %v\n", latencies[0])
		fmt.Printf("    Max:                %v\n", latencies[len(latencies)-1])
	}

	// Read latencies
	if len(pt.metrics.readLatencies) > 0 {
		latencies := make([]time.Duration, len(pt.metrics.readLatencies))
		copy(latencies, pt.metrics.readLatencies)

		// Sort for percentiles
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}

		avg := pt.calculateAverage(latencies)
		p50 := latencies[len(latencies)/2]
		p95 := latencies[int(float64(len(latencies))*0.95)]
		p99 := latencies[int(float64(len(latencies))*0.99)]

		fmt.Printf("  Read Latencies:\n")
		fmt.Printf("    Average:            %v\n", avg)
		fmt.Printf("    50th Percentile:    %v\n", p50)
		fmt.Printf("    95th Percentile:    %v\n", p95)
		fmt.Printf("    99th Percentile:    %v (%s)\n", p99, pt.assessReadLatency(p99))
		fmt.Printf("    Min:                %v\n", latencies[0])
		fmt.Printf("    Max:                %v\n", latencies[len(latencies)-1])
	}

	fmt.Printf("\n")
}

func (pt *PerformanceTest) calculateAverage(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	return total / time.Duration(len(latencies))
}

func (pt *PerformanceTest) assessReadLatency(p99 time.Duration) string {
	if p99 <= pt.config.TargetReadLatency {
		return "✓ PASS"
	}
	return "✗ FAIL"
}

func (pt *PerformanceTest) analyzeMemory() {
	pt.metrics.mu.RLock()
	defer pt.metrics.mu.RUnlock()

	if len(pt.metrics.memoryUsage) == 0 {
		return
	}

	fmt.Printf("Memory Analysis:\n")

	first := pt.metrics.memoryUsage[0]
	last := pt.metrics.memoryUsage[len(pt.metrics.memoryUsage)-1]

	fmt.Printf("  Memory Growth:\n")
	fmt.Printf("    Initial Alloc:      %.2f MB\n", float64(first.AllocBytes)/(1024*1024))
	fmt.Printf("    Final Alloc:        %.2f MB\n", float64(last.AllocBytes)/(1024*1024))
	fmt.Printf("    Total Allocated:    %.2f MB\n", float64(last.TotalAllocBytes)/(1024*1024))
	fmt.Printf("    GC Count:           %d\n", last.NumGC)

	// Calculate peak memory
	var peakAlloc, peakHeap uint64
	for _, snapshot := range pt.metrics.memoryUsage {
		if snapshot.AllocBytes > peakAlloc {
			peakAlloc = snapshot.AllocBytes
		}
		if snapshot.HeapBytes > peakHeap {
			peakHeap = snapshot.HeapBytes
		}
	}

	fmt.Printf("  Peak Usage:\n")
	fmt.Printf("    Peak Alloc:         %.2f MB\n", float64(peakAlloc)/(1024*1024))
	fmt.Printf("    Peak Heap:          %.2f MB\n", float64(peakHeap)/(1024*1024))

	fmt.Printf("\n")
}

func (pt *PerformanceTest) analyzeErrors() {
	producerErrors := atomic.LoadInt64(&pt.metrics.producerErrors)
	consumerErrors := atomic.LoadInt64(&pt.metrics.consumerErrors)
	readErrors := atomic.LoadInt64(&pt.metrics.readErrors)

	fmt.Printf("Error Analysis:\n")
	fmt.Printf("  Producer Errors:      %d\n", producerErrors)
	fmt.Printf("  Consumer Errors:      %d\n", consumerErrors)
	fmt.Printf("  Read Errors:          %d\n", readErrors)
	fmt.Printf("  Total Errors:         %d\n", producerErrors+consumerErrors+readErrors)
	fmt.Printf("\n")
}

func (pt *PerformanceTest) assessPerformance() {
	fmt.Printf("Performance Assessment:\n")

	// Read latency assessment
	if len(pt.metrics.readLatencies) > 0 {
		pt.metrics.mu.RLock()
		latencies := make([]time.Duration, len(pt.metrics.readLatencies))
		copy(latencies, pt.metrics.readLatencies)
		pt.metrics.mu.RUnlock()

		// Sort for percentiles
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}

		p99 := latencies[int(float64(len(latencies))*0.99)]

		fmt.Printf("  Read Latency Target:  %v\n", pt.config.TargetReadLatency)
		fmt.Printf("  Read Latency P99:     %v\n", p99)
		if p99 <= pt.config.TargetReadLatency {
			fmt.Printf("  Read Performance:     ✓ PASS\n")
		} else {
			fmt.Printf("  Read Performance:     ✗ FAIL (%.2fx slower than target)\n",
				float64(p99)/float64(pt.config.TargetReadLatency))
		}
	}

	// Throughput assessment
	totalDuration := time.Since(pt.metrics.startTime)
	events := atomic.LoadInt64(&pt.metrics.totalEvents)
	eventsPerSecond := float64(events) / totalDuration.Seconds()

	fmt.Printf("  Throughput:           %.2f events/second\n", eventsPerSecond)

	// Error rate assessment
	totalOps := events + atomic.LoadInt64(&pt.metrics.totalRefreshes) + atomic.LoadInt64(&pt.metrics.totalReads)
	totalErrors := atomic.LoadInt64(&pt.metrics.producerErrors) +
		atomic.LoadInt64(&pt.metrics.consumerErrors) +
		atomic.LoadInt64(&pt.metrics.readErrors)

	if totalOps > 0 {
		errorRate := float64(totalErrors) / float64(totalOps) * 100
		fmt.Printf("  Error Rate:           %.4f%%\n", errorRate)
		if errorRate < 0.1 {
			fmt.Printf("  Error Performance:    ✓ PASS\n")
		} else {
			fmt.Printf("  Error Performance:    ✗ FAIL (too many errors)\n")
		}
	}

	fmt.Printf("\n")
}

// Record methods for metrics
func (pm *PerformanceMetrics) recordCycle(eventCount int, byteSize int64) {
	atomic.AddInt64(&pm.totalEvents, int64(eventCount))
	atomic.AddInt64(&pm.totalCycles, 1)
}

func (pm *PerformanceMetrics) recordCycleLatency(latency time.Duration) {
	pm.mu.Lock()
	pm.cycleLatencies = append(pm.cycleLatencies, latency)
	pm.mu.Unlock()
}

func (pm *PerformanceMetrics) recordRefreshLatency(latency time.Duration) {
	pm.mu.Lock()
	pm.refreshLatencies = append(pm.refreshLatencies, latency)
	pm.mu.Unlock()
}

func (pm *PerformanceMetrics) recordReadLatency(latency time.Duration) {
	pm.mu.Lock()
	pm.readLatencies = append(pm.readLatencies, latency)
	pm.mu.Unlock()
}

func main() {
	fmt.Printf("Hollow-Go Performance Test\n")
	fmt.Printf("Testing ultra-fast read performance with 10k events\n\n")

	config := PerformanceConfig{
		TotalEvents:       10000,
		Duration:          10 * time.Minute,
		NumProducers:      2,
		NumConsumers:      3,
		BatchSizeMin:      5,
		BatchSizeMax:      50,
		ReadTestInterval:  500 * time.Millisecond,
		MemoryProfiler:    true,
		CPUProfiler:       true,
		TargetReadLatency: 5 * time.Microsecond, // 5µs p99 target
	}

	test := NewPerformanceTest(config)
	test.Setup()
	test.Run()
	test.GenerateReport()

	fmt.Printf("Performance test completed successfully!\n")
}
