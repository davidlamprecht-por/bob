# AI Layer

The AI Layer provides a provider-agnostic interface for AI-powered structured conversations. It enables workflows to interact with various AI services (OpenAI, Anthropic, etc.) using a unified schema builder API.

## Architecture

```
Workflow → Actions → Orchestrator → AI Layer → Provider (OpenAI/Anthropic/etc.) → AI API
```

The AI Layer consists of:
- **Schema Builder** - Fluent API for defining structured output schemas
- **Provider Interface** - Common interface all AI providers must implement
- **Response Wrapper** - Type-safe access to AI responses

## Schema Builder

The `SchemaBuilder` provides a fluent, chainable API for defining structured AI outputs without writing JSON Schema or provider-specific code.

### Basic Example

```go
schema := ai.NewSchema().
    AddString("response", ai.Required(), ai.Description("Message to user")).
    AddBool("is_complete", ai.Required(), ai.Description("All info collected"))

resp, _ := openai.SendMessage(ctx, &convID, prompt, personality, schema)
data := resp.Data()

message := data.MustGetString("response")
complete := data.MustGetBool("is_complete")
```

### Field Types

- **AddString(name, opts...)** - String field
- **AddInt(name, opts...)** - Integer field
- **AddFloat(name, opts...)** - Floating point number field
- **AddBool(name, opts...)** - Boolean field
- **AddArray(name, itemType, opts...)** - Array field with typed items
- **AddObject(name, nestedBuilder, opts...)** - Nested object field

### Field Options

**Common:**
- `Required()` - Mark field as required
- `Description(string)` - Add field description
- `Default(any)` - Set default value

**String:**
- `Enum(values...)` - Restrict to specific values
- `MinLength(int)` - Minimum string length
- `MaxLength(int)` - Maximum string length
- `Pattern(regex)` - Regex pattern validation

**Numeric (Int/Float):**
- `Range(min, max)` - Min and max values (shorthand)
- `Min(value)` - Minimum value
- `Max(value)` - Maximum value

**Array:**
- `MinItems(int)` - Minimum array length
- `MaxItems(int)` - Maximum array length
- `UniqueItems()` - Require unique elements

### Complex Example

```go
schema := ai.NewSchema().
    AddString("title", ai.Required(), ai.Description("Ticket title"), ai.MinLength(5)).
    AddString("priority", ai.Required(), ai.Enum("low", "medium", "high", "critical")).
    AddString("description", ai.Description("Detailed description"), ai.MaxLength(1000)).
    AddInt("estimated_hours", ai.Range(1, 100), ai.Description("Estimated work hours")).
    AddArray("tags", ai.FieldTypeString, ai.MinItems(1), ai.MaxItems(5)).
    AddBool("is_urgent", ai.Default(false))

resp, _ := openai.SendMessage(ctx, nil, prompt, personality, schema)
data := resp.Data()

title := data.MustGetString("title")
priority := data.MustGetString("priority")
hours, err := data.GetInt("estimated_hours")  // Optional field
tags := data.MustGetArray("tags")
```

### Nested Objects

```go
addressBuilder := ai.NewSchema().
    AddString("street", ai.Required()).
    AddString("city", ai.Required()).
    AddString("zip", ai.Pattern("\\d{5}"))

personSchema := ai.NewSchema().
    AddString("name", ai.Required()).
    AddInt("age", ai.Range(0, 120)).
    AddObject("address", addressBuilder)

resp, _ := openai.SendMessage(ctx, nil, prompt, personality, personSchema)
data := resp.Data()

name := data.MustGetString("name")
addressMap := data.MustGetObject("address")
addressData := &ai.SchemaData{Data: addressMap}
city := addressData.MustGetString("city")
```

## Response Access

The `SchemaData` wrapper provides type-safe accessors for response fields.

### Safe Getters (Return Error)

```go
data := resp.Data()

str, err := data.GetString("field")
num, err := data.GetInt("field")
flt, err := data.GetFloat("field")
bol, err := data.GetBool("field")
arr, err := data.GetArray("field")
obj, err := data.GetObject("field")
```

### Must Getters (Panic on Error)

```go
str := data.MustGetString("field")  // Panic if field missing or wrong type
num := data.MustGetInt("field")
```

### Utility Methods

```go
data.Has("field")     // Check if field exists
data.IsSet("field")   // Check if field exists and is not nil
data.Raw()            // Get underlying map[string]any
```

## Provider Interface

All AI providers implement the same interface:

```go
type Provider interface {
    SendMessage(
        ctx context.Context,
        conversationID *string,
        userPrompt string,
        personality string,
        schemaBuilder *SchemaBuilder,
        opts ...Option,
    ) (*Response, error)

    Connect(apiKey string) error
    Close() error
}
```

This allows workflows to switch providers without changing code:

```go
// Use OpenAI
resp, _ := openai.SendMessage(ctx, convID, prompt, personality, schema)

// Future: Use Anthropic (same interface)
resp, _ := anthropic.SendMessage(ctx, convID, prompt, personality, schema)
```

## Benefits

1. **Provider-Agnostic** - Same schema builder works with any AI provider
2. **Type-Safe** - Compile-time checking of field types and options
3. **Discoverable** - IDE autocomplete for all options
4. **No Boilerplate** - No struct definitions needed
5. **Flexible** - Build schemas dynamically at runtime
6. **Cached** - Schemas are converted once and cached per provider
7. **Clear Errors** - Type mismatches and missing fields have clear error messages

## Files

- `provider.go` - Provider interface and Response type
- `schema_builder.go` - SchemaBuilder fluent API
- `schema_types.go` - FieldDef, FieldType, and field option functions
- `schema_response.go` - SchemaData wrapper for type-safe response access
