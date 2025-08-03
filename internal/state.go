package internal

import (
	"fmt"
	"sync"
	"sync/atomic"
	"github.com/leowmjw/go-hollow/schema"
)

// WriteState represents the state during write operations
type WriteState struct {
	version int64
	data    map[string][]interface{}
	schemas map[string]schema.Schema
}

func NewWriteState() *WriteState {
	return &WriteState{
		data:    make(map[string][]interface{}),
		schemas: make(map[string]schema.Schema),
	}
}

func (ws *WriteState) Add(value interface{}) {
	typeName := getTypeName(value)
	if ws.data[typeName] == nil {
		ws.data[typeName] = make([]interface{}, 0)
	}
	ws.data[typeName] = append(ws.data[typeName], value)
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

// WriteStateEngine manages write operations
type WriteStateEngine struct {
	currentState   *WriteState
	headerTags     map[string]string
	populatedCount int32
	shardCounts    map[string]int
}

func NewWriteStateEngine() *WriteStateEngine {
	return &WriteStateEngine{
		currentState: NewWriteState(),
		headerTags:   make(map[string]string),
		shardCounts:  make(map[string]int),
	}
}

func (wse *WriteStateEngine) PrepareForCycle() {
	wse.currentState = NewWriteState()
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
