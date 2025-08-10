package index

import (
	"context"
	"fmt"
	"reflect"
	"github.com/leowmjw/go-hollow/internal"
)

// Iterator interface for iterating over collections
type Iterator[T any] interface {
	Next() bool
	Value() T
	Close() error
}

// sliceIterator implements Iterator for slices
type sliceIterator[T any] struct {
	slice   []T
	current int
}

func NewSliceIterator[T any](slice []T) Iterator[T] {
	return &sliceIterator[T]{slice: slice, current: -1}
}

func (it *sliceIterator[T]) Next() bool {
	it.current++
	return it.current < len(it.slice)
}

func (it *sliceIterator[T]) Value() T {
	if it.current >= 0 && it.current < len(it.slice) {
		return it.slice[it.current]
	}
	var zero T
	return zero
}

func (it *sliceIterator[T]) Close() error {
	return nil
}

// HashIndex interface for multi-field lookups
type HashIndex[T any] interface {
	FindMatches(ctx context.Context, values ...interface{}) Iterator[T]
	DetectUpdates(ctx context.Context) error
}

// UniqueKeyIndex interface for single-result queries
type UniqueKeyIndex[T any] interface {
	GetMatch(ctx context.Context, key interface{}) (T, bool)
	DetectUpdates(ctx context.Context) error
}

// PrimaryKeyIndex interface for composite key lookups
type PrimaryKeyIndex[T any] interface {
	GetMatch(ctx context.Context, keys ...interface{}) (T, bool)
	DetectUpdates(ctx context.Context) error
}

// hashIndex implementation
type hashIndex[T any] struct {
	engine      *internal.ReadStateEngine
	typeName    string
	matchFields []string
	indexData   map[string][]T
}

// NewHashIndex creates a new hash index
func NewHashIndex[T any](engine *internal.ReadStateEngine, typeName string, matchFields []string) HashIndex[T] {
	return &hashIndex[T]{
		engine:      engine,
		typeName:    typeName,
		matchFields: matchFields,
		indexData:   make(map[string][]T),
	}
}

func (hi *hashIndex[T]) FindMatches(ctx context.Context, values ...interface{}) Iterator[T] {
	// Create key from values
	key := hi.createKey(values...)
	
	// Find matches in index
	matches, exists := hi.indexData[key]
	if !exists {
		matches = []T{}
	}
	
	return NewSliceIterator(matches)
}

func (hi *hashIndex[T]) DetectUpdates(ctx context.Context) error {
	// Clear existing index
	hi.indexData = make(map[string][]T)
	
	// Rebuild index from current state
	return hi.rebuildIndex()
}

func (hi *hashIndex[T]) rebuildIndex() error {
	// Get data from read state engine
	// This is a simplified implementation
	// In a real system, would extract data from the state engine
	return nil
}

func (hi *hashIndex[T]) createKey(values ...interface{}) string {
	key := ""
	for i, value := range values {
		if i > 0 {
			key += "|"
		}
		key += fmt.Sprintf("%v", value)
	}
	return key
}

// uniqueKeyIndex implementation
type uniqueKeyIndex[T any] struct {
	engine    *internal.ReadStateEngine
	typeName  string
	keyFields []string
	indexData map[string]T
}

// NewUniqueKeyIndex creates a new unique key index
func NewUniqueKeyIndex[T any](engine *internal.ReadStateEngine, typeName string, keyFields []string) UniqueKeyIndex[T] {
	return &uniqueKeyIndex[T]{
		engine:    engine,
		typeName:  typeName,
		keyFields: keyFields,
		indexData: make(map[string]T),
	}
}

func (uki *uniqueKeyIndex[T]) GetMatch(ctx context.Context, key interface{}) (T, bool) {
	keyStr := fmt.Sprintf("%v", key)
	value, exists := uki.indexData[keyStr]
	return value, exists
}

func (uki *uniqueKeyIndex[T]) DetectUpdates(ctx context.Context) error {
	// Clear existing index
	uki.indexData = make(map[string]T)
	
	// Rebuild index from current state
	return uki.rebuildIndex()
}

func (uki *uniqueKeyIndex[T]) rebuildIndex() error {
	// Get data from read state engine
	// This is a simplified implementation
	return nil
}

// primaryKeyIndex implementation  
type primaryKeyIndex[T any] struct {
	engine     *internal.ReadStateEngine
	typeName   string
	keyFields  []string
	indexData  map[string]T
}

// NewPrimaryKeyIndex creates a new primary key index
func NewPrimaryKeyIndex[T any](engine *internal.ReadStateEngine, typeName string, keyFields []string) PrimaryKeyIndex[T] {
	return &primaryKeyIndex[T]{
		engine:    engine,
		typeName:  typeName,
		keyFields: keyFields,
		indexData: make(map[string]T),
	}
}

func (pki *primaryKeyIndex[T]) GetMatch(ctx context.Context, keys ...interface{}) (T, bool) {
	keyStr := pki.createCompositeKey(keys...)
	value, exists := pki.indexData[keyStr]
	return value, exists
}

func (pki *primaryKeyIndex[T]) DetectUpdates(ctx context.Context) error {
	// Clear existing index
	pki.indexData = make(map[string]T)
	
	// Rebuild index from current state
	return pki.rebuildIndex()
}

func (pki *primaryKeyIndex[T]) rebuildIndex() error {
	// Get data from read state engine
	// This is a simplified implementation
	return nil
}

func (pki *primaryKeyIndex[T]) createCompositeKey(keys ...interface{}) string {
	keyStr := ""
	for i, key := range keys {
		if i > 0 {
			keyStr += "|"
		}
		keyStr += fmt.Sprintf("%v", key)
	}
	return keyStr
}

// Helper functions for working with indexed data
func ExtractFieldValue(obj interface{}, fieldName string) interface{} {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	if v.Kind() != reflect.Struct {
		return nil
	}
	
	field := v.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	
	if !field.CanInterface() {
		return nil
	}
	
	return field.Interface()
}

// ByteArrayIndex for indexing byte array fields
type ByteArrayIndex[T any] struct {
	engine    *internal.ReadStateEngine
	typeName  string
	fieldName string
	indexData map[string][]T
}

func NewByteArrayIndex[T any](engine *internal.ReadStateEngine, typeName string, fieldName string) *ByteArrayIndex[T] {
	return &ByteArrayIndex[T]{
		engine:    engine,
		typeName:  typeName,
		fieldName: fieldName,
		indexData: make(map[string][]T),
	}
}

func (bai *ByteArrayIndex[T]) FindByBytes(ctx context.Context, bytes []byte) Iterator[T] {
	key := string(bytes)
	matches, exists := bai.indexData[key]
	if !exists {
		matches = []T{}
	}
	
	return NewSliceIterator(matches)
}

func (bai *ByteArrayIndex[T]) DetectUpdates(ctx context.Context) error {
	// Clear existing index
	bai.indexData = make(map[string][]T)
	
	// Rebuild index from current state
	return bai.rebuildIndex()
}

func (bai *ByteArrayIndex[T]) rebuildIndex() error {
	// Get data from read state engine
	// This is a simplified implementation
	return nil
}
