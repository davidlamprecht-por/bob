package orchestrator

import (
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/workflow"
	"context"
	"errors"
	"fmt"
)

type (
	Action = core.Action
	ConversationContext = core.ConversationContext
	Response = core.Response
)

func ProcessAction(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	logger.Debugf("🎬 ProcessAction: type=%d", a.ActionType)

	var result []*Action
	var err error

	switch a.ActionType{
	case core.ActionWorkflow:
		logger.Debug("   → ActionWorkflow")
		result, err = ActionWorkflow(a, ctx, responder, actionChan)
	case core.ActionWorkflowResult:
		logger.Debug("   → ActionWorkflowResult")
		result, err = ActionWorkflowResult(a, ctx, responder, actionChan)
	case core.ActionAi:
		logger.Debug("   → ActionAI")
		result, err = ActionAI(a, ctx, responder, actionChan)
	case core.ActionTool:
		logger.Debug("   → ActionTool")
		result, err = ActionTool(a, ctx, responder, actionChan)
	case core.ActionUserMessage:
		logger.Debug("   → ActionUserMessage")
		result, err = ActionUserMessage(a, ctx, responder, actionChan)
	case core.ActionUserWait:
		logger.Debug("   → ActionUserWait")
		result, err = ActionUserWait(a, ctx, responder, actionChan)
	case core.ActionAsync:
		logger.Debug("   → ActionAsync")
		result, err = ActionAsync(a, ctx, responder, actionChan)
	default:
		logger.Debugf("   → Unknown action type: %d", a.ActionType)
	}

	logger.Debugf("🎬 ProcessAction complete: returned %d actions, error=%v", len(result), err)
	return result, err
}


func ActionWorkflow(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	logger.Debug("🔧 ActionWorkflow: Calling workflow.RunWorkflow")
	actions, err := workflow.RunWorkflow(ctx, a)
	logger.Debugf("🔧 ActionWorkflow: RunWorkflow returned %d actions, error=%v", len(actions), err)
	return actions, err
}

func ActionWorkflowResult(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	logger.Debug("🔙 ActionWorkflowResult: Sending result back to workflow")
	// This action carries a result (like AI response) back to the workflow
	// We need to call the workflow again with this result
	actions, err := workflow.RunWorkflow(ctx, a)
	logger.Debugf("🔙 ActionWorkflowResult: RunWorkflow returned %d actions, error=%v", len(actions), err)
	return actions, err
}

func ActionAI(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
		logger.Debug("🤖 ActionAI: Extracting input data")
		// Extract input data
		userPrompt := a.Input[core.InputMessage].(string)
		personality, _ := a.Input[core.InputPersonality].(string)
		schema, _ := a.Input[core.InputSchema].(*ai.SchemaBuilder)
		conversationKey, _ := a.Input[core.InputConversationKey].(string)

		logger.Debugf("🤖 ActionAI: userPrompt=%q, personality=%q, conversationKey=%q", userPrompt, personality, conversationKey)

		// Resolve conversation ID
		wf := ctx.GetCurrentWorkflow()
		// If conversationKey is empty string, pass nil (main conversation)
		var keyPtr *string
		if conversationKey != "" {
			keyPtr = &conversationKey
		}
		conversationID := wf.GetAIConversation(keyPtr)
		logger.Debugf("🤖 ActionAI: conversationID=%v", conversationID)

		// Call AI layer
		logger.Debug("🤖 ActionAI: Calling ai.SendMessage")
		goCtx := context.Background() // TODO: Pass context from higher level
		response, err := ai.SendMessage(goCtx, conversationID, userPrompt, personality, schema)
		if err != nil {
			logger.Errorf("❌ ActionAI: AI request failed: %v", err)
			// Return error - orchestrator will set StatusError
			return nil, fmt.Errorf("ai request failed: %w", err)
		}

		logger.Infof("✅ ActionAI: AI responded successfully, conversationID=%s", response.ConversationID)

		// Store the updated conversation ID
		if response.ConversationID != "" {
			// If conversationKey is empty string, pass nil (main conversation)
			var keyPtr *string
			if conversationKey != "" {
				keyPtr = &conversationKey
			}
			convID := response.ConversationID
			wf.SetAIConversation(keyPtr, &convID)
			logger.Debugf("🤖 ActionAI: Stored conversation ID with key=%v", keyPtr)
		}

		// For synchronous calls, create ActionWorkflowResult with response data
		// The workflow will continue processing with the AI response
		logger.Debug("🤖 ActionAI: Creating ActionWorkflowResult")
		resultAction := core.NewAction(core.ActionWorkflowResult)
		if resultAction.Input == nil {
			resultAction.Input = make(map[core.InputType]any)
		}
		resultAction.Input[core.InputAIResponse] = response
		resultAction.SourceWorkflow = a.SourceWorkflow

		return []*Action{resultAction}, nil
}

func ActionTool(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	return nil, nil
}

func ActionUserMessage(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
		logger.Debug("💬 ActionUserMessage: Sending message to user")
		// Non-blocking message to user
		if msg, ok := a.Input[core.InputMessage].(string); ok {
			logger.Debugf("💬 Message content: %q", msg)
			err := responder(Response{Message: msg})
			if err != nil {
				logger.Errorf("❌ Failed to send message: %v", err)
			} else {
				logger.Debug("✅ Message sent successfully")
			}
		} else {
			logger.Warn("⚠️  No message found in action input")
		}
		// Continue processing without creating more actions
		return nil, nil
}

func ActionUserWait(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
		logger.Debug("⏸️  ActionUserWait: Waiting for user response")
		// Blocking message to user - wait for response
		ctx.SetCurrentStatus(core.StatusWaitForUser) // Signal to stop main loop
		if msg, ok := a.Input[core.InputMessage].(string); ok {
			logger.Debugf("⏸️  Sending wait message: %q", msg)
			ctx.SetRequestToUser(msg)
			responder(Response{Message: msg})
		} else {
			// Okay this is an issue and something has gone wrong because we are supposed to have something for the user
			logger.Error("❌ ActionUserWait: No message found in action input")
			return nil, errors.New("expecting a message for the user but didn't receive one")
		}
		return nil, nil
}

func ActionAsync(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
		logger.Debugf("🔀 ActionAsync: Spawning %d goroutines", len(a.AsyncActions))
		// Spawn goroutines for each sub-action
		for i, subAction := range a.AsyncActions {
			go func(action *Action, index int) {
				logger.Debugf("🔀 ActionAsync: Processing sub-action %d", index)
				// Process the action in parallel
				newActions, _ := ProcessAction(action, ctx, responder, actionChan)
				// Send new actions back to main loop via channel
				for _, newAction := range newActions {
					actionChan <- newAction
				}
			}(subAction, i)
		}
		return nil, nil
}
