package app

import (
	"slices"
	"testing"

	"github.com/local/mayak/internal/tracker"
)

func TestForwardTaskUpdatesNeverSetATaskBack(t *testing.T) {
	var progress tracker.Progress
	for _, task := range []struct {
		id               string
		complete, failed bool
	}{{"done", true, false}, {"lost", false, true}} {
		progress.Data.Tasks = append(progress.Data.Tasks, struct {
			ID       string `json:"id"`
			Complete bool   `json:"complete"`
			Failed   bool   `json:"failed"`
		}{task.id, task.complete, task.failed})
	}
	updates := []tracker.TaskUpdate{
		{ID: "done", State: "uncompleted"}, // done stays done
		{ID: "lost", State: "uncompleted"}, // a failed task restarted
		{ID: "lost", State: "failed"},      // failed already
		{ID: "new", State: "uncompleted"},  // started: nothing to set
		{ID: "new", State: "failed"},
		{ID: "new", State: "completed"},
	}
	got := forwardTaskUpdates(updates, progress)
	want := []tracker.TaskUpdate{{ID: "lost", State: "uncompleted"}, {ID: "new", State: "failed"}, {ID: "new", State: "completed"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
