package core

/*
Intent is the return data the initial ai gives to show what the user wants to do.
*/
type Intent struct {
	IntentType   IntentType
	WorkflowName string
	Confidence   float64

	Reasoning     string
	MessageToUser *string // Optional
}

type IntentType string

const (
	IntentNewWorkflow    = "NewWorkflow" // If no workflow was set beforehand, this needs a higher confidence level!
	IntentAnswerQuestion = "AnswerQuestion"
	IntentAskQuestion    = "AskRelatedQuestion" // This is an easy one to get back to if unsure of others. Any Workflow should be able to deal with clarifications.
)
