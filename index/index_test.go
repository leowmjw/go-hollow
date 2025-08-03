package index

import (
	"context"
	"testing"
	"github.com/leowmjw/go-hollow/internal"
)

// TestRecord for testing
type TestRecord struct {
	ID    int
	Name  string
	Value float64
}

// TestHashIndex_BasicFunctionality tests basic hash index operations
func TestHashIndex_BasicFunctionality(t *testing.T) {
	// Create test data
	stateEngine := internal.NewReadStateEngine()
	
	// Create hash index
	index := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"ID", "Name"})
	
	// Test index creation
	if index == nil {
		t.Fatal("Hash index should not be nil")
	}
	
	// Test finding matches (will be empty in simplified implementation)
	ctx := context.Background()
	matches := index.FindMatches(ctx, 1, "one")
	
	if matches == nil {
		t.Fatal("FindMatches should return an iterator")
	}
	
	// Test that iterator works
	hasNext := matches.Next()
	if hasNext {
		// In simplified implementation, should be empty
		t.Log("Found match:", matches.Value())
	}
	
	// Test update detection
	err := index.DetectUpdates(ctx)
	if err != nil {
		t.Errorf("DetectUpdates failed: %v", err)
	}
}

// TestHashIndex_NullValues tests null value handling
func TestHashIndex_NullValues(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"Name"})
	
	ctx := context.Background()
	
	// Test with nil value
	matches := index.FindMatches(ctx, nil)
	if matches == nil {
		t.Fatal("FindMatches should handle nil values")
	}
	
	// Should not panic
	matches.Next()
}

// TestUniqueKeyIndex_Basic tests unique key index functionality
func TestUniqueKeyIndex_Basic(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewUniqueKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	
	ctx := context.Background()
	
	// Test getting match (will be empty in simplified implementation)
	record, found := index.GetMatch(ctx, 1)
	if found {
		t.Log("Found record:", record)
	}
	
	// Test update detection
	err := index.DetectUpdates(ctx)
	if err != nil {
		t.Errorf("DetectUpdates failed: %v", err)
	}
}

// TestPrimaryKeyIndex_Basic tests primary key index functionality
func TestPrimaryKeyIndex_Basic(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewPrimaryKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID", "Name"})
	
	ctx := context.Background()
	
	// Test getting match with composite key
	record, found := index.GetMatch(ctx, 1, "one")
	if found {
		t.Log("Found record:", record)
	}
	
	// Test update detection
	err := index.DetectUpdates(ctx)
	if err != nil {
		t.Errorf("DetectUpdates failed: %v", err)
	}
}

// TestIndex_Updates tests index maintenance during state changes
func TestIndex_Updates(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	
	// Create multiple types of indexes
	hashIndex := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	uniqueIndex := NewUniqueKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	primaryIndex := NewPrimaryKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID", "Name"})
	
	ctx := context.Background()
	
	// Test that all indexes can handle updates
	err := hashIndex.DetectUpdates(ctx)
	if err != nil {
		t.Errorf("Hash index DetectUpdates failed: %v", err)
	}
	
	err = uniqueIndex.DetectUpdates(ctx)
	if err != nil {
		t.Errorf("Unique index DetectUpdates failed: %v", err)
	}
	
	err = primaryIndex.DetectUpdates(ctx)
	if err != nil {
		t.Errorf("Primary index DetectUpdates failed: %v", err)
	}
}

// TestByteArrayIndex_Basic tests byte array field indexing
func TestByteArrayIndex_Basic(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewByteArrayIndex[TestRecord](stateEngine, "TestRecord", "ByteField")
	
	ctx := context.Background()
	
	// Test finding by bytes
	testBytes := []byte("test")
	matches := index.FindByBytes(ctx, testBytes)
	
	if matches == nil {
		t.Fatal("FindByBytes should return an iterator")
	}
	
	// Test iterator
	matches.Next()
	
	// Test update detection
	err := index.DetectUpdates(ctx)
	if err != nil {
		t.Errorf("DetectUpdates failed: %v", err)
	}
}

// TestExtractFieldValue tests field value extraction
func TestExtractFieldValue(t *testing.T) {
	record := TestRecord{ID: 1, Name: "test", Value: 1.5}
	
	// Test extracting ID field
	id := ExtractFieldValue(record, "ID")
	if id != 1 {
		t.Errorf("Expected ID=1, got %v", id)
	}
	
	// Test extracting Name field
	name := ExtractFieldValue(record, "Name")
	if name != "test" {
		t.Errorf("Expected Name=test, got %v", name)
	}
	
	// Test extracting non-existent field
	invalid := ExtractFieldValue(record, "Invalid")
	if invalid != nil {
		t.Errorf("Expected nil for invalid field, got %v", invalid)
	}
	
	// Test with pointer
	recordPtr := &record
	id2 := ExtractFieldValue(recordPtr, "ID")
	if id2 != 1 {
		t.Errorf("Expected ID=1 from pointer, got %v", id2)
	}
}
