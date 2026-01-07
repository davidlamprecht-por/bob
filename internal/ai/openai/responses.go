// Package openai provides OpenAI Responses API integration with structured output support.
package openai

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"bob/internal/ai"

	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

// SendMessage sends a message to OpenAI with structured output support
// This is the primary function the AI Layer will call
func SendMessage(
	ctx context.Context,
	conversationID *string,
	userPrompt string,
	personality string,
	schemaBuilder *ai.SchemaBuilder,
	opts ...Option,
) (*ai.Response, error) {
	config := defaultConfig()
	for _, opt := range opts {
		opt(&config)
	}

	schema, err := buildSchemaWithCache(schemaBuilder)
	if err != nil {
		return nil, err
	}

	convID, err := resolveConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	resp, err := sendWithRetry(ctx, convID, userPrompt, personality, schema, config)
	if err != nil {
		return nil, err
	}

	data, err := parseResponseToMap(resp)
	if err != nil {
		return nil, err
	}

	return ai.NewResponse(
		data,
		convID,
		resp.ID,
		string(resp.Model),
		string(resp.Status),
		int(resp.Usage.TotalTokens),
	), nil
}

func resolveConversationID(ctx context.Context, id *string) (string, error) {
	if id == nil {
		return createConversation(ctx)
	}
	return *id, nil
}

func sendWithRetry(
	ctx context.Context,
	convID string,
	userPrompt string,
	personality string,
	schema map[string]interface{},
	config RequestConfig,
) (*responses.Response, error) {
	maxRetries := 3
	backoff := time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := sendMessage(ctx, convID, userPrompt, personality, schema, config)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	return nil, &Error{
		Type:    ErrTypeAPIError,
		Message: "max retries exceeded",
		Err:     lastErr,
	}
}

func sendMessage(
	ctx context.Context,
	convID string,
	userPrompt string,
	personality string,
	schema map[string]interface{},
	config RequestConfig,
) (*responses.Response, error) {
	c, err := getClient()
	if err != nil {
		return nil, err
	}

	params := responses.ResponseNewParams{
		Conversation: responses.ResponseNewParamsConversationUnion{
			OfString: param.NewOpt(convID),
		},
		Model:       shared.ResponsesModel(config.Model),
		Temperature: param.NewOpt(float64(config.Temperature)),
		TopP:        param.NewOpt(float64(config.TopP)),
	}

	if config.MaxTokens > 0 {
		params.MaxOutputTokens = param.NewOpt(int64(config.MaxTokens))
	}

	if shouldIncludePersonality(convID, personality) {
		params.Instructions = param.NewOpt(personality)
	}

	input := responses.ResponseNewParamsInputUnion{
		OfInputItemList: responses.ResponseInputParam{
			responses.ResponseInputItemParamOfMessage(userPrompt, responses.EasyInputMessageRoleUser),
		},
	}
	params.Input = input

	params.Text = responses.ResponseTextConfigParam{
		Format: responses.ResponseFormatTextConfigParamOfJSONSchema("response", schema),
	}

	resp, err := c.Responses.New(ctx, params)
	if err != nil {
		return nil, wrapOpenAIError(err)
	}

	return resp, nil
}

func parseResponseToMap(resp *responses.Response) (map[string]any, error) {
	if len(resp.Output) == 0 {
		return nil, &Error{
			Type:    ErrTypeResponseParsing,
			Message: "no output in response",
		}
	}

	content := resp.OutputText()
	if content == "" {
		return nil, &Error{
			Type:    ErrTypeResponseParsing,
			Message: "no text content in response",
		}
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil, &Error{
			Type:    ErrTypeResponseParsing,
			Message: "failed to unmarshal response",
			Err:     err,
		}
	}

	return data, nil
}

func wrapOpenAIError(err error) error {
	errStr := err.Error()

	if containsAny(errStr, []string{"unauthorized", "invalid_api_key", "authentication"}) {
		return &Error{
			Type:    ErrTypeAuth,
			Message: "authentication failed",
			Err:     err,
		}
	}

	if containsAny(errStr, []string{"rate_limit", "too_many_requests"}) {
		return &Error{
			Type:    ErrTypeRateLimit,
			Message: "rate limit exceeded",
			Err:     err,
		}
	}

	if containsAny(errStr, []string{"invalid_request", "bad_request"}) {
		return &Error{
			Type:    ErrTypeInvalidRequest,
			Message: "invalid request",
			Err:     err,
		}
	}

	return &Error{
		Type:    ErrTypeAPIError,
		Message: "API error",
		Err:     err,
	}
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(strings.ToLower(s), strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
