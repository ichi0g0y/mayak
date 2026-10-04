package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/local/mayak/internal/model"
)

func TestQuestPageDestinations(t *testing.T) {
	dev := "https://tarkov.dev/task/debut"
	for _, tc := range []struct{ site, want string }{
		{"", dev}, {"unknown", dev}, {"tarkov-dev", dev},
		{"official-wiki", "https://escapefromtarkov.fandom.com/wiki/Debut"},
		{"japanese-wiki", "https://wikiwiki.jp/eft/Prapor/Debut"},
	} {
		if got := questPageURL(tc.site, "Debut", "Prapor", dev); got != tc.want {
			t.Fatalf("%s: %s", tc.site, got)
		}
	}
	for _, site := range []string{"official-wiki", "japanese-wiki"} {
		parsed, err := url.Parse(questPageURL(site, "A/B #1?", "Prapor", dev))
		if err != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
			t.Fatal("task name changed URL structure", err)
		}
	}
}

func TestNormalizeQuestSites(t *testing.T) {
	for _, tc := range []struct {
		order []string
		first string
		want  []string
	}{
		{nil, "", []string{"official-wiki", "japanese-wiki", "tarkov-dev"}},
		// Settings from before the order: the one site goes first.
		{nil, "japanese-wiki", []string{"japanese-wiki", "official-wiki", "tarkov-dev"}},
		{nil, "tarkov-dev", []string{"tarkov-dev", "official-wiki", "japanese-wiki"}},
		{[]string{"tarkov-dev", "x", "tarkov-dev"}, "", []string{"tarkov-dev", "official-wiki", "japanese-wiki"}},
		{[]string{"japanese-wiki", "official-wiki", "tarkov-dev"}, "japanese-wiki", []string{"japanese-wiki", "official-wiki", "tarkov-dev"}},
		// A version that knows one site sets it alone.
		{[]string{"japanese-wiki", "official-wiki", "tarkov-dev"}, "tarkov-dev", []string{"tarkov-dev", "japanese-wiki", "official-wiki"}},
	} {
		if got := normalizeQuestSites(tc.order, tc.first); !slices.Equal(got, tc.want) {
			t.Fatalf("%v %q: %v", tc.order, tc.first, got)
		}
	}
}

// fakeWiki answers HEAD requests: the pages listed are there, the others not;
// a host listed as down does not answer.
type fakeWiki struct {
	pages map[string]bool
	down  string
	asked *int
}

func (f fakeWiki) RoundTrip(req *http.Request) (*http.Response, error) {
	*f.asked++
	if req.URL.Host == f.down {
		return nil, errors.New("down")
	}
	status := http.StatusNotFound
	if f.pages[req.URL.String()] {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}, Request: req}, nil
}

func TestQuestSiteForFallsBack(t *testing.T) {
	saved := questPages
	t.Cleanup(func() { questPages = saved })
	asked := 0
	wiki := fakeWiki{pages: map[string]bool{"https://wikiwiki.jp/eft/Prapor/All-Inclusive%20Support": true}, asked: &asked}
	questPages = &questPageChecker{client: &http.Client{Transport: wiki}, known: map[string]questPageCheck{}}
	// A task only the wiki knows: no tarkov.dev page, none on the official
	// wiki yet, one on the Japanese wiki.
	urls := questSiteURLs(model.Status{LastQuest: "All-Inclusive Support", QuestTrader: "Prapor"})
	if got := questSiteFor(context.Background(), []string{"tarkov-dev", "official-wiki", "japanese-wiki"}, urls); got != "japanese-wiki" {
		t.Fatalf("site = %s", got)
	}
	if asked != 2 {
		t.Fatalf("asked %d times", asked)
	}
	// The wikis' answers are kept.
	questSiteFor(context.Background(), []string{"official-wiki", "japanese-wiki"}, urls)
	if asked != 2 {
		t.Fatalf("asked again: %d", asked)
	}
	// No site has it: the first one set.
	urls["japanese-wiki"] = "https://wikiwiki.jp/eft/Prapor/Missing"
	if got := questSiteFor(context.Background(), []string{"official-wiki", "japanese-wiki", "tarkov-dev"}, urls); got != "official-wiki" {
		t.Fatalf("none: %s", got)
	}
	// A wiki that does not answer is taken as having the page.
	questPages = &questPageChecker{client: &http.Client{Transport: fakeWiki{down: "escapefromtarkov.fandom.com", asked: &asked}}, known: map[string]questPageCheck{}}
	if got := questSiteFor(context.Background(), []string{"tarkov-dev", "official-wiki"}, urls); got != "official-wiki" {
		t.Fatalf("down: %s", got)
	}
	// tarkov.dev's page is known from the catalog, without asking.
	asked = 0
	urls = questSiteURLs(model.Status{LastQuest: "Debut", QuestURL: "https://tarkov.dev/task/debut"})
	if got := questSiteFor(context.Background(), []string{"tarkov-dev", "official-wiki"}, urls); got != "tarkov-dev" || asked != 0 {
		t.Fatalf("tarkov.dev: %s, asked %d", got, asked)
	}
}
