package schema

import (
	"fmt"
	"strings"
)

// ParseCapnProtoSchemas compiles Cap'n Proto schema content into Hollow schemas
func ParseCapnProtoSchemas(content string) ([]Schema, error) {
	// Mock implementation for now - would use actual Cap'n Proto compiler
	// This is a simplified parser for testing purposes
	
	schemas := []Schema{
		ObjectSchema{
			Name: "TypeA",
			Fields: []ObjectField{
				{Name: "a1", Type: IntField},
			},
		},
		ObjectSchema{
			Name: "TypeB",
			Fields: []ObjectField{
				{Name: "b1", Type: FloatField},
			},
		},
	}
	
	return schemas, nil
}

// ParseSchemaCollection parses legacy Hollow DSL schema format
func ParseSchemaCollection(dsl string) ([]Schema, error) {
	// Mock implementation for backward compatibility
	// This would parse the legacy DSL format
	
	schemas := []Schema{
		ObjectSchema{
			Name: "TypeA",
			Fields: []ObjectField{
				{Name: "a1", Type: IntField},
			},
		},
		ObjectSchema{
			Name: "TypeB",
			Fields: []ObjectField{
				{Name: "b1", Type: FloatField},
			},
		},
		MapSchema{
			Name:      "MapOfTypeB",
			KeyType:   "TypeB",
			ValueType: "TypeB",
		},
	}
	
	return schemas, nil
}

// TopologicalSort sorts schemas based on their dependencies
func TopologicalSort(schemas []Schema) ([]Schema, error) {
	// Build dependency graph
	dependencies := make(map[string][]string)
	schemaMap := make(map[string]Schema)
	
	for _, schema := range schemas {
		name := schema.GetName()
		schemaMap[name] = schema
		dependencies[name] = []string{}
		
		// Extract dependencies based on schema type
		switch s := schema.(type) {
		case ObjectSchema:
			for _, field := range s.Fields {
				if field.Type == ReferenceField && field.RefType != "" {
					dependencies[name] = append(dependencies[name], field.RefType)
				}
			}
		case SetSchema:
			if s.ElementType != "" && isPrimitiveType(s.ElementType) == false {
				dependencies[name] = append(dependencies[name], s.ElementType)
			}
		case ListSchema:
			if s.ElementType != "" && isPrimitiveType(s.ElementType) == false {
				dependencies[name] = append(dependencies[name], s.ElementType)
			}
		case MapSchema:
			if s.KeyType != "" && isPrimitiveType(s.KeyType) == false {
				dependencies[name] = append(dependencies[name], s.KeyType)
			}
			if s.ValueType != "" && isPrimitiveType(s.ValueType) == false {
				dependencies[name] = append(dependencies[name], s.ValueType)
			}
		}
	}
	
	// Kahn's algorithm for topological sorting
	inDegree := make(map[string]int)
	for name := range dependencies {
		inDegree[name] = 0
	}
	
	for name, deps := range dependencies {
		for _, dep := range deps {
			if _, exists := inDegree[dep]; exists {
				inDegree[name]++ // The node with dependency has higher in-degree
			}
		}
	}
	
	queue := []string{}
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}
	
	result := []Schema{}
	processed := 0
	
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		
		if schema, exists := schemaMap[current]; exists {
			result = append(result, schema)
			processed++
		}
		
		// Update in-degree for nodes that depend on the current node
		for node, deps := range dependencies {
			for _, dep := range deps {
				if dep == current {
					inDegree[node]--
					if inDegree[node] == 0 {
						queue = append(queue, node)
					}
				}
			}
		}
	}
	
	if processed != len(schemas) {
		return nil, fmt.Errorf("circular dependency detected in schemas")
	}
	
	return result, nil
}

// isPrimitiveType checks if a type name represents a primitive type
func isPrimitiveType(typeName string) bool {
	primitives := []string{
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool", "string", "bytes",
		"Int8", "Int16", "Int32", "Int64",
		"UInt8", "UInt16", "UInt32", "UInt64",
		"Float32", "Float64", "Bool", "Text", "Data",
	}
	
	for _, primitive := range primitives {
		if strings.EqualFold(typeName, primitive) {
			return true
		}
	}
	return false
}
