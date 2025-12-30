/*
Package orchestrator is the heart of Bob the slackbot. It manages how the user interacts with the AI_Layer and enables the AI to use tools and other actions
*/
package orchestrator

type Orchestrator struct {

}

// Init starts up the Orchestrator when the script starts
func (o *Orchestrator) Init(){

}

func (o *Orchestrator) HandleUserMessage(message Message, responder func(response Response)error) error {
	// TODO: Handle bursts of messages by only processing them after 1-2s as one

	// Potentially need to load from DB, think about caching.
	context := GetContext(message)

	// Identify intend from user with AI
	// - Use current workflow state as context
	// - Depending on context is how hard the ai can push towards a new workflow or suggest change direction or next steps within workflow
	// IN: User Message and Context and Workflow Specific Actions
	// OUT: Intend: Workflow to call, extra info the workflow might need
	intend := Intend{}

	initialActions := ProcessUserIntend(intend)
	err := StartHandlingActions(initialActions, context, responder)
	return err
}

func ProcessUserIntend(intend Intend) []Action{
	return nil
}

func StartHandlingActions(actionQueue []Action, context *Context, responder func(response Response)error) error{
	for len(actionQueue) > 0 {
		// -- Popleft steps
		currentAction := actionQueue[0]
		actionQueue = actionQueue[1:]
		// --

		newActions := currentAction.ProcessAction(context, responder)
		actionQueue = append(actionQueue, newActions...)
	}
	return nil
}


// CanExecute Helps identify if the app can do certain actions on behalf of the user
func (o *Orchestrator) CanExecute(action Action, ctx *Context) bool {
  // default allow for now
  return true
}
