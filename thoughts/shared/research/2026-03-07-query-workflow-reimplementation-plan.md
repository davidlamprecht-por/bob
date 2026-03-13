# Query Workflow Reimplementation Plan
> Source: `query-workflow-ai-implementation` branch analysis
> Date: 2026-03-07

This document lists every feature from the branch with a checklist for manual reimplementation.
Mark each as `[x]` (implement), `[-]` (reject/skip), or leave blank while deciding.

---

## Session Updates

### 2026-03-13 (GO-007: sub-workflow dispatch + async hardening)

All infrastructure checklist items are now implemented on master. See commit `641bb43`.

**What landed:**
- `InputWorkflowName` / `InputSubWorkerID` constants on `core/action.go`
- `RunWorkflow` dispatches to named workflow from action input (sub-workflow dispatch)
- `WorkflowDefinition.Internal` flag — hidden from `AvailableWorkflows()` and intent analyzer
- `workflows` map moved to `init()` (avoids initialization cycles)
- `runAsyncSubtask` mini action loop in goroutines — flattens nested `ActionAsync` sequentially
- `ActionTool` swallows errors instead of killing async sub-workers silently
- `ActionAI` / `ActionTool` copy all routing inputs (`InputStep`, `InputWorkflowName`, `InputSubWorkerID`) to result actions
- Filter stale `ActionCompleteAsync` from queue before `WaitForUser` pause
- `pendingAsyncCount` exit check changed to `<= 0` (handles negative counts after pause)
- `StepUserAnsweringQuestion` default now routes to `handleSideQuestion`; workflows needing raw user answer must set `OptionOverwriteHandleDefaultSteps`
- Intent analyzer falls back to `GetMainConversation()` when `lastResponseID` is nil
- Generic pointer serialization (`ptr_<innerType>`) in `workflow_repository` via reflect

**Next:** Implement `QueryTicket` and `QueryTicketSearcher` workflows.

---

### 2026-03-13 (GO-006 decision — intent routing ambiguity)

Decided **not** to implement clarifying questions in the intent analyzer on master.

The intent analyzer routes greedily to the best-guess workflow. Ambiguity is resolved by the
workflow itself, which has full domain context and will ask for any missing information as part
of normal execution. This keeps the intent layer simple and the main conversation clean.

A complete implementation of the clarifying question approach (persistent branch, synthesized
forwarded message, 3-turn hard break) exists on `feature/intent-clarification` and is ready
to merge if greedy routing proves insufficient.

**What landed on master from this session:**
- `Intent.Step string` — explicit step override, empty = use default for IntentType
- `Intent.NeedsUserInput bool` — controls whether `MessageToUser` triggers `ActionUserWait` (blocking) or `ActionUserMessage`
- `ProcessUserIntent` updated to use both fields

See `thoughts/shared/research/2026-03-13-intent-routing-ambiguity.md` for the full trade-off analysis.

---

### 2026-03-10 (GO-005 + small hardening)

**GO-005** — In `ActionAI` (`process_actions.go`), when starting a fresh sub-conversation
(`keyPtr != nil && conversationID == nil`), seed it with `ai.BranchFromResponse(*respID)` if
`lastResponseID` is available. Single-line functional change; storage unchanged.

**`SetMainConversation` guard** — Added nil-check so the `conv_*` ID can't be overwritten
once set. `lastResponseID` remains mutable as it's the branch seed, not the stable ID.

**Docstring fixes** — `BranchFromResponse` and `sendBranchedMessage` comments updated to
reflect that branches can be continued across multiple turns, not just used for one-shot queries.

### Redesign: Intent Analyzer Clarifying Questions (2026-03-10)

**The problem with the old design:**
The intent analyzer branches off `lastResponseID`, gets a response, then discards the branch tip.
If it asks a clarifying question, the user answers, and the next intent call branches off
`lastResponseID` again — a completely fresh branch. The AI has no memory of what it asked.

**The new design (leverages persistent conversation):**

The intent analyzer owns a persistent branch. When it asks a clarifying question it stores the
branch tip on the context. The next call continues from that tip — the AI already knows the full
thread history AND what it just asked. No reconstructed context, no workflow history needed.

**New field on `ConversationContext`:**
```go
pendingIntentResponseID *string  // branch tip stored while waiting for clarifying answer
```
Persisted to DB (new nullable column `pending_intent_response_id` on `conversation_context`).

**Flow — normal routing (no clarification needed):**
1. `callIntentAI()` branches off `lastResponseID` (as today) → routes → **discards** branch tip ✓

**Flow — clarifying question:**
1. `callIntentAI()` branches off `lastResponseID` → AI returns `clarifying_question` non-empty
2. Store returned `response.ResponseID` → `ctx.SetPendingIntentResponseID(&respID)`
3. Send question to user via `ActionUserWait` (blocking, sets `StatusWaitForUser`)
4. User answers → `HandleUserMessage` → `callIntentAI()`
5. This time: `pendingIntentResponseID` is set → use it as `previous_response_id` instead of `lastResponseID`
6. AI continues its own branch: sees the thread + what it asked + the user's answer → routes confidently
7. Clear `pendingIntentResponseID`, proceed with routing

**Changes needed:**
- `ConversationContext`: add `pendingIntentResponseID *string` + getter/setter
- DB: new nullable column `pending_intent_response_id` on `conversation_context` (migration m0004)
- `callIntentAI()`: if `pendingIntentResponseID` set → use it (continue branch); else → branch off `lastResponseID` as today
- Intent AI schema: add `clarifying_question string` field
- `AnalyzeIntent()`: if `clarifying_question` non-empty → store branch tip + return new `IntentClarifying` type
- `ProcessUserIntent()`: handle `IntentClarifying` → emit `ActionUserWait` with the question
- `RouteUserMessage()`: when resuming from `StatusWaitForUser` with a `pendingIntentResponseID` → don't treat as a normal workflow message, let intent re-analyze with the stored branch

**What stays the same:**
- `lastResponseID` is never touched by intent analyzer calls (unchanged)
- All existing routing logic for confident intents is unchanged
- Branch tip for clarification is stored separately from sub-workflow branches

---

## Feature Checklist

### Infrastructure / Core

- [x] **Workflows map moved to `init()`**
  Move `var workflows = map[...]{}` to a `func init()` block. Needed to avoid initialization
  cycles when workflow functions reference the map at call time (e.g. `handleSideQuestion(workflows[WorkflowQueryTicket])`).

- [x] **`WorkflowDefinition.Internal` flag**
  A bool on `WorkflowDefinition` that hides a workflow from `AvailableWorkflows()` and the
  intent analyzer. Required for any sub-workflow that should never be user-triggered.

- [x] **Sub-workflow dispatch via `InputWorkflowName` / `InputSubWorkerID`**
  `ActionWorkflow` can carry `InputWorkflowName` to override `GetCurrentWorkflow()` in
  `RunWorkflow`. Combined with `InputSubWorkerID` (a string), this lets a parent spawn named
  parallel workers running a separate `WorkflowDefinition`.

- [x] **Copy all routing inputs through `ActionWorkflowResult`**
  In `ActionAI` and `ActionTool`, copy `InputStep`, `InputWorkflowName`, and `InputSubWorkerID`
  from the source action onto the result action. Previously only `InputStep` was copied, which
  silently dropped sub-worker identity through the chain.

- [x] **`runAsyncSubtask` goroutine mini-loop**
  Goroutines spawned by `ActionAsync` run a local action loop instead of processing one action
  and returning. This lets sub-workers execute multi-step sequences (tool → AI → result)
  entirely in parallel. Actions they cannot handle (user messages, waits) are forwarded to the
  main loop via the channel. Nested `ActionAsync` inside a goroutine is flattened sequentially
  rather than spawning sub-goroutines.

- [x] **`ActionTool` error recovery in async context**
  On tool failure, return an empty result map (`{count:0, items:[], error:"..."}`) instead of
  propagating the error. A returned error from a goroutine silently kills it, leaving
  `pendingAsyncCount` undecrementable and the parent workflow waiting forever.

- [x] **Filter stale `ActionCompleteAsync` on `WaitForUser` pause**
  When pausing the action loop for user input, strip any `ActionCompleteAsync` from the
  remaining queue before storing it. Those decrement actions become invalid after the pause
  because `pendingAsyncCount` is not explicitly reset on resume.

- [x] **`OptionOverwriteHandleDefaultSteps` controls `StepUserAnsweringQuestion` handling**
  The new default behavior for `StepUserAnsweringQuestion` calls `handleSideQuestion` (answer
  and continue) instead of passing through to the workflow. Any workflow that calls
  `ActionUserWait` and needs the user's answer routed to its own handler must set
  `OptionOverwriteHandleDefaultSteps: true`.

- [x] **Generic pointer serialization in `workflow_repository`**
  `serializeValue` / `deserializeValue` handle pointer types via reflection using a
  `ptr_<innerType>` data type prefix. Motivated by storing `*string` values in workflow data.

---

### AI Layer

- [x] **`BranchFromResponse` AI option**
  A new `ai.Option` (`BranchOption{ResponseID string}`) that branches off an existing response.
  The model sees full conversation context. Store the returned tip to continue the branch across
  multiple turns, or discard it for one-shot queries. The original chain is unaffected.
  _Implemented in GO-003. Docstring clarified 2026-03-10 to reflect multi-turn support._

- [x] **`resp_` prefix detection in `SendMessage`**
  When `convID` starts with `resp_`, use `PreviousResponseID` instead of `Conversation` in
  the API params. The returned conversation ID becomes `resp.ID` (the new tip) rather than the
  original `convID`. `Conversation` and `PreviousResponseID` are mutually exclusive in the API.
  _Implemented in GO-003._

- [x] **`last_response_id` on `ConversationContext`**
  Tracks the OpenAI response ID from the most recent main-thread AI call. Persisted to DB
  (migration m0003). Set by `ActionAI` for calls with no `conversationKey`. Used by the intent
  analyzer and sub-workflow seeding to branch-read context.
  _Implemented in GO-003. Lives on `ConversationContext`, not `WorkflowContext`._

---

### Intent Routing

- [-] **4-tier confidence thresholds**
Not needed - intend analyzer will use main conversation as history instead of manually handling history.
  Replace the 2-tier system (currently 0.6 new, 0.8 change) with:
  - `idle → new workflow`: 0.70
  - `idle → historical workflow`: 0.65 (lower bar to return to a known topic)
  - `active → historical workflow`: 0.82
  - `active → new workflow`: 0.90
  When returning to a historical workflow the intent AI also picks the starting step
  (`StepInit` for restart, `StepUserAsksQuestion` for follow-up) rather than always `StepInit`.

- [-] **Workflow history in intent prompt**
  Not needed — the intent analyzer already branches off `lastResponseID`, giving it full thread
  conversation history. The AI knows what was discussed without a separate history log.

- [-] **Clarifying question flow** _(not on master — see `feature/intent-clarification`)_
  Decided against: greedy routing + workflow-level disambiguation is simpler and keeps the
  intent layer clean. Full implementation preserved on `feature/intent-clarification`.
  See `thoughts/shared/research/2026-03-13-intent-routing-ambiguity.md`.

- [x] **`Intent.Step` field + `Intent.NeedsUserInput` flag**
  `Step string` added — `ProcessUserIntent` passes it to `ActionWorkflow` when non-empty.
  `NeedsUserInput bool` controls whether `MessageToUser` triggers `ActionUserWait` (blocking)
  or `ActionUserMessage`. _Implemented on master (2026-03-13)._

- [ ] **`StatusWaitForUser` overrides intent step**
  When the context is in `StatusWaitForUser` and the same workflow is suggested, force
  `StepUserAnsweringQuestion` regardless of what step the AI suggested. Prevents workflow-
  specific steps from being injected when the user is simply replying to a question.

- [-] **Returning from clarifying question changes workflow if needed** — not needed on master (no clarifying flow).

---

### Workflow History

- [-] **`WorkflowHistoryEntry` type** — Not needed. Persistent AI conversation makes this redundant.
- [-] **`recordWorkflowCompletion`** — Not needed. No history to record.
- [-] **DB migration: `workflow_history` column** — Not needed.

---

### `QueryTicket` Workflow

- [ ] **Init / Extract step**
  AI extracts: keywords, assigned_to, created_by, qa_person, work_item_type, state, tags,
  area_path, iteration_path from the user's message. Optionally sets `should_clarify=true`
  and a `clarifying_question` if it genuinely cannot search without more info.

- [ ] **Plan step**
  AI plans 1–5 parallel search strategies. Returns `worker_count` and `strategies_json`
  (a JSON array where each strategy has: worker_id, angle, title, assigned_to, created_by,
  qa_person, work_item_type, state, tags, area_path, iteration_path, max_results).

- [ ] **Spawn Workers step**
  Creates one `ActionAsync` containing N `ActionWorkflow` sub-actions, one per strategy.
  Each sub-action targets `WorkflowQueryTicketSearcher` with `InputWorkflowName` and carries
  the strategy as a JSON string in `InputMessage`. Stores `expectedWorkers` count before spawning.

- [ ] **Collect Result step**
  Increments a counter each time a sub-worker reports in. Proceeds only when all expected
  workers have reported. Accumulates all candidate sets in `allCandidatesJSON` for use in the
  exhausted-search fallback. Emits `ActionCompleteAsync` as the last action to release the
  async goroutine counter.

- [ ] **Analyze step**
  AI synthesizes all sub-worker candidates. Branches:
  - `present_ticket`: confident single match — fetch full ticket via `ado_get_ticket` tool
  - `show_candidates`: 2-3 plausible matches — show list and wait for user pick
  - `disambiguate`: targeted clarifying question
  - `narrow_down`: too many results, ask user to narrow
  - `refine`: no good matches — retry

- [ ] **Refine loop (attempt counter)**
  Attempts 1-2: auto-retry planning with new angles. Attempt 3: AI generates one specific
  clarifying question. Attempt 4+: present "keep trying / show best matches / give up" menu.
  Rejected ticket IDs are accumulated and excluded from all future search plans.

- [ ] **Present Ticket step**
  Fetches full ticket via `ado_get_ticket`, formats it for Slack, shows it, and waits.
  On follow-up, AI classifies intent: `answer_question` (answer from ticket fields),
  `wrong_ticket` (add to rejected IDs, re-search), `new_ticket_search` (reset and re-extract),
  or `done`.

- [ ] **`pendingStep` user-answer router**
  A workflow data key `qt_pending_step` records what state the workflow was in when it paused.
  `StepUserAnsweringQuestion` dispatches on this key to route the user's reply to the correct
  handler (post-clarify, post-candidates, post-disambiguate, post-exhausted, post-present).

- [ ] **`OptionOverwriteHandleDefaultSteps` set on `QueryTicket`**
  Required because QueryTicket manages its own `StepInit` reset and its own
  `StepUserAnsweringQuestion` routing via `pendingStep`.

- [ ] **Rejected IDs list**
  Stored as JSON in workflow data. Passed to both the planner AI and each searcher worker so
  confirmed-wrong tickets never appear in results or suggestions.

- [ ] **AI JSON backslash sanitization**
  ADO area paths contain backslashes (`Enterprise\RMS`). The AI sometimes produces invalid JSON
  escapes. A regex pre-pass before `json.Unmarshal` replaces `\X` (where X is not a valid JSON
  escape character) with `\\X`.

---

### `QueryTicketSearcher` Sub-Workflow

- [ ] **Search step**
  Unmarshals the instruction JSON from `InputMessage`, stores it in namespaced workflow data
  (`subWorkerKey(workerID, "instruction")`), then calls `ado_search_tickets` tool. The tool
  action carries `InputWorkflowName: QueryTicketSearcher` and `InputSubWorkerID` so the result
  routes back to this sub-workflow's evaluate step.

- [ ] **Evaluate step**
  AI evaluates each search result for relevance against the user's context. Returns
  `worker_id`, `found_any`, and `candidates_json` (array with id, title, confidence, reasoning,
  state, assigned_to, work_item_type, summary). Routes result back to parent's
  `StepQtCollectResult` by omitting `InputWorkflowName` (falls through to `GetCurrentWorkflow()`
  which is the parent).

- [ ] **`Internal: true` on registration**
  Must be hidden from the intent analyzer and user-facing workflow list.

---

### DB / Persistence

- [x] **`last_response_id` column on `conversation_context`**
  Nullable VARCHAR. Persisted and loaded alongside `main_conversation_id`. Used to feed
  `BranchFromResponse` in the intent analyzer and sub-workflow seeding.
  _Implemented in GO-003 (migration m0003)._

- [-] **`workflow_history` column on `conversation_context`** — Not needed.

---

## Design Considerations

**Async result collection must be order-independent.**
Workers report back in whatever order they finish. The collect step counts arrivals and proceeds
only when the total matches the stored expected count. Never assume order.

**Sub-worker identity must survive the entire action chain.**
`InputWorkflowName` and `InputSubWorkerID` must be copied from every action a sub-worker emits
onto the resulting `ActionWorkflowResult`. If they are lost at any hop, the result routes
back to the wrong workflow or step.

**Results route back to the parent by omitting `InputWorkflowName`.**
The searcher's final AI call omits `InputWorkflowName`, so `RunWorkflow` falls through to
`GetCurrentWorkflow()`, which is still the parent `QueryTicket`. This is the mechanism —
it is not incidental.

**`ActionCompleteAsync` must be emitted after the last real action, not before.**
If a goroutine emits it early, `pendingAsyncCount` can reach zero while work is still in
flight, causing the main loop to exit. In the searcher, `ActionCompleteAsync` is emitted
from the collect step (parent side), not from within the goroutine.

**`pendingAsyncCount` can go negative.**
If a `WaitForUser` pause strips `ActionCompleteAsync` from the queue but goroutines are still
running and later decrement the counter, the count goes negative. The `<= 0` exit check
handles it but the semantics are imprecise. Consider whether an explicit reset on resume is
cleaner.

**`pendingStep` and the step dispatch table must be kept in sync.**
The workflow has two dispatch tables: the top-level `switch step` and the `switch pendingStep`
inside `StepUserAnsweringQuestion`. Adding a new pause point requires a new `pendingStep`
value and a corresponding case in both places.

**`handleSideQuestion` and `ActionUserWait` answers are different things.**
A side question is answered without pausing the workflow. `ActionUserWait` is a deliberate
pause where the workflow needs the user's specific reply. Workflows using `ActionUserWait`
must handle `StepUserAnsweringQuestion` themselves — the default behavior treats it as a side
question.

**The intent analyzer must not see internal workflows.**
If `QueryTicketSearcher` appeared in routing options, the AI could try to route user messages
directly to it. Filter via `Internal` flag in `AvailableWorkflows()`.

**`BranchFromResponse` gives full context access, and the branch can be continued.**
Store the returned response ID to continue the branch across multiple turns (e.g. sub-workflow
conversations). Discard it only for one-shot queries (e.g. intent routing). The original
conversation chain is always unaffected regardless of what you do with the returned ID.

**`recordWorkflowCompletion` blocks before marking idle.**
The summary AI call runs synchronously before `SetCurrentStatus(StatusIdle)`. If it hangs,
the context is never marked idle. Use a short timeout context rather than `context.Background()`.

**`pendingClarification` is in-memory only.**
The flag prevents a second clarifying question while waiting for the first answer, but it is
not persisted to DB. A bot restart between asking and receiving the clarifying answer resets
it to false. Decide whether this gap matters for your reliability requirements.

**AI-generated JSON from the plan step needs validation and sanitization.**
`strategies_json` is a JSON string embedded inside a JSON response. Validate structure,
clamp `worker_count` to a safe range (1–5), and sanitize backslashes before unmarshaling.
Do not trust the AI to emit valid JSON for ADO paths.
