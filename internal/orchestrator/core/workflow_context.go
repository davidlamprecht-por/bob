package core

import (
	"fmt"
	"time"
)

type WorkflowContext struct {
	id           int
	workflowName string
	step         string

	workflowData map[string]any
	aiConverstation *string // Main aiConverstation

	context *ConversationContext // Pointer to parent context for mutex locking
}

func NewWorkflow(name string) *WorkflowContext {
	return &WorkflowContext{workflowName: name, step: "init"}
}

// SetContext sets the parent ConversationContext (needed for mutex locking)
func (wf *WorkflowContext) SetContext(ctx *ConversationContext) {
	wf.context = ctx
}

func (wf *WorkflowContext) GetID() int{
	if wf.context != nil {
		wf.context.mu.RLock()
		defer wf.context.mu.RUnlock()
	}
	return wf.id
}

func (wf *WorkflowContext) GetWorkflowName() string {
	if wf.context != nil {
		wf.context.mu.RLock()
		defer wf.context.mu.RUnlock()
	}
	return wf.workflowName
}

func (wf *WorkflowContext) GetStep() string{
	if wf.context != nil {
		wf.context.mu.RLock()
		defer wf.context.mu.RUnlock()
	}
	return wf.step
}

func (wf *WorkflowContext) GetWorkflowData(key string) any {
	if wf.context != nil {
		wf.context.mu.RLock()
		defer wf.context.mu.RUnlock()
	}
	val, ok := wf.workflowData[key]
	if !ok {
		return nil
	}
	return val // nil could be in any as well
}

func (wf *WorkflowContext) GetAIConversation(key *string) *string {
	if wf.context != nil {
		wf.context.mu.RLock()
		defer wf.context.mu.RUnlock()
	}

	// Main Conversation
	if key == nil {
		if wf.aiConverstation == nil {
			fmt.Println("🔍 DEBUG: wf.aiConverstation is nil")
		} else {
			fmt.Printf("🔍 DEBUG: wf.aiConverstation = %s\n", *wf.aiConverstation)
		}
		return wf.aiConverstation
	}

	// Sub Conversations
	convKey := fmt.Sprintf("ai_conv_%s", *key)
	conv := wf.GetWorkflowData(convKey)

	// Try *string first (what we actually store)
	if strPtr, ok := conv.(*string); ok {
		return strPtr
	}

	// Fallback to string
	if str, ok := conv.(string); ok {
		return &str
	}

	return nil
}

// setters --------------------------------------------------------------------

func (wf *WorkflowContext) SetID(id int) {
	if wf.context != nil {
		wf.context.mu.Lock()
		defer wf.context.mu.Unlock()
		wf.context.lastUpdated = time.Now()
	}
	wf.id = id
}

func (wf *WorkflowContext) SetWorkflowName(name string) {
	if wf.context != nil {
		wf.context.mu.Lock()
		defer wf.context.mu.Unlock()
		wf.context.lastUpdated = time.Now()
	}
	wf.workflowName = name
}

func (wf *WorkflowContext) SetStep(step string) {
	if wf.context != nil {
		wf.context.mu.Lock()
		defer wf.context.mu.Unlock()
		wf.context.lastUpdated = time.Now()
	}
	wf.step = step
}

func (wf *WorkflowContext) SetWorkflowData(key string, value any) {
	if wf.context != nil {
		wf.context.mu.Lock()
		defer wf.context.mu.Unlock()
		wf.context.lastUpdated = time.Now()
	}
	if wf.workflowData == nil {
		wf.workflowData = make(map[string]any)
	}
	wf.workflowData[key] = value
}

func (wf *WorkflowContext) ResetWorkflowData(){
	if wf.context != nil {
		wf.context.mu.Lock()
		defer wf.context.mu.Unlock()
		wf.context.lastUpdated = time.Now()
	}
	wf.workflowData = make(map[string]any)
}

func (wf *WorkflowContext) SetAIConversation(key *string, conv *string) {
	if wf.context != nil {
		wf.context.mu.Lock()
		defer wf.context.mu.Unlock()
		wf.context.lastUpdated = time.Now()
	}
	if key == nil {
		wf.aiConverstation = conv
		return
	}
	convKey := fmt.Sprintf("ai_conv_%s", *key)
	wf.SetWorkflowData(convKey, conv)
}

