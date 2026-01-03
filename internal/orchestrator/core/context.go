package core

import (
	"log"
	"sync"
	"time"

	"bob/internal/database"
)

type ConversationContext struct {
	mu sync.RWMutex

	// Identity (resolved once at load time)
	userID   int // Internal DB ID
	threadID int // Internal DB ID

	currentWorkflow  *WorkflowContext
	currentStatus    ContextStatus
	lastUserMessages []*Message

	// State preservation for blocking/resuming
	remainingActions []*Action
	requestToUser    string

	// Timestamp of last modification
	lastUpdated time.Time
	createdAt   time.Time
}

// Getters

func (c *ConversationContext) GetCurrentWorkflow() *WorkflowContext {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentWorkflow
}

func (c *ConversationContext) GetCurrentStatus() ContextStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentStatus
}

func (c *ConversationContext) GetLastUserMessages() []*Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUserMessages
}

func (c *ConversationContext) GetRemainingActions() []*Action {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.remainingActions
}

func (c *ConversationContext) GetRequestToUser() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.requestToUser
}

func (c *ConversationContext) GetLastUpdated() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdated
}

// Setters

func (c *ConversationContext) SetCurrentWorkflow(wf *WorkflowContext) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentWorkflow = wf
	c.lastUpdated = time.Now()
}

func (c *ConversationContext) SetCurrentStatus(status ContextStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentStatus = status
	c.lastUpdated = time.Now()
}

func (c *ConversationContext) SetLastUserMessages(messages []*Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUserMessages = messages
	c.lastUpdated = time.Now()
}

func (c *ConversationContext) SetRemainingActions(actions []*Action) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remainingActions = actions
	c.lastUpdated = time.Now()
}

func (c *ConversationContext) SetRequestToUser(request string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestToUser = request
	c.lastUpdated = time.Now()
}

// Helper methods

func (c *ConversationContext) AppendUserMessage(msg *Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUserMessages = append(c.lastUserMessages, msg)
	c.lastUpdated = time.Now()
}

func (c *ConversationContext) AppendRemainingActions(actions []*Action) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remainingActions = append(c.remainingActions, actions...)
	c.lastUpdated = time.Now()
}

// PopRemainingActions atomically gets and clears remaining actions
func (c *ConversationContext) PopRemainingActions() []*Action {
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
	Id           int
	WorkflowName string
	Step         string

	WorkflowData map[string]any
}

func NewWorkflow(name string) *WorkflowContext {
	return &WorkflowContext{WorkflowName: name, Step: "init"}
}

func LoadContext(refMessage *Message) *ConversationContext {
	uID, tID, err := refMessage.GetResolved()
	if err != nil {
		log.Printf("ERROR: Failed to resolve IDs: %v", err)
		return nil
	}
	if uID == nil || tID == nil {
		log.Printf("something went wrong when loading user and thread")
		return nil
	}
	userID, threadID := *uID, *tID

	// 1. Check cache first (hot)
	context := GetFromCache(userID, threadID)
	if context != nil {
		context.AppendUserMessage(refMessage)
		return context
	}

	// 2. Load from DB (cold)
	context = loadContextFromDB(userID, threadID)

	// 3. If not found, create new
	if context == nil {
		context = &ConversationContext{
			userID:          userID,
			threadID:        threadID,
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

func loadContextFromDB(userID, threadID int) *ConversationContext {
	// Create repository
	repo := database.NewContextRepository(database.DB)

	// Load from DB using internal IDs
	dbContext, err := repo.LoadContext(userID, threadID)

	if err != nil {
		log.Printf("ERROR: Failed to load context from DB: %v", err)
		return nil
	}

	if dbContext == nil {
		return nil
	}

	// Convert database.WorkflowContext to orchestrator.WorkflowContext
	var wf *WorkflowContext
	if dbContext.Workflow != nil {
		wf = &WorkflowContext{
			Id:           *dbContext.Workflow.ID,
			WorkflowName: dbContext.Workflow.WorkflowName,
			Step:         dbContext.Workflow.Step,
			WorkflowData: dbContext.Workflow.WorkflowData,
		}
	}

	// Reconstruct ConversationContext with resolved IDs
	ctx := &ConversationContext{
		userID:   userID,
		threadID: threadID,

		currentWorkflow:  wf,
		currentStatus:    ContextStatus(dbContext.ContextStatus),
		lastUserMessages: []*Message{}, // Not persisted, starts empty
		remainingActions: nil,          // Action queue not persisted
		requestToUser:    dbContext.RequestToUser,
		lastUpdated:      time.Now(), // Set to now on load
		createdAt:        time.Now(), // Not persisted, approximate
	}

	return ctx
}

func (context *ConversationContext) UpdateDB() error {
	// Acquire read lock to read context state
	context.mu.RLock()
	currentWorkflow := context.currentWorkflow

	// Create repository
	repo := database.NewContextRepository(database.DB)

	// Convert orchestrator.WorkflowContext to database.WorkflowContext
	var dbWorkflow *database.WorkflowContext
	if currentWorkflow != nil {
		dbWorkflow = &database.WorkflowContext{
			ID:           &currentWorkflow.Id,
			WorkflowName: currentWorkflow.WorkflowName,
			Step:         currentWorkflow.Step,
			WorkflowData: currentWorkflow.WorkflowData,
		}
	}

	// Save to DB (using internal IDs)
	var dbContext *database.Context = &database.Context{
		UserID:        context.userID,
		ThreadID:      context.threadID,
		ContextStatus: string(context.currentStatus),
		RequestToUser: context.requestToUser,
		Workflow:      dbWorkflow,
	}
	context.mu.RUnlock()
	updatedWorkflowID, err := repo.SaveContext(dbContext)
	if err != nil {
		return err
	}

	// Update workflow DB ID if it changed
	if updatedWorkflowID != nil && context.currentWorkflow != nil {
		context.mu.Lock()
		context.currentWorkflow.Id = *updatedWorkflowID
		context.mu.Unlock()
	}

	return nil
}
