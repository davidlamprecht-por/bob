/*
Package orchestrator is the heart of Bob the slackbot. It manages how the user interacts with the AI_Layer and enables the AI to use tools and other actions
*/
package orchestrator

import (
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/workflow"
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

type Orchestrator struct {
}

// Init starts up the Orchestrator when the script starts
func (o *Orchestrator) Init() {

}

func (o *Orchestrator) HandleUserMessage(message *core.Message, responder func(response core.Response) error) error {
	logger.Debug("🎯 Orchestrator.HandleUserMessage: Entry")
	// TODO: Handle bursts of messages by only processing them after 1-2s as one

	context := core.LoadContext(message)
	logger.Debugf("📦 Context loaded: status=%s, workflow=%v", context.GetCurrentStatus(), context.GetCurrentWorkflow())

	if msg := handleEvictedContext(context, message); msg != "" {
		logger.Debug("⚠️  Context was evicted, sending recovery message")
		responder(core.Response{Message: msg})
	}

	intent := AnalyzeIntent(message, context)
	logger.Debugf("🧠 Intent analyzed: type=%s, workflow=%s, confidence=%.2f", intent.IntentType, intent.WorkflowName, intent.Confidence)

	initialActions := ProcessUserIntent(intent)
	logger.Debugf("⚙️  Initial actions created: count=%d", len(initialActions))
	for i, action := range initialActions {
		logger.Debugf("   Action[%d]: type=%d", i, action.ActionType)
	}

	shouldHandleActions := RouteUserMessage(context, &intent, initialActions)
	logger.Debugf("🚦 RouteUserMessage result: shouldHandle=%v", shouldHandleActions)

	if !shouldHandleActions {
		logger.Debug("🛑 Not handling actions this turn")
		return nil
	}

	logger.Debug("▶️  Starting action handling loop")
	err := StartHandlingActions(initialActions, context, responder)
	if err != nil {
		logger.Errorf("❌ Action handling error: %v", err)
	} else {
		logger.Debug("✅ Action handling completed successfully")
	}

	// Save context to database after handling
	if dbErr := context.UpdateDB(); dbErr != nil {
		logger.Warnf("⚠️  Failed to save context to DB: %v", dbErr)
	} else {
		logger.Debug("💾 Context saved to database")
	}

	return err
}

// formatIntentDetails formats an Intent object into a readable string for testing
func formatIntentDetails(intent core.Intent) string {
	msg := "=== Intent Analysis ===\n"
	msg += "Intent Type: " + string(intent.IntentType) + "\n"
	msg += "Workflow Name: " + intent.WorkflowName + "\n"
	msg += "Confidence: " + fmt.Sprintf("%.2f", intent.Confidence) + "\n"
	msg += "Reasoning: " + intent.Reasoning + "\n"
	if intent.MessageToUser != nil && *intent.MessageToUser != "" {
		msg += "Message to User: " + *intent.MessageToUser + "\n"
	} else {
		msg += "Message to User: (none)\n"
	}
	msg += "====================="
	return msg
}

// handleEvictedContext handles context that was evicted from cache (stub for smart recovery)
func handleEvictedContext(context *core.ConversationContext, _ *core.Message) string {
	// Special handling for evicted contexts
	if context.GetCurrentStatus() == core.StatusEvicted {
		// This is a stub. Later we need smarter handling of the eviction process
		return "Sorry, I lost track of our conversation due to high load. Can you remind me what you needed help with?"
	}
	return ""
}

func ProcessUserIntent(intent core.Intent) []*core.Action {
	// NeedsUserInput = true means the intent analyzer asked a clarifying question and needs
	// the user's answer before routing can proceed. A non-nil MessageToUser without
	// NeedsUserInput is just a courtesy acknowledgment — it goes out non-blocking below.
	if intent.NeedsUserInput && intent.MessageToUser != nil && *intent.MessageToUser != "" {
		waitAction := core.NewAction(core.ActionUserWait)
		waitAction.Input = map[core.InputType]any{
			core.InputMessage: *intent.MessageToUser,
		}
		return []*core.Action{waitAction}
	}

	a := core.NewAction(core.ActionWorkflow)
	a.Input = make(map[core.InputType]any)

	switch intent.IntentType {
	case core.IntentNewWorkflow:
		step := intent.Step
		if step == "" {
			step = workflow.StepInit
		}
		a.Input[core.InputStep] = step
	case core.IntentAnswerQuestion:
		step := intent.Step
		if step == "" {
			step = workflow.StepUserAnsweringQuestion
		}
		a.Input[core.InputStep] = step
	case core.IntentAskQuestion:
		// MessageToUser is nil/empty here (handled above), so this is a side question
		step := intent.Step
		if step == "" {
			step = workflow.StepUserAsksQuestion
		}
		a.Input[core.InputStep] = step
	}

	return []*core.Action{a}
}

func RouteUserMessage(context *core.ConversationContext, intent *core.Intent, actions []*core.Action) (startNewLoop bool) {
	// Case 1: Context exists and we're waiting for user response
	if context != nil && context.GetCurrentStatus() == core.StatusWaitForUser {
		if intent.IntentType == core.IntentNewWorkflow {
			context.SetCurrentWorkflow(core.NewWorkflow(intent.WorkflowName))
			context.SetRemainingActions(nil)
		} else if intent.WorkflowName != "" {
			// Returning from clarifying question — route to different workflow if needed
			current := context.GetCurrentWorkflow()
			if current == nil || current.GetWorkflowName() != intent.WorkflowName {
				context.SetCurrentWorkflow(core.NewWorkflow(intent.WorkflowName))
				context.SetRemainingActions(nil)
			}
		}
		context.SetCurrentStatus(core.StatusRunning)
		return true
	}

	// Case 2: Context exists and we're actively processing
	if context != nil && context.GetCurrentStatus() == core.StatusRunning {
		context.AppendRemainingActions(actions)
		return false
	}

	// Case 3: Starting fresh (idle or no workflow)
	if intent.IntentType == core.IntentNewWorkflow {
		context.SetCurrentWorkflow(core.NewWorkflow(intent.WorkflowName))
	}

	return true
}

func StartHandlingActions(actionQueue []*core.Action, context *core.ConversationContext, responder func(response core.Response) error) error {
	logger.Debugf("🔄 StartHandlingActions: Starting with %d actions", len(actionQueue))
	// Channel for goroutines to send actions back to main loop
	actionChan := make(chan *core.Action, 100)

	// Mark as actively running
	context.SetCurrentStatus(core.StatusRunning)

	// Track pending async operations (int32 for atomic operations)
	var pendingAsyncCount int32 = 0

	actionCount := 0
	for {
		// Check if we should stop (waiting for user) - MUST check this FIRST
		if context.GetCurrentStatus() == core.StatusWaitForUser {
			logger.Debug("⏸️  Waiting for user, pausing action loop")
			// Filter out stale ActionCompleteAsync — these decrement pendingAsyncCount and
			// become invalid after a WaitForUser pause (pendingAsyncCount resets to 0 on resume).
			filtered := make([]*core.Action, 0, len(actionQueue))
			for _, a := range actionQueue {
				if a.ActionType != core.ActionCompleteAsync {
					filtered = append(filtered, a)
				}
			}
			context.SetRemainingActions(filtered)
			break
		}

		// Check if we're truly done (atomic read)
		currentPending := atomic.LoadInt32(&pendingAsyncCount)
		if len(actionQueue) == 0 && currentPending <= 0 {
			logger.Debug("🏁 Action queue empty and no pending async operations")
			break
		}

		// If queue is empty but we have pending async, wait for results
		if len(actionQueue) == 0 && currentPending > 0 {
			logger.Debugf("⏳ Queue empty but %d async operations pending, waiting on channel...", currentPending)
			action := <-actionChan
			logger.Debug("📨 Received action from async operation")
			actionQueue = append(actionQueue, action)
		}

		actionCount++
		logger.Debugf("🔁 Action loop iteration %d: queue size=%d, pending async=%d", actionCount, len(actionQueue), atomic.LoadInt32(&pendingAsyncCount))

		// If context still, or again has remaining actions, insert them now!
		// TODO: Might want to add priorty logic later, for now, just add to back
		actionQueue = append(actionQueue, context.PopRemainingActions()...)

		// -- Popleft steps
		currentAction := actionQueue[0]
		actionQueue = actionQueue[1:]
		logger.Debugf("📤 Processing action: type=%d", currentAction.ActionType)
		// --

		newActions, err := ProcessAction(currentAction, context, responder, actionChan, &pendingAsyncCount)
		logger.Debugf("📥 ProcessAction returned: %d new actions, error=%v, pending async=%d", len(newActions), err, pendingAsyncCount)
		actionQueue = append(actionQueue, newActions...)

		// Drain channel (non-blocking) to collect any actions from goroutines
		for {
			select {
			case action := <-actionChan:
				logger.Debug("📨 Received action from goroutine channel")
				actionQueue = append(actionQueue, action)
			default:
				goto continueLoop
			}
		}
	continueLoop:

		if err != nil {
			logger.Errorf("❌ Action processing failed: %v", err)
			context.SetCurrentStatus(core.StatusError)
			context.SetRemainingActions(actionQueue)
			return err
		}
	}

	// If we finished normally (not waiting), record history and mark as idle
	if context.GetCurrentStatus() == core.StatusRunning {
		logger.Debug("✓ Recording workflow completion and marking context as idle")
		recordWorkflowCompletion(context)
		context.SetCurrentStatus(core.StatusIdle)
		context.SetRemainingActions(nil) // just to make sure
	}

	return nil
}

// CanExecute Helps identify if the app can do certain actions on behalf of the user
func (o *Orchestrator) CanExecute(action core.Action, ctx *core.ConversationContext) bool {
	// default allow for now
	return true
}

// recordWorkflowCompletion generates a 1-sentence AI summary of the completed workflow and
// appends it to the thread's workflow history. Runs synchronously before marking idle so
// the summary is always persisted. The bot has already sent its last user-facing message
// at this point, so the brief AI call is invisible to the user.
func recordWorkflowCompletion(ctx *core.ConversationContext) {
	wf := ctx.GetCurrentWorkflow()
	if wf == nil {
		return
	}
	workflowName := wf.GetWorkflowName()

	// Use the first user message in this turn as the trigger message
	triggerMsg := ""
	messages := ctx.GetLastUserMessages()
	if len(messages) > 0 {
		triggerMsg = messages[0].Message
	}

	schema := ai.NewSchema().
		AddString("summary", ai.Required(), ai.Description("One sentence summary of what the workflow did. Make sure to add specific details that makes this workflow run stand out from others like Ticket Number, Request type or other."))

	prompt := fmt.Sprintf(
		"Summarize in one sentence: the user said %q and the %q workflow ran.",
		triggerMsg, workflowName,
	)

	summary := ""
	response, err := ai.SendMessage(
		context.Background(),
		nil, // fresh conversation — no history needed
		prompt,
		"You are a brief workflow summarizer. Respond with a single sentence.",
		schema,
	)
	if err != nil {
		logger.Warnf("⚠️  recordWorkflowCompletion: AI summary failed: %v", err)
	} else {
		if s, err := response.Data().GetString("summary"); err == nil {
			summary = s
		}
	}

	entry := &core.WorkflowHistoryEntry{
		WorkflowName:   workflowName,
		TriggerMessage: triggerMsg,
		Summary:        summary,
		CompletedAt:    time.Now(),
	}
	ctx.PushWorkflowHistory(entry)
	logger.Debugf("📚 Recorded workflow history: %s — %q", workflowName, summary)
}
