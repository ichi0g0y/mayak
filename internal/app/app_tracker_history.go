package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/local/mayak/internal/eftdetect"
	"github.com/local/mayak/internal/sound"
	"github.com/local/mayak/internal/tracker"
	"github.com/local/mayak/internal/trackerlog"
)

// syncAssignedHistory sends a profile's past logs once a key is assigned to
// it, so the assignment alone brings TarkovTracker up to date: the live sync
// only follows the logs while MAYAK runs. The settings page shows it running
// and its result ("tracker:history": running, then sent, error, deferred or
// idle); logs with nothing to send report nothing but the end.
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
	// Reading every session takes a while (a minute for a long-played
	// profile): the settings page says so meanwhile.
	a.emitEvent("tracker:history", map[string]any{"mode": mode, "profileId": profileID, "running": true})
	working := "history:" + accountID + "/" + profileID + "/" + mode
	a.toast(Toast{Key: working, Category: ToastWorking, Persistent: true, Message: "toastHistoryWorking", Params: map[string]string{"mode": mode}})
	sent, err := a.SyncTrackerProfileHistory(accountID, profileID, mode)
	a.endToast(working)
	if errors.Is(err, errNoTrackerHistory) {
		a.markHistorySynced(accountID, profileID, mode)
		a.emitEvent("tracker:history", map[string]any{"mode": mode, "profileId": profileID, "idle": true})
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
// first session is where its progress starts, unless a Prestige reset it:
// then the profile's HistoryFrom day is (SetTrackerHistoryFrom). It returns
// how many task states were sent.
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
	from := historyFromTime(a.trackerData.HistoryFrom(accountID, profileID, mode))
	a.mu.RUnlock()
	if !enabled {
		return 0, errors.New("TarkovTracker sync is disabled")
	}
	if token == "" {
		return 0, errors.New("no key is assigned to this EFT profile")
	}
	states, sessions, prestige := trackerlog.ProfileTaskHistory(root, accountID, profileID, mode, from)
	if !prestige.IsZero() {
		a.addLog("Info", "TarkovTracker", fmt.Sprintf("Past logs of %s (%s) are read from its Prestige at %s", maskProfileID(profileID), mode, prestige.Format("2006-01-02 15:04:05")))
		// One taken while MAYAK was not running is known from here.
		a.notePrestige(accountID, profileID, mode, prestige)
	}
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
		a.toast(Toast{Event: "trackerHistory", Category: ToastTracker, Level: "error", Message: "toastHistoryFailed", Params: map[string]string{"mode": mode, "error": err.Error()}})
		return 0, err
	}
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Synced %d task states from existing logs for %s (%s)", len(updates), maskProfileID(profileID), mode))
	a.toast(Toast{Event: "trackerHistory", Category: ToastTracker, Level: "success", Message: "toastHistorySynced", Params: map[string]string{"mode": mode, "n": fmt.Sprint(len(updates))}})
	a.markHistorySynced(accountID, profileID, mode)
	a.mu.RLock()
	active := a.status.Tracker.AccountID == accountID && a.status.Tracker.ProfileID == profileID && a.status.Tracker.Mode == mode
	a.mu.RUnlock()
	if active {
		go func() { _ = a.refreshTrackerIdentity(mode, profileID, accountID) }()
	}
	return len(updates), nil
}

// historyFromTime is the start of a HistoryFrom day in this PC's time (the
// EFT log folders are named in it), zero for none.
func historyFromTime(day string) time.Time {
	from, err := time.ParseInLocation(time.DateOnly, day, time.Local)
	if err != nil {
		return time.Time{}
	}
	return from
}

// SetTrackerHistoryFrom sets the day a profile's past logs are read from
// (YYYY-MM-DD; "" for all of them): after a Prestige, its day, so the tasks
// done before it are not sent again to the reset progress.
func (a *App) SetTrackerHistoryFrom(accountID, profileID, mode, from string) error {
	if from != "" && historyFromTime(from).IsZero() {
		return errors.New("the day must be YYYY-MM-DD")
	}
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	a.mu.Unlock()
	if !document.SetHistoryFrom(accountID, profileID, mode, from) {
		return errors.New("the EFT profile was not found")
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
	return nil
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
// past logs waits for it to close) and the notification of its closing, and
// runs the syncs deferred meanwhile once it has closed.
func (a *App) watchGame() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	first := true
	lastPrestigeRefresh := time.Now()
	for {
		if time.Since(lastPrestigeRefresh) >= prestigeRefreshEvery {
			lastPrestigeRefresh = time.Now()
			go a.refreshPendingPrestige()
		}
		running := eftdetect.GameRunning()
		a.mu.Lock()
		changed := a.status.Tracker.GameRunning != running
		a.status.Tracker.GameRunning = running
		status := a.status
		settings := a.settings
		a.mu.Unlock()
		if changed {
			a.emitStatus(status)
			// Closed while MAYAK watched (not already closed at its start).
			if !running && !first && !a.quitting.Load() {
				a.addLog("Info", "Raid", "Escape from Tarkov closed")
				a.notify(settings, sound.GameExit)
			}
		}
		first = false
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
