package openai

import (
	"sync"

	"bob/internal/ai"
)

var (
	schemaCache = make(map[string]map[string]any)
	cacheMu     sync.RWMutex
)

func buildSchemaWithCache(builder *ai.SchemaBuilder) (map[string]any, error) {
	if builder == nil {
		return nil, &Error{
			Type:    ErrTypeInvalidRequest,
			Message: "schema builder is nil",
		}
	}

	id := builder.ID()

	cacheMu.RLock()
	if cached, ok := schemaCache[id]; ok {
		cacheMu.RUnlock()
		return cached, nil
	}
	cacheMu.RUnlock()

	schema, err := buildSchema(builder)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	schemaCache[id] = schema
	cacheMu.Unlock()

	return schema, nil
}

func buildSchema(builder *ai.SchemaBuilder) (map[string]any, error) {
	fields := builder.Fields()

	properties := make(map[string]any)
	required := []string{}

	for _, field := range fields {
		propSchema := buildPropertySchema(field)
		properties[field.Name] = propSchema

		if field.Required {
			required = append(required, field.Name)
		}
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

func buildPropertySchema(field ai.FieldDef) map[string]any {
	prop := make(map[string]any)

	switch field.Type {
	case ai.FieldTypeString:
		prop["type"] = "string"
		if field.Enum != nil && len(field.Enum) > 0 {
			prop["enum"] = field.Enum
		}
		if field.MinLength != nil {
			prop["minLength"] = *field.MinLength
		}
		if field.MaxLength != nil {
			prop["maxLength"] = *field.MaxLength
		}
		if field.Pattern != "" {
			prop["pattern"] = field.Pattern
		}

	case ai.FieldTypeInt:
		prop["type"] = "integer"
		if field.Min != nil {
			prop["minimum"] = *field.Min
		}
		if field.Max != nil {
			prop["maximum"] = *field.Max
		}

	case ai.FieldTypeFloat:
		prop["type"] = "number"
		if field.Min != nil {
			prop["minimum"] = *field.Min
		}
		if field.Max != nil {
			prop["maximum"] = *field.Max
		}

	case ai.FieldTypeBool:
		prop["type"] = "boolean"

	case ai.FieldTypeArray:
		prop["type"] = "array"
		itemSchema := buildItemSchema(field)
		prop["items"] = itemSchema
		if field.MinItems != nil {
			prop["minItems"] = *field.MinItems
		}
		if field.MaxItems != nil {
			prop["maxItems"] = *field.MaxItems
		}
		if field.UniqueItems {
			prop["uniqueItems"] = true
		}

	case ai.FieldTypeObject:
		if field.NestedSchema != nil {
			nestedSchema, _ := buildSchema(field.NestedSchema)
			return nestedSchema
		}
	}

	if field.Description != "" {
		prop["description"] = field.Description
	}

	if field.Default != nil {
		prop["default"] = field.Default
	}

	return prop
}

func buildItemSchema(field ai.FieldDef) map[string]any {
	switch field.ItemType {
	case ai.FieldTypeString:
		return map[string]any{"type": "string"}
	case ai.FieldTypeInt:
		return map[string]any{"type": "integer"}
	case ai.FieldTypeFloat:
		return map[string]any{"type": "number"}
	case ai.FieldTypeBool:
		return map[string]any{"type": "boolean"}
	case ai.FieldTypeObject:
		if field.NestedSchema != nil {
			schema, _ := buildSchema(field.NestedSchema)
			return schema
		}
	}
	return map[string]any{"type": "string"}
}
