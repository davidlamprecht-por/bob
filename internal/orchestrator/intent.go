package orchestrator

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
	IntentNewRequest     = "NewRequest"
	IntentAnswerQuestion = "AnswerQuestion"
)
