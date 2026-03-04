package workflow

import (
	"testing"

	"bob/internal/ai/aimock"
	"bob/internal/orchestrator/core"
)

// -----------------------------------------------------------------------
// Pure helpers
// -----------------------------------------------------------------------

func TestSubWorkerKey(t *testing.T) {
	got := subWorkerKey("3", "result")
	want := "sw_3_result"
	if got != want {
		t.Errorf("subWorkerKey(3, result) = %q, want %q", got, want)
	}
}

func TestAvailableWorkflows_ExcludesInternal(t *testing.T) {
	list := AvailableWorkflows()

	// testSubWorker is Internal=true — must not appear
	for _, wf := range list {
		if wf.Name == WorkflowTestSubWorker {
			t.Errorf("internal workflow %q should not appear in AvailableWorkflows()", WorkflowTestSubWorker)
		}
	}

	// testSubworkflows is NOT internal — must be present
	found := false
	for _, wf := range list {
		if wf.Name == WorkflowTestSubworkflows {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("workflow %q should appear in AvailableWorkflows()", WorkflowTestSubworkflows)
	}
}

// -----------------------------------------------------------------------
// handleDefaultSteps
// -----------------------------------------------------------------------

func TestHandleDefaultSteps_StepInit(t *testing.T) {
	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))

	action := core.NewAction(core.ActionWorkflow)
	action.Input = map[core.InputType]any{core.InputStep: StepInit}

	def := workflows[WorkflowTestAI]
	actions, skip, err := handleDefaultSteps(def, ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Error("StepInit should not skip the workflow")
	}
	if actions != nil {
		t.Errorf("StepInit should return nil actions, got %v", actions)
	}
}

func TestHandleDefaultSteps_StepUserAsksQuestion(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	mock.QueueResponse(map[string]any{"message": "2 + 2 = 4"})

	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetLastUserMessages([]*core.Message{{Message: "what is 2+2?"}})

	action := core.NewAction(core.ActionWorkflow)
	action.Input = map[core.InputType]any{core.InputStep: StepUserAsksQuestion}

	def := workflows[WorkflowTestAI]
	actions, skip, err := handleDefaultSteps(def, ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Error("StepUserAsksQuestion should skip the workflow")
	}
	if len(actions) == 0 {
		t.Error("expected at least one action for side question")
	}
}

func TestHandleDefaultSteps_StepUserAnsweringQuestion(t *testing.T) {
	mock := aimock.New()
	aimock.Install(t, mock)
	mock.QueueResponse(map[string]any{"message": "Got it, thanks for clarifying"})

	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))
	ctx.SetLastUserMessages([]*core.Message{{Message: "I meant yes"}})

	action := core.NewAction(core.ActionWorkflow)
	action.Input = map[core.InputType]any{core.InputStep: StepUserAnsweringQuestion}

	def := workflows[WorkflowTestAI]
	actions, skip, err := handleDefaultSteps(def, ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Error("StepUserAnsweringQuestion should skip the workflow by default")
	}
	if len(actions) == 0 {
		t.Error("expected at least one action for answer-question handling")
	}
}

func TestHandleDefaultSteps_OtherStep_NoAction(t *testing.T) {
	ctx := core.NewConversationContext()
	ctx.SetCurrentWorkflow(core.NewWorkflow("testAI"))

	action := core.NewAction(core.ActionWorkflow)
	action.Input = map[core.InputType]any{core.InputStep: "some_workflow_specific_step"}

	def := workflows[WorkflowTestAI]
	actions, skip, err := handleDefaultSteps(def, ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Error("unknown step should not skip the workflow")
	}
	if actions != nil {
		t.Errorf("unknown step should return nil actions, got %v", actions)
	}
}
