package workflow

import (
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/tool"
	"fmt"
)

const (
	StepHandleAsyncResults = "handle_async_results"
	StepCallTool           = "call_tool"
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
		aiAction1.Input[core.InputSystemPrompt] = "You are a helpful test assistant."
		aiAction1.Input[core.InputPersonality] = ""
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
		aiAction2.Input[core.InputSystemPrompt] = "You pick random categories. Respond only with one of the allowed category names."
		aiAction2.Input[core.InputPersonality] = ""
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
			// Both received! Call the tool and signal async completion
			logger.Debug("🔷 TestAI workflow: Both async results received, calling tool")
			category := workflow.GetWorkflowData("category").(string)

			toolAction := core.NewAction(core.ActionTool)
			if toolAction.Input == nil {
				toolAction.Input = make(map[core.InputType]any)
			}
			toolAction.Input[core.InputStep] = StepCallTool
			toolAction.Input[core.InputToolName] = tool.ToolSampleData
			toolAction.Input[core.InputToolArgs] = map[string]any{"category": category}
			toolAction.SourceWorkflow = "testAI"

			// Create complete async signal
			completeAction := core.NewAction(core.ActionCompleteAsync)

			// Wrap both in an async container
			asyncAction := core.NewAction(core.ActionAsync)
			asyncAction.AsyncActions = []*core.Action{toolAction, completeAction}

			logger.Debug("🔷 TestAI workflow: Returning tool action + async complete signal")
			return []*core.Action{asyncAction}, nil
		}

		// Still waiting for the other result
		logger.Debug("🔷 TestAI workflow: Waiting for more async results")
		return nil, nil

	case StepCallTool:
		logger.Debug("🔷 TestAI workflow: StepCallTool - Handling tool result")
		// Handle tool result
		toolResult := getInput(sourceAction, core.InputToolResult)
		if toolResult == nil {
			logger.Error("❌ TestAI workflow: No tool result")
			return nil, fmt.Errorf("expected tool result but got none")
		}

		resultMap, ok := toolResult.(map[string]any)
		if !ok {
			logger.Errorf("❌ TestAI workflow: Invalid tool result type: %T", toolResult)
			return nil, fmt.Errorf("invalid tool result type")
		}

		sampleDataResult := resultMap["result"].(string)
		logger.Debugf("🔷 TestAI workflow: Got tool result: %s", sampleDataResult)

		// Get the stored category
		category := workflow.GetWorkflowData("category").(string)

		// Send first message: just the sample data (non-blocking)
		sampleDataMessage := fmt.Sprintf("**Random Sample Data (category: %s):**\n%s", category, sampleDataResult)

		sampleDataAction := core.NewAction(core.ActionUserMessage)
		if sampleDataAction.Input == nil {
			sampleDataAction.Input = make(map[core.InputType]any)
		}
		sampleDataAction.Input[core.InputMessage] = sampleDataMessage

		// Send second message: ask how to assist (blocking)
		waitMessage := "How can I assist you?"

		waitAction := core.NewAction(core.ActionUserWait)
		if waitAction.Input == nil {
			waitAction.Input = make(map[core.InputType]any)
		}
		waitAction.Input[core.InputMessage] = waitMessage

		logger.Debug("🔷 TestAI workflow: Sending sample data + wait for user")
		return []*core.Action{sampleDataAction, waitAction}, nil

	default:
		logger.Warnf("⚠️  TestAI workflow: Unknown step: %v", step)
		return nil, fmt.Errorf("unknown workflow step: %v", step)
	}
}
