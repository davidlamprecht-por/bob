package orchestrator

type Context struct {
	CurrentWorkflow *WorkflowContext
	CurrentStatus    ContextStatus
	LastUserMessages []*Message

	// State preservation for blocking/resuming
	RemainingActions []Action
	RequestToUser     string
}

type ContextStatus int

const (
	StatusIdle = iota
	StatusWaitForUser
	StatusRunning
	StatusError
)

type WorkflowContext struct {
	WorkflowName string
	Step         string

	WorkflowData map[string]interface{}
}

func NewWorkflow(name string) *WorkflowContext {
	return &WorkflowContext{WorkflowName: name, Step: "init"}
}

func LoadContext(refMessage *Message) *Context {
	var context *Context = nil
	// TODO: Check in-memory cache first (hot)
	// TODO: If not in cache, load from DB (cold)
	// TODO: If loading from DB, warm up cache

	if context == nil {
		context = loadContextFromDB(refMessage)
	}
	if context == nil {
		context = &Context{
			CurrentWorkflow: nil,
			CurrentStatus: StatusIdle,
		} // New empty context
	}
	context.LastUserMessages = append(context.LastUserMessages, refMessage)

	addContextToCache(refMessage, context)
	return context
}

func addContextToCache(refMessage *Message, context *Context) {
	// TODO
}

func loadContextFromDB(refMessage *Message) *Context {
	// TODO: Query DB by user id and thread id
	// TODO: Load persisted context if exists
	// TODO: Add refMessage to LastUserMessages

	return nil
}

func (context *Context) UpdateDB() error {
	// TODO: Serialize and save context to DB
	// TODO: Store by user id + thread id
	// TODO: Include timestamp
	// TODO: Do not include ActionQueue

	return nil
}
