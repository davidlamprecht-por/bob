package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

type SchemaBuilder struct {
	fields []fieldDef
}

func NewSchema() *SchemaBuilder {
	return &SchemaBuilder{
		fields: make([]fieldDef, 0),
	}
}

func (b *SchemaBuilder) AddString(name string, opts ...FieldOption) *SchemaBuilder {
	field := fieldDef{
		Name: name,
		Type: FieldTypeString,
	}
	for _, opt := range opts {
		opt(&field)
	}
	b.fields = append(b.fields, field)
	return b
}

func (b *SchemaBuilder) AddInt(name string, opts ...FieldOption) *SchemaBuilder {
	field := fieldDef{
		Name: name,
		Type: FieldTypeInt,
	}
	for _, opt := range opts {
		opt(&field)
	}
	b.fields = append(b.fields, field)
	return b
}

func (b *SchemaBuilder) AddFloat(name string, opts ...FieldOption) *SchemaBuilder {
	field := fieldDef{
		Name: name,
		Type: FieldTypeFloat,
	}
	for _, opt := range opts {
		opt(&field)
	}
	b.fields = append(b.fields, field)
	return b
}

func (b *SchemaBuilder) AddBool(name string, opts ...FieldOption) *SchemaBuilder {
	field := fieldDef{
		Name: name,
		Type: FieldTypeBool,
	}
	for _, opt := range opts {
		opt(&field)
	}
	b.fields = append(b.fields, field)
	return b
}

func (b *SchemaBuilder) AddArray(name string, itemType FieldType, opts ...FieldOption) *SchemaBuilder {
	field := fieldDef{
		Name:     name,
		Type:     FieldTypeArray,
		ItemType: itemType,
	}
	for _, opt := range opts {
		opt(&field)
	}
	b.fields = append(b.fields, field)
	return b
}

func (b *SchemaBuilder) AddObject(name string, nestedBuilder *SchemaBuilder, opts ...FieldOption) *SchemaBuilder {
	field := fieldDef{
		Name:         name,
		Type:         FieldTypeObject,
		NestedSchema: nestedBuilder,
	}
	for _, opt := range opts {
		opt(&field)
	}
	b.fields = append(b.fields, field)
	return b
}

func (b *SchemaBuilder) Fields() []FieldDef {
	return b.fields
}

func (b *SchemaBuilder) ID() string {
	h := sha256.New()
	for _, f := range b.fields {
		io.WriteString(h, f.Name)
		io.WriteString(h, fmt.Sprintf("%d", f.Type))
		io.WriteString(h, f.Description)
		io.WriteString(h, fmt.Sprintf("%t", f.Required))
		if f.Default != nil {
			io.WriteString(h, fmt.Sprintf("%v", f.Default))
		}

		for _, e := range f.Enum {
			io.WriteString(h, e)
		}
		if f.MinLength != nil {
			io.WriteString(h, fmt.Sprintf("%d", *f.MinLength))
		}
		if f.MaxLength != nil {
			io.WriteString(h, fmt.Sprintf("%d", *f.MaxLength))
		}
		io.WriteString(h, f.Pattern)

		if f.Min != nil {
			io.WriteString(h, fmt.Sprintf("%f", *f.Min))
		}
		if f.Max != nil {
			io.WriteString(h, fmt.Sprintf("%f", *f.Max))
		}

		io.WriteString(h, fmt.Sprintf("%d", f.ItemType))
		if f.NestedSchema != nil {
			io.WriteString(h, f.NestedSchema.ID())
		}
		if f.MinItems != nil {
			io.WriteString(h, fmt.Sprintf("%d", *f.MinItems))
		}
		if f.MaxItems != nil {
			io.WriteString(h, fmt.Sprintf("%d", *f.MaxItems))
		}
		io.WriteString(h, fmt.Sprintf("%t", f.UniqueItems))
	}
	return hex.EncodeToString(h.Sum(nil))
}
