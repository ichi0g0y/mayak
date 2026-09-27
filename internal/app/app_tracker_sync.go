
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/tracker"
	"github.com/local/mayak/internal/trackerlog"
	"github.com/local/mayak/internal/trackerstore"
)

func (a *App) discoverTrackerProfiles() error {
	a.mu.RLock()
	dir := a.settings.LogsDirectory
	a.mu.RUnlock()
	observed := trackerlog.DiscoverProfiles(dir)
	profiles := make([]trackerstore.Profile, 0, len(observed))
	for _, profile := range observed {
		profiles = append(profiles, trackerstore.Profile{AccountID: profile.AccountID, ProfileID: profile.ProfileID, Mode: profile.Mode, FirstSeen: profile.FirstSeen.Format(time.RFC3339), LastSeen: profile.LastSeen.Format(time.RFC3339)})
	}
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	changed := document.RememberProfiles(profiles)
	a.mu.Unlock()
	if changed {
		if err := a.trackerStore.Save(document); err != nil {
			return err
		}
		a.mu.Lock()
		a.trackerData = document
		a.applyTrackerTokenFlagsLocked()
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
	}
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Found %d EFT profiles in existing logs", len(profiles)))
	return nil
}

func (a *App) rememberTrackerProfile(event trackerlog.Event) {
	seenAt := event.SeenAt
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	}
	now := seenAt.Format(time.RFC3339)
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	a.mu.Unlock()
	if !document.RememberProfiles([]trackerstore.Profile{{AccountID: event.AccountID, ProfileID: event.ProfileID, Mode: event.Mode, FirstSeen: now, LastSeen: now}}) {
		return
	}
	if err := a.trackerStore.Save(document); err != nil {
		a.addLog("Warn", "TarkovTracker", "Could not remember EFT profile: "+err.Error())
		return
	}
	a.mu.Lock()
	a.trackerData = document
	a.mu.Unlock()
}

func (a *App) updateTrackerTokenStatus() {
	a.mu.Lock()
	a.applyTrackerTokenFlagsLocked()
	a.updateTrackerConnectionLocked()
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
}

func (a *App) applyTrackerTokenFlagsLocked() {
	a.status.Tracker.PVPConfigured = false
	a.status.Tracker.PVEConfigured = false
	a.status.Tracker.SeasonalConfigured = false
	a.status.Tracker.Keys = make([]model.TrackerKeySummary, 0, len(a.trackerData.Keys))
	for _, key := range a.trackerData.Keys {
		switch key.Mode {
		case "pvp":
			a.status.Tracker.PVPConfigured = true
		case "pve":
			a.status.Tracker.PVEConfigured = true
		case "seasonal":
			a.status.Tracker.SeasonalConfigured = true
		}
		a.status.Tracker.Keys = append(a.status.Tracker.Keys, model.TrackerKeySummary{ID: key.ID, Name: key.Name, Mode: key.Mode, MaskedToken: maskToken(key.Token), AccountID: key.AccountID, ProfileID: key.ProfileID, Bound: key.IsBound()})
	}
	a.status.Tracker.Profiles = make([]model.TrackerProfileSummary, 0, len(a.trackerData.Profiles))
	for _, profile := range a.trackerData.Profiles {
		boundID := ""
		for _, key := range a.trackerData.Keys {
			if key.Mode == profile.Mode && key.AccountID == profile.AccountID && key.ProfileID == profile.ProfileID {
				boundID = key.ID
				break
			}
		}
		a.status.Tracker.Profiles = append(a.status.Tracker.Profiles, model.TrackerProfileSummary{AccountID: profile.AccountID, ProfileID: profile.ProfileID, Mode: profile.Mode, FirstSeen: profile.FirstSeen, LastSeen: profile.LastSeen, BoundKeyID: boundID, HistorySyncedAt: profile.HistorySyncedAt, Current: a.status.Tracker.AccountID == profile.AccountID && a.status.Tracker.ProfileID == profile.ProfileID && a.status.Tracker.Mode == profile.Mode})
	}
}

func (a *App) updateTrackerConnectionLocked() {
	a.updateHideoutLocked()
	if !a.settings.TarkovTrackerEnabled {
		a.status.Tracker.Connection = "disabled"
		return
	}
	if a.status.Tracker.Mode == "" {
		a.status.Tracker.Connection = "waiting-profile"
		return
	}
	if a.trackerData.TokenFor(a.status.Tracker.AccountID, a.status.Tracker.ProfileID, a.status.Tracker.Mode) == "" {
		a.status.Tracker.Connection = "missing-token"
		return
	}
	if a.status.Tracker.Connection == "disabled" || a.status.Tracker.Connection == "missing-token" {
		a.status.Tracker.Connection = "connecting"
	}
}

func (a *App) handleTrackerLogEvent(event trackerlog.Event) {
	if a.quitting.Load() {
		return
	}
	switch event.Kind {
	case trackerlog.ProfileDetected:
		a.rememberTrackerProfile(event)
		a.mu.Lock()
		changed := a.status.Tracker.Mode != event.Mode || a.status.Tracker.ProfileID != event.ProfileID || a.status.Tracker.AccountID != event.AccountID
		a.status.Tracker.Mode = event.Mode
		a.status.Tracker.ProfileID = event.ProfileID
		a.status.Tracker.AccountID = event.AccountID
		if changed {
			a.clearTrackerProgressLocked()
			// A mode set by hand that is not the one played leaves the hideout
			// and the item panel's progress empty: say so.
			if set := a.settings.GameMode; set != "" && set != "auto" && catalogMode(event.Mode) != "" && catalogMode(event.Mode) != set {
				defer a.addLog("Warn", "TarkovTracker", "The game mode is set to "+set+" but EFT is playing "+event.Mode+"; progress follows the setting. Set the game mode to auto to follow the game.")
			}
		}
		a.updateHideoutLocked()
		enabled := a.settings.TarkovTrackerEnabled
		a.applyTrackerTokenFlagsLocked()
		tokenConfigured := a.trackerData.TokenFor(event.AccountID, event.ProfileID, event.Mode) != ""
		if !enabled {
			a.status.Tracker.Connection = "disabled"
		} else if !tokenConfigured {
			a.status.Tracker.Connection = "missing-token"
		} else {
			a.status.Tracker.Connection = "connecting"
		}
		// The mode is often found after a raid in progress was restored at
		// startup (or even after it started): a PvE raid gets its timer now.
		var runThroughAt time.Time
		settings := a.settings
		if event.Mode == "pve" && (settings.GameMode == "" || settings.GameMode == "auto") && a.status.RunThroughAt == "" {
			if started, err := time.Parse(time.RFC3339, a.status.RaidStartedAt); err == nil && a.status.RaidStartedAt != "" {
				runThroughAt = started.Add(time.Duration(settings.RunThroughSeconds) * time.Second)
				a.status.RunThroughAt = runThroughAt.Format(time.RFC3339)
			}
		}
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
		if !runThroughAt.IsZero() {
			a.scheduleRunThroughAlert(settings, runThroughAt)
		}
		if changed {
			a.addLog("Info", "TarkovTracker", fmt.Sprintf("EFT profile detected: %s (%s)", maskProfileID(event.ProfileID), event.Mode))
			go func() { _ = a.refreshCatalog(false) }()
		}
		if enabled && tokenConfigured {
			go func() { _ = a.refreshTrackerIdentity(event.Mode, event.ProfileID, event.AccountID) }()
		}
	case trackerlog.TaskChanged:
		a.mu.RLock()
		enabled := a.settings.TarkovTrackerEnabled
		mode := a.status.Tracker.Mode
		profileID := a.status.Tracker.ProfileID
		accountID := a.status.Tracker.AccountID
		token := a.trackerData.TokenFor(accountID, profileID, mode)
		a.mu.RUnlock()
		if !enabled || mode == "" || profileID == "" || token == "" {
			a.addLog("Debug", "TarkovTracker", "Ignored task event because its profile token is not active")
			return
		}
		go a.syncTrackerTask(mode, profileID, accountID, token, event.TaskID, event.TaskState)
	}
}

func (a *App) refreshTrackerMode(mode string) error {
	a.mu.RLock()
	profileID := a.status.Tracker.ProfileID
	accountID := a.status.Tracker.AccountID
	a.mu.RUnlock()
	return a.refreshTrackerIdentity(mode, profileID, accountID)
}

func (a *App) refreshTrackerIdentity(mode, profileID, accountID string) error {
	a.trackerSyncMu.Lock()
	defer a.trackerSyncMu.Unlock()
	a.mu.RLock()
	if a.status.Tracker.Mode != mode || a.status.Tracker.ProfileID != profileID || a.status.Tracker.AccountID != accountID {
		a.mu.RUnlock()
		return errors.New("EFT profile changed before TarkovTracker refresh")
	}
	token := a.trackerData.TokenFor(accountID, profileID, mode)
	enabled := a.settings.TarkovTrackerEnabled
	a.mu.RUnlock()
	if !enabled {
		return errors.New("TarkovTracker sync is disabled")
	}
	if token == "" {
		return errors.New("no TarkovTracker token is configured for " + mode)
	}
	a.setTrackerConnection("connecting", "")
	ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
	defer cancel()
	progress, err := a.trackerClient.Progress(ctx, token)
	if err != nil {
		a.setTrackerConnection("error", err.Error())
		a.addLog("Error", "TarkovTracker", "Progress refresh failed: "+err.Error())
		return err
	}
	if tracker.Mode(progress.Meta.GameMode) != tracker.Mode(mode) {
		err = fmt.Errorf("TarkovTracker progress belongs to %s, not %s", progress.Meta.GameMode, mode)
		a.setTrackerConnection("error", err.Error())
		return err
	}
	completed, failed := 0, 0
	taskStates := make(map[string]string, len(progress.Data.Tasks))
	for _, task := range progress.Data.Tasks {
		if task.Complete {
			completed++
			taskStates[task.ID] = "completed"
		}
		if task.Failed {
			failed++
			taskStates[task.ID] = "failed"
		}
		if _, exists := taskStates[task.ID]; !exists {
			taskStates[task.ID] = "uncompleted"
		}
	}
	a.mu.Lock()
	if !a.settings.TarkovTrackerEnabled || a.trackerData.TokenFor(accountID, profileID, mode) != token || a.status.Tracker.Mode != mode || a.status.Tracker.ProfileID != profileID || a.status.Tracker.AccountID != accountID {
		a.mu.Unlock()
		return errors.New("EFT profile changed during TarkovTracker refresh")
	}
	a.hideoutProgressToken = token
	a.hideoutProgress = nil
	if progress.Data.HideoutModules != nil {
		a.hideoutProgress = make(map[string]bool)
	}
	for _, module := range progress.Data.HideoutModules {
		a.hideoutProgress[module.ID] = module.Complete
	}
	a.updateHideoutLocked()
	a.status.Tracker.Connection = "connected"
	a.status.Tracker.DisplayName = progress.Data.DisplayName
	a.status.Tracker.PlayerLevel = progress.Data.PlayerLevel
	a.status.Tracker.CompletedTasks = completed
	a.status.Tracker.FailedTasks = failed
	a.trackerTasks = taskStates
	a.status.Tracker.LastSync = time.Now().Format(time.RFC3339)
	a.status.Tracker.LastError = ""
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Progress loaded for %s: %d completed", mode, completed))
	return nil
}

func (a *App) syncTrackerTask(mode, profileID, accountID, token, taskID, state string) {
	a.trackerSyncMu.Lock()
	defer a.trackerSyncMu.Unlock()
	a.mu.RLock()
	current := a.settings.TarkovTrackerEnabled && a.status.Tracker.Mode == mode && a.status.Tracker.ProfileID == profileID && a.status.Tracker.AccountID == accountID && a.trackerData.TokenFor(accountID, profileID, mode) == token
	a.mu.RUnlock()
	if !current {
		return
	}
	a.mu.RLock()
	previous := a.trackerTasks[taskID]
	a.mu.RUnlock()
	if state == "uncompleted" && previous != "failed" {
		a.addLog("Debug", "TarkovTracker", "Ignored task-start event because the task was not failed")
		return
	}
	if previous == state {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 35*time.Second)
	defer cancel()
	if err := a.trackerClient.SetTask(ctx, token, taskID, state); err != nil {
		a.setTrackerConnection("error", err.Error())
		a.addLog("Error", "TarkovTracker", fmt.Sprintf("Task %s sync failed: %s", taskID, err))
		return
	}
	a.mu.Lock()
	if a.status.Tracker.Mode != mode || a.status.Tracker.ProfileID != profileID {
		a.mu.Unlock()
		return
	}
	a.status.Tracker.Connection = "connected"
	a.status.Tracker.LastEvent = taskID + "/" + state
	a.status.Tracker.LastSync = time.Now().Format(time.RFC3339)
	a.status.Tracker.LastError = ""
	a.trackerTasks[taskID] = state
	if previous == "completed" {
		a.status.Tracker.CompletedTasks--
	}
	if previous == "failed" {
		a.status.Tracker.FailedTasks--
	}
	if state == "completed" {
		a.status.Tracker.CompletedTasks++
	}
	if state == "failed" {
		a.status.Tracker.FailedTasks++
	}
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Synced task %s as %s (%s)", taskID, state, mode))
}

func (a *App) setTrackerConnection(connection, message string) {
	a.mu.Lock()
	a.status.Tracker.Connection = connection
	a.status.Tracker.LastError = message
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
}
