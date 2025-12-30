package orchestrator

type Context struct {
	CurrentWorkflow WorkflowContext
	UserRequest     string

	CurrentStatus ContextStatus
	LastUserMessages []Message
}

type ContextStatus int

const (
	StatusWaitForUser = iota
	StatusProcessingUserMessage = iota
)

type WorkflowContext struct {
	step string
}


func GetContext(refMessage Message) *Context{
	return nil
}
