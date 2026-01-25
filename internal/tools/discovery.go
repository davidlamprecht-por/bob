package tools

import (
	"bob/internal/ai"
	"fmt"
	"sort"
	"strings"
)

// ParameterInfo describes a single parameter for a tool
type ParameterInfo struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Enum        []string // For enum types
}

// ToolInfo provides structured information about a tool for AI and programmatic access
type ToolInfo struct {
	Name        ToolName
	Description string
	Category    ToolCategory
	Parameters  []ParameterInfo
	Options     map[ToolOption]any
}

// AvailableTools returns structured information about all registered tools
// This can be used programmatically to discover and inspect tools
func AvailableTools() []ToolInfo {
	allTools := GetAllTools()
	result := make([]ToolInfo, 0, len(allTools))

	for _, tool := range allTools {
		info := ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			Category:    tool.Category,
			Parameters:  extractParameters(tool.InputSchema),
			Options:     tool.Options,
		}
		result = append(result, info)
	}

	// Sort by category then name for consistent output
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		return result[i].Name < result[j].Name
	})

	return result
}

// GetAvailableToolsContext generates a formatted markdown context string
// describing all available tools for inclusion in AI prompts
func GetAvailableToolsContext() string {
	tools := AvailableTools()

	if len(tools) == 0 {
		return "## Available Tools\n\nNo tools currently available.\n"
	}

	var sb strings.Builder
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("The following tools can be called using the `callTool()` function in workflows.\n\n")

	// Group tools by category
	toolsByCategory := make(map[ToolCategory][]ToolInfo)
	for _, tool := range tools {
		toolsByCategory[tool.Category] = append(toolsByCategory[tool.Category], tool)
	}

	// Get sorted category names
	categories := make([]ToolCategory, 0, len(toolsByCategory))
	for category := range toolsByCategory {
		categories = append(categories, category)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i] < categories[j]
	})

	// Generate output by category
	for _, category := range categories {
		categoryTools := toolsByCategory[category]

		sb.WriteString(fmt.Sprintf("### %s Tools\n\n", category))

		for i, tool := range categoryTools {
			sb.WriteString(fmt.Sprintf("**%d. Tool: %s**\n", i+1, tool.Name))
			sb.WriteString(fmt.Sprintf("   Description: %s\n", tool.Description))

			if len(tool.Parameters) > 0 {
				sb.WriteString("   Parameters:\n")
				for _, param := range tool.Parameters {
					requiredStr := ""
					if param.Required {
						requiredStr = " (required)"
					}

					enumStr := ""
					if len(param.Enum) > 0 {
						enumStr = fmt.Sprintf(" [enum: %s]", strings.Join(param.Enum, ", "))
					}

					sb.WriteString(fmt.Sprintf("   - %s (%s)%s%s: %s\n",
						param.Name,
						param.Type,
						requiredStr,
						enumStr,
						param.Description))
				}
			} else {
				sb.WriteString("   Parameters: None\n")
			}

			// Add options if present
			if len(tool.Options) > 0 {
				sb.WriteString("   Options: ")
				optionStrs := make([]string, 0, len(tool.Options))
				for opt := range tool.Options {
					optionStrs = append(optionStrs, string(opt))
				}
				sb.WriteString(strings.Join(optionStrs, ", "))
				sb.WriteString("\n")
			}

			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// extractParameters converts a SchemaBuilder's fields into ParameterInfo structs
func extractParameters(schema *ai.SchemaBuilder) []ParameterInfo {
	if schema == nil {
		return []ParameterInfo{}
	}

	fields := schema.Fields()
	params := make([]ParameterInfo, 0, len(fields))

	for _, field := range fields {
		param := ParameterInfo{
			Name:        field.Name,
			Type:        fieldTypeToString(field.Type),
			Description: field.Description,
			Required:    field.Required,
			Enum:        field.Enum,
		}
		params = append(params, param)
	}

	return params
}

// fieldTypeToString converts a FieldType enum to a human-readable string
func fieldTypeToString(ft ai.FieldType) string {
	switch ft {
	case ai.FieldTypeString:
		return "string"
	case ai.FieldTypeInt:
		return "int"
	case ai.FieldTypeFloat:
		return "float"
	case ai.FieldTypeBool:
		return "bool"
	case ai.FieldTypeArray:
		return "array"
	case ai.FieldTypeObject:
		return "object"
	default:
		return "unknown"
	}
}

// GetToolsByCategory returns all tools in a specific category
func GetToolsByCategory(category ToolCategory) []ToolInfo {
	allTools := AvailableTools()
	result := make([]ToolInfo, 0)

	for _, tool := range allTools {
		if tool.Category == category {
			result = append(result, tool)
		}
	}

	return result
}

// GetToolInfo returns detailed information about a specific tool
func GetToolInfo(name ToolName) (*ToolInfo, bool) {
	tool, exists := GetTool(name)
	if !exists {
		return nil, false
	}

	info := &ToolInfo{
		Name:        tool.Name,
		Description: tool.Description,
		Category:    tool.Category,
		Parameters:  extractParameters(tool.InputSchema),
		Options:     tool.Options,
	}

	return info, true
}
