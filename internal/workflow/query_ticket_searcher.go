package workflow

import (
	"bob/definitions/personalities"
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/tool"
	"encoding/json"
	"fmt"
)

const (
	StepQtswSearch   = "qtsw_search"
	StepQtswEvaluate = "qtsw_evaluate"
)

// qtswInstruction is the JSON payload passed from the main workflow to each sub-worker.
type qtswInstruction struct {
	WorkerID     string         `json:"worker_id"`
	Angle        string         `json:"angle"`
	SearchParams map[string]any `json:"search_params"`
	UserContext  string         `json:"user_context"`
	RejectedIDs  []int          `json:"rejected_ids"`
}

// QueryTicketSearcher is an internal sub-workflow spawned by QueryTicket.
// It runs one search angle, evaluates candidates, and reports back.
func QueryTicketSearcher(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🔍 QueryTicketSearcher: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🔍 QueryTicketSearcher: step=%v", step)

	switch step {
	case StepQtswSearch:
		return qtswHandleSearch(context, sourceAction)
	case StepQtswEvaluate:
		return qtswHandleEvaluate(context, sourceAction)
	default:
		return nil, fmt.Errorf("queryTicketSearcher: unknown step: %v", step)
	}
}

func qtswHandleSearch(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	workerID, ok := getInput(sourceAction, core.InputSubWorkerID).(string)
	if !ok || workerID == "" {
		return nil, fmt.Errorf("queryTicketSearcher: missing worker_id")
	}

	instructionStr, ok := getInput(sourceAction, core.InputMessage).(string)
	if !ok || instructionStr == "" {
		return nil, fmt.Errorf("queryTicketSearcher: missing instruction")
	}

	var instruction qtswInstruction
	if err := json.Unmarshal([]byte(instructionStr), &instruction); err != nil {
		return nil, fmt.Errorf("queryTicketSearcher: failed to parse instruction: %w", err)
	}

	logger.Debugf("🔍 QueryTicketSearcher worker=%s: angle=%q", workerID, instruction.Angle)

	// Store instruction so we can access it in the evaluate step
	wf := context.GetCurrentWorkflow()
	wf.SetWorkflowData(subWorkerKey(workerID, "instruction"), instructionStr)

	// Build tool action for ado_search_tickets
	toolAction := core.NewAction(core.ActionTool)
	toolAction.Input = map[core.InputType]any{
		core.InputToolName:      tool.ToolADOSearchTickets,
		core.InputToolArgs:      instruction.SearchParams,
		core.InputStep:          StepQtswEvaluate,
		core.InputWorkflowName:  WorkflowQueryTicketSearcher,
		core.InputSubWorkerID:   workerID,
	}

	return []*core.Action{toolAction}, nil
}

func qtswHandleEvaluate(context *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	workerID, ok := getInput(sourceAction, core.InputSubWorkerID).(string)
	if !ok || workerID == "" {
		return nil, fmt.Errorf("queryTicketSearcher evaluate: missing worker_id")
	}

	toolResult, ok := getInput(sourceAction, core.InputToolResult).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("queryTicketSearcher evaluate: missing tool result")
	}

	wf := context.GetCurrentWorkflow()

	// Retrieve the instruction stored in search step
	instructionStr, _ := wf.GetWorkflowData(subWorkerKey(workerID, "instruction")).(string)
	var instruction qtswInstruction
	json.Unmarshal([]byte(instructionStr), &instruction) //nolint:errcheck

	// Format search results for AI evaluation
	resultsJSON, _ := json.Marshal(toolResult)
	count, _ := toolResult["count"].(float64)

	logger.Debugf("🔍 QueryTicketSearcher worker=%s: search returned %.0f results", workerID, count)

	prompt := fmt.Sprintf(
		"You are worker %s. Your search angle: %q\n\n"+
			"User context:\n%s\n\n"+
			"Search results (%d total):\n%s\n\n"+
			"Rejected ticket IDs (exclude from candidates): %v\n\n"+
			"Evaluate each result for how well it matches what the user is looking for. "+
			"Return your worker_id, found_any, and candidates_json.",
		workerID,
		instruction.Angle,
		instruction.UserContext,
		int(count),
		string(resultsJSON),
		instruction.RejectedIDs,
	)

	schema := buildQtswEvaluateSchema(workerID)
	personality := personalities.GetPersonality(personalities.PersonalityQueryTicketSearcher).PersonalityPrompt

	convKey := fmt.Sprintf("qt_searcher_%s", workerID)
	actions := askAI(prompt, "", personality, schema, convKey)
	// Route result back to parent workflow's collect step (no InputWorkflowName = uses GetCurrentWorkflow = parent)
	actions[0].Input[core.InputStep] = StepQtCollectResult

	return actions, nil
}

func buildQtswEvaluateSchema(workerID string) *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("worker_id",
			ai.Required(),
			ai.Description(fmt.Sprintf("Your worker ID — must be exactly %q", workerID))).
		AddBool("found_any",
			ai.Required(),
			ai.Description("True if at least one plausible candidate was found (confidence >= 0.50)")).
		AddString("candidates_json",
			ai.Required(),
			ai.Description(`JSON array of evaluated candidates. Each item: {"id":int,"title":string,"confidence":float,"reasoning":string,"state":string,"assigned_to":string,"work_item_type":string,"summary":string}. Return empty array [] if no plausible candidates found.`))
}
