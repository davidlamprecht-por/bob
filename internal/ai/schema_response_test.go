package ai

import (
	"testing"
)

func TestSchemaData_GetString(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"name": "John",
		},
	}

	val, err := data.GetString("name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "John" {
		t.Errorf("expected 'John', got %q", val)
	}

	_, err = data.GetString("missing")
	if err == nil {
		t.Error("expected error for missing field")
	}

	data.data["wrong_type"] = 123
	_, err = data.GetString("wrong_type")
	if err == nil {
		t.Error("expected error for wrong type")
	}
}

func TestSchemaData_GetInt(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"age":      42,
			"float_as_int": 42.0, // JSON unmarshaling gives float64
		},
	}

	val, err := data.GetInt("age")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}

	// Should handle float64 (JSON numbers)
	val, err = data.GetInt("float_as_int")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
}

func TestSchemaData_GetFloat(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"price":   19.99,
			"int_val": 20,
		},
	}

	val, err := data.GetFloat("price")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 19.99 {
		t.Errorf("expected 19.99, got %f", val)
	}

	// Should handle int as float
	val, err = data.GetFloat("int_val")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 20.0 {
		t.Errorf("expected 20.0, got %f", val)
	}
}

func TestSchemaData_GetBool(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"active": true,
		},
	}

	val, err := data.GetBool("active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !val {
		t.Error("expected true")
	}
}

func TestSchemaData_GetArray(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"tags": []any{"tag1", "tag2", "tag3"},
		},
	}

	val, err := data.GetArray("tags")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(val) != 3 {
		t.Errorf("expected 3 items, got %d", len(val))
	}
}

func TestSchemaData_GetObject(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"address": map[string]any{
				"street": "123 Main St",
				"city":   "Boston",
			},
		},
	}

	val, err := data.GetObject("address")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val["street"] != "123 Main St" {
		t.Errorf("unexpected street value: %v", val["street"])
	}
}

func TestSchemaData_MustGet_Success(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"name": "John",
			"age":  30.0,
		},
	}

	name := data.MustGetString("name")
	if name != "John" {
		t.Errorf("expected 'John', got %q", name)
	}

	age := data.MustGetInt("age")
	if age != 30 {
		t.Errorf("expected 30, got %d", age)
	}
}

func TestSchemaData_MustGet_Panic(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing field")
		}
	}()

	data.MustGetString("missing")
}

func TestSchemaData_Has(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"name": "John",
		},
	}

	if !data.Has("name") {
		t.Error("expected Has to return true for existing field")
	}
	if data.Has("missing") {
		t.Error("expected Has to return false for missing field")
	}
}

func TestSchemaData_IsSet(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"name":  "John",
			"empty": nil,
		},
	}

	if !data.IsSet("name") {
		t.Error("expected IsSet to return true for set field")
	}
	if data.IsSet("empty") {
		t.Error("expected IsSet to return false for nil field")
	}
	if data.IsSet("missing") {
		t.Error("expected IsSet to return false for missing field")
	}
}

func TestSchemaData_Raw(t *testing.T) {
	original := map[string]any{
		"name": "John",
		"age":  30,
	}

	data := &SchemaData{data: original}

	raw := data.Raw()
	if len(raw) != 2 {
		t.Errorf("expected 2 fields, got %d", len(raw))
	}
	if raw["name"] != "John" {
		t.Error("raw data doesn't match original")
	}
}

func TestSchemaData_NestedAccess(t *testing.T) {
	data := &SchemaData{
		data: map[string]any{
			"person": map[string]any{
				"name": "John",
				"address": map[string]any{
					"city": "Boston",
				},
			},
		},
	}

	personMap := data.MustGetObject("person")
	person := &SchemaData{data: personMap}

	name := person.MustGetString("name")
	if name != "John" {
		t.Errorf("expected 'John', got %q", name)
	}

	addressMap := person.MustGetObject("address")
	address := &SchemaData{data: addressMap}

	city := address.MustGetString("city")
	if city != "Boston" {
		t.Errorf("expected 'Boston', got %q", city)
	}
}
