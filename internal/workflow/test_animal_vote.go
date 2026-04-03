package workflow

import (
	"bob/definitions/personalities"
	"bob/internal/ai"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"fmt"
	"strings"
)

const (
	WorkflowAnimalVoter  WorkflowName = "animalVoter"
	WorkflowAnimalPicker WorkflowName = "animalPicker"

	StepAnimalLaunchVoters = "animal_launch_voters"
	StepAnimalPickReport   = "animal_pick_report"
	StepAnimalCollect      = "animal_collect"
	StepAnimalPresent      = "animal_present"

	animalVoterCount = 3
	wfKeyAnimals     = "animals"
)

func voterKey(id string) string { return "vote_" + id }

// getVoterIDs reads the dispatched voter ID list from workflow data.
func getVoterIDs(wf *core.WorkflowContext) []string {
	raw := wf.GetWorkflowData("voter_ids")
	ids, _ := raw.([]string)
	return ids
}

// getVoteAnimals reads the animal list from workflow data.
func getVoteAnimals(wf *core.WorkflowContext) []string {
	raw := wf.GetWorkflowData(wfKeyAnimals)
	animals, _ := raw.([]string)
	return animals
}

// AnimalVoter is the main workflow.
//
// Flow:
//  1. StepInit        — asks AI to extract/generate 3 animals from the user's message.
//  2. StepAnimalLaunchVoters — stores the animal list, sends the intro, dispatches
//     animalVoterCount AnimalPicker sub-workflows in parallel.
//  3. StepAnimalCollect — called by each picker when it finishes; tallies once all
//     votes are present and emits the single ActionCompleteAsync.
func AnimalVoter(ctx *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🗳️  AnimalVoter: Entry")
	step := getInput(sourceAction, core.InputStep)
	logger.Debugf("🗳️  AnimalVoter: step=%v", step)

	switch step {
	case StepInit:
		messages := ctx.GetLastUserMessages()
		if len(messages) == 0 {
			return nil, fmt.Errorf("AnimalVoter: no user message found")
		}
		userMessage := messages[len(messages)-1].Message

		schema := ai.NewSchema().
			AddArray("animals", ai.FieldTypeString,
				ai.Required(),
				ai.Description("Exactly 3 unique animal names, lowercased. Extract from the user's message if they specified animals; otherwise invent 3 fun ones."),
				ai.MinItems(3),
				ai.MaxItems(3),
			)

		actions := askAI(
			userMessage,
			personalities.PersonalityAnimalNameExtractor.Render(nil),
			schema,
			"", // main conversation — establishes thread context for side questions
		)
		actions[0].Input[core.InputStep] = StepAnimalLaunchVoters
		return actions, nil

	case StepAnimalLaunchVoters:
		aiResp := getInput(sourceAction, core.InputAIResponse)
		if aiResp == nil {
			return nil, fmt.Errorf("AnimalVoter: expected AI response at %s", StepAnimalLaunchVoters)
		}
		response, ok := aiResp.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("AnimalVoter: invalid AI response type")
		}
		rawAnimals, err := response.Data().GetArray("animals")
		if err != nil {
			return nil, fmt.Errorf("AnimalVoter: failed to get animals: %w", err)
		}

		animals := make([]string, 0, len(rawAnimals))
		for _, a := range rawAnimals {
			if s, ok := a.(string); ok {
				animals = append(animals, strings.ToLower(strings.TrimSpace(s)))
			}
		}
		if len(animals) != animalVoterCount {
			return nil, fmt.Errorf("AnimalVoter: expected %d animals, got %d", animalVoterCount, len(animals))
		}

		voterIDs := make([]string, animalVoterCount)
		for i := range voterIDs {
			voterIDs[i] = fmt.Sprintf("voter_%d", i)
		}

		wf := ctx.GetCurrentWorkflow()
		wf.SetWorkflowData(wfKeyAnimals, animals)
		wf.SetWorkflowData("voter_ids", voterIDs)

		intro := fmt.Sprintf(
			"Starting animal vote! I'll ask %d sub-workers to each pick their favourite from: *%s*. Stand by…",
			animalVoterCount,
			strings.Join(animals, ", "),
		)
		msgAction := core.NewAction(core.ActionUserMessage)
		msgAction.Input = map[core.InputType]any{core.InputMessage: intro}

		var subActions []*core.Action
		for _, id := range voterIDs {
			sub := core.NewAction(core.ActionWorkflow)
			sub.Input = map[core.InputType]any{
				core.InputWorkflowName: WorkflowAnimalPicker,
				core.InputStep:         StepInit,
				core.InputSubWorkerID:  id,
			}
			subActions = append(subActions, sub)
		}
		asyncAction := core.NewAction(core.ActionAsync)
		asyncAction.AsyncActions = subActions

		logger.Debugf("🗳️  AnimalVoter: animals=%v, dispatching voters=%v", animals, voterIDs)
		return []*core.Action{msgAction, asyncAction}, nil

	case StepAnimalCollect:
		wf := ctx.GetCurrentWorkflow()

		// Guard: only the first caller that sees all votes presents the summary.
		if wf.GetWorkflowData("summary_sent") != nil {
			logger.Debug("🗳️  AnimalVoter: summary already sent, skipping")
			return nil, nil
		}

		voterIDs := getVoterIDs(wf)
		for _, id := range voterIDs {
			if wf.GetWorkflowData(voterKey(id)) == nil {
				logger.Debugf("🗳️  AnimalVoter: not all votes in yet (missing %s)", id)
				return nil, nil
			}
		}

		wf.SetWorkflowData("summary_sent", true)

		tally := make(map[string]int)
		var voteLines, reasonLines []string
		for i, id := range voterIDs {
			animal, _ := wf.GetWorkflowData(voterKey(id)).(string)
			tally[animal]++
			voteLines = append(voteLines, fmt.Sprintf("Worker %d chose %s", i+1, animal))
			if reason, _ := wf.GetWorkflowData("reason_" + id).(string); reason != "" {
				reasonLines = append(reasonLines, fmt.Sprintf("Worker %d (%s): %s", i+1, animal, reason))
			}
		}

		winner, maxVotes := "", 0
		for animal, count := range tally {
			if count > maxVotes || (count == maxVotes && animal < winner) {
				winner, maxVotes = animal, count
			}
		}

		prompt := fmt.Sprintf(
			"The animal vote is complete.\n\nVotes:\n%s\n\nReasons:\n%s\n\nWinner: %s with %d vote(s).\n\nPresent these results to the user in a fun and engaging way.",
			strings.Join(voteLines, "\n"),
			strings.Join(reasonLines, "\n"),
			winner, maxVotes,
		)

		schema := ai.NewSchema().
			AddString("message", ai.Required(), ai.Description("Your fun presentation of the animal vote results to the user"))

		// Use the main conversation (empty key) so the results are part of the AI conversation
		// history. Side questions can then reference them naturally without data injection.
		actions := askAI(prompt, personalities.PersonalityAnimalVotePresenter.Render(nil), schema, "")
		actions[0].Input[core.InputWorkflowName] = WorkflowAnimalVoter
		actions[0].Input[core.InputStep] = StepAnimalPresent

		logger.Debugf("🗳️  AnimalVoter: requesting AI to present results, winner=%s", winner)
		return actions, nil

	case StepAnimalPresent:
		aiResp := getInput(sourceAction, core.InputAIResponse)
		if aiResp == nil {
			return nil, fmt.Errorf("AnimalVoter: expected AI response at %s", StepAnimalPresent)
		}
		response, ok := aiResp.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("AnimalVoter: invalid AI response type")
		}
		message, err := response.Data().GetString("message")
		if err != nil {
			return nil, fmt.Errorf("AnimalVoter: failed to get message: %w", err)
		}

		msgAction := core.NewAction(core.ActionUserMessage)
		msgAction.Input = map[core.InputType]any{core.InputMessage: message}

		// Emit exactly one ActionCompleteAsync for the entire fan-out.
		completeAction := core.NewAction(core.ActionCompleteAsync)

		logger.Debug("🗳️  AnimalVoter: results presented via AI, completing async")
		return []*core.Action{msgAction, completeAction}, nil

	default:
		return nil, fmt.Errorf("AnimalVoter: unknown step: %v", step)
	}
}

// AnimalPicker is an internal sub-workflow. Each instance picks one animal from
// the list stored in shared workflow data, writes the result back, then notifies
// the parent at StepAnimalCollect.
func AnimalPicker(ctx *core.ConversationContext, sourceAction *core.Action) ([]*core.Action, error) {
	logger.Debug("🐾 AnimalPicker: Entry")
	step := getInput(sourceAction, core.InputStep)
	id, _ := getInput(sourceAction, core.InputSubWorkerID).(string)
	logger.Debugf("🐾 AnimalPicker: step=%v id=%s", step, id)

	switch step {
	case StepInit:
		wf := ctx.GetCurrentWorkflow()
		animals := getVoteAnimals(wf)
		if len(animals) == 0 {
			return nil, fmt.Errorf("AnimalPicker %s: no animals in workflow data", id)
		}
		animalList := strings.Join(animals, ", ")

		schema := ai.NewSchema().
			AddString("animal", ai.Required(), ai.Description(
				fmt.Sprintf("Your favourite animal. Must be exactly one of: %s", animalList),
			)).
			AddString("reason", ai.Required(), ai.Description(
				"A short personal reason (1-2 sentences) for why you prefer this animal over the others.",
			))

		prompt := fmt.Sprintf(
			"You are voter %s in an animal preference poll. Pick your single favourite animal from this list: %s. Reply with only the animal name, lowercased.",
			id, animalList,
		)

		actions := askAIBranch(prompt, personalities.PersonalityAnimalPicker.Render(nil), schema)
		actions[0].Input[core.InputWorkflowName] = WorkflowAnimalPicker
		actions[0].Input[core.InputStep] = StepAnimalPickReport
		actions[0].Input[core.InputSubWorkerID] = id
		return actions, nil

	case StepAnimalPickReport:
		aiResp := getInput(sourceAction, core.InputAIResponse)
		if aiResp == nil {
			return nil, fmt.Errorf("AnimalPicker %s: expected AI response", id)
		}
		response, ok := aiResp.(*ai.Response)
		if !ok {
			return nil, fmt.Errorf("AnimalPicker %s: invalid AI response type", id)
		}
		animal, err := response.Data().GetString("animal")
		if err != nil {
			return nil, fmt.Errorf("AnimalPicker %s: failed to get animal: %w", id, err)
		}

		animal = strings.ToLower(strings.TrimSpace(animal))

		// Validate against the live animal list from workflow data.
		wf := ctx.GetCurrentWorkflow()
		animals := getVoteAnimals(wf)
		valid := false
		for _, a := range animals {
			if a == animal {
				valid = true
				break
			}
		}
		if !valid {
			fallback := animals[0]
			logger.Warnf("🐾 AnimalPicker %s: AI returned unexpected animal %q, defaulting to %s", id, animal, fallback)
			animal = fallback
		}

		reason, _ := response.Data().GetString("reason")

		logger.Debugf("🐾 AnimalPicker %s: voted for %s (%s)", id, animal, reason)
		wf.SetWorkflowData(voterKey(id), animal)
		wf.SetWorkflowData("reason_"+id, reason)

		voteMsg := core.NewAction(core.ActionUserMessage)
		voteMsg.Input = map[core.InputType]any{
			core.InputMessage: fmt.Sprintf("_Worker %s voted: **%s**_", id, animal),
		}

		notify := core.NewAction(core.ActionWorkflow)
		notify.Input = map[core.InputType]any{
			core.InputWorkflowName: WorkflowAnimalVoter,
			core.InputStep:         StepAnimalCollect,
			core.InputSubWorkerID:  id,
		}

		return []*core.Action{voteMsg, notify}, nil

	default:
		return nil, fmt.Errorf("AnimalPicker %s: unknown step: %v", id, step)
	}
}
