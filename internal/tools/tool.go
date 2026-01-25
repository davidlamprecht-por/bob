package tools

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
)

// ToolName represents the unique identifier for a tool
type ToolName string

// Tool name constants
const (
	// Testing tools
	ToolTestEcho       ToolName = "test_echo"
	ToolTestCalculator ToolName = "test_calculator"

	// Azure DevOps tools
	ToolADOCreateTicket ToolName = "ado_create_ticket"
	ToolADOQueryTicket  ToolName = "ado_query_ticket"
)

// ToolCategory represents the category a tool belongs to
type ToolCategory string

// Tool category constants
const (
	CategoryTesting ToolCategory = "Testing"
	CategoryADO     ToolCategory = "Azure DevOps"
)

// ToolOption represents behavioral flags for tools
type ToolOption string

// Tool option constants
const (
	OptionAsync         ToolOption = "async"          // Tool runs asynchronously
	OptionRequiresAuth  ToolOption = "requires_auth"  // Tool requires authentication
	OptionCacheable     ToolOption = "cacheable"      // Tool results can be cached
	OptionTimeout       ToolOption = "timeout"        // Tool has custom timeout
)

// ToolFn is the function signature for tool execution
// Takes a conversation context and input parameters, returns a result and error
type ToolFn func(*core.ConversationContext, *ai.SchemaData) (*ToolResult, error)

// ToolDefinition defines a tool's metadata and execution function
type ToolDefinition struct {
	Name         ToolName                 // Unique identifier
	Description  string                   // Human-readable description
	Category     ToolCategory             // Tool category for organization
	InputSchema  *ai.SchemaBuilder        // Schema defining expected inputs
	OutputSchema *ai.SchemaBuilder        // Schema defining expected outputs (optional)
	ToolFn       ToolFn                   // Function to execute the tool
	Options      map[ToolOption]any       // Behavioral configuration options
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool           // Whether the tool executed successfully
	Data    map[string]any // Result data (for success cases)
	Message string         // Human-readable message
	Error   string         // Error message (for failure cases)
}

// NewToolResult creates a successful tool result
func NewToolResult(data map[string]any, message string) *ToolResult {
	return &ToolResult{
		Success: true,
		Data:    data,
		Message: message,
		Error:   "",
	}
}

// NewToolError creates a failed tool result
func NewToolError(errorMsg string, message string) *ToolResult {
	return &ToolResult{
		Success: false,
		Data:    nil,
		Message: message,
		Error:   errorMsg,
	}
}
