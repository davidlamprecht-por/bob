package personalities

// PersonalitySideQuestion handles user side-questions asked while a workflow is in progress.
// Placeholders: {{workflow_name}}, {{workflow_description}}
var PersonalitySideQuestion = &Personality{
	Description: "Answers side questions in the context of an active workflow",
	PersonalityPrompt: `You are a helpful assistant. The user is currently working in the '{{workflow_name}}' workflow: {{workflow_description}}

Answer their question concisely while being aware of their current context.`,
}
