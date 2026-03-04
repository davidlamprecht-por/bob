package orchestrator_test

import (
	"testing"

	"bob/internal/orchestrator"
	"bob/internal/orchestrator/core"
	"bob/internal/workflow"
)

// -----------------------------------------------------------------------
// ProcessUserIntent
// -----------------------------------------------------------------------

func TestProcessUserIntent_NeedsUserInput(t *testing.T) {
	msg := "Did you mean to create a ticket?"
	intent := core.Intent{
		IntentType:     core.IntentAskQuestion,
		NeedsUserInput: true,
		MessageToUser:  &msg,
	}
	actions := orchestrator.ProcessUserIntent(intent)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ActionType != core.ActionUserWait {
		t.Errorf("ActionType = %d, want ActionUserWait (%d)", actions[0].ActionType, core.ActionUserWait)
	}
	if got, ok := actions[0].Input[core.InputMessage].(string); !ok || got != msg {
		t.Errorf("InputMessage = %v, want %q", actions[0].Input[core.InputMessage], msg)
	}
}

func TestProcessUserIntent_NewWorkflow_DefaultStep(t *testing.T) {
	intent := core.Intent{
		IntentType:   core.IntentNewWorkflow,
		WorkflowName: "testAI",
	}
	actions := orchestrator.ProcessUserIntent(intent)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ActionType != core.ActionWorkflow {
		t.Errorf("ActionType = %d, want ActionWorkflow (%d)", actions[0].ActionType, core.ActionWorkflow)
	}
	if got, ok := actions[0].Input[core.InputStep].(string); !ok || got != workflow.StepInit {
		t.Errorf("InputStep = %v, want %q", actions[0].Input[core.InputStep], workflow.StepInit)
	}
}

func TestProcessUserIntent_AskQuestion_DefaultStep(t *testing.T) {
	intent := core.Intent{
		IntentType:   core.IntentAskQuestion,
		WorkflowName: "testAI",
	}
	actions := orchestrator.ProcessUserIntent(intent)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if got := actions[0].Input[core.InputStep]; got != workflow.StepUserAsksQuestion {
		t.Errorf("InputStep = %v, want %q", got, workflow.StepUserAsksQuestion)
	}
}

func TestProcessUserIntent_AnswerQuestion_DefaultStep(t *testing.T) {
	intent := core.Intent{
		IntentType:   core.IntentAnswerQuestion,
		WorkflowName: "testAI",
	}
	actions := orchestrator.ProcessUserIntent(intent)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if got := actions[0].Input[core.InputStep]; got != workflow.StepUserAnsweringQuestion {
		t.Errorf("InputStep = %v, want %q", got, workflow.StepUserAnsweringQuestion)
	}
}

func TestProcessUserIntent_ExplicitStep_UsedVerbatim(t *testing.T) {
	intent := core.Intent{
		IntentType:   core.IntentNewWorkflow,
		WorkflowName: "testSubworkflows",
		Step:         "tsw_spawn_workers",
	}
	actions := orchestrator.ProcessUserIntent(intent)
	if got := actions[0].Input[core.InputStep]; got != "tsw_spawn_workers" {
		t.Errorf("InputStep = %v, want %q", got, "tsw_spawn_workers")
	}
}

// -----------------------------------------------------------------------
// RouteUserMessage
// -----------------------------------------------------------------------

func TestRouteUserMessage_Idle_NewWorkflow(t *testing.T) {
	ctx := core.NewConversationContext() // idle
	intent := core.Intent{
		IntentType:   core.IntentNewWorkflow,
		WorkflowName: "testAI",
	}
	actions := []*core.Action{core.NewAction(core.ActionWorkflow)}

	shouldHandle := orchestrator.RouteUserMessage(ctx, &intent, actions)
	if !shouldHandle {
		t.Error("expected shouldHandle=true for idle+NewWorkflow")
	}
	if wf := ctx.GetCurrentWorkflow(); wf == nil || wf.GetWorkflowName() != "testAI" {
		t.Errorf("expected current workflow=testAI, got %v", ctx.GetCurrentWorkflow())
	}
}

func TestRouteUserMessage_WaitForUser_AnswerQuestion(t *testing.T) {
	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusWaitForUser)

	intent := core.Intent{
		IntentType:   core.IntentAnswerQuestion,
		WorkflowName: "testAI",
	}
	actions := []*core.Action{core.NewAction(core.ActionWorkflow)}

	shouldHandle := orchestrator.RouteUserMessage(ctx, &intent, actions)
	if !shouldHandle {
		t.Error("expected shouldHandle=true when WaitForUser")
	}
	if ctx.GetCurrentStatus() != core.StatusRunning {
		t.Errorf("expected status=Running after WaitForUser→AnswerQuestion, got %q", ctx.GetCurrentStatus())
	}
}

func TestRouteUserMessage_WaitForUser_NewWorkflow_SwitchesWorkflow(t *testing.T) {
	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusWaitForUser)

	intent := core.Intent{
		IntentType:   core.IntentNewWorkflow,
		WorkflowName: "createTicket",
	}
	actions := []*core.Action{core.NewAction(core.ActionWorkflow)}

	shouldHandle := orchestrator.RouteUserMessage(ctx, &intent, actions)
	if !shouldHandle {
		t.Error("expected shouldHandle=true")
	}
	if wf := ctx.GetCurrentWorkflow(); wf == nil || wf.GetWorkflowName() != "createTicket" {
		t.Errorf("expected workflow=createTicket after switch, got %v", ctx.GetCurrentWorkflow())
	}
	if ctx.GetRemainingActions() != nil {
		t.Error("expected remaining actions to be cleared on workflow switch")
	}
}

func TestRouteUserMessage_Running_AppendsAndReturnsFalse(t *testing.T) {
	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusRunning)

	intent := core.Intent{
		IntentType:   core.IntentAskQuestion,
		WorkflowName: "testAI",
	}
	a := core.NewAction(core.ActionWorkflow)
	actions := []*core.Action{a}

	shouldHandle := orchestrator.RouteUserMessage(ctx, &intent, actions)
	if shouldHandle {
		t.Error("expected shouldHandle=false when status=Running")
	}
	if len(ctx.GetRemainingActions()) != 1 {
		t.Errorf("expected 1 remaining action, got %d", len(ctx.GetRemainingActions()))
	}
}
