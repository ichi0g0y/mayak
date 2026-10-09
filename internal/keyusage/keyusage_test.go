package keyusage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The wordings of the wiki's key pages (2026-10-08), and some that must not
// count.
func TestClassify(t *testing.T) {
	for usage, want := range map[string]Kind{
		"This key has no usage. Room 323 is always unlocked.":                                           AlwaysOpen,
		"This key has no usage. Pinewood hotel room 206 on [[Streets of Tarkov]] is always unlocked.":   AlwaysOpen,
		"This key currently has no use since the safe at the gas station is always unlocked.":           AlwaysOpen,
		"This key does currently not open any lock.":                                                    OpensNothing,
		"This key does not currently open any lock.":                                                    OpensNothing,
		"This keycard does not open any lock.":                                                          OpensNothing,
		"This key does currently not open any safe.":                                                    OpensNothing,
		"This key has no usage.":                                                                        OpensNothing,
		"Unlocks dorm room 118 in the three story dorms on [[Customs]].":                                Unclassified,
		"Unlocks the door to the [[Interchange|EMERCOM]] medical care unit. It does not open the gate.": Unclassified,
		"": Unclassified,
	} {
		if got := Classify(usage); got != want {
			t.Errorf("%q: %q, want %q", usage, got, want)
		}
	}
}

func TestParsePage(t *testing.T) {
	content := "{{Infobox key\n|image = x.png\n|node = 5A0EE30786F774023B6EE08F\n|usage = This key has no usage.\nRoom 216 is always unlocked.\n|weight = 0.01\n}}\nText."
	node, usage := parsePage(content)
	if node != "5a0ee30786f774023b6ee08f" || Classify(usage) != AlwaysOpen {
		t.Fatalf("node %q usage %q", node, usage)
	}
	if node, usage := parsePage("{{Infobox key\n|node = \n}}"); node != "" || usage != "" {
		t.Fatalf("empty page: %q %q", node, usage)
	}
}

// The store reads its cache without the network, fetches every page of both
// categories when enabled (following "continue"), and writes the cache.
func TestStoreFetchAndCache(t *testing.T) {
	pages := map[string][]string{
		"Category:Keys":     {"|node = 5a0ee30786f774023b6ee08f\n|usage = Room 216 is always unlocked.\n|x=1", "|node = 5d08d21286f774736e7c94c3\n|usage = Unlocks a door.\n|x=1"},
		"Category:Keycards": {"|node = 62a9cb937377a65d7b070cef\n|usage = This keycard does not open any lock.\n}}"},
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		list := pages[r.URL.Query().Get("gcmtitle")]
		start := 0
		if r.URL.Query().Get("gcmcontinue") != "" {
			start = 1
		}
		body := map[string]any{}
		if start == 0 && len(list) > 1 {
			body["continue"] = map[string]string{"gcmcontinue": "next", "continue": "gcmcontinue||"}
			list = list[:1]
		} else {
			list = list[start:]
		}
		var out []map[string]any
		for _, content := range list {
			out = append(out, map[string]any{"revisions": []any{map[string]any{"slots": map[string]any{"main": map[string]any{"content": content}}}}})
		}
		body["query"] = map[string]any{"pages": out}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "key-usage.json")
	s := New(path).Enable()
	s.api = server.URL
	s.Warm()
	deadline := time.Now().Add(5 * time.Second)
	for s.Kind("62a9cb937377a65d7b070cef") == Unclassified && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if s.Kind("5a0ee30786f774023b6ee08f") != AlwaysOpen || s.Kind("62a9cb937377a65d7b070cef") != OpensNothing || s.Kind("5d08d21286f774736e7c94c3") != Unclassified {
		t.Fatalf("kinds %v after %d requests", s.kinds, requests)
	}
	if requests != 3 {
		t.Fatalf("%d requests, want 3", requests)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("no cache:", err)
	}
	// A new store, offline, reads the cache.
	if got := New(path).Kind("5a0ee30786f774023b6ee08f"); got != AlwaysOpen {
		t.Fatalf("from the cache: %q", got)
	}
}
