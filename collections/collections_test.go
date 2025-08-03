package collections

import (
	"testing"
)

// TestHollowSet_Basic tests basic set operations
func TestHollowSet_Basic(t *testing.T) {
	// Create set with elements
	elements := []int{10, 20, 30}
	set := NewHollowSet(elements, 1) // ordinal 1 (valid)
	
	// Test Contains
	if !set.Contains(20) {
		t.Error("Set should contain 20")
	}
	if set.Contains(40) {
		t.Error("Set should not contain 40")
	}
	
	// Test Size
	if set.Size() != 3 {
		t.Errorf("Set size should be 3, got %d", set.Size())
	}
	
	// Test IsEmpty
	if set.IsEmpty() {
		t.Error("Set should not be empty")
	}
	
	// Test Iterator
	iter := set.Iterator()
	count := 0
	for iter.Next() {
		count++
		value := iter.Value()
		if value != 10 && value != 20 && value != 30 {
			t.Errorf("Unexpected value in iterator: %d", value)
		}
	}
	if count != 3 {
		t.Errorf("Iterator should return 3 elements, got %d", count)
	}
	iter.Close()
}

// TestHollowSet_ZeroOrdinal tests zero ordinal handling
func TestHollowSet_ZeroOrdinal(t *testing.T) {
	// Create set with zero ordinal
	elements := []int{10, 20, 30}
	set := NewHollowSet(elements, 0) // ordinal 0 (invalid)
	
	// Should behave as empty set
	if set.Contains(20) {
		t.Error("Zero ordinal set should not contain any elements")
	}
	
	if set.Size() != 0 {
		t.Errorf("Zero ordinal set size should be 0, got %d", set.Size())
	}
	
	if !set.IsEmpty() {
		t.Error("Zero ordinal set should be empty")
	}
}

// TestHollowMap_Basic tests basic map operations
func TestHollowMap_Basic(t *testing.T) {
	// Create map with entries
	entries := []Entry[string, int]{
		{Key: "one", Value: 1},
		{Key: "two", Value: 2},
		{Key: "three", Value: 3},
	}
	m := NewHollowMap(entries, 1) // ordinal 1 (valid)
	
	// Test Get
	value, exists := m.Get("two")
	if !exists {
		t.Error("Map should contain key 'two'")
	}
	if value != 2 {
		t.Errorf("Expected value 2 for key 'two', got %d", value)
	}
	
	// Test non-existent key
	_, exists = m.Get("four")
	if exists {
		t.Error("Map should not contain key 'four'")
	}
	
	// Test Size
	if m.Size() != 3 {
		t.Errorf("Map size should be 3, got %d", m.Size())
	}
	
	// Test EntrySet
	entrySet := m.EntrySet()
	if len(entrySet) != 3 {
		t.Errorf("EntrySet should have 3 entries, got %d", len(entrySet))
	}
	
	// Test Keys iterator
	keys := m.Keys()
	keyCount := 0
	for keys.Next() {
		keyCount++
		key := keys.Value()
		if key != "one" && key != "two" && key != "three" {
			t.Errorf("Unexpected key: %s", key)
		}
	}
	if keyCount != 3 {
		t.Errorf("Keys iterator should return 3 keys, got %d", keyCount)
	}
	keys.Close()
	
	// Test Values iterator
	values := m.Values()
	valueCount := 0
	for values.Next() {
		valueCount++
		value := values.Value()
		if value != 1 && value != 2 && value != 3 {
			t.Errorf("Unexpected value: %d", value)
		}
	}
	if valueCount != 3 {
		t.Errorf("Values iterator should return 3 values, got %d", valueCount)
	}
	values.Close()
}

// TestHollowMap_Equals tests map equality
func TestHollowMap_Equals(t *testing.T) {
	entries1 := []Entry[string, int]{
		{Key: "one", Value: 1},
		{Key: "two", Value: 2},
	}
	entries2 := []Entry[string, int]{
		{Key: "two", Value: 2},
		{Key: "one", Value: 1},
	}
	entries3 := []Entry[string, int]{
		{Key: "one", Value: 1},
		{Key: "two", Value: 3}, // Different value
	}
	
	map1 := NewHollowMap(entries1, 1)
	map2 := NewHollowMap(entries2, 1) // Same entries, different order
	map3 := NewHollowMap(entries3, 1) // Different entries
	
	// Test equality
	if !map1.Equals(map2) {
		t.Error("Maps with same entries should be equal")
	}
	
	// Test inequality
	if map1.Equals(map3) {
		t.Error("Maps with different values should not be equal")
	}
}

// TestHollowList_Basic tests basic list operations
func TestHollowList_Basic(t *testing.T) {
	// Create list with elements
	elements := []string{"first", "second", "third"}
	list := NewHollowList(elements, 1) // ordinal 1 (valid)
	
	// Test Get
	if list.Get(0) != "first" {
		t.Errorf("Expected 'first' at index 0, got %s", list.Get(0))
	}
	if list.Get(1) != "second" {
		t.Errorf("Expected 'second' at index 1, got %s", list.Get(1))
	}
	if list.Get(2) != "third" {
		t.Errorf("Expected 'third' at index 2, got %s", list.Get(2))
	}
	
	// Test out of bounds
	outOfBounds := list.Get(10)
	if outOfBounds != "" {
		t.Errorf("Out of bounds access should return zero value, got %s", outOfBounds)
	}
	
	// Test Size
	if list.Size() != 3 {
		t.Errorf("List size should be 3, got %d", list.Size())
	}
	
	// Test IsEmpty
	if list.IsEmpty() {
		t.Error("List should not be empty")
	}
	
	// Test Iterator
	iter := list.Iterator()
	index := 0
	for iter.Next() {
		value := iter.Value()
		expected := elements[index]
		if value != expected {
			t.Errorf("Iterator at index %d: expected %s, got %s", index, expected, value)
		}
		index++
	}
	if index != 3 {
		t.Errorf("Iterator should return 3 elements, got %d", index)
	}
	iter.Close()
}

// TestCollections_StateInvalidation tests stale reference exception handling
func TestCollections_StateInvalidation(t *testing.T) {
	// Test state validity checking
	err := CheckStateValidity(1, true) // Valid state
	if err != nil {
		t.Errorf("Valid state should not return error: %v", err)
	}
	
	err = CheckStateValidity(1, false) // Invalid state
	if err == nil {
		t.Error("Invalid state should return error")
	}
	
	// Check error type
	if _, ok := err.(StateInvalidationException); !ok {
		t.Errorf("Error should be StateInvalidationException, got %T", err)
	}
	
	err = CheckStateValidity(0, false) // Zero ordinal should not error
	if err != nil {
		t.Errorf("Zero ordinal should not return error: %v", err)
	}
}
