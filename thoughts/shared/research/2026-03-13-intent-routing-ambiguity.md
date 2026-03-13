# Intent Routing: Handling Ambiguity

> Date: 2026-03-13

## The Problem

The intent analyzer routes user messages to workflows. Sometimes a message is genuinely
ambiguous — the AI cannot confidently pick a workflow. The question is how to handle that
without polluting the main conversation with routing noise.

**Core constraint:** The main conversation (`mainConversationID` / `lastResponseID`) must
stay clean. It is the shared context for all workflow AI calls. Injecting intent analysis
chatter into it degrades every subsequent AI call in the thread.

---

## Approach A: Intent Analyzer Asks Clarifying Questions (implemented, on branch `feature/intent-clarification`)

The intent analyzer runs in its own **private branch** off `lastResponseID`. When it cannot
route confidently it asks the user a clarifying question. The branch tip is stored on the
context (`pendingIntentResponseID`). The next call continues that branch — the AI already
knows what it asked. The main conversation is never touched.

**Flow:**
1. `callIntentAI` branches off `lastResponseID` → AI returns `clarifying_question` non-empty
2. Store `ResponseID` → `pendingIntentResponseID`; store original message → `clarifyingOriginalMessage`; increment `clarifyingTurnCount`
3. Emit `ActionUserWait` with the question (blocking)
4. User answers → `callIntentAI` continues from `pendingIntentResponseID` (AI has full context)
5. Repeat steps 2–4 if AI asks another question (hard break at 3 turns)
6. On exit (natural or hard break): synthesize forwarded message:
   - If AI `clarification_summary` is non-empty: `"<original>\n(System Clarification: <summary>)"`
   - If empty (nothing workflow-relevant learned): just the original message
7. Replace last entry in `lastUserMessages` with synthesized message
8. Clear `pendingIntentResponseID`, `clarifyingOriginalMessage`, `clarifyingTurnCount`
9. Normal routing proceeds — workflow sees the original request + any relevant clarification

**Why the summary matters:** the clarifying Q&A happens in the intent analyzer's private
branch. The workflow AI never sees it. The summary is the only channel to pass workflow-
relevant information (e.g. "user wants to assign the ticket to John in the backend project")
from the clarification cycle to the workflow.

**What stays clean:**
- Main conversation never sees the clarifying Q&A
- Summary is minimal — only workflow-relevant facts, never the routing decision itself
- If summary is empty, the original message is forwarded unchanged

**Trade-offs:**
- Extra round-trips when the user is asked a clarifying question
- More state on `ConversationContext` (3 in-memory fields + 1 DB column)
- The intent analyzer takes on some responsibility that workflows could handle naturally

---

## Approach B: Route Greedily, Let Workflows Handle Ambiguity (implemented on master)

When confidence is too low, just route to the best-guess workflow (or a safe default when
there is no active workflow). The workflow AI has full main conversation context and will
ask for any missing information as part of its normal execution. Those questions land in
the main conversation naturally.

**Flow:**
- Below threshold, no current workflow → route to best-guess / safe default workflow
- Below threshold, current workflow → stay in current workflow (as today)
- Intent analyzer never asks questions, never stores extra state

**Why this is simpler:**
- Intent analyzer has one job: pick a workflow
- Ambiguity resolution belongs to the workflow, which knows exactly what it needs
- No out-of-band Q&A, no synthesized messages, no extra context fields
- In practice, clarifying questions in the intent analyzer are mostly for *routing* —
  once routed to the right workflow, the workflow will ask its own questions anyway

**Trade-offs:**
- One extra round-trip if the default workflow re-routes
- Rare case: info from routing clarification that is *also* workflow-relevant is re-asked
  by the workflow (acceptable UX cost)

---

## Decision

**Master uses Approach B.** The intent analyzer routes, never asks.

**`feature/intent-clarification` preserves Approach A** as a ready-to-merge implementation
if future requirements make greedy routing insufficient (e.g. a large number of workflows
where wrong-workflow startup cost is high).
