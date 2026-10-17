package questapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// growingWiki is a wiki whose Quests category can get new pages. Every page
// is given by Therapist; it counts the requests and the pages read.
type growingWiki struct {
	mu       sync.Mutex
	quests   []string
	requests int
	read     []string
}

func (w *growingWiki) add(title string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.quests = append(w.quests, title)
}

func (w *growingWiki) RoundTrip(req *http.Request) (*http.Response, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.requests++
	query := req.URL.Query()
	var body any
	if titles := query.Get("titles"); titles != "" {
		type page struct {
			Title     string `json:"title"`
			Revisions []any  `json:"revisions"`
		}
		var pages []page
		for _, title := range strings.Split(titles, "|") {
			w.read = append(w.read, title)
			pages = append(pages, page{title, []any{map[string]any{"slots": map[string]any{"main": map[string]any{"content": "|given by =[[Therapist]]"}}}}})
		}
		body = map[string]any{"query": map[string]any{"pages": pages}}
	} else {
		members := []map[string]string{}
		if query.Get("cmtitle") == "Category:Quests" {
			for _, title := range w.quests {
				members = append(members, map[string]string{"title": title})
			}
		}
		body = map[string]any{"query": map[string]any{"categorymembers": members}}
	}
	data, _ := json.Marshal(body)
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(string(data))), Header: http.Header{}, Request: req}, nil
}

// A title that matched no task fetches the wiki's list again: a page made
// since the last fetch is in the next task list, and only its page is read.
func TestRecheckWikiFindsANewPage(t *testing.T) {
	wiki := &growingWiki{quests: []string{"Extending the Operation"}}
	c := NewWithSource(catalogSource{}).EnableWiki()
	c.http = &http.Client{Transport: wiki}
	if _, err := c.QuestsForMode(context.Background(), "pve"); err != nil {
		t.Fatal(err)
	}
	wiki.add("Information Trail")
	// Right after the first fetch, the wiki is not asked again.
	wiki.mu.Lock()
	before := wiki.requests
	wiki.mu.Unlock()
	if c.RecheckWiki(context.Background()) {
		t.Fatal("the list changed without a fetch")
	}
	wiki.mu.Lock()
	if wiki.requests != before {
		t.Fatalf("the wiki was asked again at once: %d requests", wiki.requests-before)
	}
	wiki.read = nil
	wiki.mu.Unlock()

	c.wikiMu.Lock()
	c.wikiStarted = time.Now().Add(-wikiRecheck)
	c.wikiMu.Unlock()
	if !c.RecheckWiki(context.Background()) {
		t.Fatal("the new page did not change the list")
	}
	quests, err := c.QuestsForMode(context.Background(), "pve")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range quests {
		if q.Name == "Information Trail" && q.Trader != "Therapist" {
			t.Fatalf("trader = %q", q.Trader)
		}
	}
	if !hasQuest(quests, "Information Trail") {
		t.Fatalf("the new page is not in the task list: %v", quests)
	}
	wiki.mu.Lock()
	defer wiki.mu.Unlock()
	if len(wiki.read) != 1 || wiki.read[0] != "Information Trail" {
		t.Fatalf("pages read again: %v", wiki.read)
	}
}
