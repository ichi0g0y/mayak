package app

import (
	"context"
	"time"

	"github.com/local/mayak/internal/config"
)

// The tarkov.dev pages open on the registered Remote IDs, counted every 20
// seconds: while one is, the Host is looked at from elsewhere, and the shell
// shows on this PC only what it is set to (state.js receiveOf).

const remoteViewersEvery = 20 * time.Second

func (a *App) watchRemoteViewers() {
	ticker := time.NewTicker(remoteViewersEvery)
	defer ticker.Stop()
	for {
		a.countRemoteViewers()
		select {
		case <-a.done:
			return
		case <-ticker.C:
		}
	}
}

func (a *App) countRemoteViewers() {
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	n := 0
	for _, id := range viewerTargetIDs(settings) {
		ctx, cancel := context.WithTimeout(a.parentContext(), 5*time.Second)
		viewing, _ := a.remoteClient(id).Viewing(ctx)
		cancel()
		if viewing {
			n++
		}
	}
	a.mu.Lock()
	changed := a.status.RemoteViewers != n
	a.status.RemoteViewers = n
	status := a.status
	a.mu.Unlock()
	if changed && a.ctx != nil {
		a.emitEvent("status:update", status)
	}
}

// viewerTargetIDs are the registered Remote IDs, each once.
func viewerTargetIDs(settings config.Settings) []string {
	ids := make([]string, 0, len(settings.RemoteTargets))
	seen := map[string]bool{"": true}
	for _, target := range settings.RemoteTargets {
		if !seen[target.ID] {
			seen[target.ID] = true
			ids = append(ids, target.ID)
		}
	}
	return ids
}
