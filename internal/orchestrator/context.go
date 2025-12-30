package orchestrator

type Context struct {
	CurrentWorkflow WorkflowContext
	UserRequest string
}

type WorkflowContext struct {
	step string
}
