/* Package tool is the place in which the bot can access external data */
package tool

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"fmt"
)

const (
	// ToolADOQueryTicket ToolName = "adoQueryTicket"
	// ToolADOCreateTicket ToolName = "adoCreateTicket"
	ToolSampleData ToolName = "sampleData"
)

var tools = map[ToolName]ToolDefinition{
	ToolSampleData: {
		Description:  "This is for testing and will return a random phrase",
		ToolFn:       SampleData,
		ArgsRequired: SampleDataArgs,
	},
}

// -----------------------------------------

type ToolName string

type ToolDefinition struct {
	Description string

	ToolFn       func(context *core.ConversationContext, args map[string]any) (map[string]any, error)
	ArgsRequired *ai.SchemaBuilder // Input Schema
}

// RunTool will run any tool
func RunTool(context *core.ConversationContext, toolName ToolName, toolArgs map[string]any) (map[string]any, error) {
	tool, ok := tools[toolName]
	if !ok {
		return nil, fmt.Errorf("Unknown Tool: %q", toolName)
	}

	return tool.ToolFn(context, toolArgs)
}

func init() {
	for name, def := range tools {
		if def.ToolFn == nil {
			panic(fmt.Sprintf("tool %q has nil ToolFn", name))
		}
		if def.Description == "" {
			panic(fmt.Sprintf("tool %q has empty Description", name))
		}
	}
}

// To pass to ai...

type ToolInfo struct {
	Name        ToolName `json:"name"`
	Description string   `json:"description"`
	// TODO: Should send the required args?
}

func AvailableTools() []ToolInfo {
	out := make([]ToolInfo, 0, len(tools))
	for name, def := range tools {
		out = append(out, ToolInfo{
			Name:        name,
			Description: def.Description,
			// TODO: Somehow give out required tags?
		})
	}
	return out
}

// TODO: Likely need a GetAvailableToolContext() string  similar to GetAvailableWorkflowContext()
