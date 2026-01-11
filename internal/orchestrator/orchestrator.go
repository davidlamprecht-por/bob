/*
Package orchestrator is the heart of Bob the slackbot. It manages how the user interacts with the AI_Layer and enables the AI to use tools and other actions
*/
package orchestrator

import (
	"bob/internal/orchestrator/core"
	"bob/internal/workflow"
)

type Orchestrator struct {

}

// Init starts up the Orchestrator when the script starts
func (o *Orchestrator) Init(){

}

func (o *Orchestrator) HandleUserMessage(message *core.Message, responder func(response core.Response)error) error {
	// TODO: Handle bursts of messages by only processing them after 1-2s as one

	context := core.LoadContext(message)

	if msg := handleEvictedContext(context, message) ; msg != "" {
		responder(core.Response{Message: msg})
	}

	intent := AnalyzeIntent(message, context)
	initialActions := ProcessUserIntent(intent)
		
	shouldHandleActions := RouteUserMessage(context, &intent, initialActions)

	if !shouldHandleActions{
		// If we are not starting another run, the AI might have had something to say about it
		if intent.MessageToUser != nil{
			responder(core.Response{Message: *intent.MessageToUser})
		}
		return nil
	}

	err := StartHandlingActions(initialActions, context, responder)
	return err
}

// AnalyzeIntent determines if message is answering a question or starting new request (stub)
func AnalyzeIntent(message *core.Message, context *core.ConversationContext) core.Intent {
	// TODO
	// Give the ai all the things it needs to know to identify user intend
	// ContextStatus (is it already running and the user wants to interject, is it a new request, an answer?),
	// last message to user, user message and whatever else
	// Also give AI Workflow specific actions that the user could be doing
	// Decide if the ai should give intent from a preselect

	// Parse it out as intent object

	return core.Intent{}
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
	if *intent.MessageToUser != "" && intent.MessageToUser != nil{
		a2 := core.NewAction(core.ActionUserMessage)
		a2.Input[core.InputMessage] = intent.MessageToUser
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

	return true
}

func StartHandlingActions(actionQueue []*core.Action, context *core.ConversationContext, responder func(response core.Response)error) error{
	// Channel for goroutines to send actions back to main loop
	actionChan := make(chan *core.Action, 100)

	// Mark as actively running
	context.SetCurrentStatus(core.StatusRunning)

	for len(actionQueue) > 0 {
		// If context still, or again has remaining actions, insert them now!
		// TODO: Might want to add priorty logic later, for now, just add to back
		actionQueue = append(actionQueue, context.PopRemainingActions()...)

		// Check if we should stop (e.g., hit ActionUserWait)
		if context.GetCurrentStatus() == core.StatusWaitForUser {
			// Store remaining actions for resumption
			context.SetRemainingActions(actionQueue)
			break
		}

		// -- Popleft steps
		currentAction := actionQueue[0]
		actionQueue = actionQueue[1:]
		// --

		newActions, err := ProcessAction(currentAction, context, responder, actionChan) // TODO: this might need to be done here in orchestrator to be able to keep action callable by other layers
		actionQueue = append(actionQueue, newActions...)

		// Drain channel (non-blocking) to collect any actions from goroutines
		for {
			select {
			case action := <-actionChan:
				actionQueue = append(actionQueue, action)
			default:
				goto continueLoop
			}
		}
		continueLoop:

		if err != nil {
			context.SetCurrentStatus(core.StatusError)
			context.SetRemainingActions(actionQueue)
			return err
		}
	}

	// If we finished normally (not waiting), mark as idle
	if context.GetCurrentStatus() == core.StatusRunning {
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
