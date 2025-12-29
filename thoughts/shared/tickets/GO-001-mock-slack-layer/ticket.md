---
ticket_id: GO-001
title: Mock Slack Layer - Basic Message Handler
type: Story
state: New
priority: High
created_date: 2025-12-29
---

# GO-001: Mock Slack Layer - Basic Message Handler

## Description
Create a minimalistic command-line driven script to test the Slack integration layer. This is a proof-of-concept to validate that the Slack Bolt SDK for Go works correctly and we can receive/send messages.

## Goal
Build a simple cmd-driven Slack bot that:
- Connects to Slack via Socket Mode (no webhook needed)
- Listens for DM messages only (keep it lean!)
- Logs received messages to stdout
- Responds with a simple "Pong" message
- Can be run from command line with `go run cmd/slack-test/main.go`

## Tasks
- [ ] Create cmd-driven entry point for Slack test
- [ ] Initialize Slack client with Bolt SDK
- [ ] Implement DM message event handler only
- [ ] Parse and log incoming message structure
- [ ] Send simple response back to Slack
- [ ] Test manually in Slack workspace with DMs

## Implementation Structure

### Entry Point (cmd/slack-test/main.go)
```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/slack-go/slack"
    "github.com/slack-go/slack/socketmode"
)

func main() {
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

### Message Parser (internal/slack/parser.go)
```go
package slack

type ParsedMessage struct {
    UserID      string
    ThreadID    string
    Message     string
    Channel     string
    Timestamp   string
    IsThread    bool
}

func ParseEvent(event interface{}) (*ParsedMessage, error) {
    // Extract common fields from Slack events
    // Return structured message data
    return &ParsedMessage{}, nil
}
```

## Testing
**Manual Tests** (in Slack workspace):
- [ ] Start bot: `go run cmd/slack-test/main.go`
- [ ] Bot shows as online in Slack
- [ ] Send DM to bot → bot responds with "Pong! 🏓"
- [ ] Send multiple DMs → bot responds to each
- [ ] Check stdout logs show parsed message structure (user, text, channel, timestamp)

## Required Environment Variables
```
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_APP_TOKEN=xapp-your-app-token
```

Copy `.env.dist` to `.env` and fill in your values:
```bash
cp .env.dist .env
# Edit .env with your actual tokens
```

## Required Dependencies
```bash
go get github.com/slack-go/slack
go get github.com/joho/godotenv  # for loading .env files
```

## Acceptance Criteria
- [ ] Bot connects to Slack successfully via Socket Mode
- [ ] Bot receives and logs DM messages only
- [ ] Bot responds with "Pong! 🏓" to each DM
- [ ] Bot ignores non-DM messages (channels, threads)
- [ ] Bot ignores its own messages (checks BotID)
- [ ] Clean stdout logging shows message structure
- [ ] Bot can be stopped gracefully (Ctrl+C)
- [ ] Code is simple, minimal, and cmd-driven

## Notes
- This is a **proof-of-concept** - keep it minimal!
- **DMs only** - no mentions, no channels, no threads
- Use Socket Mode (no ngrok or webhooks needed)
- Focus on validating the Slack SDK integration works
- Logging is key - we want to see the message structure
- No database, no complex logic - just receive/respond
- Filter out bot's own messages with `ev.BotID == ""`

## Success Metric
Run the command, see it connect, send a message in Slack, see the log output, and get a response back. That's it!

## Reference
- Slack Bolt Go: https://github.com/slack-go/slack
- Socket Mode docs: https://api.slack.com/apis/connections/socket
