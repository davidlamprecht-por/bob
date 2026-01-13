package slack

import (
	"bob/internal/orchestrator/core"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack/slackevents"
)

func ParseMessage(ev *slackevents.MessageEvent) *core.Message {
	// Extract thread ID - use ThreadTimeStamp if in thread, otherwise use TimeStamp
	threadID := ev.ThreadTimeStamp
	if threadID == "" {
		threadID = ev.TimeStamp
	}

	// Parse Slack timestamp to time.Time
	timestamp := parseSlackTS(ev.TimeStamp)

	return core.NewMessage(
		ev.User,            // userID
		ev.Channel,         // channel
		threadID,           // threadID
		core.PlatformSlack, // platform
		ev.Text,            // message
		timestamp,          // timestamp
	)
}

// parseSlackTS converts Slack timestamp (e.g., "1234567890.123456") to time.Time
func parseSlackTS(ts string) time.Time {
	// Slack timestamps are Unix timestamps with microseconds: "1234567890.123456"
	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Now() // Fallback if format is unexpected
	}

	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Now()
	}

	microseconds, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Now()
	}

	// Convert microseconds to nanoseconds for time.Unix
	return time.Unix(seconds, microseconds*1000)
}
