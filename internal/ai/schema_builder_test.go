package ai

import (
	"testing"
)

func TestSchemaBuilder_AddString(t *testing.T) {
	schema := NewSchema().
		AddString("name", Required(), Description("User name"))

	fields := schema.Fields()
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}

	field := fields[0]
	if field.Name != "name" {
		t.Errorf("expected name 'name', got %q", field.Name)
	}
	if field.Type != FieldTypeString {
		t.Errorf("expected type String, got %v", field.Type)
	}
	if !field.Required {
		t.Error("expected field to be required")
	}
	if field.Description != "User name" {
		t.Errorf("expected description 'User name', got %q", field.Description)
	}
}

func TestSchemaBuilder_AddInt(t *testing.T) {
	minVal := 1.0
	maxVal := 100.0
	schema := NewSchema().
		AddInt("age", Range(1, 100))

	fields := schema.Fields()
	field := fields[0]

	if field.Type != FieldTypeInt {
		t.Errorf("expected type Int, got %v", field.Type)
	}
	if field.Min == nil || *field.Min != minVal {
		t.Errorf("expected min %v, got %v", minVal, field.Min)
	}
	if field.Max == nil || *field.Max != maxVal {
		t.Errorf("expected max %v, got %v", maxVal, field.Max)
	}
}

func TestSchemaBuilder_AddEnum(t *testing.T) {
	schema := NewSchema().
		AddString("status", Enum("active", "inactive", "pending"))

	fields := schema.Fields()
	field := fields[0]

	if len(field.Enum) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(field.Enum))
	}
	if field.Enum[0] != "active" || field.Enum[1] != "inactive" || field.Enum[2] != "pending" {
		t.Errorf("unexpected enum values: %v", field.Enum)
	}
}

func TestSchemaBuilder_AddArray(t *testing.T) {
	minItems := 1
	maxItems := 5
	schema := NewSchema().
		AddArray("tags", FieldTypeString, MinItems(1), MaxItems(5), UniqueItems())

	fields := schema.Fields()
	field := fields[0]

	if field.Type != FieldTypeArray {
		t.Errorf("expected type Array, got %v", field.Type)
	}
	if field.ItemType != FieldTypeString {
		t.Errorf("expected item type String, got %v", field.ItemType)
	}
	if field.MinItems == nil || *field.MinItems != minItems {
		t.Errorf("expected minItems %d, got %v", minItems, field.MinItems)
	}
	if field.MaxItems == nil || *field.MaxItems != maxItems {
		t.Errorf("expected maxItems %d, got %v", maxItems, field.MaxItems)
	}
	if !field.UniqueItems {
		t.Error("expected uniqueItems to be true")
	}
}

func TestSchemaBuilder_AddObject(t *testing.T) {
	nested := NewSchema().
		AddString("street", Required()).
		AddString("city", Required())

	schema := NewSchema().
		AddObject("address", nested)

	fields := schema.Fields()
	field := fields[0]

	if field.Type != FieldTypeObject {
		t.Errorf("expected type Object, got %v", field.Type)
	}
	if field.NestedSchema == nil {
		t.Fatal("expected nested schema, got nil")
	}

	nestedFields := field.NestedSchema.Fields()
	if len(nestedFields) != 2 {
		t.Errorf("expected 2 nested fields, got %d", len(nestedFields))
	}
}

func TestSchemaBuilder_Chaining(t *testing.T) {
	schema := NewSchema().
		AddString("name", Required()).
		AddInt("age", Range(0, 120)).
		AddBool("active", Default(true))

	fields := schema.Fields()
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}

	if fields[0].Name != "name" || fields[0].Type != FieldTypeString {
		t.Error("first field should be name string")
	}
	if fields[1].Name != "age" || fields[1].Type != FieldTypeInt {
		t.Error("second field should be age int")
	}
	if fields[2].Name != "active" || fields[2].Type != FieldTypeBool {
		t.Error("third field should be active bool")
	}
}

func TestSchemaBuilder_ID_Stability(t *testing.T) {
	schema1 := NewSchema().
		AddString("name", Required()).
		AddInt("age", Range(0, 120))

	schema2 := NewSchema().
		AddString("name", Required()).
		AddInt("age", Range(0, 120))

	id1 := schema1.ID()
	id2 := schema2.ID()

	if id1 != id2 {
		t.Error("identical schemas should have same ID")
	}

	schema3 := NewSchema().
		AddString("name", Required()).
		AddInt("age", Range(0, 121)) // Different max

	id3 := schema3.ID()
	if id1 == id3 {
		t.Error("different schemas should have different IDs")
	}
}

func TestSchemaBuilder_ID_Uniqueness(t *testing.T) {
	schema1 := NewSchema().AddString("field1")
	schema2 := NewSchema().AddString("field2")
	schema3 := NewSchema().AddInt("field1")

	id1 := schema1.ID()
	id2 := schema2.ID()
	id3 := schema3.ID()

	if id1 == id2 {
		t.Error("schemas with different field names should have different IDs")
	}
	if id1 == id3 {
		t.Error("schemas with different field types should have different IDs")
	}
}

func TestSchemaBuilder_StringConstraints(t *testing.T) {
	minLen := 5
	maxLen := 100
	schema := NewSchema().
		AddString("text", MinLength(5), MaxLength(100), Pattern("^[a-z]+$"))

	field := schema.Fields()[0]

	if field.MinLength == nil || *field.MinLength != minLen {
		t.Errorf("expected minLength %d, got %v", minLen, field.MinLength)
	}
	if field.MaxLength == nil || *field.MaxLength != maxLen {
		t.Errorf("expected maxLength %d, got %v", maxLen, field.MaxLength)
	}
	if field.Pattern != "^[a-z]+$" {
		t.Errorf("expected pattern '^[a-z]+$', got %q", field.Pattern)
	}
}

func TestSchemaBuilder_DefaultValue(t *testing.T) {
	schema := NewSchema().
		AddBool("active", Default(true)).
		AddInt("count", Default(0)).
		AddString("status", Default("pending"))

	fields := schema.Fields()

	if fields[0].Default != true {
		t.Errorf("expected default true, got %v", fields[0].Default)
	}
	if fields[1].Default != 0 {
		t.Errorf("expected default 0, got %v", fields[1].Default)
	}
	if fields[2].Default != "pending" {
		t.Errorf("expected default 'pending', got %v", fields[2].Default)
	}
}
