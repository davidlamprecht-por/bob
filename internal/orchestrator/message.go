package orchestrator

type Message struct {
	UserID    string
	ThreadID  string
	Message   string
	Timestamp string
}

type Response struct {
	Message string
}
