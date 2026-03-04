package workflow

import (
	"bob/definitions/personalities"
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/tool"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Steps for the queryTicket workflow
const (
	StepQtClarify          = "qt_clarify"
	StepQtSpawnWorkers     = "qt_spawn_workers"
	StepQtCollectResult    = "qt_collect_result"
	StepQtAnalyze          = "qt_analyze"
	StepQtAskRefineQuestion = "qt_ask_refine_question"
	StepQtHandlePick       = "qt_handle_pick"
	StepQtPresentTicket    = "qt_present_ticket"
)

// Workflow data keys
const (
	qtKeyAttemptCount      = "qt_attempt_count"
	qtKeyRejectedIDsJSON   = "qt_rejected_ids_json"
	qtKeyExpectedWorkers   = "qt_expected_workers"
	qtKeyCollectedResults  = "qt_collected_results"
	qtKeyFoundTicketJSON   = "qt_found_ticket_json"
	qtKeyCandidatesJSON    = "qt_candidates_json"
	qtKeyAllCandidatesJSON = "qt_all_candidates_json"
	qtKeyPendingStep       = "qt_pending_step"
	qtKeyKeywords          = "qt_keywords"
	qtKeyAssignedTo        = "qt_assigned_to"
	qtKeyCreatedBy         = "qt_created_by"
	qtKeyQAPerson          = "qt_qa_person"
	qtKeyWorkItemType      = "qt_work_item_type"
	qtKeyState             = "qt_state"
	qtKeyTagsJSON          = "qt_tags_json"
	qtKeyAreaPath          = "qt_area_path"
	qtKeyIterationPath     = "qt_iteration_path"
)

// qtStrategy is used to parse the strategies_json returned by the plan AI.
type qtStrategy struct {
	WorkerID     string   `json:"worker_id"`
	Angle        string   `json:"angle"`
	Title        string   `json:"title"`
	AssignedTo   string   `json:"assigned_to"`
	CreatedBy    string   `json:"created_by"`
	QAPerson     string   `json:"qa_person"`
	WorkItemType string   `json:"work_item_type"`
	State        string   `json:"state"`
	Tags         []string `json:"tags"`
	AreaPath     string   `json:"area_path"`
	IterationPath string  `json:"iteration_path"`
	MaxResults   int      `json:"max_results"`
}

func (s qtStrategy) searchParams() map[string]any {
	params := map[string]any{}
	if s.Title != "" {
		params["title"] = s.Title
	}
	if s.AssignedTo != "" {
		params["assigned_to"] = s.AssignedTo
	}
	if s.CreatedBy != "" {
		params["created_by"] = s.CreatedBy
	}
	if s.QAPerson != "" {
		params["qa_person"] = s.QAPerson
	}
	if s.WorkItemType != "" {
		params["work_item_type"] = s.WorkItemType
	}
	if s.State != "" {
		params["state"] = s.State
	}
	if len(s.Tags) > 0 {
		anyTags := make([]any, len(s.Tags))
		for i, t := range s.Tags {
			anyTags[i] = t
		}
		params["tags"] = anyTags
	}
	if s.AreaPath != "" {
		params["area_path"] = s.AreaPath
	}
	if s.IterationPath != "" {
		params["iteration_path"] = s.IterationPath
	}
	if s.MaxResults > 0 {
		params["max_results"] = s.MaxResults
	} else {
		params["max_results"] = 15
	}
	return params
}

// QueryTicket finds ADO tickets based on natural language description.
// This workflow is stateful: once a ticket is found it stays in context for follow-up questions.
func QueryTicket(ctx *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🎫 QueryTicket: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🎫 QueryTicket: step=%v", step)

	wf := ctx.GetCurrentWorkflow()

	switch step {
	case StepInit:
		return qtHandleInit(ctx, wf)

	case StepQtClarify:
		return qtHandleClarify(ctx, wf, sourceAction)

	case StepQtSpawnWorkers:
		return qtHandleSpawnWorkers(ctx, wf, sourceAction)

	case StepQtCollectResult:
		return qtHandleCollectResult(ctx, wf, sourceAction)

	case StepQtAnalyze:
		return qtHandleAnalyze(ctx, wf, sourceAction)

	case StepQtAskRefineQuestion:
		return qtHandleAskRefineQuestion(wf, sourceAction)

	case StepQtHandlePick:
		return qtHandlePick(ctx, wf, sourceAction)

	case StepQtPresentTicket:
		return qtHandlePresentTicket(ctx, wf, sourceAction)

	case StepUserAnsweringQuestion:
		return qtHandleUserAnswer(ctx, wf, sourceAction)

	case StepUserAsksQuestion:
		return handleSideQuestion(ctx, workflows[WorkflowQueryTicket], sourceAction)

	default:
		logger.Warnf("⚠️  QueryTicket: unknown step: %v", step)
		return nil, fmt.Errorf("queryTicket: unknown step: %v", step)
	}
}

// ----- Step handlers -----

func qtHandleInit(ctx *core.ConversationContext, wf *core.WorkflowContext) ([]*core.Action, error) {
	logger.Debug("🎫 QueryTicket StepInit: resetting state and extracting info")

	// Manually reset since OptionOverwriteHandleDefaultSteps is set
	wf.ResetWorkflowData()
	wf.SetAIConversation(nil, nil)

	messages := ctx.GetLastUserMessages()
	if len(messages) == 0 {
		return nil, fmt.Errorf("queryTicket init: no user messages")
	}
	userMsg := messages[len(messages)-1].Message

	schema := buildQtExtractSchema()
	personality := qtOrchestratorPersonality()
	actions := askAI(userMsg, "", personality, schema, "qt_main")
	actions[0].Input[core.InputStep] = StepQtClarify
	return actions, nil
}

func qtHandleClarify(ctx *core.ConversationContext, wf *core.WorkflowContext, sourceAction *core.Action) ([]*core.Action, error) {
	aiResponse := getInput(sourceAction, core.InputAIResponse)
	if aiResponse == nil {
		return nil, fmt.Errorf("queryTicket clarify: expected AI response")
	}
	response := aiResponse.(*ai.Response)
	data := response.Data()

	// Store extracted info
	qtStoreExtractedInfo(wf, data)

	shouldClarify, _ := data.GetBool("should_clarify")
	if shouldClarify {
		q, _ := data.GetString("clarifying_question")
		if q == "" {
			q = "Do you know any other details about the ticket — like who it was assigned to, the project it was in, or roughly when it was created?"
		}
		logger.Debugf("🎫 QueryTicket clarify: asking user: %q", q)
		wf.SetWorkflowData(qtKeyPendingStep, "qt_clarify_asked")
		return []*core.Action{qtWaitAction(q)}, nil
	}

	// No clarify needed — go straight to planning
	messages := ctx.GetLastUserMessages()
	var latestMsg string
	if len(messages) > 0 {
		latestMsg = messages[len(messages)-1].Message
	}
	return qtPlanSearch(wf, latestMsg)
}

func qtHandleSpawnWorkers(_ *core.ConversationContext, wf *core.WorkflowContext, sourceAction *core.Action) ([]*core.Action, error) {
	aiResponse := getInput(sourceAction, core.InputAIResponse)
	if aiResponse == nil {
		return nil, fmt.Errorf("queryTicket spawnWorkers: expected AI response")
	}
	response := aiResponse.(*ai.Response)
	data := response.Data()

	workerCount, err := data.GetInt("worker_count")
	if err != nil || workerCount < 1 {
		workerCount = 2
	}
	if workerCount > 5 {
		workerCount = 5
	}

	strategiesJSON, err := data.GetString("strategies_json")
	if err != nil || strategiesJSON == "" {
		return nil, fmt.Errorf("queryTicket spawnWorkers: missing strategies_json")
	}

	// Sanitize unescaped backslashes the AI may produce (e.g. "Enterprise\REI" → "Enterprise\\REI").
	// In JSON, only "\", """, "/", "b","f","n","r","t","u" are valid after a backslash.
	strategiesJSON = sanitizeJSONBackslashes(strategiesJSON)

	var strategies []qtStrategy
	if err := json.Unmarshal([]byte(strategiesJSON), &strategies); err != nil {
		return nil, fmt.Errorf("queryTicket spawnWorkers: failed to parse strategies: %w", err)
	}
	if len(strategies) == 0 {
		return nil, fmt.Errorf("queryTicket spawnWorkers: empty strategies list")
	}

	wf.SetWorkflowData(qtKeyExpectedWorkers, len(strategies))
	wf.SetWorkflowData(qtKeyCollectedResults, 0)

	userContext := qtBuildUserContext(wf)
	rejectedIDs := qtGetRejectedIDs(wf)

	logger.Debugf("🎫 QueryTicket spawnWorkers: spawning %d workers", len(strategies))

	asyncAction := core.NewAction(core.ActionAsync)
	for _, strategy := range strategies {
		instruction := qtswInstruction{
			WorkerID:     strategy.WorkerID,
			Angle:        strategy.Angle,
			SearchParams: strategy.searchParams(),
			UserContext:  userContext,
			RejectedIDs:  rejectedIDs,
		}
		instructionJSON, _ := json.Marshal(instruction)

		subAction := core.NewAction(core.ActionWorkflow)
		subAction.Input = map[core.InputType]any{
			core.InputWorkflowName: WorkflowQueryTicketSearcher,
			core.InputStep:         StepQtswSearch,
			core.InputSubWorkerID:  strategy.WorkerID,
			core.InputMessage:      string(instructionJSON),
		}
		asyncAction.AsyncActions = append(asyncAction.AsyncActions, subAction)
	}

	return []*core.Action{asyncAction}, nil
}

func qtHandleCollectResult(ctx *core.ConversationContext, wf *core.WorkflowContext, sourceAction *core.Action) ([]*core.Action, error) {
	aiResponse := getInput(sourceAction, core.InputAIResponse)
	if aiResponse == nil {
		return nil, fmt.Errorf("queryTicket collectResult: expected AI response")
	}
	response := aiResponse.(*ai.Response)
	data := response.Data()

	workerID, _ := data.GetString("worker_id")
	candidatesJSON, _ := data.GetString("candidates_json")

	logger.Debugf("🎫 QueryTicket collectResult: worker=%s", workerID)

	wf.SetWorkflowData(subWorkerKey(workerID, "result"), candidatesJSON)

	collected, _ := wf.GetWorkflowData(qtKeyCollectedResults).(int)
	collected++
	wf.SetWorkflowData(qtKeyCollectedResults, collected)

	expected, _ := wf.GetWorkflowData(qtKeyExpectedWorkers).(int)
	if collected < expected {
		logger.Debugf("🎫 QueryTicket collectResult: waiting for more results (%d/%d)", collected, expected)
		return nil, nil
	}

	// All results in — build synthesis prompt
	logger.Debug("🎫 QueryTicket collectResult: all results received, synthesizing")
	var allResultsText []string
	for i := 1; i <= expected; i++ {
		wID := fmt.Sprintf("%d", i)
		result, _ := wf.GetWorkflowData(subWorkerKey(wID, "result")).(string)
		if result != "" && result != "[]" {
			allResultsText = append(allResultsText, fmt.Sprintf("Worker %s candidates: %s", wID, result))
		}
	}

	// Accumulate for exhausted fallback
	if len(allResultsText) > 0 {
		existing, _ := wf.GetWorkflowData(qtKeyAllCandidatesJSON).(string)
		combined := strings.Join(allResultsText, "\n")
		if existing != "" {
			combined = existing + "\n" + combined
		}
		wf.SetWorkflowData(qtKeyAllCandidatesJSON, combined)
	}

	rejectedIDs := qtGetRejectedIDs(wf)
	synthPrompt := qtBuildSynthesisPrompt(wf, allResultsText, rejectedIDs)

	personality := qtOrchestratorPersonality()
	schema := buildQtAnalyzeSchema()
	actions := askAI(synthPrompt, "", personality, schema, "qt_main")
	actions[0].Input[core.InputStep] = StepQtAnalyze

	completeAction := core.NewAction(core.ActionCompleteAsync)
	return append(actions, completeAction), nil
}

func qtHandleAnalyze(ctx *core.ConversationContext, wf *core.WorkflowContext, sourceAction *core.Action) ([]*core.Action, error) {
	aiResponse := getInput(sourceAction, core.InputAIResponse)
	if aiResponse == nil {
		return nil, fmt.Errorf("queryTicket analyze: expected AI response")
	}
	response := aiResponse.(*ai.Response)
	data := response.Data()

	branch, _ := data.GetString("branch")
	messageToUser, _ := data.GetString("message_to_user")
	candidatesJSON, _ := data.GetString("candidates_json")

	logger.Debugf("🎫 QueryTicket analyze: branch=%q", branch)

	switch branch {
	case "present_ticket":
		topID, _ := data.GetInt("top_ticket_id")
		logger.Debugf("🎫 QueryTicket analyze: fetching ticket #%d", topID)
		toolAction := core.NewAction(core.ActionTool)
		toolAction.Input = map[core.InputType]any{
			core.InputToolName: tool.ToolADOGetTicket,
			core.InputToolArgs: map[string]any{"work_item_id": topID},
			core.InputStep:     StepQtPresentTicket,
		}
		return []*core.Action{toolAction}, nil

	case "show_candidates":
		wf.SetWorkflowData(qtKeyCandidatesJSON, candidatesJSON)
		wf.SetWorkflowData(qtKeyPendingStep, "qt_show_candidates")
		msg := qtFormatCandidatesList(messageToUser, candidatesJSON)
		return []*core.Action{qtWaitAction(msg)}, nil

	case "disambiguate":
		wf.SetWorkflowData(qtKeyCandidatesJSON, candidatesJSON)
		wf.SetWorkflowData(qtKeyPendingStep, "qt_disambiguate")
		return []*core.Action{qtWaitAction(messageToUser)}, nil

	case "narrow_down":
		wf.SetWorkflowData(qtKeyPendingStep, "qt_narrow_down")
		return []*core.Action{qtWaitAction(messageToUser)}, nil

	case "refine":
		return qtRefine(ctx, wf)

	default:
		logger.Warnf("⚠️  QueryTicket analyze: unknown branch %q, falling back to refine", branch)
		return qtRefine(ctx, wf)
	}
}

func qtHandleAskRefineQuestion(wf *core.WorkflowContext, sourceAction *core.Action) ([]*core.Action, error) {
	aiResponse := getInput(sourceAction, core.InputAIResponse)
	if aiResponse == nil {
		return nil, fmt.Errorf("queryTicket askRefineQuestion: expected AI response")
	}
	response := aiResponse.(*ai.Response)
	data := response.Data()

	question, _ := data.GetString("question")
	if question == "" {
		question = "Do you remember any other details about the ticket that might help me narrow it down?"
	}

	wf.SetWorkflowData(qtKeyPendingStep, "qt_refine_asked")
	return []*core.Action{qtWaitAction(question)}, nil
}

func qtHandlePick(ctx *core.ConversationContext, wf *core.WorkflowContext, sourceAction *core.Action) ([]*core.Action, error) {
	aiResponse := getInput(sourceAction, core.InputAIResponse)
	if aiResponse == nil {
		return nil, fmt.Errorf("queryTicket handlePick: expected AI response")
	}
	response := aiResponse.(*ai.Response)
	data := response.Data()

	action, _ := data.GetString("action")
	logger.Debugf("🎫 QueryTicket handlePick: action=%q", action)

	switch action {
	case "pick":
		ticketID, _ := data.GetInt("ticket_id")
		toolAction := core.NewAction(core.ActionTool)
		toolAction.Input = map[core.InputType]any{
			core.InputToolName: tool.ToolADOGetTicket,
			core.InputToolArgs: map[string]any{"work_item_id": ticketID},
			core.InputStep:     StepQtPresentTicket,
		}
		return []*core.Action{toolAction}, nil

	case "none":
		return qtRefine(ctx, wf)

	case "keep_trying":
		messages := ctx.GetLastUserMessages()
		var latestMsg string
		if len(messages) > 0 {
			latestMsg = messages[len(messages)-1].Message
		}
		return qtPlanSearch(wf, latestMsg)

	case "show_best":
		allCandidates, _ := wf.GetWorkflowData(qtKeyAllCandidatesJSON).(string)
		msg := "Here are the closest matches I found across all my searches:\n" + allCandidates
		wf.SetWorkflowData(qtKeyCandidatesJSON, allCandidates)
		wf.SetWorkflowData(qtKeyPendingStep, "qt_show_candidates")
		return []*core.Action{qtWaitAction(msg)}, nil

	case "give_up", "done":
		msgAction := core.NewAction(core.ActionUserMessage)
		msgAction.Input = map[core.InputType]any{
			core.InputMessage: "No problem! Let me know if you need anything else.",
		}
		return []*core.Action{msgAction}, nil

	default:
		logger.Warnf("⚠️  QueryTicket handlePick: unknown action %q", action)
		return qtRefine(ctx, wf)
	}
}

func qtHandlePresentTicket(ctx *core.ConversationContext, wf *core.WorkflowContext, sourceAction *core.Action) ([]*core.Action, error) {
	toolResult := getInput(sourceAction, core.InputToolResult)
	aiResponse := getInput(sourceAction, core.InputAIResponse)

	if toolResult != nil {
		// First time — store ticket and show it
		ticket, ok := toolResult.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("queryTicket presentTicket: invalid tool result type")
		}
		ticketJSON, _ := json.Marshal(ticket)
		wf.SetWorkflowData(qtKeyFoundTicketJSON, string(ticketJSON))

		msg := qtFormatTicket(ticket)
		wf.SetWorkflowData(qtKeyPendingStep, "qt_present_ticket")

		msgAction := core.NewAction(core.ActionUserMessage)
		msgAction.Input = map[core.InputType]any{core.InputMessage: msg}
		return []*core.Action{msgAction, qtWaitAction("Is this the one? Feel free to ask me anything about it.")}, nil
	}

	if aiResponse != nil {
		// Follow-up intent response
		response := aiResponse.(*ai.Response)
		data := response.Data()
		intent, _ := data.GetString("intent")

		logger.Debugf("🎫 QueryTicket presentTicket follow-up: intent=%q", intent)

		switch intent {
		case "answer_question":
			answer, _ := data.GetString("answer")
			wf.SetWorkflowData(qtKeyPendingStep, "qt_present_ticket")
			return []*core.Action{qtWaitAction(answer)}, nil

		case "wrong_ticket":
			ticketJSON, _ := wf.GetWorkflowData(qtKeyFoundTicketJSON).(string)
			var ticket map[string]any
			if err := json.Unmarshal([]byte(ticketJSON), &ticket); err == nil {
				if idFloat, ok := ticket["id"].(float64); ok {
					qtAddRejectedID(wf, int(idFloat))
				}
			}
			wf.SetWorkflowData(qtKeyFoundTicketJSON, "")
			wf.SetWorkflowData(qtKeyPendingStep, "")

			msgAction := core.NewAction(core.ActionUserMessage)
			msgAction.Input = map[core.InputType]any{
				core.InputMessage: "Got it, that's not the one — let me keep searching.",
			}
			messages := ctx.GetLastUserMessages()
			var latestMsg string
			if len(messages) > 0 {
				latestMsg = messages[len(messages)-1].Message
			}
			planActions, err := qtPlanSearch(wf, latestMsg)
			if err != nil {
				return nil, err
			}
			return append([]*core.Action{msgAction}, planActions...), nil

		case "new_ticket_search":
			wf.ResetWorkflowData()
			messages := ctx.GetLastUserMessages()
			var latestMsg string
			if len(messages) > 0 {
				latestMsg = messages[len(messages)-1].Message
			}
			schema := buildQtExtractSchema()
			personality := qtOrchestratorPersonality()
			actions := askAI(latestMsg, "", personality, schema, "qt_main")
			actions[0].Input[core.InputStep] = StepQtClarify
			return actions, nil

		case "done":
			msgAction := core.NewAction(core.ActionUserMessage)
			msgAction.Input = map[core.InputType]any{
				core.InputMessage: "Happy to help! Let me know if you need anything else.",
			}
			return []*core.Action{msgAction}, nil
		}
	}

	return nil, fmt.Errorf("queryTicket presentTicket: called with neither tool result nor AI response")
}

func qtHandleUserAnswer(ctx *core.ConversationContext, wf *core.WorkflowContext, sourceAction *core.Action) ([]*core.Action, error) {
	pendingStep, _ := wf.GetWorkflowData(qtKeyPendingStep).(string)
	messages := ctx.GetLastUserMessages()
	var userMsg string
	if len(messages) > 0 {
		userMsg = messages[len(messages)-1].Message
	}

	logger.Debugf("🎫 QueryTicket userAnswer: pendingStep=%q", pendingStep)

	switch pendingStep {
	case "qt_clarify_asked", "qt_disambiguate", "qt_narrow_down", "qt_refine_asked":
		// User gave more info — re-plan search
		return qtPlanSearch(wf, userMsg)

	case "qt_show_candidates":
		// User responded to candidates list
		candidatesJSON, _ := wf.GetWorkflowData(qtKeyCandidatesJSON).(string)
		wf.SetWorkflowData(qtKeyPendingStep, "")

		prompt := fmt.Sprintf(
			"The user was shown these ticket candidates:\n%s\n\nUser said: %q\n\n"+
				"Determine if they selected one (pick), want to try differently (none), "+
				"or are done (done). If they picked one, identify the ticket_id.",
			candidatesJSON, userMsg,
		)
		schema := buildQtPickSchema()
		personality := qtOrchestratorPersonality()
		actions := askAI(prompt, "", personality, schema, "qt_main")
		actions[0].Input[core.InputStep] = StepQtHandlePick
		return actions, nil

	case "qt_exhausted":
		// User chose an option after exhausted search
		wf.SetWorkflowData(qtKeyPendingStep, "")

		allCandidatesJSON, _ := wf.GetWorkflowData(qtKeyAllCandidatesJSON).(string)
		prompt := fmt.Sprintf(
			"The user was told the search was exhausted. User said: %q\n\n"+
				"Available candidates from all searches:\n%s\n\n"+
				"Determine: keep_trying (give more info), show_best (show closest matches), or give_up.",
			userMsg, allCandidatesJSON,
		)
		schema := buildQtExhaustedSchema()
		personality := qtOrchestratorPersonality()
		actions := askAI(prompt, "", personality, schema, "qt_main")
		actions[0].Input[core.InputStep] = StepQtHandlePick
		return actions, nil

	case "qt_present_ticket":
		// Follow-up question about the currently shown ticket
		ticketJSON, _ := wf.GetWorkflowData(qtKeyFoundTicketJSON).(string)
		prompt := fmt.Sprintf(
			"The user is asking about this ticket:\n%s\n\nUser says: %q\n\n"+
				"Determine intent: answer_question (answer from ticket data), "+
				"wrong_ticket (this isn't the right one), new_ticket_search (find a different ticket), or done.",
			ticketJSON, userMsg,
		)
		schema := buildQtFollowUpSchema()
		personality := qtOrchestratorPersonality()
		actions := askAI(prompt, "", personality, schema, "qt_main")
		actions[0].Input[core.InputStep] = StepQtPresentTicket
		return actions, nil

	default:
		// Unknown pending step — treat as side question
		logger.Warnf("⚠️  QueryTicket userAnswer: unknown pendingStep %q, treating as side question", pendingStep)
		return handleSideQuestion(ctx, workflows[WorkflowQueryTicket], sourceAction)
	}
}

// ----- Helpers -----

func qtOrchestratorPersonality() string {
	p := personalities.GetPersonality(personalities.PersonalityQueryTicketOrchestrator)
	if p == nil {
		return ""
	}
	return p.PersonalityPrompt
}

func qtPlanSearch(wf *core.WorkflowContext, latestUserMsg string) ([]*core.Action, error) {
	prompt := qtBuildPlanPrompt(wf, latestUserMsg)
	schema := buildQtPlanSchema()
	personality := qtOrchestratorPersonality()

	actions := askAI(prompt, "", personality, schema, "qt_main")
	actions[0].Input[core.InputStep] = StepQtSpawnWorkers

	msgAction := core.NewAction(core.ActionUserMessage)
	msgAction.Input = map[core.InputType]any{
		core.InputMessage: "Let me search for that...",
	}
	return append([]*core.Action{msgAction}, actions...), nil
}

func qtRefine(ctx *core.ConversationContext, wf *core.WorkflowContext) ([]*core.Action, error) {
	attemptCount, _ := wf.GetWorkflowData(qtKeyAttemptCount).(int)
	attemptCount++
	wf.SetWorkflowData(qtKeyAttemptCount, attemptCount)

	logger.Debugf("🎫 QueryTicket refine: attempt=%d", attemptCount)

	if attemptCount >= 4 {
		allCandidates, _ := wf.GetWorkflowData(qtKeyAllCandidatesJSON).(string)
		var msg string
		if allCandidates != "" {
			msg = "I've tried several searches and can't pin down the exact ticket. What would you like to do?\n" +
				"• Say *keep trying* and share anything else you remember\n" +
				"• Say *show me your best matches* to see what I found so far\n" +
				"• Say *never mind* to stop here"
		} else {
			msg = "I've tried several searches but couldn't find anything matching. What would you like to do?\n" +
				"• Say *keep trying* and give me more details\n" +
				"• Say *never mind* to stop here"
		}
		wf.SetWorkflowData(qtKeyPendingStep, "qt_exhausted")
		return []*core.Action{qtWaitAction(msg)}, nil
	}

	if attemptCount == 3 {
		// Ask a specific clarifying question
		userContext := qtBuildUserContext(wf)
		prompt := fmt.Sprintf(
			"We've tried searching several times without finding the ticket.\n\n%s\n\n"+
				"Ask the user ONE specific, helpful question to better pinpoint the ticket. "+
				"Be specific based on what you know. Do not ask vague questions.",
			userContext,
		)
		schema := buildQtRefineQuestionSchema()
		personality := qtOrchestratorPersonality()
		actions := askAI(prompt, "", personality, schema, "qt_main")
		actions[0].Input[core.InputStep] = StepQtAskRefineQuestion
		return actions, nil
	}

	// Attempt 1–2: auto-retry with different angles
	messages := ctx.GetLastUserMessages()
	var latestMsg string
	if len(messages) > 0 {
		latestMsg = messages[len(messages)-1].Message
	}
	return qtPlanSearch(wf, latestMsg)
}

func qtStoreExtractedInfo(wf *core.WorkflowContext, data *ai.SchemaData) {
	if keywords, err := data.GetArray("keywords"); err == nil {
		parts := make([]string, 0, len(keywords))
		for _, k := range keywords {
			if s, ok := k.(string); ok {
				parts = append(parts, s)
			}
		}
		wf.SetWorkflowData(qtKeyKeywords, strings.Join(parts, ", "))
	}
	if v, err := data.GetString("assigned_to"); err == nil {
		wf.SetWorkflowData(qtKeyAssignedTo, v)
	}
	if v, err := data.GetString("created_by"); err == nil {
		wf.SetWorkflowData(qtKeyCreatedBy, v)
	}
	if v, err := data.GetString("qa_person"); err == nil {
		wf.SetWorkflowData(qtKeyQAPerson, v)
	}
	if v, err := data.GetString("work_item_type"); err == nil {
		wf.SetWorkflowData(qtKeyWorkItemType, v)
	}
	if v, err := data.GetString("state"); err == nil {
		wf.SetWorkflowData(qtKeyState, v)
	}
	if tags, err := data.GetArray("tags"); err == nil {
		b, _ := json.Marshal(tags)
		wf.SetWorkflowData(qtKeyTagsJSON, string(b))
	}
	if v, err := data.GetString("area_path"); err == nil {
		wf.SetWorkflowData(qtKeyAreaPath, v)
	}
	if v, err := data.GetString("iteration_path"); err == nil {
		wf.SetWorkflowData(qtKeyIterationPath, v)
	}
}

func qtBuildUserContext(wf *core.WorkflowContext) string {
	var parts []string

	if k, ok := wf.GetWorkflowData(qtKeyKeywords).(string); ok && k != "" {
		parts = append(parts, "Keywords: "+k)
	}
	if t, ok := wf.GetWorkflowData(qtKeyWorkItemType).(string); ok && t != "" {
		parts = append(parts, "Type: "+t)
	}
	if s, ok := wf.GetWorkflowData(qtKeyState).(string); ok && s != "" {
		parts = append(parts, "State: "+s)
	}
	if a, ok := wf.GetWorkflowData(qtKeyAssignedTo).(string); ok && a != "" {
		parts = append(parts, "Assigned to: "+a)
	}
	if c, ok := wf.GetWorkflowData(qtKeyCreatedBy).(string); ok && c != "" {
		parts = append(parts, "Created by: "+c)
	}
	if qa, ok := wf.GetWorkflowData(qtKeyQAPerson).(string); ok && qa != "" {
		parts = append(parts, "QA person: "+qa)
	}
	if ap, ok := wf.GetWorkflowData(qtKeyAreaPath).(string); ok && ap != "" {
		parts = append(parts, "Area/project: "+ap)
	}
	if ip, ok := wf.GetWorkflowData(qtKeyIterationPath).(string); ok && ip != "" {
		parts = append(parts, "Sprint/iteration: "+ip)
	}

	if len(parts) == 0 {
		return "No specific search criteria extracted yet."
	}
	return strings.Join(parts, "\n")
}

func qtBuildPlanPrompt(wf *core.WorkflowContext, latestUserMsg string) string {
	prompt := "Plan parallel search strategies to find an ADO ticket.\n\n"
	prompt += "## What we know about the ticket\n"
	prompt += qtBuildUserContext(wf) + "\n"

	attemptCount, _ := wf.GetWorkflowData(qtKeyAttemptCount).(int)
	if attemptCount > 0 {
		prompt += fmt.Sprintf(
			"\n## Search Attempt #%d\nPrevious searches did not find the right ticket. "+
				"Try DIFFERENT keyword combinations, synonyms, or angles.\n", attemptCount+1)
	}

	rejectedIDs := qtGetRejectedIDs(wf)
	if len(rejectedIDs) > 0 {
		idsStr := make([]string, len(rejectedIDs))
		for i, id := range rejectedIDs {
			idsStr[i] = fmt.Sprintf("%d", id)
		}
		prompt += fmt.Sprintf("\n## Excluded Ticket IDs\nUser confirmed these are wrong — do NOT search for or include them: %s\n", strings.Join(idsStr, ", "))
	}

	if latestUserMsg != "" {
		prompt += fmt.Sprintf("\n## Latest User Message\n%q\n", latestUserMsg)
	}

	prompt += "\nReturn worker_count and strategies_json."
	return prompt
}

func qtBuildSynthesisPrompt(wf *core.WorkflowContext, workerResults []string, rejectedIDs []int) string {
	prompt := "Synthesize search results from parallel sub-workers to find the best matching ADO ticket.\n\n"
	prompt += "## What the user is looking for\n"
	prompt += qtBuildUserContext(wf) + "\n"

	if len(workerResults) > 0 {
		prompt += "\n## Sub-Worker Results\n"
		for _, result := range workerResults {
			prompt += result + "\n"
		}
	} else {
		prompt += "\n## Sub-Worker Results\nNo candidates found by any worker.\n"
	}

	if len(rejectedIDs) > 0 {
		idsStr := make([]string, len(rejectedIDs))
		for i, id := range rejectedIDs {
			idsStr[i] = fmt.Sprintf("%d", id)
		}
		prompt += fmt.Sprintf("\n## Excluded IDs\nDo NOT include tickets: %s\n", strings.Join(idsStr, ", "))
	}

	prompt += "\nChoose the correct branch and write message_to_user for Slack."
	return prompt
}

func qtGetRejectedIDs(wf *core.WorkflowContext) []int {
	jsonStr, ok := wf.GetWorkflowData(qtKeyRejectedIDsJSON).(string)
	if !ok || jsonStr == "" {
		return nil
	}
	var ids []int
	json.Unmarshal([]byte(jsonStr), &ids) //nolint:errcheck
	return ids
}

func qtAddRejectedID(wf *core.WorkflowContext, id int) {
	ids := qtGetRejectedIDs(wf)
	ids = append(ids, id)
	b, _ := json.Marshal(ids)
	wf.SetWorkflowData(qtKeyRejectedIDsJSON, string(b))
}

func qtFormatTicket(ticket map[string]any) string {
	str := func(key string) string {
		if v, ok := ticket[key].(string); ok {
			return v
		}
		return ""
	}

	id := ""
	if idFloat, ok := ticket["id"].(float64); ok {
		id = fmt.Sprintf("#%.0f", idFloat)
	}

	lines := []string{
		fmt.Sprintf("*%s — %s*", id, str("title")),
		fmt.Sprintf("Type: %s | State: *%s*", str("work_item_type"), str("state")),
		fmt.Sprintf("Assigned to: %s", str("assigned_to")),
	}
	if qa := str("qa_person"); qa != "" && qa != "Unassigned" {
		lines = append(lines, fmt.Sprintf("QA: %s", qa))
	}
	if ap := str("area_path"); ap != "" {
		lines = append(lines, fmt.Sprintf("Area: %s", ap))
	}
	if ip := str("iteration_path"); ip != "" {
		lines = append(lines, fmt.Sprintf("Sprint: %s", ip))
	}
	if url := str("url"); url != "" {
		lines = append(lines, fmt.Sprintf("<%s|View in ADO>", url))
	}
	return strings.Join(lines, "\n")
}

func qtFormatCandidatesList(intro, candidatesJSON string) string {
	if intro == "" {
		intro = "I found a few tickets that might match:"
	}

	var candidates []map[string]any
	if err := json.Unmarshal([]byte(candidatesJSON), &candidates); err != nil || len(candidates) == 0 {
		return intro
	}

	lines := []string{intro}
	for i, c := range candidates {
		id := ""
		if idFloat, ok := c["id"].(float64); ok {
			id = fmt.Sprintf("#%.0f", idFloat)
		}
		title, _ := c["title"].(string)
		state, _ := c["state"].(string)
		assignedTo, _ := c["assigned_to"].(string)
		summary, _ := c["summary"].(string)

		lines = append(lines, fmt.Sprintf("\n*%d. %s — %s*", i+1, id, title))
		lines = append(lines, fmt.Sprintf("   State: %s | Assigned: %s", state, assignedTo))
		if summary != "" {
			lines = append(lines, fmt.Sprintf("   _%s_", summary))
		}
	}
	lines = append(lines, "\nReply with the number, name, or say \"none of these\".")
	return strings.Join(lines, "\n")
}

// sanitizeJSONBackslashes fixes unescaped backslashes that AI sometimes emits in JSON strings.
// In valid JSON, a backslash must be followed by one of: " \ / b f n r t u
// Anything else (e.g. \R, \E) is illegal and causes json.Unmarshal to fail.
var invalidJSONEscape = regexp.MustCompile(`\\([^"\\/bfnrtu])`)

func sanitizeJSONBackslashes(s string) string {
	return invalidJSONEscape.ReplaceAllString(s, `\\$1`)
}

func qtWaitAction(msg string) *core.Action {
	a := core.NewAction(core.ActionUserWait)
	a.Input = map[core.InputType]any{core.InputMessage: msg}
	return a
}

// ----- Schemas -----

func buildQtExtractSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddBool("should_clarify",
			ai.Description("True only if you genuinely cannot start a search without one more piece of info. Default false.")).
		AddString("clarifying_question",
			ai.Description("One optional light clarifying question. Only set if should_clarify=true.")).
		AddArray("keywords", ai.FieldTypeString,
			ai.Required(),
			ai.Description("Title keywords to search for. Extract even partial hints.")).
		AddString("assigned_to",
			ai.Description("Person the ticket is assigned to, if mentioned.")).
		AddString("created_by",
			ai.Description("Person who created the ticket, if mentioned.")).
		AddString("qa_person",
			ai.Description("QA person, if mentioned.")).
		AddString("work_item_type",
			ai.Description("Story, Defect, Tech Debt, Task, or Bug — if mentioned.")).
		AddString("state",
			ai.Description("New, Active, Resolved, or Closed — if mentioned.")).
		AddArray("tags", ai.FieldTypeString,
			ai.Description("Tags, if mentioned.")).
		AddString("area_path",
			ai.Description("Full ADO area path in backslash format (e.g. Enterprise\\RMS). ONLY set when the user explicitly provides the complete path. A short name like 'REI' or 'POS' alone is NOT a valid area path — leave empty and use it as a keyword instead.")).
		AddString("iteration_path",
			ai.Description("Sprint or iteration path, if mentioned."))
}

func buildQtPlanSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddInt("worker_count",
			ai.Required(),
			ai.Range(1, 5),
			ai.Description("How many parallel sub-workers to spawn. 1 for very specific queries, up to 5 for vague ones.")).
		AddString("strategies_json",
			ai.Required(),
			ai.Description(`Valid JSON array of search strategies. Each item must have: worker_id (string), angle (string description), title (string), assigned_to (string), created_by (string), qa_person (string), work_item_type (string), state (string), tags (array of strings), area_path (string), iteration_path (string), max_results (int, default 15). Each worker must try a different angle.`))
}

func buildQtAnalyzeSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("branch",
			ai.Required(),
			ai.Enum("present_ticket", "show_candidates", "disambiguate", "narrow_down", "refine"),
			ai.Description("Which path to take based on results.")).
		AddInt("top_ticket_id",
			ai.Description("Set only when branch=present_ticket. The ID of the confident match.")).
		AddString("candidates_json",
			ai.Description(`JSON array of top 2-3 candidates for show_candidates or disambiguate. Each: {"id":int,"title":string,"state":string,"assigned_to":string,"work_item_type":string,"summary":string}`)).
		AddString("message_to_user",
			ai.Required(),
			ai.Description("What to say to the user. For disambiguate: a targeted question. For narrow_down: explain too many results. For show_candidates: brief intro. For refine: brief explanation."))
}

func buildQtPickSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("action",
			ai.Required(),
			ai.Enum("pick", "none", "done", "keep_trying", "show_best", "give_up"),
			ai.Description("What the user wants to do.")).
		AddInt("ticket_id",
			ai.Description("The ID of the selected ticket. Only set when action=pick."))
}

func buildQtExhaustedSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("action",
			ai.Required(),
			ai.Enum("keep_trying", "show_best", "give_up"),
			ai.Description("What the user chose to do after exhausting search attempts.")).
		AddInt("ticket_id",
			ai.Description("Leave empty."))
}

func buildQtFollowUpSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("intent",
			ai.Required(),
			ai.Enum("answer_question", "wrong_ticket", "new_ticket_search", "done"),
			ai.Description("What the user wants: answer about this ticket, reject it, search for a different ticket, or stop.")).
		AddString("answer",
			ai.Description("The answer to the user's question using the ticket data provided. Only set when intent=answer_question."))
}

func buildQtRefineQuestionSchema() *ai.SchemaBuilder {
	return ai.NewSchema().
		AddString("question",
			ai.Required(),
			ai.Description("One specific, helpful question to ask the user to better pinpoint the ticket."))
}
