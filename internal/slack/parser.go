package slack

// ParsedMessage represents a structured Slack message
type ParsedMessage struct {
	UserID    string
	ThreadID  string
	Message   string
	Channel   string
	Timestamp string
	IsThread  bool
}

// ParseEvent extracts common fields from Slack events
// This is a placeholder for future expansion
func ParseEvent(event interface{}) (*ParsedMessage, error) {
	// For this PoC, we parse directly in the handler
	// This stub exists for future development
	return &ParsedMessage{}, nil
}
