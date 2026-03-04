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

	// For response-ID chains (resp_...) the caller must use the new response ID as the
	// next conversation pointer — the old ID is no longer the tip of the chain.
	returnConvID := convID
	if strings.HasPrefix(convID, "resp_") {
		returnConvID = resp.ID
	}

	return ai.NewResponse(
		data,
		returnConvID,
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
		Model:       shared.ResponsesModel(config.Model),
		Temperature: param.NewOpt(float64(config.Temperature)),
		TopP:        param.NewOpt(float64(config.TopP)),
	}

	if config.MaxTokens > 0 {
		params.MaxOutputTokens = param.NewOpt(int64(config.MaxTokens))
	}

	// Response IDs (resp_...) use the previous_response_id chain — the Conversation and
	// PreviousResponseID fields are mutually exclusive in the API.
	if strings.HasPrefix(convID, "resp_") {
		params.PreviousResponseID = param.NewOpt(convID)
		params.Instructions = param.NewOpt(personality) // no caching — ID changes every call
	} else {
		params.Conversation = responses.ResponseNewParamsConversationUnion{
			OfString: param.NewOpt(convID),
		}
		if shouldIncludePersonality(convID, personality) {
			params.Instructions = param.NewOpt(personality)
		}
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

// sendBranchedMessage makes an AI call using previous_response_id so the model sees
// full conversation context. Returns resp.ID as ConversationID — store it to continue
// the branch, or discard it to abandon. The original thread is always unaffected.
//
// Use via ai.BranchFromResponse(responseID) — see thoughts/shared/patterns/ai-response-branching.md
func sendBranchedMessage(
	ctx context.Context,
	previousResponseID string,
	userPrompt string,
	personality string,
	schemaBuilder *ai.SchemaBuilder,
) (*ai.Response, error) {
	schema, err := buildSchemaWithCache(schemaBuilder)
	if err != nil {
		return nil, err
	}
	config := defaultConfig()
	// previousResponseID starts with "resp_" so sendMessage automatically uses
	// PreviousResponseID and sendWithRetry handles retries as normal.
	resp, err := sendWithRetry(ctx, previousResponseID, userPrompt, personality, schema, config)
	if err != nil {
		return nil, err
	}
	data, err := parseResponseToMap(resp)
	if err != nil {
		return nil, err
	}
	// Return resp.ID as ConversationID — callers can store and continue from this branch,
	// or simply discard it. SendMessage's returnConvID logic also handles this automatically
	// when going through the normal path.
	return ai.NewResponse(
		data,
		resp.ID,
		resp.ID,
		string(resp.Model),
		string(resp.Status),
		int(resp.Usage.TotalTokens),
	), nil
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
