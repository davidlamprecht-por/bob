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

// BranchOption instructs the provider to branch off an existing response rather than
// continuing a conversation chain directly. The provider sees the full conversation
// history up to that response. The returned ConversationID is the new branch tip and
// can be stored to continue the branch or discarded if only a one-shot query is needed.
// The original conversation chain is completely unaffected.
type BranchOption struct {
	ResponseID string
}

func (BranchOption) Apply(_ any) {} // handled via type assertion in providers

// BranchFromResponse returns an Option that gives the AI full conversation context
// by branching off an existing response. The branch starts from the given response ID
// and the returned ConversationID is the new branch tip — store it to continue the
// branch across multiple turns, or discard it if only a one-shot query is needed.
// The original conversation chain is never affected.
func BranchFromResponse(responseID string) Option {
	return BranchOption{ResponseID: responseID}
}
