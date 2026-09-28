package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/local/mayak/internal/userdata"
)

// The shell's state is kept in three files and read back as one: its
// preferences key by key, its bookmarks record by record (a removed one a
// tombstone), and this PC's part (tabs, the Host/client role).
func TestBrowserStateSplitsPreferencesBookmarksAndDevice(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	a := &App{}
	first := `{"theme":"dark","bookmarks":[{"id":"a","name":"A","url":"https://a.example/"},{"id":"b","name":"B","url":"https://b.example/"}],"tabs":[{"id":"t"}],"connection":{"mode":"local"}}`
	if err := a.BrowserSave(first); err != nil {
		t.Fatal(err)
	}
	p, _ := browserStatePath()
	var device map[string]json.RawMessage
	b, _ := os.ReadFile(p)
	_ = json.Unmarshal(b, &device)
	if device["theme"] != nil || device["bookmarks"] != nil || device["tabs"] == nil || device["connection"] == nil {
		t.Fatalf("browser.json %s", b)
	}
	// B removed: a tombstone stays, and the state reads without it.
	if err := a.BrowserSave(`{"theme":"dark","bookmarks":[{"id":"a","name":"A","url":"https://a.example/"}],"tabs":[{"id":"t"}],"connection":{"mode":"local"}}`); err != nil {
		t.Fatal(err)
	}
	records, ok, err := userdata.LoadRecords(filepath.Join(filepath.Dir(p), "bookmarks.json"))
	if !ok || err != nil || len(records.Items) != 2 || !records.Items[1].Deleted || records.Items[1].ID != "b" {
		t.Fatalf("bookmarks.json %+v %v %v", records, ok, err)
	}
	got, err := a.BrowserLoad()
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Theme     string              `json:"theme"`
		Bookmarks []map[string]string `json:"bookmarks"`
		Tabs      []map[string]string `json:"tabs"`
		Conn      map[string]string   `json:"connection"`
	}
	if json.Unmarshal([]byte(got), &state) != nil || state.Theme != "dark" || len(state.Bookmarks) != 1 || state.Bookmarks[0]["id"] != "a" || len(state.Tabs) != 1 || state.Conn["mode"] != "local" {
		t.Fatalf("loaded %s", got)
	}
}

// State saved before the split (all in browser.json) reads as it was.
func TestBrowserStateBeforeSplitStillLoads(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	p, _ := browserStatePath()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	old := `{"bookmarks":[{"id":"x","name":"X","url":"https://x.example/"}],"theme":"light","tabs":[]}`
	if err := os.WriteFile(p, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := (&App{}).BrowserLoad()
	var state map[string]json.RawMessage
	if err != nil || json.Unmarshal([]byte(got), &state) != nil || string(state["theme"]) != `"light"` || len(state["bookmarks"]) < 10 {
		t.Fatalf("loaded %s %v", got, err)
	}
}
