// Package internal provides core serialization interfaces that support both traditional and zero-copy modes
package internal

import (
	"context"
	"fmt"

	"capnproto.org/go/capnp/v3"
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
	
	// Deserialize converts blob bytes back to accessible data format
	Deserialize(ctx context.Context, data []byte) (DeserializedData, error)
	
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
