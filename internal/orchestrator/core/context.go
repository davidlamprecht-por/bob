// Package core deals with Orchestrator related structs that are needed by other layers as well
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
	if wf != nil {
		wf.SetContext(c) // Link workflow to parent context
	}
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
	log.Printf("🔍 LoadContext: userID=%d, threadID=%d", userID, threadID)

	// 1. Check cache first (hot)
	context := GetFromCache(userID, threadID)
	if context != nil {
		log.Printf("🔍 LoadContext: Found in cache")
		context.AppendUserMessage(refMessage)
		return context
	}

	// 2. Load from DB (cold)
	log.Printf("🔍 LoadContext: Not in cache, loading from DB")
	context = loadContextFromDB(userID, threadID)

	// 3. If not found, create new
	if context == nil {
		log.Printf("🔍 LoadContext: Not found in DB, creating new context")
		context = &ConversationContext{
			userID:          userID,
			threadID:        threadID,
			currentWorkflow: nil,
			currentStatus:   StatusIdle,
			lastUpdated:     time.Now(),
			createdAt:       time.Now(),
		}
	} else {
		log.Printf("🔍 LoadContext: Loaded from DB, workflow=%v", context.currentWorkflow)
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
		log.Printf("🔍 loadContextFromDB: repo.LoadContext returned nil")
		return nil
	}

	log.Printf("🔍 loadContextFromDB: dbContext.Workflow=%v", dbContext.Workflow)
	if dbContext.Workflow != nil {
		log.Printf("🔍 loadContextFromDB: workflow.ID=%v, name=%s, ai_conversation=%v",
			dbContext.Workflow.ID, dbContext.Workflow.WorkflowName, dbContext.Workflow.AIConversation)
	}

	// Reconstruct ConversationContext with resolved IDs
	ctx := &ConversationContext{
		userID:   userID,
		threadID: threadID,

		currentWorkflow:  nil, // Set after linking
		currentStatus:    ContextStatus(dbContext.ContextStatus),
		lastUserMessages: []*Message{}, // Not persisted, starts empty
		remainingActions: nil,          // Action queue not persisted
		requestToUser:    dbContext.RequestToUser,
		lastUpdated:      time.Now(), // Set to now on load
		createdAt:        time.Now(), // Not persisted, approximate
	}

	// Convert database.WorkflowContext to orchestrator.WorkflowContext
	if dbContext.Workflow != nil {
		wf := &WorkflowContext{
			id:              *dbContext.Workflow.ID,
			workflowName:    dbContext.Workflow.WorkflowName,
			step:            dbContext.Workflow.Step,
			workflowData:    dbContext.Workflow.WorkflowData,
			aiConverstation: dbContext.Workflow.AIConversation,
		}
		// Link workflow to its parent context
		wf.SetContext(ctx)
		ctx.currentWorkflow = wf
	}

	return ctx
}

func (c *ConversationContext) UpdateDB() error {
	// Acquire read lock to read context state
	c.mu.RLock()
	currentWorkflow := c.currentWorkflow

	// Create repository
	repo := database.NewContextRepository(database.DB)

	// Convert orchestrator.WorkflowContext to database.WorkflowContext
	var dbWorkflow *database.WorkflowContext
	if currentWorkflow != nil {
		// Only set ID if it's been saved before (non-zero)
		var workflowID *int
		if currentWorkflow.id > 0 {
			workflowID = &currentWorkflow.id
			log.Printf("🔍 UpdateDB: workflow.id=%d (will UPDATE)", currentWorkflow.id)
		} else {
			log.Printf("🔍 UpdateDB: workflow.id=0 (will INSERT)")
		}

		log.Printf("🔍 UpdateDB: aiConverstation=%v", currentWorkflow.aiConverstation)

		dbWorkflow = &database.WorkflowContext{
			ID:             workflowID,
			WorkflowName:   currentWorkflow.workflowName,
			Step:           currentWorkflow.step,
			WorkflowData:   currentWorkflow.workflowData,
			AIConversation: currentWorkflow.aiConverstation,
		}
	}

	// Save to DB (using internal IDs)
	var dbContext = &database.Context{
		UserID:        c.userID,
		ThreadID:      c.threadID,
		ContextStatus: string(c.currentStatus),
		RequestToUser: c.requestToUser,
		Workflow:      dbWorkflow,
	}
	c.mu.RUnlock()
	updatedWorkflowID, err := repo.SaveContext(dbContext)
	if err != nil {
		return err
	}

	// Update workflow DB ID if it changed
	if updatedWorkflowID != nil && c.currentWorkflow != nil {
		log.Printf("🔍 UpdateDB: SaveContext returned workflow ID=%d", *updatedWorkflowID)
		c.mu.Lock()
		c.currentWorkflow.id = *updatedWorkflowID
		log.Printf("🔍 UpdateDB: Set workflow.id to %d", c.currentWorkflow.id)
		c.mu.Unlock()
	}

	return nil
}
