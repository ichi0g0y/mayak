package userdata

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRecordsSetListStampsChangesAndKeepsTombstones(t *testing.T) {
	t0 := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	var d Records
	d.SetList([]string{"a", "b"}, []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)}, t0)
	if d.SetList([]string{"a", "b"}, []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`2`)}, t0.Add(time.Hour)) {
		t.Fatal("the same list was a change")
	}
	d.SetList([]string{"b"}, []json.RawMessage{json.RawMessage(`3`)}, t0.Add(time.Hour))
	live := d.Live()
	if len(live) != 1 || live[0].ID != "b" || string(live[0].Value) != "3" || !live[0].UpdatedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("live %+v", live)
	}
	// A copy that still has "a" (older) does not bring it back.
	var other Records
	other.SetList([]string{"a"}, []json.RawMessage{json.RawMessage(`1`)}, t0)
	d.Merge(other)
	if live := d.Live(); len(live) != 1 {
		t.Fatalf("removed record came back: %+v", live)
	}
	// Tombstones go after half a year.
	d.SetList([]string{"b"}, []json.RawMessage{json.RawMessage(`3`)}, t0.Add(200*24*time.Hour))
	if len(d.Items) != 1 {
		t.Fatalf("old tombstone kept: %+v", d.Items)
	}
}
