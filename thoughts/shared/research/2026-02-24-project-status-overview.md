---
date: 2026-02-24T00:00:00-06:00
researcher: Claude Sonnet 4.6
git_commit: 3f4a65374718b87b743b93b93ddbe159f7cd7122
branch: master
repository: bob
topic: "Project Status Overview — returning after a break"
tags: [research, codebase, status, orchestrator, tools, ado, workflow, slack, ai]
status: complete
last_updated: 2026-02-24
last_updated_by: Claude Sonnet 4.6
---

# Research: Project Status Overview

**Date**: 2026-02-24
**Researcher**: Claude Sonnet 4.6
**Git Commit**: `3f4a65374718b87b743b93b93ddbe159f7cd7122`
**Branch**: master
**Repository**: bob

## Research Question

Returning to the project after a break. What is the current state of the codebase? What has been implemented, what is in progress, and what remains? Checked all markdown documentation files, source files, and git history.

## Summary

Bob is a **Slack bot with AI-powered workflow orchestration**, written in Go — a rewrite of a prior Python v1 implementation. The project is substantially further along than the last recorded tracker estimate (~15% in early January 2026). As of the last commit (Feb 1, 2026), the core infrastructure is complete: the Slack layer, orchestrator action loop, AI layer (OpenAI Responses API), database layer, and ADO tool suite are all implemented and tested. The two "real" workflows (`createTicket`, `queryTicket`) remain as stubs. There are two untracked files not yet committed: a new ADO Get Ticket tool and a design doc for an RMS research sub-workflow.

---

## Detailed Findings

### Recent Git History (Last 3 Commits)

| Commit | Date | Description | Key Files Changed |
|---|---|---|---|
| `3f4a653` | Feb 1, 2026 | **ADO Tools** | Added `toolADOCreateTicket.go`, `toolADOGetMetadata.go`, `toolADOSearchTickets.go`, `tests/ado_tools_test.go`; removed `toolSampleData.go`; updated `tool.go` and `test_ai.go` |
| `d1ca2eb` | Jan 25, 2026 | **Implement Tool in a workflow** | `orchestrator.go`, `process_actions.go`, `action.go`, `test_ai.go`, `workflow.go` |
| `4e7f282` | Jan 25, 2026 | **Add Tool Infrastructure** | `tool.go`, `toolSampleData.go` (later removed), `action.go`, `process_actions.go` |

**Net additions in last 3 commits:** ~2,229 lines across 11 files.

### Untracked Files (Not Yet Committed)

1. **`internal/tool/toolADOGetTicket.go`** — Retrieves a single ADO work item by ID. The function (`ADOGetTicket`) and its `ADOGetTicketArgs` schema are defined, but the tool is **not registered** in `tool.go`'s `tools` map yet.
2. **`thoughts/rms-research-workflow-design.md`** — Detailed design doc for a future `rms_research` sub-workflow (see Architecture Documentation section).

---

## Layer-by-Layer Implementation Status

### ✅ Database Layer — Complete

- `internal/database/database.go` — MySQL connection pool (25 max open, 5 idle, 5min lifetime)
- `internal/database/migrations.go` — SHA-256 checksum-validated migration runner
- `internal/database/transaction.go` — `WithTransaction` and `WithTransactionAndIsolation` helpers
- `internal/database/id_resolver.go` — External platform ID → internal int ID (get-or-create upsert)
- `internal/database/context_repository.go` — Save/load `ConversationContext` to DB
- `internal/database/workflow_repository.go` — Save/load `WorkflowContext` + key-value data, with type-aware serialization
- **2 migrations applied:** `m0001_schema.sql` (7 tables), `m0002_add_main_conversation_id.sql` (adds `main_conversation_id` to `workflow_context`)
- **Docker:** Single `bob_db` MySQL 8.0 container on host port 3316

### ✅ Configuration — Complete

- `internal/config/config.go` — Loads all env vars; required: DB (5), Slack (3), OpenAI (1), ADO (3); optional with defaults: logging (2), session cache (3)
- `.env.dist` documents all variables

### ✅ Logger — Complete

- `internal/logger/` — 5 severity levels (DEBUG/INFO/WARN/ERROR/FATAL), thread-safe, package-level convenience functions, custom instance support

### ✅ AI Layer — Complete

- `internal/ai/provider.go` — `Provider` interface (`SendMessage`, `Connect`, `Close`)
- `internal/ai/schema_builder.go` — Fluent builder: `AddString/Int/Float/Bool/Array/Object` with option funcs (`Required`, `Enum`, `Range`, `MinLength`, `Pattern`, etc.)
- `internal/ai/schema_types.go` — `FieldDef`, `FieldType` enum, all option constructors
- `internal/ai/schema_response.go` — Type-safe `SchemaData` accessor (`GetString`, `MustGetInt`, `Has`, `IsSet`, `Raw`)
- `internal/ai/init.go` — Provider registry; `Init()` called at app startup
- `internal/ai/ai_client.go` — `AIClient` singleton bridging orchestrator to provider
- `internal/ai/openai/` — Full OpenAI Responses API (stateful) implementation:
  - `client.go` — Thread-safe client; personality tracking to avoid redundant system messages (saves ~50-100 tokens/message)
  - `responses.go` — `SendMessage` with 3-attempt exponential backoff (1s/2s/4s), structured JSON schema output via `text.format`
  - `schema_builder.go` — Converts `SchemaBuilder` → OpenAI JSON Schema; cached by `SchemaBuilder.ID()` (SHA256 hash)
  - `options.go` / `types.go` / `errors.go` — Standard functional options, defaults (`gpt-4o-mini`, temp 0.7, 4096 tokens), error type classification with retry detection
  - `provider.go` — Implements `ai.Provider`; auto-registers via `init()` triggered by blank import in `main.go`

### ✅ Orchestrator Core — Complete

- `internal/orchestrator/core/action.go` — `Action` struct; 8 `ActionType` constants; 13 `InputType` string constants
- `internal/orchestrator/core/context.go` — `ConversationContext` with `sync.RWMutex`; `LoadContext` (cache → DB → new); `UpdateDB`; `PopRemainingActions`
- `internal/orchestrator/core/cache.go` — In-memory map; 4-tier eviction (Idle → Error → WaitForUser → Running)
- `internal/orchestrator/core/intent.go` — `Intent` struct; 3 `IntentType` constants
- `internal/orchestrator/core/message.go` — `Message`, `PlatformRef`, `PlatformSlack` constant; `GetResolved()` for lazy ID resolution
- `internal/orchestrator/core/workflow_context.go` — `WorkflowContext` with `GetAIConversation`/`SetAIConversation` for main + named sub-conversations

### ✅ Orchestrator Dispatcher — Complete

- `internal/orchestrator/orchestrator.go`:
  - `HandleUserMessage` — full pipeline: LoadContext → AnalyzeIntent → ProcessUserIntent → RouteUserMessage → StartHandlingActions → UpdateDB
  - `ProcessUserIntent` — maps intent types to initial actions with correct `InputStep` values
  - `RouteUserMessage` — 3-case routing (WaitForUser, Running, Idle)
  - `StartHandlingActions` — action loop with async channel drain, safe-point injection of queued messages, status transitions
- `internal/orchestrator/intent_analyzer.go`:
  - `AnalyzeIntent` — calls AI with fresh conversation; applies confidence thresholds (0.6 new workflow, 0.8 change workflow)
  - `callIntentAI` — uses `buildIntentSchema()` and `buildIntentPrompt()` (includes workflow context, history, current message)
- `internal/orchestrator/process_actions.go`:
  - `ProcessAction` — dispatches all 8 action types
  - `ActionAI` — resolves conversation ID, calls AI, stores returned conversation ID back to workflow context
  - `ActionTool` — calls `tool.RunTool`, returns `ActionWorkflowResult`
  - `ActionAsync` — spawns one goroutine per sub-action, uses `atomic.AddInt32` for pending count
  - `ActionUserMessage` / `ActionUserWait` — call responder; UserWait sets `StatusWaitForUser`

### ✅ Slack Layer — Complete

- `internal/slack/slack.go` — Socket Mode initialization; `StartSlack` blocks
- `internal/slack/handler.go` — Event routing; DM-only filter (`ChannelType == "im"`, `BotID == ""`); bridges to `orchestrator.HandleUserMessage`
- `internal/slack/parser.go` — `ParseMessage` converts `MessageEvent` to `core.Message`; `parseSlackTS` converts Slack timestamps to `time.Time`

### ✅ Tool Infrastructure — Complete (3 registered tools)

- `internal/tool/tool.go` — `ToolDefinition` struct, registry map, `RunTool` dispatcher, `AvailableTools`, panic-on-init validation
- **Registered tools:**
  - `ado_create_ticket` → `ADOCreateTicket` — PATCH to ADO REST API; handles User Story, Technical Debt, Defect; auto-prepends `BobBot` tag
  - `ado_search_tickets` → `ADOSearchTickets` — Dynamic WIQL query builder; 14+ filter fields; max 200 results
  - `ado_get_metadata` → `ADOGetMetadata` — 7 metadata types: tags, area_paths, iteration_paths, states, work_item_types, team_members, severity_values
- **Defined but unregistered:** `ado_get_ticket` → `ADOGetTicket` (in `toolADOGetTicket.go`, untracked)

### ✅ Test Coverage — ADO Integration Test

- `tests/ado_tools_test.go` — `TestADOComprehensive`: 21 sub-tests exercising all ADO tools against a live ADO environment
  - Sub-tests 00-06: All 7 metadata types
  - Sub-tests 10-12: Create User Story, Technical Debt, Defect (all fields)
  - Sub-tests 20-22: Verify created items via search
  - Sub-tests 30-37: Search with 8 different filter combinations
  - Sub-test 99: Summary with cleanup instructions (items tagged `Bob-Test`)
  - Note: `emergent_defect` excluded from test (requires specific picklist values, not boolean)
  - Area path hardcoded: `"Enterprise\\Cloud Native RMS\\Essentials"`

### 🔶 Workflows — Partially Implemented

- `internal/workflow/workflow.go` — Registry, `RunWorkflow` dispatcher, `handleDefaultSteps`, `handleSideQuestion`, `GetAvailableWorkflowContext`; 3 default steps (`init`, `asking_question`, `answering_question`)
- `internal/workflow/workflow_funcs.go` — `getInput` helper, `askAI` helper (creates `ActionAI`)
- **Registered workflows:**
  - `testAI` — **Fully implemented**: demonstrates async parallel AI calls (two simultaneous `ActionAI` in `ActionAsync`), result aggregation by schema field inspection, `ActionUserWait` pattern
  - `createTicket` — **Stub**: returns `nil, nil`
  - `queryTicket` — **Stub**: returns `nil, nil`

### 🔶 Personalities — Defined but Unused

- `definitions/personalities/personality.go` — Registry with `GetPersonality` and `AvailablePersonalities`
- `definitions/personalities/intent_analyzer.go` — `personalityIntentAnalyzer` prompt
- **Note:** This package is not currently imported by any other package. The intent analyzer in `orchestrator/intent_analyzer.go` constructs its personality prompt inline rather than using this registry.

---

## Code References

- `main.go:15-54` — Application startup sequence
- `internal/orchestrator/orchestrator.go:23` — `HandleUserMessage` — main message pipeline
- `internal/orchestrator/orchestrator.go:146` — `StartHandlingActions` — the core action loop
- `internal/orchestrator/intent_analyzer.go:30` — `AnalyzeIntent` — confidence thresholds and routing logic
- `internal/orchestrator/process_actions.go:21` — `ProcessAction` — all 8 action type handlers
- `internal/workflow/workflow.go:66` — `RunWorkflow` — default step handling + workflow dispatch
- `internal/workflow/test_ai.go:16` — `TestAI` — the fully implemented reference workflow
- `internal/tool/tool.go:10-26` — Tool registry (3 registered; 1 defined but unregistered)
- `internal/tool/toolADOGetTicket.go` — **Untracked**, not registered
- `internal/ai/openai/responses.go:19` — `SendMessage` — OpenAI Responses API call with retry
- `internal/ai/openai/schema_builder.go:14` — Schema compilation and caching
- `internal/orchestrator/core/context.go:143` — `LoadContext` — cache → DB → new flow
- `internal/orchestrator/core/cache.go:58` — 4-tier eviction strategy
- `internal/database/workflow_repository.go:145` — `serializeValue` — type-aware DB serialization
- `tests/ado_tools_test.go:16` — `TestADOComprehensive` — 21-sub-test ADO integration suite

---

## Architecture Documentation

### Full Request Flow (Slack Message → AI → Response)

```
Slack Event (Socket Mode)
  → slack/handler.go:handleMessage
  → slack/parser.go:ParseMessage         → core.Message (external platform IDs)
  → orchestrator.go:HandleUserMessage
      → core/context.go:LoadContext
          → message.GetResolved()        → IDResolver: Slack ID → internal DB int ID
          → GetFromCache (hot)           → or loadContextFromDB (cold) → create new
      → intent_analyzer.go:AnalyzeIntent
          → ai.SendMessage (nil convID)  → fresh OpenAI conversation each time
          → confidence thresholds applied → returns core.Intent
      → ProcessUserIntent                → []*Action{ActionWorkflow{step: "init"}}
      → RouteUserMessage                 → start/queue/skip
      → StartHandlingActions             → action loop
          loop:
            ProcessAction(ActionWorkflow)   → RunWorkflow → workflowFn → []*Action
            ProcessAction(ActionAsync)      → goroutines → actionChan
            ProcessAction(ActionAi)         → ai.SendMessage (persistent convID)
                                            → stores convID in WorkflowContext
            ProcessAction(ActionTool)       → tool.RunTool → map[string]any
            ProcessAction(ActionWorkflowResult) → RunWorkflow (step: handle_async_results)
            ProcessAction(ActionUserMessage)  → responder → api.PostMessage
            ProcessAction(ActionUserWait)   → StatusWaitForUser → break
          → StatusIdle on clean exit
      → context.UpdateDB()               → database/context_repository.go
```

### Three-Tier State Storage

| Tier | Location | Contents | Persistence |
|---|---|---|---|
| Hot | `core/cache.go` in-memory map | Full `ConversationContext` + action queue | Process lifetime; evicted on capacity |
| Warm | `workflow_context` DB table | Workflow name, step, `main_conversation_id` | Survives restart |
| Cold | `workflow_context_data` DB table | Key-value workflow state (typed) | Survives restart |

### AI Conversation Scoping

- **Main conversation** (`key == nil`): Stored in `WorkflowContext.aiConverstation`; persisted in `workflow_context.main_conversation_id`
- **Named sub-conversations** (`key = "category_picker"`): Stored in `WorkflowData["ai_conv_<key>"]`; ephemeral (not persisted to dedicated DB column)
- **Intent analysis**: Always fresh (nil conversationID), never persisted

### ADO Integration Pattern

All ADO tools use Basic Auth `("", ADO_PAT)` against `{ADO_ORG_URL}/{ADO_PROJECT}/_apis/...` at version `7.1-preview.3`. Tools return `map[string]any` for uniform handling by `ActionTool` → `ActionWorkflowResult`.

---

## Historical Context (from thoughts/)

- `thoughts/implementation-tracker.md` — Last updated 2026-01-02; showed ~15% overall parity. The project has moved significantly since (orchestrator complete, Slack layer complete, AI layer complete, ADO tools added).
- `thoughts/nextSteps.md` — Decision log: one run per conversation, hot-only action queue, coalesce timer (0.8-1.5s), `/bobstop` interrupt, advisory intent. Several of these (coalescing, `/bobstop`, safe-point injection of mid-run messages) are not yet implemented.
- `thoughts/ai-context-and-schema-design.md` (2026-01-03) — Describes the three AI conversation scopes and the `SchemaBuilder` API. Both are now implemented as described.
- `thoughts/shared/research/2026-01-07-workflow-askAI-orchestrator-integration.md` — Design decisions for the `askAI` helper and `ActionAI` handler. These are now fully implemented.
- `thoughts/rms-research-workflow-design.md` — **Untracked design doc** for a future `rms_research` sub-workflow:
  - 3 new DB tables: `rms_knowledge_cache`, `rms_file_paths`, `rms_knowledge_updates`
  - 3 new git tools: `git_fetch_file`, `git_search_files`, `git_get_recent_changes`
  - Two-phase execution: fast path (< 2s) using cached AI conversation + parallel file fetch; background enrichment (< 30s)
  - Path tiering: always/conditional/discovered
  - Conversation lifecycle management (summarize at turn 25)
  - 5-week phased implementation plan

---

## Related Research

- `thoughts/shared/research/2026-01-01-database-layer-integration.md` — Full DB layer documentation (commit `065ab62`)
- `thoughts/shared/research/2026-01-07-workflow-askAI-orchestrator-integration.md` — AI/orchestrator integration decisions (commit `e34fc85`)

---

## Open Questions / Things to Pick Up Next

1. **`toolADOGetTicket.go` is untracked and unregistered** — needs to be registered in `tool.go`'s `tools` map and committed.
2. **`createTicket` and `queryTicket` workflows are stubs** — these are the two "real" production workflows, both return `nil, nil`.
3. **`definitions/personalities/` package is unused** — personality prompts are defined there but no code imports it. The intent analyzer constructs its prompt inline.
4. **Message coalescing** (0.8-1.5s buffer + merge) not yet implemented.
5. **`/bobstop` interrupt command** not yet implemented.
6. **`ai_conversations` DB table** exists in schema but is not written to; conversation IDs go to `workflow_context.main_conversation_id` and `workflow_context_data` instead.
7. **RMS research workflow** (`thoughts/rms-research-workflow-design.md`) is fully designed but not started.
8. **Cache TTL** — `SESSION_CACHE_TTL_SECONDS` is in config (default 28800s) but eviction only happens on capacity, not on TTL.
9. **go.mod diagnostic**: `github.com/openai/openai-go/v3` marked as indirect but should be direct (`go mod tidy` needed).
