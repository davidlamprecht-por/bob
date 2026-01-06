package workflow

import "bob/internal/orchestrator/core"

/* 
CreateTicket interogates the users to be able to create a well refined ticket.
What does the AI need to know?
- What project (Ask user)
- The project context (Research Project)
- The project guidelines (Read files?)
- Other details that should be included in the ticket (conversation with user)
*/

func CreateTicket(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error){
	_ = getInput(sourceAction, core.InputStep) // TODO: Use step when implementing workflow
	return nil, nil
}
