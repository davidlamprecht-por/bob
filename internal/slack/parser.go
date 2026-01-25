package slack

import (
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack/slackevents"
)

func ParseMessage(ev *slackevents.MessageEvent) *core.Message {
	// DEBUG: Log what Slack is sending
	logger.Debugf("🔍 Slack message - TS: %s, ThreadTS: %s, Channel: %s, ChannelType: %s",
		ev.TimeStamp, ev.ThreadTimeStamp, ev.Channel, ev.ChannelType)

	// Extract thread ID - use ThreadTimeStamp if in thread, otherwise use TimeStamp
	threadID := ev.ThreadTimeStamp
	if threadID == "" {
		threadID = ev.TimeStamp
		logger.Debugf("🔍 No ThreadTS, using message TS as threadID: %s", threadID)
	} else {
		logger.Debugf("🔍 Using ThreadTS as threadID: %s", threadID)
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
