package orchestrator

import "errors"

type Action struct {
	ActionType     ActionType
	SourceWorkflow string // Track which workflow spawned this action

	// For async result correlation
	AsyncGroupID   string                 // Workflow-generated ID for tracking async groups (empty if not part of a group)
	AsyncGroupSize int                    // Number of expected results in this group (1 if not async)

	// Generic data carrier
	Input map[string]any

	AsyncActions []Action
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
)

func (a *Action) ProcessAction(context *ConversationContext, responder func(response Response)error, actionChan chan<- Action) ([]Action, error){
	switch a.ActionType{
	case ActionWorkflow:
	case ActionWorkflowResult:
	case ActionAi:
	case ActionTool:
	case ActionUserMessage:
		// Non-blocking message to user
		if msg, ok := a.Input["message"].(string); ok {
			responder(Response{Message: msg})
		}
		// Continue processing without creating more actions

	case ActionUserWait:
		// Blocking message to user - wait for response
		context.SetCurrentStatus(StatusWaitForUser) // Signal to stop main loop
		if msg, ok := a.Input["message"].(string); ok {
			context.SetRequestToUser(msg)
			responder(Response{Message: msg})
		} else {
			// Okay this is an issue and something has gone wrong because we are supposed to have something for the user
			return nil, errors.New("expecting a message for the user but didn't receive one")
		}

	case ActionAsync:
		// Spawn goroutines for each sub-action
		for _, subAction := range a.AsyncActions {
			go func(action Action) {
				// Process the action in parallel
				newActions, _ := action.ProcessAction(context, responder, actionChan)
				// Send new actions back to main loop via channel
				for _, newAction := range newActions {
					actionChan <- newAction
				}
			}(subAction)
		}
	}

	return nil, nil
}
