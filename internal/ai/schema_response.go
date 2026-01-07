package ai

import "fmt"

type SchemaData struct {
	data map[string]any
}

func (d *SchemaData) GetString(name string) (string, error) {
	val, exists := d.data[name]
	if !exists {
		return "", fmt.Errorf("field %q not found", name)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("field %q is %T, not string", name, val)
	}

	return str, nil
}

func (d *SchemaData) GetInt(name string) (int, error) {
	val, exists := d.data[name]
	if !exists {
		return 0, fmt.Errorf("field %q not found", name)
	}

	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("field %q is %T, not int", name, val)
	}
}

func (d *SchemaData) GetFloat(name string) (float64, error) {
	val, exists := d.data[name]
	if !exists {
		return 0, fmt.Errorf("field %q not found", name)
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("field %q is %T, not float", name, val)
	}
}

func (d *SchemaData) GetBool(name string) (bool, error) {
	val, exists := d.data[name]
	if !exists {
		return false, fmt.Errorf("field %q not found", name)
	}

	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("field %q is %T, not bool", name, val)
	}

	return b, nil
}

func (d *SchemaData) GetArray(name string) ([]any, error) {
	val, exists := d.data[name]
	if !exists {
		return nil, fmt.Errorf("field %q not found", name)
	}

	arr, ok := val.([]any)
	if !ok {
		return nil, fmt.Errorf("field %q is %T, not array", name, val)
	}

	return arr, nil
}

func (d *SchemaData) GetObject(name string) (map[string]any, error) {
	val, exists := d.data[name]
	if !exists {
		return nil, fmt.Errorf("field %q not found", name)
	}

	obj, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("field %q is %T, not object", name, val)
	}

	return obj, nil
}

func (d *SchemaData) MustGetString(name string) string {
	val, err := d.GetString(name)
	if err != nil {
		panic(err)
	}
	return val
}

func (d *SchemaData) MustGetInt(name string) int {
	val, err := d.GetInt(name)
	if err != nil {
		panic(err)
	}
	return val
}

func (d *SchemaData) MustGetFloat(name string) float64 {
	val, err := d.GetFloat(name)
	if err != nil {
		panic(err)
	}
	return val
}

func (d *SchemaData) MustGetBool(name string) bool {
	val, err := d.GetBool(name)
	if err != nil {
		panic(err)
	}
	return val
}

func (d *SchemaData) MustGetArray(name string) []any {
	val, err := d.GetArray(name)
	if err != nil {
		panic(err)
	}
	return val
}

func (d *SchemaData) MustGetObject(name string) map[string]any {
	val, err := d.GetObject(name)
	if err != nil {
		panic(err)
	}
	return val
}

func (d *SchemaData) Has(name string) bool {
	_, exists := d.data[name]
	return exists
}

func (d *SchemaData) IsSet(name string) bool {
	val, exists := d.data[name]
	return exists && val != nil
}

func (d *SchemaData) Raw() map[string]any {
	return d.data
}
