package orchestrator

import "time"

type Message struct {
	UserID    *PlatformRef
	ThreadID  *PlatformRef
	Message   string
	Timestamp time.Time
}

type PlatformRef struct {
	ExternalID string
	Platform   PlatformType

	InternalID *string // Could be resolved later
}

type PlatformType string

const (
	PlatformSlack = "Slack"
	// PlatformTeams
	// PlatformDiscord etc
	// PlatformEmail?
)

type Response struct {
	Message string
}

func (m *Message) GetResolved(){
	if m.UserID.InternalID == nil {
		// TODO: Resolve with db class
	}
	if m.ThreadID.InternalID == nil {
		// TODO: Resolve with db class
	}
}
