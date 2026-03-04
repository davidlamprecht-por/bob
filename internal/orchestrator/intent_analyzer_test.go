package orchestrator

import (
	"testing"
	"time"

	"bob/internal/ai/aimock"
	"bob/internal/orchestrator/core"
	"bob/internal/workflow"
)

// newMsg is a helper that returns a minimal Message with the given text.
func newMsg(text string) *core.Message {
	return &core.Message{Message: text}
}

// intentResponse builds the data map expected by callIntentAI.
func intentResponse(wfName, step string, confidence float64, reasoning, clarifyingQ string) map[string]any {
	d := map[string]any{
		"workflow_name": wfName,
		"step":          step,
		"confidence":    confidence,
		"reasoning":     reasoning,
	}
	if clarifyingQ != "" {
		d["clarifying_question"] = clarifyingQ
	}
	return d
}

// -----------------------------------------------------------------------
// Pure helper tests (no mock needed)
// -----------------------------------------------------------------------

func TestWorkflowInHistory_Found(t *testing.T) {
	history := []*core.WorkflowHistoryEntry{
		{WorkflowName: "createTicket"},
		{WorkflowName: "testAI"},
	}
	if !workflowInHistory("createTicket", history) {
		t.Error("expected createTicket to be found in history")
	}
}

func TestWorkflowInHistory_NotFound(t *testing.T) {
	history := []*core.WorkflowHistoryEntry{
		{WorkflowName: "testAI"},
	}
	if workflowInHistory("createTicket", history) {
		t.Error("expected createTicket NOT to be found in history")
	}
}

func TestWorkflowInHistory_EmptySlice(t *testing.T) {
	if workflowInHistory("testAI", nil) {
		t.Error("expected false for empty history slice")
	}
}

func TestMapStepToIntentType_AllCases(t *testing.T) {
	cases := []struct {
		step string
		want core.IntentType
	}{
		{workflow.StepInit, core.IntentNewWorkflow},
		{workflow.StepUserAsksQuestion, core.IntentAskQuestion},
		{workflow.StepUserAnsweringQuestion, core.IntentAnswerQuestion},
		{"tsw_collect_result", core.IntentAnswerQuestion}, // default
		{"", core.IntentAnswerQuestion},                   // default
	}
	for _, tc := range cases {
		got := mapStepToIntentType(tc.step)
		if got != tc.want {
			t.Errorf("mapStepToIntentType(%q) = %q, want %q", tc.step, got, tc.want)
		}
	}
}

// -----------------------------------------------------------------------
// AnalyzeIntent integration tests (one mock response per test)
// -----------------------------------------------------------------------

func TestAnalyzeIntent_NewWorkflow_HighConf(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	mock.QueueResponse(intentResponse("testAI", workflow.StepInit, 0.90, "clear match", ""))

	ctx := core.NewConversationContext() // idle, no active workflow
	intent := AnalyzeIntent(newMsg("run a test"), ctx)

	if intent.IntentType != core.IntentNewWorkflow {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentNewWorkflow)
	}
	if intent.WorkflowName != "testAI" {
		t.Errorf("WorkflowName = %q, want testAI", intent.WorkflowName)
	}
}

func TestAnalyzeIntent_NewWorkflow_LowConf(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	mock.QueueResponse(intentResponse("testAI", workflow.StepInit, 0.50, "unclear", ""))

	ctx := core.NewConversationContext()
	intent := AnalyzeIntent(newMsg("hmm"), ctx)

	if intent.IntentType != core.IntentAskQuestion {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentAskQuestion)
	}
}

func TestAnalyzeIntent_ReturnHistorical_MeetsThreshold(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	// confidence 0.67 >= 0.65 (confidenceThresholdReturnHistory)
	mock.QueueResponse(intentResponse("createTicket", workflow.StepUserAsksQuestion, 0.67, "return to ticket topic", ""))

	ctx := core.NewConversationContext()
	ctx.PushWorkflowHistory(&core.WorkflowHistoryEntry{
		WorkflowName: "createTicket",
		CompletedAt:  time.Now().Add(-10 * time.Minute),
	})

	intent := AnalyzeIntent(newMsg("back to the ticket"), ctx)

	if intent.IntentType != core.IntentNewWorkflow {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentNewWorkflow)
	}
	if intent.WorkflowName != "createTicket" {
		t.Errorf("WorkflowName = %q, want createTicket", intent.WorkflowName)
	}
	if intent.Step != workflow.StepUserAsksQuestion {
		t.Errorf("Step = %q, want %q", intent.Step, workflow.StepUserAsksQuestion)
	}
}

func TestAnalyzeIntent_ReturnHistorical_TooLow(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	// confidence 0.60 < 0.65 threshold
	mock.QueueResponse(intentResponse("createTicket", workflow.StepUserAsksQuestion, 0.60, "maybe", ""))

	ctx := core.NewConversationContext()
	ctx.PushWorkflowHistory(&core.WorkflowHistoryEntry{
		WorkflowName: "createTicket",
		CompletedAt:  time.Now().Add(-5 * time.Minute),
	})

	intent := AnalyzeIntent(newMsg("something vague"), ctx)

	if intent.IntentType != core.IntentAskQuestion {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentAskQuestion)
	}
}

func TestAnalyzeIntent_SameWorkflow_SideQuestion(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	mock.QueueResponse(intentResponse("testAI", workflow.StepUserAsksQuestion, 0.80, "side question", ""))

	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusRunning)

	intent := AnalyzeIntent(newMsg("what is 2+2?"), ctx)

	if intent.IntentType != core.IntentAskQuestion {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentAskQuestion)
	}
	if intent.Step != workflow.StepUserAsksQuestion {
		t.Errorf("Step = %q, want %q", intent.Step, workflow.StepUserAsksQuestion)
	}
}

func TestAnalyzeIntent_WaitForUser_AnswerQuestion(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	mock.QueueResponse(intentResponse("testAI", workflow.StepUserAsksQuestion, 0.80, "answer", ""))

	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusWaitForUser)

	intent := AnalyzeIntent(newMsg("yes, that's what I meant"), ctx)

	if intent.IntentType != core.IntentAnswerQuestion {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentAnswerQuestion)
	}
	if intent.Step != workflow.StepUserAnsweringQuestion {
		t.Errorf("Step = %q, want %q", intent.Step, workflow.StepUserAnsweringQuestion)
	}
}

func TestAnalyzeIntent_NotWaiting_AnsweringStepOverridden(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	// AI suggests answering_question but status is idle — must be treated as side question
	mock.QueueResponse(intentResponse("testAI", workflow.StepUserAnsweringQuestion, 0.80, "answer", ""))

	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusIdle)

	intent := AnalyzeIntent(newMsg("some follow-up"), ctx)

	if intent.IntentType != core.IntentAskQuestion {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentAskQuestion)
	}
	if intent.Step != workflow.StepUserAsksQuestion {
		t.Errorf("Step = %q, want %q", intent.Step, workflow.StepUserAsksQuestion)
	}
}

func TestAnalyzeIntent_StayActiveWorkflow_LowSwitchConf(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	// 0.85 < 0.90 new-wf threshold; createTicket is NOT in history → stays in testAI
	mock.QueueResponse(intentResponse("createTicket", workflow.StepInit, 0.85, "maybe ticket", ""))

	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusRunning)

	intent := AnalyzeIntent(newMsg("create a ticket maybe"), ctx)

	if intent.WorkflowName != "testAI" {
		t.Errorf("WorkflowName = %q, want testAI (should stay)", intent.WorkflowName)
	}
	if intent.IntentType != core.IntentAskQuestion {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentAskQuestion)
	}
}

func TestAnalyzeIntent_SwitchToNewWorkflow_HighConf(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	// 0.92 >= 0.90 — switch to brand-new workflow
	mock.QueueResponse(intentResponse("createTicket", workflow.StepInit, 0.92, "clear switch signal", ""))

	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusRunning)

	intent := AnalyzeIntent(newMsg("switch to ticket creation"), ctx)

	if intent.IntentType != core.IntentNewWorkflow {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentNewWorkflow)
	}
	if intent.WorkflowName != "createTicket" {
		t.Errorf("WorkflowName = %q, want createTicket", intent.WorkflowName)
	}
}

func TestAnalyzeIntent_SwitchToHistoricalWorkflow(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	// 0.83 >= 0.82 historical threshold — switch to known workflow
	mock.QueueResponse(intentResponse("createTicket", workflow.StepUserAsksQuestion, 0.83, "back to tickets", ""))

	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetCurrentStatus(core.StatusRunning)
	ctx.PushWorkflowHistory(&core.WorkflowHistoryEntry{
		WorkflowName: "createTicket",
		CompletedAt:  time.Now().Add(-20 * time.Minute),
	})

	intent := AnalyzeIntent(newMsg("go back to the ticket we were creating"), ctx)

	if intent.IntentType != core.IntentNewWorkflow {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentNewWorkflow)
	}
	if intent.WorkflowName != "createTicket" {
		t.Errorf("WorkflowName = %q, want createTicket", intent.WorkflowName)
	}
}

func TestAnalyzeIntent_ClarifyingQuestion_Fires(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	// Low confidence + clarifying question + NOT pending clarification
	mock.QueueResponse(intentResponse("testAI", workflow.StepInit, 0.50, "unsure", "Did you mean X?"))

	ctx := core.NewConversationContext() // pendingClarification=false by default
	intent := AnalyzeIntent(newMsg("do the thing"), ctx)

	if intent.IntentType != core.IntentAskQuestion {
		t.Errorf("IntentType = %q, want %q", intent.IntentType, core.IntentAskQuestion)
	}
	if !intent.NeedsUserInput {
		t.Error("expected NeedsUserInput=true for clarifying question")
	}
	if intent.MessageToUser == nil || *intent.MessageToUser != "Did you mean X?" {
		t.Errorf("MessageToUser = %v, want %q", intent.MessageToUser, "Did you mean X?")
	}
}

func TestAnalyzeIntent_ClarifyingQuestion_SuppressedWhenPending(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	mock.QueueResponse(intentResponse("testAI", workflow.StepInit, 0.50, "still unsure", "Did you mean X?"))

	ctx := core.NewConversationContext()
	ctx.SetPendingClarification(true) // already waiting — must not ask again

	intent := AnalyzeIntent(newMsg("do the thing again"), ctx)

	if intent.NeedsUserInput {
		t.Error("expected NeedsUserInput=false when clarification already pending")
	}
}

func TestAnalyzeIntent_ClarifyingQuestion_SuppressedWhenHighConf(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	// High confidence overrides the clarifying question the AI included
	mock.QueueResponse(intentResponse("testAI", workflow.StepInit, 0.90, "very confident", "Did you mean X?"))

	ctx := core.NewConversationContext()
	intent := AnalyzeIntent(newMsg("run test AI"), ctx)

	// Confidence 0.90 >= 0.65 threshold → clarifying question is suppressed
	if intent.NeedsUserInput {
		t.Error("expected NeedsUserInput=false at high confidence even if AI included a question")
	}
}
