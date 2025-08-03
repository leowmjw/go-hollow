package schema

import (
	"errors"
)

// SchemaType represents the type of a schema
type SchemaType int

const (
	ObjectType SchemaType = iota
	SetType
	ListType
	MapType
)

// FieldType represents the type of a field
type FieldType int

const (
	IntField FieldType = iota
	StringField
	FloatField
	BoolField
	BytesField
	ReferenceField
)

// ObjectField represents a field in an object schema
type ObjectField struct {
	Name    string
	Type    FieldType
	RefType string
}

// Schema interface defines the basic schema operations
type Schema interface {
	GetName() string
	GetSchemaType() SchemaType
	Validate() error
	Equals(other Schema) bool
	HashCode() uint64
}

// ObjectSchema represents an object schema
type ObjectSchema struct {
	Name       string
	Fields     []ObjectField
	PrimaryKey []string
}

func (os ObjectSchema) GetName() string {
	return os.Name
}

func (os ObjectSchema) GetSchemaType() SchemaType {
	return ObjectType
}

func (os ObjectSchema) Validate() error {
	if os.Name == "" {
		return errors.New("Type name in Hollow Schema was an empty string")
	}
	return nil
}

func (os ObjectSchema) Equals(other Schema) bool {
	otherObj, ok := other.(ObjectSchema)
	if !ok {
		return false
	}
	
	if os.Name != otherObj.Name {
		return false
	}
	
	if len(os.Fields) != len(otherObj.Fields) {
		return false
	}
	
	// Create field maps for order-insensitive comparison
	thisFields := make(map[string]ObjectField)
	for _, field := range os.Fields {
		thisFields[field.Name] = field
	}
	
	otherFields := make(map[string]ObjectField)
	for _, field := range otherObj.Fields {
		otherFields[field.Name] = field
	}
	
	// Compare field maps
	for name, field := range thisFields {
		otherField, exists := otherFields[name]
		if !exists || field != otherField {
			return false
		}
	}
	
	return true
}

func (os ObjectSchema) HashCode() uint64 {
	// Simple hash implementation
	hash := uint64(0)
	for _, b := range []byte(os.Name) {
		hash = hash*31 + uint64(b)
	}
	
	// Include fields in hash (order-insensitive)
	fieldNames := make([]string, 0, len(os.Fields))
	for _, field := range os.Fields {
		fieldNames = append(fieldNames, field.Name)
	}
	
	// Sort field names for consistent hashing
	for i := 0; i < len(fieldNames); i++ {
		for j := i + 1; j < len(fieldNames); j++ {
			if fieldNames[i] > fieldNames[j] {
				fieldNames[i], fieldNames[j] = fieldNames[j], fieldNames[i]
			}
		}
	}
	
	for _, name := range fieldNames {
		for _, b := range []byte(name) {
			hash = hash*31 + uint64(b)
		}
	}
	
	return hash
}

// SetSchema represents a set schema
type SetSchema struct {
	Name        string
	ElementType string
}

func (ss SetSchema) GetName() string {
	return ss.Name
}

func (ss SetSchema) GetSchemaType() SchemaType {
	return SetType
}

func (ss SetSchema) Validate() error {
	if ss.Name == "" {
		return errors.New("Type name in Hollow Schema was an empty string")
	}
	return nil
}

func (ss SetSchema) Equals(other Schema) bool {
	otherSet, ok := other.(SetSchema)
	if !ok {
		return false
	}
	return ss.Name == otherSet.Name && ss.ElementType == otherSet.ElementType
}

func (ss SetSchema) HashCode() uint64 {
	hash := uint64(0)
	for _, b := range []byte(ss.Name + ss.ElementType) {
		hash = hash*31 + uint64(b)
	}
	return hash
}

func (ss SetSchema) GetElementType() string {
	return ss.ElementType
}

// ListSchema represents a list schema
type ListSchema struct {
	Name        string
	ElementType string
}

func (ls ListSchema) GetName() string {
	return ls.Name
}

func (ls ListSchema) GetSchemaType() SchemaType {
	return ListType
}

func (ls ListSchema) Validate() error {
	if ls.Name == "" {
		return errors.New("Type name in Hollow Schema was an empty string")
	}
	return nil
}

func (ls ListSchema) Equals(other Schema) bool {
	otherList, ok := other.(ListSchema)
	if !ok {
		return false
	}
	return ls.Name == otherList.Name && ls.ElementType == otherList.ElementType
}

func (ls ListSchema) HashCode() uint64 {
	hash := uint64(0)
	for _, b := range []byte(ls.Name + ls.ElementType) {
		hash = hash*31 + uint64(b)
	}
	return hash
}

func (ls ListSchema) GetElementType() string {
	return ls.ElementType
}

// MapSchema represents a map schema
type MapSchema struct {
	Name      string
	KeyType   string
	ValueType string
	HashKey   string
}

func (ms MapSchema) GetName() string {
	return ms.Name
}

func (ms MapSchema) GetSchemaType() SchemaType {
	return MapType
}

func (ms MapSchema) Validate() error {
	if ms.Name == "" {
		return errors.New("Type name in Hollow Schema was an empty string")
	}
	return nil
}

func (ms MapSchema) Equals(other Schema) bool {
	otherMap, ok := other.(MapSchema)
	if !ok {
		return false
	}
	return ms.Name == otherMap.Name &&
		ms.KeyType == otherMap.KeyType &&
		ms.ValueType == otherMap.ValueType &&
		ms.HashKey == otherMap.HashKey
}

func (ms MapSchema) HashCode() uint64 {
	hash := uint64(0)
	for _, b := range []byte(ms.Name + ms.KeyType + ms.ValueType + ms.HashKey) {
		hash = hash*31 + uint64(b)
	}
	return hash
}

func (ms MapSchema) GetKeyType() string {
	return ms.KeyType
}

func (ms MapSchema) GetValueType() string {
	return ms.ValueType
}

func (ms MapSchema) GetHashKey() string {
	return ms.HashKey
}

// Dataset represents a collection of schemas
type Dataset struct {
	schemas []Schema
}

func NewDataset(schemas []Schema) *Dataset {
	return &Dataset{schemas: schemas}
}

func (d *Dataset) HasIdenticalSchemas(other *Dataset) bool {
	if len(d.schemas) != len(other.schemas) {
		return false
	}
	
	// Create maps for order-insensitive comparison
	thisSchemas := make(map[string]Schema)
	for _, schema := range d.schemas {
		thisSchemas[schema.GetName()] = schema
	}
	
	otherSchemas := make(map[string]Schema)
	for _, schema := range other.schemas {
		otherSchemas[schema.GetName()] = schema
	}
	
	// Compare schemas
	for name, schema := range thisSchemas {
		otherSchema, exists := otherSchemas[name]
		if !exists || !schema.Equals(otherSchema) {
			return false
		}
	}
	
	return true
}

// Utility functions
func SchemaTypeFromId(typeId int) SchemaType {
	switch typeId {
	case 0, 6:
		return ObjectType
	case 1, 4:
		return SetType
	case 2:
		return ListType
	case 3, 5:
		return MapType
	default:
		return ObjectType
	}
}

func SchemaTypeHasKey(typeId int) bool {
	return typeId == 4 || typeId == 5 || typeId == 6
}
