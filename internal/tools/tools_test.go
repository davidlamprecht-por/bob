package tools

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"testing"
)

// TestToolRegistration verifies that tools are properly registered
func TestToolRegistration(t *testing.T) {
	// Test that echo tool is registered
	echoTool, exists := GetTool(ToolTestEcho)
	if !exists {
		t.Errorf("Echo tool not registered")
	}
	if echoTool.Name != ToolTestEcho {
		t.Errorf("Echo tool has wrong name: got %s, want %s", echoTool.Name, ToolTestEcho)
	}
	if echoTool.Category != CategoryTesting {
		t.Errorf("Echo tool has wrong category: got %s, want %s", echoTool.Category, CategoryTesting)
	}

	// Test that calculator tool is registered
	calcTool, exists := GetTool(ToolTestCalculator)
	if !exists {
		t.Errorf("Calculator tool not registered")
	}
	if calcTool.Name != ToolTestCalculator {
		t.Errorf("Calculator tool has wrong name: got %s, want %s", calcTool.Name, ToolTestCalculator)
	}
}

// TestEchoTool verifies the echo tool works correctly
func TestEchoTool(t *testing.T) {
	// Create test context
	ctx := &core.ConversationContext{}

	// Create test parameters
	params := ai.NewSchemaData(map[string]any{
		"message": "Hello, Tool System!",
	})

	// Execute the echo tool
	result, err := ExecuteTool(ToolTestEcho, ctx, params)
	if err != nil {
		t.Errorf("Echo tool execution failed: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("Echo tool returned nil result")
	}
	if !result.Success {
		t.Errorf("Echo tool failed: %s", result.Error)
	}

	// Check echoed message
	echoedMessage, ok := result.Data["echoed_message"].(string)
	if !ok {
		t.Error("Echo tool did not return echoed_message")
	}
	if echoedMessage != "Hello, Tool System!" {
		t.Errorf("Echo tool returned wrong message: got %s, want %s", echoedMessage, "Hello, Tool System!")
	}
}

// TestCalculatorTool verifies the calculator tool works correctly
func TestCalculatorTool(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		operand1  float64
		operand2  float64
		expected  float64
		shouldFail bool
	}{
		{"Addition", "add", 10.0, 5.0, 15.0, false},
		{"Subtraction", "subtract", 10.0, 5.0, 5.0, false},
		{"Multiplication", "multiply", 10.0, 5.0, 50.0, false},
		{"Division", "divide", 10.0, 5.0, 2.0, false},
		{"Division by zero", "divide", 10.0, 0.0, 0.0, true},
	}

	ctx := &core.ConversationContext{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := ai.NewSchemaData(map[string]any{
				"operation": tt.operation,
				"operand1":  tt.operand1,
				"operand2":  tt.operand2,
			})

			result, err := ExecuteTool(ToolTestCalculator, ctx, params)
			if err != nil {
				t.Errorf("Calculator tool execution failed: %v", err)
			}

			if result == nil {
				t.Fatal("Calculator tool returned nil result")
			}

			if tt.shouldFail {
				if result.Success {
					t.Error("Calculator tool should have failed but succeeded")
				}
			} else {
				if !result.Success {
					t.Errorf("Calculator tool failed: %s", result.Error)
				}

				// Check result
				resultValue, ok := result.Data["result"].(float64)
				if !ok {
					t.Error("Calculator tool did not return result")
				}
				if resultValue != tt.expected {
					t.Errorf("Calculator tool returned wrong result: got %f, want %f", resultValue, tt.expected)
				}

				// Check expression exists
				_, ok = result.Data["expression"].(string)
				if !ok {
					t.Error("Calculator tool did not return expression")
				}
			}
		})
	}
}

// TestToolNotFound verifies error handling for non-existent tools
func TestToolNotFound(t *testing.T) {
	ctx := &core.ConversationContext{}
	params := ai.NewSchemaData(map[string]any{})

	result, err := ExecuteTool("nonexistent_tool", ctx, params)
	if err == nil {
		t.Error("Expected error for non-existent tool, got nil")
	}
	if result != nil {
		t.Error("Expected nil result for non-existent tool")
	}
}

// TestToolWithMissingParams verifies error handling for missing parameters
func TestToolWithMissingParams(t *testing.T) {
	ctx := &core.ConversationContext{}

	// Call echo tool without message parameter
	params := ai.NewSchemaData(map[string]any{})

	result, err := ExecuteTool(ToolTestEcho, ctx, params)
	if err != nil {
		// It's okay if it returns an error
		return
	}

	// If no error, check that result indicates failure
	if result == nil {
		t.Fatal("Tool returned nil result and nil error")
	}
	if result.Success {
		t.Error("Tool should have failed due to missing parameter")
	}
}

// TestGetAllTools verifies GetAllTools returns all registered tools
func TestGetAllTools(t *testing.T) {
	allTools := GetAllTools()

	if len(allTools) == 0 {
		t.Error("GetAllTools returned no tools")
	}

	// Verify at least the testing tools are present
	if _, exists := allTools[ToolTestEcho]; !exists {
		t.Error("GetAllTools missing echo tool")
	}
	if _, exists := allTools[ToolTestCalculator]; !exists {
		t.Error("GetAllTools missing calculator tool")
	}
}

// TestToolExists verifies ToolExists function
func TestToolExists(t *testing.T) {
	if !ToolExists(ToolTestEcho) {
		t.Error("ToolExists returned false for registered echo tool")
	}
	if !ToolExists(ToolTestCalculator) {
		t.Error("ToolExists returned false for registered calculator tool")
	}
	if ToolExists("nonexistent_tool") {
		t.Error("ToolExists returned true for non-existent tool")
	}
}

// TestAvailableTools verifies AvailableTools returns correct information
func TestAvailableTools(t *testing.T) {
	tools := AvailableTools()

	if len(tools) == 0 {
		t.Error("AvailableTools returned no tools")
	}

	// Find echo tool
	var echoTool *ToolInfo
	for i := range tools {
		if tools[i].Name == ToolTestEcho {
			echoTool = &tools[i]
			break
		}
	}

	if echoTool == nil {
		t.Fatal("Echo tool not found in AvailableTools")
	}

	// Verify echo tool properties
	if echoTool.Category != CategoryTesting {
		t.Errorf("Echo tool has wrong category: got %s, want %s", echoTool.Category, CategoryTesting)
	}
	if len(echoTool.Parameters) == 0 {
		t.Error("Echo tool has no parameters")
	}

	// Verify echo tool has message parameter
	hasMessageParam := false
	for _, param := range echoTool.Parameters {
		if param.Name == "message" {
			hasMessageParam = true
			if param.Type != "string" {
				t.Errorf("Message parameter has wrong type: got %s, want string", param.Type)
			}
			if !param.Required {
				t.Error("Message parameter should be required")
			}
		}
	}
	if !hasMessageParam {
		t.Error("Echo tool missing message parameter")
	}
}

// TestGetAvailableToolsContext verifies context generation
func TestGetAvailableToolsContext(t *testing.T) {
	context := GetAvailableToolsContext()

	if context == "" {
		t.Error("GetAvailableToolsContext returned empty string")
	}

	// Verify context contains expected strings
	expectedStrings := []string{
		"## Available Tools",
		"Testing Tools",
		"test_echo",
		"test_calculator",
		"message",
		"operation",
		"operand1",
		"operand2",
	}

	for _, expected := range expectedStrings {
		if !contains(context, expected) {
			t.Errorf("Context missing expected string: %s", expected)
		}
	}

	// Print context for visual verification
	t.Logf("Generated context:\n%s", context)
}

// TestGetToolInfo verifies GetToolInfo function
func TestGetToolInfo(t *testing.T) {
	info, exists := GetToolInfo(ToolTestEcho)
	if !exists {
		t.Fatal("GetToolInfo returned false for echo tool")
	}
	if info == nil {
		t.Fatal("GetToolInfo returned nil for echo tool")
	}
	if info.Name != ToolTestEcho {
		t.Errorf("GetToolInfo returned wrong tool name: got %s, want %s", info.Name, ToolTestEcho)
	}

	// Test non-existent tool
	info, exists = GetToolInfo("nonexistent")
	if exists {
		t.Error("GetToolInfo returned true for non-existent tool")
	}
	if info != nil {
		t.Error("GetToolInfo returned non-nil for non-existent tool")
	}
}

// TestGetToolsByCategory verifies GetToolsByCategory function
func TestGetToolsByCategory(t *testing.T) {
	testingTools := GetToolsByCategory(CategoryTesting)

	if len(testingTools) == 0 {
		t.Error("GetToolsByCategory returned no testing tools")
	}

	// Verify all returned tools are in the testing category
	for _, tool := range testingTools {
		if tool.Category != CategoryTesting {
			t.Errorf("Tool %s has wrong category: got %s, want %s", tool.Name, tool.Category, CategoryTesting)
		}
	}

	// Verify at least echo and calculator are present
	foundEcho := false
	foundCalc := false
	for _, tool := range testingTools {
		if tool.Name == ToolTestEcho {
			foundEcho = true
		}
		if tool.Name == ToolTestCalculator {
			foundCalc = true
		}
	}
	if !foundEcho {
		t.Error("Testing category missing echo tool")
	}
	if !foundCalc {
		t.Error("Testing category missing calculator tool")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
