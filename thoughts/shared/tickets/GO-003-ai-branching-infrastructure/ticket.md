---
ticket_id: GO-003
title: Port AI Response Branching Infrastructure
type: Story
state: New
priority: High
created_date: 2026-03-07
depends_on: []
blocked_by: []
---

# GO-003: Port AI Response Branching Infrastructure

## Description

Port the branching infrastructure from the `query-workflow-ai-implementation` worktree into master.
This is the foundational primitive that enables the intent analyzer (and later sub-workflows) to read
full conversation history without writing to it. Purely an `internal/ai/` layer change — no orchestrator
or DB changes in this ticket.

## Background

The `ai_progress` worktree (`/media/dlamprecht/Data/goProjects/bob-ai_progress`) already implemented this pattern.
The key insight: OpenAI's Responses API supports `previous_response_id` to continue from any prior response.
By using this instead of the `conversation_id`, the model gets full history context but the returned
`resp_xxx` ID is a new branch tip — the original conversation chain is completely unaffected.

The worktree uses a `BranchFromResponse(responseID string)` option that the OpenAI provider intercepts
and routes to `sendBranchedMessage`, which uses `previous_response_id` instead of `conversation_id`.
The response ID returned from a branch call should be discarded by the caller (or stored separately).

## Goal

After this ticket:
- `ai.BranchFromResponse(responseID)` exists and the OpenAI provider handles it
- `SendMessage` correctly handles `resp_xxx`-prefixed IDs using the `previous_response_id` chain
- Callers can pass this option to get full conversation context for one-shot reads

## Tasks

- [ ] **`internal/ai/provider.go`** — Add `BranchOption` struct and `BranchFromResponse` constructor:
  ```go
  type BranchOption struct {
      ResponseID string
  }
  func (BranchOption) Apply(_ any) {} // handled via type assertion in providers
  func BranchFromResponse(responseID string) Option {
      return BranchOption{ResponseID: responseID}
  }
  ```

- [ ] **`internal/ai/openai/responses.go`** — Two changes:
  1. Add `sendBranchedMessage` (uses `previous_response_id`, returns `resp.ID` as ConversationID)
  2. In `SendMessage`: detect `resp_xxx`-prefixed `convID` and use `params.PreviousResponseID` instead of `params.Conversation`; update `returnConvID` to be `resp.ID` when using response-ID chain

- [ ] **`internal/ai/openai/provider.go`** — Update `SendMessage` method to type-assert for `BranchOption` before calling `SendMessage`:
  ```go
  for _, opt := range opts {
      if b, ok := opt.(ai.BranchOption); ok {
          return sendBranchedMessage(ctx, b.ResponseID, userPrompt, personality, schemaBuilder)
      }
  }
  return SendMessage(ctx, conversationID, userPrompt, personality, schemaBuilder)
  ```

## Files Changed

| File | Change |
|---|---|
| `internal/ai/provider.go` | Add `BranchOption`, `BranchFromResponse` |
| `internal/ai/openai/responses.go` | Add `sendBranchedMessage`, `previous_response_id` chain support, `returnConvID` fix |
| `internal/ai/openai/provider.go` | Type-assert `BranchOption` and route to `sendBranchedMessage` |

## No DB changes. No orchestrator changes.

## Reference Implementation

See `ai_progress` worktree:
- `internal/ai/provider.go` — `BranchOption` + `BranchFromResponse`
- `internal/ai/openai/responses.go` — `sendBranchedMessage` + `resp_xxx` chain logic (lines 139-148, 170-208)
- `internal/ai/openai/provider.go` — type assertion (lines 22-26)

## Acceptance Criteria

- [ ] `ai.BranchFromResponse("resp_xxx")` compiles and is exported
- [ ] Passing `BranchFromResponse` to `ai.SendMessage` routes through `sendBranchedMessage`
- [ ] Normal `SendMessage` with `resp_xxx` conversationID uses `previous_response_id` chain correctly
- [ ] `go build ./...` passes with no errors
