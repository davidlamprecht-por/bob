package workflow

import "bob/internal/orchestrator/core"

func getInput(a *core.Action, i core.InputType) any{
	if a.Input == nil {
		return nil
	}
	
	inputVal, ok := a.Input[i]
	if !ok{
		return nil
	}

	return inputVal
}

func askAI(userMsg *string, systemPrompt string, personality string, ) {
	
}
