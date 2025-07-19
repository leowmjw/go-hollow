package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leowmjw/go-hollow"
	"github.com/leowmjw/go-hollow/internal/memblob"
)

const (
	TotalEvents     = 10000
	DurationMinutes = 10
	DurationSeconds = DurationMinutes * 60
)

// Event represents a data event in our system
type Event struct {
	ID        int       `json:"id"`
	UserID    string    `json:"user_id"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// EventGenerator generates events with normal distribution timing
type EventGenerator struct {
	events    []Event
	schedule  []time.Duration
	startTime time.Time
}

func NewEventGenerator() *EventGenerator {
	eg := &EventGenerator{
		events:    make([]Event, TotalEvents),
		schedule:  make([]time.Duration, TotalEvents),
		startTime: time.Now(),
	}
	eg.generateEvents()
	eg.generateSchedule()
	return eg
}

func (eg *EventGenerator) generateEvents() {
	eventTypes := []string{"login", "logout", "page_view", "purchase", "signup", "click"}

	for i := 0; i < TotalEvents; i++ {
		eg.events[i] = Event{
			ID:        i + 1,
			UserID:    fmt.Sprintf("user_%d", rand.Intn(1000)+1),
			EventType: eventTypes[rand.Intn(len(eventTypes))],
			Timestamp: time.Now(),
			Data: map[string]any{
				"session_id": fmt.Sprintf("session_%d", rand.Intn(10000)),
				"ip_address": fmt.Sprintf("192.168.%d.%d", rand.Intn(255), rand.Intn(255)),
				"user_agent": "Mozilla/5.0 (compatible; HollowGo/1.0)",
				"value":      rand.Float64() * 1000,
			},
		}
	}
}

func (eg *EventGenerator) generateSchedule() {
	// Generate normal distribution centered at 5 minutes (300 seconds)
	mean := float64(DurationSeconds) / 2.0   // 300 seconds
	stdDev := float64(DurationSeconds) / 6.0 // 100 seconds (99.7% within 10 minutes)

	for i := 0; i < TotalEvents; i++ {
		// Box-Muller transform for normal distribution
		u1 := rand.Float64()
		u2 := rand.Float64()
		z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)

		// Scale and shift to our desired mean and stdDev
		seconds := mean + stdDev*z

		// Clamp to [0, DurationSeconds]
		if seconds < 0 {
			seconds = 0
		} else if seconds > DurationSeconds {
			seconds = DurationSeconds
		}

		eg.schedule[i] = time.Duration(seconds * float64(time.Second))
	}
}

func (eg *EventGenerator) GetScheduledEvents() []ScheduledEvent {
	scheduled := make([]ScheduledEvent, TotalEvents)
	for i := 0; i < TotalEvents; i++ {
		scheduled[i] = ScheduledEvent{
			Event: eg.events[i],
			When:  eg.startTime.Add(eg.schedule[i]),
		}
	}
	return scheduled
}

type ScheduledEvent struct {
	Event Event
	When  time.Time
}

// MetricsCollector collects comprehensive metrics
type MetricsCollector struct {
	mu              sync.RWMutex
	totalCycles     int64
	totalEvents     int64
	totalBytes      int64
	cycleDurations  []time.Duration
	lastCycleTime   time.Time
	eventsPerSecond []int64
	startTime       time.Time
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		cycleDurations:  make([]time.Duration, 0, 1000),
		eventsPerSecond: make([]int64, 0, DurationSeconds),
		startTime:       time.Now(),
		lastCycleTime:   time.Now(),
	}
}

func (mc *MetricsCollector) Collect(m hollow.Metrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	mc.totalCycles++
	mc.totalEvents += int64(m.RecordCount)
	mc.totalBytes += m.ByteSize

	if !mc.lastCycleTime.IsZero() {
		duration := now.Sub(mc.lastCycleTime)
		mc.cycleDurations = append(mc.cycleDurations, duration)
	}
	mc.lastCycleTime = now

	// Track events per second
	second := int64(now.Sub(mc.startTime).Seconds())
	if second >= int64(len(mc.eventsPerSecond)) {
		// Extend slice if needed
		for i := int64(len(mc.eventsPerSecond)); i <= second; i++ {
			mc.eventsPerSecond = append(mc.eventsPerSecond, 0)
		}
	}
	if second >= 0 && second < int64(len(mc.eventsPerSecond)) {
		mc.eventsPerSecond[second] += int64(m.RecordCount)
	}
}

func (mc *MetricsCollector) PrintReport() {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	totalDuration := time.Since(mc.startTime)

	fmt.Print("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("BENCHMARK REPORT\n")
	fmt.Print(strings.Repeat("=", 60) + "\n")
	fmt.Printf("Total Events:     %d\n", mc.totalEvents)
	fmt.Printf("Total Cycles:     %d\n", mc.totalCycles)
	fmt.Printf("Total Duration:   %v\n", totalDuration)
	fmt.Printf("Total Bytes:      %d (%.2f MB)\n", mc.totalBytes, float64(mc.totalBytes)/(1024*1024))
	fmt.Printf("Events/Second:    %.2f\n", float64(mc.totalEvents)/totalDuration.Seconds())
	fmt.Printf("Cycles/Second:    %.2f\n", float64(mc.totalCycles)/totalDuration.Seconds())
	fmt.Printf("Avg Cycle Size:   %.2f events\n", float64(mc.totalEvents)/float64(mc.totalCycles))

	if len(mc.cycleDurations) > 0 {
		var totalCycleDuration time.Duration
		var minDuration, maxDuration time.Duration = time.Hour, 0

		for _, d := range mc.cycleDurations {
			totalCycleDuration += d
			if d < minDuration {
				minDuration = d
			}
			if d > maxDuration {
				maxDuration = d
			}
		}

		avgCycleDuration := totalCycleDuration / time.Duration(len(mc.cycleDurations))
		fmt.Printf("Avg Cycle Time:   %v\n", avgCycleDuration)
		fmt.Printf("Min Cycle Time:   %v\n", minDuration)
		fmt.Printf("Max Cycle Time:   %v\n", maxDuration)
	}

	// Print distribution
	fmt.Printf("\nEvent Distribution (events per 30-second window):\n")
	for i := 0; i < len(mc.eventsPerSecond); i += 30 {
		var windowTotal int64
		for j := i; j < i+30 && j < len(mc.eventsPerSecond); j++ {
			windowTotal += mc.eventsPerSecond[j]
		}
		windowStart := i
		windowEnd := i + 30
		if windowEnd > len(mc.eventsPerSecond) {
			windowEnd = len(mc.eventsPerSecond)
		}
		fmt.Printf("  %3d-%3ds: %4d events ", windowStart, windowEnd, windowTotal)

		// Simple bar chart
		bars := int(windowTotal / 50) // Scale for display
		for k := 0; k < bars && k < 40; k++ {
			fmt.Print("█")
		}
		fmt.Println()
	}

	fmt.Print(strings.Repeat("=", 60) + "\n")
}

// ConsumerTracker tracks consumer performance
type ConsumerTracker struct {
	mu               sync.RWMutex
	refreshCount     int64
	refreshDurations []time.Duration
	readCounts       []int64
	lastRefreshTime  time.Time
}

func NewConsumerTracker() *ConsumerTracker {
	return &ConsumerTracker{
		refreshDurations: make([]time.Duration, 0, 1000),
		readCounts:       make([]int64, 0, 1000),
		lastRefreshTime:  time.Now(),
	}
}

func (ct *ConsumerTracker) TrackRefresh(consumer *hollow.Consumer) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	start := time.Now()
	if err := consumer.Refresh(); err != nil {
		log.Printf("Consumer refresh error: %v", err)
		return
	}

	refreshDuration := time.Since(start)
	ct.refreshCount++
	ct.refreshDurations = append(ct.refreshDurations, refreshDuration)

	// Track read count
	rs := consumer.ReadState()
	ct.readCounts = append(ct.readCounts, int64(rs.Size()))

	ct.lastRefreshTime = time.Now()
}

func (ct *ConsumerTracker) PrintReport() {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	if len(ct.refreshDurations) == 0 {
		fmt.Printf("No consumer refresh data available\n")
		return
	}

	var totalRefreshDuration time.Duration
	var minRefresh, maxRefresh time.Duration = time.Hour, 0

	for _, d := range ct.refreshDurations {
		totalRefreshDuration += d
		if d < minRefresh {
			minRefresh = d
		}
		if d > maxRefresh {
			maxRefresh = d
		}
	}

	avgRefresh := totalRefreshDuration / time.Duration(len(ct.refreshDurations))

	fmt.Printf("Consumer Performance:\n")
	fmt.Printf("  Total Refreshes:    %d\n", ct.refreshCount)
	fmt.Printf("  Avg Refresh Time:   %v\n", avgRefresh)
	fmt.Printf("  Min Refresh Time:   %v\n", minRefresh)
	fmt.Printf("  Max Refresh Time:   %v\n", maxRefresh)

	if len(ct.readCounts) > 0 {
		finalReadCount := ct.readCounts[len(ct.readCounts)-1]
		fmt.Printf("  Final Dataset Size: %d records\n", finalReadCount)
	}
}

func main() {
	fmt.Printf("Hollow-Go Benchmark: %d events over %d minutes\n", TotalEvents, DurationMinutes)
	fmt.Printf("Generating events with normal distribution...\n")

	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create blob store
	blob := memblob.New()

	// Create metrics collector
	metricsCollector := NewMetricsCollector()

	// Create producer
	producer := hollow.NewProducer(
		hollow.WithBlobStager(blob),
		hollow.WithLogger(logger),
		hollow.WithMetricsCollector(metricsCollector.Collect),
	)

	// Create consumer tracker
	consumerTracker := NewConsumerTracker()

	// Create multiple consumers for stress testing
	consumers := make([]*hollow.Consumer, 3)
	for i := 0; i < 3; i++ {
		consumers[i] = hollow.NewConsumer(
			hollow.WithBlobRetriever(blob),
			hollow.WithAnnouncementWatcher(func() (uint64, bool, error) {
				latest, err := blob.Latest(context.Background())
				return latest, latest > 0, err
			}),
			hollow.WithConsumerLogger(logger),
		)
	}

	// Generate event schedule
	generator := NewEventGenerator()
	scheduledEvents := generator.GetScheduledEvents()

	fmt.Printf("Starting benchmark at %v\n", time.Now())
	fmt.Printf("Events scheduled from %v to %v\n",
		scheduledEvents[0].When,
		scheduledEvents[len(scheduledEvents)-1].When)

	// Concurrent event processing
	var wg sync.WaitGroup
	eventChan := make(chan ScheduledEvent, 100)

	// Start event scheduler
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(eventChan)

		for _, scheduledEvent := range scheduledEvents {
			sleepDuration := time.Until(scheduledEvent.When)
			if sleepDuration > 0 {
				time.Sleep(sleepDuration)
			}
			eventChan <- scheduledEvent
		}
	}()

	// Start event processor
	wg.Add(1)
	go func() {
		defer wg.Done()

		batchSize := 10
		batch := make([]Event, 0, batchSize)
		batchTimer := time.NewTimer(100 * time.Millisecond)

		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					// Process final batch
					if len(batch) > 0 {
						processBatch(producer, batch)
					}
					return
				}

				batch = append(batch, event.Event)

				if len(batch) >= batchSize {
					processBatch(producer, batch)
					batch = batch[:0]
					batchTimer.Reset(100 * time.Millisecond)
				}

			case <-batchTimer.C:
				if len(batch) > 0 {
					processBatch(producer, batch)
					batch = batch[:0]
				}
				batchTimer.Reset(100 * time.Millisecond)
			}
		}
	}()

	// Start consumer refresh routine
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				for _, consumer := range consumers {
					consumerTracker.TrackRefresh(consumer)
				}
			case <-time.After(time.Duration(DurationSeconds+30) * time.Second):
				return
			}
		}
	}()

	// Start progress reporter
	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		processed := int64(0)

		for {
			select {
			case <-ticker.C:
				currentProcessed := atomic.LoadInt64(&metricsCollector.totalEvents)
				newEvents := currentProcessed - processed
				processed = currentProcessed

				elapsed := time.Since(metricsCollector.startTime)
				fmt.Printf("Progress: %d/%d events (%.1f%%) - %d events in last 30s - Elapsed: %v\n",
					processed, TotalEvents,
					float64(processed)/float64(TotalEvents)*100,
					newEvents, elapsed)

			case <-time.After(time.Duration(DurationSeconds+60) * time.Second):
				return
			}
		}
	}()

	// Wait for completion
	wg.Wait()

	// Final consumer refresh
	for _, consumer := range consumers {
		consumerTracker.TrackRefresh(consumer)
	}

	fmt.Printf("\nBenchmark completed at %v\n", time.Now())

	// Print comprehensive reports
	metricsCollector.PrintReport()
	consumerTracker.PrintReport()

	// Test diff engine with final state
	fmt.Printf("\nTesting diff engine...\n")
	testDiffEngine(consumers[0])
}

func processBatch(producer *hollow.Producer, batch []Event) {
	_, err := producer.RunCycle(func(ws hollow.WriteState) error {
		for _, event := range batch {
			key := fmt.Sprintf("event_%d", event.ID)
			if err := ws.Add(key); err != nil {
				return fmt.Errorf("failed to add event %d: %w", event.ID, err)
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Batch processing error: %v", err)
	}
}

func testDiffEngine(consumer *hollow.Consumer) {
	// Create two different datasets for diff testing
	oldData := map[string]any{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	newData := map[string]any{
		"key1": "modified_value1",
		"key3": "value3",
		"key4": "value4",
	}

	diff := hollow.DiffData(oldData, newData)

	fmt.Printf("Diff Engine Test:\n")
	fmt.Printf("  Added: %v\n", diff.GetAdded())
	fmt.Printf("  Removed: %v\n", diff.GetRemoved())
	fmt.Printf("  Changed: %v\n", diff.GetChanged())
	fmt.Printf("  Is Empty: %v\n", diff.IsEmpty())

	// Test diff application
	targetData := make(map[string]any)
	for k, v := range oldData {
		targetData[k] = v
	}

	diff.Apply(targetData, newData)
	fmt.Printf("  Applied diff successfully\n")
}
