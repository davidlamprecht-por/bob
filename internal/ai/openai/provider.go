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
	// Check for BranchOption — branch calls get full context but don't advance the chain
	for _, opt := range opts {
		if b, ok := opt.(ai.BranchOption); ok {
			return sendBranchedMessage(ctx, b.ResponseID, userPrompt, personality, schemaBuilder)
		}
	}
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
