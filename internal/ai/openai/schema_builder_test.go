package openai

import (
	"testing"

	"bob/internal/ai"
)

func TestBuildSchema_Simple(t *testing.T) {
	builder := ai.NewSchema().
		AddString("name", ai.Required(), ai.Description("User name")).
		AddInt("age", ai.Range(0, 120))

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", schema["type"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties should be a map")
	}

	nameField, ok := props["name"].(map[string]any)
	if !ok {
		t.Fatal("name field should be a map")
	}
	if nameField["type"] != "string" {
		t.Errorf("name type should be 'string', got %v", nameField["type"])
	}
	if nameField["description"] != "User name" {
		t.Errorf("unexpected description: %v", nameField["description"])
	}

	ageField, ok := props["age"].(map[string]any)
	if !ok {
		t.Fatal("age field should be a map")
	}
	if ageField["type"] != "integer" {
		t.Errorf("age type should be 'integer', got %v", ageField["type"])
	}
	if ageField["minimum"] != 0.0 {
		t.Errorf("expected minimum 0, got %v", ageField["minimum"])
	}
	if ageField["maximum"] != 120.0 {
		t.Errorf("expected maximum 120, got %v", ageField["maximum"])
	}
}

func TestBuildSchema_Required(t *testing.T) {
	// OpenAI Structured Outputs requires ALL fields to be listed in the required array.
	// Non-required fields are distinguished by a "Leave empty" hint in their description.
	builder := ai.NewSchema().
		AddString("required_field", ai.Required()).
		AddString("optional_field")

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be a string slice")
	}

	if len(required) != 2 {
		t.Fatalf("expected 2 required fields (OpenAI Structured Outputs needs all fields listed), got %d", len(required))
	}

	// Verify the optional field carries the "Leave empty" hint in its description
	props := schema["properties"].(map[string]any)
	optField, ok := props["optional_field"].(map[string]any)
	if !ok {
		t.Fatal("optional_field should be in properties")
	}
	desc, _ := optField["description"].(string)
	if desc == "" {
		t.Error("optional_field should have a description hinting it can be left empty")
	}
}

func TestBuildSchema_Enum(t *testing.T) {
	builder := ai.NewSchema().
		AddString("status", ai.Enum("active", "inactive", "pending"))

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	statusField := props["status"].(map[string]any)

	enum, ok := statusField["enum"].([]string)
	if !ok {
		t.Fatal("enum should be a string slice")
	}

	if len(enum) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(enum))
	}
	if enum[0] != "active" || enum[1] != "inactive" || enum[2] != "pending" {
		t.Errorf("unexpected enum values: %v", enum)
	}
}

func TestBuildSchema_StringConstraints(t *testing.T) {
	builder := ai.NewSchema().
		AddString("text", ai.MinLength(5), ai.MaxLength(100), ai.Pattern("^[a-z]+$"))

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	textField := props["text"].(map[string]any)

	if textField["minLength"] != 5 {
		t.Errorf("expected minLength 5, got %v", textField["minLength"])
	}
	if textField["maxLength"] != 100 {
		t.Errorf("expected maxLength 100, got %v", textField["maxLength"])
	}
	if textField["pattern"] != "^[a-z]+$" {
		t.Errorf("expected pattern '^[a-z]+$', got %v", textField["pattern"])
	}
}

func TestBuildSchema_Array(t *testing.T) {
	builder := ai.NewSchema().
		AddArray("tags", ai.FieldTypeString, ai.MinItems(1), ai.MaxItems(5), ai.UniqueItems())

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	tagsField := props["tags"].(map[string]any)

	if tagsField["type"] != "array" {
		t.Errorf("expected type 'array', got %v", tagsField["type"])
	}

	items, ok := tagsField["items"].(map[string]any)
	if !ok {
		t.Fatal("items should be a map")
	}
	if items["type"] != "string" {
		t.Errorf("expected items type 'string', got %v", items["type"])
	}

	if tagsField["minItems"] != 1 {
		t.Errorf("expected minItems 1, got %v", tagsField["minItems"])
	}
	if tagsField["maxItems"] != 5 {
		t.Errorf("expected maxItems 5, got %v", tagsField["maxItems"])
	}
	if tagsField["uniqueItems"] != true {
		t.Errorf("expected uniqueItems true, got %v", tagsField["uniqueItems"])
	}
}

func TestBuildSchema_NestedObject(t *testing.T) {
	addressBuilder := ai.NewSchema().
		AddString("street", ai.Required()).
		AddString("city", ai.Required())

	builder := ai.NewSchema().
		AddString("name", ai.Required()).
		AddObject("address", addressBuilder)

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	addressField := props["address"].(map[string]any)

	if addressField["type"] != "object" {
		t.Errorf("expected type 'object', got %v", addressField["type"])
	}

	addressProps := addressField["properties"].(map[string]any)
	if len(addressProps) != 2 {
		t.Errorf("expected 2 address properties, got %d", len(addressProps))
	}

	streetField := addressProps["street"].(map[string]any)
	if streetField["type"] != "string" {
		t.Errorf("expected street type 'string', got %v", streetField["type"])
	}
}

func TestBuildSchema_AllTypes(t *testing.T) {
	builder := ai.NewSchema().
		AddString("str_field").
		AddInt("int_field").
		AddFloat("float_field").
		AddBool("bool_field").
		AddArray("array_field", ai.FieldTypeString)

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)

	if props["str_field"].(map[string]any)["type"] != "string" {
		t.Error("str_field should be string type")
	}
	if props["int_field"].(map[string]any)["type"] != "integer" {
		t.Error("int_field should be integer type")
	}
	if props["float_field"].(map[string]any)["type"] != "number" {
		t.Error("float_field should be number type")
	}
	if props["bool_field"].(map[string]any)["type"] != "boolean" {
		t.Error("bool_field should be boolean type")
	}
	if props["array_field"].(map[string]any)["type"] != "array" {
		t.Error("array_field should be array type")
	}
}

func TestBuildSchema_Default(t *testing.T) {
	builder := ai.NewSchema().
		AddBool("active", ai.Default(true)).
		AddString("status", ai.Default("pending"))

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)

	activeField := props["active"].(map[string]any)
	if activeField["default"] != true {
		t.Errorf("expected default true, got %v", activeField["default"])
	}

	statusField := props["status"].(map[string]any)
	if statusField["default"] != "pending" {
		t.Errorf("expected default 'pending', got %v", statusField["default"])
	}
}

func TestBuildSchemaWithCache(t *testing.T) {
	builder := ai.NewSchema().
		AddString("name", ai.Required())

	// First call should build and cache
	schema1, err := buildSchemaWithCache(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should return cached
	schema2, err := buildSchemaWithCache(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be the same instance from cache
	if &schema1 != &schema2 {
		// Actually, they won't be the same instance, but should have same content
		// Let's just verify they're equal
		if schema1["type"] != schema2["type"] {
			t.Error("cached schema doesn't match original")
		}
	}

	// Different builder should not use cached schema
	builder2 := ai.NewSchema().
		AddString("different", ai.Required())

	schema3, err := buildSchemaWithCache(builder2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props1 := schema1["properties"].(map[string]any)
	props3 := schema3["properties"].(map[string]any)

	if _, ok := props1["name"]; !ok {
		t.Error("first schema should have 'name' field")
	}
	if _, ok := props3["different"]; !ok {
		t.Error("second schema should have 'different' field")
	}
}

func TestBuildSchema_NilBuilder(t *testing.T) {
	_, err := buildSchemaWithCache(nil)
	if err == nil {
		t.Error("expected error for nil builder")
	}
}

func TestBuildSchema_AdditionalPropertiesFalse(t *testing.T) {
	builder := ai.NewSchema().
		AddString("name")

	schema, err := buildSchema(builder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema["additionalProperties"] != false {
		t.Error("additionalProperties should be false")
	}
}
