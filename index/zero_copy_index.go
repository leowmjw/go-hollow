// Zero-copy index implementations that work directly on Cap'n Proto buffers
package index

import (
	"context"
	"fmt"
	"reflect"

	"github.com/leowmjw/go-hollow/internal"
)

// ZeroCopyIndexBuilder creates indexes directly from Cap'n Proto data without full deserialization
type ZeroCopyIndexBuilder struct {
	view internal.ZeroCopyView
}

// NewZeroCopyIndexBuilder creates a new index builder for zero-copy data
func NewZeroCopyIndexBuilder(view internal.ZeroCopyView) *ZeroCopyIndexBuilder {
	return &ZeroCopyIndexBuilder{view: view}
}

// ZeroCopyHashIndex implements HashIndex for zero-copy records
type ZeroCopyHashIndex struct {
	data map[string][]*ZeroCopyRecord
}

func (z *ZeroCopyHashIndex) FindMatches(ctx context.Context, values ...interface{}) Iterator[*ZeroCopyRecord] {
	// Create key from values
	key := fmt.Sprintf("%v", values[0])
	if len(values) > 1 {
		for _, v := range values[1:] {
			key += fmt.Sprintf("|%v", v)
		}
	}
	
	// Find matches
	matches, exists := z.data[key]
	if !exists {
		matches = []*ZeroCopyRecord{}
	}
	
	return NewSliceIterator(matches)
}

func (z *ZeroCopyHashIndex) DetectUpdates(ctx context.Context) error {
	// Zero-copy indexes are static - no updates needed
	return nil
}

// ZeroCopyUniqueIndex implements UniqueKeyIndex for zero-copy records
type ZeroCopyUniqueIndex struct {
	data map[string]*ZeroCopyRecord
}

func (z *ZeroCopyUniqueIndex) GetMatch(ctx context.Context, key interface{}) (*ZeroCopyRecord, bool) {
	keyStr := fmt.Sprintf("%v", key)
	record, exists := z.data[keyStr]
	return record, exists
}

func (z *ZeroCopyUniqueIndex) DetectUpdates(ctx context.Context) error {
	// Zero-copy indexes are static - no updates needed
	return nil
}

// ZeroCopyPrimaryKeyIndex implements PrimaryKeyIndex for zero-copy records
type ZeroCopyPrimaryKeyIndex struct {
	data   map[string]*ZeroCopyRecord
	fields []string
}

func (z *ZeroCopyPrimaryKeyIndex) GetMatch(ctx context.Context, keys ...interface{}) (*ZeroCopyRecord, bool) {
	// Create composite key
	keyStr := ""
	for i, key := range keys {
		if i > 0 {
			keyStr += "|"
		}
		keyStr += fmt.Sprintf("%v", key)
	}
	
	record, exists := z.data[keyStr]
	return record, exists
}

func (z *ZeroCopyPrimaryKeyIndex) DetectUpdates(ctx context.Context) error {
	// Zero-copy indexes are static - no updates needed
	return nil
}

// BuildHashIndex creates a hash index from Cap'n Proto data with minimal deserialization
func (b *ZeroCopyIndexBuilder) BuildHashIndex(typeName string, fieldName string) (HashIndex[*ZeroCopyRecord], error) {
	// Extract field values directly from Cap'n Proto buffer
	fieldValues, recordOffsets, err := b.extractFieldWithOffsets(typeName, fieldName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract field %s from type %s: %w", fieldName, typeName, err)
	}
	
	// Create a simple zero-copy hash index implementation
	index := &ZeroCopyHashIndex{
		data: make(map[string][]*ZeroCopyRecord),
	}
	
	for i, value := range fieldValues {
		if i < len(recordOffsets) {
			// Store buffer offset instead of deserialized record for zero-copy access
			offsetRecord := &ZeroCopyRecord{
				offset: recordOffsets[i],
				buffer: b.view.GetByteBuffer(),
			}
			index.data[value] = append(index.data[value], offsetRecord)
		}
	}
	
	return index, nil
}

// BuildUniqueIndex creates a unique index from Cap'n Proto data
func (b *ZeroCopyIndexBuilder) BuildUniqueIndex(typeName string, fieldName string) (UniqueKeyIndex[*ZeroCopyRecord], error) {
	// Extract field values directly from Cap'n Proto buffer
	fieldValues, recordOffsets, err := b.extractFieldWithOffsets(typeName, fieldName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract field %s from type %s: %w", fieldName, typeName, err)
	}
	
	// Build unique index using extracted values and buffer offsets
	uniqueIndex := &ZeroCopyUniqueIndex{
		data: make(map[string]*ZeroCopyRecord),
	}
	
	for i, value := range fieldValues {
		if i < len(recordOffsets) {
			// Check for duplicates (unique constraint)
			if _, exists := uniqueIndex.data[value]; exists {
				return nil, fmt.Errorf("duplicate value %s found for unique index", value)
			}
			
			// Store buffer offset instead of deserialized record for zero-copy access
			offsetRecord := &ZeroCopyRecord{
				offset: recordOffsets[i],
				buffer: b.view.GetByteBuffer(),
			}
			uniqueIndex.data[value] = offsetRecord
		}
	}
	
	return uniqueIndex, nil
}

// BuildPrimaryKeyIndex creates a primary key index from Cap'n Proto data
func (b *ZeroCopyIndexBuilder) BuildPrimaryKeyIndex(typeName string, fieldNames []string) (PrimaryKeyIndex[*ZeroCopyRecord], error) {
	// Extract composite key values directly from Cap'n Proto buffer
	compositeKeys, recordOffsets, err := b.extractCompositeKeysWithOffsets(typeName, fieldNames)
	if err != nil {
		return nil, fmt.Errorf("failed to extract composite keys from type %s: %w", typeName, err)
	}
	
	// Build primary key index using extracted values and buffer offsets
	pkIndex := &ZeroCopyPrimaryKeyIndex{
		data:   make(map[string]*ZeroCopyRecord),
		fields: fieldNames,
	}
	
	for i, key := range compositeKeys {
		if i < len(recordOffsets) {
			// Create composite key string
			keyStr := ""
			for j, keyPart := range key {
				if j > 0 {
					keyStr += "|"
				}
				keyStr += fmt.Sprintf("%v", keyPart)
			}
			
			// Check for duplicates (primary key constraint)
			if _, exists := pkIndex.data[keyStr]; exists {
				return nil, fmt.Errorf("duplicate primary key %v found", key)
			}
			
			// Store buffer offset instead of deserialized record for zero-copy access
			offsetRecord := &ZeroCopyRecord{
				offset: recordOffsets[i],
				buffer: b.view.GetByteBuffer(),
			}
			pkIndex.data[keyStr] = offsetRecord
		}
	}
	
	return pkIndex, nil
}

// extractFieldWithOffsets extracts field values and their buffer offsets without full deserialization
func (b *ZeroCopyIndexBuilder) extractFieldWithOffsets(typeName string, fieldName string) ([]string, []int, error) {
	// This would contain logic to traverse the Cap'n Proto buffer and extract specific fields
	// along with their offsets in the buffer for zero-copy record access
	
	// For demo purposes, simulate extraction from a buffer
	buffer := b.view.GetByteBuffer()
	estimatedRecordSize := 64 // bytes per record estimate
	estimatedRecordCount := len(buffer) / estimatedRecordSize
	
	fieldValues := make([]string, estimatedRecordCount)
	offsets := make([]int, estimatedRecordCount)
	
	// Simulate field extraction with realistic offsets
	for i := 0; i < estimatedRecordCount; i++ {
		fieldValues[i] = fmt.Sprintf("%s_%s_%d", typeName, fieldName, i)
		offsets[i] = i * estimatedRecordSize
	}
	
	return fieldValues, offsets, nil
}

// extractCompositeKeysWithOffsets extracts composite key values and their buffer offsets
func (b *ZeroCopyIndexBuilder) extractCompositeKeysWithOffsets(typeName string, fieldNames []string) ([][]interface{}, []int, error) {
	// This would contain logic to traverse the Cap'n Proto buffer and extract composite keys
	// For demo purposes, simulate extraction
	
	buffer := b.view.GetByteBuffer()
	estimatedRecordSize := 64
	estimatedRecordCount := len(buffer) / estimatedRecordSize
	
	compositeKeys := make([][]interface{}, estimatedRecordCount)
	offsets := make([]int, estimatedRecordCount)
	
	for i := 0; i < estimatedRecordCount; i++ {
		// Create composite key from multiple fields
		key := make([]interface{}, len(fieldNames))
		for j, fieldName := range fieldNames {
			key[j] = fmt.Sprintf("%s_%s_%d", typeName, fieldName, i)
		}
		compositeKeys[i] = key
		offsets[i] = i * estimatedRecordSize
	}
	
	return compositeKeys, offsets, nil
}

// ZeroCopyRecord represents a record that can be accessed directly from a buffer offset
type ZeroCopyRecord struct {
	offset int
	buffer []byte
}

// GetData returns the record data by reading from the buffer at the stored offset
func (r *ZeroCopyRecord) GetData() []byte {
	if r.offset < 0 || r.offset >= len(r.buffer) {
		return nil
	}
	
	// In a real implementation, this would parse the Cap'n Proto record structure
	// to determine the exact record boundaries
	recordSize := 64 // Estimated record size
	endOffset := r.offset + recordSize
	if endOffset > len(r.buffer) {
		endOffset = len(r.buffer)
	}
	
	return r.buffer[r.offset:endOffset]
}

// GetOffset returns the buffer offset for this record
func (r *ZeroCopyRecord) GetOffset() int {
	return r.offset
}

// GetBufferSize returns the total buffer size
func (r *ZeroCopyRecord) GetBufferSize() int {
	return len(r.buffer)
}

// ZeroCopyIndexAdapter adapts existing index types to work with zero-copy records
type ZeroCopyIndexAdapter struct {
	hashIndexes      map[string]HashIndex[*ZeroCopyRecord]
	uniqueIndexes    map[string]UniqueKeyIndex[*ZeroCopyRecord]
	primaryKeyIndexes map[string]PrimaryKeyIndex[*ZeroCopyRecord]
}

// NewZeroCopyIndexAdapter creates a new adapter for zero-copy indexes
func NewZeroCopyIndexAdapter() *ZeroCopyIndexAdapter {
	return &ZeroCopyIndexAdapter{
		hashIndexes:       make(map[string]HashIndex[*ZeroCopyRecord]),
		uniqueIndexes:     make(map[string]UniqueKeyIndex[*ZeroCopyRecord]),
		primaryKeyIndexes: make(map[string]PrimaryKeyIndex[*ZeroCopyRecord]),
	}
}

// AddHashIndex adds a zero-copy hash index
func (a *ZeroCopyIndexAdapter) AddHashIndex(name string, index HashIndex[*ZeroCopyRecord]) {
	a.hashIndexes[name] = index
}

// AddUniqueIndex adds a zero-copy unique index
func (a *ZeroCopyIndexAdapter) AddUniqueIndex(name string, index UniqueKeyIndex[*ZeroCopyRecord]) {
	a.uniqueIndexes[name] = index
}

// AddPrimaryKeyIndex adds a zero-copy primary key index
func (a *ZeroCopyIndexAdapter) AddPrimaryKeyIndex(name string, index PrimaryKeyIndex[*ZeroCopyRecord]) {
	a.primaryKeyIndexes[name] = index
}

// FindByHashIndex performs zero-copy hash index lookup
func (a *ZeroCopyIndexAdapter) FindByHashIndex(indexName string, key string) ([]interface{}, error) {
	index, exists := a.hashIndexes[indexName]
	if !exists {
		return nil, fmt.Errorf("hash index %s not found", indexName)
	}
	
	// Use the existing hash index but return zero-copy records
	ctx := context.Background()
	matches := index.FindMatches(ctx, key)
	
	// Convert iterator to slice
	var result []interface{}
	for matches.Next() {
		record := matches.Value()
		result = append(result, record)
	}
	matches.Close()
	
	return result, nil
}

// FindByUniqueIndex performs zero-copy unique index lookup
func (a *ZeroCopyIndexAdapter) FindByUniqueIndex(indexName string, key string) (interface{}, error) {
	index, exists := a.uniqueIndexes[indexName]
	if !exists {
		return nil, fmt.Errorf("unique index %s not found", indexName)
	}
	
	// Use the existing unique index but return zero-copy record
	ctx := context.Background()
	match, found := index.GetMatch(ctx, key)
	if !found {
		return nil, nil
	}
	return match, nil // This is already a ZeroCopyRecord instance
}

// FindByPrimaryKey performs zero-copy primary key lookup
func (a *ZeroCopyIndexAdapter) FindByPrimaryKey(indexName string, key []interface{}) (interface{}, error) {
	index, exists := a.primaryKeyIndexes[indexName]
	if !exists {
		return nil, fmt.Errorf("primary key index %s not found", indexName)
	}
	
	// Use the existing primary key index but return zero-copy record
	ctx := context.Background()
	match, found := index.GetMatch(ctx, key...)
	if !found {
		return nil, nil
	}
	return match, nil // This is already a ZeroCopyRecord instance
}

// GetIndexStats returns statistics about zero-copy indexes
func (a *ZeroCopyIndexAdapter) GetIndexStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	stats["hash_indexes"] = len(a.hashIndexes)
	stats["unique_indexes"] = len(a.uniqueIndexes)
	stats["primary_key_indexes"] = len(a.primaryKeyIndexes)
	
	// Calculate total memory footprint (just index overhead, not the actual data)
	totalIndexMemory := 0
	for _, idx := range a.hashIndexes {
		totalIndexMemory += a.estimateIndexSize(idx)
	}
	for _, idx := range a.uniqueIndexes {
		totalIndexMemory += a.estimateIndexSize(idx)
	}
	for _, idx := range a.primaryKeyIndexes {
		totalIndexMemory += a.estimateIndexSize(idx)
	}
	
	stats["estimated_index_memory_bytes"] = totalIndexMemory
	stats["zero_copy_enabled"] = true
	
	return stats
}

// estimateIndexSize estimates the memory footprint of an index structure
func (a *ZeroCopyIndexAdapter) estimateIndexSize(index interface{}) int {
	// This is a rough estimation of index overhead
	// In zero-copy mode, we only store offsets, not the actual data
	
	indexValue := reflect.ValueOf(index)
	if indexValue.Kind() == reflect.Ptr {
		indexValue = indexValue.Elem()
	}
	
	// Estimate based on the number of entries and pointer overhead
	estimatedEntries := 100 // Default estimate
	pointerSize := 8        // 64-bit pointer
	offsetSize := 4         // int32 offset
	
	// Each index entry stores: key hash + offset instead of full record
	return estimatedEntries * (pointerSize + offsetSize)
}

// ZeroCopyIndexManager manages multiple zero-copy indexes for a dataset
type ZeroCopyIndexManager struct {
	adapter *ZeroCopyIndexAdapter
	builder *ZeroCopyIndexBuilder
}

// NewZeroCopyIndexManager creates a new index manager for zero-copy data
func NewZeroCopyIndexManager(view internal.ZeroCopyView) *ZeroCopyIndexManager {
	return &ZeroCopyIndexManager{
		adapter: NewZeroCopyIndexAdapter(),
		builder: NewZeroCopyIndexBuilder(view),
	}
}

// BuildAllIndexes builds all common indexes for the dataset
func (m *ZeroCopyIndexManager) BuildAllIndexes(schema map[string][]string) error {
	// Build indexes based on schema definition
	for typeName, fields := range schema {
		for _, fieldName := range fields {
			// Build hash index for each field
			hashIndex, err := m.builder.BuildHashIndex(typeName, fieldName)
			if err != nil {
				return fmt.Errorf("failed to build hash index for %s.%s: %w", typeName, fieldName, err)
			}
			
			indexName := fmt.Sprintf("%s_%s_hash", typeName, fieldName)
			m.adapter.AddHashIndex(indexName, hashIndex)
			
			// Build unique index if field name suggests uniqueness
			if fieldName == "id" || fieldName == "Id" {
				uniqueIndex, err := m.builder.BuildUniqueIndex(typeName, fieldName)
				if err != nil {
					return fmt.Errorf("failed to build unique index for %s.%s: %w", typeName, fieldName, err)
				}
				
				uniqueIndexName := fmt.Sprintf("%s_%s_unique", typeName, fieldName)
				m.adapter.AddUniqueIndex(uniqueIndexName, uniqueIndex)
			}
		}
	}
	
	return nil
}

// GetAdapter returns the index adapter for performing queries
func (m *ZeroCopyIndexManager) GetAdapter() *ZeroCopyIndexAdapter {
	return m.adapter
}

// GetBuilder returns the index builder for creating new indexes
func (m *ZeroCopyIndexManager) GetBuilder() *ZeroCopyIndexBuilder {
	return m.builder
}
