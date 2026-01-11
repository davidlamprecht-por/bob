// Package ai is responsible for the communication to the ai provder
package ai

import (
	"fmt"

	"bob/internal/config"
)

// defaultProvider is the global default provider instance
// It will be set by the provider package (e.g., openai) during its initialization
var defaultProvider Provider

// RegisterDefaultProvider sets the default provider
// This is called by provider packages (e.g., openai) during their initialization
func RegisterDefaultProvider(provider Provider) {
	defaultProvider = provider
}

// Init initializes the AI layer with the default provider
// This should be called during application startup, after provider packages are imported
func Init() error {
	if defaultProvider == nil {
		return fmt.Errorf("ai: no default provider registered. Import bob/internal/ai/openai to register OpenAI")
	}

	// Get OpenAI API key from config
	apiKey := config.Current.OpenAIAPIKey
	if apiKey == "" {
		return fmt.Errorf("ai: OPENAI_API_KEY not configured")
	}

	// Connect to the provider
	if err := defaultProvider.Connect(apiKey); err != nil {
		return fmt.Errorf("ai: failed to connect to provider: %w", err)
	}

	// Create AI client with the default provider
	client := NewAIClient(defaultProvider)
	SetGlobalAIClient(client)

	return nil
}
