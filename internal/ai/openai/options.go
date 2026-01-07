package openai

type Option func(*RequestConfig)

func WithModel(model string) Option {
	return func(c *RequestConfig) {
		c.Model = model
	}
}

func WithTemperature(temp float32) Option {
	return func(c *RequestConfig) {
		c.Temperature = temp
	}
}

func WithMaxTokens(tokens int) Option {
	return func(c *RequestConfig) {
		c.MaxTokens = tokens
	}
}

func WithTopP(topP float32) Option {
	return func(c *RequestConfig) {
		c.TopP = topP
	}
}

func WithFrequencyPenalty(penalty float32) Option {
	return func(c *RequestConfig) {
		c.FrequencyPenalty = penalty
	}
}

func WithPresencePenalty(penalty float32) Option {
	return func(c *RequestConfig) {
		c.PresencePenalty = penalty
	}
}

func WithStopSequences(sequences []string) Option {
	return func(c *RequestConfig) {
		c.StopSequences = sequences
	}
}
