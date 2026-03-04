// Package workflow contains all the logic for all the
package workflow

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"fmt"
)

// -----------------------------------------------------------------
// All you need to do is define workflows here.

const (
	WorkflowCreateTicket    WorkflowName = "createTicket"
	WorkflowQueryTicket     WorkflowName = "queryTicket"
	WorkflowTestAI          WorkflowName = "testAI"
	WorkflowTestSubworkflows WorkflowName = "testSubworkflows"
	WorkflowTestSubWorker   WorkflowName = "testSubWorker"
)

var workflows = map[WorkflowName]WorkflowDefinition{
	WorkflowCreateTicket: {
		Description: "Create, make, open, or submit a new Azure DevOps (ADO) work item/ticket. Use when user wants to create new tickets. Keywords: create, make, new, open, submit, add ticket/work item/task/bug/story.",
		WorkflowFn:  CreateTicket,
		AvailableSteps: []string{},
	},
	WorkflowQueryTicket: {
		Description: "Query, search, find, lookup, retrieve, view, or get an Azure DevOps (ADO) work item/ticket by ID or description. Use when user wants to fetch/check/see existing tickets. Keywords: query, search, find, get, lookup, retrieve, show, view, check, fetch, pull ticket/work item.",
		WorkflowFn:  QueryTicket,
		AvailableSteps: []string{},
	},
	WorkflowTestAI: {
		Description: "General AI conversation and testing. Use for general questions, testing, or when no other workflow matches. Keywords: test, chat, ask, general questions.",
		WorkflowFn:  TestAI,
		AvailableSteps: []string{"handle_async_results", "call_tool"},
	},
	WorkflowTestSubworkflows: {
		Description: "Tests sub-workflow dispatch, async execution, personality registry, and context propagation. Trigger with: 'test subworkflows'.",
		WorkflowFn:  TestSubworkflows,
		AvailableSteps: []string{StepTswSpawnWorkers, StepTswCollectResult, StepTswSendSummary},
	},
	WorkflowTestSubWorker: {
		Description: "Internal sub-worker for testSubworkflows.",
		Internal:    true,
		WorkflowFn:  TestSubWorker,
		AvailableSteps: []string{StepSubWorkerRun},
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
	Description string
	AvailableSteps []string
	Internal bool // Internal workflows are not user-triggerable and hidden from the intent analyzer

	WorkflowFn func(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error)
	Options map[Option]any
}


// RunWorkflow will run any workflow
func RunWorkflow(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	// Check if the action specifies a target workflow (sub-workflow dispatch)
	var wf WorkflowName
	if sourceAction.Input != nil {
		if name, ok := sourceAction.Input[core.InputWorkflowName].(WorkflowName); ok && name != "" {
			wf = name
		}
	}
	if wf == "" {
		cw := context.GetCurrentWorkflow()
		if cw == nil {
			return nil, fmt.Errorf("no current workflow set")
		}
		wf = WorkflowName(cw.GetWorkflowName())
	}
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
		// Treat as a side question by default — the user's reply goes through the main
		// conversation thread so the AI has full context to respond naturally.
		// Workflows that need to process a specific user answer (i.e. they called
		// ActionUserWait themselves) should set OptionOverwriteHandleDefaultSteps and
		// handle StepUserAnsweringQuestion directly in their WorkflowFn.
		actions, err := handleSideQuestion(c, w, a)
		if err != nil {
			return nil, false, err
		}
		return actions, true, nil

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

// handleSideQuestion handles user's side question with prepared context
func handleSideQuestion(c *core.ConversationContext, w WorkflowDefinition, a *core.Action) ([]*core.Action, error) {
	// Get user's question from last message
	messages := c.GetLastUserMessages()
	if len(messages) == 0 {
		return nil, fmt.Errorf("no user messages found")
	}
	userMessage := messages[len(messages)-1].Message

	workflow := c.GetCurrentWorkflow()
	if workflow == nil {
		return nil, fmt.Errorf("no current workflow")
	}

	// Check if this is an AI response being returned
	aiResponse := getInput(a, core.InputAIResponse)
	if aiResponse != nil {
		// Handle AI response - extract message and send to user
		response, ok := aiResponse.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("invalid AI response type")
		}

		message, err := response.Data().GetString("message")
		if err != nil {
			return nil, fmt.Errorf("failed to get message from AI response: %w", err)
		}

		// Send the AI's response back to the user
		userMessageAction := core.NewAction(core.ActionUserMessage)
		if userMessageAction.Input == nil {
			userMessageAction.Input = make(map[core.InputType]any)
		}
		userMessageAction.Input[core.InputMessage] = message

		return []*core.Action{userMessageAction}, nil
	}

	// Create AI request for side question
	schema := ai.NewSchema().
		AddString("message", ai.Required(), ai.Description("The AI assistant's response to the user's question"))

	systemPrompt := fmt.Sprintf("You are a helpful assistant. The user is currently working in the '%s' workflow: %s\n\nAnswer their question concisely while being aware of their current context.",
		workflow.GetWorkflowName(), w.Description)

	// Use main conversation (empty key) to maintain conversation history across workflow interactions
	// Custom keys should only be used when workflows explicitly need isolated AI contexts
	conversationKey := ""

	actions := askAI(userMessage, systemPrompt, "", schema, conversationKey)
	// Set the step on the AI action if not already set by the workflow
	if len(actions) > 0 && actions[0].Input != nil {
		if _, hasStep := actions[0].Input[core.InputStep]; !hasStep {
			actions[0].Input[core.InputStep] = StepUserAsksQuestion
		}
	}
	return actions, nil
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
		if def.Internal {
			continue
		}
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
