package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/leowmjw/go-hollow"
	"github.com/leowmjw/go-hollow/internal/memblob"
)

// MetricsCollector showcases comprehensive metrics collection
type MetricsCollector struct {
	mu sync.RWMutex

	// Basic metrics
	totalCycles int64
	totalEvents int64
	totalBytes  int64

	// Performance metrics
	cycleLatencies  []time.Duration
	cycleTimestamps []time.Time

	// Data quality metrics
	versionHistory     []uint64
	recordCountHistory []int
	byteSizeHistory    []int64

	// Throughput metrics
	eventsPerSecond []int64
	cyclesPerSecond []int64

	// Error metrics
	errorCount int64

	startTime time.Time
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime:          time.Now(),
		cycleLatencies:     make([]time.Duration, 0),
		cycleTimestamps:    make([]time.Time, 0),
		versionHistory:     make([]uint64, 0),
		recordCountHistory: make([]int, 0),
		byteSizeHistory:    make([]int64, 0),
		eventsPerSecond:    make([]int64, 0),
		cyclesPerSecond:    make([]int64, 0),
	}
}

func (mc *MetricsCollector) Collect(m hollow.Metrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()

	// Update basic metrics
	mc.totalCycles++
	mc.totalEvents += int64(m.RecordCount)
	mc.totalBytes += m.ByteSize

	// Record performance metrics
	mc.cycleTimestamps = append(mc.cycleTimestamps, now)

	// Calculate cycle latency (time since last cycle)
	if len(mc.cycleTimestamps) > 1 {
		lastCycle := mc.cycleTimestamps[len(mc.cycleTimestamps)-2]
		latency := now.Sub(lastCycle)
		mc.cycleLatencies = append(mc.cycleLatencies, latency)
	}

	// Record data quality metrics
	mc.versionHistory = append(mc.versionHistory, m.Version)
	mc.recordCountHistory = append(mc.recordCountHistory, m.RecordCount)
	mc.byteSizeHistory = append(mc.byteSizeHistory, m.ByteSize)

	// Update throughput metrics
	second := int64(now.Sub(mc.startTime).Seconds())
	mc.ensureSliceSize(&mc.eventsPerSecond, second)
	mc.ensureSliceSize(&mc.cyclesPerSecond, second)

	if second >= 0 && second < int64(len(mc.eventsPerSecond)) {
		mc.eventsPerSecond[second] += int64(m.RecordCount)
		mc.cyclesPerSecond[second]++
	}

	// Log metrics every 10 cycles
	if mc.totalCycles%10 == 0 {
		mc.logCurrentMetrics()
	}
}

func (mc *MetricsCollector) ensureSliceSize(slice *[]int64, minSize int64) {
	for int64(len(*slice)) <= minSize {
		*slice = append(*slice, 0)
	}
}

func (mc *MetricsCollector) logCurrentMetrics() {
	// Don't hold lock while logging
	mc.mu.RUnlock()
	defer mc.mu.RLock()

	elapsed := time.Since(mc.startTime)
	fmt.Printf("Metrics Update: Cycles=%d, Events=%d, Bytes=%d, Elapsed=%v\n",
		mc.totalCycles, mc.totalEvents, mc.totalBytes, elapsed)
}

// GenerateDetailedReport creates a comprehensive metrics report
func (mc *MetricsCollector) GenerateDetailedReport() {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	totalDuration := time.Since(mc.startTime)

	fmt.Print("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("COMPREHENSIVE METRICS REPORT\n")
	fmt.Print(strings.Repeat("=", 80) + "\n")

	// Basic Statistics
	fmt.Printf("Basic Statistics:\n")
	fmt.Printf("  Total Duration:       %v\n", totalDuration)
	fmt.Printf("  Total Cycles:         %d\n", mc.totalCycles)
	fmt.Printf("  Total Events:         %d\n", mc.totalEvents)
	fmt.Printf("  Total Bytes:          %d (%.2f MB)\n", mc.totalBytes, float64(mc.totalBytes)/(1024*1024))
	fmt.Printf("  Average Events/Cycle: %.2f\n", float64(mc.totalEvents)/float64(mc.totalCycles))
	fmt.Printf("  Average Bytes/Cycle:  %.2f\n", float64(mc.totalBytes)/float64(mc.totalCycles))
	fmt.Printf("\n")

	// Throughput Analysis
	fmt.Printf("Throughput Analysis:\n")
	fmt.Printf("  Events/Second:        %.2f\n", float64(mc.totalEvents)/totalDuration.Seconds())
	fmt.Printf("  Cycles/Second:        %.2f\n", float64(mc.totalCycles)/totalDuration.Seconds())
	fmt.Printf("  Bytes/Second:         %.2f (%.2f KB/s)\n",
		float64(mc.totalBytes)/totalDuration.Seconds(),
		float64(mc.totalBytes)/(1024*totalDuration.Seconds()))
	fmt.Printf("\n")

	// Latency Analysis
	if len(mc.cycleLatencies) > 0 {
		fmt.Printf("Latency Analysis:\n")

		var total time.Duration
		min, max := mc.cycleLatencies[0], mc.cycleLatencies[0]

		for _, latency := range mc.cycleLatencies {
			total += latency
			if latency < min {
				min = latency
			}
			if latency > max {
				max = latency
			}
		}

		avg := total / time.Duration(len(mc.cycleLatencies))

		// Calculate percentiles
		latencies := make([]time.Duration, len(mc.cycleLatencies))
		copy(latencies, mc.cycleLatencies)

		// Simple sort for percentiles
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}

		p50 := latencies[len(latencies)/2]
		p95 := latencies[int(float64(len(latencies))*0.95)]
		p99 := latencies[int(float64(len(latencies))*0.99)]

		fmt.Printf("  Average Latency:      %v\n", avg)
		fmt.Printf("  Minimum Latency:      %v\n", min)
		fmt.Printf("  Maximum Latency:      %v\n", max)
		fmt.Printf("  50th Percentile:      %v\n", p50)
		fmt.Printf("  95th Percentile:      %v\n", p95)
		fmt.Printf("  99th Percentile:      %v\n", p99)
		fmt.Printf("\n")
	}

	// Data Quality Analysis
	fmt.Printf("Data Quality Analysis:\n")
	if len(mc.versionHistory) > 0 {
		fmt.Printf("  Version Range:        %d - %d\n",
			mc.versionHistory[0], mc.versionHistory[len(mc.versionHistory)-1])
		fmt.Printf("  Version Growth:       %d versions\n",
			mc.versionHistory[len(mc.versionHistory)-1]-mc.versionHistory[0])
	}

	if len(mc.recordCountHistory) > 0 {
		minRecords, maxRecords := mc.recordCountHistory[0], mc.recordCountHistory[0]
		var totalRecords int64

		for _, count := range mc.recordCountHistory {
			totalRecords += int64(count)
			if count < minRecords {
				minRecords = count
			}
			if count > maxRecords {
				maxRecords = count
			}
		}

		avgRecords := float64(totalRecords) / float64(len(mc.recordCountHistory))
		fmt.Printf("  Records per Cycle:    %.2f avg (min: %d, max: %d)\n",
			avgRecords, minRecords, maxRecords)
	}

	if len(mc.byteSizeHistory) > 0 {
		minBytes, maxBytes := mc.byteSizeHistory[0], mc.byteSizeHistory[0]
		var totalBytes int64

		for _, size := range mc.byteSizeHistory {
			totalBytes += size
			if size < minBytes {
				minBytes = size
			}
			if size > maxBytes {
				maxBytes = size
			}
		}

		avgBytes := float64(totalBytes) / float64(len(mc.byteSizeHistory))
		fmt.Printf("  Bytes per Cycle:      %.2f avg (min: %d, max: %d)\n",
			avgBytes, minBytes, maxBytes)
	}
	fmt.Printf("\n")

	// Time Series Analysis
	fmt.Printf("Time Series Analysis:\n")
	mc.printTimeSeriesDistribution("Events", mc.eventsPerSecond)
	mc.printTimeSeriesDistribution("Cycles", mc.cyclesPerSecond)

	// Export metrics to JSON
	mc.exportMetricsToJSON()

	fmt.Print(strings.Repeat("=", 80) + "\n")
}

func (mc *MetricsCollector) printTimeSeriesDistribution(name string, data []int64) {
	if len(data) == 0 {
		return
	}

	// Group by 30-second windows
	windowSize := 30
	fmt.Printf("  %s Distribution (30-second windows):\n", name)

	for i := 0; i < len(data); i += windowSize {
		var windowTotal int64
		windowEnd := i + windowSize
		if windowEnd > len(data) {
			windowEnd = len(data)
		}

		for j := i; j < windowEnd; j++ {
			windowTotal += data[j]
		}

		fmt.Printf("    %3d-%3ds: %6d %s ", i, windowEnd, windowTotal, name)

		// Simple bar chart
		maxBar := int64(40)
		bars := windowTotal / 10 // Scale for display
		if bars > maxBar {
			bars = maxBar
		}

		for k := int64(0); k < bars; k++ {
			fmt.Print("█")
		}
		fmt.Println()
	}
}

func (mc *MetricsCollector) exportMetricsToJSON() {
	// Create a summary for JSON export
	summary := map[string]interface{}{
		"total_duration_seconds": time.Since(mc.startTime).Seconds(),
		"total_cycles":           mc.totalCycles,
		"total_events":           mc.totalEvents,
		"total_bytes":            mc.totalBytes,
		"version_history":        mc.versionHistory,
		"record_count_history":   mc.recordCountHistory,
		"byte_size_history":      mc.byteSizeHistory,
		"events_per_second":      mc.eventsPerSecond,
		"cycles_per_second":      mc.cyclesPerSecond,
	}

	// Add calculated metrics
	if mc.totalCycles > 0 {
		summary["avg_events_per_cycle"] = float64(mc.totalEvents) / float64(mc.totalCycles)
		summary["avg_bytes_per_cycle"] = float64(mc.totalBytes) / float64(mc.totalCycles)
	}

	if len(mc.cycleLatencies) > 0 {
		var total time.Duration
		for _, lat := range mc.cycleLatencies {
			total += lat
		}
		summary["avg_cycle_latency_ms"] = float64(total.Nanoseconds()) / float64(len(mc.cycleLatencies)) / 1e6
	}

	// Write to file
	file, err := os.Create("metrics_report.json")
	if err != nil {
		fmt.Printf("    Error creating metrics file: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(summary); err != nil {
		fmt.Printf("    Error encoding metrics: %v\n", err)
		return
	}

	fmt.Printf("    Metrics exported to: metrics_report.json\n")
}

// CustomMetricsCollector demonstrates custom metrics collection
type CustomMetricsCollector struct {
	*MetricsCollector

	// Custom metrics
	peakEventsPerSecond int64
	peakCyclesPerSecond int64
	efficiencyScore     float64

	// Business metrics
	userEventCounts map[string]int64
	eventTypeCounts map[string]int64
}

func NewCustomMetricsCollector() *CustomMetricsCollector {
	return &CustomMetricsCollector{
		MetricsCollector: NewMetricsCollector(),
		userEventCounts:  make(map[string]int64),
		eventTypeCounts:  make(map[string]int64),
	}
}

func (cmc *CustomMetricsCollector) Collect(m hollow.Metrics) {
	// Call parent collector
	cmc.MetricsCollector.Collect(m)

	// Custom metrics calculation
	cmc.calculateCustomMetrics()

	// Simulate business metrics tracking
	cmc.trackBusinessMetrics(m)
}

func (cmc *CustomMetricsCollector) calculateCustomMetrics() {
	cmc.mu.Lock()
	defer cmc.mu.Unlock()

	// Calculate peak throughput
	for _, eps := range cmc.eventsPerSecond {
		if eps > cmc.peakEventsPerSecond {
			cmc.peakEventsPerSecond = eps
		}
	}

	for _, cps := range cmc.cyclesPerSecond {
		if cps > cmc.peakCyclesPerSecond {
			cmc.peakCyclesPerSecond = cps
		}
	}

	// Calculate efficiency score (events per cycle vs. ideal)
	idealEventsPerCycle := 50.0 // Ideal batch size
	if cmc.totalCycles > 0 {
		actualEventsPerCycle := float64(cmc.totalEvents) / float64(cmc.totalCycles)
		cmc.efficiencyScore = math.Min(actualEventsPerCycle/idealEventsPerCycle, 1.0) * 100
	}
}

func (cmc *CustomMetricsCollector) trackBusinessMetrics(m hollow.Metrics) {
	// Simulate tracking business metrics
	eventTypes := []string{"login", "purchase", "view", "click", "search"}

	for _, eventType := range eventTypes {
		count := int64(rand.Intn(m.RecordCount + 1))
		cmc.eventTypeCounts[eventType] += count
	}

	// Track user activity
	for i := 0; i < m.RecordCount; i++ {
		userID := fmt.Sprintf("user_%d", rand.Intn(100))
		cmc.userEventCounts[userID]++
	}
}

func (cmc *CustomMetricsCollector) PrintCustomReport() {
	cmc.mu.RLock()
	defer cmc.mu.RUnlock()

	fmt.Printf("Custom Metrics Report:\n")
	fmt.Printf("  Peak Events/Second:   %d\n", cmc.peakEventsPerSecond)
	fmt.Printf("  Peak Cycles/Second:   %d\n", cmc.peakCyclesPerSecond)
	fmt.Printf("  Efficiency Score:     %.2f%%\n", cmc.efficiencyScore)
	fmt.Printf("\n")

	fmt.Printf("Business Metrics:\n")
	fmt.Printf("  Event Types:\n")
	for eventType, count := range cmc.eventTypeCounts {
		fmt.Printf("    %-10s: %d\n", eventType, count)
	}

	fmt.Printf("  Top 10 Active Users:\n")
	// Sort users by activity
	type userActivity struct {
		userID string
		count  int64
	}

	var users []userActivity
	for userID, count := range cmc.userEventCounts {
		users = append(users, userActivity{userID, count})
	}

	// Simple sort
	for i := 0; i < len(users); i++ {
		for j := i + 1; j < len(users); j++ {
			if users[i].count < users[j].count {
				users[i], users[j] = users[j], users[i]
			}
		}
	}

	for i, user := range users {
		if i >= 10 {
			break
		}
		fmt.Printf("    %s: %d events\n", user.userID, user.count)
	}
}

func main() {
	fmt.Printf("Hollow-Go Metrics Showcase\n")
	fmt.Printf("Demonstrating comprehensive metrics collection\n\n")

	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create blob store
	blob := memblob.New()

	// Create custom metrics collector
	customMetrics := NewCustomMetricsCollector()

	// Create producer with custom metrics
	producer := hollow.NewProducer(
		hollow.WithBlobStager(blob),
		hollow.WithLogger(logger),
		hollow.WithMetricsCollector(customMetrics.Collect),
	)

	// Create consumer
	consumer := hollow.NewConsumer(
		hollow.WithBlobRetriever(blob),
		hollow.WithAnnouncementWatcher(func() (uint64, bool, error) {
			latest, err := blob.Latest(context.Background())
			return latest, latest > 0, err
		}),
		hollow.WithConsumerLogger(logger),
	)

	// Generate varied workload with normal distribution
	fmt.Printf("Starting metrics collection with varied workload...\n")

	// Simulate 10 minutes of activity with 10k events
	totalEvents := 10000
	duration := 10 * time.Minute

	startTime := time.Now()
	eventCount := 0

	for eventCount < totalEvents {
		// Calculate time offset based on normal distribution
		elapsed := time.Since(startTime)

		if elapsed >= duration {
			break
		}

		// Variable batch size based on time (peak in middle)
		normalizedTime := elapsed.Seconds() / duration.Seconds()
		peakMultiplier := math.Exp(-math.Pow((normalizedTime-0.5)*4, 2)) // Gaussian peak at 0.5
		batchSize := int(5 + peakMultiplier*45)                          // 5-50 events per batch

		// Generate batch
		_, err := producer.RunCycle(func(ws hollow.WriteState) error {
			for i := 0; i < batchSize && eventCount < totalEvents; i++ {
				eventKey := fmt.Sprintf("event_%d", eventCount)
				if err := ws.Add(eventKey); err != nil {
					return err
				}
				eventCount++
			}
			return nil
		})

		if err != nil {
			logger.Error("Producer cycle failed", "error", err)
			break
		}

		// Periodically refresh consumer
		if eventCount%100 == 0 {
			if err := consumer.Refresh(); err != nil {
				logger.Error("Consumer refresh failed", "error", err)
			}
		}

		// Variable delay based on distribution
		delay := time.Duration(10+peakMultiplier*40) * time.Millisecond
		time.Sleep(delay)
	}

	fmt.Printf("\nCompleted processing %d events in %v\n", eventCount, time.Since(startTime))

	// Final consumer refresh
	consumer.Refresh()

	// Generate comprehensive reports
	customMetrics.GenerateDetailedReport()
	customMetrics.PrintCustomReport()

	// Test diff functionality
	fmt.Printf("\nTesting diff functionality...\n")
	testDiffMetrics()

	fmt.Printf("\nMetrics showcase completed successfully!\n")
}

func testDiffMetrics() {
	// Create test data for diff metrics
	oldData := map[string]any{
		"users_active":  1000,
		"events_today":  5000,
		"revenue_today": 25000.50,
		"feature_a":     "enabled",
		"feature_b":     "disabled",
	}

	newData := map[string]any{
		"users_active":  1250,
		"events_today":  7500,
		"revenue_today": 31250.75,
		"feature_a":     "enabled",
		"feature_c":     "beta",
	}

	diff := hollow.DiffData(oldData, newData)

	fmt.Printf("Data Diff Analysis:\n")
	fmt.Printf("  Added Fields:     %v\n", diff.GetAdded())
	fmt.Printf("  Removed Fields:   %v\n", diff.GetRemoved())
	fmt.Printf("  Changed Fields:   %v\n", diff.GetChanged())
	fmt.Printf("  Total Changes:    %d\n", len(diff.GetAdded())+len(diff.GetRemoved())+len(diff.GetChanged()))

	// Calculate change impact
	changeImpact := float64(len(diff.GetAdded())+len(diff.GetRemoved())+len(diff.GetChanged())) /
		float64(len(oldData)+len(newData)) * 100
	fmt.Printf("  Change Impact:    %.1f%%\n", changeImpact)
}
