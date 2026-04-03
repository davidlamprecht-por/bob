package workflow

import (
	"bob/definitions/personalities"
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"fmt"
)

const (
	WorkflowTestSubWorkflow   WorkflowName = "testSubWorkflow"
	WorkflowSubContextChecker WorkflowName = "subContextChecker"

	StepSubLaunchWorker   = "sub_launch_worker"
	StepSubHandleResponse = "sub_handle_response"
)

// TestSubWorkflow tests sub-workflow dispatch and conversation context branching.
//
// Flow:
//  1. Sends a message on the main conversation, asking the AI to pick a secret animal
//     (establishes mainConversationID and lastResponseID on the thread context).
//  2. Launches SubContextChecker as an async sub-workflow.
//  3. SubContextChecker branches its AI conversation from the main conv's lastResponseID
//     and asks the AI to recall the secret animal — proving context is inherited.
func TestSubWorkflow(ctx *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🔷 TestSubWorkflow: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🔷 TestSubWorkflow: step=%v", step)

	switch step {
	case StepInit:
		messages := ctx.GetLastUserMessages()
		if len(messages) == 0 {
			return nil, fmt.Errorf("no user message found")
		}
		userMessage := messages[len(messages)-1].Message

		schema := ai.NewSchema().
			AddString("message", ai.Required(), ai.Description("Your response to the user"))

		actions := askAI(
			userMessage+"\n\n(Also pick a random secret animal and mention it in your response — this is part of a context-branching test.)",
			personalities.PersonalitySecretAnimal.Render(nil),
			schema,
			"", // main conversation — establishes mainConversationID + lastResponseID
		)
		actions[0].Input[core.InputStep] = StepSubLaunchWorker
		return actions, nil

	case StepSubLaunchWorker:
		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			return nil, fmt.Errorf("expected AI response")
		}
		response, ok := aiResponse.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("invalid AI response type")
		}
		message, err := response.Data().GetString("message")
		if err != nil {
			return nil, fmt.Errorf("failed to get main AI message: %w", err)
		}

		// Show the main AI response to the user.
		msgAction := core.NewAction(core.ActionUserMessage)
		msgAction.Input = map[core.InputType]any{
			core.InputMessage: fmt.Sprintf("**Main conversation response:**\n%s", message),
		}

		// Dispatch SubContextChecker as an async sub-workflow.
		// It will branch its conversation from the main conv's lastResponseID and verify
		// that it can see the secret animal picked above.
		subAction := core.NewAction(core.ActionWorkflow)
		subAction.Input = map[core.InputType]any{
			core.InputWorkflowName: WorkflowSubContextChecker,
			core.InputStep:         StepInit,
		}
		asyncAction := core.NewAction(core.ActionAsync)
		asyncAction.AsyncActions = []*core.Action{subAction}

		logger.Debug("🔷 TestSubWorkflow: Launching SubContextChecker to verify context branching")
		return []*core.Action{msgAction, asyncAction}, nil

	default:
		return nil, fmt.Errorf("unknown step: %v", step)
	}
}

// SubContextChecker is an internal sub-workflow that verifies conversation context branching.
//
// On its first AI call it uses a named conversation key ("sub_branch"), which causes ActionAI
// to branch from the main conversation's lastResponseID. It then asks the AI to recall the
// secret animal mentioned in the previous response. If the AI can answer correctly, context
// branching is working as expected.
func SubContextChecker(ctx *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🔶 SubContextChecker: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🔶 SubContextChecker: step=%v", step)

	switch step {
	case StepInit:
		schema := ai.NewSchema().
			AddString("message", ai.Required(), ai.Description("Your response referencing what was said in the prior conversation"))

		actions := askAI(
			"Look at the conversation history. What secret animal was mentioned in the previous response? This is a context-branching verification test — answer based on what you can see in the conversation.",
			personalities.PersonalityContextVerifier.Render(nil),
			schema,
			"sub_branch", // Named key → branches from main conv's lastResponseID on first call
		)
		// Route the AI result back to this sub-workflow at StepSubHandleResponse.
		actions[0].Input[core.InputWorkflowName] = WorkflowSubContextChecker
		actions[0].Input[core.InputStep] = StepSubHandleResponse
		return actions, nil

	case StepSubHandleResponse:
		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			return nil, fmt.Errorf("expected AI response in sub-workflow")
		}
		response, ok := aiResponse.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("invalid AI response type")
		}
		message, err := response.Data().GetString("message")
		if err != nil {
			return nil, fmt.Errorf("failed to get sub-workflow AI message: %w", err)
		}

		msgAction := core.NewAction(core.ActionUserMessage)
		msgAction.Input = map[core.InputType]any{
			core.InputMessage: fmt.Sprintf(
				"**Sub-workflow (branched context) response:**\n%s\n\n_If the sub-workflow correctly identified the secret animal, context branching is confirmed working!_",
				message,
			),
		}
		completeAction := core.NewAction(core.ActionCompleteAsync)

		return []*core.Action{msgAction, completeAction}, nil

	default:
		return nil, fmt.Errorf("unknown sub-workflow step: %v", step)
	}
}
