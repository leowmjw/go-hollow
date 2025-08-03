package schema

import (
	"testing"
)

// TestSchemaType_FromTypeId tests schema type identification
func TestSchemaType_FromTypeId(t *testing.T) {
	tests := []struct {
		typeId   int
		expected SchemaType
	}{
		{0, ObjectType},
		{6, ObjectType},
		{1, SetType},
		{4, SetType},
		{2, ListType},
		{3, MapType},
		{5, MapType},
	}

	for _, test := range tests {
		result := SchemaTypeFromId(test.typeId)
		if result != test.expected {
			t.Errorf("SchemaTypeFromId(%d) = %v, want %v", test.typeId, result, test.expected)
		}
	}
}

// TestSchemaType_HasKey tests key-based type identification
func TestSchemaType_HasKey(t *testing.T) {
	tests := []struct {
		typeId  int
		hasKey  bool
	}{
		{4, true},  // Set with key
		{5, true},  // Map with key
		{6, true},  // Object with key
		{0, false}, // Object without key
		{1, false}, // Set without key
		{2, false}, // List without key
		{3, false}, // Map without key
	}

	for _, test := range tests {
		result := SchemaTypeHasKey(test.typeId)
		if result != test.hasKey {
			t.Errorf("SchemaTypeHasKey(%d) = %v, want %v", test.typeId, result, test.hasKey)
		}
	}
}

// TestObjectSchema_Validation tests object schema validation
func TestObjectSchema_Validation(t *testing.T) {
	tests := []struct {
		name      string
		schema    ObjectSchema
		wantError bool
	}{
		{
			name: "valid schema",
			schema: ObjectSchema{
				Name: "ValidType",
				Fields: []ObjectField{
					{Name: "id", Type: IntField},
					{Name: "name", Type: StringField},
				},
			},
			wantError: false,
		},
		{
			name: "empty name",
			schema: ObjectSchema{
				Name: "",
				Fields: []ObjectField{
					{Name: "field", Type: IntField},
				},
			},
			wantError: true,
		},
		{
			name: "nil name",
			schema: ObjectSchema{
				Name: "",
				Fields: []ObjectField{
					{Name: "field", Type: IntField},
				},
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.schema.Validate()
			if (err != nil) != test.wantError {
				t.Errorf("ObjectSchema.Validate() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}

// TestSetSchema_Basic tests set schema functionality
func TestSetSchema_Basic(t *testing.T) {
	schema := SetSchema{
		Name:        "TestSet",
		ElementType: "TestElement",
	}

	if schema.GetName() != "TestSet" {
		t.Errorf("GetName() = %s, want TestSet", schema.GetName())
	}

	if schema.GetSchemaType() != SetType {
		t.Errorf("GetSchemaType() = %v, want SetType", schema.GetSchemaType())
	}

	if schema.GetElementType() != "TestElement" {
		t.Errorf("GetElementType() = %s, want TestElement", schema.GetElementType())
	}
}

// TestMapSchema_Basic tests map schema functionality
func TestMapSchema_Basic(t *testing.T) {
	schema := MapSchema{
		Name:      "TestMap",
		KeyType:   "String",
		ValueType: "Integer",
		HashKey:   "value",
	}

	if schema.GetName() != "TestMap" {
		t.Errorf("GetName() = %s, want TestMap", schema.GetName())
	}

	if schema.GetSchemaType() != MapType {
		t.Errorf("GetSchemaType() = %v, want MapType", schema.GetSchemaType())
	}

	if schema.GetKeyType() != "String" {
		t.Errorf("GetKeyType() = %s, want String", schema.GetKeyType())
	}

	if schema.GetValueType() != "Integer" {
		t.Errorf("GetValueType() = %s, want Integer", schema.GetValueType())
	}

	if schema.GetHashKey() != "value" {
		t.Errorf("GetHashKey() = %s, want value", schema.GetHashKey())
	}
}

// TestListSchema_Basic tests list schema functionality
func TestListSchema_Basic(t *testing.T) {
	schema := ListSchema{
		Name:        "TestList",
		ElementType: "TestElement",
	}

	if schema.GetName() != "TestList" {
		t.Errorf("GetName() = %s, want TestList", schema.GetName())
	}

	if schema.GetSchemaType() != ListType {
		t.Errorf("GetSchemaType() = %v, want ListType", schema.GetSchemaType())
	}

	if schema.GetElementType() != "TestElement" {
		t.Errorf("GetElementType() = %s, want TestElement", schema.GetElementType())
	}
}

// TestSchema_Equality tests schema equality comparison
func TestSchema_Equality(t *testing.T) {
	schema1 := ObjectSchema{
		Name: "TestType",
		Fields: []ObjectField{
			{Name: "id", Type: IntField},
			{Name: "name", Type: StringField},
		},
	}

	schema2 := ObjectSchema{
		Name: "TestType",
		Fields: []ObjectField{
			{Name: "name", Type: StringField}, // Different order
			{Name: "id", Type: IntField},
		},
	}

	schema3 := ObjectSchema{
		Name: "TestType",
		Fields: []ObjectField{
			{Name: "id", Type: IntField},
			{Name: "name", Type: StringField},
			{Name: "extra", Type: BoolField}, // Extra field
		},
	}

	schema4 := ObjectSchema{
		Name: "DifferentType", // Different name
		Fields: []ObjectField{
			{Name: "id", Type: IntField},
			{Name: "name", Type: StringField},
		},
	}

	// Test equality (should ignore field order)
	if !schema1.Equals(schema2) {
		t.Error("Schemas with same fields in different order should be equal")
	}

	// Test inequality
	if schema1.Equals(schema3) {
		t.Error("Schemas with different fields should not be equal")
	}

	if schema1.Equals(schema4) {
		t.Error("Schemas with different names should not be equal")
	}

	// Test hash codes
	if schema1.HashCode() != schema2.HashCode() {
		t.Error("Equal schemas should have same hash code")
	}
}

// TestSchemaParser_ParseCapnProto tests Cap'n Proto schema compilation
func TestSchemaParser_ParseCapnProto(t *testing.T) {
	// Cap'n Proto schema content
	capnpContent := `
@0x85d3acc39d94e0f8;

struct TypeA {
  a1 @0 :Int32;
  a2 @1 :Text;
  a3 @2 :TypeB;
  a4 @3 :List(TypeB);
}

struct TypeB {
  b1 @0 :Float32;
  b2 @1 :Data;
}
	`

	schemas, err := ParseCapnProtoSchemas(capnpContent)
	if err != nil {
		t.Fatalf("ParseCapnProtoSchemas failed: %v", err)
	}

	if len(schemas) != 2 {
		t.Errorf("Expected 2 schemas, got %d", len(schemas))
	}

	// Verify schema names are present
	schemaNames := make(map[string]bool)
	for _, schema := range schemas {
		schemaNames[schema.GetName()] = true
	}

	expectedNames := []string{"TypeA", "TypeB"}
	for _, name := range expectedNames {
		if !schemaNames[name] {
			t.Errorf("Expected schema %s not found", name)
		}
	}
}

// TestSchemaParser_ParseCollection tests legacy schema collection parsing for backward compatibility
func TestSchemaParser_ParseCollection(t *testing.T) {
	schemaString := `
		TypeA { 
			int a1; 
			string a2; 
			TypeB a3; 
			MapOfTypeB a4; 
		}
		MapOfTypeB Map<TypeB, TypeB>;
		TypeB { 
			float b1; 
			bytes b2; 
		}
	`

	schemas, err := ParseSchemaCollection(schemaString)
	if err != nil {
		t.Fatalf("ParseSchemaCollection failed: %v", err)
	}

	if len(schemas) != 3 {
		t.Errorf("Expected 3 schemas, got %d", len(schemas))
	}

	// Verify schema names are present
	schemaNames := make(map[string]bool)
	for _, schema := range schemas {
		schemaNames[schema.GetName()] = true
	}

	expectedNames := []string{"TypeA", "TypeB", "MapOfTypeB"}
	for _, name := range expectedNames {
		if !schemaNames[name] {
			t.Errorf("Expected schema %s not found", name)
		}
	}
}

// TestSchemaSorter_TopologicalSort tests schema dependency sorting
func TestSchemaSorter_TopologicalSort(t *testing.T) {
	schemas := []Schema{
		ObjectSchema{
			Name: "TypeA",
			Fields: []ObjectField{
				{Name: "b", Type: ReferenceField, RefType: "TypeB"},
			},
		},
		ObjectSchema{
			Name: "TypeB",
			Fields: []ObjectField{
				{Name: "c", Type: ReferenceField, RefType: "TypeC"},
			},
		},
		ObjectSchema{
			Name: "TypeC",
			Fields: []ObjectField{
				{Name: "id", Type: IntField},
			},
		},
	}

	sorted, err := TopologicalSort(schemas)
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	// TypeC should come before TypeB, TypeB should come before TypeA
	positions := make(map[string]int)
	for i, schema := range sorted {
		positions[schema.GetName()] = i
	}

	if positions["TypeC"] >= positions["TypeB"] {
		t.Error("TypeC should come before TypeB in sorted order")
	}

	if positions["TypeB"] >= positions["TypeA"] {
		t.Error("TypeB should come before TypeA in sorted order")
	}
}

// TestSchema_HashCalculation tests schema hash calculation
func TestSchema_HashCalculation(t *testing.T) {
	schema1 := ObjectSchema{
		Name: "TestType",
		Fields: []ObjectField{
			{Name: "id", Type: IntField},
			{Name: "name", Type: StringField},
		},
	}

	schema2 := ObjectSchema{
		Name: "TestType",
		Fields: []ObjectField{
			{Name: "name", Type: StringField}, // Different field order
			{Name: "id", Type: IntField},
		},
	}

	schema3 := ObjectSchema{
		Name: "TestType",
		Fields: []ObjectField{
			{Name: "id", Type: IntField},
			{Name: "name", Type: StringField},
			{Name: "extra", Type: BoolField}, // Extra field
		},
	}

	// Test that equal schemas have same hash
	if schema1.HashCode() != schema2.HashCode() {
		t.Error("Equal schemas should have same hash code")
	}

	// Test that different schemas have different hashes
	if schema1.HashCode() == schema3.HashCode() {
		t.Error("Different schemas should have different hash codes")
	}
}
