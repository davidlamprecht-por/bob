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
		AvailableSteps: []string{},
	},
	WorkflowQueryTicket: {
		Description: "This workflow aims to fetch an ado ticket for the user by given ADO ticket id or or generic description",
		WorkflowFn:  QueryTicket,
		AvailableSteps: []string{},
	},
}

// -----------------------------------------------------------------

type WorkflowName string

const (
	// Default Steps
	StepInit = "init"
	StepUserAsksQuestion = "asking_question"
	StepUserAnsweringQuestion = "answering_question"

	// Workfow Specific Steps...
)

type Option string

const (
	OptionOverwriteHandleDefaultSteps Option = "overwrite_handle_default_steps" // Assign anything but false
)

type WorkflowDefinition struct {
	Name        WorkflowName
	Description string
	AvailableSteps []string

	WorkflowFn func(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error)
	Options map[Option]any
}


// RunWorkflow will run any workflow
func RunWorkflow(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	cw := context.GetCurrentWorkflow()
	if cw == nil {
		return nil, fmt.Errorf("no current workflow set")
	}
	wf := WorkflowName(cw.GetWorkflowName())
	workflow, ok := workflows[wf]
	if !ok {
		return nil, fmt.Errorf("unknown workflow: %q", wf)
	}

	// Handle default steps first
	defaultActions, skipWorkflow, err := handleDefaultSteps(workflow, context, sourceAction)
	if err != nil {
		return nil, err
	}

	// If default handling says skip workflow, return just default actions
	if skipWorkflow {
		return defaultActions, nil
	}

	// Otherwise, call workflow and return its actions
	workflowActions, err := workflow.WorkflowFn(context, sourceAction)
	if err != nil {
		return nil, err
	}
	if defaultActions != nil {
		return append(defaultActions, workflowActions...), nil
	}
	return workflowActions, nil
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
func handleDefaultSteps(w WorkflowDefinition, c *core.ConversationContext, a *core.Action) ([]*core.Action, bool, error) {
	// Check if workflow opts out of default handling
	if overwrite, ok := w.Options[OptionOverwriteHandleDefaultSteps]; ok && overwrite != false {
		return nil, false, nil
	}

	step := getInput(a, core.InputStep)
	switch step {
	case StepInit:
		// Reset workflow data for fresh start
		if err := resetWorkflowData(c); err != nil {
			return nil, false, err
		}

		// Reset AI conversation for this workflow (will be newly generated at next requets)
		c.GetCurrentWorkflow().SetAIConversation(nil, nil)

		// Let workflow continue with initialization
		return nil, false, nil

	case StepUserAnsweringQuestion:
		// Let workflow handle the user's answer
		return nil, false, nil

	case StepUserAsksQuestion:
		// Handle side question with proper context preparation
		actions, err := handleSideQuestion(c, w, a)
		if err != nil {
			return nil, false, err
		}

		// Skip workflow execution this turn
		return actions, true, nil
	}

	// Default: no special handling
	return nil, false, nil
}

// resetWorkflowData clears workflow data to prepare for fresh workflow run
func resetWorkflowData(c *core.ConversationContext) error {
	workflow := c.GetCurrentWorkflow()
	if workflow == nil {
		return fmt.Errorf("no current workflow to reset")
	}

	// Clear all workflow data
	workflow.ResetWorkflowData()

	return nil
}

// handleSideQuestion handles user's side question with prepared context (stub)
func handleSideQuestion(c *core.ConversationContext, w WorkflowDefinition, a *core.Action) ([]*core.Action, error) {
	// Get user's question from last message
	messages := c.GetLastUserMessages()
	if len(messages) == 0 {
		return nil, fmt.Errorf("no user messages found")
	}
	_ = messages[len(messages)-1] // lastMessage - TODO: Use this when integrating AI service

	workflow := c.GetCurrentWorkflow()
	if workflow == nil {
		return nil, fmt.Errorf("no current workflow")
	}

	// TODO: Integrate with real AI service

	return []*core.Action{}, nil
}

// To pass to ai...

type WorkflowInfo struct {
	Name        WorkflowName `json:"name"`
	Description string       `json:"description"`
	AvailableSteps []string
}

func AvailableWorkflows() []WorkflowInfo {
	out := make([]WorkflowInfo, 0, len(workflows))
	for name, def := range workflows {
		out = append(out, WorkflowInfo{
			Name: name,
			Description: def.Description,
			AvailableSteps: def.AvailableSteps,
		})
	}
	return out
}

// GetAvailableWorkflowContext is a function that can be passed to ai to help identify what it can choose to do!
func GetAvailableWorkflowContext() string {
	context := "## Available Workflows\n\n"

	// Add default steps information
	context += "### Default Steps (Available in ALL Workflows)\n"
	context += "These steps are automatically available in every workflow:\n"
	context += fmt.Sprintf("- **%s**: Initialize a new workflow. Cleans up previous workflow state and starts fresh.\n", StepInit)
	context += fmt.Sprintf("- **%s**: User is asking a clarifying or side question related to the current workflow. Handles the question without interrupting workflow progress.\n", StepUserAsksQuestion)
	context += fmt.Sprintf("- **%s**: User is responding to a question posed by the workflow. Used when workflow needs information from the user.\n\n", StepUserAnsweringQuestion)

	// Add workflow-specific information
	context += "### Workflow Options\n"
	workflows := AvailableWorkflows()

	if len(workflows) == 0 {
		context += "No workflows currently available.\n"
		return context
	}

	for i, wf := range workflows {
		context += fmt.Sprintf("\n**%d. Workflow: %s**\n", i+1, wf.Name)
		context += fmt.Sprintf("   Description: %s\n", wf.Description)

		if len(wf.AvailableSteps) > 0 {
			context += "   Additional Available Steps for this specific Workflow:\n"
			for _, step := range wf.AvailableSteps {
				context += fmt.Sprintf("   - %s\n", step)
			}
		} else {
			context += "   Additional Steps: None (uses only default steps)\n"
		}
	}

	return context
}
