package slack

import (
	"bob/internal/orchestrator"
	"time"
)

func ParseMessage(userId, threadId, text string, ts time.Time) (*orchestrator.Message, error) {
	// This stub exists for future development
	return &orchestrator.Message{
		UserID: &orchestrator.PlatformRef{
			Platform:   orchestrator.PlatformSlack,
			ExternalID: userId,
		},
		ThreadID: &orchestrator.PlatformRef{
			Platform:   orchestrator.PlatformSlack,
			ExternalID: threadId,
		},

		Message:   text,
		Timestamp: ts,
	}, nil
}
