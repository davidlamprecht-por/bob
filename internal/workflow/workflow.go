// Package workflow contains all the logic for all the
package workflow

import (
	"bob/internal/orchestrator/core"
	"fmt"
)

// -----------------------------------------------------------------
// All you need to do is define workflows here.

const (
	WorkflowCreateTicket WorkflowName = "createTicket"
	WorkflowQueryTicket  WorkflowName = "queryTicket"
)

var workflows = map[WorkflowName]WorkflowDefinition{
	WorkflowCreateTicket: {
		Description: "This workflow guides the user to create a new ADO work ticket",
		WorkflowFn:  CreateTicket,
	},
	WorkflowQueryTicket: {
		Description: "This workflow aims to fetch an ado ticket for the user by given ADO ticket id or or generic description",
		WorkflowFn:  QueryTicket,
	},
}

// -----------------------------------------------------------------

type WorkflowName string

const (
	StepInit = "init"
	StepUserAsksQuestion = "asking_question"
	StepUserAnsweringQuestion = "answering_question"
)

type Option string

const (
	OptionOverwriteHandleDefaultSteps Option = "overwrite_handle_default_steps" // Assign anything but false
)

type WorkflowDefinition struct {
	Name        WorkflowName
	Description string

	WorkflowFn func(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error)
	Options map[Option]any
	// Later some other restrictions etc that ai should know about this workflow
}


// RunWorkflow will run any workflow
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
	// Run Workflow here
	handleDefaultSteps(workflow, context, sourceAction)
	return workflow.WorkflowFn(context, sourceAction)
}

func init() {
	for name, def := range workflows {
		if def.WorkflowFn == nil {
			panic(fmt.Sprintf("workflow %q has nil WorkflowFn", name))
		}
		if def.Description == "" {
			panic(fmt.Sprintf("workflow %q has empty Description", name))
		}
	}
}

/* handleDefaultSteps will make sure that every workflow allways allows these steps:
- Init (new workflow has started. Likely do some cleanup, workflow specific init can be done at the beginning of workflow)
- StepUserAsksQuestion (This should intersect the Workflow without interuppting it and allow for side questions)
*/
func handleDefaultSteps(w *WorkflowDefinition, c *core.ConversationContext, a *core.Action){
	if overwrite, ok := w.Options[OptionOverwriteHandleDefaultSteps] ; ok && overwrite != false{
		return
	}
	step := getInput(a, core.InputStep)
	switch step{
	case StepInit:
	case StepUserAnsweringQuestion:
	case StepUserAsksQuestion:
		askAI()
	}
}

// To pass to ai...

type WorkflowInfo struct {
	Name        WorkflowName `json:"name"`
	Description string       `json:"description"`
}

func AvailableWorkflows() []WorkflowInfo {
	out := make([]WorkflowInfo, 0, len(workflows))
	for name, def := range workflows {
		out = append(out, WorkflowInfo{
			Name: name,
			Description: def.Description,
		})
	}
	return out
}
