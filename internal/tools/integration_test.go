package tools

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"testing"
)

// TestADOToolRegistration verifies ADO tools are registered
func TestADOToolRegistration(t *testing.T) {
	// Test that create ticket tool is registered
	createTool, exists := GetTool(ToolADOCreateTicket)
	if !exists {
		t.Errorf("ADO create ticket tool not registered")
	}
	if createTool.Category != CategoryADO {
		t.Errorf("Create ticket tool has wrong category: got %s, want %s", createTool.Category, CategoryADO)
	}

	// Test that query ticket tool is registered
	queryTool, exists := GetTool(ToolADOQueryTicket)
	if !exists {
		t.Errorf("ADO query ticket tool not registered")
	}
	if queryTool.Category != CategoryADO {
		t.Errorf("Query ticket tool has wrong category: got %s, want %s", queryTool.Category, CategoryADO)
	}
}

// TestADOCreateTicketTool verifies the ADO create ticket tool stub works
func TestADOCreateTicketTool(t *testing.T) {
	ctx := &core.ConversationContext{}

	params := ai.NewSchemaData(map[string]any{
		"project":        "TestProject",
		"title":          "Test Work Item",
		"work_item_type": "Task",
		"description":    "This is a test description",
		"priority":       "3",
	})

	result, err := ExecuteTool(ToolADOCreateTicket, ctx, params)
	if err != nil {
		t.Errorf("ADO create ticket tool execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("ADO create ticket tool returned nil result")
	}

	if !result.Success {
		t.Errorf("ADO create ticket tool failed: %s", result.Error)
	}

	// Verify result contains expected fields
	if _, ok := result.Data["id"]; !ok {
		t.Error("Result missing 'id' field")
	}
	if _, ok := result.Data["url"]; !ok {
		t.Error("Result missing 'url' field")
	}
	if title, ok := result.Data["title"].(string); !ok || title != "Test Work Item" {
		t.Errorf("Result has incorrect title: got %v", result.Data["title"])
	}
}

// TestADOQueryTicketToolByID verifies the ADO query ticket tool works with work_item_id
func TestADOQueryTicketToolByID(t *testing.T) {
	ctx := &core.ConversationContext{}

	params := ai.NewSchemaData(map[string]any{
		"work_item_id": 12345,
	})

	result, err := ExecuteTool(ToolADOQueryTicket, ctx, params)
	if err != nil {
		t.Errorf("ADO query ticket tool execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("ADO query ticket tool returned nil result")
	}

	if !result.Success {
		t.Errorf("ADO query ticket tool failed: %s", result.Error)
	}

	// Verify result contains expected fields
	if id, ok := result.Data["id"].(int); !ok || id != 12345 {
		t.Errorf("Result has incorrect id: got %v", result.Data["id"])
	}
}

// TestADOQueryTicketToolBySearch verifies the ADO query ticket tool works with search_query
func TestADOQueryTicketToolBySearch(t *testing.T) {
	ctx := &core.ConversationContext{}

	params := ai.NewSchemaData(map[string]any{
		"search_query": "authentication issue",
	})

	result, err := ExecuteTool(ToolADOQueryTicket, ctx, params)
	if err != nil {
		t.Errorf("ADO query ticket tool execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("ADO query ticket tool returned nil result")
	}

	if !result.Success {
		t.Errorf("ADO query ticket tool failed: %s", result.Error)
	}

	// Verify result contains expected fields
	if _, ok := result.Data["id"]; !ok {
		t.Error("Result missing 'id' field")
	}
	if _, ok := result.Data["title"]; !ok {
		t.Error("Result missing 'title' field")
	}
}

// TestADOQueryTicketToolNoParams verifies error handling when no params provided
func TestADOQueryTicketToolNoParams(t *testing.T) {
	ctx := &core.ConversationContext{}

	params := ai.NewSchemaData(map[string]any{})

	result, err := ExecuteTool(ToolADOQueryTicket, ctx, params)
	if err != nil {
		// It's okay if it returns an error
		return
	}

	// If no error, check that result indicates failure
	if result == nil {
		t.Fatal("Tool returned nil result and nil error")
	}
	if result.Success {
		t.Error("Tool should have failed due to missing parameters")
	}
}

// TestAllToolsHaveValidSchemas verifies all tools have valid input schemas
func TestAllToolsHaveValidSchemas(t *testing.T) {
	allTools := GetAllTools()

	for name, tool := range allTools {
		if tool.InputSchema == nil {
			t.Errorf("Tool %s has nil input schema", name)
			continue
		}

		// Verify schema has at least one field or is intentionally empty
		fields := tool.InputSchema.Fields()
		if len(fields) == 0 && name != ToolTestEcho {
			t.Logf("Warning: Tool %s has no input fields", name)
		}
	}
}

// TestToolCategorization verifies tools are properly categorized
func TestToolCategorization(t *testing.T) {
	testingTools := GetToolsByCategory(CategoryTesting)
	adoTools := GetToolsByCategory(CategoryADO)

	if len(testingTools) != 2 {
		t.Errorf("Expected 2 testing tools, got %d", len(testingTools))
	}

	if len(adoTools) != 2 {
		t.Errorf("Expected 2 ADO tools, got %d", len(adoTools))
	}

	// Verify testing tools
	testingToolNames := make(map[ToolName]bool)
	for _, tool := range testingTools {
		testingToolNames[tool.Name] = true
	}
	if !testingToolNames[ToolTestEcho] {
		t.Error("Testing category missing echo tool")
	}
	if !testingToolNames[ToolTestCalculator] {
		t.Error("Testing category missing calculator tool")
	}

	// Verify ADO tools
	adoToolNames := make(map[ToolName]bool)
	for _, tool := range adoTools {
		adoToolNames[tool.Name] = true
	}
	if !adoToolNames[ToolADOCreateTicket] {
		t.Error("ADO category missing create ticket tool")
	}
	if !adoToolNames[ToolADOQueryTicket] {
		t.Error("ADO category missing query ticket tool")
	}
}

// TestToolOptionsPresence verifies expected tools have expected options
func TestToolOptionsPresence(t *testing.T) {
	// ADO tools should have requires_auth option
	createTool, _ := GetTool(ToolADOCreateTicket)
	if _, hasAuth := createTool.Options[OptionRequiresAuth]; !hasAuth {
		t.Error("ADO create ticket tool missing requires_auth option")
	}

	queryTool, _ := GetTool(ToolADOQueryTicket)
	if _, hasAuth := queryTool.Options[OptionRequiresAuth]; !hasAuth {
		t.Error("ADO query ticket tool missing requires_auth option")
	}

	// Testing tools should not have requires_auth
	echoTool, _ := GetTool(ToolTestEcho)
	if _, hasAuth := echoTool.Options[OptionRequiresAuth]; hasAuth {
		t.Error("Test echo tool should not have requires_auth option")
	}
}
