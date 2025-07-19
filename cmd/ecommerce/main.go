package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/leow/go-raw-hollow"
	"github.com/leow/go-raw-hollow/internal/memblob"
)

// E-commerce simulation with realistic event patterns
type EcommerceEvent struct {
	EventID   string                 `json:"event_id"`
	UserID    string                 `json:"user_id"`
	SessionID string                 `json:"session_id"`
	EventType string                 `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	ProductID string                 `json:"product_id,omitempty"`
	Category  string                 `json:"category,omitempty"`
	Price     float64                `json:"price,omitempty"`
	Quantity  int                    `json:"quantity,omitempty"`
	Revenue   float64                `json:"revenue,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type EcommerceSimulator struct {
	// Product catalog
	products []Product
	
	// User sessions
	activeSessions map[string]*UserSession
	sessionMutex   sync.RWMutex
	
	// Event generation
	eventCounter int64
	
	// Metrics
	metrics *EcommerceMetrics
	
	// Configuration
	config SimulationConfig
}

type Product struct {
	ID          string
	Name        string
	Category    string
	Price       float64
	Popularity  float64 // 0-1 scale
	Seasonality float64 // 0-1 scale
}

type UserSession struct {
	SessionID     string
	UserID        string
	StartTime     time.Time
	LastActivity  time.Time
	ViewedProducts []string
	CartItems     []CartItem
	HasPurchased  bool
	EventCount    int
}

type CartItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

type SimulationConfig struct {
	TotalDuration      time.Duration
	PeakHours          []int // Hours of day for peak traffic
	NumUsers           int
	ProductCatalogSize int
	EventDistribution  map[string]float64 // Event type -> probability
}

type EcommerceMetrics struct {
	mu sync.RWMutex
	
	// Business metrics
	totalRevenue       float64
	totalTransactions  int64
	totalPageViews     int64
	totalUniqueUsers   int64
	totalSessions      int64
	
	// Product metrics
	productViews       map[string]int64
	productPurchases   map[string]int64
	categoryPerformance map[string]float64
	
	// User behavior metrics
	sessionDurations   []time.Duration
	cartAbandonment    int64
	conversionFunnel   map[string]int64
	
	// Time-based metrics
	hourlyRevenue      map[int]float64
	hourlyTraffic      map[int]int64
	
	startTime time.Time
}

func NewEcommerceSimulator(config SimulationConfig) *EcommerceSimulator {
	sim := &EcommerceSimulator{
		products:       generateProductCatalog(config.ProductCatalogSize),
		activeSessions: make(map[string]*UserSession),
		metrics:        NewEcommerceMetrics(),
		config:         config,
	}
	
	return sim
}

func generateProductCatalog(size int) []Product {
	categories := []string{"Electronics", "Clothing", "Books", "Home", "Sports", "Beauty", "Toys"}
	products := make([]Product, size)
	
	for i := 0; i < size; i++ {
		category := categories[rand.Intn(len(categories))]
		products[i] = Product{
			ID:          fmt.Sprintf("prod_%d", i+1),
			Name:        fmt.Sprintf("%s Product %d", category, i+1),
			Category:    category,
			Price:       math.Round((rand.Float64()*500+10)*100) / 100,
			Popularity:  rand.Float64(),
			Seasonality: rand.Float64(),
		}
	}
	
	return products
}

func NewEcommerceMetrics() *EcommerceMetrics {
	return &EcommerceMetrics{
		productViews:        make(map[string]int64),
		productPurchases:    make(map[string]int64),
		categoryPerformance: make(map[string]float64),
		sessionDurations:    make([]time.Duration, 0),
		conversionFunnel:    make(map[string]int64),
		hourlyRevenue:       make(map[int]float64),
		hourlyTraffic:       make(map[int]int64),
		startTime:           time.Now(),
	}
}

func (sim *EcommerceSimulator) generateEvent(eventType string) *EcommerceEvent {
	sim.eventCounter++
	
	// Get or create user session
	userID := sim.getRandomUser()
	session := sim.getOrCreateSession(userID)
	
	event := &EcommerceEvent{
		EventID:   fmt.Sprintf("evt_%d", sim.eventCounter),
		UserID:    userID,
		SessionID: session.SessionID,
		EventType: eventType,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
	
	// Add event-specific data
	switch eventType {
	case "page_view":
		product := sim.getRandomProduct()
		event.ProductID = product.ID
		event.Category = product.Category
		event.Price = product.Price
		session.ViewedProducts = append(session.ViewedProducts, product.ID)
		sim.metrics.recordPageView(product.ID, product.Category)
		
	case "add_to_cart":
		if len(session.ViewedProducts) > 0 {
			productID := session.ViewedProducts[rand.Intn(len(session.ViewedProducts))]
			product := sim.getProductByID(productID)
			quantity := rand.Intn(3) + 1
			
			event.ProductID = product.ID
			event.Category = product.Category
			event.Price = product.Price
			event.Quantity = quantity
			
			session.CartItems = append(session.CartItems, CartItem{
				ProductID: product.ID,
				Quantity:  quantity,
				Price:     product.Price,
			})
		}
		
	case "purchase":
		if len(session.CartItems) > 0 {
			var totalRevenue float64
			for _, item := range session.CartItems {
				totalRevenue += item.Price * float64(item.Quantity)
				sim.metrics.recordPurchase(item.ProductID, item.Price*float64(item.Quantity))
			}
			
			event.Revenue = totalRevenue
			event.Metadata["cart_items"] = len(session.CartItems)
			session.HasPurchased = true
			session.CartItems = []CartItem{} // Clear cart
		}
		
	case "search":
		searchTerms := []string{"laptop", "dress", "book", "phone", "shoes", "watch", "camera"}
		event.Metadata["search_query"] = searchTerms[rand.Intn(len(searchTerms))]
		
	case "logout":
		sim.endSession(session)
	}
	
	session.LastActivity = time.Now()
	session.EventCount++
	
	return event
}

func (sim *EcommerceSimulator) getRandomUser() string {
	return fmt.Sprintf("user_%d", rand.Intn(sim.config.NumUsers)+1)
}

func (sim *EcommerceSimulator) getRandomProduct() Product {
	// Weighted selection based on popularity
	totalWeight := 0.0
	for _, product := range sim.products {
		totalWeight += product.Popularity
	}
	
	target := rand.Float64() * totalWeight
	current := 0.0
	
	for _, product := range sim.products {
		current += product.Popularity
		if current >= target {
			return product
		}
	}
	
	return sim.products[rand.Intn(len(sim.products))]
}

func (sim *EcommerceSimulator) getProductByID(id string) Product {
	for _, product := range sim.products {
		if product.ID == id {
			return product
		}
	}
	return sim.products[0] // Fallback
}

func (sim *EcommerceSimulator) getOrCreateSession(userID string) *UserSession {
	sim.sessionMutex.Lock()
	defer sim.sessionMutex.Unlock()
	
	sessionID := fmt.Sprintf("session_%s_%d", userID, time.Now().Unix())
	
	// Check if user has active session
	for _, session := range sim.activeSessions {
		if session.UserID == userID && time.Since(session.LastActivity) < 30*time.Minute {
			return session
		}
	}
	
	// Create new session
	session := &UserSession{
		SessionID:      sessionID,
		UserID:         userID,
		StartTime:      time.Now(),
		LastActivity:   time.Now(),
		ViewedProducts: make([]string, 0),
		CartItems:      make([]CartItem, 0),
		HasPurchased:   false,
		EventCount:     0,
	}
	
	sim.activeSessions[sessionID] = session
	sim.metrics.recordNewSession()
	
	return session
}

func (sim *EcommerceSimulator) endSession(session *UserSession) {
	sim.sessionMutex.Lock()
	defer sim.sessionMutex.Unlock()
	
	duration := time.Since(session.StartTime)
	sim.metrics.recordSessionEnd(duration, session.HasPurchased, len(session.CartItems) > 0)
	
	delete(sim.activeSessions, session.SessionID)
}

func (sim *EcommerceSimulator) cleanupSessions() {
	sim.sessionMutex.Lock()
	defer sim.sessionMutex.Unlock()
	
	cutoff := time.Now().Add(-30 * time.Minute)
	
	for sessionID, session := range sim.activeSessions {
		if session.LastActivity.Before(cutoff) {
			duration := session.LastActivity.Sub(session.StartTime)
			sim.metrics.recordSessionEnd(duration, session.HasPurchased, len(session.CartItems) > 0)
			delete(sim.activeSessions, sessionID)
		}
	}
}

// Metrics recording methods
func (em *EcommerceMetrics) recordPageView(productID, category string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	
	em.totalPageViews++
	em.productViews[productID]++
	em.conversionFunnel["page_view"]++
	
	hour := time.Now().Hour()
	em.hourlyTraffic[hour]++
}

func (em *EcommerceMetrics) recordPurchase(productID string, revenue float64) {
	em.mu.Lock()
	defer em.mu.Unlock()
	
	em.totalRevenue += revenue
	em.totalTransactions++
	em.productPurchases[productID]++
	em.conversionFunnel["purchase"]++
	
	hour := time.Now().Hour()
	em.hourlyRevenue[hour] += revenue
}

func (em *EcommerceMetrics) recordNewSession() {
	em.mu.Lock()
	defer em.mu.Unlock()
	
	em.totalSessions++
	em.totalUniqueUsers++
}

func (em *EcommerceMetrics) recordSessionEnd(duration time.Duration, purchased bool, hadCart bool) {
	em.mu.Lock()
	defer em.mu.Unlock()
	
	em.sessionDurations = append(em.sessionDurations, duration)
	
	if hadCart && !purchased {
		em.cartAbandonment++
	}
}

func (em *EcommerceMetrics) PrintBusinessReport() {
	em.mu.RLock()
	defer em.mu.RUnlock()
	
	fmt.Print("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("E-COMMERCE BUSINESS REPORT\n")
	fmt.Print(strings.Repeat("=", 60) + "\n")
	
	// Revenue metrics
	fmt.Printf("Revenue Metrics:\n")
	fmt.Printf("  Total Revenue:        $%.2f\n", em.totalRevenue)
	fmt.Printf("  Total Transactions:   %d\n", em.totalTransactions)
	if em.totalTransactions > 0 {
		fmt.Printf("  Average Order Value:  $%.2f\n", em.totalRevenue/float64(em.totalTransactions))
	}
	fmt.Printf("\n")
	
	// Traffic metrics
	fmt.Printf("Traffic Metrics:\n")
	fmt.Printf("  Total Page Views:     %d\n", em.totalPageViews)
	fmt.Printf("  Total Sessions:       %d\n", em.totalSessions)
	fmt.Printf("  Total Unique Users:   %d\n", em.totalUniqueUsers)
	fmt.Printf("\n")
	
	// Conversion metrics
	fmt.Printf("Conversion Metrics:\n")
	if em.totalPageViews > 0 {
		conversionRate := float64(em.totalTransactions) / float64(em.totalPageViews) * 100
		fmt.Printf("  Conversion Rate:      %.2f%%\n", conversionRate)
	}
	if em.cartAbandonment > 0 {
		abandonmentRate := float64(em.cartAbandonment) / float64(em.totalSessions) * 100
		fmt.Printf("  Cart Abandonment:     %.2f%%\n", abandonmentRate)
	}
	fmt.Printf("\n")
	
	// Session analysis
	if len(em.sessionDurations) > 0 {
		var totalDuration time.Duration
		for _, duration := range em.sessionDurations {
			totalDuration += duration
		}
		avgDuration := totalDuration / time.Duration(len(em.sessionDurations))
		fmt.Printf("Session Analysis:\n")
		fmt.Printf("  Average Session:      %v\n", avgDuration)
		fmt.Printf("  Completed Sessions:   %d\n", len(em.sessionDurations))
		fmt.Printf("\n")
	}
	
	// Top products
	fmt.Printf("Top 10 Viewed Products:\n")
	em.printTopProducts(em.productViews, 10)
	
	fmt.Printf("\nTop 10 Purchased Products:\n")
	em.printTopProducts(em.productPurchases, 10)
	
	// Hourly patterns
	fmt.Printf("\nHourly Traffic Pattern:\n")
	for hour := 0; hour < 24; hour++ {
		traffic := em.hourlyTraffic[hour]
		revenue := em.hourlyRevenue[hour]
		fmt.Printf("  %02d:00 - %6d views, $%.2f revenue\n", hour, traffic, revenue)
	}
	
	fmt.Print(strings.Repeat("=", 60) + "\n")
}

func (em *EcommerceMetrics) printTopProducts(data map[string]int64, limit int) {
	type productStat struct {
		id    string
		count int64
	}
	
	var stats []productStat
	for id, count := range data {
		stats = append(stats, productStat{id, count})
	}
	
	// Sort by count
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[i].count < stats[j].count {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}
	
	for i, stat := range stats {
		if i >= limit {
			break
		}
		fmt.Printf("  %s: %d\n", stat.id, stat.count)
	}
}

func main() {
	fmt.Printf("Hollow-Go E-commerce Simulation\n")
	fmt.Printf("Realistic event patterns with 10k events over 10 minutes\n\n")
	
	// Configuration
	config := SimulationConfig{
		TotalDuration:      10 * time.Minute,
		PeakHours:          []int{9, 10, 11, 14, 15, 16, 20, 21}, // Business hours + evening
		NumUsers:           500,
		ProductCatalogSize: 100,
		EventDistribution: map[string]float64{
			"page_view":    0.60, // 60% of events
			"search":       0.15, // 15% of events
			"add_to_cart":  0.12, // 12% of events
			"purchase":     0.08, // 8% of events
			"logout":       0.05, // 5% of events
		},
	}
	
	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	
	// Create simulator
	simulator := NewEcommerceSimulator(config)
	
	// Create blob store
	blob := memblob.New()
	
	// Create producer with business metrics
	producer := hollow.NewProducer(
		hollow.WithBlobStager(blob),
		hollow.WithLogger(logger),
		hollow.WithMetricsCollector(func(m hollow.Metrics) {
			logger.Info("Producer cycle", "version", m.Version, "records", m.RecordCount, "bytes", m.ByteSize)
		}),
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
	
	// Generate events with normal distribution
	fmt.Printf("Starting e-commerce simulation...\n")
	
	startTime := time.Now()
	targetEvents := 10000
	processedEvents := 0
	
	// Background session cleanup
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				simulator.cleanupSessions()
			case <-time.After(config.TotalDuration + time.Minute):
				return
			}
		}
	}()
	
	// Event generation loop
	for processedEvents < targetEvents {
		elapsed := time.Since(startTime)
		if elapsed >= config.TotalDuration {
			break
		}
		
		// Calculate normal distribution timing
		progress := elapsed.Seconds() / config.TotalDuration.Seconds()
		
		// Peak activity in the middle of the simulation
		normalizedTime := (progress - 0.5) * 4 // Scale to -2 to 2
		intensity := math.Exp(-normalizedTime * normalizedTime) // Gaussian curve
		
		// Determine batch size based on intensity
		batchSize := int(1 + intensity*19) // 1-20 events per batch
		
		// Generate event batch
		batch := make([]*EcommerceEvent, 0, batchSize)
		for i := 0; i < batchSize && processedEvents < targetEvents; i++ {
			// Choose event type based on distribution
			eventType := chooseEventType(config.EventDistribution)
			event := simulator.generateEvent(eventType)
			batch = append(batch, event)
			processedEvents++
		}
		
		// Process batch
		if len(batch) > 0 {
			_, err := producer.RunCycle(func(ws hollow.WriteState) error {
				for _, event := range batch {
					key := fmt.Sprintf("event_%s", event.EventID)
					if err := ws.Add(key); err != nil {
						return err
					}
				}
				return nil
			})
			
			if err != nil {
				logger.Error("Producer cycle failed", "error", err)
				break
			}
		}
		
		// Refresh consumer periodically
		if processedEvents%100 == 0 {
			if err := consumer.Refresh(); err != nil {
				logger.Error("Consumer refresh failed", "error", err)
			}
		}
		
		// Progress update
		if processedEvents%1000 == 0 {
			fmt.Printf("Progress: %d/%d events (%.1f%%) - Elapsed: %v\n",
				processedEvents, targetEvents, 
				float64(processedEvents)/float64(targetEvents)*100,
				elapsed)
		}
		
		// Variable delay based on intensity
		delay := time.Duration(float64(50*time.Millisecond) * (2.0 - intensity))
		time.Sleep(delay)
	}
	
	// Final processing
	consumer.Refresh()
	simulator.cleanupSessions()
	
	totalDuration := time.Since(startTime)
	fmt.Printf("\nSimulation completed!\n")
	fmt.Printf("Processed %d events in %v\n", processedEvents, totalDuration)
	
	// Print business report
	simulator.metrics.PrintBusinessReport()
	
	// Final consumer state
	rs := consumer.ReadState()
	fmt.Printf("\nFinal Dataset:\n")
	fmt.Printf("  Consumer Version: %d\n", consumer.CurrentVersion())
	fmt.Printf("  Dataset Size:     %d records\n", rs.Size())
	
	// Test diff functionality on business data
	fmt.Printf("\nTesting business data diffs...\n")
	testBusinessDiffs()
}

func chooseEventType(distribution map[string]float64) string {
	r := rand.Float64()
	cumulative := 0.0
	
	for eventType, prob := range distribution {
		cumulative += prob
		if r <= cumulative {
			return eventType
		}
	}
	
	return "page_view" // Default
}

func testBusinessDiffs() {
	// Simulate business metrics changes
	yesterdayMetrics := map[string]any{
		"daily_revenue":       15000.50,
		"daily_transactions":  120,
		"daily_users":         890,
		"conversion_rate":     2.3,
		"top_product":         "prod_15",
		"cart_abandonment":    68.5,
	}
	
	todayMetrics := map[string]any{
		"daily_revenue":       18500.75,
		"daily_transactions":  145,
		"daily_users":         1050,
		"conversion_rate":     2.8,
		"top_product":         "prod_23",
		"cart_abandonment":    62.1,
		"new_feature_usage":   125,
	}
	
	diff := hollow.DiffData(yesterdayMetrics, todayMetrics)
	
	fmt.Printf("Business Metrics Diff:\n")
	fmt.Printf("  Added Metrics:    %v\n", diff.GetAdded())
	fmt.Printf("  Removed Metrics:  %v\n", diff.GetRemoved())
	fmt.Printf("  Changed Metrics:  %v\n", diff.GetChanged())
	
	// Calculate business impact
	if len(diff.GetChanged()) > 0 {
		fmt.Printf("  Business Impact:  %d metrics changed\n", len(diff.GetChanged()))
	}
	
	if len(diff.GetAdded()) > 0 {
		fmt.Printf("  New Features:     %d new metrics tracked\n", len(diff.GetAdded()))
	}
}
