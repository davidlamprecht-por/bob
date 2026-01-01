package orchestrator

import (
	"sync"
	"time"
)

type Context struct {
	mu sync.RWMutex

	currentWorkflow  *WorkflowContext
	currentStatus    ContextStatus
	lastUserMessages []*Message

	// State preservation for blocking/resuming
	remainingActions []Action
	requestToUser    string

	// Timestamp of last modification
	lastUpdated time.Time
	createdAt   time.Time
}

// Getters
func (c *Context) GetCurrentWorkflow() *WorkflowContext {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentWorkflow
}

func (c *Context) GetCurrentStatus() ContextStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentStatus
}

func (c *Context) GetLastUserMessages() []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUserMessages
}

func (c *Context) GetRemainingActions() []Action {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.remainingActions
}

func (c *Context) GetRequestToUser() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.requestToUser
}

func (c *Context) GetLastUpdated() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdated
}

// Setters
func (c *Context) SetCurrentWorkflow(wf *WorkflowContext) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentWorkflow = wf
	c.lastUpdated = time.Now()
}

func (c *Context) SetCurrentStatus(status ContextStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentStatus = status
	c.lastUpdated = time.Now()
}

func (c *Context) SetLastUserMessages(messages []*Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUserMessages = messages
	c.lastUpdated = time.Now()
}

func (c *Context) SetRemainingActions(actions []Action) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remainingActions = actions
	c.lastUpdated = time.Now()
}

func (c *Context) SetRequestToUser(request string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestToUser = request
	c.lastUpdated = time.Now()
}

// Helper methods
func (c *Context) AppendUserMessage(msg *Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUserMessages = append(c.lastUserMessages, msg)
	c.lastUpdated = time.Now()
}

func (c *Context) AppendRemainingActions(actions []Action) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remainingActions = append(c.remainingActions, actions...)
	c.lastUpdated = time.Now()
}

// PopRemainingActions atomically gets and clears remaining actions
func (c *Context) PopRemainingActions() []Action {
	c.mu.Lock()
	defer c.mu.Unlock()
	actions := c.remainingActions
	c.remainingActions = nil
	c.lastUpdated = time.Now()
	return actions
}

type ContextStatus string

const (
	StatusIdle        = "idle"
	StatusWaitForUser = "waitingForUser"
	StatusRunning     = "running"
	StatusError       = "error"
	StatusEvicted     = "evicted" // Context was evicted from cache while active
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
	userID := refMessage.UserID.ExternalID
	threadID := refMessage.ThreadID.ExternalID

	// 1. Check cache first (hot)
	context := GetFromCache(userID, threadID)
	if context != nil {
		context.AppendUserMessage(refMessage)
		return context
	}

	// 2. Load from DB (cold)
	context = loadContextFromDB(refMessage)

	// 3. If not found, create new
	if context == nil {
		context = &Context{
			currentWorkflow: nil,
			currentStatus:   StatusIdle,
			lastUpdated:     time.Now(),
			createdAt:       time.Now(),
		}
	}

	context.AppendUserMessage(refMessage)

	// 4. Put in cache
	PutInCache(userID, threadID, context)

	return context
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
