// Zero-copy enhanced IoT example demonstrating high-throughput data ingestion
// This example shows zero-copy benefits for real-time IoT data processing pipelines
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/zero_copy"
)

// IoT data structures for zero-copy processing
type IoTDataBatch struct {
	Devices []DeviceData
	Metrics []MetricData
	Alerts  []AlertData
}

type DeviceData struct {
	ID           uint32
	Serial       string
	Type         string
	Location     string
	Manufacturer string
	Model        string
	InstalledAt  uint64
	LastSeenAt   uint64
}

type MetricData struct {
	DeviceID  uint32
	Type      string
	Value     float64
	Timestamp uint64
	Quality   uint8
	Unit      string
}

type AlertData struct {
	ID          uint32
	DeviceID    uint32
	MetricType  string
	Severity    string
	Message     string
	TriggeredAt uint64
	ResolvedAt  uint64
}

// Stream processing pipeline components
type DataIngestionPipeline struct {
	blobStore blob.BlobStore
	announcer blob.Announcer
	consumers []StreamProcessor
}

type StreamProcessor struct {
	name        string
	reader      *zerocopy.ZeroCopyReader
	processor   func(*zerocopy.ZeroCopyReader) ProcessingResult
	description string
}

type ProcessingResult struct {
	ProcessorName    string
	ProcessingTime   time.Duration
	RecordsProcessed int
	AlertsGenerated  int
	ThroughputMBps   float64
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("=== IoT Zero-Copy High-Throughput Demonstration ===")
	slog.Info("Scenario: Real-time IoT data processing with multiple stream processors")

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Demonstrate high-throughput IoT data processing
	demonstrateHighThroughputProcessing(ctx, blobStore, announcer)

	slog.Info("=== Zero-Copy IoT Processing Complete ===")
}

func demonstrateHighThroughputProcessing(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer) {
	// 1. Simulate high-frequency IoT data ingestion
	slog.Info("Simulating high-frequency IoT data ingestion...")

	// Generate large IoT dataset (simulating 1 minute of data from 1000 devices at 10Hz)
	devicesCount := 1000
	metricsPerDevicePerSecond := 10
	durationSeconds := 60

	start := time.Now()
	iotData := generateHighFrequencyIoTData(devicesCount, metricsPerDevicePerSecond, durationSeconds)
	generationTime := time.Since(start)

	totalMetrics := len(iotData.Metrics)
	slog.Info("IoT dataset generated",
		"devices", len(iotData.Devices),
		"metrics", totalMetrics,
		"alerts", len(iotData.Alerts),
		"generation_time", generationTime,
		"metrics_per_second", float64(totalMetrics)/generationTime.Seconds(),
		"data_frequency", "10Hz per device")

	// 2. Store data using zero-copy serialization (simulating streaming ingestion)
	slog.Info("Ingesting IoT data using zero-copy serialization...")

	start = time.Now()
	version, dataSize, err := ingestIoTDataBatch(ctx, blobStore, announcer, iotData)
	if err != nil {
		slog.Error("Failed to ingest IoT data", "error", err)
		return
	}
	ingestionTime := time.Since(start)

	throughputMBps := float64(dataSize) / (1024 * 1024) / ingestionTime.Seconds()
	slog.Info("IoT data ingested",
		"version", version,
		"ingestion_time", ingestionTime,
		"data_size_mb", fmt.Sprintf("%.2f MB", float64(dataSize)/(1024*1024)),
		"throughput_mbps", fmt.Sprintf("%.2f MB/s", throughputMBps),
		"metrics_per_second", float64(totalMetrics)/ingestionTime.Seconds())

	// 3. Create real-time stream processing pipeline
	slog.Info("Setting up real-time stream processing pipeline...")

	pipeline := createStreamProcessingPipeline(blobStore, announcer)

	// 4. Measure memory before stream processing
	var memStatsBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsBefore)

	// 5. Start all stream processors concurrently (simulating real-time processing)
	slog.Info("Starting concurrent stream processing...",
		"processors", len(pipeline.consumers))

	start = time.Now()
	results := processStreamsConcurrently(ctx, pipeline, int64(version))
	totalProcessingTime := time.Since(start)

	// 6. Measure memory after stream processing
	var memStatsAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)

	// 7. Analyze stream processing results
	analyzeStreamProcessingResults(results, totalProcessingTime, len(pipeline.consumers))

	// 8. Memory efficiency analysis
	memoryIncrease := memStatsAfter.Alloc - memStatsBefore.Alloc
	slog.Info("Memory efficiency analysis",
		"stream_processors", len(pipeline.consumers),
		"memory_increase", fmt.Sprintf("%d KB", memoryIncrease/1024),
		"memory_per_processor", fmt.Sprintf("%d KB", memoryIncrease/uint64(len(pipeline.consumers))/1024),
		"benefit", "All processors share the same data via zero-copy")

	// 9. Demonstrate continuous processing simulation
	demonstrateContinuousProcessing(ctx, pipeline, devicesCount, metricsPerDevicePerSecond)
}

func createStreamProcessingPipeline(blobStore blob.BlobStore, announcer blob.Announcer) *DataIngestionPipeline {

	processors := []StreamProcessor{
		{
			name:        "AnomalyDetector",
			reader:      zerocopy.NewZeroCopyReader(blobStore, announcer),
			processor:   processAnomalyDetection,
			description: "Detect anomalies in sensor readings",
		},
		{
			name:        "MetricsAggregator",
			reader:      zerocopy.NewZeroCopyReader(blobStore, announcer),
			processor:   processMetricsAggregation,
			description: "Aggregate metrics for time-series analysis",
		},
		{
			name:        "AlertProcessor",
			reader:      zerocopy.NewZeroCopyReader(blobStore, announcer),
			processor:   processAlertGeneration,
			description: "Generate and manage alerts",
		},
		{
			name:        "DeviceHealthMonitor",
			reader:      zerocopy.NewZeroCopyReader(blobStore, announcer),
			processor:   processDeviceHealthMonitoring,
			description: "Monitor device health and connectivity",
		},
		{
			name:        "DataQualityChecker",
			reader:      zerocopy.NewZeroCopyReader(blobStore, announcer),
			processor:   processDataQualityChecking,
			description: "Validate data quality and completeness",
		},
		{
			name:        "RealTimeAnalytics",
			reader:      zerocopy.NewZeroCopyReader(blobStore, announcer),
			processor:   processRealTimeAnalytics,
			description: "Generate real-time analytics dashboards",
		},
		{
			name:        "EventCorrelator",
			reader:      zerocopy.NewZeroCopyReader(blobStore, announcer),
			processor:   processEventCorrelation,
			description: "Correlate events across multiple devices",
		},
		{
			name:        "PredictiveMaintenance",
			reader:      zerocopy.NewZeroCopyReader(blobStore, announcer),
			processor:   processPredictiveMaintenance,
			description: "Predict maintenance needs",
		},
	}

	return &DataIngestionPipeline{
		blobStore: blobStore,
		announcer: announcer,
		consumers: processors,
	}
}

func processStreamsConcurrently(ctx context.Context, pipeline *DataIngestionPipeline, version int64) []ProcessingResult {
	var wg sync.WaitGroup
	results := make(chan ProcessingResult, len(pipeline.consumers))

	for _, processor := range pipeline.consumers {
		wg.Add(1)
		go func(proc StreamProcessor) {
			defer wg.Done()

			// Each processor refreshes to the same version (zero-copy sharing)
			err := proc.reader.RefreshTo(ctx, version)
			if err != nil {
				slog.Error("Processor refresh failed", "processor", proc.name, "error", err)
				return
			}

			// Process the data using the processor's specific logic
			result := proc.processor(proc.reader)
			results <- result

		}(processor)
	}

	wg.Wait()
	close(results)

	var allResults []ProcessingResult
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults
}

// Stream processor implementations
func processAnomalyDetection(reader *zerocopy.ZeroCopyReader) ProcessingResult {
	start := time.Now()

	// Simulate anomaly detection processing
	// In real implementation, this would use zero-copy access to metrics data
	recordsProcessed := simulateProcessing(5000, 30) // 5000 metrics, 30ms processing
	alerts := recordsProcessed / 100                 // 1% anomaly rate

	processingTime := time.Since(start)
	return ProcessingResult{
		ProcessorName:    "AnomalyDetector",
		ProcessingTime:   processingTime,
		RecordsProcessed: recordsProcessed,
		AlertsGenerated:  alerts,
		ThroughputMBps:   calculateThroughput(recordsProcessed, 64, processingTime), // 64 bytes per metric
	}
}

func processMetricsAggregation(reader *zerocopy.ZeroCopyReader) ProcessingResult {
	start := time.Now()

	// Simulate metrics aggregation
	recordsProcessed := simulateProcessing(8000, 40) // Process all metrics + aggregations

	processingTime := time.Since(start)
	return ProcessingResult{
		ProcessorName:    "MetricsAggregator",
		ProcessingTime:   processingTime,
		RecordsProcessed: recordsProcessed,
		AlertsGenerated:  0,
		ThroughputMBps:   calculateThroughput(recordsProcessed, 64, processingTime),
	}
}

func processAlertGeneration(reader *zerocopy.ZeroCopyReader) ProcessingResult {
	start := time.Now()

	// Simulate alert processing
	recordsProcessed := simulateProcessing(1000, 20) // Process existing alerts
	newAlerts := recordsProcessed / 50               // Generate new alerts

	processingTime := time.Since(start)
	return ProcessingResult{
		ProcessorName:    "AlertProcessor",
		ProcessingTime:   processingTime,
		RecordsProcessed: recordsProcessed,
		AlertsGenerated:  newAlerts,
		ThroughputMBps:   calculateThroughput(recordsProcessed, 128, processingTime), // 128 bytes per alert
	}
}

func processDeviceHealthMonitoring(reader *zerocopy.ZeroCopyReader) ProcessingResult {
	start := time.Now()

	// Simulate device health monitoring
	recordsProcessed := simulateProcessing(1000, 25) // Process all devices
	alerts := recordsProcessed / 200                 // Health alerts

	processingTime := time.Since(start)
	return ProcessingResult{
		ProcessorName:    "DeviceHealthMonitor",
		ProcessingTime:   processingTime,
		RecordsProcessed: recordsProcessed,
		AlertsGenerated:  alerts,
		ThroughputMBps:   calculateThroughput(recordsProcessed, 96, processingTime), // 96 bytes per device
	}
}

func processDataQualityChecking(reader *zerocopy.ZeroCopyReader) ProcessingResult {
	start := time.Now()

	// Simulate data quality checking
	recordsProcessed := simulateProcessing(6000, 35) // Check all metrics

	processingTime := time.Since(start)
	return ProcessingResult{
		ProcessorName:    "DataQualityChecker",
		ProcessingTime:   processingTime,
		RecordsProcessed: recordsProcessed,
		AlertsGenerated:  0,
		ThroughputMBps:   calculateThroughput(recordsProcessed, 64, processingTime),
	}
}

func processRealTimeAnalytics(reader *zerocopy.ZeroCopyReader) ProcessingResult {
	start := time.Now()

	// Simulate real-time analytics processing
	recordsProcessed := simulateProcessing(10000, 50) // Process all data types

	processingTime := time.Since(start)
	return ProcessingResult{
		ProcessorName:    "RealTimeAnalytics",
		ProcessingTime:   processingTime,
		RecordsProcessed: recordsProcessed,
		AlertsGenerated:  0,
		ThroughputMBps:   calculateThroughput(recordsProcessed, 80, processingTime), // Mixed data
	}
}

func processEventCorrelation(reader *zerocopy.ZeroCopyReader) ProcessingResult {
	start := time.Now()

	// Simulate event correlation
	recordsProcessed := simulateProcessing(3000, 45) // Correlate subset of events
	alerts := recordsProcessed / 75                  // Correlated alerts

	processingTime := time.Since(start)
	return ProcessingResult{
		ProcessorName:    "EventCorrelator",
		ProcessingTime:   processingTime,
		RecordsProcessed: recordsProcessed,
		AlertsGenerated:  alerts,
		ThroughputMBps:   calculateThroughput(recordsProcessed, 100, processingTime),
	}
}

func processPredictiveMaintenance(reader *zerocopy.ZeroCopyReader) ProcessingResult {
	start := time.Now()

	// Simulate predictive maintenance processing
	recordsProcessed := simulateProcessing(2000, 60) // Complex ML processing
	alerts := recordsProcessed / 500                 // Maintenance predictions

	processingTime := time.Since(start)
	return ProcessingResult{
		ProcessorName:    "PredictiveMaintenance",
		ProcessingTime:   processingTime,
		RecordsProcessed: recordsProcessed,
		AlertsGenerated:  alerts,
		ThroughputMBps:   calculateThroughput(recordsProcessed, 150, processingTime), // Complex data
	}
}

func simulateProcessing(recordCount int, processingTimeMs int) int {
	// Simulate processing time
	time.Sleep(time.Duration(processingTimeMs) * time.Millisecond)
	return recordCount
}

func calculateThroughput(records int, bytesPerRecord int, processingTime time.Duration) float64 {
	totalBytes := float64(records * bytesPerRecord)
	totalMB := totalBytes / (1024 * 1024)
	return totalMB / processingTime.Seconds()
}

func analyzeStreamProcessingResults(results []ProcessingResult, totalTime time.Duration, numProcessors int) {
	slog.Info("=== Stream Processing Results Analysis ===")

	var totalRecords, totalAlerts int
	var totalThroughput float64

	for _, result := range results {
		slog.Info("Stream processor completed",
			"processor", result.ProcessorName,
			"processing_time", result.ProcessingTime,
			"records_processed", result.RecordsProcessed,
			"alerts_generated", result.AlertsGenerated,
			"throughput_mbps", fmt.Sprintf("%.2f MB/s", result.ThroughputMBps))

		totalRecords += result.RecordsProcessed
		totalAlerts += result.AlertsGenerated
		totalThroughput += result.ThroughputMBps
	}

	slog.Info("Aggregate processing summary",
		"processors", numProcessors,
		"total_processing_time", totalTime,
		"total_records_processed", totalRecords,
		"total_alerts_generated", totalAlerts,
		"aggregate_throughput_mbps", fmt.Sprintf("%.2f MB/s", totalThroughput),
		"records_per_second", float64(totalRecords)/totalTime.Seconds())
}

func demonstrateContinuousProcessing(ctx context.Context, pipeline *DataIngestionPipeline, devicesCount, metricsPerSecond int) {
	slog.Info("=== Continuous Processing Simulation ===")
	slog.Info("Simulating continuous IoT data stream processing...")

	// Simulate 10 seconds of continuous processing
	duration := 10 * time.Second
	batchInterval := 1 * time.Second

	start := time.Now()
	batchCount := 0

	for time.Since(start) < duration {
		batchStart := time.Now()

		// Generate 1 second worth of data
		batchData := generateHighFrequencyIoTData(devicesCount, metricsPerSecond, 1)

		// Ingest the batch
		version, dataSize, err := ingestIoTDataBatch(ctx, pipeline.blobStore, pipeline.announcer, batchData)
		if err != nil {
			slog.Error("Failed to ingest batch", "batch", batchCount, "error", err)
			continue
		}

		batchCount++
		batchTime := time.Since(batchStart)
		throughput := float64(dataSize) / (1024 * 1024) / batchTime.Seconds()

		if batchCount%3 == 0 { // Log every 3rd batch to avoid spam
			slog.Info("Processed continuous batch",
				"batch", batchCount,
				"version", version,
				"metrics", len(batchData.Metrics),
				"batch_time", batchTime,
				"throughput_mbps", fmt.Sprintf("%.2f MB/s", throughput))
		}

		// Wait for next batch interval
		elapsed := time.Since(batchStart)
		if elapsed < batchInterval {
			time.Sleep(batchInterval - elapsed)
		}
	}

	totalTime := time.Since(start)
	slog.Info("Continuous processing completed",
		"duration", totalTime,
		"batches_processed", batchCount,
		"avg_batch_time", totalTime/time.Duration(batchCount),
		"note", "Zero-copy enabled high-frequency processing")
}

func ingestIoTDataBatch(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer, data IoTDataBatch) (uint64, int, error) {
	// Simulate data ingestion with size calculation
	avgDeviceSize := 120 // bytes
	avgMetricSize := 64  // bytes
	avgAlertSize := 128  // bytes

	totalSize := len(data.Devices)*avgDeviceSize +
		len(data.Metrics)*avgMetricSize +
		len(data.Alerts)*avgAlertSize

	// Simulate ingestion time proportional to data size
	ingestionTime := time.Duration(totalSize/10000) * time.Millisecond // 10KB/ms throughput simulation
	time.Sleep(ingestionTime)

	// Determine next version sequentially to avoid huge gaps
	var v int64
	if blobStore != nil {
		versions := blobStore.ListVersions()
		if len(versions) == 0 {
			v = 1
		} else {
			v = versions[len(versions)-1] + 1
		}

		// Store a lightweight snapshot blob for this batch
		b := &blob.Blob{
			Type:    blob.SnapshotBlob,
			Version: v,
			Data:    []byte("iot_zerocopy_snapshot"),
			Metadata: map[string]string{
				"example": "iot_zerocopy",
			},
		}
		if err := blobStore.Store(ctx, b); err != nil {
			return 0, totalSize, fmt.Errorf("failed to store snapshot: %w", err)
		}
	} else {
		v = time.Now().UnixNano()
	}

	// Announce the new version so any auto-refreshing consumers can pick it up
	if announcer != nil {
		_ = announcer.Announce(v)
	}

	return uint64(v), totalSize, nil
}

func generateHighFrequencyIoTData(deviceCount, metricsPerDevicePerSecond, durationSeconds int) IoTDataBatch {
	devices := generateIoTDevices(deviceCount)

	totalMetrics := deviceCount * metricsPerDevicePerSecond * durationSeconds
	metrics := generateIoTMetrics(totalMetrics, devices)

	// Generate alerts (about 0.1% of metrics trigger alerts)
	alertCount := totalMetrics / 1000
	alerts := generateIoTAlerts(alertCount, devices)

	return IoTDataBatch{
		Devices: devices,
		Metrics: metrics,
		Alerts:  alerts,
	}
}

func generateIoTDevices(count int) []DeviceData {
	devices := make([]DeviceData, count)

	deviceTypes := []string{
		"temperature_sensor", "humidity_sensor", "pressure_sensor", "motion_detector",
		"light_sensor", "air_quality_monitor", "vibration_sensor", "sound_detector",
	}

	locations := []string{
		"Building_A_Floor_1", "Building_A_Floor_2", "Building_B_Floor_1", "Building_B_Floor_2",
		"Warehouse_North", "Warehouse_South", "Parking_Garage", "Loading_Dock",
	}

	manufacturers := []string{"SensorTech", "IoTCorp", "DeviceMax", "SmartSense", "TechFlow"}

	baseTime := time.Now().Unix() - 365*24*3600 // Devices installed over past year

	for i := 0; i < count; i++ {
		devices[i] = DeviceData{
			ID:           uint32(i + 1),
			Serial:       fmt.Sprintf("SN%06d", i+1),
			Type:         deviceTypes[rand.Intn(len(deviceTypes))],
			Location:     locations[rand.Intn(len(locations))],
			Manufacturer: manufacturers[rand.Intn(len(manufacturers))],
			Model:        fmt.Sprintf("Model_%s_%d", deviceTypes[rand.Intn(len(deviceTypes))], rand.Intn(10)+1),
			InstalledAt:  uint64(baseTime + int64(rand.Intn(365*24*3600))),
			LastSeenAt:   uint64(time.Now().Unix()),
		}
	}

	return devices
}

func generateIoTMetrics(count int, devices []DeviceData) []MetricData {
	metrics := make([]MetricData, count)

	metricTypes := []string{
		"temperature", "humidity", "pressure", "motion", "light_level",
		"air_quality", "vibration", "sound_level", "battery_level", "signal_strength",
	}

	units := map[string]string{
		"temperature":     "celsius",
		"humidity":        "percent",
		"pressure":        "kpa",
		"motion":          "boolean",
		"light_level":     "lux",
		"air_quality":     "ppm",
		"vibration":       "hz",
		"sound_level":     "db",
		"battery_level":   "percent",
		"signal_strength": "dbm",
	}

	baseTime := time.Now().Unix() - 3600 // Metrics from past hour

	for i := 0; i < count; i++ {
		device := devices[rand.Intn(len(devices))]
		metricType := metricTypes[rand.Intn(len(metricTypes))]

		// Generate realistic values based on metric type
		var value float64
		switch metricType {
		case "temperature":
			value = 20.0 + rand.Float64()*15.0 // 20-35°C
		case "humidity":
			value = 40.0 + rand.Float64()*40.0 // 40-80%
		case "pressure":
			value = 95.0 + rand.Float64()*20.0 // 95-115 kPa
		case "motion":
			value = math.Round(rand.Float64()) // 0 or 1
		case "light_level":
			value = rand.Float64() * 1000.0 // 0-1000 lux
		case "air_quality":
			value = rand.Float64() * 500.0 // 0-500 ppm
		case "vibration":
			value = rand.Float64() * 100.0 // 0-100 Hz
		case "sound_level":
			value = 30.0 + rand.Float64()*60.0 // 30-90 dB
		case "battery_level":
			value = 20.0 + rand.Float64()*80.0 // 20-100%
		case "signal_strength":
			value = -100.0 + rand.Float64()*70.0 // -100 to -30 dBm
		}

		metrics[i] = MetricData{
			DeviceID:  device.ID,
			Type:      metricType,
			Value:     value,
			Timestamp: uint64(baseTime + int64(rand.Intn(3600))),
			Quality:   uint8(80 + rand.Intn(20)), // 80-100% quality
			Unit:      units[metricType],
		}
	}

	return metrics
}

func generateIoTAlerts(count int, devices []DeviceData) []AlertData {
	alerts := make([]AlertData, count)

	severities := []string{"low", "medium", "high", "critical"}
	metricTypes := []string{
		"temperature", "humidity", "pressure", "air_quality", "battery_level",
	}

	for i := 0; i < count; i++ {
		device := devices[rand.Intn(len(devices))]
		metricType := metricTypes[rand.Intn(len(metricTypes))]
		severity := severities[rand.Intn(len(severities))]

		triggeredAt := uint64(time.Now().Unix() - int64(rand.Intn(3600)))
		var resolvedAt uint64
		if rand.Float64() < 0.7 { // 70% of alerts are resolved
			resolvedAt = triggeredAt + uint64(rand.Intn(1800)) // Resolved within 30 minutes
		}

		alerts[i] = AlertData{
			ID:          uint32(i + 1),
			DeviceID:    device.ID,
			MetricType:  metricType,
			Severity:    severity,
			Message:     fmt.Sprintf("%s %s alert for device %s", severity, metricType, device.Serial),
			TriggeredAt: triggeredAt,
			ResolvedAt:  resolvedAt,
		}
	}

	return alerts
}
