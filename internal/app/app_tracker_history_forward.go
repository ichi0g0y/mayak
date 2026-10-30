package app

import "github.com/local/mayak/internal/tracker"

// forwardTaskUpdates keeps the task states of past logs that move a task
// forward on TarkovTracker, as the live sync does: logs can miss a session,
// and a task can be set on TarkovTracker by hand, so an older state there
// must not take a newer one back. A completed task stays completed; a failed
// one is sent unless it is done or failed already; a started one
// ("uncompleted") only for a failed task, which it restarts.
func forwardTaskUpdates(updates []tracker.TaskUpdate, progress tracker.Progress) []tracker.TaskUpdate {
	completed := make(map[string]bool, len(progress.Data.Tasks))
	failed := make(map[string]bool)
	for _, task := range progress.Data.Tasks {
		completed[task.ID] = task.Complete
		failed[task.ID] = task.Failed
	}
	kept := make([]tracker.TaskUpdate, 0, len(updates))
	for _, update := range updates {
		switch {
		case completed[update.ID]:
			continue
		case update.State == "failed" && failed[update.ID]:
			continue
		case update.State == "uncompleted" && !failed[update.ID]:
			continue
		}
		kept = append(kept, update)
	}
	return kept
}
