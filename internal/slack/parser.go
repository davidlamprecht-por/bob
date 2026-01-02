package slack

import (
	"bob/internal/orchestrator"
	"time"
)

func ParseMessage(userID, threadID, text string, ts time.Time) (*orchestrator.Message, error) {
	// This stub exists for future development
	return orchestrator.NewMessage(userID, threadID, orchestrator.PlatformSlack, text, ts), nil
}
