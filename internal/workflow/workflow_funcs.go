package workflow

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"fmt"
)

func getInput(a *core.Action, i core.InputType) any{
	if a.Input == nil {
		return nil
	}

	inputVal, ok := a.Input[i]
	if !ok{
		return nil
	}

	return inputVal
}


// subWorkerKey returns a namespaced workflowData key for a sub-worker.
// All sub-worker state lives under "sw_{id}_{key}" to avoid collisions.
func subWorkerKey(id string, key string) string {
	return fmt.Sprintf("sw_%s_%s", id, key)
}

// askAI creates an ActionAi action for sending a message to the AI layer
// Parameters:
//   - userMsg: The user's message (can be "" for system-only prompts)
//   - systemPrompt: System-level instructions (optional, can be empty)
//   - personality: AI personality/behavior instructions
//   - schema: Schema builder for structured output (optional, can be nil for free-form responses)
//   - conversationKey: Key for conversation ID resolution ("" = main conversation, "research" = ai_conv_research, etc.)
//
// Returns: A slice containing a single ActionAi action
func askAI(userMsg string, systemPrompt string, personality string, schema *ai.SchemaBuilder, conversationKey string) []*core.Action {
	action := core.NewAction(core.ActionAi)

	// Initialize Input map if needed
	if action.Input == nil {
		action.Input = make(map[core.InputType]any)
	}

	// Set input data
	action.Input[core.InputMessage] = userMsg
	action.Input[core.InputSystemPrompt] = systemPrompt
	action.Input[core.InputPersonality] = personality
	action.Input[core.InputSchema] = schema
	action.Input[core.InputConversationKey] = conversationKey

	return []*core.Action{action}
}
