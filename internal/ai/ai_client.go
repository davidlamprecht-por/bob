package ai

import (
	"context"
	"fmt"
)

// AIClient bridges the orchestrator and AI providers
// It handles conversation ID management and provider abstraction
type AIClient struct {
	provider Provider
}

// NewAIClient creates an AI client with the given provider
func NewAIClient(provider Provider) *AIClient {
	return &AIClient{
		provider: provider,
	}
}

// SendMessageFromOrchestrator sends messages from orchestrator to AI provider
// Parameters:
//   - ctx: Context for the request
//   - conversationID: Pointer to conversation ID (nil = create new)
//   - userPrompt: User's message
//   - personality: System instructions/personality
//   - schema: Schema builder for structured output
//   - opts: Provider-specific options
//
// Returns: AI response and error if any
func (c *AIClient) SendMessageFromOrchestrator(
	ctx context.Context,
	conversationID *string,
	userPrompt string,
	personality string,
	schema *SchemaBuilder,
	opts ...Option,
) (*Response, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("ai client: no provider configured")
	}

	// Call the provider's SendMessage
	response, err := c.provider.SendMessage(
		ctx,
		conversationID,
		userPrompt,
		personality,
		schema,
		opts...,
	)

	if err != nil {
		return nil, fmt.Errorf("ai client: provider error: %w", err)
	}

	return response, nil
}

// Global AI client instance
var globalAIClient *AIClient

// SetGlobalAIClient sets the global AI client instance
func SetGlobalAIClient(client *AIClient) {
	globalAIClient = client
}

// GetGlobalAIClient returns the global AI client instance
func GetGlobalAIClient() *AIClient {
	return globalAIClient
}

// SendMessage is a convenience function that uses the global AI client
func SendMessage(
	ctx context.Context,
	conversationID *string,
	userPrompt string,
	personality string,
	schema *SchemaBuilder,
	opts ...Option,
) (*Response, error) {
	if globalAIClient == nil {
		return nil, fmt.Errorf("ai: global AI client not initialized. Call ai.Init() first")
	}

	return globalAIClient.SendMessageFromOrchestrator(
		ctx,
		conversationID,
		userPrompt,
		personality,
		schema,
		opts...,
	)
}
