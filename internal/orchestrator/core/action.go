package core

type Action struct {
	ActionType     ActionType
	SourceWorkflow string // Track which workflow spawned this action

	// For async result correlation
	AsyncGroupID   string                 // Workflow-generated ID for tracking async groups (empty if not part of a group)
	AsyncGroupSize int                    // Number of expected results in this group (1 if not async)

	// Generic data carrier
	Input map[InputType]any

	AsyncActions []*Action
}

type ActionType int

const (
	ActionWorkflow       = iota // Execute workflow step
	ActionWorkflowResult        // Deliver result back to workflow from async operation
	ActionAi
	ActionTool
	ActionUserMessage // Sending a message to user, not expecting a result
	ActionUserWait    // Sending a message to user, expecting a result = blocking
	ActionAsync
	ActionCompleteAsync // Signal that async operations are complete
)

type InputType string

const (
	InputStep            = "step"
	InputMessage         = "message"
	InputSystemPrompt    = "system_prompt"
	InputPersonality     = "personality"
	InputSchema          = "schema"
	InputConversationKey = "conversation_key"
	InputAsyncGroupID    = "async_group_id"
	InputAIResponse      = "ai_response"      // For storing AI response data
	InputError           = "error"            // For error handling in results
	InputToolName        = "tool_name"        // ToolName to execute
	InputToolArgs        = "tool_args"        // Tool arguments map[string]any
	InputToolResult      = "tool_result"      // Tool result map[string]any
	InputWorkflowName    = "workflow_name"    // Target workflow for sub-workflow dispatch
	InputSubWorkerID     = "sub_worker_id"   // String ID for sub-workflow instance
)

func NewAction(actionType ActionType) *Action{
	return &Action{
		ActionType: actionType,
		SourceWorkflow: "",
		AsyncGroupID: "",
		AsyncGroupSize: 1,
		Input: nil,
		AsyncActions: make([]*Action, 0),
	}
}

