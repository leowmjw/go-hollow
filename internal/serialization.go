// Package internal provides core serialization interfaces that support both traditional and zero-copy modes
package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"capnproto.org/go/capnp/v3"
	delta "github.com/leowmjw/go-hollow/generated/go/delta/schemas"
)

// SerializationMode determines how data is serialized and accessed
type SerializationMode int

const (
	// TraditionalMode uses Go struct serialization (current behavior)
	TraditionalMode SerializationMode = iota
	// ZeroCopyMode uses Cap'n Proto zero-copy serialization
	ZeroCopyMode
	// HybridMode supports both modes for gradual migration
	HybridMode
)

// Serializer provides pluggable serialization for different data formats
type Serializer interface {
	// Serialize converts WriteState data to bytes for blob storage
	Serialize(ctx context.Context, writeState *WriteState) ([]byte, error)
	
	// SerializeDelta converts a DeltaSet to bytes for efficient delta storage
	SerializeDelta(ctx context.Context, deltaSet *DeltaSet) ([]byte, error)
	
	// Deserialize converts blob bytes back to accessible data format
	Deserialize(ctx context.Context, data []byte) (DeserializedData, error)
	
	// DeserializeDelta converts delta blob bytes to DeltaSet
	DeserializeDelta(ctx context.Context, data []byte) (*DeltaSet, error)
	
	// Mode returns the serialization mode
	Mode() SerializationMode
}

// DeserializedData represents data that can be accessed after deserialization
type DeserializedData interface {
	// GetData returns data in the format expected by the current consumer API
	GetData() map[string][]interface{}
	
	// GetZeroCopyView returns a zero-copy view if supported (optional)
	GetZeroCopyView() (ZeroCopyView, bool)
	
	// Size returns the data size for metrics
	Size() int
}

// ZeroCopyView provides direct access to Cap'n Proto data without copying
type ZeroCopyView interface {
	// GetMessage returns the underlying Cap'n Proto message
	GetMessage() *capnp.Message
	
	// GetRootStruct returns the root struct for type-safe access
	GetRootStruct() (capnp.Struct, error)
	
	// GetByteBuffer returns direct access to the underlying buffer
	GetByteBuffer() []byte
}

// TraditionalSerializer implements the current serialization approach
type TraditionalSerializer struct{}

func NewTraditionalSerializer() *TraditionalSerializer {
	return &TraditionalSerializer{}
}

func (s *TraditionalSerializer) Mode() SerializationMode {
	return TraditionalMode
}

func (s *TraditionalSerializer) Serialize(ctx context.Context, writeState *WriteState) ([]byte, error) {
	// Current implementation - serialize Go objects to bytes
	// This maintains backward compatibility
	data := writeState.GetData()
	
	// Simple byte serialization (current approach)
	// In real implementation, this would use the existing serialization logic
	serialized := fmt.Sprintf("%v", data)
	return []byte(serialized), nil
}

func (s *TraditionalSerializer) Deserialize(ctx context.Context, data []byte) (DeserializedData, error) {
	// Current implementation - deserialize to Go map
	// This maintains backward compatibility with existing consumer API
	
	// Parse back to the expected format
	// In real implementation, this would use the existing deserialization logic
	result := &TraditionalData{
		data: make(map[string][]interface{}),
		size: len(data),
	}
	
	// For demo purposes, create some mock data
	result.data["default"] = []interface{}{string(data)}
	
	return result, nil
}

func (s *TraditionalSerializer) SerializeDelta(ctx context.Context, deltaSet *DeltaSet) ([]byte, error) {
	// Traditional mode doesn't optimize for deltas, falls back to basic serialization
	serialized := fmt.Sprintf("DeltaSet:%v", deltaSet)
	return []byte(serialized), nil
}

func (s *TraditionalSerializer) DeserializeDelta(ctx context.Context, data []byte) (*DeltaSet, error) {
	// Traditional mode creates empty delta set (not optimized)
	return NewDeltaSet(), nil
}

// TraditionalData implements DeserializedData for traditional mode
type TraditionalData struct {
	data map[string][]interface{}
	size int
}

func (d *TraditionalData) GetData() map[string][]interface{} {
	return d.data
}

func (d *TraditionalData) GetZeroCopyView() (ZeroCopyView, bool) {
	return nil, false // Traditional mode doesn't support zero-copy
}

func (d *TraditionalData) Size() int {
	return d.size
}

// CapnProtoSerializer implements zero-copy serialization using Cap'n Proto
type CapnProtoSerializer struct {
	schemaRegistry map[string]uint64 // Maps type names to Cap'n Proto type IDs
}

func NewCapnProtoSerializer() *CapnProtoSerializer {
	return &CapnProtoSerializer{
		schemaRegistry: make(map[string]uint64),
	}
}

func (s *CapnProtoSerializer) Mode() SerializationMode {
	return ZeroCopyMode
}

func (s *CapnProtoSerializer) RegisterSchema(typeName string, typeID uint64) {
	s.schemaRegistry[typeName] = typeID
}

func (s *CapnProtoSerializer) Serialize(ctx context.Context, writeState *WriteState) ([]byte, error) {
	// Create Cap'n Proto message with all data from WriteState
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create Cap'n Proto message: %w", err)
	}
	
	// Serialize WriteState data to Cap'n Proto format
	err = s.serializeWriteStateToCapnProto(writeState, seg)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize to Cap'n Proto: %w", err)
	}
	
	// Marshal to bytes - this is the only copy operation
	data, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Cap'n Proto message: %w", err)
	}
	
	return data, nil
}

func (s *CapnProtoSerializer) Deserialize(ctx context.Context, data []byte) (DeserializedData, error) {
	// Zero-copy deserialization - just parse the message structure
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal Cap'n Proto message: %w", err)
	}
	
	// Return zero-copy view that keeps reference to original buffer
	result := &CapnProtoData{
		message: msg,
		buffer:  data,
	}
	
	return result, nil
}

func (s *CapnProtoSerializer) SerializeDelta(ctx context.Context, deltaSet *DeltaSet) ([]byte, error) {
	// Create Cap'n Proto message for efficient delta storage
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to create Cap'n Proto delta message: %w", err)
	}
	
	// This would serialize the delta set using a Cap'n Proto schema
	// For now, use a simple approach
	err = s.serializeDeltaToCapnProto(deltaSet, seg)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize delta to Cap'n Proto: %w", err)
	}
	
	// Marshal to bytes with packed encoding for compression
	data, err := msg.MarshalPacked()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal packed Cap'n Proto delta: %w", err)
	}
	
	return data, nil
}

func (s *CapnProtoSerializer) DeserializeDelta(ctx context.Context, data []byte) (*DeltaSet, error) {
	// Unmarshal packed Cap'n Proto delta
	msg, err := capnp.UnmarshalPacked(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal packed Cap'n Proto delta: %w", err)
	}
	
	// Convert back to DeltaSet
	deltaSet, err := s.deserializeDeltaFromCapnProto(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize delta from Cap'n Proto: %w", err)
	}
	
	return deltaSet, nil
}

func (s *CapnProtoSerializer) serializeWriteStateToCapnProto(writeState *WriteState, seg *capnp.Segment) error {
	// This would contain the logic to convert WriteState data to Cap'n Proto structures
	// For now, this is a placeholder that demonstrates the integration point
	
	// In a real implementation, this would:
	// 1. Inspect the types in WriteState.GetData()
	// 2. Create appropriate Cap'n Proto structs based on registered schemas
	// 3. Populate the structs with data from WriteState
	// 4. Set the root pointer to the serialized data
	
	return nil // Placeholder - actual implementation would populate the segment
}

func (s *CapnProtoSerializer) serializeDeltaToCapnProto(deltaSet *DeltaSet, seg *capnp.Segment) error {
	// Import the generated delta schema
	deltaSchema, err := delta.NewRootDeltaSet(seg)
	if err != nil {
		return fmt.Errorf("failed to create delta set root: %w", err)
	}
	
	// Set metadata
	deltaSchema.SetOptimized(deltaSet.optimized)
	deltaSchema.SetChangeCount(uint32(deltaSet.changeCount))
	
	// Create list of type deltas
	typeDeltas, err := deltaSchema.NewDeltas(int32(len(deltaSet.Deltas)))
	if err != nil {
		return fmt.Errorf("failed to create deltas list: %w", err)
	}
	
	i := 0
	for typeName, typeDelta := range deltaSet.Deltas {
		typeDeltaCapnp := typeDeltas.At(i)
		typeDeltaCapnp.SetTypeName(typeName)
		
		// Create list of delta records for this type
		records, err := typeDeltaCapnp.NewRecords(int32(len(typeDelta.Records)))
		if err != nil {
			return fmt.Errorf("failed to create records list for type %s: %w", typeName, err)
		}
		
		for j, record := range typeDelta.Records {
			recordCapnp := records.At(j)
			
			// Set operation
			switch record.Operation {
			case DeltaAdd:
				recordCapnp.SetOperation(delta.DeltaOperation_add)
			case DeltaUpdate:
				recordCapnp.SetOperation(delta.DeltaOperation_update)
			case DeltaDelete:
				recordCapnp.SetOperation(delta.DeltaOperation_delete)
			}
			
			recordCapnp.SetOrdinal(uint32(record.Ordinal))
			
			// Serialize the value if it's not a delete operation
			if record.Operation != DeltaDelete && record.Value != nil {
				valueData, err := s.serializeValue(record.Value)
				if err != nil {
					return fmt.Errorf("failed to serialize value for record %d: %w", record.Ordinal, err)
				}
				recordCapnp.SetValue(valueData)
			}
		}
		
		i++
	}
	
	return nil
}

// serializeValue serializes a value to bytes using JSON for now
// In a production implementation, this could use more efficient serialization
func (s *CapnProtoSerializer) serializeValue(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}

func (s *CapnProtoSerializer) deserializeDeltaFromCapnProto(msg *capnp.Message) (*DeltaSet, error) {
	// Parse the Cap'n Proto delta message
	deltaRoot, err := delta.ReadRootDeltaSet(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to read delta set root: %w", err)
	}
	
	deltaSet := NewDeltaSet()
	deltaSet.optimized = deltaRoot.Optimized()
	deltaSet.changeCount = int(deltaRoot.ChangeCount())
	
	// Parse type deltas
	typeDeltas, err := deltaRoot.Deltas()
	if err != nil {
		return nil, fmt.Errorf("failed to get deltas list: %w", err)
	}
	
	for i := 0; i < typeDeltas.Len(); i++ {
		typeDelta := typeDeltas.At(i)
		typeName, err := typeDelta.TypeName()
		if err != nil {
			return nil, fmt.Errorf("failed to get type name for delta %d: %w", i, err)
		}
		
		records, err := typeDelta.Records()
		if err != nil {
			return nil, fmt.Errorf("failed to get records for type %s: %w", typeName, err)
		}
		
		// Create TypeDelta
		td := &TypeDelta{
			TypeName: typeName,
			Records:  make([]DeltaRecord, records.Len()),
		}
		
		for j := 0; j < records.Len(); j++ {
			record := records.At(j)
			
			// Convert operation
			var operation DeltaOperation
			switch record.Operation() {
			case delta.DeltaOperation_add:
				operation = DeltaAdd
			case delta.DeltaOperation_update:
				operation = DeltaUpdate
			case delta.DeltaOperation_delete:
				operation = DeltaDelete
			}
			
			// Deserialize value if present
			var value interface{}
			if operation != DeltaDelete {
				valueData, err := record.Value()
				if err == nil && len(valueData) > 0 {
					err = json.Unmarshal(valueData, &value)
					if err != nil {
						return nil, fmt.Errorf("failed to deserialize value for record %d: %w", j, err)
					}
				}
			}
			
			td.Records[j] = DeltaRecord{
				Operation: operation,
				Ordinal:   int(record.Ordinal()),
				Value:     value,
			}
		}
		
		deltaSet.Deltas[typeName] = td
	}
	
	return deltaSet, nil
}

// CapnProtoData implements DeserializedData for zero-copy mode
type CapnProtoData struct {
	message *capnp.Message
	buffer  []byte
	
	// Cached traditional data for backward compatibility
	cachedData map[string][]interface{}
}

func (d *CapnProtoData) GetData() map[string][]interface{} {
	// Lazy conversion to traditional format for backward compatibility
	if d.cachedData == nil {
		d.cachedData = d.convertToTraditionalFormat()
	}
	return d.cachedData
}

func (d *CapnProtoData) GetZeroCopyView() (ZeroCopyView, bool) {
	return &CapnProtoView{
		message: d.message,
		buffer:  d.buffer,
	}, true
}

func (d *CapnProtoData) Size() int {
	return len(d.buffer)
}

func (d *CapnProtoData) convertToTraditionalFormat() map[string][]interface{} {
	// Convert Cap'n Proto data to traditional map format
	// This is for backward compatibility with existing consumer API
	
	result := make(map[string][]interface{})
	
	// This would contain logic to traverse the Cap'n Proto message
	// and extract data into the traditional format
	// For demo purposes, create placeholder data
	result["capnproto"] = []interface{}{"zero-copy data"}
	
	return result
}

// CapnProtoView implements ZeroCopyView
type CapnProtoView struct {
	message *capnp.Message
	buffer  []byte
}

func (v *CapnProtoView) GetMessage() *capnp.Message {
	return v.message
}

func (v *CapnProtoView) GetRootStruct() (capnp.Struct, error) {
	root, err := v.message.Root()
	if err != nil {
		return capnp.Struct{}, fmt.Errorf("failed to get root struct: %w", err)
	}
	return root.Struct(), nil
}

func (v *CapnProtoView) GetByteBuffer() []byte {
	return v.buffer
}

// HybridSerializer supports both traditional and zero-copy modes
type HybridSerializer struct {
	traditional *TraditionalSerializer
	capnproto   *CapnProtoSerializer
	defaultMode SerializationMode
}

func NewHybridSerializer(defaultMode SerializationMode) *HybridSerializer {
	return &HybridSerializer{
		traditional: NewTraditionalSerializer(),
		capnproto:   NewCapnProtoSerializer(),
		defaultMode: defaultMode,
	}
}

func (s *HybridSerializer) Mode() SerializationMode {
	return HybridMode
}

func (s *HybridSerializer) Serialize(ctx context.Context, writeState *WriteState) ([]byte, error) {
	// Choose serialization mode based on data characteristics or configuration
	mode := s.determineSerializationMode(writeState)
	
	switch mode {
	case ZeroCopyMode:
		return s.capnproto.Serialize(ctx, writeState)
	case TraditionalMode:
		return s.traditional.Serialize(ctx, writeState)
	default:
		return s.traditional.Serialize(ctx, writeState)
	}
}

func (s *HybridSerializer) Deserialize(ctx context.Context, data []byte) (DeserializedData, error) {
	// Detect format from data header or metadata
	mode := s.detectSerializationMode(data)
	
	switch mode {
	case ZeroCopyMode:
		return s.capnproto.Deserialize(ctx, data)
	case TraditionalMode:
		return s.traditional.Deserialize(ctx, data)
	default:
		return s.traditional.Deserialize(ctx, data)
	}
}

func (s *HybridSerializer) determineSerializationMode(writeState *WriteState) SerializationMode {
	// Logic to choose serialization mode based on:
	// - Data size (large datasets benefit from zero-copy)
	// - Data types (Cap'n Proto schemas available)
	// - Performance requirements
	// - Configuration settings
	
	data := writeState.GetData()
	totalItems := 0
	for _, items := range data {
		totalItems += len(items)
	}
	
	// Use zero-copy for large datasets
	if totalItems > 1000 {
		return ZeroCopyMode
	}
	
	return s.defaultMode
}

func (s *HybridSerializer) detectSerializationMode(data []byte) SerializationMode {
	// Detect format by examining data header or magic bytes
	// Cap'n Proto messages have a specific structure that can be detected
	
	if len(data) >= 8 {
		// Cap'n Proto messages start with a specific pattern
		// This is a simplified detection - real implementation would be more robust
		if data[0] == 0x00 && data[1] == 0x00 {
			return ZeroCopyMode
		}
	}
	
	return TraditionalMode
}

func (s *HybridSerializer) SerializeDelta(ctx context.Context, deltaSet *DeltaSet) ([]byte, error) {
	// Choose serialization mode based on delta characteristics
	mode := s.determineSerializationModeForDelta(deltaSet)
	
	switch mode {
	case ZeroCopyMode:
		return s.capnproto.SerializeDelta(ctx, deltaSet)
	case TraditionalMode:
		return s.traditional.SerializeDelta(ctx, deltaSet)
	default:
		return s.traditional.SerializeDelta(ctx, deltaSet)
	}
}

func (s *HybridSerializer) DeserializeDelta(ctx context.Context, data []byte) (*DeltaSet, error) {
	// Detect format from data header or metadata
	mode := s.detectSerializationMode(data)
	
	switch mode {
	case ZeroCopyMode:
		return s.capnproto.DeserializeDelta(ctx, data)
	case TraditionalMode:
		return s.traditional.DeserializeDelta(ctx, data)
	default:
		return s.traditional.DeserializeDelta(ctx, data)
	}
}

func (s *HybridSerializer) determineSerializationModeForDelta(deltaSet *DeltaSet) SerializationMode {
	// Use zero-copy for deltas with significant change count
	if deltaSet.GetChangeCount() > 100 {
		return ZeroCopyMode
	}
	
	return s.defaultMode
}

// SerializerFactory creates appropriate serializers based on configuration
type SerializerFactory struct {
	defaultMode SerializationMode
}

func NewSerializerFactory(defaultMode SerializationMode) *SerializerFactory {
	return &SerializerFactory{defaultMode: defaultMode}
}

func (f *SerializerFactory) CreateSerializer() Serializer {
	switch f.defaultMode {
	case ZeroCopyMode:
		return NewCapnProtoSerializer()
	case HybridMode:
		return NewHybridSerializer(ZeroCopyMode)
	case TraditionalMode:
		fallthrough
	default:
		return NewTraditionalSerializer()
	}
}
