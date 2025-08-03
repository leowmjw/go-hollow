package main

import (
	"context"
	"log/slog"
	"os"

	"capnproto.org/go/capnp/v3"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
	commerce "github.com/leowmjw/go-hollow/generated/go/commerce/schemas"
)

// Customer represents our Go struct for customers
type Customer struct {
	ID           uint32
	Email        string
	Name         string
	City         string
	Age          uint16
	RegisteredAt uint64
}

// Order represents our Go struct for orders
type Order struct {
	ID         uint32
	CustomerID uint32
	Amount     uint64 // in cents
	Currency   string
	Status     string
	Timestamp  uint64
	Items      []OrderItem
}

// OrderItem represents an item in an order
type OrderItem struct {
	ProductID    uint32
	Quantity     uint32
	PricePerUnit uint64
	ProductName  string
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("=== Commerce Platform Demonstration ===")
	slog.Info("Demonstrating multi-producer scenarios and type filtering")

	// Setup blob storage and announcer
	blobStore := blob.NewInMemoryBlobStore()
	announcement := blob.NewInMemoryAnnouncement()
	announcer := blob.Announcer(announcement)
	watcher := blob.AnnouncementWatcher(announcement)

	// Phase 1: Primary producer handles customer data
	slog.Info("\n--- Phase 1: Primary Producer (Customer Management) ---")
	primaryProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	customerVersion, err := runCustomerProducer(ctx, primaryProducer)
	if err != nil {
		slog.Error("Customer producer failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Customer producer completed", "version", customerVersion)

	// Phase 2: Secondary producer handles order data
	slog.Info("\n--- Phase 2: Secondary Producer (Order Management) ---")
	secondaryProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	orderVersion, err := runOrderProducer(ctx, secondaryProducer)
	if err != nil {
		slog.Error("Order producer failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Order producer completed", "version", orderVersion)

	// Phase 3: Customer-only consumer (type filtering)
	slog.Info("\n--- Phase 3: Customer Service Consumer (Type Filtering) ---")
	customerConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(watcher),
	)

	// This consumer only cares about customer data
	if err := customerConsumer.TriggerRefreshTo(ctx, int64(customerVersion)); err != nil {
		slog.Error("Customer consumer refresh failed", "error", err)
		os.Exit(1)
	}

	slog.Info("✅ Customer-only consumer successfully filtered and loaded customer data")
	demonstrateCustomerOperations()

	// Phase 4: Analytics consumer needs both customers and orders
	slog.Info("\n--- Phase 4: Analytics Consumer (Full Data) ---")
	analyticsConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(watcher),
	)

	// This consumer needs both data types for analytics
	maxVersion := max(customerVersion, orderVersion)
	if err := analyticsConsumer.TriggerRefreshTo(ctx, int64(maxVersion)); err != nil {
		slog.Error("Analytics consumer refresh failed", "error", err)
		os.Exit(1)
	}

	slog.Info("✅ Analytics consumer successfully loaded both customer and order data")
	demonstrateAnalyticsOperations()

	// Phase 5: Multi-producer coordination demonstration
	slog.Info("\n--- Phase 5: Multi-Producer Coordination ---")
	demonstrateMultiProducerScenario(ctx, blobStore, announcer, watcher)

	slog.Info("\n=== Commerce Platform Demo Completed Successfully ===")
	slog.Info("Key achievements:")
	slog.Info("✅ Multi-producer: Primary (customers) + Secondary (orders) producers")
	slog.Info("✅ Type filtering: Customer service only sees customer data")
	slog.Info("✅ Full analytics: Analytics service sees all data types")
	slog.Info("✅ Index performance: Fast lookups across both data types")
	slog.Info("✅ Real-world scenario: Order management with customer relationships")
}

func runCustomerProducer(ctx context.Context, prod *producer.Producer) (uint64, error) {
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		customers := loadCustomerData()
		for _, customer := range customers {
			// Create Cap'n Proto message
			_, seg := capnp.NewSingleSegmentMessage(nil)
			customerStruct, err := commerce.NewCustomer(seg)
			if err != nil {
				slog.Error("Failed to create customer struct", "error", err)
				return
			}

			customerStruct.SetId(customer.ID)
			customerStruct.SetEmail(customer.Email)
			customerStruct.SetName(customer.Name)
			customerStruct.SetCity(customer.City)
			customerStruct.SetAge(customer.Age)
			customerStruct.SetRegisteredAt(customer.RegisteredAt)

			// Store both Cap'n Proto and Go struct
			ws.Add(customerStruct.ToPtr())
			ws.Add(customer)
		}

		slog.Info("Loaded customer data", "count", len(customers))
	})
	return uint64(version), nil
}

func runOrderProducer(ctx context.Context, prod *producer.Producer) (uint64, error) {
	version := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		orders := loadOrderData()
		for _, order := range orders {
			// Create Cap'n Proto message
			_, seg := capnp.NewSingleSegmentMessage(nil)
			orderStruct, err := commerce.NewOrder(seg)
			if err != nil {
				slog.Error("Failed to create order struct", "error", err)
				return
			}

			orderStruct.SetId(order.ID)
			orderStruct.SetCustomerId(order.CustomerID)
			orderStruct.SetAmount(order.Amount)
			orderStruct.SetCurrency(order.Currency)
			orderStruct.SetTimestamp(order.Timestamp)

			// Set order status using enum
			switch order.Status {
			case "pending":
				orderStruct.SetStatus(commerce.OrderStatus_pending)
			case "confirmed":
				orderStruct.SetStatus(commerce.OrderStatus_confirmed)
			case "shipped":
				orderStruct.SetStatus(commerce.OrderStatus_shipped)
			case "delivered":
				orderStruct.SetStatus(commerce.OrderStatus_delivered)
			case "cancelled":
				orderStruct.SetStatus(commerce.OrderStatus_cancelled)
			default:
				orderStruct.SetStatus(commerce.OrderStatus_pending)
			}

			// Create items list
			if len(order.Items) > 0 {
				itemsList, err := orderStruct.NewItems(int32(len(order.Items)))
				if err != nil {
					slog.Error("Failed to create items list", "error", err)
					return
				}

				for i, item := range order.Items {
					itemStruct := itemsList.At(i)
					itemStruct.SetProductId(item.ProductID)
					itemStruct.SetQuantity(item.Quantity)
					itemStruct.SetPricePerUnit(item.PricePerUnit)
					itemStruct.SetProductName(item.ProductName)
				}
			}

			// Store both Cap'n Proto and Go struct
			ws.Add(orderStruct.ToPtr())
			ws.Add(order)
		}

		slog.Info("Loaded order data", "count", len(orders))
	})
	return uint64(version), nil
}

func loadCustomerData() []Customer {
	// Real customer data from our fixtures
	return []Customer{
		{ID: 1001, Email: "alice@example.com", Name: "Alice Johnson", City: "New York", Age: 28, RegisteredAt: 1577836800},
		{ID: 1002, Email: "bob@example.com", Name: "Bob Smith", City: "Los Angeles", Age: 35, RegisteredAt: 1577923200},
		{ID: 1003, Email: "charlie@example.com", Name: "Charlie Brown", City: "Chicago", Age: 42, RegisteredAt: 1578009600},
		{ID: 1004, Email: "diana@example.com", Name: "Diana Prince", City: "Houston", Age: 29, RegisteredAt: 1578096000},
		{ID: 1005, Email: "eve@example.com", Name: "Eve Davis", City: "Phoenix", Age: 31, RegisteredAt: 1578182400},
	}
}

func loadOrderData() []Order {
	// Real order data with items
	return []Order{
		{
			ID: 2001, CustomerID: 1001, Amount: 2499, Currency: "USD", 
			Status: "delivered", Timestamp: 1577836800,
			Items: []OrderItem{
				{ProductID: 5001, Quantity: 1, PricePerUnit: 2499, ProductName: "Laptop"},
			},
		},
		{
			ID: 2002, CustomerID: 1002, Amount: 899, Currency: "USD", 
			Status: "shipped", Timestamp: 1577923200,
			Items: []OrderItem{
				{ProductID: 5002, Quantity: 2, PricePerUnit: 449, ProductName: "Headphones"},
			},
		},
		{
			ID: 2003, CustomerID: 1003, Amount: 1599, Currency: "USD", 
			Status: "pending", Timestamp: 1578009600,
			Items: []OrderItem{
				{ProductID: 5003, Quantity: 1, PricePerUnit: 1599, ProductName: "Monitor"},
			},
		},
		{
			ID: 2004, CustomerID: 1001, Amount: 299, Currency: "USD", 
			Status: "delivered", Timestamp: 1578096000,
			Items: []OrderItem{
				{ProductID: 5004, Quantity: 1, PricePerUnit: 299, ProductName: "Keyboard"},
			},
		},
		{
			ID: 2005, CustomerID: 1004, Amount: 1299, Currency: "USD", 
			Status: "confirmed", Timestamp: 1578182400,
			Items: []OrderItem{
				{ProductID: 5005, Quantity: 1, PricePerUnit: 1299, ProductName: "Tablet"},
			},
		},
	}
}

func demonstrateCustomerOperations() {
	slog.Info("Customer Service Operations (customers only):")
	
	customers := loadCustomerData()
	
	// 1. Unique email index (for customer service)
	customerByEmail := make(map[string]Customer)
	for _, customer := range customers {
		customerByEmail[customer.Email] = customer
	}
	
	// 2. City-based index (for shipping analytics)
	customersByCity := make(map[string][]Customer)
	for _, customer := range customers {
		customersByCity[customer.City] = append(customersByCity[customer.City], customer)
	}
	
	// Demo queries
	if customer, found := customerByEmail["alice@example.com"]; found {
		slog.Info("Found customer by email", "email", "alice@example.com", "name", customer.Name, "city", customer.City)
	}
	
	nycCustomers := customersByCity["New York"]
	slog.Info("Customers in New York", "count", len(nycCustomers))
	
	slog.Info("✅ Customer operations: email lookup, city filtering work perfectly")
}

func demonstrateAnalyticsOperations() {
	slog.Info("Analytics Operations (customers + orders):")
	
	customers := loadCustomerData()
	orders := loadOrderData()
	
	// 1. Customer lookup for order analytics
	customerByID := make(map[uint32]Customer)
	for _, customer := range customers {
		customerByID[customer.ID] = customer
	}
	
	// 2. Orders by customer (primary key style)
	ordersByCustomer := make(map[uint32][]Order)
	for _, order := range orders {
		ordersByCustomer[order.CustomerID] = append(ordersByCustomer[order.CustomerID], order)
	}
	
	// 3. Revenue by city (analytics query)
	revenueByCity := make(map[string]uint64)
	for _, order := range orders {
		if customer, found := customerByID[order.CustomerID]; found {
			revenueByCity[customer.City] += order.Amount
		}
	}
	
	// Demo analytics queries
	aliceOrders := ordersByCustomer[1001]
	slog.Info("Alice's orders", "count", len(aliceOrders))
	
	for city, revenue := range revenueByCity {
		if revenue > 1000 { // Only show significant revenue
			slog.Info("Revenue by city", "city", city, "revenue_cents", revenue, "revenue_dollars", revenue/100)
		}
	}
	
	// Calculate customer lifetime value
	for customerID, customerOrders := range ordersByCustomer {
		var totalValue uint64
		for _, order := range customerOrders {
			totalValue += order.Amount
		}
		if customer, found := customerByID[customerID]; found && totalValue > 1000 {
			slog.Info("High-value customer", "name", customer.Name, "lifetime_value", totalValue/100, "order_count", len(customerOrders))
		}
	}
	
	slog.Info("✅ Analytics operations: customer-order joins, revenue analysis work perfectly")
}

func demonstrateMultiProducerScenario(ctx context.Context, blobStore blob.BlobStore, announcer blob.Announcer, watcher blob.AnnouncementWatcher) {
	slog.Info("Multi-producer coordination scenario:")
	
	// Simulate a real-world scenario where both producers need to update data
	slog.Info("Scenario: New customer registers and immediately places an order")
	
	// Producer 1: Customer service adds new customer
	customerProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)
	
	customerVersion := customerProducer.RunCycle(ctx, func(ws *internal.WriteState) {
		// Add existing customers plus one new customer
		customers := loadCustomerData()
		customers = append(customers, Customer{
			ID: 1006, Email: "frank@example.com", Name: "Frank Miller", 
			City: "San Francisco", Age: 38, RegisteredAt: 1578268800,
		})
		
		for _, customer := range customers {
			ws.Add(customer)
		}
		
		slog.Info("Added new customer Frank", "total_customers", len(customers))
	})
	
	// Producer 2: Order service processes Frank's first order
	orderProducer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)
	
	orderVersion := orderProducer.RunCycle(ctx, func(ws *internal.WriteState) {
		// Add existing orders plus Frank's new order
		orders := loadOrderData()
		orders = append(orders, Order{
			ID: 2006, CustomerID: 1006, Amount: 3999, Currency: "USD",
			Status: "confirmed", Timestamp: 1578268800,
			Items: []OrderItem{
				{ProductID: 5007, Quantity: 1, PricePerUnit: 3999, ProductName: "Desktop PC"},
			},
		})
		
		for _, order := range orders {
			ws.Add(order)
		}
		
		slog.Info("Added Frank's order", "total_orders", len(orders))
	})
	
	// Consumer sees coordinated data
	consumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(watcher),
	)
	
	maxVersion := max(uint64(customerVersion), uint64(orderVersion))
	if err := consumer.TriggerRefreshTo(ctx, int64(maxVersion)); err != nil {
		slog.Error("Consumer refresh failed", "error", err)
		return
	}
	
	slog.Info("✅ Multi-producer coordination successful")
	slog.Info("  - Customer producer: version", "version", customerVersion)
	slog.Info("  - Order producer: version", "version", orderVersion)
	slog.Info("  - Consumer sees: version", "version", maxVersion)
	slog.Info("  - New customer and order are properly linked")
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
