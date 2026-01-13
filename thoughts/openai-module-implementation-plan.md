# OpenAI Responses API Module Implementation Plan

## Overview

Build an OpenAI module at `/media/dlamprecht/Data/goProjects/bob/internal/openai/` that integrates OpenAI's Responses API to provide conversation management and structured outputs for the Bob project.

## Key Requirements

✅ Use OpenAI **Responses API** (stateful conversation management)
✅ Conversation ID management (create new or use existing)
✅ Per-request personality control (system messages)
✅ **Structured outputs** via Go struct tags → JSON Schema reflection
✅ Thread-safe, following database module pattern
✅ Module does NOT store message history (Responses API handles this)
✅ Define clean interface for AI Layer to call

## Architecture

### Module Structure

```
internal/openai/
├── client.go          # Client initialization (Connect pattern)
├── responses.go       # Core SendMessage implementation
├── schema.go          # Struct tag → JSON Schema conversion
├── types.go           # Request/Response types
├── errors.go          # Error handling
├── options.go         # Functional options pattern
└── conversation.go    # Conversation helpers
```

### Primary Interface for AI Layer

```go
// Core function AI Layer will call
func SendMessage(
    ctx context.Context,
    conversationID *string,        // nil = create new, non-nil = use existing
    userPrompt string,
    personality string,             // Per-request system message
    responseStruct interface{},     // Struct with openai tags for structured output
    opts ...Option,                 // WithModel(), WithTemperature(), etc.
) (*Response, error)

type Response struct {
    Data           interface{}  // Parsed structured output matching responseStruct
    ConversationID string       // New or existing conversation ID
    ResponseID     string       // OpenAI response ID
    TokensUsed     int
}
```

## Structured Output: Go Struct Tags

### Tag Format

```go
type CreateTicketResponse struct {
    Response       string `json:"response" openai:"description=Message to user,required"`
    OverallSummary string `json:"overall_summary" openai:"description=Conversation summary,required"`
    TicketTitle    string `json:"ticket_title,omitempty" openai:"description=Ticket title"`
    Priority       string `json:"priority" openai:"description=Priority level,enum=low|medium|high"`
    IsComplete     bool   `json:"is_complete" openai:"description=All info collected,required"`
}
```

### Supported Tag Attributes

- `description=...` - Field description for schema
- `required` - Mark field as required
- `enum=val1|val2|val3` - Enum values (pipe-separated)
- `min=N`, `max=N` - Numeric constraints
- `minLength=N`, `maxLength=N` - String constraints

### Implementation (schema.go)

Use reflection to:
1. Parse struct fields and tags
2. Build OpenAI JSON Schema recursively
3. Handle nested structs, arrays, maps
4. Validate schema before sending to API

## Conversation Management

**Flow:**
```
conversationID == nil → Create new conversation via Conversations API → Return new ID
conversationID != nil → Use existing conversation ID → Return same ID
```

OpenAI Responses API manages full conversation history internally. Module only tracks conversation ID.

## Personality Control

Personality passed as **system message** with each request. User may change personality between messages.

```go
messages := []Message{
    {Role: "system", Content: personality},
    {Role: "user", Content: userPrompt},
}
```

**Optimization**: Track last personality per conversation, only include system message when it changes.

## Client Initialization

Following database module pattern:

```go
package openai

var (
    Client *openai.Client
    mu     sync.RWMutex
)

func Connect(apiKey string) error {
    mu.Lock()
    defer mu.Unlock()

    Client = openai.NewClient(apiKey)
    return healthCheck()
}
```

**Usage in main.go:**
```go
import "bob/internal/openai"

func main() {
    config.Init()

    if err := openai.Connect(config.Current.OpenAIAPIKey); err != nil {
        log.Fatalf("Failed to connect to OpenAI: %v", err)
    }
    defer openai.Close()
}
```

## Options Pattern

```go
type Option func(*RequestConfig)

// Usage:
response, err := openai.SendMessage(
    ctx, conversationID, prompt, personality, &MyStruct{},
    openai.WithModel("gpt-4o"),
    openai.WithTemperature(0.7),
    openai.WithMaxTokens(1000),
)
```

**Default values:**
- Model: `gpt-4o-mini`
- Temperature: `0.7`
- MaxTokens: `4096`

## Error Handling

Custom error types:
```go
type ErrorType int

const (
    ErrTypeAuth
    ErrTypeRateLimit
    ErrTypeInvalidRequest
    ErrTypeSchemaValidation
    ErrTypeResponseParsing
)
```

Retry logic for rate limits and transient errors (exponential backoff, max 3 retries).

## Implementation Phases

### Phase 1: Core Infrastructure
**Files:** `client.go`, `types.go`, `errors.go`

- Package-level client initialization
- Connect() function with health check
- Error type definitions
- Thread-safety (RWMutex)

### Phase 2: Schema Builder
**Files:** `schema.go`

- Reflection logic to parse struct tags
- JSON Schema generation from Go structs
- Type mapping (string, int, bool, arrays, nested)
- Tag parsing (description, required, enum, etc.)
- Schema validation

**Critical:** This is the foundation for structured outputs.

### Phase 3: Responses API Integration
**Files:** `responses.go`, `conversation.go`, `options.go`

- SendMessage() implementation
- Conversation creation/management via Conversations API
- Structured output request configuration
- Response parsing into struct
- Personality (system message) handling

### Phase 4: Testing & Polish
- Unit tests for schema generation
- Integration tests with OpenAI API (optional, behind flag)
- Error handling edge cases
- Documentation

## Key Implementation Challenges

### 1. SDK Selection

**Decision:** Use the **official OpenAI Go SDK** (`github.com/openai/openai-go/v3`)

**Rationale:**
- Native Responses API support via `client.Responses.New()`
- Maintained directly by OpenAI
- Current with latest API features
- Requires Go 1.22+ (project uses Go 1.25.5 ✓)

**Import paths:**
```go
import (
    "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
    "github.com/openai/openai-go/v3/responses"
)
```

This replaces the third-party `github.com/sashabaranov/go-openai` currently in the project.

### 2. text.format vs response_format

Responses API uses `text.format` for structured outputs (different from Chat Completions' `response_format`).

**Solution:** Abstract difference internally, AI Layer doesn't need to know.

### 3. Conversation State & Personality

System messages may accumulate in stateful conversations, increasing token usage.

**Solution:** Track last personality per conversation, only inject system message when it changes.

### 4. Response Parsing Type Safety

Parse OpenAI JSON into arbitrary struct types while maintaining type safety.

**Solution:** Use `json.Unmarshal` with provided struct type, validate required fields are present.

## Usage Examples

### Workflow Usage

```go
func CreateTicket(ctx *core.ConversationContext, action *core.Action) ([]*core.Action, error) {
    // Get conversation ID from workflow data
    var convID *string
    if storedID, ok := ctx.GetCurrentWorkflow().WorkflowData["ai_conversation_id"].(string); ok {
        convID = &storedID
    }

    personality := `You are a helpful assistant guiding users through creating ADO tickets.
Be concise and professional. Ask one question at a time.`

    userPrompt := ctx.GetLastUserMessages()[0].Message

    // Call OpenAI
    response, err := openai.SendMessage(
        context.Background(),
        convID,
        userPrompt,
        personality,
        &CreateTicketResponse{},
        openai.WithModel("gpt-4o"),
    )

    if err != nil {
        return nil, err
    }

    // Store conversation ID
    ctx.GetCurrentWorkflow().WorkflowData["ai_conversation_id"] = response.ConversationID

    // Use structured output
    data := response.Data.(*CreateTicketResponse)

    if data.IsComplete {
        return createTicketInADO(data.TicketTitle), nil
    } else {
        return []*core.Action{{
            ActionType: core.ActionUserMessage,
            Input: map[core.InputType]any{
                core.InputMessage: data.Response,
            },
        }}, nil
    }
}
```

### Intent Analysis Usage

```go
type IntentResponse struct {
    IntentType   string  `json:"intent_type" openai:"description=Intent type,required,enum=new_workflow|answer_question|ask_question"`
    WorkflowName string  `json:"workflow_name,omitempty" openai:"description=Workflow name"`
    Confidence   float64 `json:"confidence" openai:"description=Confidence score,required,min=0,max=1"`
}

func AnalyzeIntent(message *core.Message, ctx *core.ConversationContext) core.Intent {
    personality := `You are an intent classifier. Determine if the user is starting a workflow, answering a question, or asking a question.`

    // No conversation ID for ephemeral intent analysis
    response, err := openai.SendMessage(
        context.Background(),
        nil,
        message.Message,
        personality,
        &IntentResponse{},
        openai.WithModel("gpt-4o-mini"),  // Cheaper model
        openai.WithTemperature(0.3),      // Low temp for consistency
    )

    if err != nil {
        log.Printf("Intent analysis failed: %v", err)
        return defaultIntent()
    }

    data := response.Data.(*IntentResponse)
    return core.Intent{
        IntentType:   mapIntentType(data.IntentType),
        WorkflowName: data.WorkflowName,
    }
}
```

## Critical Files

**Implementation files:**
1. `internal/openai/responses.go` - Core SendMessage() implementation
2. `internal/openai/schema.go` - Struct tag → JSON Schema reflection
3. `internal/openai/client.go` - Thread-safe client initialization

**Reference files:**
4. `internal/database/database.go` - Pattern for module initialization
5. `internal/workflow/workflow.go` - Consumer showing usage patterns
6. `cmd/openai-test/main.go` - Existing OpenAI SDK usage example

## Testing Strategy

**Unit Tests:**
- Schema generation from various struct types
- Tag parsing (required, enum, min/max, etc.)
- Nested struct handling
- Error cases (invalid structs, malformed tags)

**Integration Tests** (optional, behind `testing.Short()` flag):
- Real OpenAI API calls
- Conversation creation and reuse
- Structured output parsing
- Error handling (auth, rate limits)

**Mock Service** for testing workflows without API calls.

## Performance & Security

**Performance:**
- Thread-safe concurrent requests (RWMutex)
- Responses API provides 40-80% cache hit improvement vs Chat Completions
- Track personality changes to minimize redundant system messages

**Security:**
- API key from environment (never logged)
- Input validation (sanitize prompts, validate struct types)
- Response validation (check required fields, prevent injection)

## Sources

- [Responses API Reference](https://platform.openai.com/docs/api-reference/responses)
- [Migrate to Responses API](https://platform.openai.com/docs/guides/migrate-to-responses)
- [Conversations API](https://platform.openai.com/docs/api-reference/conversations/create)
- [Structured Outputs Guide](https://platform.openai.com/docs/guides/structured-outputs)
- [Go SDK Guide](https://chris.sotherden.io/openai-responses-api-using-go/)
