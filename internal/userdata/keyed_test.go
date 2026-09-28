package userdata

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// Two copies merge key by key: the later change wins, the rest stays.
func TestKeyedMergeTakesLaterChanges(t *testing.T) {
	t0 := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	mine := Keyed{Values: map[string]json.RawMessage{}, UpdatedAt: map[string]time.Time{}}
	mine.Set(map[string]json.RawMessage{"a": json.RawMessage(`1`), "b": json.RawMessage(`1`)}, t0)
	theirs := Keyed{Values: map[string]json.RawMessage{}, UpdatedAt: map[string]time.Time{}}
	theirs.Set(map[string]json.RawMessage{"a": json.RawMessage(`2`)}, t0.Add(time.Hour))
	theirs.Set(map[string]json.RawMessage{"b": json.RawMessage(`2`)}, t0.Add(-time.Hour))
	if !mine.Merge(theirs) || string(mine.Values["a"]) != "2" || string(mine.Values["b"]) != "1" {
		t.Fatalf("merged %v", mine.Values)
	}
	// The same value again is no change (its time stays).
	if mine.Set(map[string]json.RawMessage{"b": json.RawMessage(` 1 `)}, t0.Add(2*time.Hour)) || !mine.UpdatedAt["b"].Equal(t0) {
		t.Fatalf("unchanged value restamped: %v", mine.UpdatedAt)
	}
}

func TestKeyedSaveLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "prefs.json")
	if _, ok, err := LoadKeyed(p); ok || err != nil {
		t.Fatalf("missing file: %v %v", ok, err)
	}
	doc, _, _ := LoadKeyed(p)
	doc.Set(map[string]json.RawMessage{"x": json.RawMessage(`"y"`)}, time.Now())
	if err := SaveKeyed(p, doc); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadKeyed(p)
	if !ok || err != nil || string(got.Values["x"]) != `"y"` {
		t.Fatalf("got %+v %v %v", got, ok, err)
	}
}
