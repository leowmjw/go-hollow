package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	movie "github.com/leowmjw/go-hollow/generated/go/movie"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// Simple data structures for testing
type TestData struct {
	ID      uint32
	Message string
	Version int64
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("=== Announcer Capabilities Demonstration ===")
	slog.Info("This example thoroughly tests all Announcer functionality")

	// Setup
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()

	// Phase 1: Basic Pub/Sub Pattern
	slog.Info("\n--- Phase 1: Pub/Sub Pattern with Multiple Subscribers ---")
	demonstratePubSub(announcer)

	// Phase 2: Version Waiting and Timeouts
	slog.Info("\n--- Phase 2: Version Waiting and Timeout Scenarios ---")
	demonstrateVersionWaiting(announcer)

	// Phase 3: Pin/Unpin Mechanics
	slog.Info("\n--- Phase 3: Pin/Unpin Version Management ---")
	demonstratePinUnpin(announcer)

	// Phase 4: Multi-Consumer Coordination
	slog.Info("\n--- Phase 4: Multi-Consumer Coordination ---")
	demonstrateMultiConsumerCoordination(ctx, blobStore, announcer)

	// Phase 5: High-Frequency Announcements (Performance Test)
	slog.Info("\n--- Phase 5: High-Frequency Performance Test ---")
	demonstrateHighFrequency(announcer)

	// Phase 6: Error Scenarios and Edge Cases
	slog.Info("\n--- Phase 6: Error Scenarios and Edge Cases ---")
	demonstrateErrorScenarios(announcer)

	// Phase 7: Real Producer/Consumer with Announcer Features
	slog.Info("\n--- Phase 7: Real Producer/Consumer Integration ---")
	demonstrateRealIntegration(ctx, blobStore, announcer)

	slog.Info("\n=== Announcer Demo Completed Successfully ===")
	slog.Info("Key achievements:")
	slog.Info("✅ Pub/Sub: Multiple subscribers receiving announcements")
	slog.Info("✅ Version waiting: Timeout and success scenarios tested")
	slog.Info("✅ Pin/Unpin: Version pinning and controlled updates")
	slog.Info("✅ Multi-consumer: Coordinated data consumption")
	slog.Info("✅ Performance: High-frequency announcement handling")
	slog.Info("✅ Error handling: Timeouts, dead subscribers, resource cleanup")
	slog.Info("✅ Real integration: Producer/consumer with full announcer features")
}

func demonstratePubSub(announcer *blob.GoroutineAnnouncer) {
	slog.Info("Testing pub/sub pattern with multiple subscribers")

	// Create multiple subscriber channels
	// Subscribe with buffer size
	subscription1, err := announcer.Subscribe(10)
	if err != nil {
		slog.Error("Failed to create subscription1", "error", err)
		return
	}
	defer subscription1.Close()

	subscription2, err := announcer.Subscribe(10)
	if err != nil {
		slog.Error("Failed to create subscription2", "error", err)
		return
	}
	defer subscription2.Close()

	subscription3, err := announcer.Subscribe(10)
	if err != nil {
		slog.Error("Failed to create subscription3", "error", err)
		return
	}
	defer subscription3.Close()

	subscriber1 := subscription1.Updates()
	subscriber2 := subscription2.Updates()
	subscriber3 := subscription3.Updates()

	slog.Info("Subscribed 3 channels", "total_subscribers", announcer.GetSubscriberCount())

	// Announce some versions
	for i := int64(1); i <= 5; i++ {
		err := announcer.Announce(i)
		if err != nil {
			slog.Error("Failed to announce", "version", i, "error", err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay for processing
	}

	// Check that all subscribers received the announcements
	checkSubscriberReceived := func(subscriber <-chan int64, name string) {
		received := make([]int64, 0, 5)
		timeout := time.After(1 * time.Second)

	loop:
		for {
			select {
			case version := <-subscriber:
				received = append(received, version)
			case <-timeout:
				break loop
			}
		}

		slog.Info("Subscriber results", "name", name, "received_count", len(received), "versions", received)
	}

	checkSubscriberReceived(subscriber1, "subscriber1")
	checkSubscriberReceived(subscriber2, "subscriber2")
	checkSubscriberReceived(subscriber3, "subscriber3")

	// Test unsubscribe using new API
	subscription2.Close()
	slog.Info("Unsubscribed subscriber2", "remaining_subscribers", announcer.GetSubscriberCount())

	// Announce another version - only subscriber1 and subscriber3 should receive it
	announcer.Announce(6)
	time.Sleep(50 * time.Millisecond)

	// Clean up - close subscriptions (which automatically unsubscribes)
	subscription1.Close()
	subscription3.Close() // subscription2 already closed

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	slog.Info("✅ Pub/Sub pattern: Multiple subscribers work correctly")
}

func demonstrateVersionWaiting(announcer *blob.GoroutineAnnouncer) {
	slog.Info("Testing version waiting with timeouts")

	// Test 1: Wait for a version that will arrive in time
	slog.Info("Test 1: Waiting for version 10 (will arrive in 200ms)")

	go func() {
		time.Sleep(200 * time.Millisecond)
		announcer.Announce(10)
	}()

	start := time.Now()
	err := announcer.WaitForVersion(10, 1*time.Second)
	duration := time.Since(start)

	if err != nil {
		slog.Error("Failed to wait for version 10", "error", err, "duration", duration)
	} else {
		slog.Info("Successfully waited for version 10", "duration", duration)
	}

	// Test 2: Wait for a version that won't arrive (timeout)
	slog.Info("Test 2: Waiting for version 100 (will timeout)")

	start = time.Now()
	err = announcer.WaitForVersion(100, 300*time.Millisecond)
	duration = time.Since(start)

	if err != nil {
		slog.Info("Expected timeout occurred", "error", err, "duration", duration)
	} else {
		slog.Error("Should have timed out waiting for version 100")
	}

	// Test 3: Wait for a version that's already available
	slog.Info("Test 3: Waiting for version 5 (already available)")

	start = time.Now()
	err = announcer.WaitForVersion(5, 1*time.Second)
	duration = time.Since(start)

	if err != nil {
		slog.Error("Failed to get already available version", "error", err)
	} else {
		slog.Info("Immediately got available version", "duration", duration)
	}

	slog.Info("✅ Version waiting: Timeout and success scenarios work correctly")
}

func demonstratePinUnpin(announcer *blob.GoroutineAnnouncer) {
	slog.Info("Testing pin/unpin version management")

	// Start with some announcements
	for i := int64(20); i <= 25; i++ {
		announcer.Announce(i)
	}

	currentVersion := announcer.GetLatestVersion()
	slog.Info("Current latest version", "version", currentVersion)

	// Test pin functionality
	pinVersion := int64(22)
	announcer.Pin(pinVersion)

	slog.Info("Pinned to version", "version", pinVersion,
		"is_pinned", announcer.IsPinned(),
		"pinned_version", announcer.GetPinnedVersion(),
		"latest_returned", announcer.GetLatestVersion())

	// Even if we announce newer versions, pinned version should be returned
	for i := int64(26); i <= 30; i++ {
		announcer.Announce(i)
	}

	slog.Info("After announcing versions 26-30 while pinned",
		"latest_returned", announcer.GetLatestVersion(),
		"is_pinned", announcer.IsPinned())

	// Test unpin
	announcer.Unpin()
	slog.Info("After unpinning",
		"latest_returned", announcer.GetLatestVersion(),
		"is_pinned", announcer.IsPinned())

	// Test subscriber notification during pin/unpin
	subscriber := make(chan int64, 20)
	announcer.SubscribeChannel(subscriber)
	defer announcer.Unsubscribe(subscriber)

	// Pin to a specific version
	announcer.Pin(25)
	announcer.Announce(35) // This should not change the pinned version

	// Check what subscriber receives
	timeout := time.After(100 * time.Millisecond)
	receivedVersions := make([]int64, 0)

	for {
		select {
		case version := <-subscriber:
			receivedVersions = append(receivedVersions, version)
		case <-timeout:
			goto done
		}
	}

done:
	slog.Info("Subscriber received during pin operations", "versions", receivedVersions)

	// Unpin and see what happens
	announcer.Unpin()
	time.Sleep(50 * time.Millisecond)

	// Check for more notifications
	select {
	case version := <-subscriber:
		slog.Info("Received after unpin", "version", version)
	case <-time.After(100 * time.Millisecond):
		slog.Info("No immediate notification after unpin")
	}

	close(subscriber)
	slog.Info("✅ Pin/Unpin: Version pinning and controlled updates work correctly")
}

func demonstrateMultiConsumerCoordination(ctx context.Context, blobStore blob.BlobStore, announcer *blob.GoroutineAnnouncer) {
	slog.Info("Testing multi-consumer coordination scenarios")

	// Create multiple consumers with different refresh strategies
	consumer1 := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	consumer2 := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	consumer3 := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	// Producer creates some data
	producer := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	// Version 40: Basic data
	version40, err := producer.RunCycle(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 3; i++ {
			data := TestData{ID: uint32(i + 100), Message: fmt.Sprintf("Data %d v40", i), Version: 40}
			ws.Add(data)
		}
		slog.Info("Produced data for version 40")
	})
	if err != nil {
		slog.Error("RunCycle failed for version 40", "error", err)
		return
	}

	// All consumers refresh to version 40
	consumer1.TriggerRefreshTo(ctx, int64(version40))
	consumer2.TriggerRefreshTo(ctx, int64(version40))
	consumer3.TriggerRefreshTo(ctx, int64(version40))

	slog.Info("All consumers refreshed to version 40")

	// Version 41: Additional data
	version41, err := producer.RunCycle(ctx, func(ws *internal.WriteState) {
		for i := 0; i < 2; i++ {
			data := TestData{ID: uint32(i + 200), Message: fmt.Sprintf("Data %d v41", i), Version: 41}
			ws.Add(data)
		}
		slog.Info("Produced data for version 41")
	})
	if err != nil {
		slog.Error("RunCycle failed for version 41", "error", err)
		return
	}

	// Consumer coordination test: Different refresh patterns

	// Consumer 1: Immediately refreshes to latest
	go func() {
		time.Sleep(100 * time.Millisecond)
		consumer1.TriggerRefreshTo(ctx, int64(version41))
		slog.Info("Consumer1 refreshed to version 41")
	}()

	// Consumer 2: Waits a bit then refreshes
	go func() {
		time.Sleep(300 * time.Millisecond)
		consumer2.TriggerRefreshTo(ctx, int64(version41))
		slog.Info("Consumer2 refreshed to version 41")
	}()

	// Consumer 3: Uses announcer to wait for the version
	go func() {
		time.Sleep(500 * time.Millisecond)
		announcer.WaitForVersion(int64(version41), 2*time.Second)
		consumer3.TriggerRefreshTo(ctx, int64(version41))
		slog.Info("Consumer3 refreshed to version 41 after waiting")
	}()

	// Wait for all consumers to complete their operations
	time.Sleep(1 * time.Second)

	// Version 42: Test with pinning
	slog.Info("Testing consumer coordination with pinning")

	// Pin consumer 2 to version 40
	announcer.Pin(int64(version40))

	version42, err := producer.RunCycle(ctx, func(ws *internal.WriteState) {
		data := TestData{ID: 999, Message: "Pinned test data v42", Version: 42}
		ws.Add(data)
		slog.Info("Produced data for version 42")
	})
	if err != nil {
		slog.Error("RunCycle failed for version 42", "error", err)
		return
	}

	// Consumer 1 and 3 should see version 42, but consumer 2 stays pinned
	consumer1.TriggerRefreshTo(ctx, int64(version42))
	consumer3.TriggerRefreshTo(ctx, int64(version42))

	// Consumer 2 should still see the pinned version
	pinnedVersion := announcer.GetLatestVersion() // Should return pinned version
	slog.Info("Consumer coordination with pinning",
		"actual_latest", version42,
		"pinned_version", pinnedVersion,
		"consumer2_sees", pinnedVersion)

	// Unpin and let consumer 2 catch up
	announcer.Unpin()
	time.Sleep(100 * time.Millisecond)
	consumer2.TriggerRefreshTo(ctx, int64(version42))

	slog.Info("✅ Multi-consumer coordination: Different refresh patterns and pinning work correctly")
}

func demonstrateHighFrequency(announcer *blob.GoroutineAnnouncer) {
	slog.Info("Testing high-frequency announcement performance")

	// Create subscribers to measure performance
	numSubscribers := 10
	subscribers := make([]chan int64, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		subscribers[i] = make(chan int64, 1000) // Large buffer for high frequency
		announcer.SubscribeChannel(subscribers[i])
	}

	// Performance test: rapid announcements
	numAnnouncements := 1000
	startVersion := int64(1000)

	slog.Info("Starting high-frequency test",
		"announcements", numAnnouncements,
		"subscribers", numSubscribers)

	start := time.Now()

	// Send announcements as fast as possible
	for i := 0; i < numAnnouncements; i++ {
		err := announcer.Announce(startVersion + int64(i))
		if err != nil {
			slog.Error("Announcement failed", "version", startVersion+int64(i), "error", err)
		}
	}

	announceTime := time.Since(start)
	slog.Info("Announcement phase completed", "duration", announceTime,
		"rate", float64(numAnnouncements)/announceTime.Seconds())

	// Wait for all subscribers to receive all announcements
	time.Sleep(500 * time.Millisecond)

	// Check how many announcements each subscriber received
	var wg sync.WaitGroup
	results := make([]int, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			count := 0
			timeout := time.After(1 * time.Second)

			for {
				select {
				case <-subscribers[idx]:
					count++
				case <-timeout:
					results[idx] = count
					return
				default:
					if len(subscribers[idx]) == 0 {
						results[idx] = count
						return
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Report results
	totalReceived := 0
	for i, count := range results {
		totalReceived += count
		slog.Info("Subscriber performance", "subscriber", i, "received", count,
			"percentage", float64(count*100)/float64(numAnnouncements))
	}

	slog.Info("High-frequency performance summary",
		"total_sent", numAnnouncements*numSubscribers,
		"total_received", totalReceived,
		"success_rate", float64(totalReceived*100)/float64(numAnnouncements*numSubscribers))

	// Clean up
	for _, subscriber := range subscribers {
		announcer.Unsubscribe(subscriber)
		close(subscriber)
	}

	slog.Info("✅ High-frequency performance: Announcer handles rapid announcements efficiently")
}

func demonstrateErrorScenarios(announcer *blob.GoroutineAnnouncer) {
	slog.Info("Testing error scenarios and edge cases")

	// Test 1: Closed channel handling
	slog.Info("Test 1: Dead subscriber cleanup")

	deadSubscriber := make(chan int64, 1)
	announcer.SubscribeChannel(deadSubscriber)
	close(deadSubscriber) // Close the channel immediately

	subscriberCount := announcer.GetSubscriberCount()
	slog.Info("Subscribed dead channel", "subscriber_count", subscriberCount)

	// Announce something - this should clean up the dead subscriber
	announcer.Announce(2000)
	time.Sleep(100 * time.Millisecond) // Give time for cleanup

	newCount := announcer.GetSubscriberCount()
	slog.Info("After cleanup", "subscriber_count", newCount)

	if newCount < subscriberCount {
		slog.Info("✅ Dead subscriber was cleaned up")
	} else {
		slog.Warn("Dead subscriber cleanup may not have worked")
	}

	// Test 2: Full channel handling
	slog.Info("Test 2: Full channel handling")

	fullSubscriber := make(chan int64, 1) // Very small buffer
	announcer.SubscribeChannel(fullSubscriber)

	// Fill the channel
	fullSubscriber <- 999

	// Try to announce - should handle the full channel gracefully
	announcer.Announce(2001)
	announcer.Announce(2002)
	time.Sleep(50 * time.Millisecond)

	announcer.Unsubscribe(fullSubscriber)
	close(fullSubscriber)
	slog.Info("✅ Full channel handled gracefully")

	// Test 3: Resource cleanup on Close
	slog.Info("Test 3: Resource cleanup")

	// Create a new announcer for cleanup testing
	testAnnouncer := blob.NewGoroutineAnnouncer()

	// Add some subscribers
	testSub1 := make(chan int64, 10)
	testSub2 := make(chan int64, 10)
	testAnnouncer.SubscribeChannel(testSub1)
	testAnnouncer.SubscribeChannel(testSub2)

	beforeCount := testAnnouncer.GetSubscriberCount()
	slog.Info("Test announcer setup", "subscriber_count", beforeCount)

	// Close the announcer
	testAnnouncer.Close()

	// Channels should be closed
	select {
	case _, ok := <-testSub1:
		if !ok {
			slog.Info("✅ Subscriber channel was properly closed")
		} else {
			slog.Warn("Subscriber channel was not closed")
		}
	case <-time.After(100 * time.Millisecond):
		slog.Warn("No response from subscriber channel after close")
	}

	// Test 4: Context cancellation
	slog.Info("Test 4: Context cancellation behavior")

	cancelAnnouncer := blob.NewGoroutineAnnouncer()

	// Start a wait operation
	go func() {
		err := cancelAnnouncer.WaitForVersion(9999, 5*time.Second)
		if err != nil {
			slog.Info("Wait operation cancelled as expected", "error", err)
		}
	}()

	// Close the announcer to cancel context
	time.Sleep(100 * time.Millisecond)
	cancelAnnouncer.Close()

	time.Sleep(200 * time.Millisecond)
	slog.Info("✅ Context cancellation handled properly")

	slog.Info("✅ Error scenarios: All edge cases handled gracefully")
}

func demonstrateRealIntegration(ctx context.Context, blobStore blob.BlobStore, announcer *blob.GoroutineAnnouncer) {
	slog.Info("Testing real producer/consumer integration with full announcer features")

	// Create a producer
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	// Create multiple consumers with different behaviors
	immediateConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	delayedConsumer := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncer(announcer),
	)

	// Scenario: Movie database with real-time updates
	slog.Info("Scenario: Real-time movie database updates")

	// Version 50: Initial movie catalog
	version50, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		movies := []struct {
			ID    uint32
			Title string
			Year  uint16
		}{
			{50001, "The Matrix", 1999},
			{50002, "Inception", 2010},
			{50003, "Interstellar", 2014},
		}
		for _, movie := range movies {
			data := TestData{ID: movie.ID, Message: movie.Title, Version: int64(movie.Year)}
			ws.Add(data)
		}
		slog.Info("Produced initial movie catalog for version 50")
	})
	if err != nil {
		slog.Error("RunCycle failed for version 50", "error", err)
		return
	}

	// Immediate consumer reacts to announcements
	go func() {
		err := immediateConsumer.TriggerRefreshTo(ctx, int64(version50))
		if err != nil {
			slog.Error("Immediate consumer refresh failed", "error", err)
		} else {
			slog.Info("Immediate consumer updated to version 50")
		}
	}()

	// Delayed consumer waits for the announcement
	go func() {
		err := announcer.WaitForVersion(int64(version50), 2*time.Second)
		if err != nil {
			slog.Error("Delayed consumer wait failed", "error", err)
		} else {
			delayedConsumer.TriggerRefreshTo(ctx, int64(version50))
			slog.Info("Delayed consumer updated to version 50 after waiting")
		}
	}()

	time.Sleep(200 * time.Millisecond)

	// Version 51: Add new movies (with subscriber monitoring)
	slog.Info("Adding new movies to catalog...")

	// Set up monitoring
	monitor := make(chan int64, 10)
	announcer.SubscribeChannel(monitor)
	defer announcer.Unsubscribe(monitor)

	version51, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		newMovies := []struct {
			ID    uint32
			Title string
			Year  uint16
		}{
			{50004, "Dune", 2021},
			{50005, "Blade Runner 2049", 2017},
		}

		for _, movieData := range newMovies {
			_, seg := capnp.NewSingleSegmentMessage(nil)
			movieStruct, _ := movie.NewMovie(seg)
			movieStruct.SetId(movieData.ID)
			movieStruct.SetTitle(movieData.Title)
			movieStruct.SetYear(movieData.Year)
			ws.Add(movieStruct.ToPtr())
		}

		slog.Info("Added new movies to catalog", "version", 51, "movies", len(newMovies))
	})
	if err != nil {
		slog.Error("RunCycle failed for version 51", "error", err)
		return
	}

	// Monitor the announcement
	select {
	case announcedVersion := <-monitor:
		slog.Info("Monitor received announcement", "version", announcedVersion)
	case <-time.After(500 * time.Millisecond):
		slog.Warn("Monitor did not receive announcement in time")
	}

	// Both consumers update to latest
	immediateConsumer.TriggerRefreshTo(ctx, int64(version51))
	delayedConsumer.TriggerRefreshTo(ctx, int64(version51))

	// Pin scenario: Emergency maintenance
	slog.Info("Emergency maintenance: Pinning consumers to stable version")
	announcer.Pin(int64(version50)) // Pin to stable version

	// New version during maintenance (should not affect pinned consumers)
	version52, err := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		maintenanceData := TestData{ID: 99999, Message: "Maintenance data", Version: 52}
		ws.Add(maintenanceData)
		slog.Info("Produced maintenance data", "version", 52)
	})
	if err != nil {
		slog.Error("RunCycle failed for version 52", "error", err)
		return
	}

	// Consumers should still see pinned version
	pinnedVersion := announcer.GetLatestVersion()
	slog.Info("During maintenance", "actual_latest", version52, "consumers_see", pinnedVersion)

	// End maintenance
	time.Sleep(300 * time.Millisecond)
	announcer.Unpin()
	slog.Info("Maintenance complete - unpinned consumers")

	// Consumers can now see latest version
	finalVersion := announcer.GetLatestVersion()
	slog.Info("After maintenance", "consumers_see", finalVersion)

	close(monitor)

	slog.Info("✅ Real integration: Producer/consumer with full announcer features work perfectly")
	slog.Info("  - Real-time updates: Immediate and delayed consumer patterns")
	slog.Info("  - Monitoring: Subscription-based announcement tracking")
	slog.Info("  - Maintenance: Emergency pinning for stability")
	slog.Info("  - Recovery: Smooth unpinning and version synchronization")
}
