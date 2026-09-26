package questapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// catalogSource serves a one-task tarkov.dev catalog.
type catalogSource struct{}

func (catalogSource) Get(_ context.Context, _, resource string, target any) error {
	body := `{"data":{}}`
	if resource == "tasks" {
		body = `{"data":{"tasks":{"k":{"id":"t1","name":"Debut"}}}}`
	}
	return json.Unmarshal([]byte(body), target)
}

// slowWiki answers the wiki's category queries after a delay; only the
// "Story chapters" category has a page, "Batya".
type slowWiki struct{ delay time.Duration }

func (w slowWiki) RoundTrip(req *http.Request) (*http.Response, error) {
	time.Sleep(w.delay)
	body := `{"query":{"categorymembers":[]}}`
	if strings.Contains(req.URL.Query().Get("cmtitle"), "Story chapters") {
		body = `{"query":{"categorymembers":[{"title":"Batya","timestamp":"2020-01-01T00:00:00Z"}]}}`
	}
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: req}, nil
}

// The first task list after a start waits for the wiki's first list, so a
// story chapter matches on the first screenshot, not only on the next one.
func TestFirstTaskListWaitsForTheWiki(t *testing.T) {
	c := NewWithSource(catalogSource{}).EnableWiki()
	c.http = &http.Client{Transport: slowWiki{delay: 50 * time.Millisecond}}
	quests, err := c.QuestsForMode(context.Background(), "pve")
	if err != nil {
		t.Fatal(err)
	}
	if !hasQuest(quests, "Batya") {
		t.Fatalf("the first task list has no story chapter: %v", quests)
	}
}

// The wait is bounded: a list whose fetch is under way is used as it is
// once the caller's context ends.
func TestTaskListWaitEndsWithTheContext(t *testing.T) {
	c := NewWithSource(catalogSource{}).EnableWiki()
	c.http = &http.Client{Transport: slowWiki{delay: time.Minute}}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	quests, err := c.QuestsForMode(ctx, "pve")
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("waited %v for the wiki", took)
	}
	if !hasQuest(quests, "Debut") {
		t.Fatalf("the catalog's tasks are missing: %v", quests)
	}
}

func hasQuest(quests []Quest, name string) bool {
	for _, q := range quests {
		if q.Name == name {
			return true
		}
	}
	return false
}
