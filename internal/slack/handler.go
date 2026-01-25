package slack

import (
	"bob/internal/logger"
	"bob/internal/orchestrator"
	"bob/internal/orchestrator/core"

	slackAPI "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

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
			handleMessage(api, orch, ev)
		}
	}
}

func handleMessage(api *slackAPI.Client, orch *orchestrator.Orchestrator, ev *slackevents.MessageEvent) {
	msg := ParseMessage(ev)
	threadid := msg.ThreadID.GetExternal()
	logger.Debugf("📨 Message from %s in thread %s: %s", ev.User, threadid, ev.Text)

	responder := func(response core.Response) error {
		threadTS := msg.ThreadID.GetExternal()
		logger.Debugf("🔧 Sending response with thread_ts=%s to channel=%s", threadTS, msg.Channel)

		channel, timestamp, err := api.PostMessage(msg.Channel,
			slackAPI.MsgOptionText(response.Message, false),
			slackAPI.MsgOptionTS(threadTS)) // Reply in the thread
		if err != nil {
			logger.Errorf("❌ Failed to send response to Slack: %v", err)
			return err
		}
		logger.Debugf("✅ Response sent - channel=%s, timestamp=%s, thread_ts=%s", channel, timestamp, threadTS)
		logger.Debugf("✅ Response sent in thread %s: %s", threadid, response.Message)
		return nil
	}

	// Handle message through orchestrator
	if err := orch.HandleUserMessage(msg, responder); err != nil {
		logger.Errorf("❌ Orchestrator error: %v", err)
		responder(core.Response{Message: "Sorry, I encountered an error processing your request."})
	}
}

