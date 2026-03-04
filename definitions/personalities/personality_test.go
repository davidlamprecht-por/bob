package personalities

import "testing"

func TestGetPersonality_KnownNames(t *testing.T) {
	known := []PersonalityName{
		PersonalityIntentAnalyzer,
		PersonalityQueryTicketOrchestrator,
		PersonalityQueryTicketSearcher,
	}
	for _, name := range known {
		p := GetPersonality(name)
		if p == nil {
			t.Errorf("GetPersonality(%q) returned nil", name)
			continue
		}
		if p.PersonalityPrompt == "" {
			t.Errorf("GetPersonality(%q).PersonalityPrompt is empty", name)
		}
	}
}

func TestGetPersonality_Unknown(t *testing.T) {
	p := GetPersonality("nonexistent_personality")
	if p != nil {
		t.Errorf("expected nil for unknown personality, got %v", p)
	}
}

func TestAvailablePersonalities_Count(t *testing.T) {
	avail := AvailablePersonalities()
	if got, want := len(avail), len(personalities); got != want {
		t.Errorf("AvailablePersonalities() returned %d entries, want %d", got, want)
	}
}
