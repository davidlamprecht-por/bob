// Package personalities handles all the personalities the AI can use
package personalities

type Personality struct {
	Description       string
	PersonalityPrompt string
}
type PersonalityName string

const (
	PersonalityIntentAnalyzer PersonalityName = "intent_analyzer"
)

// List all the personalities from all the different
var personalities = map[PersonalityName]*Personality{
	PersonalityIntentAnalyzer: personalityIntentAnalyzer,
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
