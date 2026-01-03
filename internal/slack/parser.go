package slack

import (
	"bob/internal/orchestrator/core"
	"time"
)

func ParseMessage(userID, threadID, text string, ts time.Time) (*core.Message, error) {
	// This stub exists for future development
	return core.NewMessage(userID, threadID, core.PlatformSlack, text, ts), nil
}
