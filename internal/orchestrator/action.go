package orchestrator

type Action struct {
	ActionType ActionType
	
	AsyncActions []Action
}

type ActionType int

const (
	ActionWorkflow = iota
	ActionAi
	ActionTool
	ActionUserMessage // Sending a message to user, not expecting a result
	ActionUserWait		// Sending a message to user, expecting a result = blocking
	ActionAsync
)

func (a *Action) ProcessAction(context *Context, responder func(response Response)error) []Action{
	switch a.ActionType{
	case ActionWorkflow:
	case ActionAi:
	case ActionTool:
	case ActionUserMessage:
		// Sending 
	case ActionUserWait:
	case ActionAsync:
	}	

	return nil
}
