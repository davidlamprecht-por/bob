package orchestrator

type Action struct {
	ActionType ActionType
}

type ActionType int

const (
	ActionWorkflow = iota
	ActionAi
	ActionTool
	ActionUserMessage
	ActionUserWait
	ActionAsync
)

