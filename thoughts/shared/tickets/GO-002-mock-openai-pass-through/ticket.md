---
ticket_id: GO-002
title: Mock OpenAI Layer & Slack-to-OpenAI Pass-through
type: Story
state: New
priority: High
created_date: 2025-12-29
---

# GO-002: Mock OpenAI Layer & Slack-to-OpenAI Pass-through

## Description
Create two minimalistic cmd-driven scripts:
1. Test OpenAI integration standalone
2. Full pass-through: Slack message → OpenAI → Slack response

This validates both the OpenAI SDK integration and the end-to-end flow from Slack to AI and back.

## Goal
Build simple cmd-driven scripts that:
- **Script 1**: Test OpenAI API standalone (hardcoded prompt)
- **Script 2**: Connect Slack → OpenAI → Slack (full pass-through)
- Validate OpenAI SDK works correctly
- Prove the core data flow works
- Keep it absolutely minimal (no database, no sessions, no complex logic)

## Tasks
- [ ] Create standalone OpenAI test script
- [ ] Test OpenAI Chat Completions API
- [ ] Create Slack-to-OpenAI pass-through script
- [ ] Handle message flow: Slack → OpenAI → Slack
- [ ] Log each step of the flow
- [ ] Test manually end-to-end
- [ ] Document environment variables

## Script 1: Standalone OpenAI Test (cmd/openai-test/main.go)

### Implementation
```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/sashabaranov/go-openai"
)

func main() {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        log.Fatal("OPENAI_API_KEY must be set")
    }

    client := openai.NewClient(apiKey)

    // Test simple completion
    log.Println("Sending request to OpenAI...")

    resp, err := client.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4oMini,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleSystem,
                    Content: "You are a helpful assistant.",
                },
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: "Say hello in a creative way!",
                },
            },
            Temperature: 0.7,
            MaxTokens:   100,
        },
    )

    if err != nil {
        log.Fatalf("OpenAI error: %v", err)
    }

    log.Println("OpenAI Response:")
    fmt.Println(resp.Choices[0].Message.Content)
    fmt.Printf("\nTokens used: %d\n", resp.Usage.TotalTokens)
}
```

### Testing Script 1
```bash
export OPENAI_API_KEY=sk-...
go run cmd/openai-test/main.go
```

Expected output:
```
Sending request to OpenAI...
OpenAI Response:
[Creative hello message from GPT]

Tokens used: 45
```

## Script 2: Slack-to-OpenAI Pass-through (cmd/slack-openai-pass-through/main.go)

### Implementation
```go
package main

import (
    "context"
    "log"
    "os"
    "strings"

    "github.com/slack-go/slack"
    "github.com/slack-go/slack/socketmode"
    "github.com/slack-go/slack/slackevents"
    "github.com/sashabaranov/go-openai"
)

var openaiClient *openai.Client

func main() {
    // Initialize OpenAI
    openaiKey := os.Getenv("OPENAI_API_KEY")
    if openaiKey == "" {
        log.Fatal("OPENAI_API_KEY must be set")
    }
    openaiClient = openai.NewClient(openaiKey)

    // Initialize Slack
    appToken := os.Getenv("SLACK_APP_TOKEN")
    botToken := os.Getenv("SLACK_BOT_TOKEN")

    if appToken == "" || botToken == "" {
        log.Fatal("SLACK_APP_TOKEN and SLACK_BOT_TOKEN must be set")
    }

    api := slack.New(
        botToken,
        slack.OptionDebug(false),  // Less verbose for pass-through
        slack.OptionAppLevelToken(appToken),
    )

    client := socketmode.New(api)

    go handleEvents(client, api)

    log.Println("🤖 Slack-to-OpenAI pass-through bot starting...")
    log.Println("Send a DM to the bot to test the pass-through!")

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
        // DMs only - keep it lean!
        if ev.ChannelType == "im" && ev.BotID == "" {
            handleMessage(api, ev.Channel, ev.Text, ev.TimeStamp)
        }
    }
}

func handleMessage(api *slack.Client, channel, text, msgTS string) {
    cleanText := strings.TrimSpace(text)
    log.Printf("📨 Received: %s", cleanText)

    // Send to OpenAI
    log.Println("🤔 Asking OpenAI...")

    resp, err := openaiClient.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4oMini,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleSystem,
                    Content: "You are Bob, a helpful assistant. Be concise and friendly.",
                },
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: cleanText,
                },
            },
            Temperature: 0.8,
            MaxTokens:   500,
        },
    )

    if err != nil {
        log.Printf("❌ OpenAI error: %v", err)
        api.PostMessage(channel,
            slack.MsgOptionText("Sorry, I had trouble thinking 🤔", false))
        return
    }

    aiResponse := resp.Choices[0].Message.Content
    log.Printf("🤖 OpenAI responded: %s", aiResponse)

    // Send back to Slack (simple DM reply, no threading)
    _, _, err = api.PostMessage(channel,
        slack.MsgOptionText(aiResponse, false))

    if err != nil {
        log.Printf("❌ Slack error: %v", err)
    } else {
        log.Println("✅ Response sent to Slack")
    }
}
```

### Testing Script 2
```bash
export OPENAI_API_KEY=sk-...
export SLACK_APP_TOKEN=xapp-...
export SLACK_BOT_TOKEN=xoxb-...

go run cmd/slack-openai-pass-through/main.go
```

**Then in Slack**:
1. Send a DM to the bot: "What's the weather like?"
2. Watch the logs:
   ```
   📨 Received: What's the weather like?
   🤔 Asking OpenAI...
   🤖 OpenAI responded: I don't have real-time data...
   ✅ Response sent to Slack
   ```
3. See the AI response in Slack!

## Required Environment Variables

Copy `.env.dist` to `.env` and fill in your values:
```bash
cp .env.dist .env
# Edit .env with your actual tokens
```

Required variables:
```
# For Script 1
OPENAI_API_KEY=sk-...

# For Script 2 (both)
OPENAI_API_KEY=sk-...
SLACK_BOT_TOKEN=xoxb-...
SLACK_APP_TOKEN=xapp-...
```

## Required Dependencies
```bash
go get github.com/sashabaranov/go-openai
go get github.com/slack-go/slack
go get github.com/joho/godotenv
```

## Acceptance Criteria

### Script 1 (openai-test)
- [ ] Connects to OpenAI successfully
- [ ] Sends hardcoded prompt
- [ ] Receives and displays response
- [ ] Shows token usage
- [ ] Handles errors gracefully

### Script 2 (slack-openai-pass-through)
- [ ] Connects to both Slack and OpenAI
- [ ] Receives Slack DM messages only
- [ ] Sends message to OpenAI
- [ ] Receives OpenAI response
- [ ] Sends response back to Slack DM
- [ ] Clean logging shows each step
- [ ] Handles errors at each layer

## Testing Flow

1. **Test OpenAI standalone**:
   ```bash
   go run cmd/openai-test/main.go
   ```
   Verify: Gets response from OpenAI

2. **Test Slack-to-OpenAI pass-through**:
   ```bash
   go run cmd/slack-openai-pass-through/main.go
   ```
   - Send message in Slack
   - Watch logs show: Received → Asking OpenAI → Response sent
   - See AI response appear in Slack

3. **Test edge cases**:
   - Long message
   - Quick successive messages
   - Error handling (invalid API key, network error)

## Notes
- Keep both scripts **extremely simple**
- **DMs only** - no mentions, no channels, no threads
- No database, no session management, no state
- Script 2 is stateless: each message is independent
- Focus: prove the integration works, log everything
- Error handling: just log and send error message to Slack
- Simple DM replies, no threading complexity

## Success Metric
**Script 1**: Run it, see OpenAI response in terminal.
**Script 2**: Send a message in Slack, see it passed to OpenAI, get intelligent response back. The whole loop works!

## Why This Matters
This ticket proves:
1. OpenAI SDK integration works ✅
2. Slack SDK integration works ✅
3. The core data flow works ✅
4. We can build the real Bob on this foundation ✅

Everything else (database, sessions, personalities, tools) builds on top of this proven foundation.

## Reference
- OpenAI Go SDK: https://github.com/sashabaranov/go-openai
- Slack Bolt Go: https://github.com/slack-go/slack
- OpenAI Chat Completions: https://platform.openai.com/docs/api-reference/chat
