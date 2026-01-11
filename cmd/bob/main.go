package main

import (
	"log"
	"os"
	"time"

	"bob/internal/ai"
	_ "bob/internal/ai/openai" // Import to register OpenAI provider
	"bob/internal/config"
	"bob/internal/database"
	"bob/internal/orchestrator"
	"bob/internal/orchestrator/core"
	"bob/internal/slack"

	"github.com/joho/godotenv"
	slackAPI "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	log.Println("🤖 Bob starting...")

	// Load .env file
	_ = godotenv.Load()

	// Initialize configuration
	config.Init()
	log.Println("✓ Configuration loaded")

	// Connect to database
	log.Println("Connecting to database...")
	connStr := config.Current.DBConnectionString()
	if err := database.Connect(connStr); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	log.Println("✓ Database connected")

	// Initialize AI layer
	log.Println("Initializing AI layer...")
	if err := ai.Init(); err != nil {
		log.Fatalf("Failed to initialize AI layer: %v", err)
	}
	log.Println("✓ AI layer initialized")

	// Initialize orchestrator
	orch := &orchestrator.Orchestrator{}
	orch.Init()
	log.Println("✓ Orchestrator initialized")

	// Initialize Slack
	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")

	if appToken == "" || botToken == "" {
		log.Fatal("SLACK_APP_TOKEN and SLACK_BOT_TOKEN must be set")
	}

	api := slackAPI.New(
		botToken,
		slackAPI.OptionDebug(false),
		slackAPI.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(api)

	// Start event handler
	go handleEvents(client, api, orch)

	log.Println("✅ Bob is ready! Send a DM to test.")
	if err := client.Run(); err != nil {
		log.Fatal(err)
	}
}

func handleEvents(client *socketmode.Client, api *slackAPI.Client, orch *orchestrator.Orchestrator) {
	for evt := range client.Events {
		switch evt.Type {
		case socketmode.EventTypeEventsAPI:
			handleEventAPI(evt, client, api, orch)
		}
	}
}

func handleEventAPI(evt socketmode.Event, client *socketmode.Client, api *slackAPI.Client, orch *orchestrator.Orchestrator) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	client.Ack(*evt.Request)

	switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		// Only respond to DMs from real users (not bots)
		if ev.ChannelType == "im" && ev.BotID == "" {
			// Use ThreadTimeStamp if message is in a thread, otherwise use TimeStamp to start a thread
			threadTS := ev.ThreadTimeStamp
			if threadTS == "" {
				threadTS = ev.TimeStamp // Start a new thread with this message
			}
			handleMessage(api, orch, ev.User, ev.Channel, ev.Text, ev.TimeStamp, threadTS)
		}
	}
}

func handleMessage(api *slackAPI.Client, orch *orchestrator.Orchestrator, userID, channel, text, ts, threadTS string) {
	log.Printf("📨 Message from %s in thread %s: %s", userID, threadTS, text)

	// Parse Slack message into core.Message
	// Use threadTS as the thread identifier for our system
	timestamp, err := parseSlackTimestamp(ts)
	if err != nil {
		log.Printf("❌ Failed to parse timestamp: %v", err)
		return
	}

	// Use channel:threadTS as the unique thread identifier
	threadID := channel + ":" + threadTS
	message, err := slack.ParseMessage(userID, threadID, text, timestamp)
	if err != nil {
		log.Printf("❌ Failed to parse message: %v", err)
		return
	}

	// Create responder function that sends messages back to Slack in the same thread
	responder := func(response core.Response) error {
		_, _, err := api.PostMessage(channel,
			slackAPI.MsgOptionText(response.Message, false),
			slackAPI.MsgOptionTS(threadTS)) // Reply in the thread
		if err != nil {
			log.Printf("❌ Failed to send response to Slack: %v", err)
			return err
		}
		log.Printf("✅ Response sent in thread %s: %s", threadTS, response.Message)
		return nil
	}

	// Handle message through orchestrator
	if err := orch.HandleUserMessage(message, responder); err != nil {
		log.Printf("❌ Orchestrator error: %v", err)
		responder(core.Response{Message: "Sorry, I encountered an error processing your request."})
	}
}

// parseSlackTimestamp converts Slack timestamp (e.g., "1234567890.123456") to time.Time
func parseSlackTimestamp(ts string) (time.Time, error) {
	// Slack timestamps are Unix timestamps with microseconds
	// For simplicity, we'll just use the current time as a stub
	// TODO: Parse the actual timestamp properly
	return time.Now(), nil
}
