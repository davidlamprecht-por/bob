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

type WorkflowDefinition struct {
	Name        WorkflowName
	Description string

	WorkflowFn func(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error)
	// Later some other restrictions etc that ai should know about this workflow
}


// RunWorkflow will run any workflow
func RunWorkflow(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	cw := context.GetCurrentWorkflow()
	if cw == nil {
		return nil, fmt.Errorf("no current workflow set")
	}
	wf := WorkflowName(cw.WorkflowName)
	if workflow, ok := workflows[wf]; ok {
		return workflow.WorkflowFn(context, sourceAction)
	}
	return nil, fmt.Errorf("unknown workflow: %q", wf)
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
