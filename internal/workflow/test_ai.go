package workflow

import (
	"log"

	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"fmt"
)

// TestAI is a simple test workflow that sends a user message to AI and returns the response
func TestAI(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	log.Println("🔷 TestAI workflow: Entry")
	step := getInput(sourceAction, core.InputStep)
	log.Printf("🔷 TestAI workflow: step=%v", step)

	switch step {
	case StepInit:
		log.Println("🔷 TestAI workflow: StepInit - Getting user message")
		// Get the user's message from the last message in context
		messages := context.GetLastUserMessages()
		if len(messages) == 0 {
			log.Println("❌ TestAI workflow: No user messages found")
			return nil, fmt.Errorf("no user message found")
		}
		userMessage := messages[len(messages)-1].Message
		log.Printf("🔷 TestAI workflow: User message: %q", userMessage)

		// Create a simple schema for free-form text response
		schema := ai.NewSchema().
			AddString("message", ai.Required(), ai.Description("The AI assistant's response message"))

		// Send the message to AI with a simple system prompt
		systemPrompt := "You are a helpful test assistant."
		log.Println("🔷 TestAI workflow: Creating AI action")
		actions := askAI(userMessage, systemPrompt, "", schema, "")
		log.Printf("🔷 TestAI workflow: Returning %d actions", len(actions))

		return actions, nil

	default:
		log.Printf("🔷 TestAI workflow: Default step (handling AI response), step=%v", step)
		// Handle AI response
		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			log.Println("❌ TestAI workflow: No AI response in action input")
			return nil, fmt.Errorf("expected AI response but got none")
		}

		response, ok := aiResponse.(*ai.Response)
		if !ok {
			log.Printf("❌ TestAI workflow: Invalid AI response type: %T", aiResponse)
			return nil, fmt.Errorf("invalid AI response type")
		}

		log.Println("🔷 TestAI workflow: Extracting message from AI response")
		// Extract the message from the AI response
		message, err := response.Data().GetString("message")
		if err != nil {
			log.Printf("❌ TestAI workflow: Failed to extract message: %v", err)
			return nil, fmt.Errorf("failed to get message from AI response: %w", err)
		}
		log.Printf("🔷 TestAI workflow: AI message extracted: %q", message)

		// Send the AI's response back to the user
		log.Println("🔷 TestAI workflow: Creating ActionUserMessage")
		userMessageAction := core.NewAction(core.ActionUserMessage)
		if userMessageAction.Input == nil {
			userMessageAction.Input = make(map[core.InputType]any)
		}
		userMessageAction.Input[core.InputMessage] = message

		log.Println("🔷 TestAI workflow: Returning user message action")
		return []*core.Action{userMessageAction}, nil
	}
}
