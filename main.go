package main

import (
	"bob/internal/ai"
	_ "bob/internal/ai/openai" // Import to register OpenAI provider
	"bob/internal/config"
	"bob/internal/database"
	"bob/internal/orchestrator"
	"bob/internal/slack"
	"bob/internal/logger"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	// Initialize configuration
	config.Init()

	// Initialize logger with configured level
	logger.InitWithString(config.Current.LogLevel)

	// Now start logging with the correct configured level
	logger.Info("🤖 Bob starting...")
	logger.Debug("✓ Configuration loaded")

	// Connect to database
	logger.Debug("Connecting to database...")
	connStr := config.Current.DBConnectionString()
	if err := database.Connect(connStr); err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	logger.Debug("✓ Database connected")

	// Initialize AI layer
	logger.Debug("Initializing AI layer...")
	if err := ai.Init(); err != nil {
		logger.Fatalf("Failed to initialize AI layer: %v", err)
	}
	logger.Debug("✓ AI layer initialized")

	// Initialize orchestrator
	orch := &orchestrator.Orchestrator{}
	orch.Init()
	logger.Debug("✓ Orchestrator initialized")

	// Initialize Slack
	slack.StartSlack(orch)

	logger.Info("✅ Bob is ready! Send a DM to test.")
}

