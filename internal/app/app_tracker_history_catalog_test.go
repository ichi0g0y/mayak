package app

import (
	"slices"
	"testing"

	"github.com/local/mayak/internal/tracker"
)

func TestTrackerHistoryUpdatesLeaveOutUnknownTasks(t *testing.T) {
	states := map[string]string{
		"5936d90786f7742b1420ba5b": "completed",
		"6a690bb7f68e3945b5e0fe62": "failed",
		"0000000000000000000000ee": "completed",
	}
	known := map[string]bool{"5936d90786f7742b1420ba5b": true, "6a690bb7f68e3945b5e0fe62": true}
	updates, skipped := trackerHistoryUpdates(states, known)
	want := []tracker.TaskUpdate{{ID: "5936d90786f7742b1420ba5b", State: "completed"}, {ID: "6a690bb7f68e3945b5e0fe62", State: "failed"}}
	if !slices.Equal(updates, want) {
		t.Fatalf("updates = %v, want %v", updates, want)
	}
	if !slices.Equal(skipped, []string{"0000000000000000000000ee"}) {
		t.Fatalf("skipped = %v", skipped)
	}
}

func TestTrackerHistoryUpdatesSendAllWithoutCatalog(t *testing.T) {
	states := map[string]string{"b": "completed", "a": "failed"}
	updates, skipped := trackerHistoryUpdates(states, nil)
	want := []tracker.TaskUpdate{{ID: "a", State: "failed"}, {ID: "b", State: "completed"}}
	if !slices.Equal(updates, want) || len(skipped) != 0 {
		t.Fatalf("updates = %v, skipped = %v", updates, skipped)
	}
}
