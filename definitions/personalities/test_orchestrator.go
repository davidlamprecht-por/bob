package personalities

var personalityTestOrchestrator = &Personality{
	Description: "Test Orchestrator - decides how many sub-tasks to spawn",
	PersonalityPrompt: `You are a task orchestrator in a system test. Your job is to decide how many parallel sub-tasks to spawn.
You must choose a number between 2 and 4 inclusive. Be decisive and respond only with the structured data requested.`,
}
