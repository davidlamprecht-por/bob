package openai

import (
	"context"

	"bob/internal/ai"
)

// provider implements the ai.Provider interface for OpenAI
type provider struct{}

// SendMessage implements ai.Provider.SendMessage
func (p *provider) SendMessage(
	ctx context.Context,
	conversationID *string,
	userPrompt string,
	personality string,
	schemaBuilder *ai.SchemaBuilder,
	opts ...ai.Option,
) (*ai.Response, error) {
	// Convert ai.Option to openai.Option
	// For now, we assume options are compatible
	// In the future, we might need a conversion mechanism
	return SendMessage(ctx, conversationID, userPrompt, personality, schemaBuilder)
}

// Connect implements ai.Provider.Connect
func (p *provider) Connect(apiKey string) error {
	return Connect(apiKey)
}

// Close implements ai.Provider.Close
func (p *provider) Close() error {
	return Close()
}

// init registers OpenAI as the default provider
func init() {
	ai.RegisterDefaultProvider(&provider{})
}
