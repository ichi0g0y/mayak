package questapi

import (
	"testing"

	"github.com/local/mayak/internal/questmatch"
)

func TestAppendWikiAddsOnlyUnknownTasks(t *testing.T) {
	quests := []Quest{{Quest: questmatch.Quest{ID: "a", Name: "Debut"}}}
	out := appendWiki(quests, []string{"Debut", "Quests", "To the Light - Trust but Verify"})
	if len(out) != 2 {
		t.Fatalf("got %+v", out)
	}
	added := out[1]
	if added.Name != "To the Light - Trust but Verify" || added.NormalizedName != "" ||
		added.WikiLink != "https://escapefromtarkov.fandom.com/wiki/To_the_Light_-_Trust_but_Verify" {
		t.Fatalf("got %+v", added)
	}
	if r := questmatch.Match("To the Light - Trust but Verify", toMatch(out)); r[0].Quest.ID != added.ID {
		t.Fatalf("match %+v", r[0])
	}
}

func toMatch(quests []Quest) []questmatch.Quest {
	out := make([]questmatch.Quest, len(quests))
	for i, q := range quests {
		out[i] = q.Quest
	}
	return out
}
