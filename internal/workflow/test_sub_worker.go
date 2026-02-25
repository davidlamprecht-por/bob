package workflow

import (
	"bob/definitions/personalities"
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"fmt"
)

const (
	StepSubWorkerRun = "sub_worker_run"
)

// TestSubWorker is an internal sub-workflow dispatched by TestSubworkflows.
// It receives a worker ID and instructions, makes one AI call, and routes
// the result back to the parent workflow's StepTswCollectResult step.
func TestSubWorker(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🔷 TestSubWorker: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🔷 TestSubWorker: step=%v", step)

	switch step {
	case StepSubWorkerRun:
		workerID, ok := getInput(sourceAction, core.InputSubWorkerID).(string)
		if !ok || workerID == "" {
			return nil, fmt.Errorf("missing or invalid sub_worker_id")
		}
		instructions, _ := getInput(sourceAction, core.InputMessage).(string)
		logger.Debugf("🔷 TestSubWorker: worker_id=%s, instructions=%q", workerID, instructions)

		schema := ai.NewSchema().
			AddString("worker_id", ai.Required(), ai.Description("Your assigned worker ID — must be exactly the ID given to you")).
			AddString("response", ai.Required(), ai.Description("Your one-line creative response. Must include your worker ID."))

		convKey := fmt.Sprintf("sub_worker_%s", workerID)
		prompt := fmt.Sprintf("Your worker ID is %s. %s", workerID, instructions)

		actions := askAI(
			prompt,
			"",
			personalities.GetPersonality(personalities.PersonalityTestSubWorker).PersonalityPrompt,
			schema,
			convKey,
		)
		actions[0].Input[core.InputStep] = StepTswCollectResult
		return actions, nil

	default:
		return nil, fmt.Errorf("unknown sub-worker step: %v", step)
	}
}
