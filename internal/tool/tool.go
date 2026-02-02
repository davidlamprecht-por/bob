/* Package tool is the place in which the bot can access external data */
package tool

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"fmt"
)

var tools = map[ToolName]ToolDefinition{
	ToolADOCreateTicket: {
		Description:  "Create a new work item (User Story, Technical Debt, or Defect) in Azure DevOps. Required fields: work_item_type, title, description. Optional fields: area_path, iteration_path, tags, test_requirements, acceptance_criteria (stories), story_points (stories), severity (defects), expected_result (defects), actual_result (defects), case_number (defects), emergent_defect (defects).",
		ToolFn:       ADOCreateTicket,
		ArgsRequired: ADOCreateTicketArgs,
	},
	ToolADOSearchTickets: {
		Description:  "Search for work items in Azure DevOps using comprehensive filters. All parameters are optional - use any combination to find work items. Supports: id (exact), title (keywords), state, assigned_to, work_item_type, tags (any match), area_path, iteration_path, created_by, severity, story_points, qa_person, emergent_defect. Returns list of matching work items with details.",
		ToolFn:       ADOSearchTickets,
		ArgsRequired: ADOSearchTicketsArgs,
	},
	ToolADOGetMetadata: {
		Description:  "Retrieve metadata from Azure DevOps to help with filtering and queries. Returns available values for: tags (all distinct tags), area_paths (area hierarchy), iteration_paths (sprint/iteration list), states (valid states per work item type), work_item_types (available types), team_members (assignable users - display names only), severity_values (valid severity values for defects). Use this before searching to get exact filter values.",
		ToolFn:       ADOGetMetadata,
		ArgsRequired: ADOGetMetadataArgs,
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
