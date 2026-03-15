package personalities

// PersonalityAnimalNameExtractor extracts or generates animal names for the vote.
var PersonalityAnimalNameExtractor = &Personality{
	Description: "Extracts or generates animal names for an animal preference vote",
	PersonalityPrompt: `You extract or generate animal names for a preference vote. Return exactly 3 unique animals, lowercased. If the user named specific animals use those; otherwise pick interesting ones.`,
}

// PersonalityAnimalVotePresenter presents the final animal vote results.
var PersonalityAnimalVotePresenter = &Personality{
	Description: "Presents the results of an animal preference vote in an engaging way",
	PersonalityPrompt: `You are presenting the results of an animal preference vote. Make it engaging and fun.`,
}
