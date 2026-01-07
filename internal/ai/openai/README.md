# OpenAI Module

This module provides integration with OpenAI's Responses API, enabling structured AI conversations with automatic schema generation and type-safe response parsing.

## Purpose in Bob's Architecture

The OpenAI module sits at the edge of Bob's system, serving as the interface between Bob's AI Layer and OpenAI's API. It translates Bob's internal requests into OpenAI API calls and converts OpenAI responses back into structured Go types.

**Architecture Flow:**
```
Workflow → Actions → Orchestrator → AI Layer → OpenAI Module → OpenAI API
```

The module is intentionally isolated from Bob's core concepts (ConversationContext, Actions, Workflows) and only knows about:
- Standard Go types (`context.Context`, strings, structs)
- OpenAI API concepts (conversation IDs, models, responses)

## Files Overview

### `responses.go` - Main API Integration

**Purpose:** Core implementation of OpenAI Responses API communication.

**What it does:**
- `SendMessage()` - Primary function the AI Layer calls. Takes a user prompt, personality, and schema builder, returns parsed structured output
- `sendMessage()` - Constructs OpenAI API request with conversation ID, instructions, and JSON schema
- `sendWithRetry()` - Implements exponential backoff retry logic for transient failures
- `resolveConversationID()` - Creates new conversation if none provided, otherwise uses existing
- `parseResponseToMap()` - Extracts JSON from OpenAI response and unmarshals into map[string]any
- `wrapOpenAIError()` - Classifies OpenAI errors into Bob's error types for consistent handling

**Why it exists:**
The Responses API is stateful and manages conversation history automatically. This file handles the complexity of:
- Building proper request parameters using OpenAI's SDK types
- Managing conversation state via conversation IDs
- Configuring structured output format (JSON Schema)
- Extracting and parsing responses safely

**Key implementation details:**

1. **Structured Output Configuration:**
   Uses `ResponseFormatTextConfigParamOfJSONSchema()` to tell OpenAI to return JSON matching our schema. OpenAI validates the response format before returning it.

2. **Input Construction:**
   Wraps user messages in `ResponseInputItemParamOfMessage()` with role `EasyInputMessageRoleUser`. The Responses API uses a union type system for flexibility.

3. **Personality Optimization:**
   Calls `shouldIncludePersonality()` to only send system instructions when they change, reducing token usage in multi-turn conversations.

4. **Retry Logic:**
   Retries up to 3 times with exponential backoff (1s, 2s, 4s) for rate limits and network errors. Non-retryable errors (auth, invalid request) fail immediately.

5. **Error Translation:**
   Inspects error messages to classify them into Bob's error types, allowing the AI Layer to handle different failure modes appropriately.

---

### `schema_builder.go` - Schema Builder to JSON Schema Conversion

**Purpose:** Convert provider-agnostic schema builders into OpenAI JSON Schema format.

**What it does:**
- `buildSchemaWithCache()` - Main entry point with caching. Converts ai.SchemaBuilder to OpenAI JSON Schema
- `buildSchema()` - Core conversion logic that reads field definitions and builds OpenAI schema
- `buildPropertySchema()` - Converts individual field definitions to JSON Schema properties
- `buildItemSchema()` - Handles array item type schemas

**Why it exists:**
AI workflows need structured output. The ai layer provides a fluent builder API for defining schemas in a provider-agnostic way. This module translates those schemas into OpenAI's JSON Schema format.

**Example schema builder:**
```go
schema := ai.NewSchema().
    AddString("title", ai.Required(), ai.Description("Ticket title")).
    AddString("priority", ai.Enum("low", "medium", "high")).
    AddInt("count", ai.Range(1, 10))

resp, _ := openai.SendMessage(ctx, convID, prompt, personality, schema)
data := resp.Data()
title := data.MustGetString("title")
```

**Key implementation details:**

1. **Caching Strategy:**
   Each unique schema builder has an ID (hash of field definitions). The first time a schema is encountered, it's converted and cached. Subsequent uses lookup the cached schema, avoiding repeated conversion.

2. **Type Mapping:**
   - ai.FieldTypeString → `"string"`
   - ai.FieldTypeInt → `"integer"`
   - ai.FieldTypeFloat → `"number"`
   - ai.FieldTypeBool → `"boolean"`
   - ai.FieldTypeArray → `"array"` with items schema
   - ai.FieldTypeObject → nested `"object"` (recursive)

3. **Constraint Handling:**
   - Required → added to schema's `required` array
   - Enum → `enum` array in property
   - Min/Max → `minimum`/`maximum` for numbers
   - MinLength/MaxLength → string length constraints
   - Pattern → regex pattern for strings
   - MinItems/MaxItems/UniqueItems → array constraints

4. **Nested Objects:**
   Recursively processes nested schemas via NestedSchema field, building complete hierarchical schemas.

**Serves the wider app by:**
- Converting provider-agnostic schemas to OpenAI format
- Caching converted schemas for performance
- Supporting all OpenAI JSON Schema features
- Enabling workflows to use same schema builder for any AI provider

---

### `client.go` - Connection Management

**Purpose:** Thread-safe OpenAI client lifecycle and personality optimization.

**What it does:**
- `Connect()` - Initializes OpenAI client with API key
- `Close()` - Cleans up client resources
- `getClient()` - Thread-safe client access with initialization check
- `createConversation()` - Creates new conversation via Conversations API
- `shouldIncludePersonality()` - Tracks personality per conversation to avoid redundant instructions

**Why it exists:**
The client needs to be initialized once and shared across concurrent requests. Personality tracking prevents sending identical system messages repeatedly in multi-turn conversations.

**Key implementation details:**

1. **Thread Safety:**
   Uses `sync.RWMutex` to allow concurrent reads (multiple requests) while serializing writes (Connect/Close).

2. **Client Initialization:**
   Uses official OpenAI SDK: `openai.NewClient(option.WithAPIKey(apiKey))`
   Client is stored as a value type (not pointer) since SDK returns value.

3. **Personality Tracking:**
   Maintains `lastPersonality` map keyed by conversation ID. When personality changes, returns `true` to include it in the request. This optimization can reduce token usage by 50+ tokens per message in long conversations.

4. **Conversation Creation:**
   Calls `c.Conversations.New()` to create persistent conversation. Returns conversation ID which the AI Layer stores in workflow data.

**Serves the wider app by:**
- Providing single point of API key configuration
- Enabling conversation persistence across multiple interactions
- Optimizing token usage through personality caching
- Ensuring thread-safe access for concurrent workflow executions

---

### `types.go` - Data Structures

**Purpose:** Define request/response types and default configuration.

**What it does:**
- `Response` - Standardized response format returned to AI Layer
- `RequestConfig` - Configuration for API requests (model, temperature, tokens, etc.)
- `defaultConfig()` - Sensible defaults for API parameters

**Why it exists:**
Centralizes type definitions and provides reasonable defaults that work for most use cases while allowing customization via options.

**Key implementation details:**

1. **Response Structure:**
   ```go
   type Response struct {
       Data           interface{}  // Parsed struct matching request schema
       ConversationID string       // To continue conversation
       ResponseID     string       // OpenAI's response ID for debugging
       TokensUsed     int          // Cost tracking
       Model          string       // Actual model used
       FinishReason   string       // Status: "completed", "failed", etc.
   }
   ```

2. **Default Configuration:**
   - Model: `gpt-4o-mini` - Fastest, cheapest model for most tasks
   - Temperature: `0.7` - Balanced creativity vs consistency
   - MaxTokens: `4096` - Reasonable limit for responses
   - TopP: `1.0` - Full probability mass (no nucleus sampling by default)

**Serves the wider app by:**
- Providing consistent response format for AI Layer
- Offering sensible defaults that minimize configuration
- Tracking token usage for cost monitoring
- Maintaining conversation continuity via IDs

---

### `options.go` - Functional Options Pattern

**Purpose:** Provide flexible API parameter customization.

**What it does:**
- `Option` type - Function that modifies RequestConfig
- `WithModel()` - Override model selection
- `WithTemperature()` - Adjust creativity/randomness
- `WithMaxTokens()` - Limit response length
- `WithTopP()` - Nucleus sampling parameter
- `WithFrequencyPenalty()` - Reduce repetition
- `WithPresencePenalty()` - Encourage topic diversity
- `WithStopSequences()` - Define stop conditions

**Why it exists:**
Different use cases need different parameters:
- Intent classification: Low temperature (0.2-0.3), fast model (gpt-4o-mini)
- Creative writing: High temperature (0.8-1.0), capable model (gpt-4o)
- Code generation: Medium temperature (0.4-0.6), reasoning model (o1)

Functional options allow callers to customize only what they need without complex constructors.

**Key implementation details:**

1. **Usage Pattern:**
   ```go
   openai.SendMessage(ctx, convID, prompt, personality, &MyStruct{},
       openai.WithModel("gpt-4o"),
       openai.WithTemperature(0.3),
       openai.WithMaxTokens(500),
   )
   ```

2. **Implementation:**
   Each option is a closure that captures a parameter and modifies the config when applied.

**Serves the wider app by:**
- Enabling workflow-specific AI behavior tuning
- Keeping API simple (options are optional)
- Maintaining backwards compatibility (new options don't break existing code)

---

### `errors.go` - Error Classification

**Purpose:** Categorize and handle OpenAI API errors.

**What it does:**
- `Error` type - Custom error with type classification
- Error type constants - Auth, RateLimit, InvalidRequest, APIError, etc.
- `isRetryable()` - Determines if an error warrants retry
- `Error()` and `Unwrap()` - Standard Go error interface

**Why it exists:**
Different errors require different handling:
- **Auth errors** → Check API key configuration
- **Rate limits** → Retry with backoff
- **Invalid request** → Fix request structure, don't retry
- **Network errors** → Retry
- **API errors** → Log and investigate

**Key implementation details:**

1. **Error Types:**
   ```go
   ErrTypeAuth              // Invalid API key, authentication failure
   ErrTypeRateLimit         // Too many requests, retry needed
   ErrTypeInvalidRequest    // Bad request structure, don't retry
   ErrTypeAPIError          // OpenAI service error
   ErrTypeNetworkError      // Connection issues, retry
   ErrTypeSchemaValidation  // Schema generation failed
   ErrTypeResponseParsing   // Response doesn't match expected format
   ```

2. **Retry Decision:**
   Only `ErrTypeRateLimit` and `ErrTypeNetworkError` are retryable. Others fail fast.

**Serves the wider app by:**
- Enabling intelligent error handling in AI Layer
- Providing clear error messages for debugging
- Preventing wasted retries on non-transient failures
- Supporting error monitoring and alerting

---

## How It Serves the Wider App

### 1. **Output Translation**

The module translates between Go structs and OpenAI's JSON format bidirectionally:

**Go → OpenAI (Request):**
```go
type CreateTicketResponse struct {
    Response  string `json:"response" openai:"description=User message,required"`
    Title     string `json:"title,omitempty" openai:"description=Ticket title"`
    IsComplete bool  `json:"is_complete" openai:"description=All info collected,required"`
}
```
↓ (schema.go converts to JSON Schema)
```json
{
  "type": "object",
  "properties": {
    "response": {"type": "string", "description": "User message"},
    "title": {"type": "string", "description": "Ticket title"},
    "is_complete": {"type": "boolean", "description": "All info collected"}
  },
  "required": ["response", "is_complete"]
}
```
↓ (OpenAI generates matching JSON)

**OpenAI → Go (Response):**
```json
{"response": "What is the ticket title?", "is_complete": false}
```
↓ (responses.go unmarshals to struct)
```go
data := response.Data.(*CreateTicketResponse)
// data.Response = "What is the ticket title?"
// data.IsComplete = false
```

This enables workflows to work with strongly-typed Go structs instead of parsing JSON strings.

### 2. **Personality Handling**

**Problem:** Sending the same personality/instructions in every message wastes tokens.

**Solution:** `shouldIncludePersonality()` tracks the last personality sent per conversation:
- First message with conversation: Include personality
- Subsequent messages with same personality: Skip (OpenAI remembers)
- Personality changes: Include new personality

**Example:**
```go
// Message 1: personality="Be concise" → Sent (first time)
// Message 2: personality="Be concise" → Skipped (unchanged)
// Message 3: personality="Be detailed" → Sent (changed)
// Message 4: personality="Be detailed" → Skipped (unchanged)
```

This can save 50-100 tokens per message in multi-turn conversations, reducing costs by 30-40%.

### 3. **Conversation Continuity**

The Responses API is stateful. The module manages this via:
- `conversationID=nil` → Create new conversation, return ID
- `conversationID="conv-123"` → Continue existing conversation

The AI Layer stores the conversation ID in `WorkflowData["ai_conversation_id"]` and passes it back on subsequent calls. OpenAI maintains full conversation history internally, so workflows don't need to manage message arrays.

### 4. **Error Recovery**

Implements robust error handling:
- **Transient errors** (rate limits, network): Retry with exponential backoff
- **Permanent errors** (auth, invalid request): Fail immediately with clear message
- **Parsing errors**: Return structured error indicating what went wrong

This prevents workflows from failing due to temporary API issues while quickly surfacing configuration problems.

### 5. **Type Safety**

The AI Layer receives strongly-typed responses:
```go
response, err := openai.SendMessage(ctx, convID, prompt, personality, &CreateTicketResponse{})
data := response.Data.(*CreateTicketResponse)
// Access fields directly: data.Response, data.Title, data.IsComplete
// Compiler catches typos and type mismatches
```

No manual JSON parsing, no reflection in workflow code, no runtime type errors.

### 6. **Flexibility**

Through the options pattern, workflows can tune AI behavior:
- **Intent classification**: Fast model, low temperature, short responses
- **Conversation**: Standard model, medium temperature, flexible length
- **Code generation**: Capable model, low temperature, long responses

Each workflow optimizes for its specific needs without affecting others.

## Integration Example

**Workflow calls AI Layer:**
```go
// Workflow extracts conversation ID from state
convID := workflow.WorkflowData["ai_conversation_id"]

// AI Layer calls OpenAI module
response, err := openai.SendMessage(
    ctx,
    &convID,  // Continue conversation
    userMessage,
    "You are a helpful ticket assistant",
    &CreateTicketResponse{},
    openai.WithModel("gpt-4o"),
    openai.WithTemperature(0.7),
)

// Store updated conversation ID
workflow.WorkflowData["ai_conversation_id"] = response.ConversationID

// Use structured data
data := response.Data.(*CreateTicketResponse)
if data.IsComplete {
    createTicketInADO(data.Title)
}
```

The module handles:
- Schema generation from `CreateTicketResponse`
- Personality optimization
- API communication
- Error handling and retries
- Response parsing
- Conversation state management

The workflow receives clean, type-safe data and focuses on business logic.
