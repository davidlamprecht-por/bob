package tools

import (
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"fmt"
)

// registerADOTools registers all Azure DevOps tools in the global registry
func registerADOTools() {
	logger.Debug("📋 Registering Azure DevOps tools")

	registerTool(ToolDefinition{
		Name:         ToolADOCreateTicket,
		Description:  "Creates a new work item (ticket) in Azure DevOps. Supports creating Tasks, Bugs, User Stories, and Features with title, description, assignment, tags, and priority.",
		Category:     CategoryADO,
		InputSchema:  createADOCreateTicketInputSchema(),
		OutputSchema: createADOCreateTicketOutputSchema(),
		ToolFn:       executeADOCreateTicket,
		Options: map[ToolOption]any{
			OptionRequiresAuth: true,
		},
	})

	registerTool(ToolDefinition{
		Name:         ToolADOQueryTicket,
		Description:  "Queries and retrieves work item information from Azure DevOps by work item ID or search query. Returns details including title, state, type, and URL.",
		Category:     CategoryADO,
		InputSchema:  createADOQueryTicketInputSchema(),
		OutputSchema: createADOQueryTicketOutputSchema(),
		ToolFn:       executeADOQueryTicket,
		Options: map[ToolOption]any{
			OptionRequiresAuth: true,
		},
	})

	logger.Debug("✅ Azure DevOps tools registered")
}

// ADO Create Ticket Tool Schemas

func createADOCreateTicketInputSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("project", ai.Required(),
			ai.Description("The Azure DevOps project name")).
		AddString("title", ai.Required(), ai.MinLength(5),
			ai.Description("The work item title (minimum 5 characters)")).
		AddString("work_item_type", ai.Required(),
			ai.Enum("Task", "Bug", "User Story", "Feature"),
			ai.Description("The type of work item to create")).
		AddString("description", ai.Required(),
			ai.Description("Detailed description of the work item")).
		AddString("assigned_to",
			ai.Description("Email or name of the person to assign the work item to (optional)")).
		AddArray("tags", ai.FieldTypeString,
			ai.Description("Array of tags to apply to the work item (optional)")).
		AddString("priority",
			ai.Enum("1", "2", "3", "4"),
			ai.Description("Priority level (1=Critical, 2=High, 3=Medium, 4=Low) (optional)"))
}

func createADOCreateTicketOutputSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddInt("id", ai.Required(),
			ai.Description("The created work item ID")).
		AddString("title", ai.Required(),
			ai.Description("The work item title")).
		AddString("state",
			ai.Description("The initial state of the work item")).
		AddString("work_item_type",
			ai.Description("The type of work item created")).
		AddString("url",
			ai.Description("Direct URL to view the work item in Azure DevOps"))
}

// executeADOCreateTicket implements the ADO create ticket functionality
// Currently returns stub/mock data - will be replaced with actual ADO API integration
func executeADOCreateTicket(ctx *core.ConversationContext, params *ai.SchemaData) (*ToolResult, error) {
	logger.Debug("🎫 Executing ado_create_ticket tool (stub implementation)")

	// Extract required parameters
	project, err := params.GetString("project")
	if err != nil {
		logger.Errorf("❌ ado_create_ticket: Failed to get project parameter: %v", err)
		return NewToolError(err.Error(), "Missing required parameter: project"), nil
	}

	title, err := params.GetString("title")
	if err != nil {
		logger.Errorf("❌ ado_create_ticket: Failed to get title parameter: %v", err)
		return NewToolError(err.Error(), "Missing required parameter: title"), nil
	}

	workItemType, err := params.GetString("work_item_type")
	if err != nil {
		logger.Errorf("❌ ado_create_ticket: Failed to get work_item_type parameter: %v", err)
		return NewToolError(err.Error(), "Missing required parameter: work_item_type"), nil
	}

	description, err := params.GetString("description")
	if err != nil {
		logger.Errorf("❌ ado_create_ticket: Failed to get description parameter: %v", err)
		return NewToolError(err.Error(), "Missing required parameter: description"), nil
	}

	// Extract optional parameters
	assignedTo := ""
	if params.Has("assigned_to") {
		assignedTo, _ = params.GetString("assigned_to")
	}

	var tags []string
	if params.Has("tags") {
		tagsRaw, _ := params.GetArray("tags")
		for _, tag := range tagsRaw {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	priority := ""
	if params.Has("priority") {
		priority, _ = params.GetString("priority")
	}

	logger.Debugf("🎫 ado_create_ticket: project=%s, title=%s, type=%s, desc=%q, assigned=%s, tags=%v, priority=%s",
		project, title, workItemType, description, assignedTo, tags, priority)

	// TODO: Replace this stub with actual Azure DevOps API call
	// For now, return mock data
	logger.Info("⚠️  Using stub implementation - returning mock work item ID")

	mockWorkItemID := 12345
	mockURL := fmt.Sprintf("https://dev.azure.com/%s/_workitems/edit/%d", project, mockWorkItemID)

	result := map[string]any{
		"id":             mockWorkItemID,
		"title":          title,
		"state":          "New",
		"work_item_type": workItemType,
		"url":            mockURL,
	}

	message := fmt.Sprintf("Created %s #%d: %s", workItemType, mockWorkItemID, title)
	logger.Infof("✅ ado_create_ticket (stub): %s", message)

	return NewToolResult(result, message), nil
}

// ADO Query Ticket Tool Schemas

func createADOQueryTicketInputSchema() *ai.SchemaBuilder {
	// Note: At least one of work_item_id or search_query must be provided
	return ai.NewSchema().
		AddInt("work_item_id",
			ai.Description("The specific work item ID to retrieve")).
		AddString("search_query",
			ai.Description("Search query to find work items (searches title and description)"))
}

func createADOQueryTicketOutputSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddInt("id", ai.Required(),
			ai.Description("The work item ID")).
		AddString("title", ai.Required(),
			ai.Description("The work item title")).
		AddString("state",
			ai.Description("Current state of the work item")).
		AddString("work_item_type",
			ai.Description("The type of work item")).
		AddString("assigned_to",
			ai.Description("Person assigned to the work item")).
		AddString("url",
			ai.Description("Direct URL to view the work item in Azure DevOps"))
}

// executeADOQueryTicket implements the ADO query ticket functionality
// Currently returns stub/mock data - will be replaced with actual ADO API integration
func executeADOQueryTicket(ctx *core.ConversationContext, params *ai.SchemaData) (*ToolResult, error) {
	logger.Debug("🔍 Executing ado_query_ticket tool (stub implementation)")

	// At least one parameter must be provided
	hasWorkItemID := params.Has("work_item_id")
	hasSearchQuery := params.Has("search_query")

	if !hasWorkItemID && !hasSearchQuery {
		logger.Error("❌ ado_query_ticket: Neither work_item_id nor search_query provided")
		return NewToolError(
			"at least one of work_item_id or search_query must be provided",
			"Missing required parameters"), nil
	}

	var workItemID int
	var searchQuery string

	if hasWorkItemID {
		var err error
		workItemID, err = params.GetInt("work_item_id")
		if err != nil {
			logger.Errorf("❌ ado_query_ticket: Failed to get work_item_id: %v", err)
			return NewToolError(err.Error(), "Invalid work_item_id parameter"), nil
		}
		logger.Debugf("🔍 ado_query_ticket: Querying by work_item_id=%d", workItemID)
	}

	if hasSearchQuery {
		var err error
		searchQuery, err = params.GetString("search_query")
		if err != nil {
			logger.Errorf("❌ ado_query_ticket: Failed to get search_query: %v", err)
			return NewToolError(err.Error(), "Invalid search_query parameter"), nil
		}
		logger.Debugf("🔍 ado_query_ticket: Querying by search_query=%s", searchQuery)
	}

	// TODO: Replace this stub with actual Azure DevOps API call
	// For now, return mock data
	logger.Info("⚠️  Using stub implementation - returning mock work item data")

	mockWorkItemID := workItemID
	if mockWorkItemID == 0 {
		mockWorkItemID = 99999 // Default for search queries
	}

	mockTitle := "Sample Work Item"
	if searchQuery != "" {
		mockTitle = fmt.Sprintf("Work item matching '%s'", searchQuery)
	}

	result := map[string]any{
		"id":             mockWorkItemID,
		"title":          mockTitle,
		"state":          "Active",
		"work_item_type": "Task",
		"assigned_to":    "user@example.com",
		"url":            fmt.Sprintf("https://dev.azure.com/project/_workitems/edit/%d", mockWorkItemID),
	}

	message := fmt.Sprintf("Found work item #%d: %s", mockWorkItemID, mockTitle)
	logger.Infof("✅ ado_query_ticket (stub): %s", message)

	return NewToolResult(result, message), nil
}
