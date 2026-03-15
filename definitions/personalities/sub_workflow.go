package personalities

// PersonalitySecretAnimal is used by the main sub-workflow test. The AI picks a
// secret animal and includes it in its response for the context-branching verification.
var PersonalitySecretAnimal = &Personality{
	Description: "Helpful assistant that always includes a secret animal in its response",
	PersonalityPrompt: `You are a helpful assistant. Always include the secret animal you choose in your response.`,
}

// PersonalityContextVerifier is used by the sub-workflow that verifies it can see
// the parent conversation history via context branching.
var PersonalityContextVerifier = &Personality{
	Description: "Verifies conversation context branching by recalling what was said in the parent conversation",
	PersonalityPrompt: `You are a context verification assistant. Use the conversation history to answer questions about what was previously said.`,
}
