package orchestrator

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"bob/internal/workflow"
	"context"
	"fmt"
)

// TODO: Load these from config
const (
	confidenceThresholdNewWorkflow    = 0.6
	confidenceThresholdChangeWorkflow = 0.8
)

// TODO: Future Enhancement - Intent Clarification Flow
// When the Intent AI is unsure (confidence below threshold but not zero), it should be able to:
// 1. Ask the user a clarifying question (e.g., "Did you want to switch to querying tickets, or continue testing?")
// 2. Interject the main orchestrator flow to wait for user response
// 3. Re-analyze intent with the clarifying response
//
// This requires:
// - New data structures to track "waiting for intent clarification" state
// - New action type (e.g., ActionIntentClarification) to pause orchestrator flow
// - Ability to resume intent analysis after receiving clarification
// - Storage of "pending intent options" that we're asking the user to choose between
//
// Benefits: Higher confidence in user intent, better UX for ambiguous requests
func AnalyzeIntent(message *core.Message, ctx *core.ConversationContext) core.Intent {
	aiResponse, err := callIntentAI(message, ctx)
	if err != nil {
		return core.Intent{
			IntentType:    core.IntentAskQuestion,
			Confidence:    0.0,
			Reasoning:     fmt.Sprintf("AI call failed: %v", err),
			MessageToUser: nil,
		}
	}

	currentWorkflow := ctx.GetCurrentWorkflow()
	suggestedWorkflow := aiResponse.WorkflowName
	suggestedStep := aiResponse.Step
	confidence := aiResponse.Confidence

	// No active workflow
	if currentWorkflow == nil {
		if confidence < confidenceThresholdNewWorkflow {
			return core.Intent{
				IntentType:    core.IntentAskQuestion,
				WorkflowName:  suggestedWorkflow,
				Confidence:    confidence,
				Reasoning:     fmt.Sprintf("Confidence too low (%.2f < %.2f): %s", confidence, confidenceThresholdNewWorkflow, aiResponse.Reasoning),
				MessageToUser: aiResponse.MessageToUser,
			}
		}
		return core.Intent{
			IntentType:    core.IntentNewWorkflow,
			WorkflowName:  suggestedWorkflow,
			Confidence:    confidence,
			Reasoning:     aiResponse.Reasoning,
			MessageToUser: aiResponse.MessageToUser,
		}
	}

	// Active workflow exists
	currentWorkflowName := currentWorkflow.GetWorkflowName()

	// AI suggests changing workflow
	if suggestedWorkflow != currentWorkflowName {
		if confidence < confidenceThresholdChangeWorkflow {
			// Not confident enough to change - route to current workflow as question
			return core.Intent{
				IntentType:    core.IntentAskQuestion,
				WorkflowName:  currentWorkflowName,
				Confidence:    confidence,
				Reasoning:     fmt.Sprintf("Uncertain input - staying with current workflow: %s", aiResponse.Reasoning),
				MessageToUser: aiResponse.MessageToUser,
			}
		}
		return core.Intent{
			IntentType:    core.IntentNewWorkflow,
			WorkflowName:  suggestedWorkflow,
			Confidence:    confidence,
			Reasoning:     aiResponse.Reasoning,
			MessageToUser: aiResponse.MessageToUser,
		}
	}

	// Same workflow - determine intent from step
	intentType := mapStepToIntentType(suggestedStep)
	return core.Intent{
		IntentType:    intentType,
		WorkflowName:  suggestedWorkflow,
		Confidence:    confidence,
		Reasoning:     aiResponse.Reasoning,
		MessageToUser: aiResponse.MessageToUser,
	}
}

func mapStepToIntentType(step string) core.IntentType {
	switch step {
	case workflow.StepInit:
		return core.IntentNewWorkflow
	case workflow.StepUserAsksQuestion:
		return core.IntentAskQuestion
	case workflow.StepUserAnsweringQuestion:
		return core.IntentAnswerQuestion
	default:
		return core.IntentAnswerQuestion
	}
}

type aiIntentResponse struct {
	WorkflowName  string
	Step          string
	Confidence    float64
	Reasoning     string
	MessageToUser *string
}

func callIntentAI(message *core.Message, ctx *core.ConversationContext) (*aiIntentResponse, error) {
	schema := buildIntentSchema()
	prompt := buildIntentPrompt(message, ctx)

	var opts []ai.Option
	if respID := ctx.GetLastResponseID(); respID != nil {
		opts = append(opts, ai.BranchFromResponse(*respID))
	}

	response, err := ai.SendMessage(
		context.Background(),
		nil,
		prompt,
		"You are an intent analyzer for Bob, a workflow-based assistant. Analyze user messages to determine the appropriate workflow and step.",
		schema,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	data := response.Data()
	workflowName, err := data.GetString("workflow_name")
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow_name: %w", err)
	}

	step, err := data.GetString("step")
	if err != nil {
		return nil, fmt.Errorf("failed to get step: %w", err)
	}

	confidence, err := data.GetFloat("confidence")
	if err != nil {
		return nil, fmt.Errorf("failed to get confidence: %w", err)
	}

	reasoning, err := data.GetString("reasoning")
	if err != nil {
		return nil, fmt.Errorf("failed to get reasoning: %w", err)
	}

	var messageToUser *string
	if data.IsSet("message_to_user") {
		msg, err := data.GetString("message_to_user")
		if err == nil && msg != "" {
			messageToUser = &msg
		}
	}

	return &aiIntentResponse{
		WorkflowName:  workflowName,
		Step:          step,
		Confidence:    confidence,
		Reasoning:     reasoning,
		MessageToUser: messageToUser,
	}, nil
}

func buildIntentSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("workflow_name", ai.Required(), ai.Description("The workflow that should handle this user message")).
		AddString("step", ai.Required(), ai.Description("The specific step to execute (use default steps when appropriate)")).
		AddFloat("confidence", ai.Required(), ai.Description("Confidence score from 0.0 to 1.0"), ai.Range(0.0, 1.0)).
		AddString("reasoning", ai.Required(), ai.Description("Brief explanation of why this workflow and step were chosen")).
		AddString("message_to_user", ai.Description("Optional message to send to user if you need to mention something. Use this only when necessary as not to confuse the user!"))
}

func buildIntentPrompt(message *core.Message, ctx *core.ConversationContext) string {
	prompt := workflow.GetAvailableWorkflowContext() + "\n\n"

	currentWorkflow := ctx.GetCurrentWorkflow()
	if currentWorkflow != nil {
		prompt += "## Current Context\n"
		prompt += fmt.Sprintf("Active Workflow: %s\n", currentWorkflow.GetWorkflowName())
		prompt += fmt.Sprintf("Current Step: %s\n\n", currentWorkflow.GetStep())
	} else {
		prompt += "## Current Context\n"
		prompt += "No active workflow\n\n"
	}

	// TODO: Add more message history for better context
	messages := ctx.GetLastUserMessages()
	if len(messages) > 1 {
		prompt += "## Recent Message History\n"
		for i := len(messages) - 2; i >= 0 && i >= len(messages)-4; i-- {
			prompt += fmt.Sprintf("- %s\n", messages[i].Message)
		}
		prompt += "\n"
	}

	prompt += "## User's Current Message\n"
	prompt += message.Message + "\n\n"

	// Add workflow switch signals
	prompt += "## Workflow Switch Signals\n"
	prompt += "The phrases like the following can indicate the user wants to CHANGE workflows:\n"
	prompt += "- \"let's change the topic\" / \"change topic\" / \"switch topic\"\n"
	prompt += "- \"switch to\" / \"move to\" / \"go to\"\n"
	prompt += "- \"I want to [action]\" / \"I need to [action]\" where action matches a different workflow\n"
	prompt += "- \"now I want to\" / \"instead, can you\" / \"let's do [something else]\"\n\n"

	prompt += "## Instructions\n"
	prompt += "Analyze the user's message and determine:\n"
	prompt += "1. Which workflow should handle this message\n"
	prompt += "2. What step should be executed\n"
	prompt += "3. Your confidence level (0.0 to 1.0)\n\n"

	if currentWorkflow != nil {
		prompt += "IMPORTANT: While continuity is valuable, users can clearly signal workflow changes. "
		prompt += "When you see workflow switch signals (like listed above) AND the user's request strongly matches another workflow's keywords/description, "
		prompt += "you can naturally have higher confidence in switching workflows.\n\n"
		prompt += "However, if only ONE of those conditions is met:\n"
		prompt += "- Switch signal present BUT request doesn't strongly match another workflow → likely changing direction WITHIN current workflow\n"
		prompt += "- Strong match to another workflow BUT no clear switch signal → could be a related question about that topic, not necessarily wanting to switch\n\n"
		prompt += "In these ambiguous cases, examine whether the request makes sense within the current workflow's scope and capabilities. "
		prompt += "If the current workflow can reasonably handle the request, prefer staying with lower confidence for switching. "
		prompt += "If the request is clearly outside the current workflow's purpose, switching may be appropriate even with weaker signals.\n"
	}

	return prompt
}
