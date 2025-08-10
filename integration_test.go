package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/consumer"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

// TestGoroutineAnnouncer tests the goroutine-based announcer
func TestGoroutineAnnouncer(t *testing.T) {
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()
	
	// Test basic announcement
	err := announcer.Announce(1)
	if err != nil {
		t.Fatalf("Failed to announce: %v", err)
	}
	
	// Wait a bit for the announcement to be processed
	time.Sleep(100 * time.Millisecond)
	
	version := announcer.GetLatestVersion()
	if version != 1 {
		t.Errorf("Expected version 1, got %d", version)
	}
	
	// Test subscription
	ch := make(chan int64, 10)
	announcer.SubscribeChannel(ch)
	
	// Announce a new version
	err = announcer.Announce(2)
	if err != nil {
		t.Fatalf("Failed to announce: %v", err)
	}
	
	// Wait for notification
	select {
	case receivedVersion := <-ch:
		if receivedVersion != 2 {
			t.Errorf("Expected to receive version 2, got %d", receivedVersion)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for announcement")
	}
	
	// Test pinning
	announcer.Pin(1)
	if !announcer.IsPinned() {
		t.Error("Should be pinned")
	}
	if announcer.GetLatestVersion() != 1 {
		t.Error("Should return pinned version")
	}
	
	announcer.Unpin()
	if announcer.IsPinned() {
		t.Error("Should not be pinned")
	}
	
	// Test wait for version
	go func() {
		time.Sleep(200 * time.Millisecond)
		announcer.Announce(5)
	}()
	
	err = announcer.WaitForVersion(5, 1*time.Second)
	if err != nil {
		t.Errorf("Failed to wait for version: %v", err)
	}
	
	announcer.Unsubscribe(ch)
	close(ch)
}

// TestS3BlobStore tests the S3 blob store (cache-only mode)
func TestS3BlobStore(t *testing.T) {
	// This will use cache-only mode if MinIO is not running
	store, err := blob.NewLocalS3BlobStore()
	if err != nil {
		t.Skipf("S3 blob store not available: %v", err)
	}
	
	ctx := context.Background()
	
	// Create a test blob
	testBlob := &blob.Blob{
		Type:    blob.SnapshotBlob,
		Version: 1,
		Data:    []byte("test data"),
	}
	
	// Store the blob
	err = store.Store(ctx, testBlob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}
	
	// Retrieve the blob
	retrievedBlob := store.RetrieveSnapshotBlob(1)
	if retrievedBlob == nil {
		t.Fatal("Failed to retrieve blob")
	}
	
	if retrievedBlob.Version != 1 {
		t.Errorf("Expected version 1, got %d", retrievedBlob.Version)
	}
	
	if string(retrievedBlob.Data) != "test data" {
		t.Errorf("Expected 'test data', got %s", string(retrievedBlob.Data))
	}
	
	// List versions
	versions := store.ListVersions()
	if len(versions) != 1 || versions[0] != 1 {
		t.Errorf("Expected [1], got %v", versions)
	}
	
	// Test removal
	err = store.RemoveSnapshot(1)
	if err != nil {
		t.Fatalf("Failed to remove snapshot: %v", err)
	}
	
	// Should not be retrievable after removal
	retrievedBlob = store.RetrieveSnapshotBlob(1)
	if retrievedBlob != nil {
		t.Error("Blob should be removed")
	}
}

// TestEndToEndIntegration tests the complete integration
func TestEndToEndIntegration(t *testing.T) {
	// Use in-memory store for reliable testing
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()
	
	// Create producer
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)
	
	// Create consumer without auto-refresh for controlled testing
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		// Disable auto-refresh for manual refresh control in tests
	)
	
	ctx := context.Background()
	
	// Subscribe to announcements for deterministic testing
	announcementCh := make(chan int64, 10)
	announcer.SubscribeChannel(announcementCh)
	defer func() {
		announcer.Unsubscribe(announcementCh)
		close(announcementCh)
	}()
	
	// Produce some data
	version1 := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add("data1")
		ws.Add("data2")
	})
	
	if version1 == 0 {
		t.Fatal("Producer should return non-zero version")
	}
	
	// Wait for announcement instead of arbitrary sleep
	select {
	case receivedVersion := <-announcementCh:
		if receivedVersion != version1 {
			t.Errorf("Expected announcement for version %d, got %d", version1, receivedVersion)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for version1 announcement")
	}
	
	// Consumer should refresh to version1 manually
	err := cons.TriggerRefreshTo(ctx, version1)
	if err != nil {
		t.Fatalf("Consumer refresh failed: %v", err)
	}
	
	if cons.GetCurrentVersion() != version1 {
		t.Errorf("Expected consumer version %d, got %d", version1, cons.GetCurrentVersion())
	}
	
	// Produce more data
	version2 := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add("data3")
		ws.Add("data4")
	})
	
	// Wait for announcement deterministically
	select {
	case receivedVersion := <-announcementCh:
		if receivedVersion != version2 {
			t.Errorf("Expected announcement for version %d, got %d", version2, receivedVersion)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for version2 announcement")
	}
	
	// Verify announcer has the latest version
	if announcer.GetLatestVersion() != version2 {
		t.Errorf("Expected announcer version %d, got %d", version2, announcer.GetLatestVersion())
	}
	
	// Test pinning functionality - consumer should still be at version1
	announcer.Pin(version1)
	
	// Verify pinning works by checking announcer state
	if !announcer.IsPinned() {
		t.Error("Announcer should be pinned")
	}
	
	if announcer.GetLatestVersion() != version1 {
		t.Errorf("Expected pinned version %d, got %d", version1, announcer.GetLatestVersion())
	}
	
	// Unpin and verify announcer goes back to latest
	announcer.Unpin()
	if announcer.IsPinned() {
		t.Error("Announcer should not be pinned")
	}
	
	if announcer.GetLatestVersion() != version2 {
		t.Errorf("Expected unpinned latest version %d, got %d", version2, announcer.GetLatestVersion())
	}
	
	// Manually refresh consumer to version2 to complete the test
	err = cons.TriggerRefreshTo(ctx, version2)
	if err != nil {
		t.Fatalf("Consumer refresh to version2 failed: %v", err)
	}
	
	if cons.GetCurrentVersion() != version2 {
		t.Errorf("Expected consumer to be at latest version %d after unpin, got %d", version2, cons.GetCurrentVersion())
	}
}

// TestConcurrentProducerConsumer tests concurrent access
func TestConcurrentProducerConsumer(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()
	
	// Create producer and consumer
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)
	
	cons := consumer.NewConsumer(
		consumer.WithBlobRetriever(blobStore),
		consumer.WithAnnouncementWatcher(announcer),
	)
	
	ctx := context.Background()
	
	// Use WaitGroup for proper synchronization
	var wg sync.WaitGroup
	const numCycles = 5
	versions := make([]int64, numCycles)
	
	for i := 0; i < numCycles; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			versions[index] = prod.RunCycle(ctx, func(ws *internal.WriteState) {
				ws.Add(fmt.Sprintf("concurrent_data_%d", index))
			})
		}(i)
	}
	
	// Wait for all cycles to complete
	wg.Wait()
	
	// Consumer should be able to refresh to latest
	err := cons.TriggerRefresh(ctx)
	if err != nil {
		t.Fatalf("Consumer refresh failed: %v", err)
	}
	
	finalVersion := cons.GetCurrentVersion()
	if finalVersion == 0 {
		t.Error("Consumer should have a non-zero version")
	}
	
	// Verify we have multiple versions stored
	storedVersions := blobStore.ListVersions()
	if len(storedVersions) == 0 {
		t.Error("Should have stored versions")
	}
	
	t.Logf("Stored versions: %v, Final consumer version: %d", storedVersions, finalVersion)
}

// TestBlobStoreErrorHandling tests error scenarios using function replacement
func TestBlobStoreErrorHandling(t *testing.T) {
	// Create a test blob store that can simulate failures
	testStore := &TestBlobStore{
		store:      make(map[string]*blob.Blob),
		shouldFail: false,
	}
	
	announcer := blob.NewGoroutineAnnouncer()
	defer announcer.Close()
	
	// Create producer with the test store
	prod := producer.NewProducer(
		producer.WithBlobStore(testStore),
		producer.WithAnnouncer(announcer),
	)
	
	ctx := context.Background()
	
	// Test successful storage first
	version1 := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add("test_data")
	})
	
	if version1 == 0 {
		t.Fatal("First cycle should succeed")
	}
	
	// Now simulate storage failure
	testStore.shouldFail = true
	
	// This should fail and return an error
	err := prod.RunCycleWithError(ctx, func(ws *internal.WriteState) {
		ws.Add("failing_data")
	})
	
	if err == nil {
		t.Fatal("Expected error when blob store fails")
	}
	
	// Verify the error message contains blob store failure
	if !strings.Contains(err.Error(), "failed to store blob") {
		t.Errorf("Expected 'failed to store blob' in error message, got: %v", err)
	}
	
	// Verify producer state wasn't corrupted by the failure
	if prod.GetReadState().GetVersion() != version1 {
		t.Errorf("Producer version should remain at %d after failure, got %d", version1, prod.GetReadState().GetVersion())
	}
	
	// Test recovery after fixing the store
	testStore.shouldFail = false
	
	version2 := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add("recovery_data")
	})
	
	if version2 <= version1 {
		t.Errorf("Recovery should produce new version, got %d, want > %d", version2, version1)
	}
}

// TestBlobStore is a test implementation that can simulate failures
type TestBlobStore struct {
	store      map[string]*blob.Blob
	mu         sync.RWMutex
	shouldFail bool
}

func (t *TestBlobStore) Store(ctx context.Context, b *blob.Blob) error {
	if t.shouldFail {
		return fmt.Errorf("simulated blob store failure")
	}
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	key := fmt.Sprintf("%d-%d", b.Type, b.Version)
	t.store[key] = b
	return nil
}

func (t *TestBlobStore) RetrieveSnapshotBlob(version int64) *blob.Blob {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	key := fmt.Sprintf("%d-%d", blob.SnapshotBlob, version)
	return t.store[key]
}

func (t *TestBlobStore) RetrieveDeltaBlob(fromVersion int64) *blob.Blob {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	key := fmt.Sprintf("%d-%d", blob.DeltaBlob, fromVersion)
	return t.store[key]
}

func (t *TestBlobStore) RetrieveReverseBlob(toVersion int64) *blob.Blob {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	key := fmt.Sprintf("%d-%d", blob.ReverseBlob, toVersion)
	return t.store[key]
}

func (t *TestBlobStore) RemoveSnapshot(version int64) error {
	if t.shouldFail {
		return fmt.Errorf("simulated removal failure")
	}
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	key := fmt.Sprintf("%d-%d", blob.SnapshotBlob, version)
	delete(t.store, key)
	return nil
}

func (t *TestBlobStore) ListVersions() []int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	versions := make([]int64, 0)
	for key, b := range t.store {
		if b.Type == blob.SnapshotBlob {
			versions = append(versions, b.Version)
		}
		_ = key // avoid unused variable
	}
	return versions
}

// TestAnnouncerErrorHandling tests error scenarios with announcer failures
func TestAnnouncerErrorHandling(t *testing.T) {
	// Create a test announcer that can simulate failures
	testAnnouncer := &TestAnnouncer{
		shouldFail: false,
		version:    0,
	}
	
	blobStore := blob.NewInMemoryBlobStore()
	
	// Create producer with the test announcer
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(testAnnouncer),
	)
	
	ctx := context.Background()
	
	// Test successful announcement first
	version1 := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add("test_data")
	})
	
	if version1 == 0 {
		t.Fatal("First cycle should succeed")
	}
	
	if testAnnouncer.version != version1 {
		t.Errorf("Announcer should have version %d, got %d", version1, testAnnouncer.version)
	}
	
	// Now simulate announcer failure
	testAnnouncer.shouldFail = true
	
	// This should still succeed (announcer failure shouldn't break the producer)
	version2 := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add("more_data")
	})
	
	if version2 <= version1 {
		t.Errorf("Producer should continue working despite announcer failure, got %d, want > %d", version2, version1)
	}
	
	// Verify the announcer wasn't updated due to failure
	if testAnnouncer.version != version1 {
		t.Errorf("Announcer version should remain at %d due to failure, got %d", version1, testAnnouncer.version)
	}
	
	// Test recovery after fixing the announcer
	testAnnouncer.shouldFail = false
	
	version3 := prod.RunCycle(ctx, func(ws *internal.WriteState) {
		ws.Add("recovery_data")
	})
	
	if version3 <= version2 {
		t.Errorf("Recovery should produce new version, got %d, want > %d", version3, version2)
	}
	
	if testAnnouncer.version != version3 {
		t.Errorf("Announcer should be updated after recovery, got %d, want %d", testAnnouncer.version, version3)
	}
}

// TestAnnouncer is a test implementation that can simulate failures
type TestAnnouncer struct {
	mu         sync.Mutex
	shouldFail bool
	version    int64
	pinned     bool
	pinnedVer  int64
}

func (t *TestAnnouncer) Announce(version int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.shouldFail {
		return fmt.Errorf("simulated announcer failure")
	}
	
	t.version = version
	return nil
}

// AnnouncementWatcher interface methods
func (t *TestAnnouncer) GetLatestVersion() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pinned {
		return t.pinnedVer
	}
	return t.version
}

func (t *TestAnnouncer) Pin(version int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pinned = true
	t.pinnedVer = version
}

func (t *TestAnnouncer) Unpin() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pinned = false
	t.pinnedVer = 0
}

func (t *TestAnnouncer) IsPinned() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pinned
}

func (t *TestAnnouncer) GetPinnedVersion() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pinnedVer
}

func (t *TestAnnouncer) Subscribe(ch chan int64) {
	// Simple test implementation - doesn't actually subscribe
}

func (t *TestAnnouncer) Unsubscribe(ch chan int64) {
	// Simple test implementation - doesn't actually unsubscribe
}
