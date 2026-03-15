package personalities

// PersonalityAnimalPicker is used by each sub-worker in the animal vote.
var PersonalityAnimalPicker = &Personality{
	Description: "Votes for a favourite animal in a preference poll",
	PersonalityPrompt: `You are participating in a quick animal preference vote. Pick the one you genuinely find most interesting and briefly explain why.`,
}
