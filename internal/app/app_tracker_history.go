
package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/local/mayak/internal/eftdetect"
	"github.com/local/mayak/internal/tracker"
	"github.com/local/mayak/internal/trackerlog"
)

// syncAssignedHistory sends a profile's past logs once a key is assigned to
// it, so the assignment alone brings TarkovTracker up to date: the live sync
// only follows the logs while MAYAK runs. The settings page shows the result
// ("tracker:history"); logs with nothing to send report nothing.
func (a *App) syncAssignedHistory(accountID, profileID, mode string) {
	a.mu.RLock()
	enabled := a.settings.TarkovTrackerEnabled
	a.mu.RUnlock()
	if !enabled {
		return
	}
	// While EFT runs, the sync waits for it to close (watchGame).
	if eftdetect.GameRunning() {
		a.deferHistory(accountID, profileID, mode)
		a.emitEvent("tracker:history", map[string]any{"mode": mode, "profileId": profileID, "deferred": true})
		return
	}
	sent, err := a.SyncTrackerProfileHistory(accountID, profileID, mode)
	if errors.Is(err, errNoTrackerHistory) {
		a.markHistorySynced(accountID, profileID, mode)
		return
	}
	if errors.Is(err, errGameRunning) {
		a.deferHistory(accountID, profileID, mode)
		a.emitEvent("tracker:history", map[string]any{"mode": mode, "profileId": profileID, "deferred": true})
		return
	}
	result := map[string]any{"mode": mode, "profileId": profileID, "sent": sent}
	if err != nil {
		result["error"] = err.Error()
	}
	a.emitEvent("tracker:history", result)
}

// SyncTrackerProfileHistory sends the task states the EFT logs recorded for
// one profile, from its first session on, to its TarkovTracker key: what was
// done before the key was assigned, or while MAYAK was not running (the live
// sync only follows the logs while it runs). A profile is one wipe, so its
// first session is where its progress starts. It returns how many task
// states were sent.
// errNoTrackerHistory: the profile's logs hold nothing to send. After an
// assignment that is no failure, just nothing to report.
var errNoTrackerHistory = errors.New("the EFT logs of this profile have no task changes")

// errGameRunning: past logs are not synced while EFT runs. Its logs are
// still being written and the live sync sends changes meanwhile; a bulk of
// older states arriving after them would set a task back (a restart undone).
var errGameRunning = errors.New("Escape from Tarkov is running; close it to recheck past logs")

func (a *App) SyncTrackerProfileHistory(accountID, profileID, mode string) (int, error) {
	if eftdetect.GameRunning() {
		return 0, errGameRunning
	}
	a.trackerSyncMu.Lock()
	defer a.trackerSyncMu.Unlock()
	a.mu.RLock()
	root := a.settings.LogsDirectory
	enabled := a.settings.TarkovTrackerEnabled
	token := a.trackerData.TokenFor(accountID, profileID, mode)
	a.mu.RUnlock()
	if !enabled {
		return 0, errors.New("TarkovTracker sync is disabled")
	}
	if token == "" {
		return 0, errors.New("no key is assigned to this EFT profile")
	}
	states, sessions := trackerlog.ProfileTaskHistory(root, accountID, profileID, mode)
	if sessions == 0 {
		return 0, errNoTrackerHistory
	}
	updates := make([]tracker.TaskUpdate, 0, len(states))
	for taskID, state := range states {
		updates = append(updates, tracker.TaskUpdate{ID: taskID, State: state})
	}
	sort.Slice(updates, func(i, j int) bool { return updates[i].ID < updates[j].ID })
	if len(updates) == 0 {
		return 0, errNoTrackerHistory
	}
	ctx, cancel := context.WithTimeout(a.ctx, 45*time.Second)
	defer cancel()
	if err := a.trackerClient.SetTasks(ctx, token, updates); err != nil {
		a.addLog("Error", "TarkovTracker", "Historical sync failed: "+err.Error())
		return 0, err
	}
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Synced %d task states from existing logs for %s (%s)", len(updates), maskProfileID(profileID), mode))
	a.markHistorySynced(accountID, profileID, mode)
	a.mu.RLock()
	active := a.status.Tracker.AccountID == accountID && a.status.Tracker.ProfileID == profileID && a.status.Tracker.Mode == mode
	a.mu.RUnlock()
	if active {
		go func() { _ = a.refreshTrackerIdentity(mode, profileID, accountID) }()
	}
	return len(updates), nil
}

// markHistorySynced records that a profile's past logs were synced now, so
// its "Recheck past logs" no longer shows as never done.
func (a *App) markHistorySynced(accountID, profileID, mode string) {
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	a.mu.Unlock()
	if !document.MarkHistorySynced(accountID, profileID, mode, time.Now().UTC().Format(time.RFC3339)) {
		return
	}
	if err := a.trackerStore.Save(document); err != nil {
		a.addLog("Warn", "TarkovTracker", "Could not remember the past-log sync: "+err.Error())
		return
	}
	a.mu.Lock()
	a.trackerData = document
	a.applyTrackerTokenFlagsLocked()
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
}

// deferHistory keeps a profile's past-log sync for when EFT closes.
func (a *App) deferHistory(accountID, profileID, mode string) {
	a.pendingHistoryMu.Lock()
	defer a.pendingHistoryMu.Unlock()
	if a.pendingHistory == nil {
		a.pendingHistory = map[[3]string]bool{}
	}
	a.pendingHistory[[3]string{accountID, profileID, mode}] = true
}

// watchGame follows whether EFT runs, for the settings page (a recheck of
// past logs waits for it to close), and runs the syncs deferred meanwhile
// once it has closed.
func (a *App) watchGame() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		running := eftdetect.GameRunning()
		a.mu.Lock()
		changed := a.status.Tracker.GameRunning != running
		a.status.Tracker.GameRunning = running
		status := a.status
		a.mu.Unlock()
		if changed {
			a.emitStatus(status)
		}
		if !running {
			a.pendingHistoryMu.Lock()
			pending := a.pendingHistory
			a.pendingHistory = nil
			a.pendingHistoryMu.Unlock()
			for profile := range pending {
				a.syncAssignedHistory(profile[0], profile[1], profile[2])
			}
		}
		select {
		case <-a.done:
			return
		case <-ticker.C:
		}
	}
}
