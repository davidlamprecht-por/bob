package workflow

import (
	"bob/definitions/personalities"
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"fmt"
)

const (
	StepTswSpawnWorkers  = "tsw_spawn_workers"
	StepTswCollectResult = "tsw_collect_result"
	StepTswSendSummary   = "tsw_send_summary"
)

// TestSubworkflows tests sub-workflow dispatch, async execution, personality registry,
// and context propagation. Trigger with "test subworkflows".
func TestSubworkflows(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🔶 TestSubworkflows: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🔶 TestSubworkflows: step=%v", step)

	wf := context.GetCurrentWorkflow()

	switch step {
	case StepInit:
		logger.Debug("🔶 TestSubworkflows: StepInit - asking orchestrator how many workers")

		schema := ai.NewSchema().
			AddInt("worker_count",
				ai.Required(),
				ai.Range(2, 4),
				ai.Description("Number of sub-workers to spawn. Must be between 2 and 4 inclusive."))

		actions := askAI(
			"Decide how many sub-tasks to create. Choose a number between 2 and 4 inclusive.",
			"",
			personalities.GetPersonality(personalities.PersonalityTestOrchestrator).PersonalityPrompt,
			schema,
			"tsw_orchestrator",
		)
		actions[0].Input[core.InputStep] = StepTswSpawnWorkers
		return actions, nil

	case StepTswSpawnWorkers:
		logger.Debug("🔶 TestSubworkflows: StepTswSpawnWorkers - spawning sub-workers")

		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			return nil, fmt.Errorf("expected AI response for spawn workers step")
		}
		response, ok := aiResponse.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("invalid AI response type")
		}

		workerCount, err := response.Data().GetInt("worker_count")
		if err != nil {
			return nil, fmt.Errorf("failed to get worker_count: %w", err)
		}
		if workerCount < 2 {
			workerCount = 2
		}
		if workerCount > 4 {
			workerCount = 4
		}

		wf.SetWorkflowData("tsw_expected_count", workerCount)
		logger.Debugf("🔶 TestSubworkflows: Spawning %d sub-workers", workerCount)

		asyncAction := core.NewAction(core.ActionAsync)
		for i := 1; i <= workerCount; i++ {
			workerID := fmt.Sprintf("%d", i)
			subAction := core.NewAction(core.ActionWorkflow)
			subAction.Input = make(map[core.InputType]any)
			subAction.Input[core.InputWorkflowName] = WorkflowTestSubWorker
			subAction.Input[core.InputStep] = StepSubWorkerRun
			subAction.Input[core.InputSubWorkerID] = workerID
			subAction.Input[core.InputMessage] = "Write a creative one-liner about any topic you find interesting."
			asyncAction.AsyncActions = append(asyncAction.AsyncActions, subAction)
		}
		return []*core.Action{asyncAction}, nil

	case StepTswCollectResult:
		logger.Debug("🔶 TestSubworkflows: StepTswCollectResult - collecting sub-worker result")

		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			return nil, fmt.Errorf("expected AI response in collect result step")
		}
		response, ok := aiResponse.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("invalid AI response type")
		}

		workerID, err := response.Data().GetString("worker_id")
		if err != nil {
			return nil, fmt.Errorf("failed to get worker_id: %w", err)
		}
		workerResponse, err := response.Data().GetString("response")
		if err != nil {
			return nil, fmt.Errorf("failed to get response: %w", err)
		}

		wf.SetWorkflowData(subWorkerKey(workerID, "result"), workerResponse)
		logger.Debugf("🔶 TestSubworkflows: Stored result for worker %s: %q", workerID, workerResponse)

		expectedCount, _ := wf.GetWorkflowData("tsw_expected_count").(int)
		results := make(map[string]string, expectedCount)
		for i := 1; i <= expectedCount; i++ {
			id := fmt.Sprintf("%d", i)
			if val := wf.GetWorkflowData(subWorkerKey(id, "result")); val != nil {
				results[id] = val.(string)
			}
		}

		logger.Debugf("🔶 TestSubworkflows: Collected %d/%d results", len(results), expectedCount)

		if len(results) < expectedCount {
			return nil, nil // Wait for more results
		}

		// All results in — build evaluation prompt
		evalPrompt := fmt.Sprintf("I spawned %d sub-workers. Here are their responses:\n\n", expectedCount)
		for i := 1; i <= expectedCount; i++ {
			id := fmt.Sprintf("%d", i)
			evalPrompt += fmt.Sprintf("Worker %s: %s\n", id, results[id])
		}
		evalPrompt += "\nEvaluate: did all expected workers respond? Which response is most interesting and why? Provide a concise summary for the user."

		schema := ai.NewSchema().
			AddInt("responses_received", ai.Required(), ai.Description("Number of worker responses received")).
			AddInt("expected_responses", ai.Required(), ai.Description("Number of workers that were supposed to respond")).
			AddString("most_interesting_worker_id", ai.Required(), ai.Description("ID of the worker with the most interesting response")).
			AddString("reason", ai.Required(), ai.Description("Why this response is most interesting")).
			AddString("summary", ai.Required(), ai.Description("A concise summary paragraph for the user"))

		actions := askAI(
			evalPrompt,
			"",
			personalities.GetPersonality(personalities.PersonalityTestEvaluator).PersonalityPrompt,
			schema,
			"", // Main conversation — gives context for follow-up side questions
		)
		actions[0].Input[core.InputStep] = StepTswSendSummary

		completeAction := core.NewAction(core.ActionCompleteAsync)
		return append(actions, completeAction), nil

	case StepTswSendSummary:
		logger.Debug("🔶 TestSubworkflows: StepTswSendSummary - sending summary to user")

		aiResponse := getInput(sourceAction, core.InputAIResponse)
		if aiResponse == nil {
			return nil, fmt.Errorf("expected AI response for summary step")
		}
		response, ok := aiResponse.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("invalid AI response type")
		}

		summary, err := response.Data().GetString("summary")
		if err != nil {
			return nil, fmt.Errorf("failed to get summary: %w", err)
		}

		msgAction := core.NewAction(core.ActionUserMessage)
		msgAction.Input = make(map[core.InputType]any)
		msgAction.Input[core.InputMessage] = summary
		return []*core.Action{msgAction}, nil

	default:
		return nil, fmt.Errorf("unknown workflow step: %v", step)
	}
}
