package workflow

import (
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/tools"
	"fmt"
)

// TestTools is a test workflow that exercises the tool system
// It calls the echo tool and calculator tool to validate tool execution
func TestTools(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🧪 TestTools workflow: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🧪 TestTools workflow: step=%v", step)

	// Get workflow-specific data storage
	workflow := context.GetCurrentWorkflow()

	switch step {
	case StepInit:
		logger.Debug("🧪 TestTools workflow: StepInit - Testing echo tool")

		// Call the echo tool with a test message
		params := map[string]any{
			"message": "Hello from the tool system!",
		}

		actions := callTool(tools.ToolTestEcho, params)
		logger.Debug("🧪 TestTools workflow: Returning echo tool action")
		return actions, nil

	case "after_echo":
		logger.Debug("🧪 TestTools workflow: after_echo - Processing echo result")

		// Get the tool result
		toolResultRaw := getInput(sourceAction, core.InputToolResult)
		if toolResultRaw == nil {
			logger.Error("❌ TestTools workflow: No tool result received from echo")
			return nil, fmt.Errorf("expected tool result but got none")
		}

		toolResult, ok := toolResultRaw.(*tools.ToolResult)
		if !ok {
			logger.Errorf("❌ TestTools workflow: Invalid tool result type: %T", toolResultRaw)
			return nil, fmt.Errorf("invalid tool result type")
		}

		// Check if echo was successful
		if !toolResult.Success {
			logger.Errorf("❌ TestTools workflow: Echo tool failed: %s", toolResult.Error)
			return nil, fmt.Errorf("echo tool failed: %s", toolResult.Error)
		}

		// Extract echoed message
		echoedMessage, ok := toolResult.Data["echoed_message"].(string)
		if !ok {
			logger.Error("❌ TestTools workflow: Failed to extract echoed_message from result")
			return nil, fmt.Errorf("failed to extract echoed_message")
		}

		logger.Infof("✅ TestTools workflow: Echo result: %s", echoedMessage)

		// Store the echo result in workflow data
		workflow.SetWorkflowData("echo_result", echoedMessage)

		// Now call the calculator tool
		params := map[string]any{
			"operation": "multiply",
			"operand1":  12.5,
			"operand2":  4.0,
		}

		actions := callTool(tools.ToolTestCalculator, params)
		logger.Debug("🧪 TestTools workflow: Returning calculator tool action")
		return actions, nil

	case "after_calculator":
		logger.Debug("🧪 TestTools workflow: after_calculator - Processing calculator result")

		// Get the tool result
		toolResultRaw := getInput(sourceAction, core.InputToolResult)
		if toolResultRaw == nil {
			logger.Error("❌ TestTools workflow: No tool result received from calculator")
			return nil, fmt.Errorf("expected tool result but got none")
		}

		toolResult, ok := toolResultRaw.(*tools.ToolResult)
		if !ok {
			logger.Errorf("❌ TestTools workflow: Invalid tool result type: %T", toolResultRaw)
			return nil, fmt.Errorf("invalid tool result type")
		}

		// Check if calculator was successful
		if !toolResult.Success {
			logger.Errorf("❌ TestTools workflow: Calculator tool failed: %s", toolResult.Error)
			return nil, fmt.Errorf("calculator tool failed: %s", toolResult.Error)
		}

		// Extract calculation result and expression
		result, ok := toolResult.Data["result"].(float64)
		if !ok {
			logger.Error("❌ TestTools workflow: Failed to extract result from calculator")
			return nil, fmt.Errorf("failed to extract result")
		}

		expression, ok := toolResult.Data["expression"].(string)
		if !ok {
			logger.Error("❌ TestTools workflow: Failed to extract expression from calculator")
			return nil, fmt.Errorf("failed to extract expression")
		}

		logger.Infof("✅ TestTools workflow: Calculator result: %s = %f", expression, result)

		// Get the stored echo result
		echoResult := workflow.GetWorkflowData("echo_result").(string)

		// Send a final message to the user with both results
		finalMessage := fmt.Sprintf("Tool test results:\n\n1. Echo tool: %s\n2. Calculator tool: %s",
			echoResult, expression)

		userMessageAction := core.NewAction(core.ActionUserMessage)
		if userMessageAction.Input == nil {
			userMessageAction.Input = make(map[core.InputType]any)
		}
		userMessageAction.Input[core.InputMessage] = finalMessage

		logger.Debug("🧪 TestTools workflow: Returning final user message")
		return []*core.Action{userMessageAction}, nil

	default:
		logger.Warnf("⚠️  TestTools workflow: Unknown step: %v", step)
		return nil, fmt.Errorf("unknown workflow step: %v", step)
	}
}
