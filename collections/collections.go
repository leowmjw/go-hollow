package collections

import (
	"github.com/leowmjw/go-hollow/index"
)

// HollowSet represents an unordered unique element collection
type HollowSet[T comparable] interface {
	Contains(element T) bool
	Size() int
	Iterator() index.Iterator[T]
	IsEmpty() bool
}

// HollowMap represents key-value mappings
type HollowMap[K comparable, V any] interface {
	Get(key K) (V, bool)
	Size() int
	EntrySet() []Entry[K, V]
	Keys() index.Iterator[K]
	Values() index.Iterator[V]
	Equals(other HollowMap[K, V]) bool
}

// HollowList represents ordered element sequences
type HollowList[T any] interface {
	Get(index int) T
	Size() int
	Iterator() index.Iterator[T]
	IsEmpty() bool
}

// Entry represents a key-value pair in a map
type Entry[K comparable, V any] struct {
	Key   K
	Value V
}

// hollowSet implementation
type hollowSet[T comparable] struct {
	elements map[T]bool
	ordinal  int
}

func NewHollowSet[T comparable](elements []T, ordinal int) HollowSet[T] {
	set := &hollowSet[T]{
		elements: make(map[T]bool),
		ordinal:  ordinal,
	}
	
	for _, element := range elements {
		set.elements[element] = true
	}
	
	return set
}

func (hs *hollowSet[T]) Contains(element T) bool {
	if hs.ordinal == 0 { // Zero ordinal handling
		return false
	}
	return hs.elements[element]
}

func (hs *hollowSet[T]) Size() int {
	if hs.ordinal == 0 {
		return 0
	}
	return len(hs.elements)
}

func (hs *hollowSet[T]) Iterator() index.Iterator[T] {
	if hs.ordinal == 0 {
		return index.NewSliceIterator([]T{})
	}
	
	slice := make([]T, 0, len(hs.elements))
	for element := range hs.elements {
		slice = append(slice, element)
	}
	
	return index.NewSliceIterator(slice)
}

func (hs *hollowSet[T]) IsEmpty() bool {
	return hs.Size() == 0
}

// hollowMap implementation
type hollowMap[K comparable, V any] struct {
	entries map[K]V
	ordinal int
}

func NewHollowMap[K comparable, V any](entries []Entry[K, V], ordinal int) HollowMap[K, V] {
	m := &hollowMap[K, V]{
		entries: make(map[K]V),
		ordinal: ordinal,
	}
	
	for _, entry := range entries {
		m.entries[entry.Key] = entry.Value
	}
	
	return m
}

func (hm *hollowMap[K, V]) Get(key K) (V, bool) {
	if hm.ordinal == 0 {
		var zero V
		return zero, false
	}
	
	value, exists := hm.entries[key]
	return value, exists
}

func (hm *hollowMap[K, V]) Size() int {
	if hm.ordinal == 0 {
		return 0
	}
	return len(hm.entries)
}

func (hm *hollowMap[K, V]) EntrySet() []Entry[K, V] {
	if hm.ordinal == 0 {
		return []Entry[K, V]{}
	}
	
	entries := make([]Entry[K, V], 0, len(hm.entries))
	for k, v := range hm.entries {
		entries = append(entries, Entry[K, V]{Key: k, Value: v})
	}
	
	return entries
}

func (hm *hollowMap[K, V]) Keys() index.Iterator[K] {
	if hm.ordinal == 0 {
		return index.NewSliceIterator([]K{})
	}
	
	keys := make([]K, 0, len(hm.entries))
	for k := range hm.entries {
		keys = append(keys, k)
	}
	
	return index.NewSliceIterator(keys)
}

func (hm *hollowMap[K, V]) Values() index.Iterator[V] {
	if hm.ordinal == 0 {
		return index.NewSliceIterator([]V{})
	}
	
	values := make([]V, 0, len(hm.entries))
	for _, v := range hm.entries {
		values = append(values, v)
	}
	
	return index.NewSliceIterator(values)
}

func (hm *hollowMap[K, V]) Equals(other HollowMap[K, V]) bool {
	if hm.Size() != other.Size() {
		return false
	}
	
	otherEntries := other.EntrySet()
	otherMap := make(map[K]V)
	for _, entry := range otherEntries {
		otherMap[entry.Key] = entry.Value
	}
	
	for k, v := range hm.entries {
		otherV, exists := otherMap[k]
		if !exists {
			return false
		}
		
		// For simple comparison - in real implementation would need proper equality
		if any(v) != any(otherV) {
			return false
		}
	}
	
	return true
}

// hollowList implementation
type hollowList[T any] struct {
	elements []T
	ordinal  int
}

func NewHollowList[T any](elements []T, ordinal int) HollowList[T] {
	return &hollowList[T]{
		elements: elements,
		ordinal:  ordinal,
	}
}

func (hl *hollowList[T]) Get(index int) T {
	if hl.ordinal == 0 || index < 0 || index >= len(hl.elements) {
		var zero T
		return zero
	}
	
	return hl.elements[index]
}

func (hl *hollowList[T]) Size() int {
	if hl.ordinal == 0 {
		return 0
	}
	return len(hl.elements)
}

func (hl *hollowList[T]) Iterator() index.Iterator[T] {
	if hl.ordinal == 0 {
		return index.NewSliceIterator([]T{})
	}
	
	return index.NewSliceIterator(hl.elements)
}

func (hl *hollowList[T]) IsEmpty() bool {
	return hl.Size() == 0
}

// StateInvalidationException represents an error when accessing stale state
type StateInvalidationException struct {
	Message string
}

func (e StateInvalidationException) Error() string {
	return e.Message
}

// CheckStateValidity checks if the state is still valid
func CheckStateValidity(ordinal int, isStateValid bool) error {
	if ordinal != 0 && !isStateValid {
		return StateInvalidationException{Message: "Accessing stale hollow object state"}
	}
	return nil
}
