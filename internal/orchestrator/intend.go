package orchestrator

/*
Intend is the return data the initial ai gives to show what the user wants to do.
*/
type Intend struct {
	WorkflowName string
	
	Confidence float64
}
