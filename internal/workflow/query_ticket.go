package workflow

import (
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/tools"
	"fmt"
)

const (
	StepParseQuery  = "parse_query"
	StepQueryADO    = "query_ado"
	StepShowResults = "show_results"
)

/*
QueryTicket workflow helps users find and view Azure DevOps work items.
Flow:
1. StepInit: Ask AI to parse user's query and extract search criteria
2. StepParseQuery: Process AI response and extract work_item_id or search_query
3. StepQueryADO: Call ADO query ticket tool
4. StepShowResults: Format and display results to user
*/

func QueryTicket(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🔍 QueryTicket workflow: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🔍 QueryTicket workflow: step=%v", step)

	workflow := context.GetCurrentWorkflow()

	switch step {
	case StepInit:
		logger.Debug("🔍 QueryTicket workflow: StepInit - Parsing user query")

		// Get user's query
		messages := context.GetLastUserMessages()
		if len(messages) == 0 {
			logger.Error("❌ QueryTicket workflow: No user messages found")
			return nil, fmt.Errorf("no user message found")
		}
		userMessage := messages[len(messages)-1].Message

		// Create schema for parsing query
		schema := ai.NewSchema().
			AddInt("work_item_id",
				ai.Description("Specific work item ID if user mentioned a number")).
			AddString("search_query",
				ai.Description("Search terms if user wants to search by keywords")).
			AddString("query_type", ai.Required(),
				ai.Enum("by_id", "by_search", "unclear"),
				ai.Description("Type of query: by_id if user specified an ID, by_search if searching by keywords, unclear if ambiguous"))

		systemPrompt := `You are helping the user query Azure DevOps work items.
Analyze their request and determine:
- work_item_id: If they mention a specific work item number/ID, extract it
- search_query: If they want to search by keywords, extract the search terms
- query_type: "by_id" if they specified an ID, "by_search" if searching, "unclear" if you can't determine

Examples:
- "Show me work item 12345" → by_id, work_item_id=12345
- "Find tickets about authentication" → by_search, search_query="authentication"
- "What's the status of ticket 999" → by_id, work_item_id=999

If the request is unclear, set query_type to "unclear" and we'll ask the user to clarify.`

		actions := askAI(userMessage, systemPrompt, "", schema, "")
		logger.Debug("🔍 QueryTicket workflow: Returning AI action")
		return actions, nil

	case StepParseQuery:
		logger.Debug("🔍 QueryTicket workflow: StepParseQuery - Processing AI response")

		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			logger.Error("❌ QueryTicket workflow: No AI response")
			return nil, fmt.Errorf("expected AI response but got none")
		}

		response, ok := aiResponse.(*ai.Response)
		if !ok {
			logger.Errorf("❌ QueryTicket workflow: Invalid AI response type: %T", aiResponse)
			return nil, fmt.Errorf("invalid AI response type")
		}

		data := response.Data()
		queryType := data.MustGetString("query_type")

		// Check if query was unclear
		if queryType == "unclear" {
			msg := "I'm not sure how to search for that work item. Could you either:\n" +
				"- Provide a specific work item ID (e.g., 'show me ticket 12345')\n" +
				"- Provide search keywords (e.g., 'find tickets about login issues')"

			action := core.NewAction(core.ActionUserMessage)
			if action.Input == nil {
				action.Input = make(map[core.InputType]any)
			}
			action.Input[core.InputMessage] = msg
			return []*core.Action{action}, nil
		}

		// Build query parameters
		params := make(map[string]any)

		if queryType == "by_id" && data.Has("work_item_id") {
			workItemID := data.MustGetInt("work_item_id")
			params["work_item_id"] = workItemID
			workflow.SetWorkflowData("query_type", "by_id")
			workflow.SetWorkflowData("work_item_id", workItemID)
			logger.Debugf("🔍 QueryTicket: Querying by work_item_id=%d", workItemID)
		} else if queryType == "by_search" && data.Has("search_query") {
			searchQuery := data.MustGetString("search_query")
			params["search_query"] = searchQuery
			workflow.SetWorkflowData("query_type", "by_search")
			workflow.SetWorkflowData("search_query", searchQuery)
			logger.Debugf("🔍 QueryTicket: Querying by search_query=%s", searchQuery)
		} else {
			// Invalid query
			msg := "I couldn't extract valid search criteria from your request. Please try again."
			action := core.NewAction(core.ActionUserMessage)
			if action.Input == nil {
				action.Input = make(map[core.InputType]any)
			}
			action.Input[core.InputMessage] = msg
			return []*core.Action{action}, nil
		}

		// Call the query tool
		actions := callTool(tools.ToolADOQueryTicket, params)
		logger.Debug("🔍 QueryTicket workflow: Calling ADO query ticket tool")
		return actions, nil

	case StepQueryADO:
		logger.Debug("🔍 QueryTicket workflow: StepQueryADO - Processing tool result")

		toolResultRaw := getInput(sourceAction, core.InputToolResult)
		if toolResultRaw == nil {
			logger.Error("❌ QueryTicket workflow: No tool result received")
			return nil, fmt.Errorf("expected tool result but got none")
		}

		toolResult, ok := toolResultRaw.(*tools.ToolResult)
		if !ok {
			logger.Errorf("❌ QueryTicket workflow: Invalid tool result type: %T", toolResultRaw)
			return nil, fmt.Errorf("invalid tool result type")
		}

		var message string
		if !toolResult.Success {
			logger.Errorf("❌ QueryTicket workflow: Tool failed: %s", toolResult.Error)
			message = fmt.Sprintf("Failed to query work item: %s", toolResult.Error)
		} else {
			// Format the work item information
			workItemID := toolResult.Data["id"]
			title := toolResult.Data["title"]
			state := toolResult.Data["state"]
			workItemType := toolResult.Data["work_item_type"]
			assignedTo := toolResult.Data["assigned_to"]
			url := toolResult.Data["url"]

			message = fmt.Sprintf(`**Work Item #%v**

**Title:** %s
**Type:** %s
**State:** %s
**Assigned To:** %s

View in Azure DevOps: %s`,
				workItemID, title, workItemType, state, assignedTo, url)

			logger.Infof("✅ QueryTicket workflow: Found work item #%v", workItemID)
		}

		action := core.NewAction(core.ActionUserMessage)
		if action.Input == nil {
			action.Input = make(map[core.InputType]any)
		}
		action.Input[core.InputMessage] = message

		return []*core.Action{action}, nil

	default:
		logger.Warnf("⚠️  QueryTicket workflow: Unknown step: %v", step)
		return nil, fmt.Errorf("unknown workflow step: %v", step)
	}
}
