package hollow

import (
	"fmt"
	"testing"
)

func TestDeltaStorage(t *testing.T) {
	ds := NewDeltaStorage()
	
	// Create test delta
	delta := &DataDiff{
		added:   []string{"key1", "key2"},
		removed: []string{"key3"},
		changed: []string{"key4"},
	}
	
	// Store delta
	err := ds.StoreDelta(1, 2, delta)
	if err != nil {
		t.Fatalf("Failed to store delta: %v", err)
	}
	
	// Retrieve delta
	retrievedDelta, err := ds.GetDelta(1, 2)
	if err != nil {
		t.Fatalf("Failed to get delta: %v", err)
	}
	
	// Verify delta content
	if len(retrievedDelta.added) != 2 {
		t.Errorf("Expected 2 added keys, got %d", len(retrievedDelta.added))
	}
	if len(retrievedDelta.removed) != 1 {
		t.Errorf("Expected 1 removed key, got %d", len(retrievedDelta.removed))
	}
	if len(retrievedDelta.changed) != 1 {
		t.Errorf("Expected 1 changed key, got %d", len(retrievedDelta.changed))
	}
	
	// Test metadata
	metadata, err := ds.GetDeltaMetadata(2)
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}
	
	if metadata.FromVersion != 1 {
		t.Errorf("Expected from version 1, got %d", metadata.FromVersion)
	}
	if metadata.ToVersion != 2 {
		t.Errorf("Expected to version 2, got %d", metadata.ToVersion)
	}
	if metadata.ChangeCount != 4 {
		t.Errorf("Expected change count 4, got %d", metadata.ChangeCount)
	}
}

func TestDeltaProducer(t *testing.T) {
	// This test is simplified since we need a proper blob store integration
	// In a real implementation, we'd test with actual state changes
	
	deltaProducer := NewDeltaProducer()
	
	// Test that delta producer is created
	if deltaProducer == nil {
		t.Fatal("Delta producer should not be nil")
	}
	
	if deltaProducer.deltaStorage == nil {
		t.Fatal("Delta storage should not be nil")
	}
}

func TestDeltaConsumer(t *testing.T) {
	// This test is simplified since we need a proper consumer integration
	// In a real implementation, we'd test with actual delta applications
	
	deltaConsumer := NewDeltaConsumer()
	
	// Test that delta consumer is created
	if deltaConsumer == nil {
		t.Fatal("Delta consumer should not be nil")
	}
	
	if deltaConsumer.deltaStorage == nil {
		t.Fatal("Delta storage should not be nil")
	}
}

func TestDeltaValidation(t *testing.T) {
	// Test valid delta
	validDelta := &DataDiff{
		added:   []string{"key1", "key2"},
		removed: []string{"key3"},
		changed: []string{"key4"},
	}
	
	err := ValidateDelta(validDelta)
	if err != nil {
		t.Errorf("Valid delta should not fail validation: %v", err)
	}
	
	// Test nil delta
	err = ValidateDelta(nil)
	if err == nil {
		t.Error("Nil delta should fail validation")
	}
	
	// Test delta with duplicate keys
	invalidDelta := &DataDiff{
		added:   []string{"key1", "key1"},
		removed: []string{"key2"},
		changed: []string{"key3"},
	}
	
	err = ValidateDelta(invalidDelta)
	if err == nil {
		t.Error("Delta with duplicate keys should fail validation")
	}
}

func TestDeltaCompression(t *testing.T) {
	delta := &DataDiff{
		added:   []string{"key1", "key2"},
		removed: []string{"key3"},
		changed: []string{"key4"},
	}
	
	// Test compression
	compressed, err := CompressDelta(delta)
	if err != nil {
		t.Errorf("Delta compression failed: %v", err)
	}
	
	// Test decompression
	decompressed, err := DecompressDelta(compressed)
	if err != nil {
		t.Errorf("Delta decompression failed: %v", err)
	}
	
	// Verify content preserved
	if len(decompressed.added) != len(delta.added) {
		t.Error("Delta compression/decompression altered content")
	}
}

func TestDeltaOptions(t *testing.T) {
	options := DefaultDeltaOptions()
	
	if options.MaxDeltaSize <= 0 {
		t.Error("MaxDeltaSize should be positive")
	}
	if options.MaxDeltaChain <= 0 {
		t.Error("MaxDeltaChain should be positive")
	}
	if options.DeltaRetentionTime <= 0 {
		t.Error("DeltaRetentionTime should be positive")
	}
}

func TestDeltaApply(t *testing.T) {
	// Create base data
	baseData := map[string]any{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	
	// Create delta
	delta := &DataDiff{
		added:   []string{"key4"},
		removed: []string{"key2"},
		changed: []string{"key1"},
	}
	
	// Create new data (what the result should be)
	newData := map[string]any{
		"key1": "new_value1",
		"key3": "value3",
		"key4": "value4",
	}
	
	// Apply delta
	result := make(map[string]any)
	for k, v := range baseData {
		result[k] = v
	}
	
	delta.Apply(result, newData)
	
	// Verify result
	if len(result) != 3 {
		t.Errorf("Expected 3 keys after delta application, got %d", len(result))
	}
	
	if result["key1"] != "new_value1" {
		t.Errorf("Expected key1 to be changed to 'new_value1', got %v", result["key1"])
	}
	
	if _, exists := result["key2"]; exists {
		t.Error("Expected key2 to be removed")
	}
	
	if result["key4"] != "value4" {
		t.Errorf("Expected key4 to be added with value 'value4', got %v", result["key4"])
	}
}

func TestDeltaMetadata(t *testing.T) {
	ds := NewDeltaStorage()
	
	delta := &DataDiff{
		added:   []string{"key1", "key2"},
		removed: []string{"key3"},
		changed: []string{"key4"},
	}
	
	// Store delta
	err := ds.StoreDelta(1, 2, delta)
	if err != nil {
		t.Fatalf("Failed to store delta: %v", err)
	}
	
	// Get metadata
	metadata, err := ds.GetDeltaMetadata(2)
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}
	
	// Verify metadata
	if metadata.FromVersion != 1 {
		t.Errorf("Expected from version 1, got %d", metadata.FromVersion)
	}
	if metadata.ToVersion != 2 {
		t.Errorf("Expected to version 2, got %d", metadata.ToVersion)
	}
	if metadata.ChangeCount != 4 {
		t.Errorf("Expected change count 4, got %d", metadata.ChangeCount)
	}
	if metadata.Size <= 0 {
		t.Error("Expected positive size")
	}
	if metadata.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestDeltaHistory(t *testing.T) {
	ds := NewDeltaStorage()
	
	// Store multiple deltas
	for i := 1; i <= 5; i++ {
		delta := &DataDiff{
			added:   []string{},
			removed: []string{},
			changed: []string{},
		}
		
		err := ds.StoreDelta(uint64(i), uint64(i+1), delta)
		if err != nil {
			t.Fatalf("Failed to store delta %d: %v", i, err)
		}
	}
	
	// Get history
	history := ds.ListDeltas()
	if len(history) != 5 {
		t.Errorf("Expected 5 deltas in history, got %d", len(history))
	}
	
	// Verify keys
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("%d_%d", i, i+1)
		if _, exists := history[key]; !exists {
			t.Errorf("Expected delta key %s to exist in history", key)
		}
	}
}

func BenchmarkDeltaStorage(b *testing.B) {
	ds := NewDeltaStorage()
	
	delta := &DataDiff{
		added:   []string{"key1", "key2"},
		removed: []string{"key3"},
		changed: []string{"key4"},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		err := ds.StoreDelta(1, uint64(i+2), delta)
		if err != nil {
			b.Fatalf("Failed to store delta: %v", err)
		}
	}
}

func BenchmarkDeltaRetrieval(b *testing.B) {
	ds := NewDeltaStorage()
	
	delta := &DataDiff{
		added:   []string{"key1", "key2"},
		removed: []string{"key3"},
		changed: []string{"key4"},
	}
	
	// Store delta
	err := ds.StoreDelta(1, 2, delta)
	if err != nil {
		b.Fatalf("Failed to store delta: %v", err)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := ds.GetDelta(1, 2)
		if err != nil {
			b.Fatalf("Failed to get delta: %v", err)
		}
	}
}
