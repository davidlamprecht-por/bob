package orchestrator

import (
	"log"

	"bob/internal/ai"
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
	log.Printf("🎬 ProcessAction: type=%d", a.ActionType)

	var result []*Action
	var err error

	switch a.ActionType{
	case core.ActionWorkflow:
		log.Println("   → ActionWorkflow")
		result, err = ActionWorkflow(a, ctx, responder, actionChan)
	case core.ActionWorkflowResult:
		log.Println("   → ActionWorkflowResult")
		result, err = ActionWorkflowResult(a, ctx, responder, actionChan)
	case core.ActionAi:
		log.Println("   → ActionAI")
		result, err = ActionAI(a, ctx, responder, actionChan)
	case core.ActionTool:
		log.Println("   → ActionTool")
		result, err = ActionTool(a, ctx, responder, actionChan)
	case core.ActionUserMessage:
		log.Println("   → ActionUserMessage")
		result, err = ActionUserMessage(a, ctx, responder, actionChan)
	case core.ActionUserWait:
		log.Println("   → ActionUserWait")
		result, err = ActionUserWait(a, ctx, responder, actionChan)
	case core.ActionAsync:
		log.Println("   → ActionAsync")
		result, err = ActionAsync(a, ctx, responder, actionChan)
	default:
		log.Printf("   → Unknown action type: %d", a.ActionType)
	}

	log.Printf("🎬 ProcessAction complete: returned %d actions, error=%v", len(result), err)
	return result, err
}


func ActionWorkflow(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	log.Println("🔧 ActionWorkflow: Calling workflow.RunWorkflow")
	actions, err := workflow.RunWorkflow(ctx, a)
	log.Printf("🔧 ActionWorkflow: RunWorkflow returned %d actions, error=%v", len(actions), err)
	return actions, err
}

func ActionWorkflowResult(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	log.Println("🔙 ActionWorkflowResult: Sending result back to workflow")
	// This action carries a result (like AI response) back to the workflow
	// We need to call the workflow again with this result
	actions, err := workflow.RunWorkflow(ctx, a)
	log.Printf("🔙 ActionWorkflowResult: RunWorkflow returned %d actions, error=%v", len(actions), err)
	return actions, err
}

func ActionAI(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
		log.Println("🤖 ActionAI: Extracting input data")
		// Extract input data
		userPrompt := a.Input[core.InputMessage].(string)
		personality, _ := a.Input[core.InputPersonality].(string)
		schema, _ := a.Input[core.InputSchema].(*ai.SchemaBuilder)
		conversationKey, _ := a.Input[core.InputConversationKey].(string)

		log.Printf("🤖 ActionAI: userPrompt=%q, personality=%q, conversationKey=%q", userPrompt, personality, conversationKey)

		// Resolve conversation ID
		wf := ctx.GetCurrentWorkflow()
		// If conversationKey is empty string, pass nil (main conversation)
		var keyPtr *string
		if conversationKey != "" {
			keyPtr = &conversationKey
		}
		conversationID := wf.GetAIConversation(keyPtr)
		log.Printf("🤖 ActionAI: conversationID=%v", conversationID)

		// Call AI layer
		log.Println("🤖 ActionAI: Calling ai.SendMessage")
		goCtx := context.Background() // TODO: Pass context from higher level
		response, err := ai.SendMessage(goCtx, conversationID, userPrompt, personality, schema)
		if err != nil {
			log.Printf("❌ ActionAI: AI request failed: %v", err)
			// Return error - orchestrator will set StatusError
			return nil, fmt.Errorf("ai request failed: %w", err)
		}

		log.Printf("✅ ActionAI: AI responded successfully, conversationID=%s", response.ConversationID)

		// Store the updated conversation ID
		if response.ConversationID != "" {
			// If conversationKey is empty string, pass nil (main conversation)
			var keyPtr *string
			if conversationKey != "" {
				keyPtr = &conversationKey
			}
			convID := response.ConversationID
			wf.SetAIConversation(keyPtr, &convID)
			log.Printf("🤖 ActionAI: Stored conversation ID with key=%v", keyPtr)
		}

		// For synchronous calls, create ActionWorkflowResult with response data
		// The workflow will continue processing with the AI response
		log.Println("🤖 ActionAI: Creating ActionWorkflowResult")
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
		log.Println("💬 ActionUserMessage: Sending message to user")
		// Non-blocking message to user
		if msg, ok := a.Input[core.InputMessage].(string); ok {
			log.Printf("💬 Message content: %q", msg)
			err := responder(Response{Message: msg})
			if err != nil {
				log.Printf("❌ Failed to send message: %v", err)
			} else {
				log.Println("✅ Message sent successfully")
			}
		} else {
			log.Println("⚠️  No message found in action input")
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
