# AI Context Management & Schema Builder Design

**Date:** 2026-01-03
**Status:** Design Phase
**Related:** Workflow system, orchestrator, database layer

---

## Table of Contents
1. [Problem Statement](#problem-statement)
2. [The Async Action Solution: Sub-Agents Simplified](#the-async-action-solution-sub-agents-simplified)
3. [AI Context Architecture](#ai-context-architecture)
4. [Structured Output Pattern](#structured-output-pattern)
5. [Schema Builder Design](#schema-builder-design)
6. [Implementation Plan](#implementation-plan)
7. [Code Examples](#code-examples)

---

## Problem Statement

### **Issue 1: AI Context Management**
Python v1 had confusing sub-agent and sub-workflow implementations. Need a cleaner way to:
- Scope AI conversations appropriately (avoid bloated context)
- Support workflow-specific conversations
- Handle side questions without polluting workflow state
- Maintain conversation history efficiently

### **Issue 2: Schema Definition Pain**
Python v1 required writing full JSON Schema for every structured output:
```json
{
  "type": "object",
  "properties": {
    "response": {"type": "string", "description": "..."},
    "summary": {"type": "string", "description": "..."}
  },
  "required": ["response", "summary"],
  "additionalProperties": false
}
```
This is verbose, error-prone, and tedious to maintain.

---

## The Async Action Solution: Sub-Agents Simplified

### **The Breakthrough: No Special Sub-Agent Machinery Needed**

Python v1 had complex sub-agent and sub-workflow implementations with special classes, nested contexts, and callback mechanisms. The Go v2 design eliminates all of this complexity.

**The key insight**: You don't need special sub-agent machinery when you have:
1. **Flat queue** - Everything flows through the same action loop
2. **Async actions** - Spawn parallel work via goroutines
3. **Source tracking** - Actions know which workflow spawned them
4. **Workflow control** - Business logic stays in workflows

Sub-agents become **just a pattern**, not a framework feature.

---

### **Traditional Sub-Agent Pattern (Complex)**

Most systems require special machinery:

```python
# Python v1 style (hypothetical)
class SubAgent:
    def __init__(self, parent_context, scope):
        self.parent = parent_context
        self.context = SubContext(parent_context, scope)
        self.result_callback = None

    async def execute(self):
        # Complex state management
        # Nested context handling
        # Result passing back to parent
        # Scope isolation
        # Error bubbling
```

**Problems with this approach:**
- ❌ Special classes for sub-agents
- ❌ Nested context management (parent/child relationships)
- ❌ Complex result callback mechanisms
- ❌ State synchronization issues between parent/child
- ❌ Hard to test (tight coupling)
- ❌ Hard to debug (nested stack traces)
- ❌ Difficult to parallelize (context dependencies)

---

### **The Go v2 Design (Simple)**

**Three ingredients, no special classes:**

#### **1. Async Actions**
Actions can spawn multiple sub-actions that run in parallel:

```go
type Action struct {
    ActionType       ActionType
    SourceWorkflow   string        // Which workflow spawned this
    AsyncGroupID     string        // For correlating parallel results
    AsyncGroupSize   int           // Expected number of results
    Input            map[string]any
    AsyncActions     []Action      // Sub-actions to spawn
}
```

#### **2. Flat Action Queue**
All actions, regardless of "nesting", flow through the same main loop:

```
Traditional (nested):
Main Workflow
  └─ Sub-Agent 1 (own queue/context)
       └─ Sub-Sub-Agent (own queue/context)
            └─ Results bubble up through layers

Go v2 (flat):
[Action1, Action2, ActionAsync, Action3, Result1, Result2, Result3, ...]
                       ↓
            (spawns goroutines)
                       ↓
            (results go to same queue via channel)
```

Everything flows through **ONE queue**. No nesting. No bubbling. Simple.

#### **3. Source Tracking**
Every action knows where it came from:

```go
action := &Action{
    ActionType: ActionAI,
    SourceWorkflow: "createTicket",  // ← Knows its origin
    AsyncGroupID: "research-123",    // ← Knows its group
}
```

When results come back, you know:
- Which workflow spawned it
- Which async group it belongs to
- How to route the result back

No callbacks. No parent references. Just data.

---

### **How It Works: Parallel Sub-Tasks Example**

**Scenario**: Workflow needs to research 3 topics in parallel

```go
// Workflow spawns 3 research "sub-agents"
return []*core.Action{
    {
        ActionType: ActionAsync,
        AsyncGroupID: "research-123",
        AsyncGroupSize: 3,
        SourceWorkflow: "createTicket",
        AsyncActions: []*Action{
            {
                ActionType: ActionAI,
                SourceWorkflow: "createTicket",
                Input: map[string]any{
                    "prompt": "Research project guidelines from docs/",
                    "conversation_id": createEphemeralConversation("research_guidelines"),
                },
            },
            {
                ActionType: ActionAI,
                SourceWorkflow: "createTicket",
                Input: map[string]any{
                    "prompt": "Search for similar existing tickets",
                    "conversation_id": createEphemeralConversation("search_duplicates"),
                },
            },
            {
                ActionType: ActionTool,
                SourceWorkflow: "createTicket",
                Input: map[string]any{
                    "tool": "validate_title_format",
                    "title": proposedTitle,
                },
            },
        },
    },
    {
        ActionType: ActionWorkflowResult,
        Input: map[string]any{
            "wait_for_group": "research-123",
            "on_complete": "processResearchResults",
        },
    },
}
```

**What happens (from orchestrator.go:StartHandlingActions)**:

1. **Orchestrator sees ActionAsync**
   ```go
   case ActionAsync:
       for _, subAction := range a.AsyncActions {
           go func(action Action) {
               newActions, err := action.ProcessAction(context, responder, actionChan)
               for _, newAction := range newActions {
                   actionChan <- newAction  // Send back to main loop
               }
           }(subAction)
       }
   ```

2. **3 goroutines spawn** - One per sub-action, running in parallel

3. **Each processes independently** - Own AI conversation, own scope

4. **Results sent via channel** - Back to main loop's actionChan

5. **Main loop drains channel** - Adds results to flat queue
   ```go
   for {
       select {
       case action := <-actionChan:
           actionQueue = append(actionQueue, action)
       default:
           goto continueLoop
       }
   }
   ```

6. **ActionWorkflowResult collects** - When AsyncGroupSize reached

7. **Results routed to workflow** - Via SourceWorkflow field

**All without any special sub-agent classes or nested contexts!**

---

### **Why This Design is Superior**

#### **1. Flat Queue = No Nesting Complexity**

Everything flows linearly:
```
Main Loop Processing:
1. Pop action from queue
2. Process action
3. Add resulting actions back to queue
4. Repeat

[Action1] → Process → [Action2, Action3]
[Action2] → Process → [ActionAsync(A, B, C)]
[ActionAsync] → Spawn goroutines → Results go to channel
Channel drain → [ResultA, ResultB, ResultC] added to queue
[ResultA] → Process → [NextAction]
...
```

No recursive calls. No nested execution contexts. Just a loop.

#### **2. Source Tracking = Natural Result Routing**

```go
// When action completes
if action.SourceWorkflow == "createTicket" {
    // Route result back to CreateTicket workflow
    return &Action{
        ActionType: ActionWorkflowResult,
        SourceWorkflow: action.SourceWorkflow,
        Input: map[string]any{"result": result},
    }
}
```

No callback functions. No parent references. Just return actions with source info.

#### **3. Workflows Control Everything**

Business logic stays in workflows:

```go
func (w *CreateTicket) processResearchResults(ctx *core.ConversationContext, results []any) ([]*core.Action, error) {
    // Workflow decides what to do with results
    if allResearchComplete(results) {
        // Continue with enriched context
        return w.askForDescription(ctx, results)
    } else {
        // Wait for more results or handle errors
        return w.handleIncompleteResearch(results)
    }
}
```

No hidden framework behavior. Workflow is in full control.

#### **4. Composable (Can Nest Arbitrarily)**

AsyncActions can spawn more AsyncActions:

```go
AsyncActions: []*Action{
    {
        ActionType: ActionAsync,  // ← Nested async!
        AsyncGroupID: "nested-research",
        AsyncActions: []*Action{
            // Even deeper parallel work
        },
    },
}
```

All results still flow back through the flat queue. No special handling needed.

#### **5. Leverages Go's Strengths**

- **Goroutines**: Cheap (2KB stack), fast spawning, thousands possible
- **Channels**: Safe communication between goroutines
- **No GIL**: True parallelism (unlike Python's asyncio which is single-threaded)
- **Type safety**: Catch errors at compile time

---

### **Comparison: Python v1 vs Go v2**

| Aspect | Python v1 (Complex) | Go v2 (Simple) |
|--------|---------------------|----------------|
| **Sub-agent creation** | Special SubAgent classes | Just `ActionAsync` |
| **State management** | Nested SubContext objects | Flat queue + source tracking |
| **Result handling** | Callbacks/promises | Actions in queue |
| **Parallelism** | asyncio (single-threaded) | Goroutines (true parallel) |
| **Debugging** | Nested stack traces | Linear queue flow |
| **Testing** | Mock parent/child contexts | Mock action processing |
| **Scope isolation** | Manual context splitting | AsyncGroupID correlation |
| **Error handling** | Bubble up through layers | Return errors in actions |
| **Code complexity** | ~200 lines for sub-agent framework | ~20 lines for async handling |
| **Mental model** | Tree of nested contexts | Flat stream of actions |

---

### **Real-World Example: Ticket Creation with Parallel Research**

**Scenario**: CreateTicket workflow needs to:
1. Research project guidelines from documentation
2. Validate proposed title format
3. Search for duplicate tickets
4. Get team member info for assignee suggestion

All in parallel (sub-agents).

**Implementation:**

```go
func (w *CreateTicket) researchPhase(ctx *core.ConversationContext) ([]*core.Action, error) {
    groupID := "research-" + uuid.New().String()

    return []*core.Action{
        {
            ActionType: ActionAsync,
            AsyncGroupID: groupID,
            AsyncGroupSize: 4,
            SourceWorkflow: "createTicket",
            AsyncActions: []*Action{
                // Sub-agent 1: Research guidelines
                {
                    ActionType: ActionAI,
                    SourceWorkflow: "createTicket",
                    Input: map[string]any{
                        "prompt": "Read and summarize docs/PROJECT_GUIDELINES.md",
                        "conversation_id": createEphemeralConversation("research_guidelines"),
                    },
                },
                // Sub-agent 2: Validate title format
                {
                    ActionType: ActionTool,
                    SourceWorkflow: "createTicket",
                    Input: map[string]any{
                        "tool": "validate_title_format",
                        "title": ctx.GetWorkflowData()["proposed_title"],
                    },
                },
                // Sub-agent 3: Search for duplicates
                {
                    ActionType: ActionTool,
                    SourceWorkflow: "createTicket",
                    Input: map[string]any{
                        "tool": "ado_search_tickets",
                        "query": ctx.GetWorkflowData()["proposed_title"],
                    },
                },
                // Sub-agent 4: Get team info
                {
                    ActionType: ActionTool,
                    SourceWorkflow: "createTicket",
                    Input: map[string]any{
                        "tool": "ado_get_team_members",
                        "project": ctx.GetWorkflowData()["project"],
                    },
                },
            },
        },
        {
            ActionType: ActionWorkflowResult,
            SourceWorkflow: "createTicket",
            Input: map[string]any{
                "wait_for_group": groupID,
                "handler": "processResearchResults",
            },
        },
    }, nil
}
```

**Result:**
- 4 "sub-agents" run in parallel (true parallelism with goroutines)
- Each has its own scope (own AI conversation or tool call)
- Results collected when all 4 complete
- Workflow processes results and continues
- Total time: ~max(sub-agent times), not sum(sub-agent times)

**No special sub-agent machinery. Just actions flowing through a flat queue.**

---

### **Benefits Summary**

✅ **Simpler** - No special classes, inheritance, or framework magic
✅ **Flatter** - No nested execution contexts or recursive calls
✅ **Clearer** - All actions visible in one queue, easy to trace
✅ **Faster** - True parallelism with goroutines (not single-threaded asyncio)
✅ **Testable** - Easy to mock action processing, no complex context setup
✅ **Debuggable** - Linear flow, clear action queue, no deep stack traces
✅ **Flexible** - Workflows control spawning and result handling
✅ **Scalable** - Spawn N sub-tasks easily, limited only by goroutine capacity
✅ **Composable** - Can nest async actions arbitrarily deep
✅ **Type-safe** - Go's compiler catches errors before runtime

---

### **The Key Insight**

Traditional systems treat sub-agents as a **framework feature** requiring special machinery.

This design treats sub-agents as a **pattern** using three simple primitives:
1. Actions can spawn more actions (AsyncActions)
2. All actions flow through one queue (flat)
3. Actions track their source (SourceWorkflow)

**Result**: Sub-agents emerge naturally from the design without any special code.

This is the kind of design that looks obvious in hindsight but requires real architectural insight to discover. It's simpler, faster, and more maintainable than traditional approaches.

---

## AI Context Architecture

### **Core Principle: Workflow-Centric**

**Workflows are THE main way things get done.** There is no "general conversation mode" - the app is workflow-centric.

### **Conversation Hierarchy**

```
User+Thread (e.g., "user123:thread456")
│
├─ Active Workflow (e.g., CreateTicket)
│   │
│   ├─ Main AI Conversation
│   │   └─ THE primary conversation for this workflow
│   │   └─ Created in StepInit
│   │   └─ All workflow AI calls use this
│   │   └─ Archived when workflow completes/changes
│   │
│   └─ Side Question (ephemeral)
│       └─ Handles user questions during workflow
│       └─ References main conversation context
│       └─ Archived immediately after answering
│
└─ Archived Workflow Conversations
    └─ Previous workflow attempts with summaries
    └─ Can be revived if user returns to topic
```

### **Conversation Types & Lifecycles**

#### **1. Intent Analysis Conversation**
- **Purpose**: Determine which workflow to route to
- **Context**: Last 10 Slack messages (user + bot) - raw text only
- **No AI conversation tracking** - ephemeral analysis
- **Future enhancement**: If AI needs more context, can request more messages

#### **2. Main Workflow Conversation**
- **Purpose**: Execute the workflow
- **Created**: In `StepInit` of workflow
- **Lifecycle**:
  - Init: Create new AI conversation, archive any previous active conversation
  - Active: All workflow AI calls use this conversation ID
  - Complete: Archive with overall summary when workflow finishes
  - Switch: Archive when user switches to different workflow
- **Storage**: Conversation ID stored in `WorkflowContext.WorkflowData["ai_conversation_id"]`

#### **3. Side Question Conversation**
- **Purpose**: Answer user questions during workflow without interrupting workflow state
- **Context Includes**:
  - Workflow name and current step
  - Recent workflow AI conversation messages (last 3-5 turns)
  - Workflow metadata (help text, guidelines)
  - User's question
- **Parent**: References main workflow conversation via `ParentConversationID`
- **Lifecycle**: Create → Answer → Archive immediately with summary

### **Key Design Decisions**

1. **No main conversation larger than workflow** - workflows ARE the conversation
2. **Intent analysis is lightweight** - last 10 messages only, no conversation state
3. **Init step creates conversation** - every workflow run gets fresh AI conversation
4. **Side questions don't interrupt state** - workflow state preserved, question answered, workflow resumes
5. **Archive with summaries** - every archived conversation has overall summary for context

---

## handleDefaultSteps: The Workflow Entry Point

### **Overview**

`handleDefaultSteps` is a critical function that runs **before every workflow execution** to handle common steps that apply to all workflows (unless a workflow opts out).

**Location**: `internal/workflow/workflow.go`

**Called from**: `RunWorkflow()` before calling the workflow's `WorkflowFn`

---

### **Design Philosophy**

**Goal**: Provide default behavior for common workflow entry points without forcing every workflow to reimplement the same logic.

**Key principle**: "Intersect the workflow without interrupting the STATE"
- Handle common scenarios (init, side questions)
- Preserve workflow state
- Allow workflow to continue normally or skip this turn

---

### **Function Signature**

```go
func handleDefaultSteps(
    w WorkflowDefinition,
    c *core.ConversationContext,
    a *core.Action,
) (actions []*core.Action, skipWorkflow bool, error)
```

**Return values**:
- `actions`: Actions to execute (e.g., answer to side question)
- `skipWorkflow`: If `true`, skip calling the workflow function this turn
- `error`: Any error that occurred

**This signature allows**:
- ✅ **Interrupt**: Skip workflow execution (side questions)
- ✅ **Enhance**: Add actions before workflow runs (future use)
- ✅ **Pass-through**: Return nothing, let workflow handle (init, answers)

---

### **The Three Default Steps**

#### **1. StepInit - Workflow Initialization**

**When**: User starts a new workflow or workflow is restarted

**Purpose**: Set up the workflow environment, create AI conversation

**Behavior**:
- Archive any existing active AI conversation
- Create new AI conversation for this workflow run
- Store conversation ID in `WorkflowContext.WorkflowData["ai_conversation_id"]`
- Initialize any workflow-specific setup

**handleDefaultSteps action**: Returns `(nil, false, nil)` - let workflow handle init
- Workflow's `StepInit` case should create the AI conversation
- This keeps control in the workflow

**Example workflow implementation**:
```go
func CreateTicket(context, sourceAction) ([]*core.Action, error) {
    step := getInput(sourceAction, core.InputStep)

    if step == StepInit {
        // Archive old conversation
        if context.ActiveAIConversationID != nil {
            aiService.ArchiveConversation(*context.ActiveAIConversationID)
        }

        // Create new conversation for this workflow
        convID, _ := aiService.CreateConversation("workflow:createTicket", nil, initialContext)
        context.ActiveAIConversationID = &convID
        context.GetCurrentWorkflow().WorkflowData["ai_conversation_id"] = convID

        // Greet user and start workflow
        return askUserWhatTicketType(context, convID)
    }
    // ... rest of workflow
}
```

#### **2. StepUserAnsweringQuestion - User Responds to Workflow**

**When**: User provides an answer to a question the workflow asked

**Purpose**: Let workflow process the user's answer

**Behavior**:
- User is responding to workflow's last question
- Workflow knows the context and expects this answer
- Workflow handles the response

**handleDefaultSteps action**: Returns `(nil, false, nil)` - let workflow handle
- Workflow is in control of its conversation flow
- No default handling needed

**Example workflow implementation**:
```go
case StepUserAnsweringQuestion:
    // Workflow knows what it last asked
    lastQuestion := context.GetWorkflowData()["last_question"]
    userAnswer := context.GetLastUserMessage().Message

    // Process answer based on workflow state
    if lastQuestion == "ticket_title" {
        return processTicketTitle(context, userAnswer)
    }
```

#### **3. StepUserAsksQuestion - User Asks Side Question**

**When**: User asks a question during workflow execution (not answering workflow's question)

**Purpose**: Answer the user's question without interrupting workflow state

**Behavior**:
- Create ephemeral AI conversation for side question
- Include context: workflow info, recent activity, workflow help text
- Answer the question
- Archive side conversation immediately
- **Skip workflow execution this turn** (skipWorkflow=true)
- Next user message will resume workflow at same state

**handleDefaultSteps action**: Returns `(actions, true, nil)` - handled completely
- Creates side question conversation
- Calls AI to answer
- Archives conversation
- Returns action to send answer to user
- `skipWorkflow=true` prevents workflow from running

**Implementation** (in handleDefaultSteps):
```go
case StepUserAsksQuestion:
    actions, err := handleSideQuestion(w, c, a)
    return actions, true, err  // Skip workflow this turn
```

**Key insight**: "Intersect without interrupting STATE"
- **State preserved**: Workflow's step, collected data, conversation - all unchanged
- **Question answered**: User gets their answer
- **Workflow resumes**: Next user message continues from where workflow left off

---

### **Opt-Out Mechanism**

Workflows can opt out of default handling:

```go
workflows[WorkflowSpecial] = WorkflowDefinition{
    Name: "special",
    Description: "Workflow with custom step handling",
    WorkflowFn: SpecialWorkflow,
    Options: map[Option]any{
        OptionOverwriteHandleDefaultSteps: true,  // Opt out
    },
}
```

When opted out, `handleDefaultSteps` returns immediately:
```go
if overwrite, ok := w.Options[OptionOverwriteHandleDefaultSteps]; ok && overwrite != false {
    return nil, false, nil  // Workflow handles everything
}
```

---

### **Side Question Context Building**

When handling `StepUserAsksQuestion`, the side question AI needs context:

```go
sideQuestionContext := map[string]any{
    "workflow_name": c.GetCurrentWorkflow().WorkflowName,
    "current_step": c.GetCurrentWorkflow().Step,
    "recent_activity": mainConversation.RecentSummary,  // Last 3-5 turns
    "workflow_help": w.HelpText,  // Workflow-provided help text
    "user_question": lastMessage.Message,
}
```

**Design decision**: Last 3-5 turns from main conversation
- Enough context to answer questions about recent interactions
- Not too much (would bloat context and cost)
- Configurable if needed later

---

### **Workflow Help Text**

To support side questions, `WorkflowDefinition` should include help text:

```go
type WorkflowDefinition struct {
    Name        WorkflowName
    Description string
    HelpText    string  // NEW: Help info for side questions
    WorkflowFn  func(...)
    Options     map[Option]any
}
```

**Example**:
```go
WorkflowCreateTicket: {
    Description: "Guides user to create ADO work ticket",
    HelpText: `
        - Title should be brief, under 80 characters
        - Description should include: what happened, expected behavior, steps to reproduce
        - Severity: low (minor issue), medium (affects workflow), high (blocks work), critical (system down)
        - Assignment: Leave blank to auto-assign, or specify team member
    `,
    WorkflowFn: CreateTicket,
}
```

This helps the side question AI answer questions like:
- "What should I include in the description?"
- "How do I format the title?"
- "What severity level should I use?"

---

### **Integration with RunWorkflow**

```go
func RunWorkflow(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
    cw := context.GetCurrentWorkflow()
    if cw == nil {
        return nil, fmt.Errorf("no current workflow set")
    }

    wf := WorkflowName(cw.WorkflowName)
    workflow, ok := workflows[wf]
    if !ok {
        return nil, fmt.Errorf("unknown workflow: %q", wf)
    }

    // Handle default steps FIRST
    defaultActions, skipWorkflow, err := handleDefaultSteps(workflow, context, sourceAction)
    if err != nil {
        return nil, err
    }

    // If default handling says skip workflow, return just default actions
    if skipWorkflow {
        return defaultActions, nil
    }

    // Otherwise, run workflow and combine actions
    workflowActions, err := workflow.WorkflowFn(context, sourceAction)
    if err != nil {
        return nil, err
    }

    // Prepend default actions (if any) before workflow actions
    return append(defaultActions, workflowActions...), nil
}
```

---

### **Why This Design Works**

✅ **Workflow control** - Workflows decide init behavior, not framework
✅ **DRY principle** - Side question handling code written once, works for all workflows
✅ **State preservation** - Side questions don't mess with workflow state
✅ **Flexibility** - Workflows can opt out if they need custom behavior
✅ **Composability** - Default actions can be combined with workflow actions
✅ **Clear semantics** - `skipWorkflow` flag makes intent explicit

---

### **Implementation Priority: Workflow-First Approach**

**Key decision**: Implement workflows first, adapt AI/service layers based on workflow needs.

**Rationale**:
- Workflows define the business logic and requirements
- AI layer should serve workflow needs, not dictate them
- Starting with workflows reveals what AI service interface should be
- Easier to mock AI layer while building workflows

**Implementation order**:
1. ✅ Document design (this file)
2. **Next**: Implement `handleDefaultSteps` with stubs
   - `StepInit`: Pass through to workflow
   - `StepUserAnsweringQuestion`: Pass through to workflow
   - `StepUserAsksQuestion`: Return stub "I can't answer questions yet" message
3. Update `RunWorkflow` to call `handleDefaultSteps` and respect `skipWorkflow`
4. Test with existing workflow stubs
5. Implement one full workflow (CreateTicket) with init and side question handling
6. This will reveal what AI service interface needs to look like
7. Implement schema builder (needed for AI responses)
8. Implement AI service interface
9. Wire everything together

---

## Structured Output Pattern

### **The Breakthrough: AI Returns Summaries Automatically**

Instead of manually managing conversation summaries, **every AI response includes structured summaries**.

### **Standard AI Response Format**

```go
type AIResponse struct {
    Response       string   // The actual message to send to user

    OverallSummary string   // Summary of entire conversation
                            // Example: "User creating bug ticket for login redirect loop.
                            //           Collected: title, description.
                            //           Next: severity, assignee"

    RecentSummary  string   // Summary of last few exchanges
                            // Example: "Just collected ticket description.
                            //           About to ask for severity level"

    Data           map[string]any  // Workflow-specific structured data (optional)
}
```

### **Benefits**

✅ **Always up-to-date** - summaries update with every AI call
✅ **No extra AI calls** - part of the normal response
✅ **Automatic context compression** - always have compressed version ready
✅ **Flexible granularity** - use overall or recent summary depending on need
✅ **Archive-ready** - summary is part of conversation state
✅ **Token efficient** - can pass summaries instead of full history

### **Usage Patterns**

#### **Intent Analysis**
```go
intentContext := IntentContext{
    RecentMessages:    last10SlackMessages,
    PreviousWorkflow:  archivedWorkflow.OverallSummary,
    // "User was creating a ticket but stopped at description step"
}
```

#### **Side Questions**
```go
sideQuestionContext := SideQuestionContext{
    WorkflowName:    "CreateTicket",
    CurrentStep:     "collecting_description",
    RecentActivity:  mainConversation.RecentSummary,
    // "Just asked for description, user provided details about login loop"
    WorkflowHelp:    workflow.HelpText,
    UserQuestion:    "What should I include in the description?",
}
```

#### **Workflow Transitions**
```go
// User switches from CreateTicket to QueryTicket mid-way
newWorkflowContext := WorkflowContext{
    PreviousWorkflow:        "CreateTicket",
    PreviousWorkflowSummary: oldConversation.OverallSummary,
    // "User started creating ticket but abandoned"
}
```

### **Conversation Updates**

Every AI call automatically updates conversation state:

```go
response := aiService.SendMessage(conversationID, messages)

// Automatically update the conversation
conversation.OverallSummary = response.OverallSummary
conversation.RecentSummary = response.RecentSummary
conversation.LastUsedAt = time.Now()
```

When archiving:

```go
func archiveConversation(convID string) {
    conv := getConversation(convID)
    conv.Status = "archived"
    // Summary already there from last AI response!
    context.ArchivedConversations = append(context.ArchivedConversations, conv)
}
```

---

## Schema Builder Design

### **Goal: Flexible, Simple Schema Definition**

Workflows need different structured outputs. Instead of writing verbose JSON Schema, use a fluent builder API.

### **Storage Location**

Schemas stored in: `definitions/response_schemas/`

This allows:
- Centralized schema definitions
- Reusable across workflows
- Easy to review and modify
- Can be loaded at runtime or compile-time

### **Schema Builder API**

#### **Core Types**

```go
// SchemaField represents a single field in the schema
type SchemaField struct {
    Type        string         // "string", "number", "boolean", "object", "array"
    Description string
    Required    bool
    Properties  map[string]*SchemaField  // For nested objects
    Items       *SchemaField             // For arrays
    Enum        []string                 // For enum values
}

// Schema represents the complete JSON Schema
type Schema struct {
    Type                 string
    Properties           map[string]*SchemaField
    Required             []string
    AdditionalProperties bool
}
```

#### **Fluent Builder**

```go
type SchemaBuilder struct {
    schema    *Schema
    lastField string  // Track last added field for chaining
}

func NewSchemaBuilder() *SchemaBuilder
func (b *SchemaBuilder) String(name, description string) *SchemaBuilder
func (b *SchemaBuilder) Number(name, description string) *SchemaBuilder
func (b *SchemaBuilder) Boolean(name, description string) *SchemaBuilder
func (b *SchemaBuilder) Object(name, description string) *SchemaBuilder
func (b *SchemaBuilder) Array(name, description string, itemType string) *SchemaBuilder
func (b *SchemaBuilder) Enum(name, description string, values ...string) *SchemaBuilder
func (b *SchemaBuilder) Required() *SchemaBuilder
func (b *SchemaBuilder) Optional() *SchemaBuilder
func (b *SchemaBuilder) Build() *Schema
```

#### **OpenAI Format Conversion**

```go
func (s *Schema) ToOpenAIFormat() map[string]any {
    result := map[string]any{
        "type": "object",
        "properties": make(map[string]any),
        "required": s.Required,
        "additionalProperties": s.AdditionalProperties,
    }

    props := result["properties"].(map[string]any)
    for name, field := range s.Properties {
        props[name] = field.toMap()
    }

    return result
}
```

### **Type Mapping**

| Go Type | JSON Schema Type | Notes |
|---------|------------------|-------|
| string | "string" | Default text |
| int, int64, uint | "integer" | Whole numbers |
| float32, float64 | "number" | Decimals |
| bool | "boolean" | True/false |
| []T | "array" | Array of items |
| map[string]any | "object" | Key-value object |

---

## Implementation Plan

### **Phase 1: Schema Builder (Week 1)**

**Files to Create**:
- `internal/ai/schema_types.go` - Core type definitions
- `internal/ai/schema.go` - Builder implementation
- `internal/ai/schema_test.go` - Unit tests

**Deliverables**:
- Working schema builder with fluent API
- Conversion to OpenAI JSON Schema format
- Support for: string, number, boolean, object, array, enum
- Test coverage for all type conversions

### **Phase 2: AI Service Interface (Week 1-2)**

**Files to Create**:
- `internal/ai/service.go` - AI service interface
- `internal/ai/conversation.go` - Conversation management
- `internal/ai/mock_service.go` - Mock for testing

**Deliverables**:
- AI service interface definition
- Conversation lifecycle methods (create, archive, get)
- Mock implementation for testing without OpenAI
- Integration points defined

### **Phase 3: ConversationContext Integration (Week 2)**

**Files to Modify**:
- `internal/orchestrator/core/context.go` - Add AI conversation tracking

**Changes**:
- Add `ActiveAIConversationID *string`
- Add `ArchivedConversations []AIConversation`
- Add methods: `CreateAIConversation()`, `ArchiveAIConversation()`, `GetActiveConversation()`

### **Phase 4: Workflow Integration (Week 2-3)**

**Files to Modify**:
- `internal/workflow/workflow.go` - Update `handleDefaultSteps`
- `internal/workflow/workflow_funcs.go` - Implement `askAI`

**Changes**:
- `StepInit`: Create workflow AI conversation, archive old ones
- `StepUserAsksQuestion`: Create side question conversation, answer, archive
- Return signature: `handleDefaultSteps() (actions []*core.Action, skipWorkflow bool, err error)`

### **Phase 5: OpenAI Integration (Week 3-4)**

**Files to Create**:
- `internal/ai/openai_service.go` - Real OpenAI implementation
- `internal/ai/openai_conversation.go` - OpenAI conversation management

**Deliverables**:
- OpenAI API integration
- Structured output support
- Conversation thread management
- Error handling and retries

### **Phase 6: Schema Definitions (Week 4)**

**Files to Create**:
- `definitions/response_schemas/base_response.go` - Standard response schema
- `definitions/response_schemas/intent_analysis.go` - Intent analysis schema
- `definitions/response_schemas/ticket_creation.go` - Ticket workflow schemas

**Deliverables**:
- Reusable schema definitions
- Workflow-specific schemas
- Documentation for adding new schemas

---

## Code Examples

### **Example 1: Base Response Schema**

File: `definitions/response_schemas/base_response.go`

```go
package response_schemas

import "bob/internal/ai"

// BaseResponseSchema is the standard response format for all AI interactions
func BaseResponseSchema() *ai.Schema {
    return ai.NewSchemaBuilder().
        String("response", "The message to send to the user").Required().
        String("overall_summary", "Summary of the entire conversation so far (2-3 sentences)").Required().
        String("recent_summary", "Summary of just the last exchange (1 sentence)").Required().
        Build()
}
```

### **Example 2: Intent Analysis Schema**

File: `definitions/response_schemas/intent_analysis.go`

```go
package response_schemas

import "bob/internal/ai"

// IntentAnalysisSchema defines the expected output for intent classification
func IntentAnalysisSchema() *ai.Schema {
    return ai.NewSchemaBuilder().
        Enum("intent_type", "The type of intent detected",
            "new_workflow", "answer_question", "ask_question").Required().
        String("workflow_name", "The workflow to route to (if new_workflow)").Optional().
        Number("confidence", "Confidence score between 0 and 1").Required().
        String("reasoning", "Brief explanation of why this intent was chosen").Optional().
        Build()
}
```

### **Example 3: Ticket Creation Schema**

File: `definitions/response_schemas/ticket_creation.go`

```go
package response_schemas

import "bob/internal/ai"

// TicketCreationSchema extends base response with ticket-specific fields
func TicketCreationSchema() *ai.Schema {
    return ai.NewSchemaBuilder().
        // Base fields
        String("response", "Message to send to user").Required().
        String("overall_summary", "Overall conversation summary").Required().
        String("recent_summary", "Recent activity summary").Required().

        // Ticket-specific fields
        String("ticket_title", "Extracted ticket title").Optional().
        String("ticket_description", "Extracted ticket description").Optional().
        Enum("ticket_severity", "Severity level",
            "low", "medium", "high", "critical").Optional().
        String("ticket_assignee", "Suggested assignee").Optional().
        Boolean("is_complete", "Whether all required info has been collected").Required().

        Build()
}
```

### **Example 4: Side Question Schema**

File: `definitions/response_schemas/side_question.go`

```go
package response_schemas

import "bob/internal/ai"

// SideQuestionSchema for handling user questions during workflow
func SideQuestionSchema() *ai.Schema {
    return ai.NewSchemaBuilder().
        String("answer", "Answer to the user's question").Required().
        Boolean("is_workflow_related", "Whether question is related to current workflow").Required().
        String("clarification", "Any clarification needed from user").Optional().
        Build()
}
```

### **Example 5: Workflow Init with AI Conversation**

File: `internal/workflow/create_ticket.go`

```go
func CreateTicket(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
    step := getInput(sourceAction, core.InputStep)

    if step == StepInit {
        // Archive any existing active AI conversation
        if context.ActiveAIConversationID != nil {
            aiService.ArchiveConversation(*context.ActiveAIConversationID)
            context.ActiveAIConversationID = nil
        }

        // Create new AI conversation for this workflow
        initialContext := map[string]any{
            "workflow_name": "CreateTicket",
            "workflow_description": "Guides user through creating ADO work ticket",
            "previous_summary": getArchivedSummaries(context),
        }

        convID, err := aiService.CreateConversation(
            "workflow:createTicket",
            nil,  // No parent
            initialContext,
        )
        if err != nil {
            return nil, err
        }

        // Store conversation ID
        context.ActiveAIConversationID = &convID
        context.GetCurrentWorkflow().WorkflowData["ai_conversation_id"] = convID

        // Initialize workflow conversation
        schema := response_schemas.TicketCreationSchema()
        response, err := aiService.SendMessageWithSchema(
            convID,
            "Greet the user and ask them what kind of ticket they want to create",
            schema,
        )

        // Update summaries
        updateConversationSummaries(context, response)

        return []*core.Action{
            {ActionType: core.ActionUserMessage, Input: map[string]any{"message": response.Response}},
        }, nil
    }

    // ... rest of workflow logic
}
```

### **Example 6: handleDefaultSteps with Side Questions**

File: `internal/workflow/workflow.go`

```go
func handleDefaultSteps(w WorkflowDefinition, c *core.ConversationContext, a *core.Action) ([]*core.Action, bool, error) {
    // Check if workflow opts out of default handling
    if overwrite, ok := w.Options[OptionOverwriteHandleDefaultSteps]; ok && overwrite != false {
        return nil, false, nil  // Don't skip workflow
    }

    step := getInput(a, core.InputStep)

    switch step {
    case StepInit:
        // Workflow handles initialization
        return nil, false, nil

    case StepUserAnsweringQuestion:
        // Workflow handles the answer
        return nil, false, nil

    case StepUserAsksQuestion:
        // Handle side question
        actions, err := handleSideQuestion(w, c, a)
        return actions, true, err  // Skip workflow this turn
    }

    return nil, false, nil
}

func handleSideQuestion(w WorkflowDefinition, c *core.ConversationContext, a *core.Action) ([]*core.Action, error) {
    // Get user's question from last message
    lastMessage := c.GetLastUserMessage()
    if lastMessage == nil {
        return nil, fmt.Errorf("no user message for side question")
    }

    // Create ephemeral AI conversation for side question
    parentConvID := c.ActiveAIConversationID
    if parentConvID == nil {
        return nil, fmt.Errorf("no active AI conversation to reference")
    }

    // Build context for side question
    mainConv, err := aiService.GetConversation(*parentConvID)
    if err != nil {
        return nil, err
    }

    sideContext := map[string]any{
        "workflow_name": c.GetCurrentWorkflow().WorkflowName,
        "current_step": c.GetCurrentWorkflow().Step,
        "recent_activity": mainConv.RecentSummary,
        "workflow_help": w.HelpText,
        "user_question": lastMessage.Message,
    }

    // Create side question conversation
    sideConvID, err := aiService.CreateConversation(
        "side_question",
        parentConvID,  // Parent is workflow's main conversation
        sideContext,
    )
    if err != nil {
        return nil, err
    }

    // Ask AI to answer the question
    schema := response_schemas.SideQuestionSchema()
    response, err := aiService.SendMessageWithSchema(
        sideConvID,
        lastMessage.Message,
        schema,
    )
    if err != nil {
        return nil, err
    }

    // Archive side conversation immediately
    aiService.ArchiveConversation(sideConvID)

    // Return action to send answer to user
    return []*core.Action{
        {
            ActionType: core.ActionUserMessage,
            Input: map[string]any{
                "message": response.Answer,
            },
        },
    }, nil
}
```

### **Example 7: AI Service Interface**

File: `internal/ai/service.go`

```go
package ai

// AIService manages AI conversations and structured outputs
type AIService interface {
    // CreateConversation starts a new AI conversation
    CreateConversation(purpose string, parentID *string, initialContext map[string]any) (string, error)

    // SendMessage sends a message and gets unstructured response
    SendMessage(conversationID string, message string) (string, error)

    // SendMessageWithSchema sends a message and gets structured response
    SendMessageWithSchema(conversationID string, message string, schema *Schema) (map[string]any, error)

    // GetConversation retrieves conversation with current summaries
    GetConversation(conversationID string) (*AIConversation, error)

    // ArchiveConversation marks conversation as archived
    ArchiveConversation(conversationID string) error

    // GetConversationHistory gets recent messages
    GetConversationHistory(conversationID string, lastNTurns int) ([]AIMessage, error)
}

// AIConversation represents a conversation's state
type AIConversation struct {
    ID                     string
    ProviderConversationID string    // OpenAI's conversation/thread ID
    Purpose                string    // "workflow:createTicket", "side_question", etc.
    ParentConversationID   *string   // For side questions

    // Automatically updated with every AI response
    OverallSummary         string
    RecentSummary          string

    Status                 string    // "active", "archived"
    CreatedAt              time.Time
    LastUsedAt             time.Time
}

// AIMessage represents a single message in conversation
type AIMessage struct {
    Role      string    // "user", "assistant", "system"
    Content   string
    Timestamp time.Time
}
```

---

## Data Structure Summary

### **ConversationContext Changes**

```go
type ConversationContext struct {
    // ... existing fields ...

    // AI conversation tracking
    ActiveAIConversationID *string              // Currently active conversation
    ArchivedConversations  []AIConversation     // History of past conversations
}
```

### **WorkflowContext Usage**

```go
// In WorkflowData map
WorkflowData["ai_conversation_id"] = "conv-abc123"  // Main conversation ID
```

### **Database Schema Extension**

The existing `ai_conversations` table can be extended:

```sql
ALTER TABLE ai_conversations
ADD COLUMN purpose VARCHAR(100),
ADD COLUMN parent_conversation_id INT NULL,
ADD COLUMN overall_summary TEXT NULL,
ADD COLUMN recent_summary TEXT NULL,
ADD COLUMN status VARCHAR(20) DEFAULT 'active',
ADD COLUMN last_used_at TIMESTAMP NULL;

ALTER TABLE ai_conversations
ADD FOREIGN KEY (parent_conversation_id) REFERENCES ai_conversations(id) ON DELETE SET NULL;
```

---

## Open Questions & Decisions Needed

### **1. Side Question Context Depth**
How much of main conversation to include in side questions?
- **Option A**: Last 3 turns
- **Option B**: Last 5 turns
- **Option C**: Everything since current step started

**Recommendation**: Start with last 3 turns, make configurable.

### **2. Workflow Metadata**
Should `WorkflowDefinition` include help text for side questions?

```go
type WorkflowDefinition struct {
    Name        WorkflowName
    Description string
    HelpText    string  // NEW: "Title should be brief, under 80 chars..."
    WorkflowFn  func(...)
    Options     map[Option]any
}
```

**Recommendation**: Yes, add `HelpText` field.

### **3. handleDefaultSteps Return Signature**

```go
func handleDefaultSteps(...) (actions []*core.Action, skipWorkflow bool, err error)
```

Where `skipWorkflow=true` means "handled completely, don't call workflow function this turn".

**Agreed**: Use this signature.

### **4. Schema Storage**
Store in `definitions/response_schemas/` as Go files (compiled) or JSON files (runtime loaded)?

**Decision**: Go files - type-safe, compiled, can use builder in init functions.

### **5. Intent Analysis Enhancement**
If AI thinks it needs more context, should it be able to request more messages?

**Recommendation**: Phase 2 enhancement - start with fixed 10 messages.

### **6. Conversation Persistence Strategy**
Persist all conversations or only main workflow conversations?

**Recommendation**:
- Persist main workflow conversations
- Keep side questions and intent analysis in memory only
- Archive side questions to ArchivedConversations in context (not DB)

---

## Next Steps

### **Approach: Workflow-First Development**

**Key decision**: Implement workflows first, then adapt AI/service layers based on what workflows need.

**Rationale**: Workflows define the requirements. Building them with stubs reveals the ideal AI service interface.

---

### **Immediate (This Week) - handleDefaultSteps Foundation**

1. ✅ Document architecture and design (this file)

2. **⏳ NEXT: Implement `handleDefaultSteps` with stubs** (`internal/workflow/workflow.go`)
   - Update signature: `func handleDefaultSteps(...) (actions []*core.Action, skipWorkflow bool, error)`
   - `StepInit`: Return `(nil, false, nil)` - pass through to workflow
   - `StepUserAnsweringQuestion`: Return `(nil, false, nil)` - pass through to workflow
   - `StepUserAsksQuestion`: Return stub action with message "I can't answer side questions yet", `skipWorkflow=true`

3. **Update `RunWorkflow` to use `handleDefaultSteps`** (`internal/workflow/workflow.go`)
   - Call `handleDefaultSteps` before workflow function
   - Respect `skipWorkflow` flag
   - Combine default actions with workflow actions

4. **Add `HelpText` field to `WorkflowDefinition`**
   - Update struct in `workflow.go`
   - Add help text to CreateTicket and QueryTicket workflows

5. **Test the flow**
   - Verify init step passes through to workflow
   - Verify side question returns stub and skips workflow
   - Verify answer step passes through to workflow

### **Short Term (Next 2 Weeks) - One Complete Workflow**

1. **Implement CreateTicket workflow with full StepInit handling**
   - Archive old AI conversation (stub for now)
   - Create new AI conversation (stub returns fake ID)
   - Store conversation ID in WorkflowData
   - Implement basic workflow flow (ask for title → description → severity)
   - **This will reveal what AI service interface needs**

2. **Design AI service interface based on CreateTicket needs**
   - What methods does CreateTicket need?
   - What data structures for conversations?
   - What schema format for structured outputs?

3. **Implement schema builder** (`internal/ai/schema*.go`)
   - Based on what CreateTicket workflow needs for structured outputs
   - SchemaBuilder fluent API
   - Conversion to OpenAI format

4. **Create base response schemas** (`definitions/response_schemas/`)
   - Base response (response, overall_summary, recent_summary)
   - Intent analysis schema
   - Ticket creation schema

5. **Implement mock AI service**
   - Returns canned responses with proper structure
   - Allows testing CreateTicket workflow end-to-end

### **Medium Term (Next 3-4 Weeks) - Real AI Integration**

1. **Update ConversationContext with AI conversation tracking**
   - Add `ActiveAIConversationID` field
   - Add `ArchivedConversations` field
   - Add methods for conversation lifecycle

2. **Implement real `handleSideQuestion` in `handleDefaultSteps`**
   - Create ephemeral AI conversation
   - Build context from workflow + recent activity
   - Call AI service
   - Archive conversation
   - Return answer action

3. **Implement real OpenAI service**
   - API integration
   - Structured output support
   - Conversation management
   - Error handling

4. **Add conversation persistence to database**
   - Extend `ai_conversations` table schema
   - Repository for AI conversations
   - Save/load workflow conversations

5. **Implement intent analysis with AI**
   - Use last 10 Slack messages
   - Intent classification schema
   - Route to appropriate workflow

6. **Test end-to-end with real AI**
   - Full CreateTicket workflow
   - Side questions during workflow
   - Workflow switching
   - Conversation archiving and revival

### **Longer Term (4+ Weeks) - Additional Workflows & Features**

1. Implement QueryTicket workflow
2. Add more personalities (beyond intent_analyzer)
3. Implement tool system and ADO integration
4. Add sub-agent workflows (using async actions)
5. Message coalescing implementation
6. Testing infrastructure

---

## Success Criteria

This design is successful if:

1. ✅ Workflows can define custom response schemas easily (< 10 lines)
2. ✅ AI conversations are scoped appropriately (no bloated context)
3. ✅ Side questions don't interrupt workflow state
4. ✅ Summaries are always available without manual management
5. ✅ Token usage is optimized (summaries > full history when possible)
6. ✅ System is extensible (easy to add new conversation types)
7. ✅ Code is testable (mock AI service works without OpenAI)

---

## References

- Orchestrator action design: `thoughts/orchestrator-action-design.md`
- Implementation tracker: `thoughts/implementation-tracker.md`
- Database research: `thoughts/shared/research/2026-01-01-database-layer-integration.md`
- Workflow implementation: `internal/workflow/`
- OpenAI Structured Outputs: https://platform.openai.com/docs/guides/structured-outputs
