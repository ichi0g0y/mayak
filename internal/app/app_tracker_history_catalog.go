package app

import (
	"context"
	"sort"
	"time"

	"github.com/local/mayak/internal/tracker"
)

// trackerCatalogTaskIDs is the task IDs of a TarkovTracker mode's catalog
// (json.tarkov.dev, the same list TarkovTracker reads): it checks every task
// write against it, and one ID it does not know fails a whole batch. nil when
// the catalog cannot be read.
func (a *App) trackerCatalogTaskIDs(mode string) map[string]bool {
	catalog := catalogMode(mode)
	if catalog == "" || a.catalogClient == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	var tasks struct {
		Data struct {
			Tasks map[string]struct {
				ID string `json:"id"`
			} `json:"tasks"`
		} `json:"data"`
	}
	if err := a.catalogClient.Get(ctx, catalog, "tasks", &tasks); err != nil || len(tasks.Data.Tasks) == 0 {
		return nil
	}
	known := make(map[string]bool, len(tasks.Data.Tasks))
	for _, task := range tasks.Data.Tasks {
		if task.ID != "" {
			known[task.ID] = true
		}
	}
	return known
}

// trackerHistoryUpdates is the batch the task states of past logs are sent
// in, sorted by ID. With known (the catalog's IDs), the IDs not in it are left
// out and returned as skipped: an event task, or one taken out of the game.
// Without it, every state is sent.
func trackerHistoryUpdates(states map[string]string, known map[string]bool) (updates []tracker.TaskUpdate, skipped []string) {
	updates = make([]tracker.TaskUpdate, 0, len(states))
	for taskID, state := range states {
		if known != nil && !known[taskID] {
			skipped = append(skipped, taskID)
			continue
		}
		updates = append(updates, tracker.TaskUpdate{ID: taskID, State: state})
	}
	sort.Slice(updates, func(i, j int) bool { return updates[i].ID < updates[j].ID })
	sort.Strings(skipped)
	return updates, skipped
}
