package tool

import (
	"bob/internal/ai"
	"bob/internal/orchestrator/core"
	"fmt"
	"math/rand"
)

var SampleDataArgs = ai.NewSchema().
	AddString("category", ai.Required(),
		ai.Enum("greetings", "weather", "dinner"),
		ai.Description("The category of sample data to retrieve"))

var (
	greetings = []string{
		"Hello there!",
		"Good morning!",
		"Hey, how's it going?",
		"Greetings!",
		"Hi, nice to see you!",
	}

	weatherPatterns = []string{
		"Sunny with clear skies",
		"Partly cloudy with a chance of rain",
		"Overcast and cool",
		"Thunderstorms expected in the afternoon",
		"Light snow showers",
	}

	dinnerPlans = []string{
		"Grilled chicken with roasted vegetables",
		"Spaghetti carbonara",
		"Tacos with all the fixings",
		"Vegetable stir-fry with rice",
		"Homemade pizza night",
	}
)

func SampleData(context *core.ConversationContext, args map[string]any) (map[string]any, error) {
	category, ok := args["category"].(string)
	if !ok {
		return nil, fmt.Errorf("category parameter missing or invalid")
	}

	var result string
	switch category {
	case "greetings":
		result = greetings[rand.Intn(len(greetings))]
	case "weather":
		result = weatherPatterns[rand.Intn(len(weatherPatterns))]
	case "dinner":
		result = dinnerPlans[rand.Intn(len(dinnerPlans))]
	default:
		return nil, fmt.Errorf("unknown category: %s", category)
	}

	return map[string]any{"result": result}, nil
}
