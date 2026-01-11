/*
Package orchestrator is the heart of Bob the slackbot. It manages how the user interacts with the AI_Layer and enables the AI to use tools and other actions
*/
package orchestrator

import (
	"log"

	"bob/internal/orchestrator/core"
	"bob/internal/workflow"
)

type Orchestrator struct {

}

// Init starts up the Orchestrator when the script starts
func (o *Orchestrator) Init(){

}

func (o *Orchestrator) HandleUserMessage(message *core.Message, responder func(response core.Response)error) error {
	log.Println("🎯 Orchestrator.HandleUserMessage: Entry")
	// TODO: Handle bursts of messages by only processing them after 1-2s as one

	context := core.LoadContext(message)
	log.Printf("📦 Context loaded: status=%s, workflow=%v", context.GetCurrentStatus(), context.GetCurrentWorkflow())

	if msg := handleEvictedContext(context, message) ; msg != "" {
		log.Println("⚠️  Context was evicted, sending recovery message")
		responder(core.Response{Message: msg})
	}

	intent := AnalyzeIntent(message, context)
	log.Printf("🧠 Intent analyzed: type=%s, workflow=%s, confidence=%.2f", intent.IntentType, intent.WorkflowName, intent.Confidence)

	initialActions := ProcessUserIntent(intent)
	log.Printf("⚙️  Initial actions created: count=%d", len(initialActions))
	for i, action := range initialActions {
		log.Printf("   Action[%d]: type=%d", i, action.ActionType)
	}

	shouldHandleActions := RouteUserMessage(context, &intent, initialActions)
	log.Printf("🚦 RouteUserMessage result: shouldHandle=%v", shouldHandleActions)

	if !shouldHandleActions{
		log.Println("🛑 Not handling actions this turn")
		// If we are not starting another run, the AI might have had something to say about it
		if intent.MessageToUser != nil{
			responder(core.Response{Message: *intent.MessageToUser})
		}
		return nil
	}

	log.Println("▶️  Starting action handling loop")
	err := StartHandlingActions(initialActions, context, responder)
	if err != nil {
		log.Printf("❌ Action handling error: %v", err)
	} else {
		log.Println("✅ Action handling completed successfully")
	}

	// Save context to database after handling
	if dbErr := context.UpdateDB(); dbErr != nil {
		log.Printf("⚠️  Failed to save context to DB: %v", dbErr)
	} else {
		log.Println("💾 Context saved to database")
	}

	return err
}

// AnalyzeIntent determines if message is answering a question or starting new request (stub)
func AnalyzeIntent(message *core.Message, context *core.ConversationContext) core.Intent {
	// TODO: Implement full AI-powered intent analysis
	// Give the ai all the things it needs to know to identify user intend
	// ContextStatus (is it already running and the user wants to interject, is it a new request, an answer?),
	// last message to user, user message and whatever else
	// Also give AI Workflow specific actions that the user could be doing
	// Decide if the ai should give intent from a preselect

	// STUB: For testing, always return IntentNewWorkflow with testAI
	// This allows us to test the Slack-to-AI integration
	return core.Intent{
		IntentType:    core.IntentNewWorkflow,
		WorkflowName:  "testAI",
		Confidence:    1.0,
		Reasoning:     "Stub implementation for testing",
		MessageToUser: nil,
	}
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

func ProcessUserIntent(intent core.Intent) []*core.Action{
	actions := make([]*core.Action, 0)
	a := core.NewAction(core.ActionWorkflow)
	switch intent.IntentType{
	case core.IntentNewWorkflow:
		a.Input[core.InputStep] = workflow.StepInit
	case core.IntentAnswerQuestion:
		a.Input[core.InputStep] = workflow.StepUserAnsweringQuestion
	case core.IntentAskQuestion:
		a.Input[core.InputStep] = workflow.StepUserAsksQuestion
	}
	actions = append(actions, a)
	if intent.MessageToUser != nil && *intent.MessageToUser != ""{
		a2 := core.NewAction(core.ActionUserMessage)
		a2.Input[core.InputMessage] = *intent.MessageToUser
		actions = append(actions, a2)
	}
	return actions
}

func RouteUserMessage (context *core.ConversationContext, intent *core.Intent, actions []*core.Action) (startNewLoop bool) {
	// Case 1: Context exists and we're waiting for user response
	if context != nil && context.GetCurrentStatus() == core.StatusWaitForUser {
		if intent.IntentType == core.IntentNewWorkflow{
			// User is changing direction - start new workflow
			// TODO: Clear old state, start fresh workflow
			context.SetCurrentWorkflow(core.NewWorkflow(intent.WorkflowName))
			context.SetRemainingActions(nil)
		}
		context.SetCurrentStatus(core.StatusRunning)
		return true
	}

	// Case 2: Context exists and we're actively processing
	if context != nil && context.GetCurrentStatus() == core.StatusRunning {
		// Add actions to context, we are reusing RemainingActions to solve a similar problem
		context.AppendRemainingActions(actions)
		return false
	}

	// Case 3: New conversation or idle context - start new workflow if needed
	if intent.IntentType == core.IntentNewWorkflow {
		// Only create a new workflow if one doesn't exist or if it's a different workflow
		currentWorkflow := context.GetCurrentWorkflow()
		if currentWorkflow == nil || currentWorkflow.GetWorkflowName() != intent.WorkflowName {
			context.SetCurrentWorkflow(core.NewWorkflow(intent.WorkflowName))
		}
	}

	return true
}

func StartHandlingActions(actionQueue []*core.Action, context *core.ConversationContext, responder func(response core.Response)error) error{
	log.Printf("🔄 StartHandlingActions: Starting with %d actions", len(actionQueue))
	// Channel for goroutines to send actions back to main loop
	actionChan := make(chan *core.Action, 100)

	// Mark as actively running
	context.SetCurrentStatus(core.StatusRunning)

	actionCount := 0
	for len(actionQueue) > 0 {
		actionCount++
		log.Printf("🔁 Action loop iteration %d: queue size=%d", actionCount, len(actionQueue))

		// If context still, or again has remaining actions, insert them now!
		// TODO: Might want to add priorty logic later, for now, just add to back
		actionQueue = append(actionQueue, context.PopRemainingActions()...)

		// Check if we should stop (e.g., hit ActionUserWait)
		if context.GetCurrentStatus() == core.StatusWaitForUser {
			log.Println("⏸️  Waiting for user, pausing action loop")
			// Store remaining actions for resumption
			context.SetRemainingActions(actionQueue)
			break
		}

		// -- Popleft steps
		currentAction := actionQueue[0]
		actionQueue = actionQueue[1:]
		log.Printf("📤 Processing action: type=%d", currentAction.ActionType)
		// --

		newActions, err := ProcessAction(currentAction, context, responder, actionChan) // TODO: this might need to be done here in orchestrator to be able to keep action callable by other layers
		log.Printf("📥 ProcessAction returned: %d new actions, error=%v", len(newActions), err)
		actionQueue = append(actionQueue, newActions...)

		// Drain channel (non-blocking) to collect any actions from goroutines
		for {
			select {
			case action := <-actionChan:
				log.Println("📨 Received action from goroutine channel")
				actionQueue = append(actionQueue, action)
			default:
				goto continueLoop
			}
		}
		continueLoop:

		if err != nil {
			log.Printf("❌ Action processing failed: %v", err)
			context.SetCurrentStatus(core.StatusError)
			context.SetRemainingActions(actionQueue)
			return err
		}
	}

	log.Println("🏁 Action loop completed")

	// If we finished normally (not waiting), mark as idle
	if context.GetCurrentStatus() == core.StatusRunning {
		log.Println("✓ Marking context as idle")
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
