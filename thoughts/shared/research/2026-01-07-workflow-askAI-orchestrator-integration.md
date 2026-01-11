---
date: 2026-01-07T00:00:00+00:00
researcher: Claude
git_commit: e34fc85a37f23ffe17e10da0ccfde929e8968cff
branch: master
repository: bob
topic: "Workflow askAI Helper Integration with Orchestrator and AI Layer"
tags: [research, codebase, workflow, orchestrator, ai-layer, openai, integration]
status: complete
last_updated: 2026-01-07
last_updated_by: Claude
last_updated_note: "Added design decisions clarifications"
---

# Research: Workflow askAI Helper Integration with Orchestrator and AI Layer

**Date**: 2026-01-07
**Researcher**: Claude
**Git Commit**: e34fc85a37f23ffe17e10da0ccfde929e8968cff
**Branch**: master
**Repository**: bob

## Research Question

What is the next step now to fill out the askAI helper inside workflow package so that it would create an action that gets sent out by orchestrator to the AI layer which then transforms context and other data to ask OpenAI the question? How do we integrate those different ends correctly?

## Summary

The integration path from workflow `askAI` to the AI layer requires implementing four key components:

1. **Workflow → Action**: The `askAI` helper creates an `ActionAi` action with input data (userMsg, systemPrompt, personality, schema, conversationKey)
2. **Orchestrator → AI Layer**: The `ActionAi` case in `ProcessAction()` calls a new AI layer translator
3. **AI Layer Translator** (NEW): Bridges orchestrator context to provider calls, resolving conversation IDs from `ConversationContext.aiConversation` (main) or `WorkflowData` (purpose-specific)
4. **Provider → Response**: The existing OpenAI provider handles API calls and returns structured responses

**Design Decisions Made:**
- **AI Layer Translator**: New intermediary needed to translate orchestrator data to provider calls
- **Sync/Async**: Blocking by default; async via `ActionAsync` pattern with `AsyncGroupID` correlation
- **Conversation IDs**: Main conversation in `ConversationContext.aiConversation`; additional purpose-specific conversations in `WorkflowData["ai_conv_<key>"]`
- **Provider Init**: OpenAI set as default via `ai.Init()` at startup
- **Schema**: Passed as `*SchemaBuilder` parameter to `askAI`
- **Errors**: Flow back via action results; workflows handle retry/failure logic

The codebase has complete provider implementation (OpenAI). Missing pieces: AI translator, `ActionAi` processing, `askAI` helper, and conversation ID management.

## Detailed Findings

### The askAI Helper (Current State)

**Location**: `/media/dlamprecht/Data/goProjects/bob/internal/workflow/workflow_funcs.go:18-20`

```go
func askAI(userMsg *string, systemPrompt string, personality string, ) {

}
```

The function is currently an empty stub with:
- `userMsg *string` - Pointer to user message
- `systemPrompt string` - System prompt for AI
- `personality string` - Personality configuration
- **No return value** - This needs to be changed to return `[]*core.Action`

### Action System (Existing Infrastructure)

**Location**: `/media/dlamprecht/Data/goProjects/bob/internal/orchestrator/core/action.go`

#### Action Structure (lines 5-17):
```go
type Action struct {
    ActionType     ActionType
    SourceWorkflow string
    AsyncGroupID   string
    AsyncGroupSize int
    Input          map[InputType]any
    AsyncActions   []*Action
}
```

#### Relevant Action Types (lines 19-29):
```go
const (
    ActionWorkflow       = iota
    ActionWorkflowResult
    ActionAi            // <-- This is the action type for AI requests
    ActionTool
    ActionUserMessage
    ActionUserWait
    ActionAsync
)
```

#### Current Input Types (lines 31-37):
```go
const (
    InputStep    = "step"
    InputMessage = "message"
)
```

**Additional input types needed for AI actions:**
- `InputSystemPrompt` - System prompt for AI
- `InputPersonality` - Personality/instructions
- `InputSchema` - Schema builder for structured output
- `InputConversationID` - For conversation continuity

### Action Processing (Empty Case)

**Location**: `/media/dlamprecht/Data/goProjects/bob/internal/orchestrator/core/action.go:50-89`

```go
func (a *Action) ProcessAction(context *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
    switch a.ActionType{
    case ActionWorkflow:
    case ActionWorkflowResult:
    case ActionAi:       // <-- EMPTY: Needs implementation
    case ActionTool:
    // ... other cases
    }
    return nil, nil
}
```

The `ActionAi` case at line 54 is empty and needs to:
1. Extract input data from the action
2. Call the AI layer's `SendMessage()` method
3. Process the response
4. Return follow-up actions (e.g., `ActionWorkflowResult` to deliver results back)

### AI Layer (Complete Implementation)

#### Provider Interface

**Location**: `/media/dlamprecht/Data/goProjects/bob/internal/ai/provider.go:7-19`

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

#### OpenAI Implementation

**Location**: `/media/dlamprecht/Data/goProjects/bob/internal/ai/openai/responses.go:19-60`

The `SendMessage` function:
1. Applies default config and custom options
2. Builds and caches JSON schema from SchemaBuilder
3. Resolves conversation ID (creates new if nil)
4. Sends with retry logic (3 attempts, exponential backoff)
5. Parses response to map
6. Returns `ai.Response` with data, conversation ID, tokens used, etc.

#### Schema Builder

**Location**: `/media/dlamprecht/Data/goProjects/bob/internal/ai/schema_builder.go`

Fluent API for defining structured output:
```go
schema := ai.NewSchema().
    AddString("title", ai.Required()).
    AddInt("priority", ai.Range(1, 5)).
    AddBool("urgent", ai.Description("Is this urgent?"))
```

#### Response Access

**Location**: `/media/dlamprecht/Data/goProjects/bob/internal/ai/schema_response.go`

Type-safe access to AI response data:
```go
data := response.Data()
title := data.MustGetString("title")
priority := data.MustGetInt("priority")
```

### Conversation Context Integration

**Location**: `/media/dlamprecht/Data/goProjects/bob/internal/orchestrator/core/context.go`

The `ConversationContext` has an `aiConverstation` field (line 21) for storing the **main workflow AI conversation**:

```go
type ConversationContext struct {
    // ...
    aiConverstation  *AIConversation  // <-- Main workflow AI conversation
    // ...
}
```

This field should hold the primary conversation ID for the current workflow. Additional purpose-specific conversation IDs can be stored in `WorkflowData map[string]any` (in `WorkflowContext`) using keys like `"ai_conv_research"`, `"ai_conv_validation"`, etc.

### Existing Action Patterns

#### Action Creation Pattern (orchestrator.go:70-87):
```go
a := core.NewAction(core.ActionWorkflow)
a.Input[core.InputStep] = workflow.StepInit
actions = append(actions, a)
```

#### Input Extraction Pattern (workflow_funcs.go:5-16):
```go
func getInput(a *core.Action, i core.InputType) any {
    if a.Input == nil {
        return nil
    }
    inputVal, ok := a.Input[i]
    if !ok {
        return nil
    }
    return inputVal
}
```

## Code References

| File | Line | Description |
|------|------|-------------|
| `internal/workflow/workflow_funcs.go` | 18-20 | Empty `askAI` helper stub |
| `internal/orchestrator/core/action.go` | 5-17 | Action struct definition |
| `internal/orchestrator/core/action.go` | 54 | Empty `ActionAi` case |
| `internal/orchestrator/core/action.go` | 31-37 | InputType constants |
| `internal/ai/provider.go` | 7-19 | Provider interface |
| `internal/ai/openai/responses.go` | 19-60 | OpenAI SendMessage |
| `internal/ai/schema_builder.go` | 10-92 | SchemaBuilder |
| `internal/ai/schema_response.go` | 5-157 | SchemaData response wrapper |
| `internal/orchestrator/core/context.go` | 21 | aiConversation field |

## Architecture Documentation

### Current Data Flow

```
User Message
    ↓
HandleUserMessage (orchestrator.go:20)
    ↓
AnalyzeIntent → ProcessUserIntent
    ↓
[ActionWorkflow created with step]
    ↓
StartHandlingActions (orchestrator.go:113)
    ↓
ProcessAction switch (action.go:50)
    ↓
ActionWorkflow case → RunWorkflow (workflow.go:55)
    ↓
WorkflowFn called → returns []*Action
    ↓
New actions added to queue
```

### Proposed Integration Flow

```
Workflow Function (e.g., CreateTicket)
    ↓
askAI(userMsg, systemPrompt, personality, schema)
    ↓
Creates ActionAi with Input data
    ↓
Returns action to orchestrator
    ↓
ProcessAction handles ActionAi case
    ↓
Calls ai.Provider.SendMessage()
    ↓
Response returned
    ↓
ActionWorkflowResult created with response data
    ↓
Workflow continues with AI response
```

### Key Integration Points

1. **Workflow → Orchestrator**: Via `ActionAi` actions in workflow function return value
2. **Orchestrator → AI Layer**: Via Provider interface in `ProcessAction()`
3. **AI Layer → OpenAI**: Via `openai.SendMessage()` with schema and options
4. **Response → Workflow**: Via `ActionWorkflowResult` or context storage

## Historical Context (from thoughts/)

The following design documents provide context for this integration:

- `thoughts/ai-context-and-schema-design.md` - Documents the async action solution with sub-agents design and flat action queue pattern
- `thoughts/orchestrator-action-design.md` - Core architectural decisions including workflow-driven design and action source tracking
- `thoughts/openai-module-implementation-plan.md` - Comprehensive plan for the OpenAI integration layer (4-phase approach)
- `thoughts/ai-layer-openai-interface-examples.md` - 4 concrete examples showing AI layer to OpenAI module communication
- `thoughts/implementation-tracker.md` - Lists AI system as missing core component

## Related Research

- `thoughts/shared/research/2026-01-01-database-layer-integration.md` - Research on conversation context and cache integration

## Design Decisions (Clarified)

### 1. AI Layer Translator

**Decision**: The AI layer needs an intermediary implementation that translates orchestrator data (context, etc.) to provider calls.

**Implementation**:
- Create an adapter/translator in the AI layer that sits between orchestrator and providers
- This translator receives context and other orchestrator-specific data
- Transforms this data into provider-specific calls (e.g., OpenAI's `SendMessage`)
- Default provider: OpenAI (set via init function)

**Location**: New file needed: `internal/ai/translator.go` or `internal/ai/orchestrator_adapter.go`

### 2. Synchronous vs Asynchronous Calls

**Decision**: AI calls can be both sync and async depending on usage.

**Rules**:
- **Synchronous (blocking)**: When `askAI` is the only action being called
  - Create single `ActionAi` action
  - Orchestrator processes it in the queue
  - Workflow waits for response before continuing

- **Asynchronous (non-blocking)**: When multiple AI calls or other operations run in parallel
  - Use `ActionAsync` pattern with `AsyncActions` array
  - Each AI call wrapped in `ActionAi` inside `AsyncActions`
  - Use `AsyncGroupID` to correlate results
  - Results delivered via `ActionWorkflowResult` actions

**Pattern**:
```go
// Sync (blocking)
actions := askAI(msg, prompt, personality, schema)
return actions  // Single ActionAi

// Async (non-blocking)
asyncAction := core.NewAction(core.ActionAsync)
asyncAction.AsyncGroupID = generateID()
asyncAction.AsyncActions = []*core.Action{
    askAI(msg1, prompt1, personality, schema1)[0],
    askAI(msg2, prompt2, personality, schema2)[0],
}
return []*core.Action{asyncAction}
```

### 3. Conversation ID Management

**Decision**: Workflows manage their own conversation IDs with flexible architecture.

**Structure**:
- Each workflow has **one main AI conversation ID**
  - Stored in: `ConversationContext.aiConversation` (current location)
  - **OR** potentially moved to: `WorkflowContext` as a dedicated field (not in WorkflowData)
  - Used for the primary workflow conversation thread
  - This is the conversation context, not stored in the generic `WorkflowData` map

- Each workflow can have **unlimited additional conversation IDs**
  - Stored in: `WorkflowData["ai_conv_<purpose>"]`
  - Managed by the workflow itself
  - Example purposes: "research", "validation", "side_query"

**Access Pattern**:
```go
// Get main conversation ID (from ConversationContext)
mainConvID := context.GetAIConversation()  // or context.aiConversation.ConversationID

// Get/set specific conversation (from WorkflowData)
researchConvID := context.GetCurrentWorkflow().WorkflowData["ai_conv_research"]

// askAI should accept conversation key
askAI(msg, prompt, personality, schema, "research")  // Uses WorkflowData["ai_conv_research"]
askAI(msg, prompt, personality, schema, "")          // Uses ConversationContext.aiConversation
```

**Note**: The main conversation lives at the `ConversationContext` level (currently in `aiConversation` field), while additional purpose-specific conversations are stored in `WorkflowData` for flexibility.

### 4. Provider Initialization

**Decision**: Default provider (OpenAI) initialized via init function at application startup.

**Implementation**:
- Create `ai.Init()` function called during application bootstrap
- Reads API key from environment/config
- Calls `openai.Connect(apiKey)`
- Sets default provider globally
- Located: `internal/ai/init.go`

### 5. Schema Passing

**Decision**: `askAI` accepts `*SchemaBuilder` parameter for structured output.

**Signature**:
```go
func askAI(
    userMsg *string,
    systemPrompt string,
    personality string,
    schema *ai.SchemaBuilder,
    conversationKey string,  // "" for main conversation
) []*core.Action
```

### 6. Error Handling

**Decision**: AI errors flow back to workflows via action result data.

**Pattern**:
- If sync: Error returned to orchestrator, sets `StatusError` on context
- If async: Error included in `ActionWorkflowResult` input data
- Workflow checks for error in result: `result.Input["error"]`
- Workflow decides how to handle (retry, fail, ask user, etc.)

## Implementation Checklist

Based on the design decisions above, here's what needs to be implemented:

### 1. AI Layer Components

- [ ] Create `internal/ai/translator.go` - Orchestrator-to-provider adapter
  - Takes `ConversationContext` and action input
  - Resolves conversation ID:
    - If `conversationKey` is empty → use `ConversationContext.aiConversation`
    - If `conversationKey` provided → use `WorkflowData["ai_conv_<key>"]`
  - Calls appropriate provider's `SendMessage()`
  - Returns `Response` or error
  - Stores new conversation ID back to appropriate location

- [ ] Create `internal/ai/init.go` - Provider initialization
  - `Init()` function to set up default provider
  - Read API key from config/env
  - Call `openai.Connect()`

### 2. Action System Extensions

- [ ] Add new `InputType` constants to `action.go`:
  - `InputUserMessage = "user_message"`
  - `InputSystemPrompt = "system_prompt"`
  - `InputPersonality = "personality"`
  - `InputSchema = "schema"`
  - `InputConversationKey = "conversation_key"`
  - `InputAsyncGroupID = "async_group_id"` (for result correlation)

- [ ] Implement `ActionAi` case in `action.go:ProcessAction()`:
  - Extract input data
  - Call AI layer translator
  - For sync: Block until response
  - For async: Return result via `ActionWorkflowResult`

### 3. Workflow Helper Functions

- [ ] Implement `askAI()` in `workflow_funcs.go`:
  - Accept parameters: userMsg, systemPrompt, personality, schema, conversationKey
  - Create `ActionAi` with all input data
  - Return `[]*core.Action`

- [ ] Add conversation ID helpers:
  - `getMainConversationID(context) *string` - Gets from `ConversationContext.aiConversation`
  - `getConversationID(context, key) *string` - Gets from `WorkflowData["ai_conv_<key>"]`
  - `setMainConversationID(context, id string)` - Sets in `ConversationContext.aiConversation`
  - `setConversationID(context, key, id string)` - Sets in `WorkflowData["ai_conv_<key>"]`

### 4. AsyncGroupID Management

- [ ] Add helper for async group creation
- [ ] Implement result correlation in orchestrator
- [ ] Update `ActionWorkflowResult` handling

## Updated Integration Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ Workflow Function (e.g., CreateTicket)                          │
│   - Calls askAI(msg, prompt, personality, schema, convKey)     │
│   - Returns []*Action with ActionAi                             │
└─────────────────────────┬───────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────────┐
│ Orchestrator: StartHandlingActions()                            │
│   - Processes action queue                                      │
│   - Encounters ActionAi                                         │
└─────────────────────────┬───────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────────┐
│ ActionAi.ProcessAction()                                        │
│   - Extracts: userMsg, systemPrompt, personality, schema, key  │
│   - Calls AI Layer Translator                                  │
└─────────────────────────┬───────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────────┐
│ AI Layer Translator (NEW)                                       │
│   - Resolves conversation ID:                                  │
│     * Empty key → ConversationContext.aiConversation           │
│     * Non-empty key → WorkflowData["ai_conv_<key>"]           │
│   - Calls provider.SendMessage()                               │
│   - Returns Response                                            │
└─────────────────────────┬───────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────────┐
│ OpenAI Provider                                                 │
│   - openai.SendMessage() - already implemented                 │
│   - Builds schema, calls API, returns Response                 │
└─────────────────────────┬───────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────────┐
│ Response Processing                                             │
│   SYNC: Store response data in context, continue workflow      │
│   ASYNC: Create ActionWorkflowResult, send to workflow         │
└─────────────────────────────────────────────────────────────────┘
```
