// Package personalities handles all the personalities the AI can use
package personalities

import "strings"

type Personality struct {
	Description       string
	PersonalityPrompt string
}

// Render returns the personality prompt with all {{key}} placeholders replaced by the
// provided vars. Pass nil (or an empty map) when the prompt has no placeholders.
func (p *Personality) Render(vars map[string]string) string {
	result := p.PersonalityPrompt
	for key, val := range vars {
		result = strings.ReplaceAll(result, "{{"+key+"}}", val)
	}
	return result
}
type PersonalityName string

const (
	PersonalityIntentAnalyzer  PersonalityName = "intent_analyzer"
	PersonalityNameSideQuestion PersonalityName = "side_question"
	PersonalityNameAnimalNameExtractor PersonalityName = "animal_name_extractor"
	PersonalityNameAnimalVotePresenter PersonalityName = "animal_vote_presenter"
	PersonalityNameAnimalPicker        PersonalityName = "animal_picker"
	PersonalityNameSecretAnimal        PersonalityName = "secret_animal"
	PersonalityNameContextVerifier     PersonalityName = "context_verifier"
)

// List all the personalities from all the different
var personalities = map[PersonalityName]*Personality{
	PersonalityIntentAnalyzer:          PersonalityIntentAnalyzerDef,
	PersonalityNameSideQuestion:        PersonalitySideQuestion,
	PersonalityNameAnimalNameExtractor: PersonalityAnimalNameExtractor,
	PersonalityNameAnimalVotePresenter: PersonalityAnimalVotePresenter,
	PersonalityNameAnimalPicker:        PersonalityAnimalPicker,
	PersonalityNameSecretAnimal:        PersonalitySecretAnimal,
	PersonalityNameContextVerifier:     PersonalityContextVerifier,
}

func GetPersonality(name PersonalityName) *Personality {
	p, ok := personalities[name]
	if !ok {
		return nil
	}
	return p
}

type PersonalityInfo struct {
	Name        PersonalityName
	Description string
}

func AvailablePersonalities() []PersonalityInfo {
	out := make([]PersonalityInfo, 0, len(personalities))
	for name, def := range personalities {
		out = append(out, PersonalityInfo{
			Name:        name,
			Description: def.Description,
		})
	}
	return out
}
