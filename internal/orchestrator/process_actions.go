package orchestrator

import (
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/tool"
	"bob/internal/workflow"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

type (
	Action = core.Action
	ConversationContext = core.ConversationContext
	Response = core.Response
)

func ProcessAction(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action, pendingAsyncCount *int32) ([]*Action, error){
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
		result, err = ActionAsync(a, ctx, responder, actionChan, pendingAsyncCount)
	case core.ActionCompleteAsync:
		logger.Debug("   → ActionCompleteAsync")
		result, err = ActionCompleteAsync(a, ctx, responder, actionChan, pendingAsyncCount)
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
		// Copy the step from source action so workflow knows which step to execute
		if step, ok := a.Input[core.InputStep]; ok {
			resultAction.Input[core.InputStep] = step
		}
		resultAction.SourceWorkflow = a.SourceWorkflow

		return []*Action{resultAction}, nil
}

func ActionTool(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action) ([]*Action, error){
	logger.Debug("🔧 ActionTool: Extracting input data")
	// Extract input data
	toolName := a.Input[core.InputToolName].(tool.ToolName)
	toolArgs, _ := a.Input[core.InputToolArgs].(map[string]any)

	logger.Debugf("🔧 ActionTool: toolName=%q, toolArgs=%v", toolName, toolArgs)

	// Call tool
	logger.Debug("🔧 ActionTool: Calling tool.RunTool")
	result, err := tool.RunTool(ctx, toolName, toolArgs)
	if err != nil {
		logger.Errorf("❌ ActionTool: Tool execution failed: %v", err)
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	logger.Infof("✅ ActionTool: Tool executed successfully")

	// Create ActionWorkflowResult with tool result
	logger.Debug("🔧 ActionTool: Creating ActionWorkflowResult")
	resultAction := core.NewAction(core.ActionWorkflowResult)
	if resultAction.Input == nil {
		resultAction.Input = make(map[core.InputType]any)
	}
	resultAction.Input[core.InputToolResult] = result
	// Copy the step from source action so workflow knows which step to execute
	if step, ok := a.Input[core.InputStep]; ok {
		resultAction.Input[core.InputStep] = step
	}
	resultAction.SourceWorkflow = a.SourceWorkflow

	return []*Action{resultAction}, nil
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

func ActionAsync(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action, pendingAsyncCount *int32) ([]*Action, error){
		logger.Debugf("🔀 ActionAsync: Spawning %d goroutines", len(a.AsyncActions))

		// Count non-complete actions and check if we have a complete signal
		nonCompleteCount := 0
		hasComplete := false
		for _, subAction := range a.AsyncActions {
			if subAction.ActionType == core.ActionCompleteAsync {
				hasComplete = true
			} else {
				nonCompleteCount++
			}
		}

		// Adjust counter based on what this async action contains (thread-safe)
		if !hasComplete {
			// Starting new async operation
			atomic.AddInt32(pendingAsyncCount, 1)
			logger.Debugf("🔀 ActionAsync: Starting new async (counter: %d)", atomic.LoadInt32(pendingAsyncCount))
		} else if nonCompleteCount == 1 {
			// Just wrapping up with complete + 1 action (don't increment, complete will process and decrement)
			logger.Debugf("🔀 ActionAsync: Wrapping async completion (counter stays: %d)", atomic.LoadInt32(pendingAsyncCount))
		} else {
			// Multiple actions + complete = starting new async while completing old
			atomic.AddInt32(pendingAsyncCount, 1)
			logger.Debugf("🔀 ActionAsync: Starting new async + completing old (counter: %d)", atomic.LoadInt32(pendingAsyncCount))
		}

		// Spawn goroutines for each sub-action
		for i, subAction := range a.AsyncActions {
			go func(action *Action, index int) {
				logger.Debugf("🔀 ActionAsync: Processing sub-action %d", index)
				// Process the action in parallel - pass the SAME counter (thread-safe with atomic)
				newActions, _ := ProcessAction(action, ctx, responder, actionChan, pendingAsyncCount)
				// Send new actions back to main loop via channel
				for _, newAction := range newActions {
					actionChan <- newAction
				}
			}(subAction, i)
		}
		return nil, nil
}

func ActionCompleteAsync(a *Action, ctx *ConversationContext, responder func(response Response)error, actionChan chan<- *Action, pendingAsyncCount *int32) ([]*Action, error){
		logger.Debug("✅ ActionCompleteAsync: Decrementing async counter")
		atomic.AddInt32(pendingAsyncCount, -1)
		logger.Debugf("✅ ActionCompleteAsync: Async operation completed (counter: %d)", atomic.LoadInt32(pendingAsyncCount))
		return nil, nil
}
