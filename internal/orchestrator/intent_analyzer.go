package orchestrator

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"bob/internal/workflow"
	"context"
	"fmt"
	"time"
)

// TODO: Load these from config
const (
	// No active workflow
	confidenceThresholdNewWorkflow   = 0.70 // start a brand-new workflow from idle
	confidenceThresholdReturnHistory = 0.65 // return to a known (historical) workflow from idle — lower bar

	// Active workflow — staying is always the default
	confidenceThresholdChangeWorkflow       = 0.90 // switch to a BRAND-NEW workflow — very high bar
	confidenceThresholdSwitchToHistory      = 0.82 // switch to a HISTORICAL workflow — middle tier
)

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

	// 1. Clarifying question — takes priority, but only when confidence is genuinely low
	// and the AI produced an actual question (not a statement). High-confidence routing
	// must never block: the AI ignores "leave empty if confident" instructions sometimes.
	if aiResponse.ClarifyingQuestion != "" && !ctx.GetPendingClarification() &&
		aiResponse.Confidence < confidenceThresholdReturnHistory {
		ctx.SetPendingClarification(true)
		q := aiResponse.ClarifyingQuestion
		return core.Intent{
			IntentType:     core.IntentAskQuestion,
			WorkflowName:   aiResponse.WorkflowName,
			Confidence:     aiResponse.Confidence,
			Reasoning:      fmt.Sprintf("Asking clarifying question: %s", aiResponse.Reasoning),
			MessageToUser:  &q,
			NeedsUserInput: true, // blocks until user answers
		}
	}
	// Reset pending flag so the user's answer can be routed normally
	ctx.SetPendingClarification(false)

	currentWorkflow := ctx.GetCurrentWorkflow()
	suggestedWorkflow := aiResponse.WorkflowName
	suggestedStep := aiResponse.Step
	confidence := aiResponse.Confidence

	// 2. Check if suggested workflow is in history (lower threshold to return to known topics)
	isHistorical := workflowInHistory(suggestedWorkflow, ctx.GetWorkflowHistory())

	// 3. No active workflow
	if currentWorkflow == nil {
		threshold := confidenceThresholdNewWorkflow
		step := workflow.StepInit
		if isHistorical {
			threshold = confidenceThresholdReturnHistory
			// Let the AI decide the step for historical workflows (StepInit vs StepUserAsksQuestion)
			step = suggestedStep
			if step == "" {
				step = workflow.StepUserAsksQuestion // sensible fallback
			}
		}
		if confidence < threshold {
			return core.Intent{
				IntentType:   core.IntentAskQuestion,
				WorkflowName: suggestedWorkflow,
				Step:         "",
				Confidence:   confidence,
				Reasoning:    fmt.Sprintf("Confidence too low (%.2f < %.2f): %s", confidence, threshold, aiResponse.Reasoning),
			}
		}
		return core.Intent{
			IntentType:   core.IntentNewWorkflow,
			WorkflowName: suggestedWorkflow,
			Step:         step,
			Confidence:   confidence,
			Reasoning:    aiResponse.Reasoning,
		}
	}

	// 4. Active workflow exists — AI suggests a different one
	// Priority: stay (default) > switch-to-history > switch-to-brand-new
	currentWorkflowName := currentWorkflow.GetWorkflowName()
	if suggestedWorkflow != currentWorkflowName {
		switchThreshold := confidenceThresholdChangeWorkflow // 0.90 for brand-new workflows
		if isHistorical {
			switchThreshold = confidenceThresholdSwitchToHistory // 0.82 for historical workflows
		}

		if confidence < switchThreshold {
			// Not confident enough to change — stay in current workflow.
			// If the bot was waiting for a user answer, this IS the answer; otherwise it's a side question.
			var stayIntentType core.IntentType = core.IntentAskQuestion
			stayStep := workflow.StepUserAsksQuestion
			if ctx.GetCurrentStatus() == core.StatusWaitForUser {
				stayIntentType = core.IntentAnswerQuestion
				stayStep = workflow.StepUserAnsweringQuestion
			}
			return core.Intent{
				IntentType:   stayIntentType,
				WorkflowName: currentWorkflowName,
				Step:         stayStep,
				Confidence:   confidence,
				Reasoning:    fmt.Sprintf("Uncertain input - staying with current workflow: %s", aiResponse.Reasoning),
			}
		}

		// Let the AI decide the step for the target workflow.
		// Historical workflows: AI picks StepInit (restart) or StepUserAsksQuestion (follow-up).
		// Brand-new workflows always start fresh.
		switchStep := workflow.StepInit
		if isHistorical {
			switchStep = suggestedStep
			if switchStep == "" {
				switchStep = workflow.StepUserAsksQuestion
			}
		}
		return core.Intent{
			IntentType:   core.IntentNewWorkflow,
			WorkflowName: suggestedWorkflow,
			Step:         switchStep,
			Confidence:   confidence,
			Reasoning:    aiResponse.Reasoning,
		}
	}

	// 5. Same workflow — determine intent type from step
	intentType := mapStepToIntentType(suggestedStep)
	// If the bot was waiting for the user's answer, this message IS that answer —
	// override StepUserAsksQuestion → StepUserAnsweringQuestion so the workflow
	// receives it as an answer rather than a side question.
	if ctx.GetCurrentStatus() == core.StatusWaitForUser && intentType == core.IntentAskQuestion {
		intentType = core.IntentAnswerQuestion
		suggestedStep = workflow.StepUserAnsweringQuestion
	}
	// When waiting for the user, always use StepUserAnsweringQuestion regardless of what
	// step the AI suggested — prevents workflow-specific steps (e.g. tsw_collect_result)
	// from being injected when the user is simply replying to a question.
	if ctx.GetCurrentStatus() == core.StatusWaitForUser && intentType == core.IntentAnswerQuestion {
		suggestedStep = workflow.StepUserAnsweringQuestion
	}
	// If the bot was NOT waiting for an answer, the user cannot be answering a question —
	// treat as a side question. This prevents workflow-specific steps (e.g. tsw_collect_result)
	// from being injected by the AI when the user sends a follow-up after workflow completion.
	if ctx.GetCurrentStatus() != core.StatusWaitForUser && intentType == core.IntentAnswerQuestion {
		intentType = core.IntentAskQuestion
		suggestedStep = workflow.StepUserAsksQuestion
	}
	return core.Intent{
		IntentType:   intentType,
		WorkflowName: suggestedWorkflow,
		Step:         suggestedStep,
		Confidence:   confidence,
		Reasoning:    aiResponse.Reasoning,
	}
}

// workflowInHistory returns true if a workflow with the given name exists in the history slice.
func workflowInHistory(name string, history []*core.WorkflowHistoryEntry) bool {
	for _, e := range history {
		if e.WorkflowName == name {
			return true
		}
	}
	return false
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
	WorkflowName       string
	Step               string
	Confidence         float64
	Reasoning          string
	ClarifyingQuestion string // non-empty = block and ask the user before routing
}

func callIntentAI(message *core.Message, ctx *core.ConversationContext) (*aiIntentResponse, error) {
	schema := buildIntentSchema()
	prompt := buildIntentPrompt(message, ctx)

	// If there is an active workflow with a recent AI response, branch off it so the
	// intent analyzer sees full conversation context without polluting the thread.
	var opts []ai.Option
	if wf := ctx.GetCurrentWorkflow(); wf != nil {
		if respID := wf.GetLastResponseID(); respID != nil {
			opts = append(opts, ai.BranchFromResponse(*respID))
		}
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

	var clarifyingQuestion string
	if data.IsSet("clarifying_question") {
		if q, err := data.GetString("clarifying_question"); err == nil {
			clarifyingQuestion = q
		}
	}

	return &aiIntentResponse{
		WorkflowName:       workflowName,
		Step:               step,
		Confidence:         confidence,
		Reasoning:          reasoning,
		ClarifyingQuestion: clarifyingQuestion,
	}, nil
}

func buildIntentSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("workflow_name", ai.Required(), ai.Description("The workflow that should handle this user message")).
		AddString("step", ai.Required(), ai.Description("The specific step to execute (use default steps when appropriate)")).
		AddFloat("confidence", ai.Required(), ai.Description("Confidence score from 0.0 to 1.0"), ai.Range(0.0, 1.0)).
		AddString("reasoning", ai.Required(), ai.Description("Brief explanation of why this workflow and step were chosen")).
		AddString("clarifying_question", ai.Description("If you cannot confidently pick a workflow, write a SHORT yes/no clarifying question for the user. Leave empty if confident. This triggers a blocking wait — use sparingly."))
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

	// Workflow history — helps route follow-up questions to completed workflows
	history := ctx.GetWorkflowHistory()
	if len(history) > 0 {
		prompt += "## Workflow History (this thread, most recent first)\n"
		for i := len(history) - 1; i >= 0; i-- {
			entry := history[i]
			ago := time.Since(entry.CompletedAt).Round(time.Minute)
			summary := entry.Summary
			if summary == "" {
				summary = "(no summary)"
			}
			prompt += fmt.Sprintf("%d. %s (%s ago): %q\n", len(history)-i, entry.WorkflowName, ago, summary)
		}
		prompt += "\nIf the user's message clearly relates to a previous workflow topic, you may route back to it using step=asking_question.\n\n"
	}

	// Pending clarification — user is answering a question Bob asked
	requestToUser := ctx.GetRequestToUser()
	if requestToUser != "" {
		prompt += "## Pending Clarification\n"
		prompt += fmt.Sprintf("The last thing Bob asked the user was: %q\n", requestToUser)
		prompt += "The user's current message is likely a response to that question — weight it heavily when routing.\n\n"
	}

	// Recent message history
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
	prompt += "The following phrases indicate the user wants to CHANGE workflows:\n"
	prompt += "- \"let's change the topic\" / \"change topic\" / \"switch topic\"\n"
	prompt += "- \"switch to\" / \"move to\" / \"go to\"\n"
	prompt += "- \"I want to [action]\" / \"I need to [action]\" where action matches a different workflow\n"
	prompt += "- \"now I want to\" / \"instead, can you\" / \"let's do [something else]\"\n\n"

	prompt += "## Instructions\n"
	prompt += "Analyze the user's message and determine:\n"
	prompt += "1. Which workflow should handle this message\n"
	prompt += "2. What step should be executed\n"
	prompt += "3. Your confidence level (0.0 to 1.0)\n\n"
	prompt += "Confidence thresholds (set by the system — do not lower these in your reasoning):\n"
	if currentWorkflow == nil {
		prompt += fmt.Sprintf("- Starting a brand-new workflow: require confidence >= %.2f\n", confidenceThresholdNewWorkflow)
		prompt += fmt.Sprintf("- Returning to a workflow seen in history: require confidence >= %.2f\n\n", confidenceThresholdReturnHistory)
	} else {
		prompt += fmt.Sprintf("- STAY in active workflow '%s' (default): confidence < %.2f → route as side question\n", currentWorkflow.GetWorkflowName(), confidenceThresholdSwitchToHistory)
		prompt += fmt.Sprintf("- Switch to a HISTORICAL workflow: require confidence >= %.2f\n", confidenceThresholdSwitchToHistory)
		prompt += fmt.Sprintf("- Switch to a BRAND-NEW workflow: require confidence >= %.2f\n\n", confidenceThresholdChangeWorkflow)
	}
	prompt += "If you are unsure and cannot meet the threshold, set clarifying_question to a short yes/no question instead of guessing.\n\n"

	if currentWorkflow != nil {
		prompt += "IMPORTANT: Staying in the current workflow is always the default. "
		prompt += "When you see workflow switch signals AND the request strongly matches another workflow, you may suggest switching. "
		prompt += fmt.Sprintf("If the other workflow is in the history above, it only requires %.2f confidence; a brand-new one requires %.2f.\n\n",
			confidenceThresholdSwitchToHistory, confidenceThresholdChangeWorkflow)
		prompt += "If only ONE of those conditions is met:\n"
		prompt += "- Switch signal present BUT request doesn't strongly match another workflow → route as side question within current workflow\n"
		prompt += "- Strong match to another workflow BUT no clear switch signal → likely a related question, not a workflow switch\n\n"
	}

	return prompt
}
