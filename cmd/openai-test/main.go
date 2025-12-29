package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
)

func main() {
	// Load .env file
	godotenv.Load()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY must be set")
	}

	client := openai.NewClient(apiKey)

	// Test simple completion
	log.Println("Sending request to OpenAI...")

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are a helpful assistant.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Say hello in a creative way!",
				},
			},
			Temperature: 0.7,
			MaxTokens:   100,
		},
	)

	if err != nil {
		log.Fatalf("OpenAI error: %v", err)
	}

	log.Println("OpenAI Response:")
	fmt.Println(resp.Choices[0].Message.Content)
	fmt.Printf("\nTokens used: %d\n", resp.Usage.TotalTokens)
}
