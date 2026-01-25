package tools

import (
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"fmt"
)

// registerTestingTools registers all testing-related tools in the global registry
func registerTestingTools() {
	logger.Debug("📋 Registering testing tools")

	registerTool(ToolDefinition{
		Name:         ToolTestEcho,
		Description:  "Simple echo tool that returns the provided message back. Used for validating basic tool execution flow.",
		Category:     CategoryTesting,
		InputSchema:  createTestEchoInputSchema(),
		OutputSchema: createTestEchoOutputSchema(),
		ToolFn:       executeTestEcho,
		Options:      map[ToolOption]any{},
	})

	registerTool(ToolDefinition{
		Name:         ToolTestCalculator,
		Description:  "Calculator tool that performs basic arithmetic operations (add, subtract, multiply, divide). Used for validating schema-based parameter extraction and structured outputs.",
		Category:     CategoryTesting,
		InputSchema:  createTestCalculatorInputSchema(),
		OutputSchema: createTestCalculatorOutputSchema(),
		ToolFn:       executeTestCalculator,
		Options:      map[ToolOption]any{},
	})

	logger.Debug("✅ Testing tools registered")
}

// Test Echo Tool Schemas

func createTestEchoInputSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("message", ai.Required(),
			ai.Description("The message to echo back"))
}

func createTestEchoOutputSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("echoed_message",
			ai.Description("The echoed message"))
}

// executeTestEcho implements the echo tool functionality
func executeTestEcho(ctx *core.ConversationContext, params *ai.SchemaData) (*ToolResult, error) {
	logger.Debug("🔊 Executing test_echo tool")

	// Extract message parameter
	message, err := params.GetString("message")
	if err != nil {
		logger.Errorf("❌ test_echo: Failed to get message parameter: %v", err)
		return NewToolError(err.Error(), "Missing required parameter: message"), nil
	}

	logger.Debugf("🔊 test_echo: Echoing message: %q", message)

	// Return the echoed message
	result := map[string]any{
		"echoed_message": message,
	}

	return NewToolResult(result, fmt.Sprintf("Echoed: %s", message)), nil
}

// Test Calculator Tool Schemas

func createTestCalculatorInputSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("operation", ai.Required(),
			ai.Enum("add", "subtract", "multiply", "divide"),
			ai.Description("The arithmetic operation to perform")).
		AddFloat("operand1", ai.Required(),
			ai.Description("The first number")).
		AddFloat("operand2", ai.Required(),
			ai.Description("The second number"))
}

func createTestCalculatorOutputSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddFloat("result",
			ai.Description("The result of the calculation")).
		AddString("expression",
			ai.Description("The mathematical expression that was evaluated"))
}

// executeTestCalculator implements the calculator tool functionality
func executeTestCalculator(ctx *core.ConversationContext, params *ai.SchemaData) (*ToolResult, error) {
	logger.Debug("🔢 Executing test_calculator tool")

	// Extract parameters
	operation, err := params.GetString("operation")
	if err != nil {
		logger.Errorf("❌ test_calculator: Failed to get operation parameter: %v", err)
		return NewToolError(err.Error(), "Missing required parameter: operation"), nil
	}

	operand1, err := params.GetFloat("operand1")
	if err != nil {
		logger.Errorf("❌ test_calculator: Failed to get operand1 parameter: %v", err)
		return NewToolError(err.Error(), "Missing required parameter: operand1"), nil
	}

	operand2, err := params.GetFloat("operand2")
	if err != nil {
		logger.Errorf("❌ test_calculator: Failed to get operand2 parameter: %v", err)
		return NewToolError(err.Error(), "Missing required parameter: operand2"), nil
	}

	logger.Debugf("🔢 test_calculator: operation=%s, operand1=%f, operand2=%f", operation, operand1, operand2)

	// Perform the calculation
	var result float64
	var expression string

	switch operation {
	case "add":
		result = operand1 + operand2
		expression = fmt.Sprintf("%.2f + %.2f = %.2f", operand1, operand2, result)
	case "subtract":
		result = operand1 - operand2
		expression = fmt.Sprintf("%.2f - %.2f = %.2f", operand1, operand2, result)
	case "multiply":
		result = operand1 * operand2
		expression = fmt.Sprintf("%.2f * %.2f = %.2f", operand1, operand2, result)
	case "divide":
		if operand2 == 0 {
			logger.Error("❌ test_calculator: Division by zero")
			return NewToolError("division by zero", "Cannot divide by zero"), nil
		}
		result = operand1 / operand2
		expression = fmt.Sprintf("%.2f / %.2f = %.2f", operand1, operand2, result)
	default:
		logger.Errorf("❌ test_calculator: Invalid operation: %s", operation)
		return NewToolError(fmt.Sprintf("invalid operation: %s", operation), "Invalid operation"), nil
	}

	logger.Infof("✅ test_calculator: %s", expression)

	// Return the result
	resultData := map[string]any{
		"result":     result,
		"expression": expression,
	}

	return NewToolResult(resultData, expression), nil
}
