package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/slack-go/slack/slackevents"
	"github.com/sashabaranov/go-openai"
)

var openaiClient *openai.Client

func main() {
	// Load .env file
	godotenv.Load()

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
