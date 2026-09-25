package questapi

import (
	"testing"

	"github.com/local/mayak/internal/questmatch"
)

func TestSupplementalPreliminarySurvey(t *testing.T) {
	base := make([]questmatch.Quest, len(supplementalQuests))
	for i, quest := range supplementalQuests {
		base[i] = quest.Quest
	}
	matches := questmatch.Match("Preliminary SUrvey", base)
	if len(matches) == 0 || matches[0].Quest.Name != "Preliminary Survey" || matches[0].Confidence != 1 {
		t.Fatalf("unexpected match: %+v", matches)
	}
}
