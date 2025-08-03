package schema

import (
	"fmt"
	"strings"
	"regexp"
)

// ParseCapnProtoSchemas parses Cap'n Proto schema content into Hollow schemas using improved parsing
func ParseCapnProtoSchemas(content string) ([]Schema, error) {
	// Basic validation of Cap'n Proto schema format
	if !strings.Contains(content, "@0x") {
		return nil, fmt.Errorf("invalid Cap'n Proto schema: missing @0x version identifier")
	}
	
	return parseCapnProtoImproved(content)
}

// parseCapnProtoImproved provides improved Cap'n Proto parsing using regex for better accuracy
func parseCapnProtoImproved(content string) ([]Schema, error) {
	// Check for struct definitions
	if !strings.Contains(content, "struct ") && !strings.Contains(content, "interface ") && 
	   !strings.Contains(content, "enum ") {
		return nil, fmt.Errorf("invalid Cap'n Proto schema: no struct, interface, or enum definitions found")
	}
	
	schemas := []Schema{}
	
	// Parse structs using improved regex-based parsing
	structs, err := parseCapnProtoStructs(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse structs: %w", err)
	}
	schemas = append(schemas, structs...)
	
	// Parse enums using improved regex-based parsing
	enums, err := parseCapnProtoEnums(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse enums: %w", err)
	}
	schemas = append(schemas, enums...)
	
	if len(schemas) == 0 {
		return nil, fmt.Errorf("no valid schemas found in Cap'n Proto content")
	}
	
	return schemas, nil
}

// parseCapnProtoStructs parses struct definitions using regex
func parseCapnProtoStructs(content string) ([]Schema, error) {
	// Regex to match struct definitions with their content
	// Matches: struct StructName { ... }
	structRegex := regexp.MustCompile(`(?s)struct\s+(\w+)\s*\{([^}]*)\}`)
	matches := structRegex.FindAllStringSubmatch(content, -1)
	
	var schemas []Schema
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		
		structName := match[1]
		structBody := match[2]
		
		// Parse fields within the struct
		fields, err := parseCapnProtoFields(structBody)
		if err != nil {
			continue // Skip structs with parsing errors
		}
		
		objSchema := ObjectSchema{
			Name:   structName,
			Fields: fields,
		}
		
		schemas = append(schemas, objSchema)
	}
	
	return schemas, nil
}

// parseCapnProtoFields parses field definitions within a struct
func parseCapnProtoFields(structBody string) ([]ObjectField, error) {
	// Regex to match field definitions
	// Matches: fieldName @N :Type; (with optional comments)
	fieldRegex := regexp.MustCompile(`(\w+)\s+@(\d+)\s*:(\w+(?:<[^>]*>)?(?:\([^)]*\))?);?`)
	matches := fieldRegex.FindAllStringSubmatch(structBody, -1)
	
	var fields []ObjectField
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		
		fieldName := match[1]
		// fieldNumber := match[2] // We could use this for ordering if needed
		fieldType := match[3]
		
		field := ObjectField{Name: fieldName}
		
		// Map Cap'n Proto types to Hollow field types
		field.Type, field.RefType = mapCapnProtoType(fieldType)
		
		fields = append(fields, field)
	}
	
	return fields, nil
}

// parseCapnProtoEnums parses enum definitions using regex
func parseCapnProtoEnums(content string) ([]Schema, error) {
	// Regex to match enum definitions
	// Matches: enum EnumName { ... }
	enumRegex := regexp.MustCompile(`(?s)enum\s+(\w+)\s*\{([^}]*)\}`)
	matches := enumRegex.FindAllStringSubmatch(content, -1)
	
	var schemas []Schema
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		
		enumName := match[1]
		
		// Represent enums as simple objects with a value field
		objSchema := ObjectSchema{
			Name: enumName,
			Fields: []ObjectField{
				{Name: "value", Type: IntField},
			},
		}
		
		schemas = append(schemas, objSchema)
	}
	
	return schemas, nil
}

// mapCapnProtoType maps Cap'n Proto type names to Hollow field types
func mapCapnProtoType(capnpType string) (FieldType, string) {
	// Handle primitive types
	switch {
	case strings.HasPrefix(capnpType, "Bool"):
		return BoolField, ""
	case strings.HasPrefix(capnpType, "Int8") || strings.HasPrefix(capnpType, "Int16") ||
		 strings.HasPrefix(capnpType, "Int32") || strings.HasPrefix(capnpType, "Int64") ||
		 strings.HasPrefix(capnpType, "UInt8") || strings.HasPrefix(capnpType, "UInt16") ||
		 strings.HasPrefix(capnpType, "UInt32") || strings.HasPrefix(capnpType, "UInt64"):
		return IntField, ""
	case strings.HasPrefix(capnpType, "Float32") || strings.HasPrefix(capnpType, "Float64"):
		return FloatField, ""
	case strings.HasPrefix(capnpType, "Text"):
		return StringField, ""
	case strings.HasPrefix(capnpType, "Data"):
		return BytesField, ""
	case strings.HasPrefix(capnpType, "List("):
		// Extract the element type from List(ElementType)
		listRegex := regexp.MustCompile(`List\(([^)]+)\)`)
		if matches := listRegex.FindStringSubmatch(capnpType); len(matches) > 1 {
			elementType := matches[1]
			return ReferenceField, "List<" + elementType + ">"
		}
		return ReferenceField, "List"
	default:
		// Assume it's a reference to another struct or enum
		return ReferenceField, capnpType
	}
}

// Helper function to extract struct definitions from Cap'n Proto schema content
func extractStructs(content string) map[string]map[string]string {
	result := make(map[string]map[string]string)
	
	// Find all struct definitions
	lines := strings.Split(content, "\n")
	var currentStruct string
	var inStruct bool
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check for struct definition
		if strings.HasPrefix(line, "struct ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentStruct = parts[1]
				currentStruct = strings.TrimSuffix(currentStruct, "{")
				result[currentStruct] = make(map[string]string)
				inStruct = true
			}
			continue
		}
		
		// Check for end of struct
		if inStruct && line == "}" {
			inStruct = false
			continue
		}
		
		// Parse field definition
		if inStruct && line != "{" && line != "" {
			// Remove comments
			if idx := strings.Index(line, "#"); idx >= 0 {
				line = line[:idx]
			}
			line = strings.TrimSpace(line)
			
			// Skip annotations
			if strings.HasPrefix(line, "@") {
				continue
			}
			
			// Parse field
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				fieldName := parts[0]
				fieldType := parts[1]
				
				// Remove trailing semicolon or comma
				fieldType = strings.TrimSuffix(strings.TrimSuffix(fieldType, ";"), ",")
				
				result[currentStruct][fieldName] = fieldType
			}
		}
	}
	
	return result
}

// Helper function to extract enum definitions from Cap'n Proto schema content
func extractEnums(content string) map[string][]string {
	result := make(map[string][]string)
	
	// Find all enum definitions
	lines := strings.Split(content, "\n")
	var currentEnum string
	var inEnum bool
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check for enum definition
		if strings.HasPrefix(line, "enum ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentEnum = parts[1]
				currentEnum = strings.TrimSuffix(currentEnum, "{")
				result[currentEnum] = make([]string, 0)
				inEnum = true
			}
			continue
		}
		
		// Check for end of enum
		if inEnum && line == "}" {
			inEnum = false
			continue
		}
		
		// Parse enum value
		if inEnum && line != "{" && line != "" {
			// Remove comments
			if idx := strings.Index(line, "#"); idx >= 0 {
				line = line[:idx]
			}
			line = strings.TrimSpace(line)
			
			// Skip annotations
			if strings.HasPrefix(line, "@") {
				continue
			}
			
			// Parse enum value
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				valueName := parts[0]
				
				// Remove trailing semicolon or comma
				valueName = strings.TrimSuffix(strings.TrimSuffix(valueName, ";"), ",")
				
				result[currentEnum] = append(result[currentEnum], valueName)
			}
		}
	}
	
	return result
}

// ParseSchemaCollection parses legacy Hollow DSL schema format
func ParseSchemaCollection(dsl string) ([]Schema, error) {
	// Parse the DSL content - handle both C-style and YAML-style formats
	schemas := []Schema{}
	lines := strings.Split(dsl, "\n")
	
	// Check if this is C-style format (like the test)
	if strings.Contains(dsl, "{") && strings.Contains(dsl, "}") {
		return parseCStyleDSL(dsl)
	}
	
	// Handle YAML-style format
	var currentSchema Schema
	var schemaType string
	var inFields bool
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}
		
		// Check for new schema definition
		if strings.HasPrefix(line, "TypeName:") {
			// Save previous schema if exists
			if currentSchema != nil {
				schemas = append(schemas, currentSchema)
			}
			
			// Start new schema
			name := strings.TrimSpace(strings.TrimPrefix(line, "TypeName:"))
			schemaType = "Object" // Default type
			currentSchema = ObjectSchema{Name: name, Fields: []ObjectField{}}
			inFields = false
			continue
		}
		
		// Check for schema type
		if strings.HasPrefix(line, "Type:") {
			schemaType = strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
			
			// Create appropriate schema type
			switch schemaType {
			case "Object":
				// Already created as default
			case "Set":
				setSchema := SetSchema{Name: currentSchema.GetName()}
				currentSchema = setSchema
			case "List":
				listSchema := ListSchema{Name: currentSchema.GetName()}
				currentSchema = listSchema
			case "Map":
				mapSchema := MapSchema{Name: currentSchema.GetName()}
				currentSchema = mapSchema
			}
			continue
		}
		
		// Check for fields section
		if strings.HasPrefix(line, "Fields:") {
			inFields = true
			continue
		}
		
		// Parse fields
		if inFields && schemaType == "Object" {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				fieldName := strings.TrimSpace(parts[0])
				fieldType := strings.TrimSpace(parts[1])
				
				field := ObjectField{Name: fieldName}
				
				// Determine field type
				switch {
				case strings.HasPrefix(fieldType, "Int"):
					field.Type = IntField
				case strings.HasPrefix(fieldType, "String"):
					field.Type = StringField
				case strings.HasPrefix(fieldType, "Float"):
					field.Type = FloatField
				case strings.HasPrefix(fieldType, "Bool"):
					field.Type = BoolField
				case strings.HasPrefix(fieldType, "Bytes"):
					field.Type = BytesField
				default:
					// Assume it's a reference to another type
					field.Type = ReferenceField
					field.RefType = fieldType
				}
				
				objSchema := currentSchema.(ObjectSchema)
				objSchema.Fields = append(objSchema.Fields, field)
				currentSchema = objSchema
			}
		}
		
		// Parse element type for Set and List
		if strings.HasPrefix(line, "ElementType:") {
			elementType := strings.TrimSpace(strings.TrimPrefix(line, "ElementType:"))
			
			switch s := currentSchema.(type) {
			case SetSchema:
				s.ElementType = elementType
				currentSchema = s
			case ListSchema:
				s.ElementType = elementType
				currentSchema = s
			}
			continue
		}
		
		// Parse key and value types for Map
		if strings.HasPrefix(line, "KeyType:") {
			keyType := strings.TrimSpace(strings.TrimPrefix(line, "KeyType:"))
			if mapSchema, ok := currentSchema.(MapSchema); ok {
				mapSchema.KeyType = keyType
				currentSchema = mapSchema
			}
			continue
		}
		
		if strings.HasPrefix(line, "ValueType:") {
			valueType := strings.TrimSpace(strings.TrimPrefix(line, "ValueType:"))
			if mapSchema, ok := currentSchema.(MapSchema); ok {
				mapSchema.ValueType = valueType
				currentSchema = mapSchema
			}
			continue
		}
	}
	
	// Add the last schema if exists
	if currentSchema != nil {
		schemas = append(schemas, currentSchema)
	}
	
	if len(schemas) == 0 {
		return nil, fmt.Errorf("no valid schemas found in DSL content")
	}
	
	return schemas, nil
}

// parseCStyleDSL parses C-style struct definitions
func parseCStyleDSL(dsl string) ([]Schema, error) {
	schemas := []Schema{}
	lines := strings.Split(dsl, "\n")
	
	var currentSchema *ObjectSchema
	inStruct := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue // Skip empty lines and comments
		}
		
		// Check for Map definition (e.g., "MapOfTypeB Map<TypeB, TypeB>;")
		if strings.Contains(line, "Map<") && strings.HasSuffix(line, ";") {
			// Find the first space to separate map name from definition
			spaceIndex := strings.Index(line, " ")
			if spaceIndex > 0 {
				mapName := strings.TrimSpace(line[:spaceIndex])
				mapDef := strings.TrimSpace(strings.TrimSuffix(line[spaceIndex+1:], ";"))
				
				// Parse Map<KeyType, ValueType>
				if strings.HasPrefix(mapDef, "Map<") && strings.HasSuffix(mapDef, ">") {
					typesPart := mapDef[4 : len(mapDef)-1] // Remove "Map<" and ">"
					typesList := strings.Split(typesPart, ",")
					if len(typesList) == 2 {
						keyType := strings.TrimSpace(typesList[0])
						valueType := strings.TrimSpace(typesList[1])
						
						mapSchema := MapSchema{
							Name:      mapName,
							KeyType:   keyType,
							ValueType: valueType,
						}
						schemas = append(schemas, mapSchema)
					}
				}
			}
			continue
		}
		
		// Check for struct definition start
		if strings.Contains(line, "{") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				structName := parts[0]
				currentSchema = &ObjectSchema{
					Name:   structName,
					Fields: []ObjectField{},
				}
				inStruct = true
			}
			continue
		}
		
		// Check for struct definition end
		if line == "}" {
			if currentSchema != nil {
				schemas = append(schemas, *currentSchema)
				currentSchema = nil
			}
			inStruct = false
			continue
		}
		
		// Parse field definitions inside struct
		if inStruct && currentSchema != nil && strings.HasSuffix(line, ";") {
			// Remove semicolon
			fieldLine := strings.TrimSuffix(line, ";")
			parts := strings.Fields(fieldLine)
			
			if len(parts) >= 2 {
				fieldType := parts[0]
				fieldName := parts[1]
				
				field := ObjectField{Name: fieldName}
				
				// Determine field type
				switch fieldType {
				case "int":
					field.Type = IntField
				case "string":
					field.Type = StringField
				case "float":
					field.Type = FloatField
				case "bool":
					field.Type = BoolField
				case "bytes":
					field.Type = BytesField
				default:
					// Assume it's a reference to another type
					field.Type = ReferenceField
					field.RefType = fieldType
				}
				
				currentSchema.Fields = append(currentSchema.Fields, field)
			}
		}
	}
	
	if len(schemas) == 0 {
		return nil, fmt.Errorf("no valid schemas found in DSL content")
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
