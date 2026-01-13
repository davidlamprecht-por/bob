# Bob Orchestrator – Decision Log (Summary)

## Core orchestration decisions

- **One active run per conversation**
  - No auto-resume after app restart
  - If the app restarts, next user message re-enters via intent + cold facts

- **Action queue is cached (hot), not persisted**
  - Remaining actions are stored in memory while the app is running
  - Losing the queue is acceptable; system degrades gracefully

- **Blocking actions (`WaitForUser`)**
  - Immediately stop the run
  - Send prompt to user
  - Persist minimal cold facts (what we were waiting for)
  - Do NOT continue the queue

---

## User message handling model

- **Coalesce short bursts**
  - Buffer messages for ~0.8–1.5s
  - Merge into a single “turn”
  - Only run intent once after burst ends
  - Improves UX and avoids intent thrashing

- **After intent is committed**
  - New messages are **queued**, not re-routed
  - Messages are injected only at safe points (before AI/tool calls, after parallel groups, at waits)

- **Interrupts**
  - No implicit AI-detected interrupts
  - Explicit slash command only: `/bobstop`
  - Cancels current run and clears queued messages

---

## Intent & workflow behavior

- **Intent is advisory, not binding**
  - Used to choose workflow or next step
  - If user changes topic later, orchestrator may re-route at safe points

- **Workflow rigidity fix**
  - Waiting state is *suggested*, not forced
  - On new message, orchestrator can resume, re-plan, or switch workflows

---

## Sub-agents

- Sub-agents are just runs with isolated scope
- They may ask clarifying questions (`WaitForUser`)
- If interrupted or app restarts, they can be restarted from cold facts
- Partial progress loss is acceptable

---

## User role handling (for now)

- **No hard authorization yet**
- Roles are for **audience/personalization only**
  - Developer vs Support affects tone, depth, suggested workflows
  - Not security-critical at this stage

- Default role: **general**
- Role capture only when useful (not upfront)
- Future restricted workflows will add real capability checks later

---

## Architectural non-decisions (explicitly deferred)

- No persistent action queues in DB
- No multi-lane concurrent workflows
- No AI-inferred authorization
- No mandatory role setup on first contact

---

## Immediate implementation checklist

- Add **run state**: `idle | collecting | running | waiting`
- Add **coalesce timer** per conversation
- Cache **action queue** in memory with TTL
- Persist minimal **wait context**
- Implement `/bobstop`
- Enforce **safe points** in action loop
- Keep main loop unchanged; all logic is pre-/post-processing
