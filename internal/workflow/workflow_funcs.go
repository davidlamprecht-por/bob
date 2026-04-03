package workflow

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
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


// askAI creates an ActionAi action for sending a message to the AI layer.
// Parameters:
//   - userMsg: The user's message
//   - personality: AI instructions — use a named Personality.Render(vars) for reusable prompts
//     or personalities.SystemPrompt("...") for simple one-offs
//   - schema: Schema builder for structured output (nil for free-form)
//   - conversationKey: "" = main conversation; any other key = isolated sub-conversation
//
// Returns: A slice containing a single ActionAi action
func askAI(userMsg string, personality string, schema *ai.SchemaBuilder, conversationKey string) []*core.Action {
	action := core.NewAction(core.ActionAi)

	if action.Input == nil {
		action.Input = make(map[core.InputType]any)
	}

	action.Input[core.InputMessage] = userMsg
	action.Input[core.InputPersonality] = personality
	action.Input[core.InputSchema] = schema
	action.Input[core.InputConversationKey] = conversationKey

	return []*core.Action{action}
}

// askAIBranch starts a new isolated AI sub-conversation on a unique auto-generated key.
// The generated key is returned in the result action's InputConversationKey field, so
// the workflow can persist it and reuse it for multi-turn sub-conversations:
//
//	case StepMyResult:
//	    convKey, _ := getInput(sourceAction, core.InputConversationKey).(string)
//	    wf.SetWorkflowData("my_conv", convKey)
//
//	case StepContinue:
//	    convKey, _ := wf.GetWorkflowData("my_conv").(string)
//	    actions := askAI(nextMsg, personality, schema, convKey)
func askAIBranch(userMsg string, personality string, schema *ai.SchemaBuilder) []*core.Action {
	actions := askAI(userMsg, personality, schema, "")
	actions[0].Input[core.InputGenerateKey] = true
	return actions
}
