// Package slack handles communication between the slack api and the orchestrator
package slack

import (
	"bob/internal/logger"
	"bob/internal/orchestrator"
	"os"

	slackAPI "github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

func StartSlack(orch *orchestrator.Orchestrator) {
	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")

	if appToken == "" || botToken == "" {
		logger.Fatal("SLACK_APP_TOKEN and SLACK_BOT_TOKEN must be set")
	}

	api := slackAPI.New(
		botToken,
		slackAPI.OptionDebug(false),
		slackAPI.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(api)

	// Start event handler
	go handleEvents(client, api, orch)

	if err := client.Run(); err != nil {
		logger.Fatal(err)
	}
}
