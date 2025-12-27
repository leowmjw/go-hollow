package producer

import (
	"context"
	"github.com/leowmjw/go-hollow/blob"
	"github.com/leowmjw/go-hollow/internal"
	"testing"
)

// TestWriteStateEngine_HeaderTags tests header tag management
func TestWriteStateEngine_HeaderTags(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	producer := NewProducer(WithBlobStore(blobStore))

	// Test header tags
	writeEngine := producer.GetWriteEngine()
	writeEngine.SetHeaderTag("test", "1")

	tags := writeEngine.GetHeaderTags()
	if tags["test"] != "1" {
		t.Errorf("Expected header tag test=1, got %v", tags["test"])
	}
}

// TestWriteStateEngine_Resharding tests type resharding functionality
func TestWriteStateEngine_Resharding(t *testing.T) {
	blobStore := blob.NewInMemoryBlobStore()
	producer := NewProducer(
		WithBlobStore(blobStore),
		WithTypeResharding(true),
		WithTargetMaxTypeShardSize(10),
	)

	// Test resharding
	_, _ = producer.RunCycle(context.Background(), func(ws *internal.WriteState) {
		for i := 0; i < 20; i++ {
			ws.Add(TestRecord{ID: i, Value: i * 10})
		}
	})

	writeEngine := producer.GetWriteEngine()
	typeName := internal.GetTypeName(TestRecord{ID: 1, Value: 1})
	shards := writeEngine.GetNumShards(typeName)
	if shards < 2 {
		t.Errorf("Expected multiple shards for large dataset, got %d", shards)
	}
}

// TestWriteStateEngine_ResetToLastPrepared tests state rollback functionality
func TestWriteStateEngine_ResetToLastPrepared(t *testing.T) {
	writeEngine := internal.NewWriteStateEngine()

	// Increment count
	writeEngine.IncrementPopulatedCount()
	writeEngine.IncrementPopulatedCount()

	count := writeEngine.GetPopulatedCount()
	if count != 2 {
		t.Errorf("Expected populated count 2, got %d", count)
	}

	// Reset count
	writeEngine.ResetPopulatedCount()

	count = writeEngine.GetPopulatedCount()
	if count != 0 {
		t.Errorf("Expected populated count 0 after reset, got %d", count)
	}
}
