package personalities

// SystemPrompt wraps a plain string as a one-off personality.
// Use for simple, non-reusable prompts. For repeatable ones, define a named personality.
func SystemPrompt(text string) string {
	return text
}
