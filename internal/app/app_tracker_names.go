package app

import (
	"context"
	"errors"
	"time"

	"github.com/local/mayak/internal/tracker"
)

// RefreshTrackerKeyNames reads metadata for all saved keys, including unassigned keys.
func (a *App) RefreshTrackerKeyNames() error {
	return a.refreshTrackerKeyNames(a.trackerClient.TokenInfo)
}

func (a *App) refreshTrackerKeyNames(fetch func(context.Context, string) (tracker.TokenInfo, error)) error {
	a.trackerNamesMu.Lock()
	defer a.trackerNamesMu.Unlock()
	a.mu.RLock()
	snapshot := a.trackerData.Clone()
	a.mu.RUnlock()
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	names := make(map[string]string)
	var refreshErr error
	for _, key := range snapshot.Keys {
		info, err := fetch(ctx, key.Token)
		if err != nil || info.GameMode != key.Mode || (info.Token != "" && info.Token != key.Token) {
			refreshErr = errors.New("could not refresh some TarkovTracker key names")
			continue
		}
		names[key.Token] = info.Note
	}
	// Merge into the latest document so assignments, additions, and removals made
	// during the request are preserved.
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.RLock()
	document := a.trackerData.Clone()
	a.mu.RUnlock()
	changed := false
	for i, key := range document.Keys {
		if name, ok := names[key.Token]; ok {
			_ = document.RenameKey(key.ID, name)
			changed = changed || document.Keys[i].Name != key.Name
		}
	}
	if !changed {
		return refreshErr
	}
	if err := a.trackerStore.Save(document); err != nil {
		return err
	}
	a.mu.Lock()
	a.trackerData = document
	a.applyTrackerTokenFlagsLocked()
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	return refreshErr
}
