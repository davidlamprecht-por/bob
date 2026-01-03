package core

import (
	"fmt"
	"time"

	"bob/internal/database"
)

type Message struct {
	UserID    *PlatformRef
	ThreadID  *PlatformRef
	Message   string
	Timestamp time.Time
}

func NewMessage(userID string, threadID string, platform PlatformType, message string, timestamp time.Time) *Message{
	return &Message{
		UserID: &PlatformRef{
			externalID: userID,
			platform: platform,
		},
		ThreadID: &PlatformRef{
			externalID: threadID,
			platform: platform,
		},
		Message: message,
		Timestamp: timestamp,
	}
}

type PlatformRef struct {
	externalID string
	platform   PlatformType

	internalID *int // Could be resolved later
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

func (p *PlatformRef) GetPlatform() PlatformType{
	return p.platform
}

func (m *Message) GetResolved() (*int, *int, error) {
	// Create ID resolver
	resolver := database.NewIDResolver(database.DB)

	// Resolve user ID
	if m.UserID.internalID == nil {
		id, err := resolver.ResolveUserID(m.UserID.externalID, string(m.UserID.platform))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve user ID: %w", err)
		}
		m.UserID.internalID = &id
	}

	// Resolve thread ID
	if m.ThreadID.internalID == nil {
		id, err := resolver.ResolveThreadID(m.ThreadID.externalID, string(m.ThreadID.platform))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve thread ID: %w", err)
		}
		m.ThreadID.internalID = &id
	}

	return m.UserID.internalID, m.ThreadID.internalID, nil
}
