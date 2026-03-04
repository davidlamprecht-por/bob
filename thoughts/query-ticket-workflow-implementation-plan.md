# QueryTicket Workflow — Implementation Plan

## Overview

The `queryTicket` workflow lets a user find an ADO ticket using natural language. It is
**stateful**: once a ticket is found, the workflow stays open and all follow-up questions are
answered from stored context without re-searching. Sub-workers are spawned dynamically — the
orchestrator AI decides how many and what each one searches.

---

## New Files

| File | Purpose |
|---|---|
| `internal/workflow/query_ticket.go` | Main workflow function (already exists as stub) |
| `internal/workflow/query_ticket_searcher.go` | Sub-worker workflow function |
| `definitions/personalities/query_ticket_orchestrator.go` | Orchestrator personality |
| `definitions/personalities/query_ticket_searcher.go` | Searcher sub-worker personality |

---

## Registry Changes

### `internal/workflow/workflow.go`

Add step constants to the `queryTicket` registration:
```go
WorkflowQueryTicket: {
    Description: "Query, search, find, lookup, retrieve, view, or get an Azure DevOps (ADO) work item/ticket...",
    WorkflowFn:  QueryTicket,
    AvailableSteps: []string{
        StepQtClarify,
        StepQtPlanSearch,
        StepQtSpawnWorkers,
        StepQtCollectResult,
        StepQtAnalyze,
        StepQtDisambiguate,
        StepQtShowCandidates,
        StepQtNarrowDown,
        StepQtRefine,
        StepQtExhausted,
        StepQtPresentTicket,
    },
},
WorkflowQueryTicketSearcher: {
    Description: "Internal searcher sub-worker for queryTicket.",
    Internal:    true,
    WorkflowFn:  QueryTicketSearcher,
    AvailableSteps: []string{StepQtswSearch},
},
```

Add workflow name constants:
```go
WorkflowQueryTicketSearcher WorkflowName = "queryTicketSearcher"
```

### `definitions/personalities/personality.go`

Add to const block:
```go
PersonalityQueryTicketOrchestrator PersonalityName = "query_ticket_orchestrator"
PersonalityQueryTicketSearcher     PersonalityName = "query_ticket_searcher"
```

Add to map:
```go
PersonalityQueryTicketOrchestrator: personalityQueryTicketOrchestrator,
PersonalityQueryTicketSearcher:     personalityQueryTicketSearcher,
```

### `internal/tool/tool.go`

Register the currently unregistered tool:
```go
ToolADOGetTicket: {
    Description:  "Retrieve a single Azure DevOps work item by ID with full details.",
    ToolFn:       ADOGetTicket,
    ArgsRequired: ADOGetTicketArgs,
},
```

---

## Step Constants

**`internal/workflow/query_ticket.go`:**
```go
const (
    StepQtClarify       = "qt_clarify"        // Optional light clarifying question before search
    StepQtPlanSearch    = "qt_plan_search"     // AI decides sub-worker count + strategies
    StepQtSpawnWorkers  = "qt_spawn_workers"   // Spawn the sub-workers based on AI plan
    StepQtCollectResult = "qt_collect_result"  // Collect one sub-worker result (called N times)
    StepQtAnalyze       = "qt_analyze"         // Synthesize all results, decide branch
    StepQtDisambiguate  = "qt_disambiguate"    // Ask a targeted question about specific candidates
    StepQtShowCandidates = "qt_show_candidates" // Present 2-3 best matches for user to pick
    StepQtNarrowDown    = "qt_narrow_down"     // Too many results — ask user to narrow
    StepQtRefine        = "qt_refine"          // No good results — try again or ask question
    StepQtExhausted     = "qt_exhausted"       // Give up gracefully, present options
    StepQtPresentTicket = "qt_present_ticket"  // STATEFUL HOME — show ticket + handle follow-ups
)
```

**`internal/workflow/query_ticket_searcher.go`:**
```go
const (
    StepQtswSearch = "qtsw_search" // Run the search, fetch details, score fit
)
```

---

## Workflow Data Keys

All stored in `workflow.SetWorkflowData(key, value)`:

| Key | Type | Purpose |
|---|---|---|
| `qt_attempt_count` | int | How many search rounds have been attempted |
| `qt_rejected_ids` | `[]int` | Ticket IDs the user explicitly rejected |
| `qt_expected_workers` | int | How many sub-workers were spawned this round |
| `qt_found_ticket` | `map[string]any` | The currently active ticket (full ado_get_ticket result) |
| `qt_all_candidates` | `[]map[string]any` | Best candidates collected across ALL rounds |
| `qt_extracted_info` | `map[string]any` | Structured info extracted from user message in init |
| `qt_last_disambiguation` | string | The last disambiguation question asked (avoid repeating) |
| `sw_{id}_result` | string (JSON) | Sub-worker result, namespaced via `subWorkerKey()` |

---

## AI Schemas

### 1. Init Extraction Schema (used in `StepInit`)
```go
ai.NewSchema().
    AddBool("should_clarify",
        ai.Description("True only if you genuinely cannot attempt a search without one more piece of info")).
    AddString("clarifying_question",
        ai.Description("One optional light question to ask before searching. Only set if should_clarify=true")).
    AddObject("extracted_info",
        ai.Description("Everything we know from the user message"),
        ai.NewSchema().
            AddArray("keywords", ai.FieldTypeString,
                ai.Description("Title keywords to search for")).
            AddString("assigned_to",
                ai.Description("Person the ticket is assigned to, if mentioned")).
            AddString("created_by",
                ai.Description("Person who created the ticket, if mentioned")).
            AddString("qa_person",
                ai.Description("QA person, if mentioned")).
            AddString("work_item_type",
                ai.Description("Story, Defect, Tech Debt, Task, Bug — if mentioned")).
            AddString("state",
                ai.Description("New, Active, Resolved, Closed — if mentioned")).
            AddArray("tags", ai.FieldTypeString,
                ai.Description("Tags if mentioned")).
            AddString("area_path",
                ai.Description("ADO area path e.g. Enterprise\\\\RMS — if user said which project")).
            AddString("iteration_path",
                ai.Description("Sprint or iteration path if mentioned")))
```

### 2. Plan Search Schema (used in `StepQtPlanSearch`)
```go
ai.NewSchema().
    AddInt("worker_count",
        ai.Required(),
        ai.Range(1, 5),
        ai.Description("How many sub-workers to spawn. 1 if query is very specific, up to 5 if many angles worth trying")).
    AddArray("strategies", ai.FieldTypeObject,
        ai.Required(),
        ai.Description("One strategy per worker. Each must use a different angle or keyword combo"),
        // Each item:
        // {
        //   "worker_id": string (unique e.g. "1", "2"),
        //   "angle_description": string (human-readable what this worker will try),
        //   "search_params": {
        //     "title": string,
        //     "assigned_to": string,
        //     "created_by": string,
        //     "qa_person": string,
        //     "work_item_type": string,
        //     "state": string,
        //     "tags": []string,
        //     "area_path": string,
        //     "iteration_path": string,
        //     "max_results": int (suggest 10-20)
        //   }
        // }
    )
```

> Note: Because nested object arrays with full schemas are complex with the current SchemaBuilder,
> consider serializing strategies as a JSON string field and having the searcher sub-worker parse it.
> Alternatively, expand SchemaBuilder to support nested object arrays. This is a known trade-off.

### 3. Sub-Worker Result Schema (returned by `StepQtswSearch`)
```go
ai.NewSchema().
    AddString("worker_id",
        ai.Required(),
        ai.Description("The worker ID assigned to this worker")).
    AddString("candidates_json",
        ai.Required(),
        ai.Description(`JSON array of candidates found. Each: {"id":int,"title":string,"confidence":float,"reasoning":string,"state":string,"assigned_to":string,"work_item_type":string,"summary":string}`)).
    AddBool("found_any",
        ai.Required(),
        ai.Description("True if at least one plausible candidate was found"))
```

### 4. Analyze/Synthesize Schema (used in `StepQtAnalyze`)
```go
ai.NewSchema().
    AddString("branch",
        ai.Required(),
        ai.Enum("present_ticket", "show_candidates", "disambiguate", "narrow_down", "refine"),
        ai.Description("Which path to take based on results")).
    AddInt("top_ticket_id",
        ai.Description("Set only when branch=present_ticket. The ID of the one confident match")).
    AddString("top_candidates_json",
        ai.Required(),
        ai.Description(`JSON array of top 2-3 candidates for show_candidates/disambiguate. Each: {"id":int,"title":string,"state":string,"assigned_to":string,"work_item_type":string,"summary":string}`)).
    AddString("message_to_user",
        ai.Required(),
        ai.Description("What to say to the user. For disambiguate: a targeted question. For narrow_down: explain too many results. For refine: explain why nothing was found. For show_candidates: intro text before the list."))
```

### 5. Follow-Up Decision Schema (used in `StepQtPresentTicket` for subsequent visits)
```go
ai.NewSchema().
    AddString("intent",
        ai.Required(),
        ai.Enum("answer_question", "wrong_ticket", "new_ticket_search", "done"),
        ai.Description("What the user wants: answer a question about this ticket, reject it, find a different ticket, or stop")).
    AddString("answer",
        ai.Description("Set when intent=answer_question. The answer to the user's question, using stored ticket data")).
    AddString("follow_up_prompt",
        ai.Description("Optional follow-up prompt to keep conversation going"))
```

### 6. Exhausted Options Schema (used in `StepQtExhausted`)
```go
ai.NewSchema().
    AddString("user_choice",
        ai.Required(),
        ai.Enum("keep_trying", "show_best", "give_up"),
        ai.Description("What the user wants to do after exhausting search attempts"))
```

---

## Personalities

### `personalityQueryTicketOrchestrator`

```go
var personalityQueryTicketOrchestrator = &Personality{
    Description: "Orchestrates ADO ticket search — extracts intent, plans sub-worker strategies, synthesizes results, handles follow-ups",
    PersonalityPrompt: `You are the search orchestrator for an Azure DevOps ticket finder integrated into Slack.

Your job has several distinct phases depending on what you are asked to do:

---

PHASE: EXTRACT & DECIDE (init)

Read the user's message and extract every useful piece of information:
- Title keywords (what the ticket is likely called or about)
- People: assigned_to, created_by, qa_person
- Work item type: Story, Defect, Tech Debt, Task, Bug
- State: New, Active, Resolved, Closed
- Tags, area path (e.g. Enterprise\RMS), sprint/iteration

Then decide: should you ask ONE light clarifying question before searching?
- ONLY ask if you genuinely cannot start searching without it
- If you have keywords, a person's name, or any meaningful detail — DO NOT ask, just search
- If the message is extremely vague (e.g. "find that ticket") with nothing to go on — ask ONE broad question like "Do you know any details about the ticket — title keywords, who it was assigned to, or which project it was in?"
- NEVER ask more than one question here, and NEVER ask for information you already have

---

PHASE: PLAN SEARCH STRATEGIES

You are deciding how many sub-workers to spawn (1-5) and what each one should search.

Rules:
- Each worker MUST try a different angle or keyword combination
- Do not have two workers try identical search params
- Think creatively: if the user said "payment timeout bug", consider:
  - Worker 1: title="payment timeout", type=Defect
  - Worker 2: title="gateway timeout", type=Defect
  - Worker 3: title="payment" + tags=["payments"], state=Active
- Workers can search the SAME area path but with different title terms
- If the user mentioned a specific person, include one worker focused on that person
- If area path is unknown, OMIT it from params (search all projects)
- Prefer 2-3 workers for specific queries, 4-5 for vague or broad ones
- Always include rejected_ticket_ids context so workers know what to exclude

---

PHASE: ANALYZE RESULTS

You receive ranked candidates from all sub-workers. Your job:
1. Merge and deduplicate (same ticket ID from multiple workers = one candidate, pick highest confidence)
2. NEVER include tickets whose ID is in the rejected list
3. Rank all candidates by confidence
4. Choose the correct branch:

   - 0 results → "refine"
   - 1 result with confidence ≥ 0.85 → "present_ticket" (confident match)
   - 1 result with confidence < 0.85 → "show_candidates" (not sure enough)
   - 2-5 results → decide between "disambiguate" and "show_candidates":
     - Use "disambiguate" ONLY if you can ask a very targeted, specific question that will clearly eliminate candidates
       Example: "Was this assigned to Sarah or John?" (when you have 2 candidates, one each)
       Example: "Was this from Sprint 42?" (when results split between sprints)
     - Use "show_candidates" if no single targeted question would clearly help, or if the user would benefit from just seeing the options
   - 6+ results → "narrow_down"

5. Write message_to_user clearly and naturally. This is sent directly to the user in Slack.
   - For disambiguate: make the question very specific and reference what you actually found
   - For show_candidates: brief intro like "I found a few tickets that might match:"
   - For narrow_down: be honest — "I found X results, can you tell me more about..."
   - For refine (on attempt 1-2): "I didn't find an exact match, let me try a different angle..."
   - For refine (on attempt 3+): "I'm having trouble finding it — do you know [specific helpful detail]?"

---

PHASE: FOLLOW-UP (stateful ticket context)

The user is asking about a ticket already found and stored. You have the full ticket data available.

Determine what the user wants:
- "answer_question" — they want to know something about this ticket (status, description, who's on it, etc.) → answer directly from the stored data, do not re-search
- "wrong_ticket" — they said this isn't the right one ("that's not it", "wrong ticket", "nope") → acknowledge and prepare to search again
- "new_ticket_search" — they want a completely different ticket ("find me the X ticket instead")
- "done" — they're finished ("thanks", "got it", "that's all")

For "answer_question": answer in a natural, conversational way using the ticket data. Be specific. If the answer isn't in the stored data, say so.

---

GENERAL RULES:
- You are integrated into Slack. Keep messages concise, friendly, and scannable.
- Never be condescending or over-explain.
- When presenting ticket info, use Slack formatting: *bold*, IDs as #1234, URLs as links.
- You understand ADO terminology: work items, sprints, area paths, iterations, stories, defects.
- Trust the sub-workers' confidence scores but always apply your own judgment.
- A ticket whose title contains a keyword but whose description clearly doesn't match the user's intent should be ranked LOW.`,
}
```

---

### `personalityQueryTicketSearcher`

```go
var personalityQueryTicketSearcher = &Personality{
    Description: "ADO ticket search sub-worker — runs one search angle, fetches full details, scores ticket fit",
    PersonalityPrompt: `You are a search specialist sub-worker for an Azure DevOps ticket finder.

You have been given:
1. The FULL conversation context — you know exactly what the user is looking for
2. A specific search strategy/angle to try (title keywords, person, tags, etc.)
3. A list of ticket IDs to EXCLUDE (already rejected by the user)

Your job is to:

STEP 1 — SEARCH
Run ado_search_tickets with your assigned parameters.
If your initial search returns 0 results:
- Try relaxing one constraint (e.g. remove state filter, or try a synonym of the title keyword)
- Try at most ONE retry before reporting 0 candidates

STEP 2 — EVALUATE FIT
For each search result, evaluate whether it genuinely matches what the user described.
This is the most important part of your job.

Do NOT just match keywords. Read the user's description and ask:
- Does the ticket's TITLE make sense given what the user described?
- Does the DESCRIPTION (if fetched) match the user's intent?
- Does the TYPE match? (If user said "bug" and this is a Story, be skeptical)
- Does the PERSON match? (If user mentioned someone and this is assigned to someone else, lower confidence)
- Does the TIMEFRAME make sense? (Check created_date/changed_date against any time hints)

Confidence scoring:
- 0.9-1.0: Near certain match — title, type, person, and description all align
- 0.7-0.89: Strong candidate — most signals match, minor uncertainty
- 0.5-0.69: Plausible — keyword match but uncertain fit
- 0.0-0.49: Weak match — surface keyword match only, description doesn't align

STEP 3 — FETCH DETAILS (for high-confidence candidates)
For any ticket with initial confidence ≥ 0.6, call ado_get_ticket to get full details.
This gives you the description, acceptance criteria, and other fields to better evaluate fit.
After reading the full details, adjust your confidence score accordingly.

EXCLUDE any ticket whose ID is in the rejected list — do not include them in candidates even if they match.

STEP 4 — RETURN
Return your worker_id, your candidate list as JSON, and whether you found anything.

Candidate JSON format:
[
  {
    "id": 1234,
    "title": "Payment Gateway Timeout",
    "confidence": 0.92,
    "reasoning": "Title matches 'payment timeout', it's a Defect (user said bug), assigned to Sarah (user mentioned Sarah), description describes retry logic which matches user context",
    "state": "Active",
    "assigned_to": "Sarah Johnson",
    "work_item_type": "Defect",
    "summary": "Handles retry logic when Stripe returns 504 during checkout"
  }
]

RULES:
- Quality over quantity. Return 1-3 well-evaluated candidates rather than 10 weak ones.
- Always explain your reasoning. The orchestrator uses your reasoning to make decisions.
- If you found nothing credible, return an empty array and found_any=false — do not fabricate candidates.
- Never include a ticket just because a keyword matched. The description must make sense.
- You are one of several workers trying different angles. Be honest about confidence — don't over-claim.`,
}
```

---

## Step-by-Step Implementation Guide

### `query_ticket.go` — Main Workflow

#### `StepInit`
1. Get last user message from `context.GetLastUserMessages()`
2. Build extracted_info + should_clarify schema
3. Call `askAI(userMessage, "", orchestratorPersonality, extractionSchema, "qt_main")`
4. Set next step to `StepQtClarify`

#### `StepQtClarify`
1. Read AI response — get `should_clarify` and `extracted_info`
2. Store `extracted_info` as `qt_extracted_info` in workflow data
3. Initialize `qt_attempt_count = 0`, `qt_rejected_ids = []`, `qt_all_candidates = []`
4. If `should_clarify == true`:
   - Send `ActionUserWait` with the clarifying question
   - Set next step to `StepQtPlanSearch` (user answer comes back here)
5. If `should_clarify == false`:
   - Proceed directly to `StepQtPlanSearch` with an empty user message (no wait needed)

#### `StepQtPlanSearch`
1. If coming from a clarifying question wait, update `qt_extracted_info` with new user info
2. Build context string from `qt_extracted_info` + `qt_rejected_ids` + `qt_attempt_count`
3. Call orchestrator AI with plan search schema
4. Set next step to `StepQtSpawnWorkers`

#### `StepQtSpawnWorkers`
1. Read AI response — get `worker_count` and `strategies`
2. Store `worker_count` as `qt_expected_workers`
3. Build `AsyncAction` containing one `ActionWorkflow` per strategy:
   - `InputWorkflowName = WorkflowQueryTicketSearcher`
   - `InputStep = StepQtswSearch`
   - `InputSubWorkerID = strategy.worker_id`
   - `InputMessage = strategy as JSON string` (search params + angle description)
   - Also pass: `qt_extracted_info` (so sub-worker has full context), `qt_rejected_ids`
4. Return `[AsyncAction]`

#### `StepQtCollectResult`
1. Read AI response (sub-worker result) — get `worker_id`, `candidates_json`, `found_any`
2. Store result as `subWorkerKey(workerID, "result")` in workflow data
3. Count how many results have arrived vs `qt_expected_workers`
4. If not all arrived: return `nil, nil` (wait for more)
5. If all arrived:
   - Collect all candidate lists, merge into one prompt for orchestrator
   - Build synthesis prompt including: all candidates, rejected IDs, attempt count
   - Call orchestrator AI with analyze schema
   - Set next step to `StepQtAnalyze`
   - Return AI action + `ActionCompleteAsync`

#### `StepQtAnalyze`
1. Read AI response — get `branch`, `top_ticket_id`, `top_candidates_json`, `message_to_user`
2. Merge `top_candidates_json` into `qt_all_candidates` workflow data
3. Route based on `branch`:
   - `"present_ticket"` → fetch full ticket via `ActionTool(ado_get_ticket)`, then go to `StepQtPresentTicket`
   - `"show_candidates"` → go to `StepQtShowCandidates`
   - `"disambiguate"` → go to `StepQtDisambiguate`
   - `"narrow_down"` → go to `StepQtNarrowDown`
   - `"refine"` → go to `StepQtRefine`

#### `StepQtDisambiguate`
1. Read `message_to_user` from stored analysis (or re-use from analyze step)
2. Send `ActionUserWait` with the disambiguation question
3. Next user reply routes back to `StepQtPlanSearch` — orchestrator will re-plan with new info

#### `StepQtShowCandidates`
1. Format top 2-3 candidates into a Slack-friendly list with summaries
2. Send `ActionUserWait` asking user to pick one or say "none of these"
3. On reply:
   - If user picks one → fetch full ticket, go to `StepQtPresentTicket`
   - If "none of these" → go to `StepQtRefine`
   - If "never mind" / "done" → end workflow

#### `StepQtNarrowDown`
1. Send `ActionUserWait` with `message_to_user` asking for narrowing detail
2. On reply: update `qt_extracted_info` with new info, go to `StepQtPlanSearch`

#### `StepQtRefine`
1. Increment `qt_attempt_count`
2. If `attempt_count <= 2`: go straight to `StepQtPlanSearch` (orchestrator will try different angles automatically)
3. If `attempt_count == 3`: send `ActionUserWait` with a specific clarifying question, go to `StepQtPlanSearch` on reply
4. If `attempt_count >= 4`: go to `StepQtExhausted`

#### `StepQtExhausted`
1. Send `ActionUserWait`:
   ```
   I've tried several searches and can't pin down the ticket. What would you like to do?
   • Reply "keep trying" and tell me anything else you remember
   • Reply "show best" to see the closest matches I found
   • Reply "give up" to stop here
   ```
2. On reply: route to `StepQtPlanSearch`, `StepQtShowCandidates` (with `qt_all_candidates`), or end workflow

#### `StepQtPresentTicket`
**First visit** (no `qt_found_ticket` yet, or arriving from tool result):
1. Store tool result as `qt_found_ticket`
2. Format ticket nicely for Slack (ID, title, state, type, assigned to, QA, URL)
3. Send `ActionUserMessage` with formatted ticket
4. Send `ActionUserWait` with "Is this the one? Feel free to ask me anything about it."
5. Next step = `StepQtPresentTicket` (loop back to self)

**Subsequent visits** (follow-up message from user):
1. Load `qt_found_ticket` from workflow data
2. Get user's new message
3. Call orchestrator AI with follow-up decision schema, providing:
   - Full ticket data as context
   - User's new message
4. Read `intent`:
   - `"answer_question"` → send `ActionUserWait` with the answer, loop back to `StepQtPresentTicket`
   - `"wrong_ticket"` → add ticket ID to `qt_rejected_ids`, clear `qt_found_ticket`, go to `StepQtPlanSearch`
   - `"new_ticket_search"` → clear all state, go to `StepInit`
   - `"done"` → send a closing message, end workflow

---

### `query_ticket_searcher.go` — Sub-Worker

#### `StepQtswSearch`
1. Get `workerID` from `InputSubWorkerID`
2. Get `instructions` (JSON string with search params) from `InputMessage`
3. Get `extractedInfo` from workflow data (passed via input)
4. Get `rejectedIDs` from workflow data
5. Parse search params from instructions JSON
6. Build `ActionTool(ado_search_tickets)` with parsed params
7. After tool result: call searcher AI with schema to evaluate candidates
   - Include: user's original description (from extractedInfo), tool results, rejected IDs
   - AI returns `candidates_json` with confidence scores
8. For any candidate with confidence ≥ 0.6: call `ado_get_ticket` for full details
9. After enrichment: re-evaluate confidence with full details
10. Return result to parent via `ActionAi` with step = `StepQtCollectResult`

> **Conversation key:** `fmt.Sprintf("qt_searcher_%s", workerID)` — isolated per worker

---

## Key Decisions & Trade-offs

### Sub-worker strategy JSON
The plan search AI returns strategies as a JSON string inside a regular string field, rather than
a nested schema object. This keeps compatibility with the current SchemaBuilder. The searcher
sub-worker parses this JSON. If SchemaBuilder is extended to support nested object arrays, revisit.

### `ado_get_ticket` registration
The tool file already exists (`toolADOGetTicket.go`) but is unregistered. It must be registered
in `tool.go` before this workflow can use it.

### Stateful follow-up routing
`StepQtPresentTicket` loops to itself via `ActionUserWait`. The intent analyzer must not override
this — the workflow handles all routing logic once in present_ticket state. Make sure
`StepQtPresentTicket` is in `AvailableSteps` so the intent analyzer knows the workflow handles it.

### Attempt count reset
`qt_attempt_count` resets to 0 when the user provides new information (from disambiguate, narrow_down,
or exhausted → keep_trying). It only counts rounds that start from the same information base.

---

## Checklist Before Coding

- [ ] Register `ado_get_ticket` tool in `tool.go`
- [ ] Add `WorkflowQueryTicketSearcher` constant and map entry in `workflow.go`
- [ ] Add personality constants + map entries in `personality.go`
- [ ] Create `query_ticket_orchestrator.go` personality file
- [ ] Create `query_ticket_searcher.go` personality file
- [ ] Implement `query_ticket_searcher.go` workflow (sub-worker, simpler — do this first)
- [ ] Implement `query_ticket.go` workflow step by step
- [ ] Update `WorkflowQueryTicket` map entry with `AvailableSteps`
