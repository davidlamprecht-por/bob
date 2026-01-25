package tools

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"fmt"
)

// tools is the global registry of all available tools
var tools map[ToolName]ToolDefinition

// init initializes and validates the tool registry
func init() {
	// Initialize the registry
	tools = make(map[ToolName]ToolDefinition)

	// Register tools (will be populated as we implement them)
	registerTestingTools()
	registerADOTools()

	// Validate all registered tools
	validateRegistry()
}

// registerTestingTools is implemented in testing_tools.go
// registerADOTools is implemented in ado_tools.go
// These are forward declarations - actual implementations are in separate files

// registerTool adds a tool to the global registry
func registerTool(def ToolDefinition) {
	if _, exists := tools[def.Name]; exists {
		panic(fmt.Sprintf("tool already registered: %s", def.Name))
	}
	tools[def.Name] = def
}

// validateRegistry ensures all registered tools are properly configured
func validateRegistry() {
	for name, def := range tools {
		if def.Name == "" {
			panic(fmt.Sprintf("tool %s has empty name", name))
		}
		if def.Description == "" {
			panic(fmt.Sprintf("tool %s has empty description", name))
		}
		if def.Category == "" {
			panic(fmt.Sprintf("tool %s has empty category", name))
		}
		if def.InputSchema == nil {
			panic(fmt.Sprintf("tool %s has nil input schema", name))
		}
		if def.ToolFn == nil {
			panic(fmt.Sprintf("tool %s has nil tool function", name))
		}
	}
}

// GetTool retrieves a tool definition from the registry
// Returns the tool definition and a boolean indicating if it was found
func GetTool(name ToolName) (ToolDefinition, bool) {
	tool, exists := tools[name]
	return tool, exists
}

// GetAllTools returns all registered tools
func GetAllTools() map[ToolName]ToolDefinition {
	// Return a copy to prevent modification
	toolsCopy := make(map[ToolName]ToolDefinition, len(tools))
	for k, v := range tools {
		toolsCopy[k] = v
	}
	return toolsCopy
}

// ExecuteTool executes a tool by name with the provided context and parameters
// Returns the tool result and any execution errors
func ExecuteTool(name ToolName, ctx *core.ConversationContext, params *ai.SchemaData) (*ToolResult, error) {
	// Look up the tool
	tool, exists := GetTool(name)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	// Validate parameters against input schema
	if params == nil {
		return nil, fmt.Errorf("tool %s requires parameters but none provided", name)
	}

	// TODO: Add schema validation here
	// For now, we trust the input matches the schema

	// Execute the tool
	result, err := tool.ToolFn(ctx, params)
	if err != nil {
		// If tool function returns an error, wrap it in a ToolResult
		// This allows workflows to handle the error gracefully
		if result == nil {
			result = NewToolError(err.Error(), fmt.Sprintf("Tool %s failed", name))
		}
	}

	return result, err
}

// ToolExists checks if a tool with the given name is registered
func ToolExists(name ToolName) bool {
	_, exists := tools[name]
	return exists
}
