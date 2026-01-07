package openai

import (
	"context"
	"sync"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/conversations"
	"github.com/openai/openai-go/v3/option"
)

var (
	client openai.Client
	mu     sync.RWMutex
)

func Connect(apiKey string) error {
	mu.Lock()
	defer mu.Unlock()

	if apiKey == "" {
		return &Error{
			Type:    ErrTypeAuth,
			Message: "API key cannot be empty",
		}
	}

	client = openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	return nil
}

func Close() error {
	mu.Lock()
	defer mu.Unlock()
	client = openai.Client{}
	return nil
}

func getClient() (openai.Client, error) {
	mu.RLock()
	defer mu.RUnlock()

	if client.Options == nil {
		return openai.Client{}, &Error{
			Type:    ErrTypeAuth,
			Message: "client not initialized, call Connect first",
		}
	}

	return client, nil
}

var lastPersonality = make(map[string]string)
var personalityMutex sync.RWMutex

func shouldIncludePersonality(convID, personality string) bool {
	if convID == "" {
		return true
	}

	personalityMutex.RLock()
	last := lastPersonality[convID]
	personalityMutex.RUnlock()

	if last != personality {
		personalityMutex.Lock()
		lastPersonality[convID] = personality
		personalityMutex.Unlock()
		return true
	}

	return false
}

func createConversation(ctx context.Context) (string, error) {
	c, err := getClient()
	if err != nil {
		return "", err
	}

	conv, err := c.Conversations.New(ctx, conversations.ConversationNewParams{})
	if err != nil {
		return "", &Error{
			Type:    ErrTypeAPIError,
			Message: "failed to create conversation",
			Err:     err,
		}
	}

	return conv.ID, nil
}
