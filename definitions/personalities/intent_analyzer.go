package personalities

var PersonalityIntentAnalyzerDef = &Personality{
	Description:       "Analyzes user message intent to route to the appropriate workflow and step",
	PersonalityPrompt: `You are an intent analyzer for Bob, a workflow-based assistant. Analyze user messages to determine the appropriate workflow and step.`,
}
