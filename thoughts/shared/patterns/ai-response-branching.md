# Pattern: AI Response Branching (Read-Only Context Fork)

## Problem

Some system-level queries (e.g. intent routing, moderation checks) need to see what the
current workflow AI conversation has been talking about, but must not pollute that
conversation thread. Injecting routing questions into the main thread adds noise that
affects all future workflow responses.

## Solution

The OpenAI Responses API stores each response as an immutable, server-side object.
Every response has a `previous_response_id` pointer back to its parent. **A parent can
have multiple children** — creating a new response off `resp_N` does not modify `resp_N`.

This means you can "branch" off the current tip of a conversation, make a query, and
then **discard the returned response ID**. The next real workflow call continues from the
original tip as if the branch never happened.

```
main thread:    A → B → C → D (current tip)
branch call:    A → B → C → D → [routing Q&A]   ← orphaned, discarded
next real turn: A → B → C → D → E               ← continues cleanly
```

The branched call sees the entire conversation history (tokens are billed) but nothing
is written back to the main thread.

## How to Use

### 1. The workflow must have made at least one AI call on the main conversation

`ActionAI` automatically stores the last response ID on the `WorkflowContext` after
every main-conversation AI call:

```go
// process_actions.go — happens automatically, no workflow code needed
if conversationKey == "" && response.ResponseID != "" {
    wf.SetLastResponseID(&response.ResponseID)
}
```

### 2. Pass `ai.BranchFromResponse` as an option to `ai.SendMessage`

```go
import "bob/internal/ai"

// Get the last response ID from the current workflow
lastRespID := ctx.GetCurrentWorkflow().GetLastResponseID()
if lastRespID == nil {
    // No previous AI turn yet — fall back to a fresh call (nil conversationID)
}

response, err := ai.SendMessage(
    ctx,
    nil,   // conversationID is ignored when BranchFromResponse is provided
    myPrompt,
    myPersonality,
    mySchema,
    ai.BranchFromResponse(*lastRespID), // ← the magic
)
// response.ConversationID will be "" — do NOT store it
// response.Data() contains your answer as normal
```

### 3. Optionally continue the branch

`response.ConversationID` is the new branch tip (`resp_xxx`). You can:

- **Discard it** — just don't store it. The branch is abandoned. Original thread untouched.
- **Continue it** — store it as a named conversation key and use it in future `ActionAI` calls.
  The AI layer detects the `resp_` prefix and automatically uses `previous_response_id`
  for all subsequent calls on that key:

```go
// Store the branch tip in the workflow
branchID := resp.ConversationID  // "resp_xxx..."
branchKey := "my_branch"
wf.SetAIConversation(&branchKey, &branchID)

// Later — continue on the branch via a normal ActionAI with that key
// The resp_ prefix is detected automatically; no extra options needed
a := workflow.NewAIAction(prompt, personality, schema, "my_branch")
```

Each continued call on the branch also returns the new `resp_` ID as `ConversationID`,
so `SetAIConversation` always holds the current tip automatically.

## When to Use This Pattern

| Use case | Approach |
|---|---|
| Intent/routing check that needs conversation context | Branch + discard |
| Moderation or safety check mid-workflow | Branch + discard |
| Parallel hypothesis exploration (pick the better path) | Branch + discard both, continue winner |
| Sub-workflow that shares history but diverges freely | Branch + continue (store in named key) |
| Draft/review: drafter and reviewer see same context independently | Two branches off same response ID |
| Sub-workflow with fully isolated AI thread (no shared history) | `conversationKey` (Conversations API) |
| Normal workflow AI call | Regular `ActionAI`, no branching |
| Summary generation after workflow completes | Fresh `nil` conversationID is fine |

## Current Usage

- **`intent_analyzer.go` `callIntentAI`**: When there is an active workflow with a
  `lastResponseID`, the intent analyzer branches off it so it can resolve references
  like "how would you rank *each of them*?" correctly.

## Caveats

- **Token cost**: All ancestor tokens in the chain are billed on every branched call,
  same as a regular continuation. The branch adds ~1 extra call's worth of output tokens.
- **30-day retention**: Orphaned branch responses sit on OpenAI's servers for 30 days
  (the default `store` TTL) then are automatically deleted. No cleanup needed.
- **~10–20% intermittent lookup failures**: OpenAI's `previous_response_id` has a known
  reliability issue at high load. The `sendWithRetryBranched` function retries up to 3
  times. If all retries fail, the intent analyzer gracefully falls back to a fresh call
  (because `BranchFromResponse` is only added when `lastRespID != nil`; if the branch
  errors, `callIntentAI` returns the error and `AnalyzeIntent` falls back to
  `IntentAskQuestion`).
- **Conversation vs previous_response_id**: These are mutually exclusive in the OpenAI
  API. `sendMessageBranched` sets only `PreviousResponseID` and leaves `Conversation`
  unset.

## Implementation Files

| File | Role |
|---|---|
| `internal/ai/provider.go` | `BranchOption` type + `BranchFromResponse()` |
| `internal/ai/openai/provider.go` | Detects `BranchOption`, routes to `sendBranchedMessage` |
| `internal/ai/openai/responses.go` | `sendBranchedMessage`, `sendWithRetryBranched`, `sendMessageBranched` |
| `internal/orchestrator/core/workflow_context.go` | `lastResponseID` field, `GetLastResponseID()`, `SetLastResponseID()` |
| `internal/orchestrator/process_actions.go` | Stores `response.ResponseID` after main-conversation AI calls |
| `internal/orchestrator/intent_analyzer.go` | Uses the pattern for routing context |
