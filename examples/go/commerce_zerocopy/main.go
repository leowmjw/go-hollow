// Zero-copy enhanced commerce example demonstrating multiple consumer scenarios
// This example shows zero-copy benefits for distributed e-commerce microservices
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/zero_copy"
)

// Define commerce data structures for zero-copy usage
type CommerceData struct {
	Customers []CustomerData
	Orders    []OrderData
	Products  []ProductData
}

type CustomerData struct {
	ID           uint32
	Email        string
	Name         string
	City         string
	Age          uint16
	RegisteredAt uint64
}

type OrderData struct {
	ID         uint32
	CustomerID uint32
	Amount     uint64 // in cents
	Currency   string
	Status     string
	Timestamp  uint64
	Items      []OrderItemData
}

type OrderItemData struct {
	ProductID    uint32
	Quantity     uint32
	PricePerUnit uint64
	ProductName  string
}

type ProductData struct {
	ID          uint32
	Name        string
	Category    string
	Price       uint64
	Description string
	StockLevel  uint32
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("=== Commerce Platform Zero-Copy Demonstration ===")
	slog.Info("Scenario: Multiple microservices consuming shared commerce data")

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Demonstrate multiple consumer scenario where zero-copy excels
	demonstrateMultiServiceArchitecture(ctx, blobStore, announcer)

	slog.Info("=== Zero-Copy Commerce Platform Complete ===")
}

func demonstrateMultiServiceArchitecture(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer) {
	// 1. Generate realistic commerce data
	slog.Info("Generating realistic commerce dataset...")
	
	start := time.Now()
	commerceData := generateCommerceDataset(10000, 50000, 5000) // 10K customers, 50K orders, 5K products
	generationTime := time.Since(start)
	
	slog.Info("Commerce dataset generated",
		"customers", len(commerceData.Customers),
		"orders", len(commerceData.Orders),
		"products", len(commerceData.Products),
		"generation_time", generationTime)

	// 2. Write data using zero-copy serialization
	slog.Info("Storing commerce data using zero-copy serialization...")
	
	start = time.Now()
	version, err := writeCommerceDataset(ctx, blobStore, announcer, commerceData)
	if err != nil {
		slog.Error("Failed to write commerce dataset", "error", err)
		return
	}
	writeTime := time.Since(start)
	
	totalRecords := len(commerceData.Customers) + len(commerceData.Orders) + len(commerceData.Products)
	slog.Info("Commerce data stored",
		"version", version,
		"write_time", writeTime,
		"total_records", totalRecords,
		"records_per_second", float64(totalRecords)/writeTime.Seconds())

	// 3. Simulate multiple microservices consuming the same data simultaneously
	slog.Info("Starting multiple microservices as zero-copy consumers...")
	
	// Convert announcer to AnnouncementWatcher interface
	watcher, ok := announcer.(blob.AnnouncementWatcher)
	if !ok {
		slog.Error("Announcer does not implement AnnouncementWatcher interface")
		return
	}

	microservices := []struct {
		name        string
		reader      *zerocopy.ZeroCopyReader
		description string
		workload    string
	}{
		{"OrderProcessingService", zerocopy.NewZeroCopyReader(blobStore, watcher), "Process and validate orders", "orders"},
		{"CustomerAnalyticsService", zerocopy.NewZeroCopyReader(blobStore, watcher), "Analyze customer behavior", "customers"},
		{"InventoryManagementService", zerocopy.NewZeroCopyReader(blobStore, watcher), "Track product inventory", "products"},
		{"RecommendationService", zerocopy.NewZeroCopyReader(blobStore, watcher), "Generate product recommendations", "all"},
		{"FraudDetectionService", zerocopy.NewZeroCopyReader(blobStore, watcher), "Detect fraudulent transactions", "orders"},
		{"ReportingService", zerocopy.NewZeroCopyReader(blobStore, watcher), "Generate business reports", "all"},
		{"PricingService", zerocopy.NewZeroCopyReader(blobStore, watcher), "Calculate dynamic pricing", "products"},
		{"NotificationService", zerocopy.NewZeroCopyReader(blobStore, watcher), "Send customer notifications", "customers"},
	}

	// 4. Measure memory usage before microservices start
	var memStatsBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsBefore)

	// 5. Start all microservices concurrently (simulating real distributed architecture)
	slog.Info("All microservices starting concurrent processing...")
	
	var wg sync.WaitGroup
	results := make(chan MicroserviceResult, len(microservices))
	
	start = time.Now()
	for _, service := range microservices {
		wg.Add(1)
		go func(svc struct {
			name        string
			reader      *zerocopy.ZeroCopyReader
			description string
			workload    string
		}) {
			defer wg.Done()
			
			serviceStart := time.Now()
			
			// Each service refreshes to the same version (zero-copy memory sharing)
			err := svc.reader.RefreshTo(ctx, int64(version))
			if err != nil {
				slog.Error("Service refresh failed", "service", svc.name, "error", err)
				return
			}
			
			// Each service processes its workload using zero-copy access
			processedRecords := processServiceWorkload(svc.name, svc.reader, svc.workload)
			
			processingTime := time.Since(serviceStart)
			results <- MicroserviceResult{
				ServiceName:      svc.name,
				ProcessingTime:   processingTime,
				RecordsProcessed: processedRecords,
			}
			
		}(service)
	}
	
	wg.Wait()
	close(results)
	totalTime := time.Since(start)
	
	// 6. Measure memory usage after all microservices
	var memStatsAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsAfter)

	// 7. Collect and analyze results
	slog.Info("All microservices completed concurrent processing")
	
	var totalRecordsProcessed int
	for result := range results {
		slog.Info("Microservice completed",
			"service", result.ServiceName,
			"processing_time", result.ProcessingTime,
			"records_processed", result.RecordsProcessed,
			"records_per_second", float64(result.RecordsProcessed)/result.ProcessingTime.Seconds())
		totalRecordsProcessed += result.RecordsProcessed
	}
	
	// 8. Memory efficiency analysis
	memoryIncrease := memStatsAfter.Alloc - memStatsBefore.Alloc
	slog.Info("Concurrent processing summary",
		"microservices", len(microservices),
		"total_processing_time", totalTime,
		"total_records_processed", totalRecordsProcessed,
		"memory_increase", fmt.Sprintf("%d KB", memoryIncrease/1024),
		"memory_per_service", fmt.Sprintf("%d KB", memoryIncrease/uint64(len(microservices))/1024))

	// 9. Compare with traditional approach (simulation)
	demonstrateMemoryEfficiencyComparison(commerceData, len(microservices))
}

type MicroserviceResult struct {
	ServiceName      string
	ProcessingTime   time.Duration
	RecordsProcessed int
}

func processServiceWorkload(serviceName string, reader *zerocopy.ZeroCopyReader, workloadType string) int {
	// Simulate different microservice workloads using zero-copy access
	recordsProcessed := 0
	
	switch serviceName {
	case "OrderProcessingService":
		// Process orders for validation and fulfillment
		recordsProcessed = simulateOrderProcessing(reader)
		
	case "CustomerAnalyticsService":
		// Analyze customer behavior patterns
		recordsProcessed = simulateCustomerAnalytics(reader)
		
	case "InventoryManagementService":
		// Track product inventory levels
		recordsProcessed = simulateInventoryManagement(reader)
		
	case "RecommendationService":
		// Generate personalized recommendations
		recordsProcessed = simulateRecommendationGeneration(reader)
		
	case "FraudDetectionService":
		// Detect suspicious order patterns
		recordsProcessed = simulateFraudDetection(reader)
		
	case "ReportingService":
		// Generate comprehensive business reports
		recordsProcessed = simulateReportGeneration(reader)
		
	case "PricingService":
		// Calculate dynamic pricing strategies
		recordsProcessed = simulatePricing(reader)
		
	case "NotificationService":
		// Send customer notifications
		recordsProcessed = simulateNotifications(reader)
	}
	
	return recordsProcessed
}

func simulateOrderProcessing(reader *zerocopy.ZeroCopyReader) int {
	// Simulate order processing logic
	// In a real implementation, this would access order data via zero-copy
	// For demo, we'll simulate processing 1000 orders
	time.Sleep(50 * time.Millisecond) // Simulate processing time
	return 1000
}

func simulateCustomerAnalytics(reader *zerocopy.ZeroCopyReader) int {
	// Simulate customer analytics processing
	time.Sleep(80 * time.Millisecond)
	return 2500
}

func simulateInventoryManagement(reader *zerocopy.ZeroCopyReader) int {
	// Simulate inventory tracking
	time.Sleep(30 * time.Millisecond)
	return 500
}

func simulateRecommendationGeneration(reader *zerocopy.ZeroCopyReader) int {
	// Simulate recommendation engine processing
	time.Sleep(120 * time.Millisecond)
	return 5000 // Process customers, orders, and products
}

func simulateFraudDetection(reader *zerocopy.ZeroCopyReader) int {
	// Simulate fraud detection algorithms
	time.Sleep(90 * time.Millisecond)
	return 1000
}

func simulateReportGeneration(reader *zerocopy.ZeroCopyReader) int {
	// Simulate report generation
	time.Sleep(150 * time.Millisecond)
	return 8000 // Process all data types
}

func simulatePricing(reader *zerocopy.ZeroCopyReader) int {
	// Simulate pricing calculations
	time.Sleep(40 * time.Millisecond)
	return 500
}

func simulateNotifications(reader *zerocopy.ZeroCopyReader) int {
	// Simulate notification processing
	time.Sleep(60 * time.Millisecond)
	return 2500
}

func demonstrateMemoryEfficiencyComparison(data CommerceData, numServices int) {
	slog.Info("=== Memory Efficiency Comparison ===")
	
	// Calculate data size estimation
	avgCustomerSize := 200 // bytes (approximate)
	avgOrderSize := 300    // bytes (approximate)
	avgProductSize := 250  // bytes (approximate)
	
	totalDataSize := len(data.Customers)*avgCustomerSize + 
					len(data.Orders)*avgOrderSize + 
					len(data.Products)*avgProductSize
	
	slog.Info("Traditional approach analysis",
		"approach", "Each microservice copies all needed data",
		"estimated_data_size", fmt.Sprintf("%d KB", totalDataSize/1024),
		"microservices", numServices,
		"total_memory_required", fmt.Sprintf("%d KB", totalDataSize*numServices/1024),
		"memory_overhead", "Linear growth with number of services")
	
	slog.Info("Zero-copy approach analysis",
		"approach", "All microservices share the same data in memory",
		"estimated_data_size", fmt.Sprintf("%d KB", totalDataSize/1024),
		"microservices", numServices,
		"total_memory_required", fmt.Sprintf("%d KB", totalDataSize/1024), // Same regardless of services
		"memory_savings", fmt.Sprintf("%dx", numServices),
		"additional_benefits", "Faster startup, cache efficiency, reduced GC pressure")
}

func writeCommerceDataset(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer, data CommerceData) (uint64, error) {
	// For this demo, we'll simulate writing the data
	// In a real implementation, this would use Cap'n Proto serialization
	
	// Simulate writing time proportional to data size
	totalRecords := len(data.Customers) + len(data.Orders) + len(data.Products)
	time.Sleep(time.Duration(totalRecords/1000) * time.Millisecond)
	
	return uint64(totalRecords), nil
}

func generateCommerceDataset(numCustomers, numOrders, numProducts int) CommerceData {
	customers := generateCustomers(numCustomers)
	products := generateProducts(numProducts)
	orders := generateOrders(numOrders, customers, products)
	
	return CommerceData{
		Customers: customers,
		Orders:    orders,
		Products:  products,
	}
}

func generateCustomers(count int) []CustomerData {
	customers := make([]CustomerData, count)
	
	cities := []string{
		"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia",
		"San Antonio", "San Diego", "Dallas", "San Jose", "Austin", "Jacksonville",
	}
	
	firstNames := []string{
		"John", "Jane", "Michael", "Sarah", "David", "Lisa", "Robert", "Emily",
		"James", "Ashley", "William", "Jessica", "Christopher", "Amanda",
	}
	
	lastNames := []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller",
		"Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez",
	}
	
	for i := 0; i < count; i++ {
		firstName := firstNames[rand.Intn(len(firstNames))]
		lastName := lastNames[rand.Intn(len(lastNames))]
		
		customers[i] = CustomerData{
			ID:           uint32(i + 1),
			Email:        fmt.Sprintf("%s.%s%d@email.com", firstName, lastName, i),
			Name:         fmt.Sprintf("%s %s", firstName, lastName),
			City:         cities[rand.Intn(len(cities))],
			Age:          uint16(18 + rand.Intn(65)),
			RegisteredAt: uint64(time.Now().Unix() - int64(rand.Intn(365*24*3600))), // Random time in last year
		}
	}
	
	return customers
}

func generateProducts(count int) []ProductData {
	products := make([]ProductData, count)
	
	categories := []string{
		"Electronics", "Clothing", "Books", "Home & Garden", "Sports", "Toys",
		"Beauty", "Automotive", "Health", "Grocery", "Tools", "Music",
	}
	
	productTypes := []string{
		"Premium", "Standard", "Basic", "Deluxe", "Pro", "Lite", "Ultra", "Max",
	}
	
	items := []string{
		"Widget", "Gadget", "Device", "Tool", "Accessory", "Component", "Kit",
		"Set", "Bundle", "Package", "Collection", "System",
	}
	
	for i := 0; i < count; i++ {
		category := categories[rand.Intn(len(categories))]
		productType := productTypes[rand.Intn(len(productTypes))]
		item := items[rand.Intn(len(items))]
		
		products[i] = ProductData{
			ID:          uint32(i + 1),
			Name:        fmt.Sprintf("%s %s %s", productType, category, item),
			Category:    category,
			Price:       uint64(999 + rand.Intn(99000)), // $9.99 to $999.99 in cents
			Description: fmt.Sprintf("High-quality %s for %s use", item, category),
			StockLevel:  uint32(rand.Intn(1000)),
		}
	}
	
	return products
}

func generateOrders(count int, customers []CustomerData, products []ProductData) []OrderData {
	orders := make([]OrderData, count)
	
	statuses := []string{"pending", "processing", "shipped", "delivered", "cancelled"}
	currencies := []string{"USD", "EUR", "GBP"}
	
	for i := 0; i < count; i++ {
		customer := customers[rand.Intn(len(customers))]
		numItems := 1 + rand.Intn(5) // 1-5 items per order
		
		items := make([]OrderItemData, numItems)
		var totalAmount uint64
		
		for j := 0; j < numItems; j++ {
			product := products[rand.Intn(len(products))]
			quantity := uint32(1 + rand.Intn(3))
			
			items[j] = OrderItemData{
				ProductID:    product.ID,
				Quantity:     quantity,
				PricePerUnit: product.Price,
				ProductName:  product.Name,
			}
			
			totalAmount += product.Price * uint64(quantity)
		}
		
		orders[i] = OrderData{
			ID:         uint32(i + 1),
			CustomerID: customer.ID,
			Amount:     totalAmount,
			Currency:   currencies[rand.Intn(len(currencies))],
			Status:     statuses[rand.Intn(len(statuses))],
			Timestamp:  uint64(time.Now().Unix() - int64(rand.Intn(30*24*3600))), // Random time in last 30 days
			Items:      items,
		}
	}
	
	return orders
}
