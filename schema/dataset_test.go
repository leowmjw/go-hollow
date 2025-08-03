package schema

import (
	"testing"
)

// TestDataset_IdenticalSchemas tests schema comparison for dataset identity
func TestDataset_IdenticalSchemas(t *testing.T) {
	tests := []struct {
		name     string
		schemas1 []Schema
		schemas2 []Schema
		want     bool
	}{
		{
			name: "identical schemas different order",
			schemas1: []Schema{
				ObjectSchema{Name: "TypeA", Fields: []ObjectField{{Name: "a1", Type: IntField}, {Name: "a2", Type: StringField}}},
				ObjectSchema{Name: "TypeB", Fields: []ObjectField{{Name: "b1", Type: FloatField}, {Name: "b2", Type: BytesField}}},
			},
			schemas2: []Schema{
				ObjectSchema{Name: "TypeB", Fields: []ObjectField{{Name: "b1", Type: FloatField}, {Name: "b2", Type: BytesField}}},
				ObjectSchema{Name: "TypeA", Fields: []ObjectField{{Name: "a1", Type: IntField}, {Name: "a2", Type: StringField}}},
			},
			want: true,
		},
		{
			name: "different schemas - extra field",
			schemas1: []Schema{
				ObjectSchema{Name: "TypeA", Fields: []ObjectField{{Name: "a1", Type: IntField}, {Name: "a2", Type: StringField}, {Name: "newField", Type: BoolField}}},
			},
			schemas2: []Schema{
				ObjectSchema{Name: "TypeA", Fields: []ObjectField{{Name: "a1", Type: IntField}, {Name: "a2", Type: StringField}}},
			},
			want: false,
		},
		{
			name: "different schemas - different type name",
			schemas1: []Schema{
				ObjectSchema{Name: "DifferentTypeName", Fields: []ObjectField{{Name: "a1", Type: IntField}}},
			},
			schemas2: []Schema{
				ObjectSchema{Name: "TypeA", Fields: []ObjectField{{Name: "a1", Type: IntField}}},
			},
			want: false,
		},
		{
			name: "missing schema",
			schemas1: []Schema{
				ObjectSchema{Name: "TypeA", Fields: []ObjectField{{Name: "a1", Type: IntField}}},
				ObjectSchema{Name: "TypeB", Fields: []ObjectField{{Name: "b1", Type: FloatField}}},
			},
			schemas2: []Schema{
				ObjectSchema{Name: "TypeB", Fields: []ObjectField{{Name: "b1", Type: FloatField}}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset1 := NewDataset(tt.schemas1)
			dataset2 := NewDataset(tt.schemas2)

			if got := dataset1.HasIdenticalSchemas(dataset2); got != tt.want {
				t.Errorf("Dataset.HasIdenticalSchemas() = %v, want %v", got, tt.want)
			}

			// Test bidirectional
			if got := dataset2.HasIdenticalSchemas(dataset1); got != tt.want {
				t.Errorf("Dataset.HasIdenticalSchemas() (reverse) = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSchema_Validation tests schema validation rules
func TestSchema_Validation(t *testing.T) {
	tests := []struct {
		name      string
		schema    Schema
		wantError bool
	}{
		{
			name:      "empty schema name",
			schema:    ObjectSchema{Name: "", Fields: []ObjectField{{Name: "field", Type: IntField}}},
			wantError: true,
		},
		{
			name:      "valid schema",
			schema:    ObjectSchema{Name: "ValidType", Fields: []ObjectField{{Name: "field", Type: IntField}}},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Schema.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
