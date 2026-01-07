package ai

import "context"

// Provider defines the interface that all AI providers must implement
// This allows the AI layer to work with different AI services (OpenAI, Anthropic, etc.)
type Provider interface {
	SendMessage(
		ctx context.Context,
		conversationID *string,
		userPrompt string,
		personality string,
		schemaBuilder *SchemaBuilder,
		opts ...Option,
	) (*Response, error)

	Connect(apiKey string) error
	Close() error
}

// Response is returned by all AI providers
type Response struct {
	data           map[string]any
	ConversationID string
	ResponseID     string
	TokensUsed     int
	Model          string
	FinishReason   string
}

func NewResponse(data map[string]any, conversationID, responseID, model, finishReason string, tokensUsed int) *Response {
	return &Response{
		data:           data,
		ConversationID: conversationID,
		ResponseID:     responseID,
		TokensUsed:     tokensUsed,
		Model:          model,
		FinishReason:   finishReason,
	}
}

func (r *Response) Data() *SchemaData {
	return &SchemaData{
		data: r.data,
	}
}

func (r *Response) Raw() map[string]any {
	return r.data
}

// Option allows providers to accept optional parameters
type Option interface {
	Apply(config any)
}
