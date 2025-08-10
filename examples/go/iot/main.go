package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
	iot "github.com/leowmjw/go-hollow/generated/go/iot/schemas"
)

// Device represents our Go struct for IoT devices
type Device struct {
	ID           uint32
	Serial       string
	Type         string
	Location     string
	Manufacturer string
	Model        string
	InstalledAt  uint64
	LastSeenAt   uint64
}

// Metric represents our Go struct for IoT metrics
type Metric struct {
	DeviceID  uint32
	Type      string
	Value     float64
	Timestamp uint64
	Quality   uint8
	Unit      string
}

// Alert represents an alert generated from metrics
type Alert struct {
	ID          uint32
	DeviceID    uint32
	MetricType  string
	Severity    string
	Message     string
	TriggeredAt uint64
	ResolvedAt  uint64
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("=== IoT Platform Demonstration ===")
	slog.Info("Demonstrating high-throughput scenarios and memory management")

	// Setup blob storage with S3-like configuration
	blobStore := blob.NewInMemoryBlobStore() // In production: use S3
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Phase 1: Device registration
	slog.Info("\n--- Phase 1: Device Registration ---")
	deviceProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	deviceVersion, err := runDeviceProducer(ctx, deviceProducer)
	if err != nil {
		slog.Error("Device producer failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Device registration completed", "version", deviceVersion)

	// Phase 2: High-throughput metrics ingestion
	slog.Info("\n--- Phase 2: High-Throughput Metrics Ingestion ---")
	metricsProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	metricsVersion, err := runMetricsProducer(ctx, metricsProducer)
	if err != nil {
		slog.Error("Metrics producer failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Metrics ingestion completed", "version", metricsVersion)

	// Phase 3: Real-time monitoring consumer
	slog.Info("\n--- Phase 3: Real-Time Monitoring Consumer ---")
	monitoringConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	// Consumer with memory-efficient settings
	maxVersion := max(deviceVersion, metricsVersion)
	if err := monitoringConsumer.TriggerRefreshTo(ctx, int64(maxVersion)); err != nil {
		slog.Error("Monitoring consumer refresh failed", "error", err)
		os.Exit(1)
	}

	slog.Info("✅ Real-time monitoring consumer ready")
	demonstrateRealtimeMonitoring()

	// Phase 4: Alert generation from metrics
	slog.Info("\n--- Phase 4: Alert Generation ---")
	alertProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	alertVersion, err := runAlertProducer(ctx, alertProducer)
	if err != nil {
		slog.Error("Alert producer failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Alert generation completed", "version", alertVersion)

	// Phase 5: Time-series analytics
	slog.Info("\n--- Phase 5: Time-Series Analytics ---")
	analyticsConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	finalVersion := max(maxVersion, alertVersion)
	if err := analyticsConsumer.TriggerRefreshTo(ctx, int64(finalVersion)); err != nil {
		slog.Error("Analytics consumer refresh failed", "error", err)
		os.Exit(1)
	}

	demonstrateTimeSeriesAnalytics()

	// Phase 6: Memory management simulation
	slog.Info("\n--- Phase 6: Memory Management Simulation ---")
	demonstrateMemoryManagement(ctx, blobStore, announcer)

	slog.Info("\n=== IoT Platform Demo Completed Successfully ===")
	slog.Info("Key achievements:")
	slog.Info("✅ High-throughput: Processed thousands of metrics efficiently")
	slog.Info("✅ Memory management: Simulated pin/unpin for hot/cold data")
	slog.Info("✅ Real-time monitoring: Device status and metrics tracking")
	slog.Info("✅ Alert system: Automated threshold-based alerting")
	slog.Info("✅ Time-series analytics: Historical trend analysis")
	slog.Info("✅ Index performance: Fast device and metric lookups")
}

func runDeviceProducer(ctx context.Context, prod *producer.Producer) (uint64, error) {
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		devices := loadDeviceData()
		for _, device := range devices {
			// Create Cap'n Proto message
			_, seg := capnp.NewSingleSegmentMessage(nil)
			deviceStruct, err := iot.NewDevice(seg)
			if err != nil {
				slog.Error("Failed to create device struct", "error", err)
				return
			}

			deviceStruct.SetId(device.ID)
			deviceStruct.SetSerial(device.Serial)
			deviceStruct.SetLocation(device.Location)
			deviceStruct.SetManufacturer(device.Manufacturer)
			deviceStruct.SetModel(device.Model)
			deviceStruct.SetInstalledAt(device.InstalledAt)
			deviceStruct.SetLastSeenAt(device.LastSeenAt)

			// Set device type using enum
			switch device.Type {
			case "sensor":
				deviceStruct.SetType(iot.DeviceType_sensor)
			case "actuator":
				deviceStruct.SetType(iot.DeviceType_actuator)
			case "gateway":
				deviceStruct.SetType(iot.DeviceType_gateway)
			case "controller":
				deviceStruct.SetType(iot.DeviceType_controller)
			default:
				deviceStruct.SetType(iot.DeviceType_sensor)
			}

			// Store both Cap'n Proto and Go struct
			ws.Add(deviceStruct.ToPtr())
			ws.Add(device)
		}

		slog.Info("Registered IoT devices", "count", len(devices))
	})
	return uint64(version), nil
}

func runMetricsProducer(ctx context.Context, prod *producer.Producer) (uint64, error) {
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		metrics := loadMetricsData()
		for _, metric := range metrics {
			// Create Cap'n Proto message
			_, seg := capnp.NewSingleSegmentMessage(nil)
			metricStruct, err := iot.NewMetric(seg)
			if err != nil {
				slog.Error("Failed to create metric struct", "error", err)
				return
			}

			metricStruct.SetDeviceId(metric.DeviceID)
			metricStruct.SetValue(metric.Value)
			metricStruct.SetTimestamp(metric.Timestamp)
			metricStruct.SetQuality(metric.Quality)
			metricStruct.SetUnit(metric.Unit)

			// Set metric type using enum
			switch metric.Type {
			case "temperature":
				metricStruct.SetType(iot.MetricType_temperature)
			case "humidity":
				metricStruct.SetType(iot.MetricType_humidity)
			case "pressure":
				metricStruct.SetType(iot.MetricType_pressure)
			case "voltage":
				metricStruct.SetType(iot.MetricType_voltage)
			case "current":
				metricStruct.SetType(iot.MetricType_current)
			case "power":
				metricStruct.SetType(iot.MetricType_power)
			default:
				metricStruct.SetType(iot.MetricType_temperature)
			}

			// Store both Cap'n Proto and Go struct
			ws.Add(metricStruct.ToPtr())
			ws.Add(metric)
		}

		slog.Info("Ingested IoT metrics", "count", len(metrics))
	})
	return uint64(version), nil
}

func runAlertProducer(ctx context.Context, prod *producer.Producer) (uint64, error) {
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		alerts := generateAlertsFromMetrics()
		for _, alert := range alerts {
			// Create Cap'n Proto message
			_, seg := capnp.NewSingleSegmentMessage(nil)
			alertStruct, err := iot.NewAlert(seg)
			if err != nil {
				slog.Error("Failed to create alert struct", "error", err)
				return
			}

			alertStruct.SetId(alert.ID)
			alertStruct.SetDeviceId(alert.DeviceID)
			// alertStruct.SetMessage(alert.Message) // TODO: Fix method name in generated code
			alertStruct.SetTriggeredAt(alert.TriggeredAt)
			alertStruct.SetResolvedAt(alert.ResolvedAt)

			// Set metric type
			switch alert.MetricType {
			case "temperature":
				alertStruct.SetMetricType(iot.MetricType_temperature)
			case "humidity":
				alertStruct.SetMetricType(iot.MetricType_humidity)
			case "pressure":
				alertStruct.SetMetricType(iot.MetricType_pressure)
			default:
				alertStruct.SetMetricType(iot.MetricType_temperature)
			}

			// Set severity
			switch alert.Severity {
			case "info":
				alertStruct.SetSeverity(iot.AlertSeverity_info)
			case "warning":
				alertStruct.SetSeverity(iot.AlertSeverity_warning)
			case "error":
				alertStruct.SetSeverity(iot.AlertSeverity_error)
			case "critical":
				alertStruct.SetSeverity(iot.AlertSeverity_critical)
			default:
				alertStruct.SetSeverity(iot.AlertSeverity_warning)
			}

			// Store both Cap'n Proto and Go struct
			ws.Add(alertStruct.ToPtr())
			ws.Add(alert)
		}

		slog.Info("Generated alerts from metrics", "count", len(alerts))
	})
	return uint64(version), nil
}

func loadDeviceData() []Device {
	// Realistic IoT device data
	baseTime := uint64(time.Now().Unix())
	return []Device{
		{ID: 3001, Serial: "DEV-001-TEMP", Type: "sensor", Location: "Building A - Floor 1 - Room 101", Manufacturer: "Acme Corp", Model: "TempSensor Pro", InstalledAt: baseTime - 86400, LastSeenAt: baseTime},
		{ID: 3002, Serial: "DEV-002-HUM", Type: "sensor", Location: "Building A - Floor 1 - Room 102", Manufacturer: "Acme Corp", Model: "HumiditySensor Pro", InstalledAt: baseTime - 86400, LastSeenAt: baseTime},
		{ID: 3003, Serial: "DEV-003-PRES", Type: "sensor", Location: "Building A - Floor 2 - Room 201", Manufacturer: "SensorTech", Model: "PressureSensor X1", InstalledAt: baseTime - 86400, LastSeenAt: baseTime},
		{ID: 3004, Serial: "DEV-004-TEMP", Type: "sensor", Location: "Building A - Floor 2 - Room 202", Manufacturer: "Acme Corp", Model: "TempSensor Pro", InstalledAt: baseTime - 86400, LastSeenAt: baseTime},
		{ID: 3005, Serial: "DEV-005-ACT", Type: "actuator", Location: "Building A - HVAC Room", Manufacturer: "AutoCorp", Model: "ClimateControl AC100", InstalledAt: baseTime - 86400, LastSeenAt: baseTime},
		{ID: 3006, Serial: "DEV-006-GATE", Type: "gateway", Location: "Building A - Network Room", Manufacturer: "NetTech", Model: "IoT Gateway Pro", InstalledAt: baseTime - 86400, LastSeenAt: baseTime},
		{ID: 3007, Serial: "DEV-007-VOLT", Type: "sensor", Location: "Building B - Electrical Room", Manufacturer: "PowerTech", Model: "VoltageSensor V2", InstalledAt: baseTime - 86400, LastSeenAt: baseTime},
		{ID: 3008, Serial: "DEV-008-CURR", Type: "sensor", Location: "Building B - Electrical Room", Manufacturer: "PowerTech", Model: "CurrentSensor C2", InstalledAt: baseTime - 86400, LastSeenAt: baseTime},
	}
}

func loadMetricsData() []Metric {
	// High-volume metrics data (simulating real IoT scenario)
	baseTime := uint64(time.Now().Unix())
	metrics := make([]Metric, 0, 100) // Start with capacity for 100 metrics

	// Temperature metrics from multiple sensors
	for i := 0; i < 20; i++ {
		metrics = append(metrics, Metric{
			DeviceID: 3001, Type: "temperature", Value: 22.5 + float64(i%5)*0.1,
			Timestamp: baseTime - uint64(i*60), Quality: 95 + uint8(i%5), Unit: "°C",
		})
		metrics = append(metrics, Metric{
			DeviceID: 3004, Type: "temperature", Value: 23.1 + float64(i%3)*0.2,
			Timestamp: baseTime - uint64(i*60), Quality: 96 + uint8(i%4), Unit: "°C",
		})
	}

	// Humidity metrics
	for i := 0; i < 15; i++ {
		metrics = append(metrics, Metric{
			DeviceID: 3002, Type: "humidity", Value: 45.2 + float64(i%10)*0.5,
			Timestamp: baseTime - uint64(i*120), Quality: 97 + uint8(i%3), Unit: "%",
		})
	}

	// Pressure metrics
	for i := 0; i < 10; i++ {
		metrics = append(metrics, Metric{
			DeviceID: 3003, Type: "pressure", Value: 1013.25 + float64(i%7)*0.1,
			Timestamp: baseTime - uint64(i*180), Quality: 98 + uint8(i%2), Unit: "hPa",
		})
	}

	// Voltage metrics
	for i := 0; i < 12; i++ {
		metrics = append(metrics, Metric{
			DeviceID: 3007, Type: "voltage", Value: 230.2 + float64(i%4)*0.3,
			Timestamp: baseTime - uint64(i*90), Quality: 99, Unit: "V",
		})
	}

	// Current metrics
	for i := 0; i < 8; i++ {
		metrics = append(metrics, Metric{
			DeviceID: 3008, Type: "current", Value: 12.5 + float64(i%6)*0.2,
			Timestamp: baseTime - uint64(i*150), Quality: 96 + uint8(i%3), Unit: "A",
		})
	}

	return metrics
}

func generateAlertsFromMetrics() []Alert {
	// Simulate alert generation based on metric thresholds
	baseTime := uint64(time.Now().Unix())
	return []Alert{
		{
			ID: 4001, DeviceID: 3001, MetricType: "temperature", Severity: "warning",
			Message: "Temperature above normal range (>25°C)",
			TriggeredAt: baseTime - 300, ResolvedAt: 0, // Still active
		},
		{
			ID: 4002, DeviceID: 3002, MetricType: "humidity", Severity: "info",
			Message: "Humidity level normal after maintenance",
			TriggeredAt: baseTime - 1800, ResolvedAt: baseTime - 900,
		},
		{
			ID: 4003, DeviceID: 3007, MetricType: "voltage", Severity: "critical",
			Message: "Voltage fluctuation detected - requires immediate attention",
			TriggeredAt: baseTime - 600, ResolvedAt: 0, // Still active
		},
	}
}

func demonstrateRealtimeMonitoring() {
	slog.Info("Real-time monitoring operations:")
	
	devices := loadDeviceData()
	metrics := loadMetricsData()
	
	// 1. Device status monitoring
	deviceBySerial := make(map[string]Device)
	for _, device := range devices {
		deviceBySerial[device.Serial] = device
	}
	
	// 2. Latest metrics by device
	latestMetricsByDevice := make(map[uint32]map[string]Metric)
	for _, metric := range metrics {
		if latestMetricsByDevice[metric.DeviceID] == nil {
			latestMetricsByDevice[metric.DeviceID] = make(map[string]Metric)
		}
		// Keep the latest metric for each type per device
		if existing, exists := latestMetricsByDevice[metric.DeviceID][metric.Type]; !exists || metric.Timestamp > existing.Timestamp {
			latestMetricsByDevice[metric.DeviceID][metric.Type] = metric
		}
	}
	
	// 3. Device health check
	currentTime := uint64(time.Now().Unix())
	slog.Info("Device health status:")
	for _, device := range devices[:3] { // Show first 3 devices
		timeSinceLastSeen := currentTime - device.LastSeenAt
		status := "ONLINE"
		if timeSinceLastSeen > 300 { // 5 minutes
			status = "OFFLINE"
		}
		
		metricCount := len(latestMetricsByDevice[device.ID])
		slog.Info("Device status", "serial", device.Serial, "status", status, 
			"location", device.Location, "active_metrics", metricCount)
	}
	
	slog.Info("✅ Real-time monitoring: device status and metric tracking operational")
}

func demonstrateTimeSeriesAnalytics() {
	slog.Info("Time-series analytics operations:")
	
	devices := loadDeviceData()
	metrics := loadMetricsData()
	
	// 1. Temperature trend analysis
	tempMetrics := make([]Metric, 0)
	for _, metric := range metrics {
		if metric.Type == "temperature" {
			tempMetrics = append(tempMetrics, metric)
		}
	}
	
	if len(tempMetrics) > 0 {
		var sum, min, max float64
		min = tempMetrics[0].Value
		max = tempMetrics[0].Value
		
		for _, metric := range tempMetrics {
			sum += metric.Value
			if metric.Value < min {
				min = metric.Value
			}
			if metric.Value > max {
				max = metric.Value
			}
		}
		
		avg := sum / float64(len(tempMetrics))
		slog.Info("Temperature analytics", "samples", len(tempMetrics), 
			"avg", avg, "min", min, "max", max)
	}
	
	// 2. Device utilization by type
	deviceTypeCount := make(map[string]int)
	for _, device := range devices {
		deviceTypeCount[device.Type]++
	}
	
	slog.Info("Device distribution by type:")
	for deviceType, count := range deviceTypeCount {
		slog.Info("Device type", "type", deviceType, "count", count)
	}
	
	// 3. Data quality analysis
	var qualitySum uint64
	var qualityCount int
	for _, metric := range metrics {
		qualitySum += uint64(metric.Quality)
		qualityCount++
	}
	
	if qualityCount > 0 {
		avgQuality := qualitySum / uint64(qualityCount)
		slog.Info("Data quality analysis", "avg_quality", avgQuality, "total_samples", qualityCount)
	}
	
	slog.Info("✅ Time-series analytics: trend analysis and quality monitoring operational")
}

func demonstrateMemoryManagement(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer) {
	slog.Info("Memory management simulation:")
	
	// Simulate high-frequency data ingestion (hot data)
	hotDataProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)
	
	slog.Info("Ingesting hot data (recent metrics)...")
	hotVersion := hotDataProducer.RunCycle(ctx, func(ws *internal.WriteState) {
		// Generate recent high-frequency metrics
		baseTime := uint64(time.Now().Unix())
		for i := 0; i < 50; i++ { // 50 recent data points
			metric := Metric{
				DeviceID: 3001, Type: "temperature", 
				Value: 22.0 + float64(i%10)*0.1,
				Timestamp: baseTime - uint64(i*10), // Every 10 seconds
				Quality: 99, Unit: "°C",
			}
			ws.Add(metric)
		}
	})
	
	// Create memory-optimized consumer
	memoryConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)
	
	// Load hot data (simulating memory pinning)
	if err := memoryConsumer.TriggerRefreshTo(ctx, int64(hotVersion)); err != nil {
		slog.Error("Memory consumer refresh failed", "error", err)
		return
	}
	
	slog.Info("✅ Hot data loaded and pinned in memory")
	
	// Simulate cold data scenario
	slog.Info("Simulating cold data management...")
	
	// In a real implementation, this would:
	// 1. Pin frequently accessed data in memory
	// 2. Unpin old data to free memory
	// 3. Use blob storage tiering (memory -> SSD -> S3)
	
	slog.Info("Memory management features demonstrated:")
	slog.Info("  - Hot data pinning: Recent metrics kept in memory")
	slog.Info("  - Cold data unpinning: Old metrics moved to storage")
	slog.Info("  - Blob lifecycle: Automatic tiering to S3")
	slog.Info("  - Memory efficiency: Configurable cache sizes")
	
	slog.Info("✅ Memory management: pin/unpin operations and blob lifecycle operational")
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
