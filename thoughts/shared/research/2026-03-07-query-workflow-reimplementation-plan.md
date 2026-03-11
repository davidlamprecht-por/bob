# Query Workflow Reimplementation Plan
> Source: `query-workflow-ai-implementation` branch analysis
> Date: 2026-03-07

This document lists every feature from the branch with a checklist for manual reimplementation.
Mark each as `[x]` (implement), `[-]` (reject/skip), or leave blank while deciding.

---

## Feature Checklist

### Infrastructure / Core

- [ ] **Workflows map moved to `init()`**
  Move `var workflows = map[...]{}` to a `func init()` block. Needed to avoid initialization
  cycles when workflow functions reference the map at call time (e.g. `handleSideQuestion(workflows[WorkflowQueryTicket])`).

- [ ] **`WorkflowDefinition.Internal` flag**
  A bool on `WorkflowDefinition` that hides a workflow from `AvailableWorkflows()` and the
  intent analyzer. Required for any sub-workflow that should never be user-triggered.

- [ ] **Sub-workflow dispatch via `InputWorkflowName` / `InputSubWorkerID`**
  `ActionWorkflow` can carry `InputWorkflowName` to override `GetCurrentWorkflow()` in
  `RunWorkflow`. Combined with `InputSubWorkerID` (a string), this lets a parent spawn named
  parallel workers running a separate `WorkflowDefinition`.

- [ ] **Copy all routing inputs through `ActionWorkflowResult`**
  In `ActionAI` and `ActionTool`, copy `InputStep`, `InputWorkflowName`, and `InputSubWorkerID`
  from the source action onto the result action. Previously only `InputStep` was copied, which
  silently dropped sub-worker identity through the chain.

- [ ] **`runAsyncSubtask` goroutine mini-loop**
  Goroutines spawned by `ActionAsync` run a local action loop instead of processing one action
  and returning. This lets sub-workers execute multi-step sequences (tool → AI → result)
  entirely in parallel. Actions they cannot handle (user messages, waits) are forwarded to the
  main loop via the channel. Nested `ActionAsync` inside a goroutine is flattened sequentially
  rather than spawning sub-goroutines.

- [ ] **`ActionTool` error recovery in async context**
  On tool failure, return an empty result map (`{count:0, items:[], error:"..."}`) instead of
  propagating the error. A returned error from a goroutine silently kills it, leaving
  `pendingAsyncCount` undecrementable and the parent workflow waiting forever.

- [ ] **Filter stale `ActionCompleteAsync` on `WaitForUser` pause**
  When pausing the action loop for user input, strip any `ActionCompleteAsync` from the
  remaining queue before storing it. Those decrement actions become invalid after the pause
  because `pendingAsyncCount` is not explicitly reset on resume.

- [ ] **`OptionOverwriteHandleDefaultSteps` controls `StepUserAnsweringQuestion` handling**
  The new default behavior for `StepUserAnsweringQuestion` calls `handleSideQuestion` (answer
  and continue) instead of passing through to the workflow. Any workflow that calls
  `ActionUserWait` and needs the user's answer routed to its own handler must set
  `OptionOverwriteHandleDefaultSteps: true`.

- [ ] **Generic pointer serialization in `workflow_repository`**
  `serializeValue` / `deserializeValue` handle pointer types via reflection using a
  `ptr_<innerType>` data type prefix. Motivated by storing `*string` values in workflow data.

---

### AI Layer

- [ ] **`BranchFromResponse` AI option**
  A new `ai.Option` (`BranchOption{ResponseID string}`) that makes a one-shot AI call using
  `previous_response_id`. The model sees full conversation context without the call being
  added to the live conversation chain. The returned response ID should be discarded by callers
  that only want read access to the context.

- [ ] **`resp_` prefix detection in `SendMessage`**
  When `convID` starts with `resp_`, use `PreviousResponseID` instead of `Conversation` in
  the API params. The returned conversation ID becomes `resp.ID` (the new tip) rather than the
  original `convID`. `Conversation` and `PreviousResponseID` are mutually exclusive in the API.

- [ ] **`last_response_id` on `WorkflowContext`**
  Track the OpenAI response ID from the most recent main-thread AI call. Persisted to DB.
  Set by `ActionAI` for calls with no `conversationKey` (i.e. the main thread). Used by the
  intent analyzer to branch-read context for routing ambiguous follow-ups.

---

### Intent Routing

- [ ] **4-tier confidence thresholds**
  Replace the 2-tier system with:
  - `idle → new workflow`: 0.70
  - `idle → historical workflow`: 0.65 (lower bar to return to a known topic)
  - `active → historical workflow`: 0.82
  - `active → new workflow`: 0.90
  When returning to a historical workflow the intent AI also picks the starting step
  (`StepInit` for restart, `StepUserAsksQuestion` for follow-up) rather than always `StepInit`.

- [ ] **Workflow history in intent prompt**
  Append the last N completed workflows (name, time ago, summary) to the intent prompt so the
  AI can route follow-up messages back to completed topics at a lower confidence threshold.

- [ ] **Clarifying question flow**
  Replace `message_to_user` in the intent schema with `clarifying_question`. A non-empty value
  triggers `ActionUserWait` (blocking) instead of a non-blocking acknowledgment. A
  `pendingClarification` bool on `ConversationContext` prevents asking a second clarifying
  question before the user has answered the first.

- [ ] **`Intent.Step` field + `Intent.NeedsUserInput` flag**
  Add an explicit `Step string` to `Intent` so the intent analyzer can specify the exact step
  to start (rather than always using a default). Add `NeedsUserInput bool` so
  `ProcessUserIntent` knows whether `MessageToUser` should cause `ActionUserWait` (blocking)
  or just be sent as an acknowledgment.

- [ ] **`StatusWaitForUser` overrides intent step**
  When the context is in `StatusWaitForUser` and the same workflow is suggested, force
  `StepUserAnsweringQuestion` regardless of what step the AI suggested. Prevents workflow-
  specific steps from being injected when the user is simply replying to a question.

- [ ] **Returning from clarifying question changes workflow if needed**
  In `RouteUserMessage`, handle the case where the user's clarifying answer routes to a
  different workflow than the one currently set (or when there is no current workflow). Compare
  `intent.WorkflowName` to the current workflow and swap if different.

---

### Workflow History

- [ ] **`WorkflowHistoryEntry` type**
  Fields: `WorkflowName string`, `TriggerMessage string`, `Summary string`,
  `CompletedAt time.Time`. Stored as JSON in `conversation_context.workflow_history`.

- [ ] **`recordWorkflowCompletion`**
  Called synchronously at the end of `StartHandlingActions` before marking context idle.
  Makes a fresh (no conversation history) AI call to produce a 1-sentence summary of what
  the workflow did, then appends a `WorkflowHistoryEntry` to the context.

- [ ] **DB migration: `workflow_history` column**
  Add nullable TEXT column `workflow_history` to `conversation_context`. Serialized as JSON.

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

- [ ] **`last_response_id` column on `workflow_context`**
  Nullable VARCHAR. Persisted and loaded alongside `main_conversation_id`. Used to feed
  `BranchFromResponse` in the intent analyzer.

- [ ] **`workflow_history` column on `conversation_context`**
  Nullable TEXT, JSON-encoded `[]WorkflowHistoryEntry`. Loaded and unmarshaled on context load.
  Marshaled and saved in `UpdateDB`.

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

**`BranchFromResponse` gives read-only context access.**
If you store the returned response ID and use it as the next conversation pointer, you fork
the real conversation chain. Callers that only want context for routing should discard the
returned response ID.

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
