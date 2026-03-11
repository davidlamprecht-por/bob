---
ticket_id: GO-004
title: Thread-Level AI Conversation + Context-Aware Intent Routing
type: Story
state: New
priority: High
created_date: 2026-03-07
depends_on: [GO-003]
blocked_by: [GO-003]
---

# GO-004: Thread-Level AI Conversation + Context-Aware Intent Routing

## Description

Implement the core philosophy change: **one persistent AI conversation per Slack thread**, shared across
all workflows. Currently the main conversation ID lives on `WorkflowContext` and is reset to nil every
time a new workflow starts (`StepInit` calls `SetAIConversation(nil, nil)`). This means:

1. The AI has no memory of what was discussed before (stateless between workflows)
2. The intent analyzer runs completely blind — no AI context at all
3. Every workflow start discards history the user might care about

After this ticket, the main AI conversation is owned by `ConversationContext` (the thread). Workflows
write to it permanently. When a workflow switches, the conversation continues where it left off. The
intent analyzer branches off the latest response to see full history, but never writes to the thread.

## What Changes

### New Philosophy
- `ConversationContext` owns: `mainConversationID` (the thread's persistent AI conversation) + `lastResponseID` (tip of the chain, used for branching)
- Workflows write to the thread's main conversation (permanent)
- Intent analyzer reads via branch (ephemeral — response discarded)
- Workflow switch does NOT reset the conversation

### What Stays the Same
- Isolated sub-conversations (non-empty `conversationKey`) remain stored in `WorkflowContext.workflowData` — unchanged
- Workflow-specific data (`ResetWorkflowData()` on StepInit) is still cleared on workflow switch

## Tasks

### 1. `ConversationContext` — Add thread-level conversation fields (`internal/orchestrator/core/context.go`)

Add two fields:
```go
mainConversationID *string // Persistent AI conversation for this thread
lastResponseID     *string // Most recent response ID in the main conversation (for branching)
```
Add getters/setters:
- `GetMainConversation() *string`
- `SetMainConversation(id *string)`
- `GetLastResponseID() *string`
- `SetLastResponseID(id *string)`

### 2. DB migration — Add columns to `conversation_context` (`definitions/migrations/m0003_thread_ai_conversation.sql`)

```sql
ALTER TABLE conversation_context
    ADD COLUMN main_conversation_id VARCHAR(255) NULL COMMENT 'Persistent OpenAI conversation ID for this thread',
    ADD COLUMN last_response_id VARCHAR(255) NULL COMMENT 'Most recent response ID in the main conversation';
```

Also remove `main_conversation_id` from `workflow_context` (now redundant for the primary conversation):
```sql
ALTER TABLE workflow_context DROP COLUMN main_conversation_id;
```

### 3. `database.Context` struct — Add new fields (`internal/database/context_repository.go`)

Add to `Context` struct:
```go
MainConversationID *string
LastResponseID     *string
```

### 4. `database.WorkflowContext` struct — Remove `MainConversationID` (`internal/database/workflow_repository.go`)

Remove `MainConversationID *string` field. Update INSERT/UPDATE/SELECT queries to drop the column.

### 5. `context_repository.go` — Persist and load new fields

Update `SaveContext` to write `main_conversation_id` and `last_response_id` to `conversation_context`.
Update `LoadContext` to read them back.

### 6. `loadContextFromDB` + `UpdateDB` in `core/context.go`

In `loadContextFromDB`:
```go
mainConversationID: dbContext.MainConversationID,
lastResponseID:     dbContext.LastResponseID,
```
In `UpdateDB`:
```go
var dbContext = &database.Context{
    ...
    MainConversationID: c.mainConversationID,
    LastResponseID:     c.lastResponseID,
}
```

### 7. `WorkflowContext` — Remove `aiConverstation` field (`core/workflow_context.go`)

- Remove `aiConverstation *string` field
- Remove the `key == nil` case from `GetAIConversation` (main conv now lives on ConversationContext)
- Remove the `key == nil` case from `SetAIConversation` (same)
- These methods now only handle named sub-conversations (non-nil key)
- Remove `GetLastResponseID` / `SetLastResponseID` if added in GO-003 (they move here to ConversationContext)

### 8. `ActionAI` — Write to thread conversation for main conv calls (`process_actions.go`)

Current logic reads/writes conversation via `wf.GetAIConversation(keyPtr)` for all keys.

New logic: when `keyPtr == nil` (main conversation), use `ctx` instead of `wf`:
```go
var conversationID *string
if keyPtr == nil {
    conversationID = ctx.GetMainConversation()
} else {
    conversationID = wf.GetAIConversation(keyPtr)
}
```

After the AI call, when storing the response:
```go
if keyPtr == nil {
    // Main conversation — write to thread-level context
    convID := response.ConversationID
    ctx.SetMainConversation(&convID)
    if response.ResponseID != "" {
        respID := response.ResponseID
        ctx.SetLastResponseID(&respID)
    }
} else {
    // Isolated sub-conversation — write to workflow data as before
    convID := response.ConversationID
    wf.SetAIConversation(keyPtr, &convID)
}
```

### 9. Intent analyzer — Branch off thread's last response (`intent_analyzer.go`)

In `callIntentAI`, use `ctx.GetLastResponseID()` (not `wf.GetLastResponseID()`):
```go
var opts []ai.Option
if respID := ctx.GetLastResponseID(); respID != nil {
    opts = append(opts, ai.BranchFromResponse(*respID))
}
response, err := ai.SendMessage(context.Background(), nil, prompt, systemPrompt, schema, opts...)
```
The returned `response.ConversationID` from a branch call is a `resp_xxx` ID — **do not save it** back to the context. It is ephemeral.

### 10. Remove conversation reset in `handleDefaultSteps` (`workflow/workflow.go`)

In `StepInit` branch, remove:
```go
c.GetCurrentWorkflow().SetAIConversation(nil, nil)
```
The workflow-specific data reset (`resetWorkflowData`) stays — only the AI conversation survives.

## Files Changed

| File | Change |
|---|---|
| `internal/orchestrator/core/context.go` | Add `mainConversationID`, `lastResponseID` fields + getters/setters |
| `internal/orchestrator/core/workflow_context.go` | Remove `aiConverstation` field; remove nil-key cases from Get/SetAIConversation |
| `internal/orchestrator/process_actions.go` | `ActionAI`: read/write main conv from `ctx`, not `wf` |
| `internal/orchestrator/intent_analyzer.go` | Use `ctx.GetLastResponseID()` for branching; discard returned ID |
| `internal/workflow/workflow.go` | Remove `SetAIConversation(nil, nil)` from StepInit handler |
| `internal/database/context_repository.go` | Add `MainConversationID`, `LastResponseID` to `Context` struct; update queries |
| `internal/database/workflow_repository.go` | Remove `MainConversationID` from `WorkflowContext` struct; update queries |
| `definitions/migrations/m0003_thread_ai_conversation.sql` | Add columns to `conversation_context`; drop from `workflow_context` |

## Important Notes

- The `handleSideQuestion` function in `workflow.go` calls `askAI(...)` with `conversationKey = ""`,
  which routes through `ActionAI` with `keyPtr == nil`. After this change it automatically uses
  the thread-level conversation — no separate change needed.
- The `WorkflowContext.aiConverstation` field was the only persisted AI conversation for the primary workflow.
  Isolated sub-conversations are stored in `workflowData` and are unaffected.
- A `resp_xxx` response ID returned by a branch call (intent analyzer) must NOT be stored on the context.
  Only responses from primary workflow AI calls (the ones that actually advance the thread) should update
  `mainConversationID` and `lastResponseID`.

## Acceptance Criteria

- [ ] Sending several messages to different workflows in one thread maintains one continuous AI conversation
- [ ] Switching from `testAI` to `createTicket` does not reset AI memory
- [ ] The intent analyzer's AI call does not create a new entry in the thread's conversation chain
- [ ] `main_conversation_id` and `last_response_id` are persisted to `conversation_context` table
- [ ] `go build ./...` passes
