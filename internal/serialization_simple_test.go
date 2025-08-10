package internal

import (
	"context"
	"testing"
	"reflect"
)

func TestSerializationModes(t *testing.T) {
	modes := []SerializationMode{TraditionalMode, ZeroCopyMode, HybridMode}
	for _, mode := range modes {
		if int(mode) < 0 || int(mode) > 2 {
			t.Errorf("unexpected serialization mode value: %d", mode)
		}
	}
}

func TestTraditionalSerializer_Basic(t *testing.T) {
	serializer := NewTraditionalSerializer()
	if serializer == nil {
		t.Fatal("expected serializer to be created")
	}
	if serializer.Mode() != TraditionalMode {
		t.Errorf("expected TraditionalMode, got %v", serializer.Mode())
	}

	ctx := context.Background()
	writeState := NewWriteState()
	writeState.data["Movie"] = []interface{}{
		map[string]interface{}{
			"ID":    "1",
			"Title": "The Matrix",
		},
	}

	// Test serialization
	data, err := serializer.Serialize(ctx, writeState)
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty serialized data")
	}

	// Test deserialization
	deserializedData, err := serializer.Deserialize(ctx, data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	result := deserializedData.GetData()
	if len(result) == 0 {
		t.Error("expected non-empty deserialized data")
	}

	// Test zero-copy view (should not be supported)
	_, supported := deserializedData.GetZeroCopyView()
	if supported {
		t.Error("traditional serializer should not support zero-copy view")
	}
}

func TestTraditionalSerializer_RawData(t *testing.T) {
	serializer := NewTraditionalSerializer()
	ctx := context.Background()

	rawData := []byte("raw test data")
	deserializedData, err := serializer.Deserialize(ctx, rawData)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	result := deserializedData.GetData()
	if stringData, ok := result["String"]; ok {
		if len(stringData) != 1 {
			t.Errorf("expected 1 string item, got %d", len(stringData))
		}
		if stringData[0] != "raw test data" {
			t.Errorf("expected 'raw test data', got %v", stringData[0])
		}
	} else {
		t.Error("expected String type in deserialized data")
	}
}

func TestTraditionalSerializer_Delta(t *testing.T) {
	serializer := NewTraditionalSerializer()
	ctx := context.Background()

	deltaSet := NewDeltaSet()
	deltaSet.AddRecord("Movie", DeltaAdd, 1, map[string]interface{}{
		"ID":    "1",
		"Title": "New Movie",
	})

	data, err := serializer.SerializeDelta(ctx, deltaSet)
	if err != nil {
		t.Fatalf("serialize delta failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty serialized delta data")
	}

	deserializedDelta, err := serializer.DeserializeDelta(ctx, data)
	if err != nil {
		t.Fatalf("deserialize delta failed: %v", err)
	}
	if deserializedDelta == nil {
		t.Error("expected non-nil deserialized delta")
	}
}

func TestCapnProtoSerializer_Basic(t *testing.T) {
	serializer := NewCapnProtoSerializer()
	if serializer == nil {
		t.Fatal("expected serializer to be created")
	}
	if serializer.Mode() != ZeroCopyMode {
		t.Errorf("expected ZeroCopyMode, got %v", serializer.Mode())
	}

	// Test schema registration
	serializer.RegisterSchema("Movie", 12345)
	if typeID, exists := serializer.schemaRegistry["Movie"]; !exists {
		t.Error("expected Movie schema to be registered")
	} else if typeID != 12345 {
		t.Errorf("expected type ID 12345, got %d", typeID)
	}

	ctx := context.Background()
	writeState := NewWriteState()
	writeState.data["Movie"] = []interface{}{
		map[string]interface{}{
			"ID":    "1",
			"Title": "The Matrix",
		},
	}

	data, err := serializer.Serialize(ctx, writeState)
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty serialized data")
	}

	deserializedData, err := serializer.Deserialize(ctx, data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	// Test zero-copy view
	view, supported := deserializedData.GetZeroCopyView()
	if !supported {
		t.Error("Cap'n Proto serializer should support zero-copy view")
	}
	if view == nil {
		t.Error("expected non-nil zero-copy view")
	}

	if view.GetMessage() == nil {
		t.Error("expected non-nil message from zero-copy view")
	}
	if len(view.GetByteBuffer()) == 0 {
		t.Error("expected non-empty byte buffer from zero-copy view")
	}
}

func TestCapnProtoSerializer_Delta(t *testing.T) {
	serializer := NewCapnProtoSerializer()
	ctx := context.Background()

	deltaSet := NewDeltaSet()
	deltaSet.AddRecord("Movie", DeltaAdd, 1, map[string]interface{}{"ID": "1"})
	deltaSet.AddRecord("Movie", DeltaUpdate, 2, map[string]interface{}{"ID": "2"})
	deltaSet.AddRecord("Movie", DeltaDelete, 3, nil)

	data, err := serializer.SerializeDelta(ctx, deltaSet)
	if err != nil {
		t.Fatalf("serialize delta failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty serialized delta data")
	}

	deserializedDelta, err := serializer.DeserializeDelta(ctx, data)
	if err != nil {
		t.Fatalf("deserialize delta failed: %v", err)
	}
	if deserializedDelta == nil {
		t.Error("expected non-nil deserialized delta")
	}
	if deserializedDelta.GetChangeCount() != 3 {
		t.Errorf("expected 3 changes, got %d", deserializedDelta.GetChangeCount())
	}
}

func TestHybridSerializer_Basic(t *testing.T) {
	serializer := NewHybridSerializer(ZeroCopyMode)
	if serializer == nil {
		t.Fatal("expected serializer to be created")
	}
	if serializer.Mode() != HybridMode {
		t.Errorf("expected HybridMode, got %v", serializer.Mode())
	}
	if serializer.defaultMode != ZeroCopyMode {
		t.Errorf("expected default mode ZeroCopyMode, got %v", serializer.defaultMode)
	}

	ctx := context.Background()
	writeState := NewWriteState()
	writeState.data["Movie"] = []interface{}{
		map[string]interface{}{"ID": "1", "Title": "Test"},
	}

	data, err := serializer.Serialize(ctx, writeState)
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	deserializedData, err := serializer.Deserialize(ctx, data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	result := deserializedData.GetData()
	if len(result) == 0 {
		t.Error("expected non-empty deserialized data")
	}
}

func TestHybridSerializer_ModeDetection(t *testing.T) {
	serializer := NewHybridSerializer(TraditionalMode)

	// Small dataset
	smallState := NewWriteState()
	smallState.data["Test"] = []interface{}{1, 2, 3}
	mode := serializer.determineSerializationMode(smallState)
	if mode != TraditionalMode {
		t.Errorf("expected TraditionalMode for small dataset, got %v", mode)
	}

	// Large dataset
	largeState := NewWriteState()
	var largeList []interface{}
	for i := 0; i < 1100; i++ {
		largeList = append(largeList, i)
	}
	largeState.data["Test"] = largeList
	mode = serializer.determineSerializationMode(largeState)
	if mode != ZeroCopyMode {
		t.Errorf("expected ZeroCopyMode for large dataset, got %v", mode)
	}

	// Test format detection
	capnprotoData := []byte{0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06} // 8 bytes
	mode = serializer.detectSerializationMode(capnprotoData)
	if mode != ZeroCopyMode {
		t.Errorf("expected ZeroCopyMode (%d) for Cap'n Proto data, got %d", ZeroCopyMode, mode)
	}

	traditionalData := []byte(`{"test": "data"}`)
	mode = serializer.detectSerializationMode(traditionalData)
	if mode != TraditionalMode {
		t.Errorf("expected TraditionalMode for traditional data, got %v", mode)
	}
}

func TestSerializerFactory(t *testing.T) {
	testCases := []struct {
		mode     SerializationMode
		expected reflect.Type
	}{
		{TraditionalMode, reflect.TypeOf(&TraditionalSerializer{})},
		{ZeroCopyMode, reflect.TypeOf(&CapnProtoSerializer{})},
		{HybridMode, reflect.TypeOf(&HybridSerializer{})},
	}

	for _, tc := range testCases {
		factory := NewSerializerFactory(tc.mode)
		serializer := factory.CreateSerializer()
		
		actualType := reflect.TypeOf(serializer)
		if actualType != tc.expected {
			t.Errorf("expected %v, got %v for mode %v", tc.expected, actualType, tc.mode)
		}
	}
}

func TestTraditionalData_Methods(t *testing.T) {
	data := &TraditionalData{
		data: map[string][]interface{}{
			"Test": {"value1", "value2"},
		},
		size: 100,
	}

	// Test GetData
	result := data.GetData()
	if len(result) != 1 {
		t.Errorf("expected 1 type, got %d", len(result))
	}

	// Test Size
	if data.Size() != 100 {
		t.Errorf("expected size 100, got %d", data.Size())
	}

	// Test GetZeroCopyView
	view, supported := data.GetZeroCopyView()
	if supported {
		t.Error("traditional data should not support zero-copy view")
	}
	if view != nil {
		t.Error("expected nil view for traditional data")
	}
}

func TestSerializeValue(t *testing.T) {
	serializer := NewCapnProtoSerializer()

	testCases := []interface{}{
		"string value",
		123,
		map[string]interface{}{"key": "value"},
		[]string{"item1", "item2"},
	}

	for _, testCase := range testCases {
		data, err := serializer.serializeValue(testCase)
		if err != nil {
			t.Errorf("failed to serialize value %v: %v", testCase, err)
		}
		if len(data) == 0 {
			t.Errorf("expected non-empty serialized data for %v", testCase)
		}
	}
}

func TestErrorCases(t *testing.T) {
	serializer := NewCapnProtoSerializer()
	ctx := context.Background()

	// Test deserialize with invalid data
	invalidData := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	_, err := serializer.Deserialize(ctx, invalidData)
	if err == nil {
		t.Error("expected error for invalid Cap'n Proto data")
	}

	// Test traditional serializer with unsupported data
	traditional := NewTraditionalSerializer()
	writeState := NewWriteState()
	writeState.data["Test"] = []interface{}{func() {}} // Function not serializable
	_, err = traditional.Serialize(ctx, writeState)
	if err == nil {
		t.Error("expected error for unserializable data")
	}
}

func TestDeltaOperations(t *testing.T) {
	deltaSet := NewDeltaSet()

	if !deltaSet.IsEmpty() {
		t.Error("expected new delta set to be empty")
	}

	deltaSet.AddRecord("Movie", DeltaAdd, 1, map[string]interface{}{"ID": "1"})
	deltaSet.AddRecord("User", DeltaUpdate, 2, map[string]interface{}{"ID": "2"})

	if deltaSet.IsEmpty() {
		t.Error("expected delta set to not be empty after adding records")
	}

	if deltaSet.GetChangeCount() != 2 {
		t.Errorf("expected 2 changes, got %d", deltaSet.GetChangeCount())
	}

	typeNames := deltaSet.GetTypeNames()
	if len(typeNames) != 2 {
		t.Errorf("expected 2 type names, got %d", len(typeNames))
	}

	// Test optimization
	deltaSet.OptimizeDeltas()
	if deltaSet.GetChangeCount() != 2 {
		t.Errorf("expected 2 changes after optimization, got %d", deltaSet.GetChangeCount())
	}
}

func TestWriteStateEngine_BasicOperations(t *testing.T) {
	engine := NewWriteStateEngine()

	if engine.GetPopulatedCount() != 0 {
		t.Errorf("expected 0 populated count, got %d", engine.GetPopulatedCount())
	}

	engine.IncrementPopulatedCount()
	if engine.GetPopulatedCount() != 1 {
		t.Errorf("expected 1 populated count, got %d", engine.GetPopulatedCount())
	}

	engine.ResetPopulatedCount()
	if engine.GetPopulatedCount() != 0 {
		t.Errorf("expected 0 populated count after reset, got %d", engine.GetPopulatedCount())
	}

	engine.SetHeaderTag("key", "value")
	tags := engine.GetHeaderTags()
	if tags["key"] != "value" {
		t.Errorf("expected 'value', got %s", tags["key"])
	}
}

func TestWriteStateEngine_PrimaryKeys(t *testing.T) {
	engine := NewWriteStateEngine()

	if engine.HasPrimaryKeys() {
		t.Error("expected no primary keys initially")
	}

	pkConfig := map[string]string{
		"Movie": "ID",
		"User":  "ID",
	}
	engine.SetPrimaryKeys(pkConfig)

	if !engine.HasPrimaryKeys() {
		t.Error("expected primary keys after setting")
	}

	// Test reflection-based primary key extraction
	type TestRecord struct {
		ID   string
		Name string
	}

	record := TestRecord{ID: "test-123", Name: "Test"}
	pkValue, err := engine.ExtractReflectionPrimaryKey(record, "ID")
	if err != nil {
		t.Fatalf("failed to extract primary key: %v", err)
	}
	if pkValue != "test-123" {
		t.Errorf("expected 'test-123', got %s", pkValue)
	}
}
