package openai

type Response struct {
	Data           any
	ConversationID string
	ResponseID     string
	TokensUsed     int
	Model          string
	FinishReason   string
}

type RequestConfig struct {
	Model            string
	Temperature      float32
	MaxTokens        int
	TopP             float32
	FrequencyPenalty float32
	PresencePenalty  float32
	StopSequences    []string
}

func defaultConfig() RequestConfig {
	return RequestConfig{
		Model:       "gpt-4o-mini",
		Temperature: 0.7,
		MaxTokens:   4096,
		TopP:        1.0,
	}
}
