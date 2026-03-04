// Package workflow tests the queryTicket workflow with deterministic mock AI responses.
//
// # Deterministic AI Testing
//
// AI models are probabilistic — the same prompt can yield different outputs on each call.
// This makes naive end-to-end tests flaky and impossible to assert on reliably.
//
// The solution: separate two distinct concerns.
//
//  1. AI QUALITY — "does the model pick the right branch for this input?"
//     This is non-deterministic. Test it manually, via prompt evaluation frameworks,
//     or with temperature=0 integration tests. Not covered here.
//
//  2. WORKFLOW LOGIC — "given the AI returns branch X, does the system behave correctly?"
//     This IS deterministic and is what every test below verifies.
//
// The aimock package replaces the AI provider with a queue-based mock.
// Tests inject the exact AI response they want, then assert on the actions and
// state the workflow produces — no network calls, no variance, fully reproducible.
//
// Example mindset shift:
//
//	"Will AI say branch=present_ticket for this query?"  ← non-deterministic, skip
//	"IF AI says branch=present_ticket with id=1234,
//	  does the workflow dispatch ado_get_ticket for 1234?" ← deterministic, test this
package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"bob/internal/ai"
	"bob/internal/ai/aimock"
	"bob/internal/orchestrator/core"
	"bob/internal/tool"
)

// =============================================================================
// Test helpers
// =============================================================================

// qtNewCtx creates a ConversationContext with the queryTicket workflow active.
func qtNewCtx(msgs ...string) (*core.ConversationContext, *core.WorkflowContext) {
	ctx := core.NewConversationContext()
	wf := core.NewWorkflow(string(WorkflowQueryTicket))
	ctx.SetCurrentWorkflow(wf)
	if len(msgs) > 0 {
		messages := make([]*core.Message, len(msgs))
		for i, m := range msgs {
			messages[i] = &core.Message{Message: m}
		}
		ctx.SetLastUserMessages(messages)
	}
	return ctx, wf
}

// qtSearcherCtx creates a context for the queryTicketSearcher sub-workflow.
func qtSearcherCtx() (*core.ConversationContext, *core.WorkflowContext) {
	ctx := core.NewConversationContext()
	wf := core.NewWorkflow(string(WorkflowQueryTicketSearcher))
	ctx.SetCurrentWorkflow(wf)
	return ctx, wf
}

// qtAIAction creates an action carrying an AI response.
// This simulates what the orchestrator creates after a real ai.SendMessage call.
func qtAIAction(step string, data map[string]any) *core.Action {
	a := core.NewAction(core.ActionWorkflowResult)
	a.Input = map[core.InputType]any{
		core.InputStep:       step,
		core.InputAIResponse: ai.NewResponse(data, "mock-conv", "mock-resp", "mock-model", "stop", 0),
	}
	return a
}

// qtToolResultAction creates an action carrying a tool result.
func qtToolResultAction(step string, data map[string]any) *core.Action {
	a := core.NewAction(core.ActionWorkflowResult)
	a.Input = map[core.InputType]any{
		core.InputStep:       step,
		core.InputToolResult: data,
	}
	return a
}

// qtStepAction creates a plain workflow step action (no payload).
func qtStepAction(step string) *core.Action {
	a := core.NewAction(core.ActionWorkflow)
	a.Input = map[core.InputType]any{core.InputStep: step}
	return a
}

// qtSubWorkerAction creates a sub-worker action with a worker ID and message payload.
func qtSubWorkerAction(step, workerID, message string) *core.Action {
	a := core.NewAction(core.ActionWorkflow)
	a.Input = map[core.InputType]any{
		core.InputStep:        step,
		core.InputSubWorkerID: workerID,
		core.InputMessage:     message,
	}
	return a
}

// qtSubWorkerToolResultAction creates an action with a tool result + worker ID for the searcher.
func qtSubWorkerToolResultAction(step, workerID string, data map[string]any) *core.Action {
	a := core.NewAction(core.ActionWorkflowResult)
	a.Input = map[core.InputType]any{
		core.InputStep:        step,
		core.InputSubWorkerID: workerID,
		core.InputToolResult:  data,
	}
	return a
}

// strategiesJSON builds a valid strategies_json string with N workers.
func strategiesJSON(count int) string {
	strategies := make([]map[string]any, count)
	for i := 0; i < count; i++ {
		strategies[i] = map[string]any{
			"worker_id":    fmt.Sprintf("%d", i+1),
			"angle":        fmt.Sprintf("angle %d", i+1),
			"title":        fmt.Sprintf("keyword%d", i+1),
			"max_results":  15,
			"assigned_to":  "",
			"created_by":   "",
			"qa_person":    "",
			"work_item_type": "",
			"state":        "",
			"tags":         []string{},
			"area_path":    "",
			"iteration_path": "",
		}
	}
	b, _ := json.Marshal(strategies)
	return string(b)
}

// requireLen fails if len(actions) != want.
func requireLen(t *testing.T, actions []*core.Action, want int) {
	t.Helper()
	if len(actions) != want {
		t.Fatalf("len(actions) = %d, want %d", len(actions), want)
	}
}

// requireAtLeast fails if len(actions) < min.
func requireAtLeast(t *testing.T, actions []*core.Action, min int) {
	t.Helper()
	if len(actions) < min {
		t.Fatalf("len(actions) = %d, want >= %d", len(actions), min)
	}
}

// requireType fails if actions[i].ActionType != want.
func requireType(t *testing.T, actions []*core.Action, i int, want core.ActionType) {
	t.Helper()
	if i >= len(actions) {
		t.Fatalf("actions[%d]: only %d actions exist", i, len(actions))
	}
	if got := actions[i].ActionType; got != want {
		t.Errorf("actions[%d].ActionType = %v, want %v", i, got, want)
	}
}

// requireStep fails if actions[i]'s step input != want.
func requireStep(t *testing.T, actions []*core.Action, i int, want string) {
	t.Helper()
	if i >= len(actions) {
		t.Fatalf("actions[%d]: only %d actions exist", i, len(actions))
	}
	got, _ := actions[i].Input[core.InputStep].(string)
	if got != want {
		t.Errorf("actions[%d] step = %q, want %q", i, got, want)
	}
}

// requireToolName fails if actions[i]'s tool name != want.
func requireToolName(t *testing.T, actions []*core.Action, i int, want tool.ToolName) {
	t.Helper()
	if i >= len(actions) {
		t.Fatalf("actions[%d]: only %d actions exist", i, len(actions))
	}
	got, _ := actions[i].Input[core.InputToolName].(tool.ToolName)
	if got != want {
		t.Errorf("actions[%d] tool = %q, want %q", i, got, want)
	}
}

// requireMessageContains fails if actions[i]'s message doesn't contain substr.
func requireMessageContains(t *testing.T, actions []*core.Action, i int, substr string) {
	t.Helper()
	if i >= len(actions) {
		t.Fatalf("actions[%d]: only %d actions exist", i, len(actions))
	}
	msg, _ := actions[i].Input[core.InputMessage].(string)
	if !strings.Contains(msg, substr) {
		t.Errorf("actions[%d] message = %q\n  want it to contain %q", i, msg, substr)
	}
}

// requireWFString checks a workflow data string value.
func requireWFString(t *testing.T, wf *core.WorkflowContext, key, want string) {
	t.Helper()
	got, _ := wf.GetWorkflowData(key).(string)
	if got != want {
		t.Errorf("workflowData[%q] = %q, want %q", key, got, want)
	}
}

// requireWFInt checks a workflow data int value.
func requireWFInt(t *testing.T, wf *core.WorkflowContext, key string, want int) {
	t.Helper()
	got, _ := wf.GetWorkflowData(key).(int)
	if got != want {
		t.Errorf("workflowData[%q] = %d, want %d", key, got, want)
	}
}

// =============================================================================
// StepInit tests
// =============================================================================

func TestQT_Init_ReturnsAIActionForClarify(t *testing.T) {
	ctx, _ := qtNewCtx("find the payment timeout bug")

	actions, err := QueryTicket(ctx, qtStepAction(StepInit))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)
	requireStep(t, actions, 0, StepQtClarify)
}

func TestQT_Init_ResetsExistingWorkflowData(t *testing.T) {
	ctx, wf := qtNewCtx("find a bug")
	// Pre-populate state from a previous search
	wf.SetWorkflowData(qtKeyAttemptCount, 3)
	wf.SetWorkflowData(qtKeyKeywords, "old keywords")
	wf.SetWorkflowData(qtKeyPendingStep, "qt_exhausted")

	_, err := QueryTicket(ctx, qtStepAction(StepInit))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All workflow data should be cleared
	if v := wf.GetWorkflowData(qtKeyAttemptCount); v != nil {
		t.Errorf("attempt count should be cleared after init, got %v", v)
	}
	if v := wf.GetWorkflowData(qtKeyKeywords); v != nil {
		t.Errorf("keywords should be cleared after init, got %v", v)
	}
}

func TestQT_Init_ErrorIfNoUserMessages(t *testing.T) {
	ctx, _ := qtNewCtx() // no messages

	_, err := QueryTicket(ctx, qtStepAction(StepInit))
	if err == nil {
		t.Fatal("expected error when no user messages, got nil")
	}
}

// =============================================================================
// StepQtClarify tests
// =============================================================================

func TestQT_Clarify_NoClarify_PlansSearch(t *testing.T) {
	ctx, _ := qtNewCtx("find the payment timeout defect")

	// AI says no clarification needed
	action := qtAIAction(StepQtClarify, map[string]any{
		"should_clarify": false,
		"keywords":       []any{"payment", "timeout"},
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// qtPlanSearch returns UserMessage("Let me search...") + ActionAi(StepQtSpawnWorkers)
	requireAtLeast(t, actions, 2)
	requireType(t, actions, 0, core.ActionUserMessage)
	requireMessageContains(t, actions, 0, "search")
	requireType(t, actions, 1, core.ActionAi)
	requireStep(t, actions, 1, StepQtSpawnWorkers)
}

func TestQT_Clarify_AsksClarifyQuestion(t *testing.T) {
	ctx, wf := qtNewCtx("find that ticket")

	action := qtAIAction(StepQtClarify, map[string]any{
		"should_clarify":     true,
		"clarifying_question": "Do you remember who it was assigned to?",
		"keywords":           []any{},
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "assigned to")
	requireWFString(t, wf, qtKeyPendingStep, "qt_clarify_asked")
}

func TestQT_Clarify_DefaultClarifyQuestion_WhenEmpty(t *testing.T) {
	ctx, _ := qtNewCtx("find that ticket")

	action := qtAIAction(StepQtClarify, map[string]any{
		"should_clarify":     true,
		"clarifying_question": "", // empty — should use fallback
		"keywords":           []any{},
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	// Fallback question mentions details
	msg, _ := actions[0].Input[core.InputMessage].(string)
	if msg == "" {
		t.Error("expected a non-empty fallback clarifying question")
	}
}

func TestQT_Clarify_StoresExtractedKeywords(t *testing.T) {
	ctx, wf := qtNewCtx("find the payment timeout bug assigned to Sarah")

	action := qtAIAction(StepQtClarify, map[string]any{
		"should_clarify": false,
		"keywords":       []any{"payment", "timeout"},
		"assigned_to":    "Sarah",
		"work_item_type": "Defect",
	})

	_, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kw, _ := wf.GetWorkflowData(qtKeyKeywords).(string)
	if !strings.Contains(kw, "payment") || !strings.Contains(kw, "timeout") {
		t.Errorf("keywords = %q, want to contain 'payment' and 'timeout'", kw)
	}
	requireWFString(t, wf, qtKeyAssignedTo, "Sarah")
	requireWFString(t, wf, qtKeyWorkItemType, "Defect")
}

func TestQT_Clarify_StoresPersonAndAreaPath(t *testing.T) {
	ctx, wf := qtNewCtx("find RMS ticket created by John")

	action := qtAIAction(StepQtClarify, map[string]any{
		"should_clarify": false,
		"keywords":       []any{"rms"},
		"created_by":     "John",
		"area_path":      `Enterprise\RMS`,
		"state":          "Active",
	})

	_, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireWFString(t, wf, qtKeyCreatedBy, "John")
	requireWFString(t, wf, qtKeyAreaPath, `Enterprise\RMS`)
	requireWFString(t, wf, qtKeyState, "Active")
}

func TestQT_Clarify_MissingAIResponse_Error(t *testing.T) {
	ctx, _ := qtNewCtx("find a ticket")

	_, err := QueryTicket(ctx, qtStepAction(StepQtClarify))
	if err == nil {
		t.Fatal("expected error when AI response is missing")
	}
}

// =============================================================================
// StepQtSpawnWorkers tests
// =============================================================================

func TestQT_SpawnWorkers_TwoStrategies_CreatesTwoSubWorkers(t *testing.T) {
	ctx, wf := qtNewCtx("find payment timeout bug")
	wf.SetWorkflowData(qtKeyKeywords, "payment, timeout")

	action := qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    2,
		"strategies_json": strategiesJSON(2),
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionAsync)

	if len(actions[0].AsyncActions) != 2 {
		t.Errorf("async sub-actions = %d, want 2", len(actions[0].AsyncActions))
	}
}

func TestQT_SpawnWorkers_SubActionsTargetSearcherWorkflow(t *testing.T) {
	ctx, _ := qtNewCtx("find bug")

	action := qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    1,
		"strategies_json": strategiesJSON(1),
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sub := actions[0].AsyncActions[0]
	wfName, _ := sub.Input[core.InputWorkflowName].(WorkflowName)
	if wfName != WorkflowQueryTicketSearcher {
		t.Errorf("sub-action WorkflowName = %q, want %q", wfName, WorkflowQueryTicketSearcher)
	}
}

func TestQT_SpawnWorkers_StoresExpectedWorkerCount(t *testing.T) {
	ctx, wf := qtNewCtx("find bug")

	action := qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    3,
		"strategies_json": strategiesJSON(3),
	})

	_, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireWFInt(t, wf, qtKeyExpectedWorkers, 3)
	requireWFInt(t, wf, qtKeyCollectedResults, 0)
}

func TestQT_SpawnWorkers_FiveWorkers_Maximum(t *testing.T) {
	ctx, _ := qtNewCtx("find something vague")

	action := qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    5,
		"strategies_json": strategiesJSON(5),
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(actions[0].AsyncActions) != 5 {
		t.Errorf("expected 5 sub-workers, got %d", len(actions[0].AsyncActions))
	}
}

func TestQT_SpawnWorkers_PassesUserContextAndRejectedIDs(t *testing.T) {
	ctx, wf := qtNewCtx("payment timeout bug")
	wf.SetWorkflowData(qtKeyKeywords, "payment, timeout")
	// Pre-set rejected IDs
	b, _ := json.Marshal([]int{999, 1000})
	wf.SetWorkflowData(qtKeyRejectedIDsJSON, string(b))

	action := qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    1,
		"strategies_json": strategiesJSON(1),
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The instruction passed to the sub-worker must include user context and rejected IDs
	sub := actions[0].AsyncActions[0]
	instructionStr, _ := sub.Input[core.InputMessage].(string)
	var instruction qtswInstruction
	if err := json.Unmarshal([]byte(instructionStr), &instruction); err != nil {
		t.Fatalf("invalid instruction JSON: %v", err)
	}

	if !strings.Contains(instruction.UserContext, "payment") {
		t.Errorf("user context = %q, want to contain 'payment'", instruction.UserContext)
	}
	if len(instruction.RejectedIDs) != 2 {
		t.Errorf("rejected IDs = %v, want [999 1000]", instruction.RejectedIDs)
	}
}

func TestQT_SpawnWorkers_InvalidStrategiesJSON_Error(t *testing.T) {
	ctx, _ := qtNewCtx("find bug")

	action := qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    2,
		"strategies_json": "not valid json {{",
	})

	_, err := QueryTicket(ctx, action)
	if err == nil {
		t.Fatal("expected error for invalid strategies JSON")
	}
}

func TestQT_SpawnWorkers_EmptyStrategies_Error(t *testing.T) {
	ctx, _ := qtNewCtx("find bug")

	action := qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    0,
		"strategies_json": "[]",
	})

	_, err := QueryTicket(ctx, action)
	if err == nil {
		t.Fatal("expected error for empty strategies list")
	}
}

// =============================================================================
// StepQtCollectResult tests
// =============================================================================

func TestQT_CollectResult_FirstOfTwo_ReturnsNil(t *testing.T) {
	ctx, wf := qtNewCtx("find bug")
	wf.SetWorkflowData(qtKeyExpectedWorkers, 2)
	wf.SetWorkflowData(qtKeyCollectedResults, 0)

	action := qtAIAction(StepQtCollectResult, map[string]any{
		"worker_id":       "1",
		"found_any":       true,
		"candidates_json": `[{"id":1234,"title":"Payment timeout","confidence":0.9}]`,
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions != nil {
		t.Errorf("expected nil actions while waiting for worker 2, got %v", actions)
	}

	// collected count incremented
	requireWFInt(t, wf, qtKeyCollectedResults, 1)
}

func TestQT_CollectResult_LastWorker_TriggersSynthesis(t *testing.T) {
	ctx, wf := qtNewCtx("find payment timeout bug")
	wf.SetWorkflowData(qtKeyExpectedWorkers, 2)
	wf.SetWorkflowData(qtKeyCollectedResults, 1)
	// Worker 1 already stored its result
	wf.SetWorkflowData(subWorkerKey("1", "result"), `[{"id":1234,"title":"Payment timeout","confidence":0.9}]`)

	action := qtAIAction(StepQtCollectResult, map[string]any{
		"worker_id":       "2",
		"found_any":       false,
		"candidates_json": "[]",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ActionAi (synthesis → StepQtAnalyze) + ActionCompleteAsync
	requireAtLeast(t, actions, 2)
	requireType(t, actions, 0, core.ActionAi)
	requireStep(t, actions, 0, StepQtAnalyze)
	requireType(t, actions, 1, core.ActionCompleteAsync)
}

func TestQT_CollectResult_AccumulatesCandidatesAcrossAttempts(t *testing.T) {
	ctx, wf := qtNewCtx("find bug")
	wf.SetWorkflowData(qtKeyExpectedWorkers, 1)
	wf.SetWorkflowData(qtKeyCollectedResults, 0)
	// Prior round already stored some candidates
	wf.SetWorkflowData(qtKeyAllCandidatesJSON, "Worker 1 candidates: [...prev...]")

	candidates := `[{"id":5678,"title":"Something else","confidence":0.7}]`
	action := qtAIAction(StepQtCollectResult, map[string]any{
		"worker_id":       "1",
		"found_any":       true,
		"candidates_json": candidates,
	})

	_, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all, _ := wf.GetWorkflowData(qtKeyAllCandidatesJSON).(string)
	if !strings.Contains(all, "prev") || !strings.Contains(all, "5678") {
		t.Errorf("allCandidates = %q, want both old and new candidates", all)
	}
}

func TestQT_CollectResult_AllEmptyResults_StillSynthesizes(t *testing.T) {
	ctx, wf := qtNewCtx("find bug")
	wf.SetWorkflowData(qtKeyExpectedWorkers, 1)
	wf.SetWorkflowData(qtKeyCollectedResults, 0)

	// Worker found nothing
	action := qtAIAction(StepQtCollectResult, map[string]any{
		"worker_id":       "1",
		"found_any":       false,
		"candidates_json": "[]",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Still triggers synthesis (AI will decide branch=refine)
	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)
	requireStep(t, actions, 0, StepQtAnalyze)
}

// =============================================================================
// StepQtAnalyze tests
// =============================================================================

func TestQT_Analyze_PresentTicket_DispatchesGetTicket(t *testing.T) {
	ctx, _ := qtNewCtx("find payment timeout")

	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch":         "present_ticket",
		"top_ticket_id":  1234,
		"message_to_user": "Found it!",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionTool)
	requireToolName(t, actions, 0, tool.ToolADOGetTicket)
	requireStep(t, actions, 0, StepQtPresentTicket)

	toolArgs, _ := actions[0].Input[core.InputToolArgs].(map[string]any)
	if toolArgs["work_item_id"] != 1234 {
		t.Errorf("work_item_id = %v, want 1234", toolArgs["work_item_id"])
	}
}

func TestQT_Analyze_ShowCandidates_ReturnsWaitWithList(t *testing.T) {
	ctx, wf := qtNewCtx("find bug")

	candidates := `[{"id":1,"title":"Ticket A","state":"Active","assigned_to":"Alice","work_item_type":"Defect","summary":"brief"},{"id":2,"title":"Ticket B","state":"Closed","assigned_to":"Bob","work_item_type":"Story","summary":"brief2"}]`
	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch":          "show_candidates",
		"candidates_json": candidates,
		"message_to_user": "I found a few tickets that might match:",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "Ticket A")
	requireWFString(t, wf, qtKeyPendingStep, "qt_show_candidates")
}

func TestQT_Analyze_Disambiguate_ReturnsWaitWithQuestion(t *testing.T) {
	ctx, wf := qtNewCtx("find bug")

	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch":          "disambiguate",
		"candidates_json": `[{"id":1,"title":"A"},{"id":2,"title":"B"}]`,
		"message_to_user": "Did you mean the one assigned to Sarah or the one from last sprint?",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "Sarah")
	requireWFString(t, wf, qtKeyPendingStep, "qt_disambiguate")
}

func TestQT_Analyze_NarrowDown_ReturnsWaitAskingForFilter(t *testing.T) {
	ctx, wf := qtNewCtx("find bug")

	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch":          "narrow_down",
		"message_to_user": "I found 12 tickets matching 'payment'. Can you give me one more detail?",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "12 tickets")
	requireWFString(t, wf, qtKeyPendingStep, "qt_narrow_down")
}

func TestQT_Analyze_Refine_Attempt1_AutoRetries(t *testing.T) {
	ctx, _ := qtNewCtx("find some bug")

	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch":          "refine",
		"message_to_user": "No results found.",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Attempt 1 → auto-retry: UserMessage("Let me search...") + ActionAi(StepQtSpawnWorkers)
	requireAtLeast(t, actions, 2)
	requireType(t, actions, 0, core.ActionUserMessage)
	requireType(t, actions, 1, core.ActionAi)
	requireStep(t, actions, 1, StepQtSpawnWorkers)
}

func TestQT_Analyze_Refine_Attempt3_AsksSpecificQuestion(t *testing.T) {
	ctx, wf := qtNewCtx("find some bug")
	wf.SetWorkflowData(qtKeyAttemptCount, 2) // will become 3

	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch":          "refine",
		"message_to_user": "Still no results.",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Attempt 3 → AI asked for specific question → ActionAi(StepQtAskRefineQuestion)
	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)
	requireStep(t, actions, 0, StepQtAskRefineQuestion)
}

func TestQT_Analyze_Refine_Attempt4_ExhaustedWithCandidates(t *testing.T) {
	ctx, wf := qtNewCtx("find some bug")
	wf.SetWorkflowData(qtKeyAttemptCount, 3) // will become 4
	wf.SetWorkflowData(qtKeyAllCandidatesJSON, `[{"id":9,"title":"Close match"}]`)

	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch":          "refine",
		"message_to_user": "No results.",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "keep trying")
	requireMessageContains(t, actions, 0, "best matches")
	requireWFString(t, wf, qtKeyPendingStep, "qt_exhausted")
}

func TestQT_Analyze_Refine_Attempt4_ExhaustedNoCandidates(t *testing.T) {
	ctx, wf := qtNewCtx("find some bug")
	wf.SetWorkflowData(qtKeyAttemptCount, 3)
	// No all-candidates stored

	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch": "refine",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "keep trying")
	// "show me your best matches" option should NOT appear when there are no candidates
	msg, _ := actions[0].Input[core.InputMessage].(string)
	if strings.Contains(msg, "best matches") {
		t.Error("exhausted message should not offer 'best matches' when no candidates were found")
	}
}

func TestQT_Analyze_UnknownBranch_FallsBackToRefine(t *testing.T) {
	ctx, _ := qtNewCtx("find bug")

	action := qtAIAction(StepQtAnalyze, map[string]any{
		"branch":          "some_unknown_branch_xyz",
		"message_to_user": "...",
	})

	// Should not error — falls back to refine path
	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Refine attempt 1 → auto-retry
	requireAtLeast(t, actions, 1)
}

// =============================================================================
// StepQtAskRefineQuestion tests
// =============================================================================

func TestQT_AskRefineQuestion_StoresAndReturnsQuestion(t *testing.T) {
	ctx, wf := qtNewCtx()

	action := qtAIAction(StepQtAskRefineQuestion, map[string]any{
		"question": "Was this ticket in the RMS project or POS?",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "RMS project")
	requireWFString(t, wf, qtKeyPendingStep, "qt_refine_asked")
}

func TestQT_AskRefineQuestion_EmptyQuestion_UsesFallback(t *testing.T) {
	ctx, wf := qtNewCtx()

	action := qtAIAction(StepQtAskRefineQuestion, map[string]any{
		"question": "",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	msg, _ := actions[0].Input[core.InputMessage].(string)
	if msg == "" {
		t.Error("expected non-empty fallback question")
	}
	requireWFString(t, wf, qtKeyPendingStep, "qt_refine_asked")
}

// =============================================================================
// StepQtHandlePick tests
// =============================================================================

func TestQT_HandlePick_Pick_DispatchesGetTicket(t *testing.T) {
	ctx, _ := qtNewCtx("find bug")

	action := qtAIAction(StepQtHandlePick, map[string]any{
		"action":    "pick",
		"ticket_id": 5678,
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionTool)
	requireToolName(t, actions, 0, tool.ToolADOGetTicket)
	requireStep(t, actions, 0, StepQtPresentTicket)

	toolArgs, _ := actions[0].Input[core.InputToolArgs].(map[string]any)
	if toolArgs["work_item_id"] != 5678 {
		t.Errorf("work_item_id = %v, want 5678", toolArgs["work_item_id"])
	}
}

func TestQT_HandlePick_None_TriggersRefine(t *testing.T) {
	ctx, _ := qtNewCtx("find bug")

	action := qtAIAction(StepQtHandlePick, map[string]any{
		"action": "none",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Refine attempt 1 → re-plan
	requireAtLeast(t, actions, 1)
}

func TestQT_HandlePick_KeepTrying_RePlansSearch(t *testing.T) {
	ctx, _ := qtNewCtx("more details about the payment bug")

	action := qtAIAction(StepQtHandlePick, map[string]any{
		"action": "keep_trying",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// UserMessage("Let me search...") + ActionAi(StepQtSpawnWorkers)
	requireAtLeast(t, actions, 2)
	requireType(t, actions, 0, core.ActionUserMessage)
	requireType(t, actions, 1, core.ActionAi)
	requireStep(t, actions, 1, StepQtSpawnWorkers)
}

func TestQT_HandlePick_ShowBest_ShowsAllCandidates(t *testing.T) {
	ctx, wf := qtNewCtx()
	allCandidates := "Worker 1: [{id:1,title:'Closest match'}]"
	wf.SetWorkflowData(qtKeyAllCandidatesJSON, allCandidates)

	action := qtAIAction(StepQtHandlePick, map[string]any{
		"action": "show_best",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "closest matches")
	requireWFString(t, wf, qtKeyPendingStep, "qt_show_candidates")
}

func TestQT_HandlePick_GiveUp_ReturnsDoneMessage(t *testing.T) {
	ctx, _ := qtNewCtx()

	action := qtAIAction(StepQtHandlePick, map[string]any{
		"action": "give_up",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserMessage)
}

func TestQT_HandlePick_Done_ReturnsDoneMessage(t *testing.T) {
	ctx, _ := qtNewCtx()

	action := qtAIAction(StepQtHandlePick, map[string]any{
		"action": "done",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserMessage)
}

// =============================================================================
// StepQtPresentTicket tests
// =============================================================================

func TestQT_PresentTicket_ToolResult_ShowsTicketAndWaits(t *testing.T) {
	ctx, wf := qtNewCtx()

	ticket := map[string]any{
		"id":             float64(1234),
		"title":          "Payment Gateway Timeout",
		"work_item_type": "Defect",
		"state":          "Active",
		"assigned_to":    "Sarah Johnson",
		"url":            "https://dev.azure.com/org/proj/_workitems/1234",
	}
	action := qtToolResultAction(StepQtPresentTicket, ticket)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ActionUserMessage (show ticket) + ActionUserWait ("Is this the one?")
	requireLen(t, actions, 2)
	requireType(t, actions, 0, core.ActionUserMessage)
	requireMessageContains(t, actions, 0, "Payment Gateway Timeout")
	requireMessageContains(t, actions, 0, "#1234")
	requireType(t, actions, 1, core.ActionUserWait)
	requireWFString(t, wf, qtKeyPendingStep, "qt_present_ticket")
}

func TestQT_PresentTicket_ToolResult_StoresTicketJSON(t *testing.T) {
	ctx, wf := qtNewCtx()

	ticket := map[string]any{
		"id":    float64(9999),
		"title": "Some Bug",
	}
	action := qtToolResultAction(StepQtPresentTicket, ticket)

	_, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored, _ := wf.GetWorkflowData(qtKeyFoundTicketJSON).(string)
	if !strings.Contains(stored, "9999") {
		t.Errorf("stored ticket JSON = %q, want to contain ticket id 9999", stored)
	}
}

func TestQT_PresentTicket_FollowUp_AnswerQuestion_ReturnsAnswer(t *testing.T) {
	ctx, wf := qtNewCtx()
	wf.SetWorkflowData(qtKeyFoundTicketJSON, `{"id":1234,"title":"Payment Bug","state":"Active"}`)

	action := qtAIAction(StepQtPresentTicket, map[string]any{
		"intent": "answer_question",
		"answer": "The ticket is currently Active and assigned to Sarah.",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserWait)
	requireMessageContains(t, actions, 0, "Active")
	requireMessageContains(t, actions, 0, "Sarah")
	// Pending step stays so user can ask more follow-ups
	requireWFString(t, wf, qtKeyPendingStep, "qt_present_ticket")
}

func TestQT_PresentTicket_FollowUp_WrongTicket_AddsToRejected(t *testing.T) {
	ctx, wf := qtNewCtx("find bug again")
	ticketJSON := `{"id":1234.0,"title":"Wrong Ticket"}`
	wf.SetWorkflowData(qtKeyFoundTicketJSON, ticketJSON)

	action := qtAIAction(StepQtPresentTicket, map[string]any{
		"intent": "wrong_ticket",
	})

	_, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1234 must now be in rejected IDs
	ids := qtGetRejectedIDs(wf)
	found := false
	for _, id := range ids {
		if id == 1234 {
			found = true
		}
	}
	if !found {
		t.Errorf("rejected IDs = %v, expected 1234 to be added", ids)
	}
}

func TestQT_PresentTicket_FollowUp_WrongTicket_ReSearches(t *testing.T) {
	ctx, wf := qtNewCtx("find a different bug")
	wf.SetWorkflowData(qtKeyFoundTicketJSON, `{"id":1234.0,"title":"Wrong Ticket"}`)

	action := qtAIAction(StepQtPresentTicket, map[string]any{
		"intent": "wrong_ticket",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ActionUserMessage("Got it, not the one...") + ActionUserMessage("Let me search...") + ActionAi(StepQtSpawnWorkers)
	requireAtLeast(t, actions, 2)
	requireType(t, actions, 0, core.ActionUserMessage)
	requireMessageContains(t, actions, 0, "not the one")
}

func TestQT_PresentTicket_FollowUp_NewTicketSearch_ResetsAndRestarts(t *testing.T) {
	ctx, wf := qtNewCtx("find a completely different ticket")
	wf.SetWorkflowData(qtKeyFoundTicketJSON, `{"id":1234,"title":"Old Ticket"}`)
	wf.SetWorkflowData(qtKeyAttemptCount, 2)

	action := qtAIAction(StepQtPresentTicket, map[string]any{
		"intent": "new_ticket_search",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// New extract AI call
	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)
	requireStep(t, actions, 0, StepQtClarify)

	// Attempt count should be reset
	if v := wf.GetWorkflowData(qtKeyAttemptCount); v != nil {
		t.Errorf("attempt count should be reset after new search, got %v", v)
	}
}

func TestQT_PresentTicket_FollowUp_Done_SendsDoneMessage(t *testing.T) {
	ctx, wf := qtNewCtx()
	wf.SetWorkflowData(qtKeyFoundTicketJSON, `{"id":1234,"title":"Some Ticket"}`)

	action := qtAIAction(StepQtPresentTicket, map[string]any{
		"intent": "done",
	})

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionUserMessage)
}

func TestQT_PresentTicket_NeitherInput_ReturnsError(t *testing.T) {
	ctx, _ := qtNewCtx()

	// Action with no tool result and no AI response
	action := qtStepAction(StepQtPresentTicket)

	_, err := QueryTicket(ctx, action)
	if err == nil {
		t.Fatal("expected error when neither tool result nor AI response is present")
	}
}

// =============================================================================
// StepUserAnsweringQuestion dispatch tests
// =============================================================================

func TestQT_UserAnswer_ClarifyAsked_RePlansSearch(t *testing.T) {
	ctx, wf := qtNewCtx("payment timeout bug, assigned to Sarah")
	wf.SetWorkflowData(qtKeyPendingStep, "qt_clarify_asked")
	wf.SetWorkflowData(qtKeyKeywords, "payment, timeout")

	action := qtStepAction(StepUserAnsweringQuestion)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Re-plan: UserMessage + ActionAi(StepQtSpawnWorkers)
	requireAtLeast(t, actions, 2)
	requireType(t, actions, 0, core.ActionUserMessage)
	requireType(t, actions, 1, core.ActionAi)
	requireStep(t, actions, 1, StepQtSpawnWorkers)
}

func TestQT_UserAnswer_Disambiguate_RePlansSearch(t *testing.T) {
	ctx, wf := qtNewCtx("the one from last sprint")
	wf.SetWorkflowData(qtKeyPendingStep, "qt_disambiguate")

	action := qtStepAction(StepUserAnsweringQuestion)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireAtLeast(t, actions, 1)
	requireType(t, actions, len(actions)-1, core.ActionAi)
	requireStep(t, actions, len(actions)-1, StepQtSpawnWorkers)
}

func TestQT_UserAnswer_ShowCandidates_AsksPickAI(t *testing.T) {
	ctx, wf := qtNewCtx("I meant the second one")
	candidates := `[{"id":1,"title":"A"},{"id":2,"title":"B"}]`
	wf.SetWorkflowData(qtKeyPendingStep, "qt_show_candidates")
	wf.SetWorkflowData(qtKeyCandidatesJSON, candidates)

	action := qtStepAction(StepUserAnsweringQuestion)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)
	requireStep(t, actions, 0, StepQtHandlePick)
	// pending step cleared
	requireWFString(t, wf, qtKeyPendingStep, "")
}

func TestQT_UserAnswer_Exhausted_AsksExhaustedAI(t *testing.T) {
	ctx, wf := qtNewCtx("show me what you found")
	wf.SetWorkflowData(qtKeyPendingStep, "qt_exhausted")
	wf.SetWorkflowData(qtKeyAllCandidatesJSON, `[{"id":5,"title":"Something close"}]`)

	action := qtStepAction(StepUserAnsweringQuestion)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)
	requireStep(t, actions, 0, StepQtHandlePick)
}

func TestQT_UserAnswer_PresentTicket_AsksFollowUpAI(t *testing.T) {
	ctx, wf := qtNewCtx("who is assigned to this ticket?")
	wf.SetWorkflowData(qtKeyPendingStep, "qt_present_ticket")
	wf.SetWorkflowData(qtKeyFoundTicketJSON, `{"id":1234,"title":"Payment Bug","assigned_to":"Sarah"}`)

	action := qtStepAction(StepUserAnsweringQuestion)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)
	requireStep(t, actions, 0, StepQtPresentTicket)
}

func TestQT_UserAnswer_RefineAsked_RePlansSearch(t *testing.T) {
	ctx, wf := qtNewCtx("it was in the billing module from last month")
	wf.SetWorkflowData(qtKeyPendingStep, "qt_refine_asked")

	action := qtStepAction(StepUserAnsweringQuestion)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireAtLeast(t, actions, 1)
	requireType(t, actions, len(actions)-1, core.ActionAi)
	requireStep(t, actions, len(actions)-1, StepQtSpawnWorkers)
}

func TestQT_UserAnswer_NarrowDown_RePlansSearch(t *testing.T) {
	ctx, wf := qtNewCtx("it was a defect in the checkout flow")
	wf.SetWorkflowData(qtKeyPendingStep, "qt_narrow_down")

	action := qtStepAction(StepUserAnsweringQuestion)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireAtLeast(t, actions, 1)
}

func TestQT_UserAnswer_UnknownPending_TreatedAsSideQuestion(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)

	ctx, wf := qtNewCtx("what time is it?")
	wf.SetWorkflowData(qtKeyPendingStep, "") // unknown/empty

	action := qtStepAction(StepUserAnsweringQuestion)

	actions, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Falls back to side question handler → ActionAi
	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)

	_ = mock // mock installed but AI called via askAI (no actual SendMessage)
}

// =============================================================================
// Unknown step error test
// =============================================================================

func TestQT_UnknownStep_ReturnsError(t *testing.T) {
	ctx, _ := qtNewCtx()

	_, err := QueryTicket(ctx, qtStepAction("totally_unknown_step"))
	if err == nil {
		t.Fatal("expected error for unknown step")
	}
}

// =============================================================================
// QueryTicketSearcher sub-workflow tests
// =============================================================================

func TestQTSearcher_Search_ReturnsToolAction(t *testing.T) {
	ctx, _ := qtSearcherCtx()

	instruction := qtswInstruction{
		WorkerID: "1",
		Angle:    "search by title keyword",
		SearchParams: map[string]any{
			"title":       "payment timeout",
			"max_results": 15,
		},
		UserContext: "Keywords: payment, timeout",
		RejectedIDs: nil,
	}
	instructionJSON, _ := json.Marshal(instruction)

	action := qtSubWorkerAction(StepQtswSearch, "1", string(instructionJSON))

	actions, err := QueryTicketSearcher(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireLen(t, actions, 1)
	requireType(t, actions, 0, core.ActionTool)
	requireToolName(t, actions, 0, tool.ToolADOSearchTickets)
	requireStep(t, actions, 0, StepQtswEvaluate)

	// Must route back to the searcher sub-workflow, not the parent
	wfName, _ := actions[0].Input[core.InputWorkflowName].(WorkflowName)
	if wfName != WorkflowQueryTicketSearcher {
		t.Errorf("tool action WorkflowName = %q, want %q", wfName, WorkflowQueryTicketSearcher)
	}
}

func TestQTSearcher_Search_StoresInstruction(t *testing.T) {
	ctx, wf := qtSearcherCtx()

	instruction := qtswInstruction{
		WorkerID:    "2",
		Angle:       "search by assigned_to",
		SearchParams: map[string]any{"assigned_to": "Sarah"},
		UserContext: "Assigned to: Sarah",
	}
	instructionJSON, _ := json.Marshal(instruction)

	action := qtSubWorkerAction(StepQtswSearch, "2", string(instructionJSON))

	_, err := QueryTicketSearcher(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored, _ := wf.GetWorkflowData(subWorkerKey("2", "instruction")).(string)
	if !strings.Contains(stored, "Sarah") {
		t.Errorf("stored instruction = %q, want to contain 'Sarah'", stored)
	}
}

func TestQTSearcher_Search_PassesSearchParamsToTool(t *testing.T) {
	ctx, _ := qtSearcherCtx()

	instruction := qtswInstruction{
		WorkerID: "1",
		Angle:    "type+keyword search",
		SearchParams: map[string]any{
			"title":          "checkout timeout",
			"work_item_type": "Defect",
			"max_results":    10,
		},
	}
	instructionJSON, _ := json.Marshal(instruction)

	action := qtSubWorkerAction(StepQtswSearch, "1", string(instructionJSON))

	actions, err := QueryTicketSearcher(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolArgs, _ := actions[0].Input[core.InputToolArgs].(map[string]any)
	if toolArgs["title"] != "checkout timeout" {
		t.Errorf("tool args title = %v, want 'checkout timeout'", toolArgs["title"])
	}
	if toolArgs["work_item_type"] != "Defect" {
		t.Errorf("tool args work_item_type = %v, want 'Defect'", toolArgs["work_item_type"])
	}
}

func TestQTSearcher_Evaluate_RoutesResultToParent(t *testing.T) {
	ctx, wf := qtSearcherCtx()

	// Pre-store instruction (done by the search step)
	instruction := qtswInstruction{
		WorkerID:    "1",
		Angle:       "title search",
		UserContext: "Keywords: payment",
		RejectedIDs: []int{999},
	}
	instrJSON, _ := json.Marshal(instruction)
	wf.SetWorkflowData(subWorkerKey("1", "instruction"), string(instrJSON))

	toolResult := map[string]any{
		"count": float64(3),
		"items": []any{
			map[string]any{"id": float64(1234), "title": "Payment Gateway Timeout"},
		},
	}

	action := qtSubWorkerToolResultAction(StepQtswEvaluate, "1", toolResult)

	actions, err := QueryTicketSearcher(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)

	// Must NOT have InputWorkflowName so it routes to the parent workflow
	if wfName, ok := actions[0].Input[core.InputWorkflowName]; ok && wfName != nil {
		t.Errorf("result action should not have WorkflowName (must route to parent), got %v", wfName)
	}

	// Step must be StepQtCollectResult so parent knows how to handle it
	requireStep(t, actions, 0, StepQtCollectResult)
}

func TestQTSearcher_Evaluate_EmptyToolResult_StillEvaluates(t *testing.T) {
	ctx, wf := qtSearcherCtx()

	instruction := qtswInstruction{WorkerID: "3", Angle: "title search"}
	instrJSON, _ := json.Marshal(instruction)
	wf.SetWorkflowData(subWorkerKey("3", "instruction"), string(instrJSON))

	// Empty results from ADO
	toolResult := map[string]any{
		"count": float64(0),
		"items": []any{},
	}

	action := qtSubWorkerToolResultAction(StepQtswEvaluate, "3", toolResult)

	actions, err := QueryTicketSearcher(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Still creates an AI evaluation action
	requireAtLeast(t, actions, 1)
	requireType(t, actions, 0, core.ActionAi)
}

func TestQTSearcher_UnknownStep_ReturnsError(t *testing.T) {
	ctx, _ := qtSearcherCtx()

	action := qtSubWorkerAction("totally_unknown_step", "1", "{}")

	_, err := QueryTicketSearcher(ctx, action)
	if err == nil {
		t.Fatal("expected error for unknown step")
	}
}

func TestQTSearcher_Search_MissingWorkerID_Error(t *testing.T) {
	ctx, _ := qtSearcherCtx()

	// Action missing InputSubWorkerID
	a := core.NewAction(core.ActionWorkflow)
	a.Input = map[core.InputType]any{
		core.InputStep:    StepQtswSearch,
		core.InputMessage: `{"worker_id":"","angle":"test","search_params":{}}`,
	}

	_, err := QueryTicketSearcher(ctx, a)
	if err == nil {
		t.Fatal("expected error when worker ID is missing")
	}
}

// =============================================================================
// qtStrategy.searchParams() tests
// =============================================================================

func TestQT_SearchParams_TitleAndType(t *testing.T) {
	s := qtStrategy{
		Title:        "payment timeout",
		WorkItemType: "Defect",
		MaxResults:   20,
	}
	p := s.searchParams()

	if p["title"] != "payment timeout" {
		t.Errorf("title = %v, want 'payment timeout'", p["title"])
	}
	if p["work_item_type"] != "Defect" {
		t.Errorf("work_item_type = %v, want 'Defect'", p["work_item_type"])
	}
	if p["max_results"] != 20 {
		t.Errorf("max_results = %v, want 20", p["max_results"])
	}
}

func TestQT_SearchParams_DefaultMaxResults(t *testing.T) {
	s := qtStrategy{Title: "some bug"} // MaxResults = 0 → default
	p := s.searchParams()

	if p["max_results"] != 15 {
		t.Errorf("max_results = %v, want 15 (default)", p["max_results"])
	}
}

func TestQT_SearchParams_EmptyFieldsOmitted(t *testing.T) {
	s := qtStrategy{Title: "bug"} // All other fields empty
	p := s.searchParams()

	for _, field := range []string{"assigned_to", "created_by", "state", "area_path"} {
		if _, exists := p[field]; exists {
			t.Errorf("empty field %q should not appear in search params", field)
		}
	}
}

func TestQT_SearchParams_AllFields(t *testing.T) {
	s := qtStrategy{
		Title:         "payment timeout",
		AssignedTo:    "Sarah",
		CreatedBy:     "John",
		QAPerson:      "Alice",
		WorkItemType:  "Defect",
		State:         "Active",
		Tags:          []string{"billing", "critical"},
		AreaPath:      `Enterprise\RMS`,
		IterationPath: "Sprint 42",
		MaxResults:    10,
	}
	p := s.searchParams()

	checks := map[string]any{
		"title":          "payment timeout",
		"assigned_to":    "Sarah",
		"created_by":     "John",
		"qa_person":      "Alice",
		"work_item_type": "Defect",
		"state":          "Active",
		"area_path":      `Enterprise\RMS`,
		"iteration_path": "Sprint 42",
		"max_results":    10,
	}
	for k, want := range checks {
		if p[k] != want {
			t.Errorf("searchParams[%q] = %v, want %v", k, p[k], want)
		}
	}
	tags, _ := p["tags"].([]any)
	if len(tags) != 2 {
		t.Errorf("tags len = %d, want 2", len(tags))
	}
}

// =============================================================================
// qtBuildUserContext helper tests
// =============================================================================

func TestQT_BuildUserContext_AllFields(t *testing.T) {
	_, wf := qtNewCtx()
	wf.SetWorkflowData(qtKeyKeywords, "payment, timeout")
	wf.SetWorkflowData(qtKeyWorkItemType, "Defect")
	wf.SetWorkflowData(qtKeyState, "Active")
	wf.SetWorkflowData(qtKeyAssignedTo, "Sarah")
	wf.SetWorkflowData(qtKeyCreatedBy, "John")
	wf.SetWorkflowData(qtKeyAreaPath, `Enterprise\RMS`)

	ctx := qtBuildUserContext(wf)

	for _, want := range []string{"payment", "Defect", "Active", "Sarah", "John", `Enterprise\RMS`} {
		if !strings.Contains(ctx, want) {
			t.Errorf("user context = %q\n  want to contain %q", ctx, want)
		}
	}
}

func TestQT_BuildUserContext_Empty_ReturnsDefault(t *testing.T) {
	_, wf := qtNewCtx()

	ctx := qtBuildUserContext(wf)

	if !strings.Contains(ctx, "No specific") {
		t.Errorf("empty context = %q, want 'No specific...'", ctx)
	}
}

// =============================================================================
// qtGetRejectedIDs / qtAddRejectedID helper tests
// =============================================================================

func TestQT_AddRejectedID_SingleID(t *testing.T) {
	_, wf := qtNewCtx()

	qtAddRejectedID(wf, 1234)

	ids := qtGetRejectedIDs(wf)
	if len(ids) != 1 || ids[0] != 1234 {
		t.Errorf("rejected IDs = %v, want [1234]", ids)
	}
}

func TestQT_AddRejectedID_MultipleIDs(t *testing.T) {
	_, wf := qtNewCtx()

	qtAddRejectedID(wf, 100)
	qtAddRejectedID(wf, 200)
	qtAddRejectedID(wf, 300)

	ids := qtGetRejectedIDs(wf)
	if len(ids) != 3 {
		t.Errorf("rejected IDs = %v, want 3 entries", ids)
	}
}

func TestQT_GetRejectedIDs_EmptyWhenNotSet(t *testing.T) {
	_, wf := qtNewCtx()

	ids := qtGetRejectedIDs(wf)
	if ids != nil {
		t.Errorf("expected nil rejected IDs when not set, got %v", ids)
	}
}

// =============================================================================
// qtFormatTicket helper tests
// =============================================================================

func TestQT_FormatTicket_BasicFields(t *testing.T) {
	ticket := map[string]any{
		"id":             float64(1234),
		"title":          "Payment Gateway Timeout",
		"work_item_type": "Defect",
		"state":          "Active",
		"assigned_to":    "Sarah Johnson",
	}

	msg := qtFormatTicket(ticket)

	for _, want := range []string{"#1234", "Payment Gateway Timeout", "Defect", "Active", "Sarah Johnson"} {
		if !strings.Contains(msg, want) {
			t.Errorf("formatted ticket = %q\n  want to contain %q", msg, want)
		}
	}
}

func TestQT_FormatTicket_IncludesQAAndURL(t *testing.T) {
	ticket := map[string]any{
		"id":             float64(42),
		"title":          "Some bug",
		"work_item_type": "Defect",
		"state":          "Closed",
		"assigned_to":    "Alice",
		"qa_person":      "Bob",
		"url":            "https://dev.azure.com/org/project/_workitems/42",
	}

	msg := qtFormatTicket(ticket)

	if !strings.Contains(msg, "Bob") {
		t.Errorf("formatted ticket = %q, want to contain QA person 'Bob'", msg)
	}
	if !strings.Contains(msg, "View in ADO") {
		t.Errorf("formatted ticket = %q, want to contain 'View in ADO' link", msg)
	}
}

func TestQT_FormatTicket_OmitsQAWhenUnassigned(t *testing.T) {
	ticket := map[string]any{
		"id":         float64(1),
		"title":      "Bug",
		"state":      "New",
		"assigned_to": "Dev",
		"qa_person":  "Unassigned",
	}

	msg := qtFormatTicket(ticket)

	if strings.Contains(msg, "QA:") {
		t.Errorf("formatted ticket should not show 'QA:' when qa_person is 'Unassigned':\n%s", msg)
	}
}

// =============================================================================
// qtFormatCandidatesList helper tests
// =============================================================================

func TestQT_FormatCandidatesList_FormatsMultipleCandidates(t *testing.T) {
	intro := "I found a few matches:"
	candidates := `[
		{"id":1,"title":"Payment Bug","state":"Active","assigned_to":"Sarah","work_item_type":"Defect","summary":"Handles retry logic"},
		{"id":2,"title":"Checkout Error","state":"Closed","assigned_to":"Bob","work_item_type":"Story","summary":"Checkout flow fix"}
	]`

	msg := qtFormatCandidatesList(intro, candidates)

	for _, want := range []string{"Payment Bug", "Checkout Error", "Sarah", "Bob", "none of these"} {
		if !strings.Contains(msg, want) {
			t.Errorf("candidates list = %q\n  want to contain %q", msg, want)
		}
	}
}

func TestQT_FormatCandidatesList_DefaultIntroWhenEmpty(t *testing.T) {
	candidates := `[{"id":1,"title":"Bug","state":"Active","assigned_to":"Dev","work_item_type":"Defect","summary":"brief"}]`

	msg := qtFormatCandidatesList("", candidates)

	if !strings.Contains(msg, "might match") {
		t.Errorf("message = %q, want default intro containing 'might match'", msg)
	}
}

func TestQT_FormatCandidatesList_InvalidJSON_ReturnsIntro(t *testing.T) {
	msg := qtFormatCandidatesList("Found something:", "not json {{")

	if msg != "Found something:" {
		t.Errorf("invalid JSON should return intro only, got %q", msg)
	}
}

// =============================================================================
// End-to-end happy path simulation
// (manually chains steps to simulate a full successful search flow)
// =============================================================================

// TestQT_HappyPath_ExtractPlanSearchPresent walks through the core success path:
// Init → Clarify (no clarify) → SpawnWorkers → CollectResult (1 worker) → Analyze (present) → PresentTicket
func TestQT_HappyPath_ExtractPlanSearchPresent(t *testing.T) {
	ctx, wf := qtNewCtx("find the payment gateway timeout defect assigned to Sarah")

	// Step 1: Init → should queue AI for extract step
	step1, err := QueryTicket(ctx, qtStepAction(StepInit))
	if err != nil {
		t.Fatalf("StepInit error: %v", err)
	}
	requireLen(t, step1, 1)
	requireType(t, step1, 0, core.ActionAi)
	requireStep(t, step1, 0, StepQtClarify)

	// Step 2: Clarify — AI extracts info, no clarify needed
	step2, err := QueryTicket(ctx, qtAIAction(StepQtClarify, map[string]any{
		"should_clarify": false,
		"keywords":       []any{"payment", "gateway", "timeout"},
		"assigned_to":    "Sarah",
		"work_item_type": "Defect",
	}))
	if err != nil {
		t.Fatalf("StepQtClarify error: %v", err)
	}
	requireAtLeast(t, step2, 2)
	requireType(t, step2, 1, core.ActionAi)
	requireStep(t, step2, 1, StepQtSpawnWorkers)

	// Step 3: SpawnWorkers — AI returns 2 strategies
	step3, err := QueryTicket(ctx, qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    2,
		"strategies_json": strategiesJSON(2),
	}))
	if err != nil {
		t.Fatalf("StepQtSpawnWorkers error: %v", err)
	}
	requireLen(t, step3, 1)
	requireType(t, step3, 0, core.ActionAsync)

	// Step 4: CollectResult — single worker returns high-confidence candidate
	wf.SetWorkflowData(qtKeyExpectedWorkers, 2)
	wf.SetWorkflowData(qtKeyCollectedResults, 1)
	wf.SetWorkflowData(subWorkerKey("1", "result"), `[{"id":1234,"confidence":0.95}]`)

	step4, err := QueryTicket(ctx, qtAIAction(StepQtCollectResult, map[string]any{
		"worker_id":       "2",
		"found_any":       false,
		"candidates_json": "[]",
	}))
	if err != nil {
		t.Fatalf("StepQtCollectResult error: %v", err)
	}
	requireAtLeast(t, step4, 2)
	requireType(t, step4, 0, core.ActionAi)
	requireStep(t, step4, 0, StepQtAnalyze)

	// Step 5: Analyze — AI picks present_ticket with high confidence
	step5, err := QueryTicket(ctx, qtAIAction(StepQtAnalyze, map[string]any{
		"branch":          "present_ticket",
		"top_ticket_id":   1234,
		"message_to_user": "Found it!",
	}))
	if err != nil {
		t.Fatalf("StepQtAnalyze error: %v", err)
	}
	requireLen(t, step5, 1)
	requireType(t, step5, 0, core.ActionTool)
	requireToolName(t, step5, 0, tool.ToolADOGetTicket)

	// Step 6: PresentTicket — tool returns full ticket data
	ticket := map[string]any{
		"id":             float64(1234),
		"title":          "Payment Gateway Timeout",
		"work_item_type": "Defect",
		"state":          "Active",
		"assigned_to":    "Sarah Johnson",
	}
	step6, err := QueryTicket(ctx, qtToolResultAction(StepQtPresentTicket, ticket))
	if err != nil {
		t.Fatalf("StepQtPresentTicket error: %v", err)
	}
	requireLen(t, step6, 2)
	requireType(t, step6, 0, core.ActionUserMessage)
	requireMessageContains(t, step6, 0, "Payment Gateway Timeout")
	requireType(t, step6, 1, core.ActionUserWait)
	requireWFString(t, wf, qtKeyPendingStep, "qt_present_ticket")
}

// TestQT_RejectedTicket_MemoryPersists simulates a user rejecting a found ticket
// and verifies the rejected ID is carried forward to the next search.
func TestQT_RejectedTicket_MemoryPersists(t *testing.T) {
	ctx, wf := qtNewCtx("find the bug again")

	// Simulate: ticket 1234 was shown, user said "wrong ticket"
	wf.SetWorkflowData(qtKeyFoundTicketJSON, `{"id":1234.0,"title":"Wrong Ticket"}`)

	action := qtAIAction(StepQtPresentTicket, map[string]any{
		"intent": "wrong_ticket",
	})

	_, err := QueryTicket(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now simulate a new SpawnWorkers call — rejected IDs must be in instruction
	wf.SetWorkflowData(qtKeyKeywords, "payment bug")
	planAction := qtAIAction(StepQtSpawnWorkers, map[string]any{
		"worker_count":    1,
		"strategies_json": strategiesJSON(1),
	})

	planActions, err := QueryTicket(ctx, planAction)
	if err != nil {
		t.Fatalf("spawn workers after rejection error: %v", err)
	}

	sub := planActions[0].AsyncActions[0]
	instructionStr, _ := sub.Input[core.InputMessage].(string)
	var instruction qtswInstruction
	json.Unmarshal([]byte(instructionStr), &instruction) //nolint:errcheck

	found := false
	for _, id := range instruction.RejectedIDs {
		if id == 1234 {
			found = true
		}
	}
	if !found {
		t.Errorf("rejected IDs in worker instruction = %v, want to contain 1234", instruction.RejectedIDs)
	}
}
