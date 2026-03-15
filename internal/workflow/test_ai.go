package workflow

import (
	"bob/definitions/personalities"
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"fmt"
)

const (
	StepHandleAsyncResults = "handle_async_results"
	StepSendResults        = "send_results"
)

// TestAI tests async execution and tool calling
func TestAI(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🔷 TestAI workflow: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🔷 TestAI workflow: step=%v", step)

	workflow := context.GetCurrentWorkflow()

	switch step {
	case StepInit:
		logger.Debug("🔷 TestAI workflow: StepInit - Creating async AI requests")
		// Get the user's message
		messages := context.GetLastUserMessages()
		if len(messages) == 0 {
			logger.Error("❌ TestAI workflow: No user messages found")
			return nil, fmt.Errorf("no user message found")
		}
		userMessage := messages[len(messages)-1].Message
		logger.Debugf("🔷 TestAI workflow: User message: %q", userMessage)

		// Create ActionAsync with two parallel AI requests
		asyncAction := core.NewAction(core.ActionAsync)

		// AI Request 1: Normal response to user (with conversation)
		schema1 := ai.NewSchema().
			AddString("message", ai.Required(), ai.Description("The AI assistant's response message"))
		aiAction1 := core.NewAction(core.ActionAi)
		if aiAction1.Input == nil {
			aiAction1.Input = make(map[core.InputType]any)
		}
		aiAction1.Input[core.InputStep] = StepHandleAsyncResults
		aiAction1.Input[core.InputMessage] = userMessage
		aiAction1.Input[core.InputPersonality] = personalities.SystemPrompt("You are a helpful test assistant.")
		aiAction1.Input[core.InputSchema] = schema1
		aiAction1.Input[core.InputConversationKey] = "" // Main conversation
		aiAction1.SourceWorkflow = "testAI"

		// AI Request 2: Contextless - ask which category to use
		schema2 := ai.NewSchema().
			AddString("category", ai.Required(),
				ai.Enum("greetings", "weather", "dinner"),
				ai.Description("The sample data category to retrieve"))
		aiAction2 := core.NewAction(core.ActionAi)
		if aiAction2.Input == nil {
			aiAction2.Input = make(map[core.InputType]any)
		}
		aiAction2.Input[core.InputStep] = StepHandleAsyncResults
		aiAction2.Input[core.InputMessage] = "Pick a random category from: greetings, weather, or dinner. Just respond with the category name."
		aiAction2.Input[core.InputPersonality] = personalities.SystemPrompt("You pick random categories. Respond only with one of the allowed category names.")
		aiAction2.Input[core.InputSchema] = schema2
		aiAction2.Input[core.InputConversationKey] = "category_picker" // Separate conversation
		aiAction2.SourceWorkflow = "testAI"

		asyncAction.AsyncActions = []*core.Action{aiAction1, aiAction2}
		asyncAction.AsyncGroupID = "test_ai_async"
		asyncAction.AsyncGroupSize = 2

		logger.Debug("🔷 TestAI workflow: Returning async action with 2 AI requests")
		return []*core.Action{asyncAction}, nil

	case StepHandleAsyncResults:
		logger.Debug("🔷 TestAI workflow: StepHandleAsyncResults - Collecting async results")
		// Handle async results coming back
		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			logger.Error("❌ TestAI workflow: No AI response in action input")
			return nil, fmt.Errorf("expected AI response but got none")
		}

		response, ok := aiResponse.(*ai.Response)
		if !ok {
			logger.Errorf("❌ TestAI workflow: Invalid AI response type: %T", aiResponse)
			return nil, fmt.Errorf("invalid AI response type")
		}

		// Determine which response this is based on what data it has
		if response.Data().Has("category") {
			// This is the category picker response
			category, _ := response.Data().GetString("category")
			workflow.SetWorkflowData("category", category)
			logger.Debugf("🔷 TestAI workflow: Got category response: %s", category)
		} else {
			// This is the main AI response
			message, _ := response.Data().GetString("message")
			workflow.SetWorkflowData("main_message", message)
			logger.Debugf("🔷 TestAI workflow: Got main AI response: %s", message)
		}

		// Check if we have both responses
		hasCategory := workflow.GetWorkflowData("category") != nil
		hasMainMessage := workflow.GetWorkflowData("main_message") != nil

		if hasCategory && hasMainMessage {
			// Both received! Send combined response
			logger.Debug("🔷 TestAI workflow: Both async results received")
			category := workflow.GetWorkflowData("category").(string)
			mainMessage := workflow.GetWorkflowData("main_message").(string)

			// Send combined message
			combinedMessage := fmt.Sprintf("**AI Response:**\n%s\n\n**Random Category Selected:** %s", mainMessage, category)

			messageAction := core.NewAction(core.ActionUserMessage)
			if messageAction.Input == nil {
				messageAction.Input = make(map[core.InputType]any)
			}
			messageAction.Input[core.InputMessage] = combinedMessage

			// Send wait message
			waitAction := core.NewAction(core.ActionUserWait)
			if waitAction.Input == nil {
				waitAction.Input = make(map[core.InputType]any)
			}
			waitAction.Input[core.InputMessage] = "How can I assist you?"

			// Create complete async signal
			completeAction := core.NewAction(core.ActionCompleteAsync)

			logger.Debug("🔷 TestAI workflow: Returning messages + async complete signal")
			return []*core.Action{messageAction, waitAction, completeAction}, nil
		}

		// Still waiting for the other result
		logger.Debug("🔷 TestAI workflow: Waiting for more async results")
		return nil, nil

	default:
		logger.Warnf("⚠️  TestAI workflow: Unknown step: %v", step)
		return nil, fmt.Errorf("unknown workflow step: %v", step)
	}
}
