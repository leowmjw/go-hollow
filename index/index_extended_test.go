package index

import (
	"context"
	"testing"

	"github.com/leowmjw/go-hollow/internal"
)

// Additional comprehensive tests for index module

func TestSliceIterator_BasicOperations(t *testing.T) {
	testData := []string{"one", "two", "three"}
	iter := NewSliceIterator(testData)
	
	if iter == nil {
		t.Fatal("NewSliceIterator should not return nil")
	}
	
	// Test iteration
	count := 0
	values := make([]string, 0)
	for iter.Next() {
		values = append(values, iter.Value())
		count++
	}
	
	if count != 3 {
		t.Errorf("expected 3 items, got %d", count)
	}
	
	if len(values) != 3 {
		t.Errorf("expected 3 values, got %d", len(values))
	}
	
	for i, expected := range testData {
		if values[i] != expected {
			t.Errorf("expected %s at index %d, got %s", expected, i, values[i])
		}
	}
	
	// Test close
	err := iter.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

func TestSliceIterator_EmptySlice(t *testing.T) {
	emptyData := []int{}
	iter := NewSliceIterator(emptyData)
	
	// Should not have any values
	if iter.Next() {
		t.Error("empty iterator should not have Next() == true")
	}
	
	// Value should return zero value
	value := iter.Value()
	if value != 0 {
		t.Errorf("expected zero value (0), got %v", value)
	}
}

func TestSliceIterator_OutOfBounds(t *testing.T) {
	testData := []string{"only_one"}
	iter := NewSliceIterator(testData)
	
	// First value should be accessible
	if !iter.Next() {
		t.Fatal("should have first value")
	}
	
	first := iter.Value()
	if first != "only_one" {
		t.Errorf("expected 'only_one', got %s", first)
	}
	
	// Second Next() should return false
	if iter.Next() {
		t.Error("should not have second value")
	}
	
	// Value() should return zero value when out of bounds
	second := iter.Value()
	if second != "" {
		t.Errorf("expected empty string for out of bounds, got %s", second)
	}
}

func TestHashIndex_KeyCreation(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"ID", "Name"})
	
	hashIdx := index.(*hashIndex[TestRecord])
	
	// Test single value key
	key1 := hashIdx.createKey(1)
	if key1 != "1" {
		t.Errorf("expected '1', got %s", key1)
	}
	
	// Test multiple value key
	key2 := hashIdx.createKey(1, "test", 2.5)
	if key2 != "1|test|2.5" {
		t.Errorf("expected '1|test|2.5', got %s", key2)
	}
	
	// Test empty key
	key3 := hashIdx.createKey()
	if key3 != "" {
		t.Errorf("expected empty string, got %s", key3)
	}
	
	// Test nil values
	key4 := hashIdx.createKey(nil, "test", nil)
	if key4 != "<nil>|test|<nil>" {
		t.Errorf("expected '<nil>|test|<nil>', got %s", key4)
	}
}

func TestHashIndex_FindMatches_Multiple(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"Category"})
	
	ctx := context.Background()
	
	// Test finding with different numbers of values
	matches1 := index.FindMatches(ctx, "category1")
	if matches1 == nil {
		t.Fatal("FindMatches should return iterator")
	}
	
	matches2 := index.FindMatches(ctx, "category1", "subcategory")
	if matches2 == nil {
		t.Fatal("FindMatches should handle multiple values")
	}
	
	// Should not panic with many values
	matches3 := index.FindMatches(ctx, 1, 2, 3, 4, 5)
	if matches3 == nil {
		t.Fatal("FindMatches should handle many values")
	}
}

func TestUniqueKeyIndex_EdgeCases(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewUniqueKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	
	ctx := context.Background()
	
	// Test with various key types
	testCases := []interface{}{
		1,
		"string_key",
		1.5,
		true,
		nil,
		[]byte("bytes"),
	}
	
	for _, testCase := range testCases {
		_, found := index.GetMatch(ctx, testCase)
		if found {
			t.Logf("unexpectedly found match for %v", testCase)
		}
	}
}

func TestPrimaryKeyIndex_CompositeKeyCreation(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewPrimaryKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID", "Name", "Category"})
	
	pkIdx := index.(*primaryKeyIndex[TestRecord])
	
	// Test composite key creation
	key1 := pkIdx.createCompositeKey(1, "test", "category")
	expected1 := "1|test|category"
	if key1 != expected1 {
		t.Errorf("expected %s, got %s", expected1, key1)
	}
	
	// Test with different types
	key2 := pkIdx.createCompositeKey(42, true, 3.14)
	expected2 := "42|true|3.14"
	if key2 != expected2 {
		t.Errorf("expected %s, got %s", expected2, key2)
	}
	
	// Test with single key
	key3 := pkIdx.createCompositeKey("single")
	if key3 != "single" {
		t.Errorf("expected 'single', got %s", key3)
	}
	
	// Test with no keys
	key4 := pkIdx.createCompositeKey()
	if key4 != "" {
		t.Errorf("expected empty string, got %s", key4)
	}
}

func TestExtractFieldValue_EdgeCases(t *testing.T) {
	// Test with non-struct types
	result1 := ExtractFieldValue("not a struct", "Field")
	if result1 != nil {
		t.Errorf("expected nil for non-struct, got %v", result1)
	}
	
	result2 := ExtractFieldValue(123, "Field")
	if result2 != nil {
		t.Errorf("expected nil for non-struct, got %v", result2)
	}
	
	// Test with nil pointer
	var nilPtr *TestRecord
	result3 := ExtractFieldValue(nilPtr, "ID")
	if result3 != nil {
		t.Errorf("expected nil for nil pointer, got %v", result3)
	}
	
	// Test with nested struct
	type NestedRecord struct {
		Inner TestRecord
		Outer string
	}
	
	nested := NestedRecord{
		Inner: TestRecord{ID: 42, Name: "nested"},
		Outer: "outer_value",
	}
	
	outerVal := ExtractFieldValue(nested, "Outer")
	if outerVal != "outer_value" {
		t.Errorf("expected 'outer_value', got %v", outerVal)
	}
	
	innerVal := ExtractFieldValue(nested, "Inner")
	if innerVal == nil {
		t.Error("expected nested struct, got nil")
	}
	
	// Test with private field (should return nil due to CanInterface)
	type PrivateFieldStruct struct {
		Public  string
		private string
	}
	
	pfs := PrivateFieldStruct{Public: "public", private: "private"}
	publicVal := ExtractFieldValue(pfs, "Public")
	if publicVal != "public" {
		t.Errorf("expected 'public', got %v", publicVal)
	}
	
	// private field should return nil due to CanInterface check
	privateVal := ExtractFieldValue(pfs, "private")
	if privateVal != nil {
		t.Errorf("expected nil for private field, got %v", privateVal)
	}
}

func TestByteArrayIndex_EdgeCases(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewByteArrayIndex[TestRecord](stateEngine, "TestRecord", "Data")
	
	ctx := context.Background()
	
	// Test with empty bytes
	matches1 := index.FindByBytes(ctx, []byte{})
	if matches1 == nil {
		t.Fatal("FindByBytes should handle empty bytes")
	}
	
	// Test with nil bytes
	matches2 := index.FindByBytes(ctx, nil)
	if matches2 == nil {
		t.Fatal("FindByBytes should handle nil bytes")
	}
	
	// Test with large byte array
	largeBytes := make([]byte, 1000)
	for i := range largeBytes {
		largeBytes[i] = byte(i % 256)
	}
	
	matches3 := index.FindByBytes(ctx, largeBytes)
	if matches3 == nil {
		t.Fatal("FindByBytes should handle large byte arrays")
	}
	
	// Test with special characters
	specialBytes := []byte("Hello\x00World\xff\xfe")
	matches4 := index.FindByBytes(ctx, specialBytes)
	if matches4 == nil {
		t.Fatal("FindByBytes should handle special characters")
	}
}

func TestHashIndex_RebuildIndex(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	
	hashIdx := index.(*hashIndex[TestRecord])
	
	// Test rebuild - should not error
	err := hashIdx.rebuildIndex()
	if err != nil {
		t.Errorf("rebuildIndex should not error: %v", err)
	}
}

func TestUniqueKeyIndex_RebuildIndex(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewUniqueKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	
	uniqueIdx := index.(*uniqueKeyIndex[TestRecord])
	
	// Test rebuild - should not error
	err := uniqueIdx.rebuildIndex()
	if err != nil {
		t.Errorf("rebuildIndex should not error: %v", err)
	}
}

func TestPrimaryKeyIndex_RebuildIndex(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewPrimaryKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID", "Name"})
	
	pkIdx := index.(*primaryKeyIndex[TestRecord])
	
	// Test rebuild - should not error
	err := pkIdx.rebuildIndex()
	if err != nil {
		t.Errorf("rebuildIndex should not error: %v", err)
	}
}

func TestByteArrayIndex_RebuildIndex(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewByteArrayIndex[TestRecord](stateEngine, "TestRecord", "Data")
	
	// Test rebuild - should not error
	err := index.rebuildIndex()
	if err != nil {
		t.Errorf("rebuildIndex should not error: %v", err)
	}
}

// Test generic type constraints
func TestIndexes_WithDifferentTypes(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	
	// Test with string type
	stringIndex := NewHashIndex[string](stateEngine, "String", []string{"Value"})
	if stringIndex == nil {
		t.Error("should create hash index for string type")
	}
	
	// Test with int type
	intIndex := NewUniqueKeyIndex[int](stateEngine, "Integer", []string{"Value"})
	if intIndex == nil {
		t.Error("should create unique index for int type")
	}
	
	// Test with custom struct
	type CustomStruct struct {
		Field1 string
		Field2 int
	}
	
	customIndex := NewPrimaryKeyIndex[CustomStruct](stateEngine, "Custom", []string{"Field1", "Field2"})
	if customIndex == nil {
		t.Error("should create primary key index for custom struct")
	}
}

func TestIndexes_ConcurrentAccess(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	index := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	
	ctx := context.Background()
	
	// Test concurrent reads (should not panic)
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// Multiple operations on same index
			matches := index.FindMatches(ctx, id)
			matches.Next()
			matches.Close()
			
			index.DetectUpdates(ctx)
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestSliceIterator_WithNilSlice(t *testing.T) {
	var nilSlice []string
	iter := NewSliceIterator(nilSlice)
	
	// Should handle nil slice gracefully
	if iter.Next() {
		t.Error("nil slice iterator should not have Next() == true")
	}
	
	value := iter.Value()
	if value != "" {
		t.Errorf("expected zero value for nil slice, got %v", value)
	}
	
	err := iter.Close()
	if err != nil {
		t.Errorf("Close should not error for nil slice: %v", err)
	}
}

func TestIndexes_WithComplexKeys(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	ctx := context.Background()
	
	// Test with complex key structures
	complexIndex := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"ID", "Name", "Category"})
	
	// Test various key combinations
	testKeys := [][]interface{}{
		{1, "name1", "cat1"},
		{1, "name1", nil},
		{nil, "name2", "cat2"},
		{1, nil, nil},
		{nil, nil, nil},
		{0, "", ""},
		{-1, "negative", "test"},
		{999999, "large_id", "category_with_special_chars_!@#$%"},
	}
	
	for _, keys := range testKeys {
		matches := complexIndex.FindMatches(ctx, keys...)
		if matches == nil {
			t.Errorf("FindMatches should handle complex keys %v", keys)
		}
		matches.Close()
	}
}

func TestIndexes_StateEngineIntegration(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	
	// Create various indexes with the same state engine
	hashIndex := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	uniqueIndex := NewUniqueKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID"})
	pkIndex := NewPrimaryKeyIndex[TestRecord](stateEngine, "TestRecord", []string{"ID", "Name"})
	byteIndex := NewByteArrayIndex[TestRecord](stateEngine, "TestRecord", "Data")
	
	ctx := context.Background()
	
	// All should be able to detect updates without interference
	if err := hashIndex.DetectUpdates(ctx); err != nil {
		t.Errorf("hash index DetectUpdates failed: %v", err)
	}
	
	if err := uniqueIndex.DetectUpdates(ctx); err != nil {
		t.Errorf("unique index DetectUpdates failed: %v", err)
	}
	
	if err := pkIndex.DetectUpdates(ctx); err != nil {
		t.Errorf("primary key index DetectUpdates failed: %v", err)
	}
	
	if err := byteIndex.DetectUpdates(ctx); err != nil {
		t.Errorf("byte array index DetectUpdates failed: %v", err)
	}
}

func TestIterator_InterfaceCompliance(t *testing.T) {
	// Test that sliceIterator implements Iterator interface correctly
	testData := []int{1, 2, 3}
	var iter Iterator[int] = NewSliceIterator(testData)
	
	// Test interface methods
	count := 0
	for iter.Next() {
		value := iter.Value()
		if value != testData[count] {
			t.Errorf("expected %d, got %d", testData[count], value)
		}
		count++
	}
	
	if count != 3 {
		t.Errorf("expected 3 iterations, got %d", count)
	}
	
	err := iter.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestIndexCreation_WithEmptyFields(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	
	// Test creating indexes with empty field lists
	hashIndex := NewHashIndex[TestRecord](stateEngine, "TestRecord", []string{})
	if hashIndex == nil {
		t.Error("should create hash index with empty fields")
	}
	
	uniqueIndex := NewUniqueKeyIndex[TestRecord](stateEngine, "TestRecord", []string{})
	if uniqueIndex == nil {
		t.Error("should create unique index with empty fields")
	}
	
	pkIndex := NewPrimaryKeyIndex[TestRecord](stateEngine, "TestRecord", []string{})
	if pkIndex == nil {
		t.Error("should create primary key index with empty fields")
	}
	
	// Test with nil field lists
	hashIndex2 := NewHashIndex[TestRecord](stateEngine, "TestRecord", nil)
	if hashIndex2 == nil {
		t.Error("should create hash index with nil fields")
	}
}

func TestIndexCreation_WithEmptyTypeName(t *testing.T) {
	stateEngine := internal.NewReadStateEngine()
	
	// Test creating indexes with empty type name
	hashIndex := NewHashIndex[TestRecord](stateEngine, "", []string{"ID"})
	if hashIndex == nil {
		t.Error("should create hash index with empty type name")
	}
	
	uniqueIndex := NewUniqueKeyIndex[TestRecord](stateEngine, "", []string{"ID"})
	if uniqueIndex == nil {
		t.Error("should create unique index with empty type name")
	}
	
	pkIndex := NewPrimaryKeyIndex[TestRecord](stateEngine, "", []string{"ID"})
	if pkIndex == nil {
		t.Error("should create primary key index with empty type name")
	}
	
	byteIndex := NewByteArrayIndex[TestRecord](stateEngine, "", "Data")
	if byteIndex == nil {
		t.Error("should create byte array index with empty type name")
	}
}
