# Bob Testing Roadmap & Feature Analysis

**Last Updated:** 2026-01-25
**Purpose:** Comprehensive analysis of implemented vs missing features, with a structured testing approach before building production workflows.

---

## Table of Contents

1. [Feature Status Analysis](#feature-status-analysis)
2. [Missing Critical Features](#missing-critical-features)
3. [Testing Strategy](#testing-strategy)
4. [Implementation Roadmap](#implementation-roadmap)
5. [Test Workflow Specifications](#test-workflow-specifications)

---

## Feature Status Analysis

### ✅ IMPLEMENTED & WORKING

#### 1. Basic Orchestration Flow
- **Action queue processing**: Sequential action execution with queue management
- **Workflow execution**: RunWorkflow with step-based routing
- **Intent analysis**: AI-powered intent detection with confidence thresholds
- **Context loading/saving**: Database persistence with caching
- **Message routing**: Proper routing based on context state

**Files:**
- `internal/orchestrator/orchestrator.go`
- `internal/orchestrator/intent_analyzer.go`
- `internal/orchestrator/core/context.go`

---

#### 2. AI Integration
- **ActionAI processing**: Complete AI request/response handling
- **Conversation persistence**: Main conversation ID persists across restarts
- **Context isolation**: Support for main + isolated conversation contexts via keys
- **Schema-based outputs**: Structured AI responses using SchemaBuilder

**Files:**
- `internal/orchestrator/process_actions.go` (ActionAI)
- `internal/ai/openai/openai.go`
- `internal/orchestrator/core/workflow_context.go` (GetAIConversation/SetAIConversation)

**Confirmed Working:**
- Conversation continuity across bot restarts ✓
- Side question handling with main conversation ✓
- Workflow-specific isolated conversations ✓

---

#### 3. User Interaction
- **ActionUserMessage**: Non-blocking messages to user (implemented)
- **ActionUserWait**: Blocking wait for user response (implemented, sets StatusWaitForUser)
- **Slack integration**: Full threading support with proper ThreadTS handling

**Files:**
- `internal/slack/handler.go`
- `internal/slack/parser.go`
- `internal/orchestrator/process_actions.go` (ActionUserMessage, ActionUserWait)

---

#### 4. Workflow System
- **Workflow registration**: Map-based registry with descriptions
- **Default step handling**: StepInit, StepUserAsksQuestion, StepUserAnsweringQuestion
- **Side question handling**: Generic handleSideQuestion for all workflows
- **Workflow switching**: Intent AI detects and routes workflow changes

**Files:**
- `internal/workflow/workflow.go`
- `internal/workflow/test_ai.go` (functional example)

**Keywords & Intent Detection:**
- Enhanced workflow descriptions with keywords ✓
- Workflow switch signals documented ✓
- Nuanced decision logic for ambiguous cases ✓

---

#### 5. Data Persistence
- **Database schema**: MySQL tables for users, threads, workflows, context
- **Context persistence**: Saves workflow state, status, request_to_user
- **Workflow state persistence**: Saves workflow_name, step, workflow_data
- **Conversation ID persistence**: Main conversation ID stored in workflow_context

**Files:**
- `definitions/migrations/m0001_schema.sql`
- `definitions/migrations/m0002_add_main_conversation_id.sql`
- `internal/database/workflow_repository.go`
- `internal/database/context_repository.go`

**Fixed Issues:**
- Foreign key constraint bug (workflow ID = 0 treated as UPDATE instead of INSERT) ✓
- Main conversation ID now persists and loads correctly ✓

---

## Missing Critical Features

### ⚠️ PARTIALLY IMPLEMENTED / NEEDS TESTING

#### 1. ActionAsync - Parallel Operations

**Status:** Spawns goroutines but NO result aggregation

**What Exists:**
```go
// action.go
AsyncGroupID   string  // Workflow-generated ID for tracking async groups
AsyncGroupSize int     // Number of expected results in this group

// process_actions.go - ActionAsync spawns goroutines
func ActionAsync(a *Action, ...) {
    for _, subAction := range a.AsyncActions {
        go func(action *Action, index int) {
            newActions, _ := ProcessAction(action, ...)
            for _, newAction := range newActions {
                actionChan <- newAction  // ⚠️ No aggregation!
            }
        }(subAction, i)
    }
    return nil, nil  // ⚠️ Returns immediately!
}
```

**What's Missing:**
- ❌ Wait for all async actions to complete
- ❌ Aggregate results from AsyncGroupID
- ❌ Respect AsyncGroupSize (expected count)
- ❌ Error handling for failed async actions
- ❌ Timeout handling for slow actions
- ❌ Status tracking per async group

**Required Implementation:**
```go
// Need to add to ConversationContext
type AsyncGroup struct {
    ID              string
    ExpectedCount   int
    ReceivedResults []*Action
    Errors          []error
    Complete        bool
}

// Track pending async groups
pendingAsyncGroups map[string]*AsyncGroup

// When ActionWorkflowResult arrives, check if it's part of async group
// If so, add to group, check if complete, then aggregate and continue workflow
```

---

#### 2. Workflow State Management

**Status:** Basic step tracking works, complex flows untested

**What Works:**
- Single-step workflows (like testAI)
- StepInit → AI call → response → done

**What's Untested:**
- Multi-step workflows with branching logic
- Resumption after ActionUserWait with state preservation
- Error recovery paths (what happens when step fails?)
- Progress tracking through 5+ step workflows
- Conditional branching based on AI responses

**Example Untested Pattern:**
```go
func ComplexWorkflow(ctx, action) {
    step := getInput(action, InputStep)
    switch step {
    case "init":
        // Ask user for input
        return askUserWait("What project?")
    case "got_project":
        // Validate project
        return callTool("ValidateProject", project)
    case "validated":
        // Branch: valid or invalid?
        if valid {
            return []Action{...next steps}
        } else {
            return askUserWait("Invalid project. Try again?")
        }
    // ... more steps
    }
}
```

---

#### 3. Context Isolation

**Status:** Can create isolated AI conversations, but untested in complex scenarios

**What Works:**
- Main conversation (conversationKey="")
- Isolated conversations (conversationKey="custom")

**What's Untested:**
- Do parallel workflows (different users) maintain separate contexts?
- Does sub-workflow isolation work correctly?
- Can parent workflow access child workflow's isolated context?
- Cleanup: Do isolated contexts get cleaned up after workflow completes?
- Memory leaks: Unlimited isolated contexts in workflowData?

---

### ❌ NOT IMPLEMENTED / STUBBED

#### 1. ActionTool - Tool/Function Calling

**Status:** Returns `nil, nil` - completely stubbed

**Current Implementation:**
```go
func ActionTool(a *Action, ctx *ConversationContext, ...) ([]*Action, error){
    return nil, nil  // ❌ Stub
}
```

**Everything Missing:**
- Tool registry (registration system)
- Tool definition structure (name, description, schema, handler)
- Tool execution (calling the handler)
- Tool result handling (returning result to workflow)
- AI requesting tools (structured output with tool_name + params)
- Tool permission system (which workflows can use which tools)
- Error handling (tool not found, execution failed, timeout)

**Required Structures:**
```go
type Tool struct {
    Name        string
    Description string
    InputSchema *SchemaBuilder
    Handler     func(ctx context.Context, input map[string]any) (any, error)
    Permissions []string  // Which workflows can use this tool
}

var toolRegistry = map[string]*Tool{}

func RegisterTool(tool *Tool) { ... }
func ExecuteTool(name string, input map[string]any) (any, error) { ... }
```

---

#### 2. Real Workflows

**Status:** Only testAI is functional

**CreateTicket:** Stubbed with design notes
```go
/*
CreateTicket interogates the users to be able to create a well refined ticket.
What does the AI need to know?
- What project (Ask user)
- The project context (Research Project)
- The project guidelines (Read files?)
- Other details that should be included in the ticket (conversation with user)
*/
func CreateTicket(...) ([]*Action, error){
    _ = getInput(sourceAction, core.InputStep) // TODO: Use step
    return nil, nil
}
```

**QueryTicket:** Completely empty
```go
func QueryTicket(...) ([]*Action, error){
    return nil, nil
}
```

---

#### 3. Sub-Workflow Orchestration

**Status:** No implementation at all

**Missing Concepts:**
- Spawning child workflows from parent workflows
- Parent-child workflow communication
- Context inheritance vs isolation (child shares parent's context or has own?)
- Sub-workflow results bubbling up to parent
- Nested sub-workflows (grandchildren)
- Sub-workflow error propagation

**Example Use Case:**
```
CreateTicketWorkflow:
  1. Ask user for project
  2. Spawn SubWorkflow: ResearchProjectContext
     - Search ADO for similar tickets
     - Read project guidelines
     - Return: project context summary
  3. Ask AI to suggest ticket details (with project context)
  4. Ask user to confirm
  5. Spawn SubWorkflow: CreateADOTicket
     - Call ADO API
     - Return: ticket URL
  6. Send success message to user
```

---

#### 4. Advanced Features (Not Needed Yet)

- **Intent clarification flow:** Marked as TODO in `intent_analyzer.go`
- **Burst message handling:** TODO in `HandleUserMessage`
- **Priority action queue:** Currently FIFO, no priority
- **Workflow composition/chaining:** No support for workflow A → workflow B transitions
- **Rollback/undo mechanisms:** No way to undo workflow steps
- **Rate limiting:** No protection against API abuse
- **Audit logging:** No structured logging of workflow decisions

---

## Testing Strategy

### Philosophy: Test Plumbing Before Building Real Workflows

**Rationale:**
- Don't build complex workflows on untested infrastructure
- Each missing feature is a potential bug multiplier
- Tools, async, and sub-workflows are the foundation
- Once foundation is solid, everything else is straightforward

### Three-Phase Approach

---

## Phase 1: Core Infrastructure Testing (Critical, Do First)

### Test 1.1: Tool System Implementation & Testing

**Goal:** Build and test the complete tool calling system

**Implementation Tasks:**

1. **Create Tool Registry**
   ```go
   // internal/tools/registry.go
   type Tool struct {
       Name        string
       Description string
       InputSchema *ai.SchemaBuilder
       Handler     func(ctx context.Context, input map[string]any) (any, error)
   }

   var registry = make(map[string]*Tool)

   func RegisterTool(tool *Tool) error { ... }
   func GetTool(name string) (*Tool, error) { ... }
   func ListTools() []*Tool { ... }
   ```

2. **Register Test Tools**
   ```go
   // internal/tools/test_tools.go
   - CalculatorTool(operation, a, b) → result
   - WeatherTool(city) → mock weather data
   - TimezoneTool(timezone) → current time
   - RandomNumberTool(min, max) → random int
   - EchoTool(message) → echoes message back
   ```

3. **Implement ActionTool Handler**
   ```go
   // internal/orchestrator/process_actions.go
   func ActionTool(a *Action, ctx *ConversationContext, ...) ([]*Action, error) {
       toolName := a.Input["tool_name"].(string)
       toolInput := a.Input["tool_input"].(map[string]any)

       tool, err := tools.GetTool(toolName)
       if err != nil {
           return nil, fmt.Errorf("tool not found: %s", toolName)
       }

       result, err := tool.Handler(context.Background(), toolInput)
       if err != nil {
           return nil, fmt.Errorf("tool execution failed: %w", err)
       }

       // Return result to workflow
       resultAction := core.NewAction(core.ActionWorkflowResult)
       resultAction.Input[core.InputToolResult] = result
       return []*Action{resultAction}, nil
   }
   ```

4. **Create Test Workflow: test_tools.go**
   ```go
   func TestTools(context, sourceAction) {
       step := getInput(sourceAction, InputStep)
       switch step {
       case StepInit:
           // Ask AI which tool to use
           return askAI("What's 5 + 3?", "You have calculator tool", ...)

       default:
           // Check if AI response suggests tool
           if toolRequest := extractToolRequest(aiResponse); toolRequest != nil {
               return callTool(toolRequest.Name, toolRequest.Params)
           }
           // Otherwise send AI response to user
           return sendToUser(aiResponse.Message)
       }
   }
   ```

**Test Scenarios:**

| Test | Description | Expected Result |
|------|-------------|----------------|
| Direct tool call | Workflow directly requests calculator tool | Tool executes, returns result |
| AI suggests tool | AI response contains tool suggestion | Tool executes, result sent to AI |
| Tool not found | Request non-existent tool | Error returned, workflow handles |
| Tool execution fails | Tool handler returns error | Error propagated to workflow |
| Sequential tools | Call 3 tools in sequence | All execute in order |
| Tool result in AI | Tool result passed back to AI | AI incorporates result in response |

**Success Criteria:**
- ✓ All test tools registered successfully
- ✓ Direct tool calls work
- ✓ AI can suggest tools (if schema supports it)
- ✓ Tool results return to workflow correctly
- ✓ Error handling works for missing/failed tools

---

### Test 1.2: Async/Parallel Action System

**Goal:** Make async action aggregation work properly

**Implementation Tasks:**

1. **Add Async Group Tracking to Context**
   ```go
   // internal/orchestrator/core/context.go
   type AsyncGroup struct {
       ID              string
       ExpectedCount   int
       ReceivedResults []*Action
       Errors          []error
       StartTime       time.Time
   }

   type ConversationContext struct {
       // ... existing fields
       pendingAsyncGroups map[string]*AsyncGroup
       mu                 sync.RWMutex
   }

   func (c *ConversationContext) RegisterAsyncGroup(groupID string, count int) { ... }
   func (c *ConversationContext) AddAsyncResult(groupID string, result *Action) bool { ... }
   func (c *ConversationContext) IsAsyncGroupComplete(groupID string) bool { ... }
   func (c *ConversationContext) GetAsyncGroupResults(groupID string) ([]*Action, []error) { ... }
   ```

2. **Update ActionAsync Implementation**
   ```go
   // internal/orchestrator/process_actions.go
   func ActionAsync(a *Action, ctx *ConversationContext, ...) ([]*Action, error) {
       groupID := a.AsyncGroupID
       if groupID == "" {
           return nil, fmt.Errorf("async action missing AsyncGroupID")
       }

       // Register async group
       ctx.RegisterAsyncGroup(groupID, len(a.AsyncActions))

       // Spawn goroutines
       for i, subAction := range a.AsyncActions {
           go func(action *Action, index int) {
               newActions, err := ProcessAction(action, ctx, responder, actionChan)

               // Mark with group ID
               for _, newAction := range newActions {
                   newAction.AsyncGroupID = groupID
                   actionChan <- newAction
               }

               if err != nil {
                   // Send error marker
                   errorAction := core.NewAction(core.ActionWorkflowResult)
                   errorAction.AsyncGroupID = groupID
                   errorAction.Input[core.InputError] = err
                   actionChan <- errorAction
               }
           }(subAction, i)
       }

       return nil, nil
   }
   ```

3. **Update Orchestrator to Handle Async Results**
   ```go
   // internal/orchestrator/orchestrator.go - StartHandlingActions

   // When processing ActionWorkflowResult, check if it's part of async group
   if action.AsyncGroupID != "" {
       complete := context.AddAsyncResult(action.AsyncGroupID, action)
       if !complete {
           continue // Wait for more results
       }

       // All results received, aggregate and send to workflow
       results, errors := context.GetAsyncGroupResults(action.AsyncGroupID)

       aggregatedAction := core.NewAction(core.ActionWorkflowResult)
       aggregatedAction.Input[core.InputAsyncResults] = results
       aggregatedAction.Input[core.InputAsyncErrors] = errors

       actionQueue = append(actionQueue, aggregatedAction)
   }
   ```

4. **Create Test Workflow: test_async.go**
   ```go
   func TestAsync(context, sourceAction) {
       step := getInput(sourceAction, InputStep)
       switch step {
       case StepInit:
           // Request 3 AI calls in parallel
           groupID := uuid.New().String()
           asyncAction := core.NewAction(core.ActionAsync)
           asyncAction.AsyncGroupID = groupID
           asyncAction.AsyncActions = []*Action{
               createAIAction("What is 2+2?"),
               createAIAction("What is 3+3?"),
               createAIAction("What is 4+4?"),
           }
           return []*Action{asyncAction}, nil

       default:
           // Aggregate async results
           results := getInput(sourceAction, InputAsyncResults)
           // Combine all AI responses
           combined := aggregateResults(results)
           return sendToUser(combined)
       }
   }
   ```

**Test Scenarios:**

| Test | Description | Expected Result |
|------|-------------|----------------|
| 3 parallel AI calls | Request 3 AI calls simultaneously | All complete, results aggregated |
| 5 parallel tool calls | Request 5 tools simultaneously | All execute, results returned |
| Mixed parallel | 2 AI + 3 tools in parallel | All complete correctly |
| One action fails | 3 actions, 1 fails | Error captured, 2 succeed |
| Timeout handling | 1 action takes 30s | Workflow handles timeout |
| Nested async | Async action spawns more async | All levels complete |

**Success Criteria:**
- ✓ Async group registration works
- ✓ All parallel actions execute
- ✓ Results aggregate correctly
- ✓ Workflow resumes after all complete
- ✓ Errors don't block other actions
- ✓ No race conditions or deadlocks

---

### Test 1.3: Sub-Workflow System

**Goal:** Implement and test workflow composition

**Implementation Tasks:**

1. **Create ActionSubWorkflow**
   ```go
   // internal/orchestrator/core/action.go
   const (
       // ... existing actions
       ActionSubWorkflow  // Spawn child workflow
   )

   // action.go
   type Action struct {
       // ... existing fields
       SubWorkflowName    string  // Name of sub-workflow to spawn
       SubWorkflowInput   map[string]any  // Input data to pass
       SubWorkflowIsolate bool    // Use isolated context?
   }
   ```

2. **Implement ActionSubWorkflow Handler**
   ```go
   // internal/orchestrator/process_actions.go
   func ActionSubWorkflow(a *Action, ctx *ConversationContext, ...) ([]*Action, error) {
       workflowName := a.SubWorkflowName
       input := a.SubWorkflowInput
       isolate := a.SubWorkflowIsolate

       // Create child workflow context
       var childCtx *ConversationContext
       if isolate {
           childCtx = createIsolatedContext(ctx, workflowName)
       } else {
           childCtx = ctx  // Share parent context
       }

       // Set up child workflow
       childCtx.SetCurrentWorkflow(core.NewWorkflow(workflowName))

       // Create init action for child
       initAction := core.NewAction(core.ActionWorkflow)
       initAction.Input[core.InputStep] = workflow.StepInit
       initAction.Input[core.InputSubWorkflowInput] = input

       // Execute child workflow synchronously (for now)
       result, err := workflow.RunWorkflow(childCtx, initAction)

       // Return child result to parent
       resultAction := core.NewAction(core.ActionWorkflowResult)
       resultAction.Input[core.InputSubWorkflowResult] = result
       return []*Action{resultAction}, err
   }
   ```

3. **Create Helper Functions in Workflow Layer**
   ```go
   // internal/workflow/workflow_funcs.go
   func callSubWorkflow(name string, input map[string]any, isolate bool) *Action {
       action := core.NewAction(core.ActionSubWorkflow)
       action.SubWorkflowName = name
       action.SubWorkflowInput = input
       action.SubWorkflowIsolate = isolate
       return action
   }
   ```

4. **Create Test Workflows**
   ```go
   // internal/workflow/test_subworkflow.go

   // Parent workflow
   func TestParentWorkflow(context, sourceAction) {
       step := getInput(sourceAction, InputStep)
       switch step {
       case StepInit:
           // Spawn child workflow
           return []*Action{
               callSubWorkflow("testChildWorkflow", map[string]any{
                   "task": "calculate",
                   "numbers": []int{1, 2, 3},
               }, true),  // isolated context
           }, nil

       default:
           // Get child result
           childResult := getInput(sourceAction, InputSubWorkflowResult)
           return sendToUser(fmt.Sprintf("Child returned: %v", childResult))
       }
   }

   // Child workflow
   func TestChildWorkflow(context, sourceAction) {
       step := getInput(sourceAction, InputStep)
       switch step {
       case StepInit:
           input := getInput(sourceAction, InputSubWorkflowInput)
           task := input["task"]
           numbers := input["numbers"]

           // Do some work
           sum := calculateSum(numbers)

           // Return result
           return []*Action{
               createResult(map[string]any{"sum": sum}),
           }, nil
       }
   }
   ```

**Test Scenarios:**

| Test | Description | Expected Result |
|------|-------------|----------------|
| Parent → Child | Parent spawns child, waits for result | Child completes, parent receives result |
| Isolated context | Child has isolated AI conversation | Child and parent have separate AI contexts |
| Shared context | Child shares parent AI conversation | Both use same AI conversation |
| Nested 3 levels | Parent → Child → Grandchild | All complete, results bubble up |
| Child fails | Child workflow returns error | Parent receives error, handles it |
| Parallel children | Parent spawns 3 children in parallel | All complete, results aggregated |

**Success Criteria:**
- ✓ Sub-workflows can be spawned
- ✓ Input data passes correctly
- ✓ Results return to parent
- ✓ Isolated contexts work
- ✓ Shared contexts work
- ✓ Nesting works (3+ levels)
- ✓ Error propagation works

---

## Phase 2: Integration Testing (After Phase 1)

### Test 2.1: Complex Integration Workflow

**Goal:** Test all features working together

**Create "ResearchAndSummarize" Workflow:**

```go
func ResearchAndSummarize(context, sourceAction) {
    step := getInput(sourceAction, InputStep)

    switch step {
    case StepInit:
        // 1. Ask AI for research topics
        return askAI("Generate 3 research topics about Go concurrency", ...)

    case "got_topics":
        topics := extractTopics(aiResponse)

        // 2. For each topic, spawn parallel research
        asyncAction := core.NewAction(core.ActionAsync)
        asyncAction.AsyncGroupID = "research_group"

        for _, topic := range topics {
            // Each spawns a sub-workflow
            asyncAction.AsyncActions = append(asyncAction.AsyncActions,
                callSubWorkflow("researchTopic", map[string]any{
                    "topic": topic,
                }, true),  // isolated
            )
        }
        return []*Action{asyncAction}, nil

    case "got_results":
        // 3. Aggregate all findings
        results := getInput(sourceAction, InputAsyncResults)

        // 4. Ask AI to summarize with all context
        return askAI(
            fmt.Sprintf("Summarize these findings: %v", results),
            "You are a technical writer",
            "",
        )

    case "got_summary":
        // 5. Send to user
        return sendToUser(extractMessage(aiResponse))
    }
}

func ResearchTopic(context, sourceAction) {
    topic := getInput(sourceAction, InputSubWorkflowInput)["topic"]

    // Call WebSearch tool
    toolResult := callTool("WebSearch", map[string]any{"query": topic})

    // Ask AI to analyze
    analysisResult := askAI(
        fmt.Sprintf("Analyze these search results: %v", toolResult),
        "",
        "",
    )

    return createResult(analysisResult)
}
```

**Test Scenarios:**
- All async operations complete
- Sub-workflows maintain isolated contexts
- Tools execute correctly
- Final aggregation works
- Error in one branch doesn't kill whole workflow

---

### Test 2.2: Context Isolation Testing

**Goal:** Verify contexts stay separate when needed

**Test Cases:**

1. **Multi-User Test**
   - User A starts workflow X
   - User B starts workflow Y
   - Both workflows run simultaneously
   - Verify: contexts don't mix

2. **Parent-Child Isolation**
   - Parent workflow with AI conversation
   - Spawns child with isolated context
   - Child makes AI calls
   - Verify: child's AI conversation separate from parent

3. **Parallel Branch Sharing**
   - Workflow spawns 3 parallel branches
   - All branches share same context
   - Verify: all see same AI conversation

4. **Conversation ID Cleanup**
   - Workflow creates 10 isolated conversations
   - Workflow completes
   - Verify: isolated conversations cleaned up (no memory leak)

---

### Test 2.3: State Persistence & Recovery

**Goal:** Test restart scenarios

**Test Cases:**

1. **Mid-Execution Persistence**
   - Start multi-step workflow
   - After step 3, save state
   - Verify: DB contains step 3 state

2. **Restart & Resume**
   - Workflow at step 3
   - Restart Bob
   - Send next message
   - Verify: workflow resumes from step 3

3. **ActionUserWait Persistence**
   - Workflow asks user a question (ActionUserWait)
   - Restart Bob
   - User answers
   - Verify: workflow continues correctly

4. **Async Groups During Restart**
   - Workflow spawns async actions
   - Restart Bob (actions in flight)
   - Verify: graceful handling (either retry or fail gracefully)

---

## Phase 3: Production Workflow Building (After Phases 1 & 2)

Only after all tests pass:

1. **CreateTicket Workflow**
   - Multi-step user interrogation
   - Project context research (sub-workflow)
   - AI-assisted ticket creation
   - ADO API integration (tool)

2. **QueryTicket Workflow**
   - Parse user query
   - Search ADO (tool)
   - Format results
   - Present to user

3. **Other Business Workflows**
   - Build on tested foundation
   - Leverage tools, async, sub-workflows

---

## Implementation Roadmap

### Week 1: Tools (Critical Foundation)
**Estimated Effort:** 1-2 days

- [ ] Create tool registry system
- [ ] Implement ActionTool handler
- [ ] Register 5 test tools (calculator, weather, timezone, random, echo)
- [ ] Create test_tools.go workflow
- [ ] Test all scenarios
- [ ] Document tool creation pattern

**Output:** Tools working end-to-end

---

### Week 2: Async Aggregation (Performance Critical)
**Estimated Effort:** 2-3 days

- [ ] Add AsyncGroup tracking to ConversationContext
- [ ] Update ActionAsync to register groups
- [ ] Update orchestrator to aggregate results
- [ ] Add timeout handling
- [ ] Create test_async.go workflow
- [ ] Test all scenarios (including failure cases)
- [ ] Performance testing (100 parallel actions)

**Output:** Async actions aggregate correctly

---

### Week 3: Sub-Workflows (Composition)
**Estimated Effort:** 3-4 days

- [ ] Create ActionSubWorkflow action type
- [ ] Implement child workflow spawning
- [ ] Implement context isolation logic
- [ ] Add result bubbling
- [ ] Create test_subworkflow.go workflows (parent + child)
- [ ] Test all scenarios (including nesting)
- [ ] Document sub-workflow patterns

**Output:** Sub-workflows fully functional

---

### Week 4: Integration Testing
**Estimated Effort:** 2-3 days

- [ ] Build ResearchAndSummarize integration test
- [ ] Multi-user context isolation tests
- [ ] State persistence & recovery tests
- [ ] Load testing (10 concurrent workflows)
- [ ] Bug fixing from integration tests
- [ ] Documentation updates

**Output:** All systems tested together, bugs fixed

---

### Week 5+: Production Workflows

Build real workflows with confidence:
- CreateTicket
- QueryTicket
- Custom business logic

---

## Test Workflow Specifications

### File Structure

```
internal/workflow/
├── test_tools.go           # Tool calling tests
├── test_async.go           # Parallel action tests
├── test_subworkflow.go     # Sub-workflow tests
├── test_integration.go     # Full integration test
└── workflow.go             # Register test workflows
```

### Workflow Registration

```go
// internal/workflow/workflow.go
const (
    WorkflowTestTools       WorkflowName = "testTools"
    WorkflowTestAsync       WorkflowName = "testAsync"
    WorkflowTestSubWorkflow WorkflowName = "testSubWorkflow"
    WorkflowTestIntegration WorkflowName = "testIntegration"
)

var workflows = map[WorkflowName]WorkflowDefinition{
    // ... existing workflows
    WorkflowTestTools: {
        Description: "Test tool calling system with calculator, weather, and other test tools. Keywords: test tools, calculator.",
        WorkflowFn:  TestTools,
    },
    WorkflowTestAsync: {
        Description: "Test async/parallel action execution and result aggregation. Keywords: test async, parallel.",
        WorkflowFn:  TestAsync,
    },
    // ... etc
}
```

### Invoking Test Workflows

From Slack:
- "I want to test tools"
- "I want to test async"
- "I want to test sub workflows"
- "I want to test integration"

---

## Success Metrics

### Definition of Done for Phase 1

- [ ] All test workflows registered and invokable
- [ ] Tools: 5 test tools working, all scenarios pass
- [ ] Async: 3+ parallel actions aggregate correctly, no race conditions
- [ ] Sub-workflows: Parent-child communication works, isolation works
- [ ] Error handling: All failure scenarios handled gracefully
- [ ] Performance: No memory leaks, reasonable speed
- [ ] Documentation: All patterns documented

### Definition of Done for Phase 2

- [ ] Integration workflow completes successfully
- [ ] Context isolation verified (no cross-contamination)
- [ ] State persistence verified (restart works)
- [ ] Load testing: 10 concurrent workflows stable
- [ ] All bugs from integration testing fixed

### Definition of Done for Phase 3

- [ ] CreateTicket workflow production-ready
- [ ] QueryTicket workflow production-ready
- [ ] Real ADO API integration working
- [ ] User testing completed
- [ ] Monitoring/logging in place

---

## Appendix: Key Files Reference

### Core Orchestration
- `internal/orchestrator/orchestrator.go` - Main orchestration loop
- `internal/orchestrator/process_actions.go` - Action handlers
- `internal/orchestrator/intent_analyzer.go` - Intent detection

### Workflows
- `internal/workflow/workflow.go` - Workflow registry and default handling
- `internal/workflow/test_ai.go` - Example functional workflow

### Data Structures
- `internal/orchestrator/core/action.go` - Action types and structures
- `internal/orchestrator/core/context.go` - Conversation context
- `internal/orchestrator/core/workflow_context.go` - Workflow state

### Database
- `internal/database/workflow_repository.go` - Workflow persistence
- `internal/database/context_repository.go` - Context persistence
- `definitions/migrations/` - Database schema

---

## Questions & Decisions Log

**Q: Should async timeout be configurable per workflow?**
A: TBD - start with global timeout, make configurable if needed

**Q: How to handle sub-workflow failures?**
A: TBD - return error to parent, let parent decide (retry, abort, fallback)

**Q: Limit on nested sub-workflow depth?**
A: TBD - suggest max 5 levels, detect cycles

**Q: Should tools be workflow-scoped or global?**
A: TBD - start global, add permissions layer later

---

**Document Status:** Living document - update as implementation progresses
**Next Review:** After Phase 1 completion
