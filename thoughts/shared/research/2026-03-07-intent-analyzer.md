---
date: 2026-03-07T00:00:00-05:00
researcher: dlamprecht
git_commit: 3f4a65374718b87b743b93b93ddbe159f7cd7122
branch: master
repository: bob
topic: "How the intent analyzer works"
tags: [research, codebase, orchestrator, intent, ai, workflow]
status: complete
last_updated: 2026-03-07
last_updated_by: dlamprecht
---

# Research: How the Intent Analyzer Works

**Date**: 2026-03-07
**Researcher**: dlamprecht
**Git Commit**: 3f4a653
**Branch**: master
**Repository**: bob

## Research Question
Get a full picture of how the intent analyzer is currently working.

## Summary

The intent analyzer is a stateless AI-backed decision gate that runs once per incoming user message. It determines:
1. Which workflow should handle the message
2. What step within that workflow to execute
3. A confidence score to gate the decision

It lives entirely in `internal/orchestrator/intent_analyzer.go` and is called from the orchestrator immediately after context is loaded, before any actions are dispatched.

---

## Detailed Findings

### 1. Entry Point & Call Site

`AnalyzeIntent(message *core.Message, ctx *core.ConversationContext) core.Intent`
(`internal/orchestrator/intent_analyzer.go:30`)

Called from `internal/orchestrator/orchestrator.go:35`:
```go
intent := AnalyzeIntent(message, context)
```

This is the first decision made after loading the `ConversationContext`. The result drives everything downstream.

---

### 2. AI Call — `callIntentAI`

(`intent_analyzer.go:122`)

Calls `ai.SendMessage` with:
- **Conversation ID**: `nil` — this is a **stateless** call. No conversation history is threaded through; each message is analyzed independently.
- **System prompt**: `"You are an intent analyzer for Bob, a workflow-based assistant. Analyze user messages to determine the appropriate workflow and step."`
- **User prompt**: built by `buildIntentPrompt`
- **Schema**: built by `buildIntentSchema` (structured output)

The AI response is parsed into an `aiIntentResponse` struct:
```go
type aiIntentResponse struct {
    WorkflowName  string
    Step          string
    Confidence    float64
    Reasoning     string
    MessageToUser *string
}
```

---

### 3. The Prompt — `buildIntentPrompt`

(`intent_analyzer.go:184`)

The prompt is assembled in sections:

**Section 1: Available Workflows**
Calls `workflow.GetAvailableWorkflowContext()` (`internal/workflow/workflow.go:246`), which generates:
- Default steps available in ALL workflows:
  - `init` — Initialize a new workflow, clears state
  - `asking_question` — User is asking a side/clarifying question
  - `answering_question` — User is responding to a workflow question
- Per-workflow entries (name, description, workflow-specific available steps):
  - `createTicket` — "Create, make, open, or submit a new Azure DevOps (ADO) work item/ticket..."
  - `queryTicket` — "Query, search, find, lookup, retrieve, view, or get an Azure DevOps (ADO) work item/ticket..."
  - `testAI` — "General AI conversation and testing. Use for general questions, testing, or when no other workflow matches..."

**Section 2: Current Context**
- If a workflow is active: `Active Workflow: <name>` and `Current Step: <step>`
- If none: `No active workflow`

**Section 3: Recent Message History**
- Pulls up to 3 of the user's previous messages (not the current one) from `ctx.GetLastUserMessages()`
- These are the in-memory `lastUserMessages` slice from `ConversationContext` — not persisted across restarts

**Section 4: User's Current Message**
The raw message text.

**Section 5: Workflow Switch Signals**
An explicit list of phrases the AI should treat as signals to switch workflows:
- "let's change the topic" / "switch topic"
- "switch to" / "move to" / "go to"
- "I want to / I need to [action matching different workflow]"
- "now I want to" / "instead, can you" / "let's do [something else]"

**Section 6: Instructions**
Asks the AI to determine: which workflow, which step, confidence (0.0–1.0).

If a workflow is currently active, additional disambiguation guidance is appended:
- Switch signals + strong match to another workflow → higher confidence in switching is natural
- Switch signal only (no strong match) → likely changing direction within current workflow
- Strong match only (no switch signal) → could be a related question, not a switch
- If current workflow can reasonably handle the request → prefer staying with lower confidence for switching

---

### 4. The Schema — `buildIntentSchema`

(`intent_analyzer.go:175`)

Uses the `SchemaBuilder` fluent API to define required structured output:

| Field | Type | Required | Notes |
|---|---|---|---|
| `workflow_name` | string | yes | Which workflow should handle the message |
| `step` | string | yes | Which step to execute |
| `confidence` | float | yes | 0.0–1.0, range-validated |
| `reasoning` | string | yes | Brief explanation |
| `message_to_user` | string | no | Optional message to send user |

---

### 5. Decision Logic in `AnalyzeIntent`

(`intent_analyzer.go:30`)

Two confidence thresholds are defined at the top of the file:
```go
const (
    confidenceThresholdNewWorkflow    = 0.6
    confidenceThresholdChangeWorkflow = 0.8
)
```

**Branch A: No active workflow** (`currentWorkflow == nil`)
- `confidence < 0.6` → `IntentAskQuestion` (includes `MessageToUser` if set)
- `confidence >= 0.6` → `IntentNewWorkflow` for `suggestedWorkflow`

**Branch B: Active workflow, AI suggests a DIFFERENT workflow** (`suggestedWorkflow != currentWorkflowName`)
- `confidence < 0.8` → `IntentAskQuestion` routed to the **current** workflow (stays put)
- `confidence >= 0.8` → `IntentNewWorkflow` for `suggestedWorkflow` (switches)

**Branch C: Active workflow, AI suggests SAME workflow**
- Calls `mapStepToIntentType(suggestedStep)`:
  - `"init"` → `IntentNewWorkflow`
  - `"asking_question"` → `IntentAskQuestion`
  - `"answering_question"` → `IntentAnswerQuestion`
  - anything else → `IntentAnswerQuestion` (default)

**Error case** (AI call fails):
- Returns `IntentAskQuestion` with `Confidence: 0.0` and error message in `Reasoning`

---

### 6. Intent Types

(`internal/orchestrator/core/intent.go:17`)

```go
const (
    IntentNewWorkflow    = "NewWorkflow"
    IntentAnswerQuestion = "AnswerQuestion"
    IntentAskQuestion    = "AskRelatedQuestion"
)
```

The `Intent` struct:
```go
type Intent struct {
    IntentType   IntentType
    WorkflowName string
    Confidence   float64
    Reasoning    string
    MessageToUser *string // Optional
}
```

---

### 7. Downstream: `ProcessUserIntent`

(`internal/orchestrator/orchestrator.go:100`)

Maps the `Intent` to an initial action queue:

| IntentType | ActionWorkflow InputStep |
|---|---|
| `IntentNewWorkflow` | `StepInit` ("init") |
| `IntentAnswerQuestion` | `StepUserAnsweringQuestion` ("answering_question") |
| `IntentAskQuestion` | `StepUserAsksQuestion` ("asking_question") |

If `intent.MessageToUser != nil && != ""`, also appends an `ActionUserMessage`.

---

### 8. Downstream: `RouteUserMessage`

(`internal/orchestrator/orchestrator.go:121`)

Decides whether to start (or resume) the action-handling loop:

| Context Status | Intent Type | Result |
|---|---|---|
| `StatusWaitForUser` | `IntentNewWorkflow` | Sets new workflow, sets status Running, starts loop |
| `StatusWaitForUser` | anything else | Sets status Running, starts loop (resumes) |
| `StatusRunning` | any | Appends actions to remaining queue, does NOT start new loop |
| idle/fresh | `IntentNewWorkflow` | Sets new workflow on context, starts loop |
| idle/fresh | anything else | Starts loop (workflow was already set or handles side question) |

If `RouteUserMessage` returns `false`, the orchestrator does not call `StartHandlingActions`. If `intent.MessageToUser` is set, it sends that message directly to the user and returns.

---

## Code References

- `internal/orchestrator/intent_analyzer.go:12` — Confidence threshold constants
- `internal/orchestrator/intent_analyzer.go:30` — `AnalyzeIntent` entry point
- `internal/orchestrator/intent_analyzer.go:101` — `mapStepToIntentType`
- `internal/orchestrator/intent_analyzer.go:122` — `callIntentAI` (stateless AI call)
- `internal/orchestrator/intent_analyzer.go:175` — `buildIntentSchema`
- `internal/orchestrator/intent_analyzer.go:184` — `buildIntentPrompt`
- `internal/orchestrator/orchestrator.go:35` — Call site
- `internal/orchestrator/orchestrator.go:100` — `ProcessUserIntent`
- `internal/orchestrator/orchestrator.go:121` — `RouteUserMessage`
- `internal/orchestrator/core/intent.go:6` — `Intent` struct and `IntentType` constants
- `internal/orchestrator/core/context.go:143` — `LoadContext` (context loaded before intent analysis)
- `internal/workflow/workflow.go:13` — Workflow registry and step constants
- `internal/workflow/workflow.go:246` — `GetAvailableWorkflowContext` (feeds the intent prompt)

---

## Architecture Documentation

**Stateless by design:** The intent AI never receives a conversation history (nil conv ID). This keeps intent analysis clean and focused — it only sees the current message, recent message history (in-memory, up to 3 prior messages), and workflow context.

**Two thresholds, three branches:** The 0.6 / 0.8 split means starting a workflow from idle requires moderate confidence, while switching an active workflow requires high confidence. This prevents accidental workflow changes mid-conversation.

**Default fallback:** When confidence is too low (either threshold), the intent always falls back to `IntentAskQuestion` routed to the current (or no) workflow. The `MessageToUser` field can carry a note to the user from the AI if something was unusual.

**Message history is in-memory only:** `lastUserMessages` on `ConversationContext` is not persisted to the DB. It accumulates during a session (cache hit) but is empty on a cold DB load. The intent prompt gets up to 3 previous messages for context (lines 199–205 of `intent_analyzer.go`).

**Workflow list is global:** `GetAvailableWorkflowContext()` iterates the `workflows` map in `workflow.go` and there is no filtering by user, permission, or conversation state. All registered workflows are visible to the intent AI on every call.

---

## Historical Context (from thoughts/)

- `thoughts/shared/research/2026-01-07-workflow-askAI-orchestrator-integration.md` — Historical decisions around AI/orchestrator integration
- `thoughts/shared/research/2026-02-24-project-status-overview.md` — Full project status reference

Note: Memory from prior sessions mentioned planned features (workflow history in the prompt, `pendingClarification` field on context, 4-tier thresholds at 0.65/0.70/0.82/0.90) that are **not present in the current code**. The implementation is simpler: two thresholds (0.6 / 0.8), no workflow history in the intent prompt, no `pendingClarification` tracking.

---

## Open Questions

- The `confidenceThresholdNewWorkflow` and `confidenceThresholdChangeWorkflow` constants have a `// TODO: Load these from config` comment — they are currently hardcoded.
- There is a `// TODO: Add more message history for better context` comment at `intent_analyzer.go:197` — currently only up to 3 prior messages are included and only from the in-memory slice.
- A full `// TODO: Future Enhancement - Intent Clarification Flow` block is documented in comments at `intent_analyzer.go:17–29` but not yet implemented.
