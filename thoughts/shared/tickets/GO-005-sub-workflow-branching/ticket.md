---
ticket_id: GO-005
title: Sub-Workflows Branch from Thread Conversation
type: Story
state: New
priority: Medium
created_date: 2026-03-07
depends_on: [GO-003, GO-004]
blocked_by: [GO-004]
---

# GO-005: Sub-Workflows Branch from Thread Conversation

## Description

After GO-004, the main AI conversation is thread-level and only primary workflow calls write to it.
Sub-workflows (non-empty `conversationKey`) still start fresh — their first AI call gets no context
about the thread's conversation history.

This ticket makes sub-workflows **branch off the thread's latest response** on their first AI call.
The branch gives the sub-workflow full conversation context. The sub-workflow's responses advance its
own private branch, not the main thread. The main thread's `lastResponseID` is never touched by
sub-workflow calls.

## Example

User has been chatting with `testAI` about a project. They trigger a `createTicket` workflow which
internally spawns a sub-workflow worker to gather details. Currently the sub-worker AI starts completely
blind. After this ticket, it gets the full thread context by branching off the last response, so it
can be aware of what was being discussed.

## What Changes

### `ActionAI` in `process_actions.go` — Seed sub-conversations from thread context

Currently: when `conversationKey != ""` and `conversationID == nil` (first call on this sub-key),
calls `ai.SendMessage` with `nil` conversationID → creates a brand new conversation.

After: when `conversationKey != ""` and `conversationID == nil`, check `ctx.GetLastResponseID()`.
If available, pass `ai.BranchFromResponse(*respID)` as an option. The OpenAI provider handles this
by using `previous_response_id`, and returns a new `resp_xxx` ID as the branch tip.

The returned `response.ConversationID` (`resp_xxx`) gets stored in `wf.workflowData["ai_conv_KEY"]`
as before, and subsequent calls to the same sub-key continue that branch chain.

**The main thread's `lastResponseID` is NOT updated from sub-workflow calls.**
Only `conversationKey == ""` calls update the thread's `lastResponseID` (unchanged from GO-004).

## Tasks

### `process_actions.go` — `ActionAI` — Seed sub-conversation from thread

Locate the section in `ActionAI` that handles `keyPtr != nil` (sub-conversation):
```go
// Sub-conversation: branch from thread context on first call
var opts []ai.Option
if conversationID == nil {
    if respID := ctx.GetLastResponseID(); respID != nil {
        opts = append(opts, ai.BranchFromResponse(*respID))
    }
}
response, err := ai.SendMessage(goCtx, conversationID, userPrompt, personality, schema, opts...)
```

After the call, store the response ID as before (no change to storage logic):
```go
convID := response.ConversationID
wf.SetAIConversation(keyPtr, &convID)
// Do NOT update ctx.SetLastResponseID — sub-workflow does not advance the main thread
```

## Files Changed

| File | Change |
|---|---|
| `internal/orchestrator/process_actions.go` | `ActionAI`: when sub-conv starts fresh, seed with `BranchFromResponse` |

## Important Notes

- **Only one file changes** in this ticket. The rest of the plumbing was set up in GO-003 and GO-004.
- Sub-workflow conversations branch at the time of their first call. If the thread conversation has
  no `lastResponseID` yet (very first message), sub-workflows start fresh as before — no change in behavior.
- Subsequent calls within the same sub-workflow use `previous_response_id` chaining (handled automatically
  by the `resp_xxx` prefix detection in GO-003).
- The sub-workflow's branch exists independently on the OpenAI side. It doesn't need to be explicitly
  discarded — it naturally diverges from the thread and just takes up token storage.

## Acceptance Criteria

- [ ] A sub-workflow's first AI call receives full thread conversation history
- [ ] The sub-workflow's AI calls do NOT appear in the thread's main conversation chain
- [ ] `ctx.GetLastResponseID()` is unchanged after sub-workflow AI calls
- [ ] When no `lastResponseID` exists yet, sub-workflows start fresh (no errors)
- [ ] `go build ./...` passes
