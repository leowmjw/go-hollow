package internal

import (
	"fmt"
	"sort"
)

// DeltaOperation represents the type of change for a record
type DeltaOperation int

const (
	DeltaAdd DeltaOperation = iota
	DeltaUpdate
	DeltaDelete
)

// DeltaRecord represents a single change in a delta
type DeltaRecord struct {
	Operation DeltaOperation
	Ordinal   int
	Value     interface{} // nil for deletes
}

// TypeDelta represents all changes for a specific type
type TypeDelta struct {
	TypeName string
	Records  []DeltaRecord
}

// DeltaSet represents a complete set of changes across all types
type DeltaSet struct {
	Deltas      map[string]*TypeDelta
	optimized   bool   // Whether this delta has been optimized
	changeCount int    // Total number of changes
}

// NewDeltaSet creates a new empty delta set
func NewDeltaSet() *DeltaSet {
	return &DeltaSet{
		Deltas: make(map[string]*TypeDelta),
	}
}

// AddRecord adds a change record to the delta set
func (ds *DeltaSet) AddRecord(typeName string, operation DeltaOperation, ordinal int, value interface{}) {
	if ds.Deltas[typeName] == nil {
		ds.Deltas[typeName] = &TypeDelta{
			TypeName: typeName,
			Records:  make([]DeltaRecord, 0),
		}
	}
	
	delta := DeltaRecord{
		Operation: operation,
		Ordinal:   ordinal,
		Value:     value,
	}
	
	ds.Deltas[typeName].Records = append(ds.Deltas[typeName].Records, delta)
}

// IsEmpty returns true if the delta set contains no changes
func (ds *DeltaSet) IsEmpty() bool {
	for _, delta := range ds.Deltas {
		if len(delta.Records) > 0 {
			return false
		}
	}
	return true
}

// GetTypeNames returns sorted list of type names with changes
func (ds *DeltaSet) GetTypeNames() []string {
	names := make([]string, 0, len(ds.Deltas))
	for typeName := range ds.Deltas {
		names = append(names, typeName)
	}
	sort.Strings(names)
	return names
}

// GetChangeCount returns total number of changes across all types
func (ds *DeltaSet) GetChangeCount() int {
	count := 0
	for _, delta := range ds.Deltas {
		count += len(delta.Records)
	}
	return count
}

// OptimizeDeltas merges and deduplicates delta records for the same ordinal
func (ds *DeltaSet) OptimizeDeltas() {
	for _, typeDelta := range ds.Deltas {
		ds.optimizeTypeDelta(typeDelta)
	}
}

func (ds *DeltaSet) optimizeTypeDelta(typeDelta *TypeDelta) {
	if len(typeDelta.Records) <= 1 {
		return
	}
	
	// Group by ordinal
	ordinalMap := make(map[int][]DeltaRecord)
	for _, record := range typeDelta.Records {
		ordinalMap[record.Ordinal] = append(ordinalMap[record.Ordinal], record)
	}
	
	// Rebuild optimized records
	optimized := make([]DeltaRecord, 0, len(ordinalMap))
	
	for _, records := range ordinalMap {
		// For multiple operations on same ordinal, keep only the last one
		// Exception: DELETE always wins
		finalRecord := records[len(records)-1]
		
		// Check if any record is a delete
		for _, record := range records {
			if record.Operation == DeltaDelete {
				finalRecord = record
				break
			}
		}
		
		optimized = append(optimized, finalRecord)
	}
	
	// Sort by ordinal for consistent output
	sort.Slice(optimized, func(i, j int) bool {
		return optimized[i].Ordinal < optimized[j].Ordinal
	})
	
	typeDelta.Records = optimized
}

// String returns a human-readable representation of the delta set
func (ds *DeltaSet) String() string {
	if ds.IsEmpty() {
		return "DeltaSet{empty}"
	}
	
	result := fmt.Sprintf("DeltaSet{%d changes across %d types}", 
		ds.GetChangeCount(), len(ds.Deltas))
	
	for _, typeName := range ds.GetTypeNames() {
		delta := ds.Deltas[typeName]
		adds := 0
		updates := 0
		deletes := 0
		
		for _, record := range delta.Records {
			switch record.Operation {
			case DeltaAdd:
				adds++
			case DeltaUpdate:
				updates++
			case DeltaDelete:
				deletes++
			}
		}
		
		result += fmt.Sprintf("\n  %s: +%d ~%d -%d", typeName, adds, updates, deletes)
	}
	
	return result
}
