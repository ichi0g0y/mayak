package questapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/local/mayak/internal/questmatch"
)

func TestAppendWikiAddsOnlyUnknownTasks(t *testing.T) {
	quests := []Quest{{Quest: questmatch.Quest{ID: "a", Name: "Debut"}}}
	out := appendWiki(quests, []wikiQuest{{Title: "Debut"}, {Title: "Quests"}, {Title: "To the Light - Trust but Verify", Trader: "Mechanic"}})
	if len(out) != 2 {
		t.Fatalf("got %+v", out)
	}
	added := out[1]
	if added.Name != "To the Light - Trust but Verify" || added.Trader != "Mechanic" || added.NormalizedName != "" ||
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

// The wiki's pages say who gives a task; a page without it gives none.
func TestWikiGivers(t *testing.T) {
	c := New()
	c.http = &http.Client{Transport: pagesWiki{}}
	got := c.wikiGivers(context.Background(), []string{"To the Light - False Call", "Tour"})
	if got["To the Light - False Call"] != "Mechanic" || got["Tour"] != "" {
		t.Fatalf("givers %v", got)
	}
}

type pagesWiki struct{}

func (pagesWiki) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"query":{"pages":[{"title":"To the Light - False Call","revisions":[{"slots":{"main":{"content":"{{Infobox quest\n|given by     =[[Mechanic]]\n}}"}}}]},{"title":"Tour","revisions":[{"slots":{"main":{"content":"{{Infobox quest\n|given by =\n}}"}}}]}]}}`
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: req}, nil
}
