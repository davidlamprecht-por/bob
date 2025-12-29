# GO-001: Mock Slack Layer - Basic Message Handler Implementation Plan

## Overview

Create a minimalistic command-line driven proof-of-concept to validate Slack Bolt SDK for Go works correctly. This is intentionally kept **extremely lean** for testing purposes only.

## Current State Analysis

**What exists:**
- Project structure: `cmd/`, `internal/slack/`, `go.mod`
- `.env.dist` already has Slack token placeholders
- Empty `internal/slack/` directory ready for code

**What's missing:**
- Slack SDK dependencies in `go.mod`
- Command entry point `cmd/slack-test/main.go`
- Message parser in `internal/slack/parser.go`
- No Slack integration code

**Key constraints:**
- Keep it minimal - this is a proof-of-concept
- DMs only (no channels, threads, mentions)
- Socket Mode (no webhooks/ngrok needed)
- Focus on validating SDK integration works

## Desired End State

A working command-line Slack bot that:
1. Connects to Slack via Socket Mode
2. Listens for DM messages only
3. Logs incoming messages to stdout with structure
4. Responds with "Pong! 🏓" to each DM
5. Can be run with: `go run cmd/slack-test/main.go`

**Verification:**
- Start bot: logs show "Slack bot starting..."
- Send DM in Slack workspace
- Bot responds with "Pong! 🏓"
- Stdout shows parsed message structure (user, text, channel, timestamp)

## What We're NOT Doing

- Channel messages, threads, or mentions
- Database integration
- Complex message parsing
- Error recovery/retry logic
- Unit tests (manual testing only for PoC)
- Production-grade code structure
- Graceful shutdown handling
- Configuration beyond env vars

## Implementation Approach

Single-phase sequential implementation since this is a small, simple PoC with tightly coupled components. The bot needs all parts working together to function, and there's no benefit to parallelization for such a small codebase.

## Execution Strategy

### Sequential Execution
All work must be completed in order:
- Phase 1: Complete implementation

**Rationale**: This is a minimal proof-of-concept with ~150 lines of code total. All components (main, handler, parser) are tightly coupled and must work together. Sequential execution is clearer and faster than the overhead of splitting into phases.

---

## Phase 1: Implement Slack Bot PoC

**Execution**: Sequential (all work in single phase)

### Overview
Implement the complete Slack bot proof-of-concept with all components needed to receive DMs and respond with "Pong".

### Changes Required:

#### 1. Add Slack SDK Dependencies
**File**: `go.mod`
**Changes**: Add Slack SDK dependencies

Run:
```bash
go get github.com/slack-go/slack
go get github.com/joho/godotenv
go mod tidy
```

Expected result: `go.mod` and `go.sum` updated with Slack dependencies.

#### 2. Create Main Entry Point
**File**: `cmd/slack-test/main.go` (new file)
**Changes**: Create command entry point with Slack client initialization and event handling

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Initialize Slack client
	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")

	if appToken == "" || botToken == "" {
		log.Fatal("SLACK_APP_TOKEN and SLACK_BOT_TOKEN must be set")
	}

	api := slack.New(
		botToken,
		slack.OptionDebug(true),
		slack.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(
		api,
		socketmode.OptionDebug(true),
	)

	// Handle events
	go handleEvents(client, api)

	log.Println("Slack bot starting...")
	if err := client.Run(); err != nil {
		log.Fatal(err)
	}
}

func handleEvents(client *socketmode.Client, api *slack.Client) {
	for evt := range client.Events {
		switch evt.Type {
		case socketmode.EventTypeEventsAPI:
			handleEventAPI(evt, client, api)
		}
	}
}

func handleEventAPI(evt socketmode.Event, client *socketmode.Client, api *slack.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	client.Ack(*evt.Request)

	switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		// Only respond to DMs - keep it lean!
		if ev.ChannelType == "im" && ev.BotID == "" {
			log.Printf("DM received: user=%s, text=%s, channel=%s, ts=%s",
				ev.User, ev.Text, ev.Channel, ev.TimeStamp)

			// Respond with Pong
			_, _, err := api.PostMessage(ev.Channel,
				slack.MsgOptionText("Pong! 🏓", false))
			if err != nil {
				log.Printf("Error posting message: %v", err)
			} else {
				log.Println("Response sent!")
			}
		}
	}
}
```

#### 3. Create Message Parser (Optional/Placeholder)
**File**: `internal/slack/parser.go` (new file)
**Changes**: Add basic message parsing structure for future use

```go
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
```

### Success Criteria:

#### Automated Verification:
- [x] Dependencies install successfully: `go mod download`
- [x] Code compiles: `go build ./cmd/slack-test`
- [x] No import errors or syntax issues

#### Manual Verification:
- [ ] Copy `.env.dist` to `.env` and add real Slack tokens
- [ ] Start bot: `go run cmd/slack-test/main.go`
- [ ] Bot shows "Slack bot starting..." in terminal
- [ ] Bot appears as online/active in Slack workspace
- [ ] Send DM to bot in Slack workspace
- [ ] Bot responds with "Pong! 🏓" message
- [ ] Terminal shows log: `DM received: user=U..., text=..., channel=D..., ts=...`
- [ ] Terminal shows log: `Response sent!`
- [ ] Send multiple DMs → bot responds to each one
- [ ] Send message in channel → bot ignores it (only DMs)
- [ ] Bot ignores its own messages (no infinite loop)

**Implementation Note**: This phase completes the entire PoC. After automated verification passes, perform manual testing in Slack workspace to verify the bot works end-to-end.

---

## Testing Strategy

### Manual Testing Steps:
1. **Setup**:
   - Ensure `.env` file exists with valid `SLACK_BOT_TOKEN` and `SLACK_APP_TOKEN`
   - Verify bot has `app_mentions:read`, `chat:write`, `im:history`, `im:read`, `im:write` scopes in Slack app settings

2. **Start Bot**:
   ```bash
   go run cmd/slack-test/main.go
   ```
   - Verify terminal shows: "Slack bot starting..."
   - Check for any connection errors

3. **Test DM Interaction**:
   - Open Slack workspace
   - Find bot in Apps section
   - Send DM: "Hello bot"
   - Expected: Bot responds with "Pong! 🏓"
   - Expected: Terminal logs show message details

4. **Test Message Filtering**:
   - Send message in public channel (mention bot if needed)
   - Expected: Bot ignores channel messages
   - Verify no response, no terminal logs for channel messages

5. **Test Multiple Messages**:
   - Send 3-5 DMs in quick succession
   - Expected: Bot responds to each with "Pong! 🏓"
   - Verify each message logged to terminal

6. **Test Bot Message Filtering**:
   - Bot should not respond to its own messages
   - Verify no infinite loop of Pong messages

### What Success Looks Like:
```
Terminal output:
--------------
Slack bot starting...
DM received: user=U12345, text=Hello bot, channel=D9876, ts=1234567890.123
Response sent!
DM received: user=U12345, text=Test message 2, channel=D9876, ts=1234567891.456
Response sent!
```

Slack workspace:
```
You: Hello bot
Bot: Pong! 🏓
```

## Performance Considerations

None - this is a minimal PoC for testing only.

## Migration Notes

N/A - new implementation, no existing data to migrate.

## References

- Original ticket: `thoughts/shared/tickets/GO-001-mock-slack-layer/ticket.md`
- Slack Bolt Go SDK: https://github.com/slack-go/slack
- Socket Mode docs: https://api.slack.com/apis/connections/socket
- Environment config: `.env.dist`
