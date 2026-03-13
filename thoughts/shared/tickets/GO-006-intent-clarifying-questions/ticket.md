---
ticket_id: GO-006
title: Intent Analyzer Clarifying Questions with Persistent Branch
type: Story
state: New
priority: Medium
created_date: 2026-03-12
depends_on: [GO-005]
---

# GO-006: Intent Analyzer Clarifying Questions with Persistent Branch

## Description

When the intent analyzer AI is uncertain about routing, it should be able to ask the user a
clarifying question and then use the answer to route correctly. The key design requirement is
that when the user answers, the intent analyzer must continue its **own persistent branch** —
not start a fresh one — so the AI already knows the full thread context AND what it just asked.

Currently `callIntentAI` branches off `lastResponseID` on every call and discards the tip.
This is correct for normal routing. But for a clarifying question cycle it means the follow-up
call starts blind. The fix: store the branch tip when a clarifying question is asked, and
continue from it when the user answers.

## What Changes

### Flow — normal routing (unchanged)
1. `callIntentAI()` branches off `ctx.GetLastResponseID()` → routes → discards tip ✓

### Flow — clarifying question (new)
1. `callIntentAI()` branches off `ctx.GetLastResponseID()` → AI returns `clarifying_question` non-empty
2. `AnalyzeIntent()` stores `response.ResponseID` → `ctx.SetPendingIntentResponseID(&respID)`
3. Returns `Intent{IntentType: IntentClarifying, MessageToUser: &q, NeedsUserInput: true}`
4. `ProcessUserIntent()` handles `IntentClarifying` → emits `ActionUserWait` with the question
5. User answers → `HandleUserMessage` → `callIntentAI()`
6. `pendingIntentResponseID` is set → use it directly as the conversation pointer (not a new branch)
7. AI continues its own chain: sees the thread + what it asked + the user's answer → routes confidently
8. `AnalyzeIntent()` clears `pendingIntentResponseID` → normal routing proceeds

## Tasks

### 1. `core/context.go` — Add `pendingIntentResponseID` field

```go
pendingIntentResponseID *string
```

Add getter/setter following the same pattern as `lastResponseID`:

```go
func (c *ConversationContext) GetPendingIntentResponseID() *string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.pendingIntentResponseID
}

func (c *ConversationContext) SetPendingIntentResponseID(id *string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.pendingIntentResponseID = id
    c.lastUpdated = time.Now()
}
```

### 2. `core/intent.go` — Update `Intent` struct

Add two fields ported from the old branch:

```go
type Intent struct {
    IntentType     IntentType
    WorkflowName   string
    Step           string  // explicit step (empty = use default for IntentType)
    Confidence     float64
    Reasoning      string
    MessageToUser  *string
    NeedsUserInput bool    // true = ActionUserWait (blocking), false = ActionUserMessage
}
```

Add new intent type:

```go
const IntentClarifying IntentType = "Clarifying"
```

### 3. DB migration `m0004` — Add `pending_intent_response_id` column

```sql
ALTER TABLE conversation_context
    ADD COLUMN pending_intent_response_id VARCHAR(255) NULL;
```

Load and save it in `context_repository.go` alongside `last_response_id`.

### 4. `intent_analyzer.go` — `callIntentAI()` — Use stored branch if pending

```go
func callIntentAI(message *core.Message, ctx *core.ConversationContext) (*aiIntentResponse, error) {
    var opts []ai.Option
    var conversationID *string

    if pendingID := ctx.GetPendingIntentResponseID(); pendingID != nil {
        // Continue the intent analyzer's own branch — AI knows what it asked
        conversationID = pendingID
    } else if respID := ctx.GetLastResponseID(); respID != nil {
        // Fresh branch off thread context for normal routing
        opts = append(opts, ai.BranchFromResponse(*respID))
    }

    response, err := ai.SendMessage(ctx, conversationID, prompt, personality, schema, opts...)
    ...
    return &aiIntentResponse{..., ResponseID: response.ResponseID}, nil
}
```

Include `ResponseID` in the returned `aiIntentResponse` so `AnalyzeIntent` can store it.

### 5. `intent_analyzer.go` — `AnalyzeIntent()` — Handle clarifying question

At the top of `AnalyzeIntent()`, after getting the AI response:

```go
if aiResponse.ClarifyingQuestion != "" && ctx.GetPendingIntentResponseID() == nil {
    respID := aiResponse.ResponseID
    ctx.SetPendingIntentResponseID(&respID)
    q := aiResponse.ClarifyingQuestion
    return core.Intent{
        IntentType:     core.IntentClarifying,
        Confidence:     aiResponse.Confidence,
        Reasoning:      aiResponse.Reasoning,
        MessageToUser:  &q,
        NeedsUserInput: true,
    }
}
// Clear pending once we're routing normally (user answered)
ctx.SetPendingIntentResponseID(nil)
```

The guard (`pendingID == nil`) prevents a second clarifying question while one is already
pending — if the AI still returns a clarifying question on the follow-up call, we ignore it
and route with whatever confidence we have.

### 6. `intent_analyzer.go` — `buildIntentSchema()` — Add `clarifying_question` field

```go
AddString("clarifying_question", ai.Description(
    "If you cannot confidently pick a workflow, write a SHORT clarifying question for the user. "+
    "Leave empty if you can route confidently. Use sparingly — only when genuinely ambiguous."))
```

### 7. `orchestrator.go` — `ProcessUserIntent()` — Handle `IntentClarifying`

```go
case core.IntentClarifying:
    // Don't dispatch a workflow — just ask the user and wait
    if intent.MessageToUser != nil {
        a := core.NewAction(core.ActionUserWait)
        a.Input = map[core.InputType]any{
            core.InputMessage: *intent.MessageToUser,
        }
        actions = append(actions, a)
    }
    return actions
```

Also update the existing `MessageToUser` handling to respect `NeedsUserInput`:

```go
if intent.MessageToUser != nil && *intent.MessageToUser != "" {
    actionType := core.ActionUserMessage
    if intent.NeedsUserInput {
        actionType = core.ActionUserWait
    }
    a2 := core.NewAction(actionType)
    a2.Input[core.InputMessage] = *intent.MessageToUser
    actions = append(actions, a2)
}
```

### 8. `orchestrator.go` — `RouteUserMessage()` — Pass `Step` through

The `Intent.Step` field needs to reach the workflow. Update `ProcessUserIntent` to set
`InputStep` on the `ActionWorkflow` from `intent.Step` when non-empty:

```go
if intent.Step != "" {
    a.Input[core.InputStep] = intent.Step
} else {
    // existing default step logic based on IntentType
}
```

## Files Changed

| File | Change |
|---|---|
| `internal/orchestrator/core/context.go` | Add `pendingIntentResponseID` field + getter/setter |
| `internal/orchestrator/core/intent.go` | Add `Step`, `NeedsUserInput` to `Intent`; add `IntentClarifying` type |
| `definitions/migrations/m0004_intent_clarification.sql` | Add `pending_intent_response_id` column |
| `internal/db/context_repository.go` | Load/save `pending_intent_response_id` |
| `internal/orchestrator/intent_analyzer.go` | Branch continuation + clarifying question logic |
| `internal/orchestrator/orchestrator.go` | Handle `IntentClarifying` in `ProcessUserIntent`; pass `Intent.Step` |

## Important Notes

- `pendingIntentResponseID` uses the same `resp_*` chaining as sub-workflows. On the follow-up
  call, passing it as `conversationID` (not via `BranchFromResponse`) means `SendMessage` routes
  it through the `resp_*` prefix path in `responses.go` — `previous_response_id` is set
  automatically. No special handling needed in the AI layer.

- The double-clarification guard works by checking `GetPendingIntentResponseID() == nil` before
  storing. If the AI returns another `clarifying_question` on the follow-up, we clear the
  pending ID and route anyway. Worst case: low-confidence routing, which is acceptable.

- `Intent.Step` is only used for the workflow dispatch. For `IntentClarifying` the step is
  irrelevant since no workflow is dispatched — the bot just waits for the user's answer.

- `lastResponseID` is never written by intent analyzer calls. Only main-thread `ActionAI`
  calls update it (unchanged from GO-004).

## Acceptance Criteria

- [ ] When intent AI returns a non-empty `clarifying_question`, Bob asks it and waits (blocking)
- [ ] When the user answers, the intent AI continues from its stored branch tip (not a fresh branch)
- [ ] A second clarifying question is never asked while one is already pending
- [ ] After routing from a clarifying answer, `pendingIntentResponseID` is cleared
- [ ] Normal routing (no clarifying question) is completely unchanged
- [ ] `lastResponseID` is never modified by any intent analyzer call
- [ ] `go build ./...` passes
