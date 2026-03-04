# AI Layer

The AI layer is Bob's interface to language model providers. Its job is to keep the rest
of the codebase completely unaware of which AI provider is in use, how conversations are
stored, or how structured output is parsed. Everything above this layer works with
generic types; everything below is provider-specific.

---

## Package Structure

```
internal/ai/
├── provider.go        # Provider interface + Option types + BranchFromResponse
├── ai_client.go       # Global AIClient, SendMessage convenience function
├── init.go            # Startup: connects provider, sets global client
├── schema_builder.go  # Fluent API for building structured output schemas
├── schema_types.go    # FieldDef, FieldOption, field constraint helpers
├── schema_response.go # SchemaData — typed accessors for AI response fields
└── openai/
    ├── provider.go    # Implements ai.Provider, routes options, entry point for OpenAI
    ├── client.go      # OpenAI client lifecycle, conversation creation, personality cache
    ├── responses.go   # Core send logic: conversation routing, retry, response parsing
    ├── options.go     # OpenAI-specific Option funcs (WithModel, WithTemperature, etc.)
    ├── types.go       # RequestConfig, defaultConfig
    ├── errors.go      # Error types, isRetryable
    └── schema_builder.go  # Converts ai.SchemaBuilder → OpenAI JSON schema format
```

---

## How It All Fits Together

### Startup

```
main.go
  import _ "bob/internal/ai/openai"   ← triggers openai.init(), registers provider
  ai.Init()                            ← connects provider with API key, sets global client
```

`RegisterDefaultProvider` is called by the provider's `init()` function. This means
adding a new provider is just importing the package — nothing else in the codebase
changes.

### A Call from a Workflow

```
workflow ActionAI
  → process_actions.go ActionAI()
      → ai.SendMessage()                       ← global convenience function (ai package)
          → AIClient.SendMessageFromOrchestrator()
              → openai.provider.SendMessage()  ← checks for special options (BranchOption)
                  → openai.SendMessage()       ← resolves conversation, builds schema
                      → sendWithRetry()
                          → sendMessage()      ← builds OpenAI params, calls API
```

---

## The Provider Interface

```go
type Provider interface {
    SendMessage(ctx context.Context, conversationID *string,
        userPrompt, personality string, schema *SchemaBuilder,
        opts ...Option) (*Response, error)
    Connect(apiKey string) error
    Close() error
}
```

`conversationID` is a pointer so `nil` means "start a new conversation." The provider
creates one and returns its ID in `Response.ConversationID`. On subsequent calls the
caller passes that ID back to continue the conversation.

---

## Conversations — Three Modes

This is the most important thing to understand when writing workflows. Every AI call
belongs to one of three conversation modes, controlled entirely by `conversationKey`
on `ActionAI` and how you manage `WorkflowContext`.

### 1. Main shared thread (default, `conversationKey: ""`)

All calls with an empty conversation key share one AI conversation for the life of the
workflow. The model remembers everything — prior questions, answers, tool results.
Use this for normal back-and-forth with the user.

```go
core.NewAIAction(prompt, personality, schema, "")  // "" = main thread
```

### 2. Isolated fresh thread (`conversationKey: "some_name"`)

A completely separate conversation with no shared history. The first call creates a new
conversation; subsequent calls with the same key continue it. Use this when a sub-task
needs to reason independently — a summarizer, a validator, a parallel worker.

```go
core.NewAIAction(prompt, personality, schema, "my_isolated_task")
```

### 3. Branch off existing history (fork, `BranchFromResponse` + named key)

A conversation that starts at a specific point in another conversation's history.
The model sees everything up to that point, then diverges freely. The original
conversation is completely unaffected. See the **Branching** section below.

---

## Conversation ID Format — How the AI Layer Routes Calls

Under the hood, conversation IDs come in two formats that the AI layer detects
automatically:

| Format | Source | API used |
|---|---|---|
| `conv_xxx` | OpenAI Conversations API | `Conversation` field in request |
| `resp_xxx` | OpenAI Response object ID | `PreviousResponseID` field in request |

`sendMessage` checks the prefix and routes to the correct API field. **This is
completely transparent to callers** — you store whatever `ConversationID` comes back
and pass it in next time. No special handling needed.

When using `resp_xxx` chain mode, `ConversationID` advances to the new `resp.ID` after
every call (the old ID is no longer the tip). When using `conv_xxx` mode,
`ConversationID` stays the same forever.

---

## Branching

Branching lets you read the full context of an existing conversation without modifying
it, and optionally continue from that point in a new direction.

The OpenAI Responses API stores every response as an immutable server-side object.
When you use `previous_response_id`, OpenAI walks the chain to reconstruct history and
feeds it to the model — but the parent object is never modified. A parent can have any
number of children.

```
Main thread:   A → B → C → D         (conv_xxx, never changes)
Branch:                    D → E     (resp_xxx chain, orphaned or continued)
Main continues:            D → F     (main thread unaware of E)
```

### Read-only branch (discard)

Pass `ai.BranchFromResponse` as an option. The call sees full conversation context but
the returned `ConversationID` can be ignored — the original thread is unaffected.

```go
resp, err := ai.SendMessage(ctx, nil, prompt, personality, schema,
    ai.BranchFromResponse(*wf.GetLastResponseID()),
)
// resp.ConversationID = "resp_xxx" — ignore it if you don't need to continue
```

`lastResponseID` on `WorkflowContext` is updated automatically after every main-thread
AI call by `process_actions.go`. Workflows don't need to manage it themselves.

### Fork and continue (store the result)

Same call, but store `resp.ConversationID` under a named key. From then on, `ActionAI`
calls with that key continue on the branch exactly like any other conversation. The
`resp_` prefix is detected automatically — no extra options needed on subsequent calls.

```go
// Fork off the main conversation
resp, _ := ai.SendMessage(ctx, nil, firstPrompt, personality, schema,
    ai.BranchFromResponse(*wf.GetLastResponseID()),
)

// Store the branch tip — it behaves like any other named conversation from here on
key := "alt_thread"
id  := resp.ConversationID        // "resp_xxx"
wf.SetAIConversation(&key, &id)

// Later — normal ActionAI with conversationKey="alt_thread" continues the branch
// Each continued call returns the new resp_ ID, SetAIConversation updates automatically
```

### When to use which

| Goal | Approach |
|---|---|
| Routing/intent check with conversation context | Branch + discard |
| Safety or moderation check mid-workflow | Branch + discard |
| Sub-workflow that shares history but diverges freely | Branch + continue |
| Parallel analysis of same context | Multiple branches off same response ID |
| Fully isolated sub-task (no shared history) | Named `conversationKey` (fresh thread) |
| Normal workflow step | Regular `ActionAI`, no branching |

**Full design rationale:** `thoughts/shared/patterns/ai-response-branching.md`

---

## Structured Output — SchemaBuilder

All AI calls in Bob use structured JSON output. The AI fills in a defined schema;
OpenAI validates the response against it before returning, so you never get back a
malformed structure.

### Defining a schema

```go
schema := ai.NewSchema().
    AddString("workflow_name", ai.Required(), ai.Description("The workflow to run")).
    AddFloat("confidence",     ai.Required(), ai.Range(0.0, 1.0)).
    AddBool("should_proceed",  ai.Required()).
    AddArray("steps", ai.Required(), ai.Description("Step names"),
        ai.ItemType(ai.FieldTypeString)).
    AddString("message", ai.Description("Optional note")) // omit Required() = optional
```

### Reading the response

```go
data := response.Data()

// Safe getters — return (value, error)
name, err := data.GetString("workflow_name")
conf, err := data.GetFloat("confidence")
ok,   err := data.GetBool("should_proceed")

// Must getters — panic if missing or wrong type (use only for Required() fields)
name := data.MustGetString("workflow_name")

// Optional field pattern
if data.IsSet("message") {       // exists AND non-nil
    msg, _ := data.GetString("message")
}
```

### Available field types

```go
AddString(name, ...FieldOption)
AddInt(name, ...FieldOption)
AddFloat(name, ...FieldOption)
AddBool(name, ...FieldOption)
AddArray(name, ...FieldOption)    // use ai.ItemType() to set element type
AddObject(name, nestedSchema, ...FieldOption)
```

### Available field options

```go
ai.Required()                      // field must be present in the response
ai.Description("...")              // hint for the model about what to put here
ai.Default(value)                  // default value
ai.Enum("a", "b", "c")            // restrict to specific string values
ai.Range(min, max)                 // numeric bounds (inclusive, both sides)
ai.Min(n) / ai.Max(n)             // one-sided numeric bound
ai.MinLength(n) / ai.MaxLength(n) // string length constraint
ai.Pattern("regex")                // string regex constraint
ai.MinItems(n) / ai.MaxItems(n)   // array length constraint
ai.UniqueItems()                   // array elements must be distinct
```

Schemas are cached after first build — passing the same `*SchemaBuilder` pointer twice
does not rebuild the JSON schema.

---

## Options

`ai.Option` is an interface with a single `Apply(config any)` method. Provider-specific
options are handled via type assertion in the provider's `SendMessage`.

### Built-in options (ai package — available everywhere)

```go
ai.BranchFromResponse(responseID string)
// Read or fork an existing conversation thread. See Branching section above.
```

### OpenAI-specific options (openai package)

Only needed in places that import `bob/internal/ai/openai` directly, which is rare.
Normal workflow code goes through `ai.SendMessage` and doesn't need these.

```go
openai.WithModel("gpt-4o")
openai.WithTemperature(0.2)
openai.WithMaxTokens(512)
openai.WithTopP(0.9)
openai.WithFrequencyPenalty(0.5)
openai.WithPresencePenalty(0.5)
openai.WithStopSequences([]string{"END"})
```

---

## Personality (System Prompt) Caching

Each conversation tracks the last personality string sent to it. If the next call uses
the same personality, the `Instructions` field is omitted from the request — saving
50–100 tokens per call on long conversations.

This is automatic. The only implication: if you change a workflow's personality
mid-conversation, the new instructions are sent once and then cached again.

For `resp_xxx` chain calls (branches and forks), personality is always sent because
the conversation ID changes every call, so the cache never matches.

---

## Error Handling

| Type | Retryable | Cause |
|---|---|---|
| `ErrTypeAuth` | No | Bad API key |
| `ErrTypeRateLimit` | Yes | Too many requests |
| `ErrTypeInvalidRequest` | No | Bad schema or params |
| `ErrTypeAPIError` | No (after 3 retries) | Generic OpenAI error |
| `ErrTypeNetworkError` | Yes | Connection issues |
| `ErrTypeResponseParsing` | No | Couldn't parse JSON output |
| `ErrTypeSchemaValidation` | No | Schema build failed |

Retryable errors are retried up to 3 times with exponential backoff (1s → 2s → 4s).
All errors bubble up through `ai.SendMessage` as wrapped `error` values.

---

## Adding a New Provider

1. Create `internal/ai/yourprovider/` with a `provider.go` that implements `ai.Provider`
2. Call `ai.RegisterDefaultProvider(&yourProvider{})` in `init()`
3. Detect `ai.BranchOption` in opts if the provider supports conversation branching
4. Import `_ "bob/internal/ai/yourprovider"` in `main.go`
5. Nothing else in the codebase needs to change
