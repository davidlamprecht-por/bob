package orchestrator

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
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
	switch a.ActionType{
	case core.ActionWorkflow:
		return ActionWorkflow(a, ctx, responder, actionChan)
	case core.ActionWorkflowResult:
		return ActionWorkflowResult(a, ctx, responder, actionChan)
	case core.ActionAi:
		return ActionAI(a, ctx, responder, actionChan)
	case core.ActionTool:
		return ActionTool(a, ctx, responder, actionChan)
	case core.ActionUserMessage:
		return ActionUserMessage(a, ctx, responder, actionChan)
	case core.ActionUserWait:
		return ActionUserWait(a, ctx, responder, actionChan)
	case core.ActionAsync:
		return ActionAsync(a, ctx, responder, actionChan)
	}

	return nil, nil
}


func ActionWorkflow(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	return nil, nil
}

func ActionWorkflowResult(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	return nil, nil
}

func ActionAI(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
		// Extract input data
		userPrompt := a.Input[core.InputMessage].(string)
		personality, _ := a.Input[core.InputPersonality].(string)
		schema, _ := a.Input[core.InputSchema].(*ai.SchemaBuilder)
		conversationKey, _ := a.Input[core.InputConversationKey].(string)

		// Resolve conversation ID
		wf := ctx.GetCurrentWorkflow()
		conversationID := wf.GetAIConversation(&conversationKey)

		// Call AI layer
		goCtx := context.Background() // TODO: Pass context from higher level
		response, err := ai.SendMessage(goCtx, conversationID, userPrompt, personality, schema)
		if err != nil {
			// Return error - orchestrator will set StatusError
			return nil, fmt.Errorf("ai request failed: %w", err)
		}

		// Store the updated conversation ID
		if response.ConversationID != "" {
			wf.SetAIConversation(&conversationKey, &response.ConversationID)
		}

		// For synchronous calls, create ActionWorkflowResult with response data
		// The workflow will continue processing with the AI response
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
		// Non-blocking message to user
		if msg, ok := a.Input[core.InputMessage].(string); ok {
			responder(Response{Message: msg})
		}
		// Continue processing without creating more actions
		return nil, nil
}

func ActionUserWait(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
		// Blocking message to user - wait for response
		ctx.SetCurrentStatus(core.StatusWaitForUser) // Signal to stop main loop
		if msg, ok := a.Input[core.InputMessage].(string); ok {
			ctx.SetRequestToUser(msg)
			responder(Response{Message: msg})
		} else {
			// Okay this is an issue and something has gone wrong because we are supposed to have something for the user
			return nil, errors.New("expecting a message for the user but didn't receive one")
		}
		return nil, nil
}

func ActionAsync(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
		// Spawn goroutines for each sub-action
		for _, subAction := range a.AsyncActions {
			go func(action *Action) {
				// Process the action in parallel
				newActions, _ := ProcessAction(action, ctx, responder, actionChan)
				// Send new actions back to main loop via channel
				for _, newAction := range newActions {
					actionChan <- newAction
				}
			}(subAction)
		}
		return nil, nil
}
