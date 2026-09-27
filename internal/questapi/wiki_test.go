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
	out := appendWiki(quests, []wikiQuest{{Title: "Debut"}, {Title: "Quests"}, {Title: "Story chapters"}, {Title: "To the Light - Trust but Verify", Trader: "Mechanic"}})
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

// categoryWiki serves the wiki's categories: an old task page in Quests, a
// past event's in Historical content too, and no page content.
type categoryWiki struct{}

func (categoryWiki) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"query":{"pages":[]}}`
	switch req.URL.Query().Get("cmtitle") {
	case "Category:Quests":
		body = `{"query":{"categorymembers":[{"title":"To the Light - Getting Acquainted","timestamp":"2023-01-08T12:17:58Z"},{"title":"Old Event"}]}}`
	case "Category:Historical content":
		body = `{"query":{"categorymembers":[{"title":"Old Event"}]}}`
	case "Category:Event content", "Category:Story chapters":
		body = `{"query":{"categorymembers":[]}}`
	}
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: req}, nil
}

// A task whose page is old still counts (it came to the game later); a past
// event's does not.
func TestWikiTasksKeepOldPages(t *testing.T) {
	c := New()
	c.http = &http.Client{Transport: categoryWiki{}}
	got, err := c.fetchWikiQuestTitles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "To the Light - Getting Acquainted" {
		t.Fatalf("got %+v", got)
	}
}
