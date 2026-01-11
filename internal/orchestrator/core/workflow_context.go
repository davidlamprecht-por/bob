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

	context ConversationContext
}

func NewWorkflow(name string) *WorkflowContext {
	return &WorkflowContext{workflowName: name, step: "init"}
}

func (wf *WorkflowContext) GetID() int{
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	return wf.id
}

func (wf *WorkflowContext) GetWorkflowName() string {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	return wf.workflowName
}

func (wf *WorkflowContext) GetStep() string{
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

func (wf *WorkflowContext) GetAIConversation(key *string) *string {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()

	// Main Conversation
	if key == nil {
		return wf.aiConverstation
	}
	
	// Sub Conversations
	convKey := fmt.Sprintf("ai_conv_%s", *key)
	conv := wf.GetWorkflowData(convKey)
	
	c, ok := conv.(string)
	if !ok {
		return nil
	}
	return &c
}

// setters --------------------------------------------------------------------

func (wf *WorkflowContext) SetID(id int) {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	wf.context.lastUpdated = time.Now()

	wf.id = id
}

func (wf *WorkflowContext) SetWorkflowName(name string) {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	wf.context.lastUpdated = time.Now()

	wf.workflowName = name
}

func (wf *WorkflowContext) SetStep(step string) {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	wf.context.lastUpdated = time.Now()

	wf.step = step
}

func (wf *WorkflowContext) SetWorkflowData(key string, value any) {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	wf.context.lastUpdated = time.Now()

	if wf.workflowData == nil {
		wf.workflowData = make(map[string]any)
	}
	wf.workflowData[key] = value
}

func (wf *WorkflowContext) ResetWorkflowData(){
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	wf.context.lastUpdated = time.Now()

	wf.workflowData = make(map[string]any)
}

func (wf *WorkflowContext) SetAIConversation(key *string, conv *string) {
	wf.context.mu.RLock()
	defer wf.context.mu.RUnlock()
	wf.context.lastUpdated = time.Now()

	if key == nil {
		wf.aiConverstation = conv
		return
	}
	convKey := fmt.Sprintf("ai_conv_%s", *key)
	wf.SetWorkflowData(convKey, conv)
}

