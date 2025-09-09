package openapi

import (
	"reflect"
	"strconv"
	"strings"
)

// TypeAnalyzer analyzes Go types and converts them to OpenAPI schemas
type TypeAnalyzer struct {
	schemas map[string]*Schema // Cache for reusable schemas
}

// NewTypeAnalyzer creates a new type analyzer
func NewTypeAnalyzer() *TypeAnalyzer {
	return &TypeAnalyzer{
		schemas: make(map[string]*Schema),
	}
}

// AnalyzeStruct converts a Go struct to an OpenAPI schema
func (a *TypeAnalyzer) AnalyzeStruct(structType reflect.Type) *Schema {
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}
	
	if structType.Kind() != reflect.Struct {
		return a.analyzeType(structType)
	}
	
	// Check if we've already processed this type
	typeName := structType.Name()
	if typeName != "" {
		if _, exists := a.schemas[typeName]; exists {
			return &Schema{
				Type: "object",
				Properties: map[string]*Schema{
					"$ref": {Type: "#/components/schemas/" + typeName},
				},
			}
		}
	}
	
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
		Required:   []string{},
	}
	
	// Cache the schema to prevent infinite recursion
	if typeName != "" {
		a.schemas[typeName] = schema
	}
	
	// Analyze struct fields
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		
		fieldSchema := a.analyzeStructField(field)
		if fieldSchema != nil {
			jsonName := a.getJSONFieldName(field)
			if jsonName != "" && jsonName != "-" {
				schema.Properties[jsonName] = fieldSchema
				
				// Check if field is required
				if a.isRequiredField(field) {
					schema.Required = append(schema.Required, jsonName)
				}
			}
		}
	}
	
	return schema
}

// analyzeStructField analyzes a single struct field
func (a *TypeAnalyzer) analyzeStructField(field reflect.StructField) *Schema {
	fieldType := field.Type
	schema := a.analyzeType(fieldType)
	
	// Add field-specific metadata from tags
	a.enhanceSchemaFromTags(schema, field)
	
	return schema
}

// analyzeType converts a Go type to OpenAPI schema
func (a *TypeAnalyzer) analyzeType(t reflect.Type) *Schema {
	// Handle pointers
	if t.Kind() == reflect.Ptr {
		schema := a.analyzeType(t.Elem())
		// Pointer fields are typically optional
		return schema
	}
	
	switch t.Kind() {
	case reflect.String:
		return a.analyzeStringType(t)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.analyzeIntegerType(t)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return a.analyzeIntegerType(t)
	case reflect.Float32, reflect.Float64:
		return a.analyzeNumberType(t)
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Array, reflect.Slice:
		return a.analyzeArrayType(t)
	case reflect.Map:
		return a.analyzeMapType(t)
	case reflect.Struct:
		return a.analyzeStructType(t)
	case reflect.Interface:
		return &Schema{} // Generic object
	default:
		return &Schema{Type: "string", Description: "Unsupported type: " + t.Kind().String()}
	}
}

// analyzeStringType handles string types with special cases
func (a *TypeAnalyzer) analyzeStringType(t reflect.Type) *Schema {
	schema := &Schema{Type: "string"}
	
	// Handle special string types
	switch t.String() {
	case "time.Time":
		schema.Format = "date-time"
	case "uuid.UUID":
		schema.Format = "uuid"
	}
	
	return schema
}

// analyzeIntegerType handles integer types
func (a *TypeAnalyzer) analyzeIntegerType(t reflect.Type) *Schema {
	schema := &Schema{Type: "integer"}
	
	switch t.Kind() {
	case reflect.Int32, reflect.Uint32:
		schema.Format = "int32"
	case reflect.Int64, reflect.Uint64:
		schema.Format = "int64"
	}
	
	return schema
}

// analyzeNumberType handles floating-point types
func (a *TypeAnalyzer) analyzeNumberType(t reflect.Type) *Schema {
	schema := &Schema{Type: "number"}
	
	switch t.Kind() {
	case reflect.Float32:
		schema.Format = "float"
	case reflect.Float64:
		schema.Format = "double"
	}
	
	return schema
}

// analyzeArrayType handles arrays and slices
func (a *TypeAnalyzer) analyzeArrayType(t reflect.Type) *Schema {
	itemType := t.Elem()
	itemSchema := a.analyzeType(itemType)
	
	schema := &Schema{
		Type:  "array",
		Items: itemSchema,
	}
	
	// Set maxItems for arrays (fixed size)
	if t.Kind() == reflect.Array {
		size := t.Len()
		schema.MaxItems = &size
		schema.MinItems = &size
	}
	
	return schema
}

// analyzeMapType handles map types
func (a *TypeAnalyzer) analyzeMapType(t reflect.Type) *Schema {
	valueType := t.Elem()
	valueSchema := a.analyzeType(valueType)
	
	return &Schema{
		Type:                 "object",
		AdditionalProperties: valueSchema,
	}
}

// analyzeStructType handles struct types
func (a *TypeAnalyzer) analyzeStructType(t reflect.Type) *Schema {
	// Handle special struct types
	switch t.String() {
	case "time.Time":
		return &Schema{Type: "string", Format: "date-time"}
	}
	
	return a.AnalyzeStruct(t)
}

// getJSONFieldName extracts the JSON field name from struct tags
func (a *TypeAnalyzer) getJSONFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag == "" {
		// Use field name if no json tag
		return strings.ToLower(field.Name[:1]) + field.Name[1:]
	}
	
	// Parse json tag (handle options like omitempty)
	parts := strings.Split(jsonTag, ",")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	
	return field.Name
}

// isRequiredField determines if a struct field is required
func (a *TypeAnalyzer) isRequiredField(field reflect.StructField) bool {
	// Check binding tag for required validation
	bindingTag := field.Tag.Get("binding")
	if strings.Contains(bindingTag, "required") {
		return true
	}
	
	// Check validate tag
	validateTag := field.Tag.Get("validate")
	if strings.Contains(validateTag, "required") {
		return true
	}
	
	// Check json tag for omitempty (indicates optional)
	jsonTag := field.Tag.Get("json")
	if strings.Contains(jsonTag, "omitempty") {
		return false
	}
	
	// Pointer fields are typically optional
	if field.Type.Kind() == reflect.Ptr {
		return false
	}
	
	// Default to required for non-pointer fields
	return true
}

// enhanceSchemaFromTags adds metadata from struct tags to schema
func (a *TypeAnalyzer) enhanceSchemaFromTags(schema *Schema, field reflect.StructField) {
	// Add description from doc tag
	if docTag := field.Tag.Get("doc"); docTag != "" {
		schema.Description = docTag
	} else if descTag := field.Tag.Get("description"); descTag != "" {
		schema.Description = descTag
	}
	
	// Add example from example tag
	if exampleTag := field.Tag.Get("example"); exampleTag != "" {
		schema.Example = a.parseExampleValue(exampleTag, schema.Type)
	}
	
	// Handle validation tags
	a.applyValidationTags(schema, field)
}

// parseExampleValue converts string example to appropriate type
func (a *TypeAnalyzer) parseExampleValue(value, schemaType string) interface{} {
	switch schemaType {
	case "integer":
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	case "number":
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	case "boolean":
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return value
}

// applyValidationTags applies validation constraints from struct tags
func (a *TypeAnalyzer) applyValidationTags(schema *Schema, field reflect.StructField) {
	// Check binding and validate tags for constraints
	tags := []string{
		field.Tag.Get("binding"),
		field.Tag.Get("validate"),
	}
	
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		
		constraints := strings.Split(tag, ",")
		for _, constraint := range constraints {
			a.applyConstraint(schema, strings.TrimSpace(constraint))
		}
	}
}

// applyConstraint applies a single validation constraint to schema
func (a *TypeAnalyzer) applyConstraint(schema *Schema, constraint string) {
	// Handle min/max constraints
	if strings.HasPrefix(constraint, "min=") {
		if minVal, err := strconv.Atoi(constraint[4:]); err == nil {
			if schema.Type == "string" {
				schema.MinLength = &minVal
			} else if schema.Type == "array" {
				schema.MinItems = &minVal
			} else if schema.Type == "integer" || schema.Type == "number" {
				min := float64(minVal)
				schema.Minimum = &min
			}
		}
	}
	
	if strings.HasPrefix(constraint, "max=") {
		if maxVal, err := strconv.Atoi(constraint[4:]); err == nil {
			if schema.Type == "string" {
				schema.MaxLength = &maxVal
			} else if schema.Type == "array" {
				schema.MaxItems = &maxVal
			} else if schema.Type == "integer" || schema.Type == "number" {
				max := float64(maxVal)
				schema.Maximum = &max
			}
		}
	}
	
	// Handle email validation
	if constraint == "email" && schema.Type == "string" {
		schema.Format = "email"
	}
	
	// Handle URL validation
	if constraint == "url" && schema.Type == "string" {
		schema.Format = "uri"
	}
	
	// Handle UUID validation
	if constraint == "uuid" && schema.Type == "string" {
		schema.Format = "uuid"
	}
}

// GetSchemas returns all cached schemas for components
func (a *TypeAnalyzer) GetSchemas() map[string]*Schema {
	return a.schemas
}