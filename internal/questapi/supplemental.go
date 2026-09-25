package questapi

import "github.com/local/mayak/internal/questmatch"

// supplementalQuests covers newly released tasks while tarkov.dev's public
// JSON catalog catches up. Leave NormalizedName empty so MAYAK uses the map
// fallback instead of navigating tarkov.dev to a task route that does not yet
// exist there.
var supplementalQuests = []Quest{
	{
		Quest: questmatch.Quest{
			ID:     "mayak:preliminary-survey",
			Name:   "Preliminary Survey",
			Trader: "Ref",
			// tarkov.dev's map name (the logs call it laboratory).
			Map: "the-lab",
		},
		Objectives: []Objective{{
			ID:          "mayak:preliminary-survey:blueprint",
			Description: "Locate the testing area blueprint flash drive in The Lab",
			Maps:        []string{"the-lab"},
		}},
	},
}
