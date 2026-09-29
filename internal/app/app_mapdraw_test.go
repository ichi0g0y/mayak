package app

import (
	"encoding/json"
	"testing"

	"github.com/local/mayak/internal/userdata"
)

func TestMapDrawingKeepsLinesAndTombstones(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	a := &App{}
	if got, err := a.MapDrawingLoad(); err != nil || got != "[]" {
		t.Fatalf("empty load = %q, %v", got, err)
	}
	two := `[{"id":"a","map":"customs","floor":"","c":"#ff3b30","w":5,"p":[[1.5,2],[3,4]]},{"id":"b","map":"customs","floor":"","c":"#ffd60a","w":3,"p":[[5,6]]}]`
	if err := a.MapDrawingSave(two); err != nil {
		t.Fatal(err)
	}
	if err := a.MapDrawingSave(`[{"id":"b","map":"customs","floor":"","c":"#ffd60a","w":3,"p":[[5,6]]}]`); err != nil {
		t.Fatal(err)
	}
	got, err := a.MapDrawingLoad()
	if err != nil {
		t.Fatal(err)
	}
	var lines []struct{ ID string }
	if json.Unmarshal([]byte(got), &lines) != nil || len(lines) != 1 || lines[0].ID != "b" {
		t.Fatalf("load = %s", got)
	}
	p, _ := mapDrawingPath()
	records, _, _ := userdata.LoadRecords(p)
	if len(records.Items) != 2 || !records.Items[0].Deleted {
		t.Fatalf("the removed line is not a tombstone: %+v", records.Items)
	}
	if a.MapDrawingSave(`{"id":"x"}`) == nil {
		t.Fatal("a non-array saved")
	}
}
