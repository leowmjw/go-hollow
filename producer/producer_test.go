package producer

import (
	"context"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
	"sync/atomic"
	"testing"
)

// TestProducer_BasicCycle tests basic producer cycle functionality
func TestProducer_BasicCycle(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	producer := NewProducer(WithBlobStore(blobStore))

	// Test empty cycle - should not publish
	version1, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		// empty cycle
	})
	if version1 != 0 {
		t.Errorf("Empty cycle should return version 0, got %d", version1)
	}

	// Test cycle with data
	version2, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("test_value")
	})
	if version2 == 0 {
		t.Error("Cycle with data should return non-zero version")
	}

	// Test cycle with same data - should return same version
	version3, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("test_value")
	})
	if version3 != version2 {
		t.Errorf("Cycle with same data should return same version, got %d, want %d", version3, version2)
	}

	// Test cycle with different data
	version4, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("different_value")
	})
	if version4 <= version3 {
		t.Errorf("Cycle with different data should return higher version, got %d, want > %d", version4, version3)
	}
}

// TestProducer_RestoreAndPublish tests producer restore functionality
func TestProducer_RestoreAndPublish(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	// Create initial producer and publish data
	producer1 := NewProducer(WithBlobStore(blobStore))
	version, _ := producer1.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("test_data")
	})

	// Create new producer and restore
	producer2 := NewProducer(WithBlobStore(blobStore))
	err := producer2.Restore(context.Background(), version, blobStore)
	if err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// Verify restored state
	readState := producer2.GetReadState()
	if readState.GetVersion() != version {
		t.Errorf("Restored version = %d, want %d", readState.GetVersion(), version)
	}
}

// TestProducer_SingleProducerEnforcement tests primary producer enforcement
func TestProducer_SingleProducerEnforcement(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	enforcer := internal.NewSingleProducerEnforcer()

	producer := NewProducer(
		WithBlobStore(blobStore),
		WithSingleProducerEnforcer(enforcer),
	)

	// Test as primary producer
	version1, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data1")
	})
	if version1 == 0 {
		t.Error("Primary producer should publish successfully")
	}

	// Disable enforcement
	enforcer.Disable()

	// Test as non-primary producer
	version2, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data2")
	})
	if version2 != version1 {
		t.Errorf("Non-primary producer should return same version, got %d, want %d", version2, version1)
	}

	// Re-enable enforcement
	enforcer.Enable()

	// Test as primary again
	version3, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data3")
	})
	if version3 <= version2 {
		t.Errorf("Primary producer should return new version, got %d, want > %d", version3, version2)
	}
}

// TestProducer_ValidationFailure tests producer validation behavior
func TestProducer_ValidationFailure(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	validator := &TestValidator{
		shouldFail: &atomic.Bool{},
	}

	producer := NewProducer(
		WithBlobStore(blobStore),
		WithValidator(validator),
	)

	// Test successful validation
	version1, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data1")
	})
	if version1 == 0 {
		t.Error("Successful validation should publish")
	}

	// Test failed validation
	validator.shouldFail.Store(true)

	_, err := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data2")
	})
	if err == nil {
		t.Error("Failed validation should return error")
	}

	// Verify state was rolled back (should still have count from successful cycle)
	populatedCount := producer.GetWriteEngine().GetPopulatedCount()
	if populatedCount != 1 {
		t.Errorf("After rollback, populated count should still be from successful cycle, got %d, want 1", populatedCount)
	}

	// Test recovery after validation failure
	validator.shouldFail.Store(false)

	version3, _ := producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		ws.Add("data3")
	})
	if version3 <= version1 {
		t.Errorf("Recovery should produce new version, got %d, want > %d", version3, version1)
	}
}

// TestProducer_TypeResharding tests automatic type resharding
func TestProducer_TypeResharding(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()

	producer := NewProducer(
		WithBlobStore(blobStore),
		WithTypeResharding(true),
		WithTargetMaxTypeShardSize(32),
	)

	// Create small dataset
	_, _ = producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		for i := 0; i < 50; i++ {
			ws.Add(TestRecord{ID: i, Value: i * 10})
		}
	})

	typeName := internal.GetTypeName(TestRecord{ID: 1, Value: 1})
	initialShards := producer.GetWriteEngine().GetNumShards(typeName)
	if initialShards < 2 {
		t.Errorf("Should have multiple shards with 50 records, got %d", initialShards)
	}

	// Create larger dataset to trigger resharding
	_, _ = producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		for i := 0; i < 100; i++ {
			ws.Add(TestRecord{ID: i, Value: i * 10})
		}
	})

	newShards := producer.GetWriteEngine().GetNumShards(typeName)
	if newShards <= initialShards {
		t.Errorf("Should have more shards after growing dataset, got %d, want > %d", newShards, initialShards)
	}

	// TODO: Test with consumer when consumer is implemented
}

// TestValidator is a test implementation of Validator
type TestValidator struct {
	shouldFail *atomic.Bool
}

func (v *TestValidator) Name() string {
	return "TestValidator"
}

func (v *TestValidator) Validate(readState *internal.ReadState) internal.ValidationResult {
	if v.shouldFail.Load() {
		return internal.ValidationResult{
			Type:    internal.ValidationFailed,
			Message: "Test validation failure",
		}
	}
	return internal.ValidationResult{
		Type:    internal.ValidationPassed,
		Message: "Test validation passed",
	}
}
