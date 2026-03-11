package core

import (
	"bob/internal/logger"
	"fmt"
	"time"
)

type WorkflowContext struct {
	id           int
	workflowName string
	step         string

	workflowData map[string]any

	context ConversationContext
}

func NewWorkflow(name string) *WorkflowContext {
	return &WorkflowContext{workflowName: name, step: "init"}
}

func (wf *WorkflowContext) GetID() int {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	return wf.id
}

func (wf *WorkflowContext) GetWorkflowName() string {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	return wf.workflowName
}

func (wf *WorkflowContext) GetStep() string {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	return wf.step
}

func (wf *WorkflowContext) GetWorkflowData(key string) any {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	val, ok := wf.workflowData[key]
	if !ok {
		return nil
	}
	return val // nil could be in any as well
}

// GetAIConversation retrieves the AI conversation ID for an isolated sub-conversation.
// key must be non-nil; use ConversationContext.GetMainConversation() for the main thread conversation.
func (wf *WorkflowContext) GetAIConversation(key *string) *string {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()

	convKey := fmt.Sprintf("ai_conv_%s", *key)
	logger.Debugf("🔍 GetAIConversation: key=%s, convKey=%s", *key, convKey)
	conv, ok := wf.workflowData[convKey]
	logger.Debugf("🔍 GetAIConversation: conv data=%v (type=%T)", conv, conv)

	if !ok {
		logger.Debugf("🔍 GetAIConversation: key not found, returning nil")
		return nil
	}
	c, ok := conv.(*string)
	if !ok {
		logger.Debugf("🔍 GetAIConversation: type assertion failed, returning nil")
		return nil
	}
	logger.Debugf("🔍 GetAIConversation: returning conversation ID=%s", *c)
	return c
}

// setters --------------------------------------------------------------------

func (wf *WorkflowContext) SetID(id int) {
	wf.context.mu.Lock()
	defer wf.context.mu.Unlock()
	wf.context.lastUpdated = time.Now()
	wf.id = id
}

func (wf *WorkflowContext) SetWorkflowName(name string) {
	wf.context.mu.Lock()
	defer wf.context.mu.Unlock()
	wf.context.lastUpdated = time.Now()
	wf.workflowName = name
}

func (wf *WorkflowContext) SetStep(step string) {
	wf.context.mu.Lock()
	defer wf.context.mu.Unlock()
	wf.context.lastUpdated = time.Now()
	wf.step = step
}

func (wf *WorkflowContext) SetWorkflowData(key string, value any) {
	wf.context.mu.Lock()
	defer wf.context.mu.Unlock()
	wf.context.lastUpdated = time.Now()
	if wf.workflowData == nil {
		wf.workflowData = make(map[string]any)
	}
	wf.workflowData[key] = value
}

func (wf *WorkflowContext) ResetWorkflowData() {
	wf.context.mu.Lock()
	defer wf.context.mu.Unlock()
	wf.context.lastUpdated = time.Now()
	wf.workflowData = make(map[string]any)
}

// SetAIConversation stores the conversation ID for an isolated sub-conversation.
// key must be non-nil; use ConversationContext.SetMainConversation() for the main thread conversation.
func (wf *WorkflowContext) SetAIConversation(key *string, conv *string) {
	wf.context.mu.Lock()
	defer wf.context.mu.Unlock()
	wf.context.lastUpdated = time.Now()

	convKey := fmt.Sprintf("ai_conv_%s", *key)
	if conv != nil {
		logger.Debugf("🔍 SetAIConversation: key=%s, convKey=%s, conversationID=%s", *key, convKey, *conv)
	} else {
		logger.Debugf("🔍 SetAIConversation: key=%s, convKey=%s, resetting to nil", *key, convKey)
	}
	if wf.workflowData == nil {
		wf.workflowData = make(map[string]any)
	}
	wf.workflowData[convKey] = conv
	logger.Debugf("🔍 SetAIConversation: stored successfully")
}
