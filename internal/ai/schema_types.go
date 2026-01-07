package ai

type FieldType int

const (
	FieldTypeString FieldType = iota
	FieldTypeInt
	FieldTypeFloat
	FieldTypeBool
	FieldTypeArray
	FieldTypeObject
)

type FieldDef struct {
	Name        string
	Type        FieldType
	Description string
	Required    bool
	Default     any

	Enum      []string
	MinLength *int
	MaxLength *int
	Pattern   string

	Min *float64
	Max *float64

	ItemType     FieldType
	NestedSchema *SchemaBuilder
	MinItems     *int
	MaxItems     *int
	UniqueItems  bool
}

type fieldDef = FieldDef

type FieldOption func(*fieldDef)

func Required() FieldOption {
	return func(f *fieldDef) {
		f.Required = true
	}
}

func Description(desc string) FieldOption {
	return func(f *fieldDef) {
		f.Description = desc
	}
}

func Default(value any) FieldOption {
	return func(f *fieldDef) {
		f.Default = value
	}
}

func Enum(values ...string) FieldOption {
	return func(f *fieldDef) {
		f.Enum = values
	}
}

func MinLength(n int) FieldOption {
	return func(f *fieldDef) {
		f.MinLength = &n
	}
}

func MaxLength(n int) FieldOption {
	return func(f *fieldDef) {
		f.MaxLength = &n
	}
}

func Pattern(regex string) FieldOption {
	return func(f *fieldDef) {
		f.Pattern = regex
	}
}

func Range(min, max float64) FieldOption {
	return func(f *fieldDef) {
		f.Min = &min
		f.Max = &max
	}
}

func Min(value float64) FieldOption {
	return func(f *fieldDef) {
		f.Min = &value
	}
}

func Max(value float64) FieldOption {
	return func(f *fieldDef) {
		f.Max = &value
	}
}

func MinItems(n int) FieldOption {
	return func(f *fieldDef) {
		f.MinItems = &n
	}
}

func MaxItems(n int) FieldOption {
	return func(f *fieldDef) {
		f.MaxItems = &n
	}
}

func UniqueItems() FieldOption {
	return func(f *fieldDef) {
		f.UniqueItems = true
	}
}
