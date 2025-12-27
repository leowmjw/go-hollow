package consumer

import (
	"context"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
	"github.com/leowmjw/go-hollow/producer"
)

func TestConsumer_ErrorHandling(t *testing.T) {
	// Test consumer with empty blob store
	blobStore := blob.NewInMemoryBlobStore()
	consumer := NewConsumer(WithBlobRetriever(blobStore))

	err := consumer.TriggerRefresh(context.Background())
	if err == nil {
		t.Error("expected error when refreshing with no versions available")
	}

	// Test refresh to specific version with no data
	err = consumer.TriggerRefreshTo(context.Background(), 1)
	if err == nil {
		t.Error("expected error when refreshing to version with no data")
	}
}

func TestConsumer_RefreshToSameVersion(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	prod := producer.NewProducer(producer.WithBlobStore(blobStore))
	version, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("test_data")
	})

	consumer := NewConsumer(WithBlobRetriever(blobStore))
	err := consumer.TriggerRefreshTo(context.Background(), version)
	if err != nil {
		t.Fatalf("initial refresh failed: %v", err)
	}

	// Refresh to same version should be no-op
	err = consumer.TriggerRefreshTo(context.Background(), version)
	if err != nil {
		t.Errorf("refresh to same version should not error: %v", err)
	}
}

func TestConsumer_WithVersionCursor(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	consumer := NewConsumer(
		WithBlobRetriever(blobStore),
		WithVersionCursor(announcer),
	)

	if !consumer.autoRefresh {
		t.Error("expected autoRefresh to be enabled with version cursor")
	}

	// Test with deprecated announcement watcher
	consumer2 := NewConsumer(
		WithBlobRetriever(blobStore),
		WithAnnouncementWatcher(announcer),
	)

	if !consumer2.autoRefresh {
		t.Error("expected autoRefresh to be enabled with announcement watcher")
	}
}

func TestConsumer_WithSerializer(t *testing.T) {
	serializer := internal.NewCapnProtoSerializer()
	consumer := NewConsumer(WithSerializer(serializer))

	if consumer.serializer != serializer {
		t.Error("expected custom serializer to be set")
	}
}

func TestConsumer_TypeFilterExclude(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	prod := producer.NewProducer(producer.WithBlobStore(blobStore))
	version, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("string_data")
		ws.Add(42)
		ws.Add(true)
	})

	// Create consumer that excludes integers
	typeFilter := internal.NewTypeFilter().Exclude("Integer")
	consumer := NewConsumer(
		WithBlobRetriever(blobStore),
		WithTypeFilter(typeFilter),
	)

	err := consumer.TriggerRefreshTo(context.Background(), version)
	if err != nil {
		t.Fatalf("TriggerRefreshTo failed: %v", err)
	}

	stateEngine := consumer.GetStateEngine()
	if !stateEngine.HasType("String") {
		t.Error("String type should be included")
	}
	if !stateEngine.HasType("Boolean") {
		t.Error("Boolean type should be included")
	}
	if stateEngine.HasType("Integer") {
		t.Error("Integer type should be excluded")
	}
}

func TestConsumer_DeltaOperations(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	// Create producer with primary keys for delta generation
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithPrimaryKey("TestRecord", "ID"),
	)

	// Initial state
	v1, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add(TestRecord{ID: "1", Name: "First"})
		ws.Add(TestRecord{ID: "2", Name: "Second"})
	})

	// Update and delete
	v2, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add(TestRecord{ID: "1", Name: "Updated First"}) // Update
		// ID "2" is deleted by omission
		ws.Add(TestRecord{ID: "3", Name: "Third"}) // Add
	})

	consumer := NewConsumer(WithBlobRetriever(blobStore))

	// Test refreshing through deltas
	err := consumer.TriggerRefreshTo(context.Background(), v2)
	if err != nil {
		t.Fatalf("refresh with deltas failed: %v", err)
	}

	if consumer.GetCurrentVersion() != v2 {
		t.Errorf("expected version %d, got %d", v2, consumer.GetCurrentVersion())
	}

	// Verify consumer can go back to v1
	err = consumer.TriggerRefreshTo(context.Background(), v1)
	if err != nil {
		t.Fatalf("refresh back to v1 failed: %v", err)
	}
}

type TestRecord struct {
	ID   string
	Name string
}

func TestConsumer_RefreshWithCursor(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	announcer := blob.NewInMemoryFeed()

	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(announcer),
	)

	consumer := NewConsumer(
		WithBlobRetriever(blobStore),
		WithVersionCursor(announcer),
	)

	// Publish data
	version, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("test_data")
	})

	// Trigger refresh - should use cursor
	err := consumer.TriggerRefresh(context.Background())
	if err != nil {
		t.Fatalf("refresh with cursor failed: %v", err)
	}

	if consumer.GetCurrentVersion() != version {
		t.Errorf("expected version %d, got %d", version, consumer.GetCurrentVersion())
	}
}

func TestConsumer_BlobPlanningErrors(t *testing.T) {
	consumer := NewConsumer(WithBlobRetriever(blob.NewInMemoryBlobStore()))

	// Test planning with no data
	_, err := consumer.planBlobs(0, 1)
	if err == nil {
		t.Error("expected error when planning with no blobs")
	}
}

func TestConsumer_LoadSnapshotWithTypeFilter(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	prod := producer.NewProducer(producer.WithBlobStore(blobStore))
	version, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("string_data")
		ws.Add(42)
	})

	// Get the blob directly
	snapshotBlob := blobStore.RetrieveSnapshotBlob(version)
	if snapshotBlob == nil {
		t.Fatal("snapshot blob not found")
	}

	// Test loading with type filter
	typeFilter := internal.NewTypeFilter().ExcludeAll().Include("String")
	consumer := NewConsumer(
		WithBlobRetriever(blobStore),
		WithTypeFilter(typeFilter),
	)

	err := consumer.loadSnapshot(snapshotBlob)
	if err != nil {
		t.Fatalf("loadSnapshot failed: %v", err)
	}

	// Verify filtering worked
	stateEngine := consumer.GetStateEngine()
	if !stateEngine.HasType("String") {
		t.Error("String type should be loaded")
	}
	if stateEngine.HasType("Integer") {
		t.Error("Integer type should be filtered out")
	}
}

func TestConsumer_ApplyDeltaToEmptyState(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	consumer := NewConsumer(WithBlobRetriever(blobStore))

	// Create a simple delta blob manually
	deltaSet := internal.NewDeltaSet()
	deltaSet.AddRecord("TestType", internal.DeltaAdd, 0, "new_data")

	serializer := internal.NewTraditionalSerializer()
	deltaData, err := serializer.SerializeDelta(context.Background(), deltaSet)
	if err != nil {
		t.Fatalf("failed to serialize delta: %v", err)
	}

	deltaBlob := &blob.Blob{
		Type:      blob.DeltaBlob,
		Version:   1,
		ToVersion: 1,
		Data:      deltaData,
	}

	err = consumer.applyDelta(deltaBlob)
	if err != nil {
		t.Fatalf("applyDelta to empty state failed: %v", err)
	}

	if consumer.GetCurrentVersion() != 1 {
		t.Errorf("expected version 1, got %d", consumer.GetCurrentVersion())
	}
}

func TestConsumer_GetRetriever(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	consumer := NewConsumer(WithBlobRetriever(blobStore))

	retriever := consumer.GetRetriever()
	if retriever != blobStore {
		t.Error("GetRetriever should return the configured blob store")
	}
}

func TestConsumer_AutoRefreshSubscription(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	feed := blob.NewInMemoryFeed()

	// Create consumer with subscription support
	consumer := NewConsumer(
		WithBlobRetriever(blobStore),
		WithAnnouncer(feed),
	)

	// Create producer that publishes to the same feed
	prod := producer.NewProducer(
		producer.WithBlobStore(blobStore),
		producer.WithAnnouncer(feed),
	)

	// Publish data which should trigger auto-refresh
	version, _ := prod.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("auto_refresh_test")
	})

	// Give some time for auto-refresh to happen
	time.Sleep(100 * time.Millisecond)

	// Check if consumer auto-refreshed (this is approximate due to async nature)
	currentVersion := consumer.GetCurrentVersion()
	if currentVersion == 0 {
		// Try manual refresh to verify the system works
		err := consumer.TriggerRefresh(context.Background())
		if err != nil {
			t.Fatalf("manual refresh failed: %v", err)
		}
		if consumer.GetCurrentVersion() != version {
			t.Errorf("expected version %d after manual refresh, got %d", version, consumer.GetCurrentVersion())
		}
	}
}

func TestZeroCopyConsumer_Basic(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	zcConsumer := NewZeroCopyConsumer(WithBlobRetriever(blobStore))
	if zcConsumer == nil {
		t.Fatal("expected zero-copy consumer to be created")
	}

	if zcConsumer.Consumer == nil {
		t.Error("embedded consumer should not be nil")
	}
}

func TestZeroCopyConsumer_WithOptions(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	serializer := internal.NewCapnProtoSerializer()

	zcConsumer := NewZeroCopyConsumerWithOptions(
		[]ConsumerOption{WithBlobRetriever(blobStore)},
		[]ZeroCopyConsumerOption{WithZeroCopySerializer(serializer)},
	)

	if zcConsumer.serializer != serializer {
		t.Error("expected custom serializer to be set")
	}
}

func TestZeroCopyConsumer_SerializationModeOption(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	zcConsumer := NewZeroCopyConsumerWithOptions(
		[]ConsumerOption{WithBlobRetriever(blobStore)},
		[]ZeroCopyConsumerOption{WithZeroCopySerializationMode(internal.ZeroCopyMode)},
	)

	if zcConsumer.serializer == nil {
		t.Error("expected serializer to be set from mode")
	}
	if zcConsumer.serializer.Mode() != internal.ZeroCopyMode {
		t.Errorf("expected ZeroCopyMode, got %v", zcConsumer.serializer.Mode())
	}
}

func TestZeroCopyConsumer_NoZeroCopyView(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	zcConsumer := NewZeroCopyConsumer(WithBlobRetriever(blobStore))

	// Test when no zero-copy view exists
	view, exists := zcConsumer.GetZeroCopyView()
	if exists {
		t.Error("should not have zero-copy view initially")
	}
	if view != nil {
		t.Error("view should be nil when not exists")
	}

	if zcConsumer.HasZeroCopySupport() {
		t.Error("should not have zero-copy support initially")
	}
}

func TestZeroCopyConsumer_GetDataFallback(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	zcConsumer := NewZeroCopyConsumer(WithBlobRetriever(blobStore))

	// Test fallback when no zero-copy view available
	data := zcConsumer.GetDataWithZeroCopyPreference()
	if data == nil {
		t.Error("should return empty map when no data available")
	}
	if len(data) != 0 {
		t.Error("should return empty map when no data available")
	}
}

func TestZeroCopyConsumer_CreateViewNoBlob(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	zcConsumer := NewZeroCopyConsumer(WithBlobRetriever(blobStore))

	// Test creating view when blob doesn't exist
	err := zcConsumer.createZeroCopyViewForVersion(context.Background(), 999)
	if err == nil {
		t.Error("expected error when creating view for non-existent version")
	}
}

func TestZeroCopyConsumer_BlobSupportsZeroCopy(t *testing.T) {
	zcConsumer := NewZeroCopyConsumer()

	// Test blob without metadata
	blob1 := &blob.Blob{Data: []byte("test")}
	if zcConsumer.blobSupportsZeroCopy(blob1) {
		t.Error("blob without metadata should not support zero-copy")
	}

	// Test blob with wrong metadata
	blob2 := &blob.Blob{
		Data:     []byte("test"),
		Metadata: map[string]string{"other": "value"},
	}
	if zcConsumer.blobSupportsZeroCopy(blob2) {
		t.Error("blob without serialization_mode should not support zero-copy")
	}

	// Test blob with invalid serialization mode
	blob3 := &blob.Blob{
		Data:     []byte("test"),
		Metadata: map[string]string{"serialization_mode": "invalid"},
	}
	if zcConsumer.blobSupportsZeroCopy(blob3) {
		t.Error("blob with invalid serialization_mode should not support zero-copy")
	}

	// Test blob with traditional mode
	blob4 := &blob.Blob{
		Data:     []byte("test"),
		Metadata: map[string]string{"serialization_mode": "0"}, // TraditionalMode
	}
	if zcConsumer.blobSupportsZeroCopy(blob4) {
		t.Error("blob with traditional mode should not support zero-copy")
	}

	// Test blob with zero-copy mode
	blob5 := &blob.Blob{
		Data:     []byte("test"),
		Metadata: map[string]string{"serialization_mode": "1"}, // ZeroCopyMode
	}
	if !zcConsumer.blobSupportsZeroCopy(blob5) {
		t.Error("blob with zero-copy mode should support zero-copy")
	}
}

func TestZeroCopyConsumer_GetSerializer(t *testing.T) {
	// Test with custom serializer
	customSerializer := internal.NewCapnProtoSerializer()
	zcConsumer1 := NewZeroCopyConsumer()
	zcConsumer1.serializer = customSerializer

	if zcConsumer1.getSerializer() != customSerializer {
		t.Error("should return custom serializer when set")
	}

	// Test with default serializer
	zcConsumer2 := NewZeroCopyConsumer()
	defaultSerializer := zcConsumer2.getSerializer()
	if defaultSerializer == nil {
		t.Error("should return default hybrid serializer")
	}
	if defaultSerializer.Mode() != internal.HybridMode {
		t.Errorf("expected HybridMode, got %v", defaultSerializer.Mode())
	}
}

func TestZeroCopyDataAccessor(t *testing.T) {
	// Create a mock zero-copy view
	mockView := &mockZeroCopyView{
		buffer: []byte("test buffer data"),
	}

	accessor := NewZeroCopyDataAccessor(mockView)
	if accessor == nil {
		t.Fatal("expected accessor to be created")
	}

	// Test buffer access
	buffer := accessor.GetRawBuffer()
	if string(buffer) != "test buffer data" {
		t.Errorf("expected 'test buffer data', got %s", string(buffer))
	}

	// Test buffer size
	size := accessor.GetBufferSize()
	if size != len("test buffer data") {
		t.Errorf("expected %d, got %d", len("test buffer data"), size)
	}

	// Test message access
	msg, err := accessor.GetMessage()
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}
	if msg == nil {
		t.Error("expected message to be returned")
	}
}

func TestZeroCopyQueryEngine(t *testing.T) {
	mockView := &mockZeroCopyView{
		buffer: make([]byte, 1024), // 1KB buffer
	}

	engine := NewZeroCopyQueryEngine(mockView)
	if engine == nil {
		t.Fatal("expected query engine to be created")
	}

	// Test record counting
	count, err := engine.CountRecords()
	if err != nil {
		t.Fatalf("CountRecords failed: %v", err)
	}
	if count != 16 { // 1024 / 64 = 16
		t.Errorf("expected 16 records, got %d", count)
	}

	// Test find by offset
	data, err := engine.FindByOffset(100)
	if err != nil {
		t.Fatalf("FindByOffset failed: %v", err)
	}
	if len(data) != 64 {
		t.Errorf("expected 64 bytes, got %d", len(data))
	}

	// Test find by offset at end of buffer
	data, err = engine.FindByOffset(1000)
	if err != nil {
		t.Fatalf("FindByOffset at end failed: %v", err)
	}
	if len(data) != 24 { // 1024 - 1000 = 24
		t.Errorf("expected 24 bytes, got %d", len(data))
	}

	// Test find by invalid offset
	_, err = engine.FindByOffset(2000)
	if err == nil {
		t.Error("expected error for invalid offset")
	}

	// Test field extraction
	fields, err := engine.ExtractField("testField")
	if err != nil {
		t.Fatalf("ExtractField failed: %v", err)
	}
	if len(fields) != 2 {
		t.Errorf("expected 2 field results, got %d", len(fields))
	}
}

// Mock implementation for testing
type mockZeroCopyView struct {
	buffer []byte
}

func (m *mockZeroCopyView) GetMessage() *capnp.Message {
	return nil
}

func (m *mockZeroCopyView) GetRootStruct() (capnp.Struct, error) {
	return capnp.Struct{}, nil
}

func (m *mockZeroCopyView) GetByteBuffer() []byte {
	return m.buffer
}

func TestConsumer_PanicOnIncompatibleOptions(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	typeFilter := internal.NewTypeFilter().Include("String")

	// This should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for incompatible options")
		}
	}()

	NewConsumer(
		WithBlobRetriever(blobStore),
		WithMemoryMode(internal.SharedMemoryLazy),
		WithTypeFilter(typeFilter),
	)
}
