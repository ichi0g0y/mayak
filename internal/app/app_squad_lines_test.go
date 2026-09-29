package app

import (
	"testing"

	"github.com/local/mayak/internal/squad"
)

func TestSquadLinesKeptBySquadAndKey(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	a := &App{}
	if got, err := a.SquadLinesLoad("ABCD-1234"); err != nil || got != "" {
		t.Fatalf("empty load = %q, %v", got, err)
	}
	if err := a.SquadLinesSave("abcd1234", `{"lines":[{"id":"x"}]}`); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.SquadLinesLoad("ABCD-1234"); got != `{"lines":[{"id":"x"}]}` {
		t.Fatalf("load = %q", got)
	}
	if got, _ := a.SquadLinesLoad("ZZZZ-9999"); got != "" {
		t.Fatalf("another squad's lines = %q", got)
	}
	if a.SquadLinesSave("ABCD-1234", "not json") == nil {
		t.Fatal("invalid lines saved")
	}
	if key := a.SquadMemberKey(); !squad.ValidKey(key) || a.SquadMemberKey() != key {
		t.Fatalf("member key %q", key)
	}
}
