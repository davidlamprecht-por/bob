package workflow

import (
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/tools"
	"fmt"
)

const (
	StepGatherInfo    = "gather_info"
	StepConfirm       = "confirm"
	StepCreateInADO   = "create_in_ado"
	StepComplete      = "complete"
)

/*
CreateTicket workflow guides the user through creating a well-defined Azure DevOps ticket.
Flow:
1. StepInit: Ask AI to gather ticket information from user
2. StepGatherInfo: Process AI response and store ticket details
3. StepConfirm: Ask user to confirm before creating
4. StepCreateInADO: Call ADO create ticket tool
5. StepComplete: Display results to user
*/

func CreateTicket(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🎫 CreateTicket workflow: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🎫 CreateTicket workflow: step=%v", step)

	workflow := context.GetCurrentWorkflow()

	switch step {
	case StepInit:
		logger.Debug("🎫 CreateTicket workflow: StepInit - Gathering ticket information")

		// Get user's initial request
		messages := context.GetLastUserMessages()
		if len(messages) == 0 {
			logger.Error("❌ CreateTicket workflow: No user messages found")
			return nil, fmt.Errorf("no user message found")
		}
		userMessage := messages[len(messages)-1].Message

		// Create schema for gathering ticket information
		schema := ai.NewSchema().
			AddString("project", ai.Required(),
				ai.Description("Azure DevOps project name")).
			AddString("title", ai.Required(),
				ai.Description("Concise ticket title")).
			AddString("work_item_type", ai.Required(),
				ai.Enum("Task", "Bug", "User Story", "Feature"),
				ai.Description("Type of work item")).
			AddString("description", ai.Required(),
				ai.Description("Detailed description of the work item")).
			AddString("assigned_to",
				ai.Description("Email or name of assignee (optional)")).
			AddArray("tags", ai.FieldTypeString,
				ai.Description("Tags to categorize the work item (optional)")).
			AddString("priority",
				ai.Enum("1", "2", "3", "4"),
				ai.Description("Priority: 1=Critical, 2=High, 3=Medium, 4=Low (optional)"))

		systemPrompt := `You are helping the user create an Azure DevOps work item.
Based on their request, extract or ask for the necessary information:
- project: The Azure DevOps project name
- title: A clear, concise title (minimum 5 characters)
- work_item_type: Task, Bug, User Story, or Feature
- description: Detailed description of what needs to be done
- assigned_to: Who should work on this (optional)
- tags: Relevant tags for categorization (optional)
- priority: 1-4, where 1 is most critical (optional, default to 3 if not specified)

If any required information is missing from the user's request, ask them for it.
Be conversational and helpful.`

		actions := askAI(userMessage, systemPrompt, "", schema, "")
		logger.Debug("🎫 CreateTicket workflow: Returning AI action")
		return actions, nil

	case StepGatherInfo:
		logger.Debug("🎫 CreateTicket workflow: StepGatherInfo - Processing AI response")

		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			logger.Error("❌ CreateTicket workflow: No AI response")
			return nil, fmt.Errorf("expected AI response but got none")
		}

		response, ok := aiResponse.(*ai.Response)
		if !ok {
			logger.Errorf("❌ CreateTicket workflow: Invalid AI response type: %T", aiResponse)
			return nil, fmt.Errorf("invalid AI response type")
		}

		data := response.Data()

		// Store all ticket information in workflow data
		workflow.SetWorkflowData("project", data.MustGetString("project"))
		workflow.SetWorkflowData("title", data.MustGetString("title"))
		workflow.SetWorkflowData("work_item_type", data.MustGetString("work_item_type"))
		workflow.SetWorkflowData("description", data.MustGetString("description"))

		if data.Has("assigned_to") {
			workflow.SetWorkflowData("assigned_to", data.MustGetString("assigned_to"))
		}
		if data.Has("tags") {
			workflow.SetWorkflowData("tags", data.MustGetArray("tags"))
		}
		if data.Has("priority") {
			workflow.SetWorkflowData("priority", data.MustGetString("priority"))
		}

		// Create confirmation message
		confirmMsg := fmt.Sprintf(`I'll create the following work item:

**Project:** %s
**Type:** %s
**Title:** %s
**Description:** %s`,
			workflow.GetWorkflowData("project"),
			workflow.GetWorkflowData("work_item_type"),
			workflow.GetWorkflowData("title"),
			workflow.GetWorkflowData("description"))

		if assignedTo := workflow.GetWorkflowData("assigned_to"); assignedTo != nil {
			confirmMsg += fmt.Sprintf("\n**Assigned To:** %s", assignedTo)
		}
		if tags := workflow.GetWorkflowData("tags"); tags != nil {
			confirmMsg += fmt.Sprintf("\n**Tags:** %v", tags)
		}
		if priority := workflow.GetWorkflowData("priority"); priority != nil {
			confirmMsg += fmt.Sprintf("\n**Priority:** %s", priority)
		}

		confirmMsg += "\n\nShould I proceed with creating this ticket?"

		// Ask user for confirmation
		action := core.NewAction(core.ActionUserWait)
		if action.Input == nil {
			action.Input = make(map[core.InputType]any)
		}
		action.Input[core.InputMessage] = confirmMsg

		logger.Debug("🎫 CreateTicket workflow: Asking user for confirmation")
		return []*core.Action{action}, nil

	case StepConfirm:
		logger.Debug("🎫 CreateTicket workflow: StepConfirm - User responded")

		// Get user's confirmation
		messages := context.GetLastUserMessages()
		if len(messages) == 0 {
			return nil, fmt.Errorf("no user response found")
		}
		userResponse := messages[len(messages)-1].Message

		// Simple confirmation check (you could use AI to parse this more intelligently)
		// For now, check for common affirmative responses
		confirmed := contains(userResponse, "yes") || contains(userResponse, "proceed") ||
			contains(userResponse, "confirm") || contains(userResponse, "go ahead") ||
			contains(userResponse, "ok") || contains(userResponse, "sure")

		if !confirmed {
			// User declined - send cancellation message
			msg := "Ticket creation cancelled. Let me know if you'd like to try again!"
			action := core.NewAction(core.ActionUserMessage)
			if action.Input == nil {
				action.Input = make(map[core.InputType]any)
			}
			action.Input[core.InputMessage] = msg
			return []*core.Action{action}, nil
		}

		// User confirmed - create the ticket
		logger.Debug("🎫 CreateTicket workflow: User confirmed, creating ticket")

		// Build parameters from workflow data
		params := map[string]any{
			"project":        workflow.GetWorkflowData("project"),
			"title":          workflow.GetWorkflowData("title"),
			"work_item_type": workflow.GetWorkflowData("work_item_type"),
			"description":    workflow.GetWorkflowData("description"),
		}

		if assignedTo := workflow.GetWorkflowData("assigned_to"); assignedTo != nil {
			params["assigned_to"] = assignedTo
		}
		if tags := workflow.GetWorkflowData("tags"); tags != nil {
			params["tags"] = tags
		}
		if priority := workflow.GetWorkflowData("priority"); priority != nil {
			params["priority"] = priority
		}

		actions := callTool(tools.ToolADOCreateTicket, params)
		logger.Debug("🎫 CreateTicket workflow: Calling ADO create ticket tool")
		return actions, nil

	case StepCreateInADO:
		logger.Debug("🎫 CreateTicket workflow: StepCreateInADO - Processing tool result")

		toolResultRaw := getInput(sourceAction, core.InputToolResult)
		if toolResultRaw == nil {
			logger.Error("❌ CreateTicket workflow: No tool result received")
			return nil, fmt.Errorf("expected tool result but got none")
		}

		toolResult, ok := toolResultRaw.(*tools.ToolResult)
		if !ok {
			logger.Errorf("❌ CreateTicket workflow: Invalid tool result type: %T", toolResultRaw)
			return nil, fmt.Errorf("invalid tool result type")
		}

		var message string
		if !toolResult.Success {
			logger.Errorf("❌ CreateTicket workflow: Tool failed: %s", toolResult.Error)
			message = fmt.Sprintf("Failed to create ticket: %s", toolResult.Error)
		} else {
			workItemID := toolResult.Data["id"]
			workItemURL := toolResult.Data["url"]
			message = fmt.Sprintf("✅ Successfully created work item #%v!\n\nView it here: %s",
				workItemID, workItemURL)
			logger.Infof("✅ CreateTicket workflow: Created work item #%v", workItemID)
		}

		action := core.NewAction(core.ActionUserMessage)
		if action.Input == nil {
			action.Input = make(map[core.InputType]any)
		}
		action.Input[core.InputMessage] = message

		return []*core.Action{action}, nil

	default:
		logger.Warnf("⚠️  CreateTicket workflow: Unknown step: %v", step)
		return nil, fmt.Errorf("unknown workflow step: %v", step)
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func toLower(s string) string {
	result := make([]rune, len(s))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + ('a' - 'A')
		} else {
			result[i] = r
		}
	}
	return string(result)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
