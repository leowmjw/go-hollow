package internal

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"github.com/leowmjw/go-hollow/schema"
	"capnproto.org/go/capnp/v3"
)

// WriteState represents the state during write operations
type WriteState struct {
	version int64
	data    map[string][]interface{}
	schemas map[string]schema.Schema
	engine  *WriteStateEngine // Reference to engine for primary key support
}

func NewWriteState() *WriteState {
	return &WriteState{
		data:    make(map[string][]interface{}),
		schemas: make(map[string]schema.Schema),
	}
}

func (ws *WriteState) Add(value interface{}) {
	// Try to use primary key functionality if engine is available
	if ws.engine != nil {
		err := ws.engine.AddWithPrimaryKey(value)
		if err != nil {
			// Fall back to traditional add on error
			ws.addTraditional(value)
		}
		return
	}
	
	// Traditional add when no engine
	ws.addTraditional(value)
}

func (ws *WriteState) addTraditional(value interface{}) {
	typeName := getTypeName(value)
	if ws.data[typeName] == nil {
		ws.data[typeName] = make([]interface{}, 0)
	}
	ws.data[typeName] = append(ws.data[typeName], value)
}

// SetEngine sets the WriteStateEngine for this WriteState
// This allows the WriteState to use primary key functionality when available
func (ws *WriteState) SetEngine(engine *WriteStateEngine) {
	ws.engine = engine
}

// AddWithEngine tries to use engine's AddWithPrimaryKey if available, falls back to Add
func (ws *WriteState) AddWithEngine(value interface{}) error {
	if ws.engine != nil {
		return ws.engine.AddWithPrimaryKey(value)
	}
	ws.Add(value)
	return nil
}

// AddWithIdentity adds a value to the write state with identity management
// This method should be used by the WriteStateEngine when primary keys are configured
func (ws *WriteState) AddWithIdentity(typeName string, value interface{}, ordinal int, isUpdate bool) {
	if ws.data[typeName] == nil {
		ws.data[typeName] = make([]interface{}, 0)
	}
	
	// For updates, we need to track the ordinal for proper delta generation
	// The value could be a Cap'n Proto struct for zero-copy access
	recordInfo := &RecordInfo{
		Value:    value,
		Ordinal:  ordinal,
		IsUpdate: isUpdate,
	}
	
	ws.data[typeName] = append(ws.data[typeName], recordInfo)
}

// RecordInfo wraps a value with its identity information for delta tracking
type RecordInfo struct {
	Value    interface{}
	Ordinal  int
	IsUpdate bool
}

func (ws *WriteState) GetData() map[string][]interface{} {
	return ws.data
}

func (ws *WriteState) IsEmpty() bool {
	for _, values := range ws.data {
		if len(values) > 0 {
			return false
		}
	}
	return true
}

// ReadState represents the state during read operations
type ReadState struct {
	version    int64
	data       map[string][]interface{}
	invalidated bool
}

func NewReadState(version int64) *ReadState {
	return &ReadState{
		version: version,
		data:    make(map[string][]interface{}),
	}
}

func (rs *ReadState) GetVersion() int64 {
	return rs.version
}

func (rs *ReadState) GetData(typeName string) []interface{} {
	if rs.invalidated {
		panic("accessing invalidated read state")
	}
	return rs.data[typeName]
}

func (rs *ReadState) GetAllData() map[string][]interface{} {
	if rs.invalidated {
		panic("accessing invalidated read state")
	}
	return rs.data
}

func (rs *ReadState) Invalidate() {
	rs.invalidated = true
}

func (rs *ReadState) AddMockType(typeName string) {
	if rs.data[typeName] == nil {
		rs.data[typeName] = []interface{}{"mock_data"}
	}
}

// IdentityMap tracks record identities across cycles using primary keys
type IdentityMap struct {
	// Map from type -> primary key value -> ordinal
	typeToKeyToOrdinal map[string]map[string]int
	// Track ordinal assignments
	nextOrdinal        map[string]int
	mutex              sync.RWMutex
}

func NewIdentityMap() *IdentityMap {
	return &IdentityMap{
		typeToKeyToOrdinal: make(map[string]map[string]int),
		nextOrdinal:        make(map[string]int),
	}
}

// GetOrCreateOrdinal returns the ordinal for a given type/key combination
// If the key doesn't exist, creates a new ordinal. Otherwise returns existing ordinal.
func (im *IdentityMap) GetOrCreateOrdinal(typeName string, primaryKeyValue string) (int, bool) {
	im.mutex.Lock()
	defer im.mutex.Unlock()
	
	if im.typeToKeyToOrdinal[typeName] == nil {
		im.typeToKeyToOrdinal[typeName] = make(map[string]int)
	}
	
	if ordinal, exists := im.typeToKeyToOrdinal[typeName][primaryKeyValue]; exists {
		return ordinal, true // Existing record - this is an UPDATE
	}
	
	// New record - assign new ordinal
	newOrdinal := im.nextOrdinal[typeName]
	im.typeToKeyToOrdinal[typeName][primaryKeyValue] = newOrdinal
	im.nextOrdinal[typeName]++
	
	return newOrdinal, false // New record - this is an ADD
}

// GetOrdinal returns the ordinal for a given type/key combination if it exists
func (im *IdentityMap) GetOrdinal(typeName string, primaryKeyValue string) (int, bool) {
	im.mutex.RLock()
	defer im.mutex.RUnlock()
	
	if typeMap, exists := im.typeToKeyToOrdinal[typeName]; exists {
		if ordinal, exists := typeMap[primaryKeyValue]; exists {
			return ordinal, true
		}
	}
	return 0, false
}

// WriteStateEngine manages write operations
type WriteStateEngine struct {
	currentState   *WriteState
	headerTags     map[string]string
	populatedCount int32
	shardCounts    map[string]int
	// Primary key configuration and identity management
	primaryKeys    map[string]string
	identityMap    *IdentityMap
	// Previous state for delta calculation
	previousState  *WriteState
	// Delta set for current cycle
	currentDelta   *DeltaSet
}

func NewWriteStateEngine() *WriteStateEngine {
	return &WriteStateEngine{
		currentState: NewWriteState(),
		headerTags:   make(map[string]string),
		shardCounts:  make(map[string]int),
		primaryKeys:  make(map[string]string),
		identityMap:  NewIdentityMap(),
		currentDelta: NewDeltaSet(),
	}
}

// SetPrimaryKeys configures the primary key fields for types
func (wse *WriteStateEngine) SetPrimaryKeys(primaryKeys map[string]string) {
	wse.primaryKeys = make(map[string]string)
	for typeName, fieldName := range primaryKeys {
		wse.primaryKeys[typeName] = fieldName
	}
}

func (wse *WriteStateEngine) PrepareForCycle() {
	// Save previous state for delta calculation
	wse.previousState = wse.currentState
	
	// Create new state for this cycle
	wse.currentState = NewWriteState()
	wse.currentState.SetEngine(wse)
	
	// Reset delta set for this cycle
	wse.currentDelta = NewDeltaSet()
	
	// Note: We don't clear the identityMap here because we need to track 
	// identity across cycles for proper delta generation. The identity map
	// tracks which ordinals correspond to which primary keys across all cycles.
}

func (wse *WriteStateEngine) GetWriteState() *WriteState {
	return wse.currentState
}

func (wse *WriteStateEngine) GetPopulatedCount() int {
	return int(atomic.LoadInt32(&wse.populatedCount))
}

func (wse *WriteStateEngine) IncrementPopulatedCount() {
	atomic.AddInt32(&wse.populatedCount, 1)
}

func (wse *WriteStateEngine) ResetPopulatedCount() {
	atomic.StoreInt32(&wse.populatedCount, 0)
}

func (wse *WriteStateEngine) SetHeaderTag(key, value string) {
	wse.headerTags[key] = value
}

func (wse *WriteStateEngine) GetHeaderTags() map[string]string {
	result := make(map[string]string)
	for k, v := range wse.headerTags {
		result[k] = v
	}
	return result
}

func (wse *WriteStateEngine) GetNumShards(typeName string) int {
	if count, exists := wse.shardCounts[typeName]; exists {
		return count
	}
	return 1
}

func (wse *WriteStateEngine) SetNumShards(typeName string, count int) {
	wse.shardCounts[typeName] = count
}

// GetCurrentDelta returns the delta set for the current cycle
func (wse *WriteStateEngine) GetCurrentDelta() *DeltaSet {
	// Optimize deltas before returning
	wse.currentDelta.OptimizeDeltas()
	return wse.currentDelta
}

// HasPrimaryKeys returns true if any primary keys are configured
func (wse *WriteStateEngine) HasPrimaryKeys() bool {
	return len(wse.primaryKeys) > 0
}

// CalculateDeletes identifies records that were removed by omission
func (wse *WriteStateEngine) CalculateDeletes() {
	if !wse.HasPrimaryKeys() || wse.previousState == nil {
		return // Can't calculate deletes without primary keys or previous state
	}
	
	// For each type with primary keys, find missing records
	for typeName, primaryKeyField := range wse.primaryKeys {
		wse.calculateDeletesForType(typeName, primaryKeyField)
	}
}

func (wse *WriteStateEngine) calculateDeletesForType(typeName, primaryKeyField string) {
	// Get records from previous state
	previousRecords, exists := wse.previousState.data[typeName]
	if !exists || len(previousRecords) == 0 {
		return // No previous records to check
	}
	
	// Build set of current primary key values
	currentKeys := make(map[string]bool)
	if currentRecords, exists := wse.currentState.data[typeName]; exists {
		for _, record := range currentRecords {
			if recordInfo, ok := record.(*RecordInfo); ok {
				if keyValue, err := wse.extractPrimaryKeyValue(recordInfo.Value, primaryKeyField, typeName); err == nil {
					currentKeys[keyValue] = true
				}
			}
		}
	}
	
	// Find missing records from previous state
	for _, record := range previousRecords {
		var value interface{}
		if recordInfo, ok := record.(*RecordInfo); ok {
			value = recordInfo.Value
		} else {
			value = record
		}
		
		if keyValue, err := wse.extractPrimaryKeyValue(value, primaryKeyField, typeName); err == nil {
			if !currentKeys[keyValue] {
				// This record was deleted by omission
				if ordinal, exists := wse.identityMap.GetOrdinal(typeName, keyValue); exists {
					wse.currentDelta.AddRecord(typeName, DeltaDelete, ordinal, nil)
				}
			}
		}
	}
}

// hasValueChanged checks if a value has actually changed compared to previous state
func (wse *WriteStateEngine) hasValueChanged(typeName string, ordinal int, newValue interface{}) bool {
	if wse.previousState == nil {
		return true // No previous state, so it's definitely changed
	}
	
	// Look for the previous value by ordinal in the previous state
	previousRecords, exists := wse.previousState.data[typeName]
	if !exists {
		return true // Type didn't exist before, so it's new
	}
	
	// Find the record with this ordinal in previous state
	for _, record := range previousRecords {
		var ordinalToCheck int
		var valueToCheck interface{}
		
		if recordInfo, ok := record.(*RecordInfo); ok {
			ordinalToCheck = recordInfo.Ordinal
			valueToCheck = recordInfo.Value
		} else {
			// For non-RecordInfo records, we can't easily determine ordinals
			// so we'll consider it changed to be safe
			return true
		}
		
		if ordinalToCheck == ordinal {
			// Found the matching record, compare values
			return !wse.valuesEqual(valueToCheck, newValue)
		}
	}
	
	// Ordinal not found in previous state, so it's new
	return true
}

// valuesEqual compares two values for equality (simplified comparison)
func (wse *WriteStateEngine) valuesEqual(a, b interface{}) bool {
	// For this implementation, use string representation comparison
	// In a production system, you'd want more sophisticated comparison
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// AddWithPrimaryKey adds a value with primary key identity management
// This method handles both Cap'n Proto structs (zero-copy) and traditional Go objects
func (wse *WriteStateEngine) AddWithPrimaryKey(value interface{}) error {
	typeName := getTypeName(value)
	
	// Check if this type has a configured primary key
	primaryKeyField, hasPrimaryKey := wse.primaryKeys[typeName]
	if !hasPrimaryKey {
		// No primary key configured, use traditional Add
		wse.currentState.addTraditional(value)
		return nil
	}
	
	// Extract primary key value using zero-copy method if possible
	primaryKeyValue, err := wse.extractPrimaryKeyValue(value, primaryKeyField, typeName)
	if err != nil {
		return fmt.Errorf("failed to extract primary key %s from %s: %w", primaryKeyField, typeName, err)
	}
	
	// Get or create ordinal for this record
	ordinal, isUpdate := wse.identityMap.GetOrCreateOrdinal(typeName, primaryKeyValue)
	
	// Add with identity information
	wse.currentState.AddWithIdentity(typeName, value, ordinal, isUpdate)
	
	// Track delta operation only if it's a real change
	if isUpdate {
		// For updates, check if the value actually changed compared to previous state
		if wse.hasValueChanged(typeName, ordinal, value) {
			wse.currentDelta.AddRecord(typeName, DeltaUpdate, ordinal, value)
		}
	} else {
		// New records are always tracked as additions
		wse.currentDelta.AddRecord(typeName, DeltaAdd, ordinal, value)
	}
	
	return nil
}

// extractPrimaryKeyValue extracts the primary key value from a record
// This method supports both Cap'n Proto structs (zero-copy) and traditional Go objects
func (wse *WriteStateEngine) extractPrimaryKeyValue(value interface{}, fieldName string, typeName string) (string, error) {
	// Check if this is a Cap'n Proto struct for zero-copy access
	if capnpStruct, ok := value.(capnp.Struct); ok {
		return wse.extractCapnProtoPrimaryKey(capnpStruct, fieldName, typeName)
	}
	
	// Fallback to reflection for traditional Go objects
	return wse.extractReflectionPrimaryKey(value, fieldName)
}

// extractCapnProtoPrimaryKey extracts primary key from Cap'n Proto struct (zero-copy)
func (wse *WriteStateEngine) extractCapnProtoPrimaryKey(s capnp.Struct, fieldName string, typeName string) (string, error) {
	// This implements zero-copy primary key extraction
	// Cap'n Proto provides direct field access without deserialization
	
	// For demonstration, we'll show the pattern for different field types
	// In a real implementation, this would use the generated accessors
	
	// Example: For a Movie struct with an "id" field
	if typeName == "Movie" && fieldName == "id" {
		// Use the generated accessor method (zero-copy)
		// id := movie.Id() // This would be the actual call
		// return fmt.Sprintf("%d", id), nil
		
		// For now, we'll extract using the generic struct interface
		// This still avoids full deserialization
		segment := s.Segment()
		if segment == nil {
			return "", fmt.Errorf("invalid Cap'n Proto struct")
		}
		
		// This is a placeholder - real implementation would use generated accessors
		return "capnproto-id", nil
	}
	
	return "", fmt.Errorf("unsupported Cap'n Proto type/field combination: %s.%s", typeName, fieldName)
}

// ExtractReflectionPrimaryKey extracts primary key using reflection (fallback)
// Exported for testing purposes
func (wse *WriteStateEngine) ExtractReflectionPrimaryKey(value interface{}, fieldName string) (string, error) {
	return wse.extractReflectionPrimaryKey(value, fieldName)
}

// extractReflectionPrimaryKey extracts primary key using reflection (fallback)
func (wse *WriteStateEngine) extractReflectionPrimaryKey(value interface{}, fieldName string) (string, error) {
	v := reflect.ValueOf(value)
	
	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", fmt.Errorf("nil pointer value")
		}
		v = v.Elem()
	}
	
	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return "", fmt.Errorf("value is not a struct")
	}
	
	// Find the field
	fieldValue := v.FieldByName(fieldName)
	if !fieldValue.IsValid() {
		return "", fmt.Errorf("field %s not found", fieldName)
	}
	
	// Convert to string representation
	return fmt.Sprintf("%v", fieldValue.Interface()), nil
}

// ReadStateEngine manages read operations
type ReadStateEngine struct {
	currentState *ReadState
	typeFilter   *TypeFilter
	memoryMode   MemoryMode
	mutex        sync.RWMutex
}

func NewReadStateEngine() *ReadStateEngine {
	return &ReadStateEngine{}
}

func (rse *ReadStateEngine) GetCurrentVersion() int64 {
	rse.mutex.RLock()
	defer rse.mutex.RUnlock()
	if rse.currentState == nil {
		return 0
	}
	return rse.currentState.GetVersion()
}

func (rse *ReadStateEngine) GetCurrentState() *ReadState {
	rse.mutex.RLock()
	defer rse.mutex.RUnlock()
	return rse.currentState
}

func (rse *ReadStateEngine) SetCurrentState(state *ReadState) {
	rse.mutex.Lock()
	defer rse.mutex.Unlock()
	// Invalidate old state
	if rse.currentState != nil {
		rse.currentState.Invalidate()
	}
	rse.currentState = state
}

func (rse *ReadStateEngine) HasType(typeName string) bool {
	rse.mutex.RLock()
	defer rse.mutex.RUnlock()
	if rse.currentState == nil {
		return false
	}
	data := rse.currentState.data[typeName]
	return data != nil
}

// TypeFilter for selective type loading
type TypeFilter struct {
	included map[string]bool
	excluded map[string]bool
	excludeAll bool
}

func NewTypeFilter() *TypeFilter {
	return &TypeFilter{
		included: make(map[string]bool),
		excluded: make(map[string]bool),
	}
}

func (tf *TypeFilter) Include(typeName string) *TypeFilter {
	tf.included[typeName] = true
	return tf
}

func (tf *TypeFilter) Exclude(typeName string) *TypeFilter {
	tf.excluded[typeName] = true
	return tf
}

func (tf *TypeFilter) ExcludeAll() *TypeFilter {
	tf.excludeAll = true
	return tf
}

func (tf *TypeFilter) ShouldInclude(typeName string) bool {
	if tf.excludeAll {
		return tf.included[typeName]
	}
	return !tf.excluded[typeName]
}

// MemoryMode represents different memory management modes
type MemoryMode int

const (
	HeapMemory MemoryMode = iota
	SharedMemoryLazy
	SharedMemoryDirect
)

// Validation types
type ValidationType int

const (
	ValidationPassed ValidationType = iota
	ValidationFailed
	ValidationWarning
)

type ValidationResult struct {
	Type    ValidationType
	Message string
}

type Validator interface {
	Name() string
	Validate(readState *ReadState) ValidationResult
}

// Utility functions
func getTypeName(value interface{}) string {
	switch v := value.(type) {
	case string:
		return "String"
	case int, int32, int64:
		return "Integer"
	case float32, float64:
		return "Float"
	case bool:
		return "Boolean"
	case []byte:
		return "Bytes"
	default:
		// Use the struct type name for structs
		return fmt.Sprintf("%T", v)
	}
}

func GetTypeName(value interface{}) string {
	return getTypeName(value)
}

// Single producer enforcement
type SingleProducerEnforcer struct {
	enabled bool
}

func NewSingleProducerEnforcer() *SingleProducerEnforcer {
	return &SingleProducerEnforcer{enabled: true}
}

func (spe *SingleProducerEnforcer) Enable() {
	spe.enabled = true
}

func (spe *SingleProducerEnforcer) Disable() {
	spe.enabled = false
}

func (spe *SingleProducerEnforcer) IsEnabled() bool {
	return spe.enabled
}
