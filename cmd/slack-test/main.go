package main

import (
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
