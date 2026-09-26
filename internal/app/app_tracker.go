package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/tracker"
	"github.com/local/mayak/internal/trackerlog"
	"github.com/local/mayak/internal/trackerstore"
)

// TarkovTracker: keys, profiles, progress and the sync of recognized tasks.

// ImportTrackerToken verifies a TarkovTracker token and stores it as a key.
// The key goes straight onto the EFT profile it is for when that is plain:
// the one profile of its mode without a key, or the profile being played.
// It returns that profile's description, or "" when the key waits to be
// assigned by hand.
func (a *App) ImportTrackerToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	mode, ok := tracker.ModeForToken(token)
	if !ok {
		return "", errors.New("expected a PVP_, PVE_, or SZN_ TarkovTracker token")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	info, err := a.trackerClient.TokenInfo(ctx, token)
	if err != nil {
		return "", err
	}
	if tracker.Mode(info.GameMode) != mode {
		return "", fmt.Errorf("TarkovTracker reports this token belongs to %s", info.GameMode)
	}
	if info.Token != "" && info.Token != token {
		return "", errors.New("TarkovTracker returned a different token identity")
	}
	if !tracker.HasPermissions(info, "GP", "WP") {
		return "", errors.New("TarkovTracker token requires Get Progress and Write Progress permissions")
	}
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	key, err := document.AddKey(string(mode), token, info.Note)
	if err != nil {
		a.mu.Unlock()
		return "", err
	}
	current := trackerstore.Profile{AccountID: a.status.Tracker.AccountID, ProfileID: a.status.Tracker.ProfileID, Mode: a.status.Tracker.Mode}
	assigned, assignedTo := autoAssignTrackerKey(&document, key, current)
	a.mu.Unlock()
	if err := a.trackerStore.Save(document); err != nil {
		return "", err
	}
	a.mu.Lock()
	a.trackerData = document
	if assigned && current.AccountID == assignedTo.AccountID && current.ProfileID == assignedTo.ProfileID && current.Mode == assignedTo.Mode {
		a.clearTrackerProgressLocked()
	}
	a.applyTrackerTokenFlagsLocked()
	a.updateTrackerConnectionLocked()
	status := a.status
	active := assigned && status.Tracker.AccountID == assignedTo.AccountID && status.Tracker.ProfileID == assignedTo.ProfileID && status.Tracker.Mode == assignedTo.Mode && a.settings.TarkovTrackerEnabled
	a.mu.Unlock()
	a.emitStatus(status)
	if !assigned {
		a.addLog("Info", "TarkovTracker", "Verified and stored an unassigned "+string(mode)+" key")
		return "", nil
	}
	description := string(mode) + " " + assignedTo.AccountID + " / " + maskProfileID(assignedTo.ProfileID)
	a.addLog("Info", "TarkovTracker", "Verified and stored a "+string(mode)+" key, assigned to "+maskProfileID(assignedTo.ProfileID)+" ("+string(mode)+")")
	if active {
		go func() { _ = a.refreshTrackerMode(string(mode)) }()
	}
	go a.syncAssignedHistory(assignedTo.AccountID, assignedTo.ProfileID, assignedTo.Mode)
	return description, nil
}

// autoAssignTrackerKey puts a new key onto the one profile of its mode that
// has no key, or onto the profile being played (current) when several have
// none and it is one of them. With no such profile the key stays free. Only
// an account's latest profile of the mode counts: the earlier ones are past
// wipes, kept from the logs, and TarkovTracker's progress is the wipe's.
func autoAssignTrackerKey(document *trackerstore.Document, key trackerstore.Key, current trackerstore.Profile) (bool, trackerstore.Profile) {
	var free []trackerstore.Profile
	for _, profile := range latestTrackerProfiles(document.Profiles) {
		if profile.Mode == key.Mode && document.TokenFor(profile.AccountID, profile.ProfileID, profile.Mode) == "" {
			free = append(free, profile)
		}
	}
	target, found := trackerstore.Profile{}, false
	if len(free) == 1 {
		target, found = free[0], true
	} else {
		for _, profile := range free {
			if profile.AccountID == current.AccountID && profile.ProfileID == current.ProfileID && profile.Mode == current.Mode {
				target, found = profile, true
			}
		}
	}
	if !found || document.SetProfileKey(target.AccountID, target.ProfileID, target.Mode, key.ID) != nil {
		return false, trackerstore.Profile{}
	}
	return true, target
}

// latestTrackerProfiles keeps, of each account's profiles of a mode, the
// one seen last in the logs (RFC 3339 times compare as strings).
func latestTrackerProfiles(profiles []trackerstore.Profile) []trackerstore.Profile {
	latest := map[string]trackerstore.Profile{}
	var order []string
	for _, profile := range profiles {
		id := profile.AccountID + "/" + profile.Mode
		if seen, ok := latest[id]; !ok {
			order = append(order, id)
			latest[id] = profile
		} else if profile.LastSeen > seen.LastSeen {
			latest[id] = profile
		}
	}
	out := make([]trackerstore.Profile, 0, len(order))
	for _, id := range order {
		out = append(out, latest[id])
	}
	return out
}

// clearTrackerProgressLocked forgets the TarkovTracker progress loaded for the
// previous identity or key. a.mu must be held.
func (a *App) clearTrackerProgressLocked() {
	a.hideoutProgress = nil
	a.trackerTasks = make(map[string]string)
	a.status.Tracker.DisplayName = ""
	a.status.Tracker.PlayerLevel = 0
	a.status.Tracker.CompletedTasks = 0
	a.status.Tracker.FailedTasks = 0
	a.status.Tracker.LastEvent = ""
	a.status.Tracker.LastSync = ""
	a.status.Tracker.LastError = ""
}

func (a *App) SetTrackerProfileKey(accountID, profileID, mode, keyID string) error {
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	if err := document.SetProfileKey(accountID, profileID, mode, keyID); err != nil {
		a.mu.Unlock()
		return err
	}
	a.mu.Unlock()
	if err := a.trackerStore.Save(document); err != nil {
		return err
	}
	a.mu.Lock()
	a.trackerData = document
	// Another key for the profile being played: its progress is not the old
	// key's.
	if a.status.Tracker.AccountID == accountID && a.status.Tracker.ProfileID == profileID && a.status.Tracker.Mode == mode {
		a.clearTrackerProgressLocked()
	}
	a.applyTrackerTokenFlagsLocked()
	a.updateTrackerConnectionLocked()
	a.status.Tracker.LastError = ""
	status := a.status
	active := status.Tracker.AccountID == accountID && status.Tracker.ProfileID == profileID && status.Tracker.Mode == mode && keyID != "" && a.settings.TarkovTrackerEnabled
	a.mu.Unlock()
	a.emitStatus(status)
	a.addLog("Info", "TarkovTracker", "Updated key assignment for "+maskProfileID(profileID)+" ("+mode+")")
	if active {
		go func() { _ = a.refreshTrackerMode(mode) }()
	}
	if keyID != "" {
		go a.syncAssignedHistory(accountID, profileID, mode)
	}
	return nil
}

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
	sent, err := a.SyncTrackerProfileHistory(accountID, profileID, mode)
	if errors.Is(err, errNoTrackerHistory) {
		return
	}
	result := map[string]any{"mode": mode, "profileId": profileID, "sent": sent}
	if err != nil {
		result["error"] = err.Error()
	}
	a.emitEvent("tracker:history", result)
}

func (a *App) RemoveTrackerKey(keyID string) error {
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	if err := document.RemoveKey(keyID); err != nil {
		a.mu.Unlock()
		return err
	}
	a.mu.Unlock()
	if err := a.trackerStore.Save(document); err != nil {
		return err
	}
	a.mu.Lock()
	a.trackerData = document
	a.applyTrackerTokenFlagsLocked()
	a.updateTrackerConnectionLocked()
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	return nil
}

func (a *App) DiscoverTrackerProfiles() error { return a.discoverTrackerProfiles() }

// SyncTrackerProfileHistory sends the task states the EFT logs recorded for
// one profile, from its first session on, to its TarkovTracker key: what was
// done before the key was assigned, or while MAYAK was not running (the live
// sync only follows the logs while it runs). A profile is one wipe, so its
// first session is where its progress starts. It returns how many task
// states were sent.
// errNoTrackerHistory: the profile's logs hold nothing to send. After an
// assignment that is no failure, just nothing to report.
var errNoTrackerHistory = errors.New("the EFT logs of this profile have no task changes")

func (a *App) SyncTrackerProfileHistory(accountID, profileID, mode string) (int, error) {
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
	points := trackerlog.HistoryBreakpoints(root, accountID, profileID, mode)
	if len(points) == 0 {
		return 0, errNoTrackerHistory
	}
	states, err := trackerlog.TaskHistory(root, points[0].ID, accountID, profileID, mode)
	if err != nil {
		return 0, err
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
	if err = a.trackerClient.SetTasks(ctx, token, updates); err != nil {
		a.addLog("Error", "TarkovTracker", "Historical sync failed: "+err.Error())
		return 0, err
	}
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Synced %d task states from existing logs for %s (%s)", len(updates), maskProfileID(profileID), mode))
	a.mu.RLock()
	active := a.status.Tracker.AccountID == accountID && a.status.Tracker.ProfileID == profileID && a.status.Tracker.Mode == mode
	a.mu.RUnlock()
	if active {
		go func() { _ = a.refreshTrackerIdentity(mode, profileID, accountID) }()
	}
	return len(updates), nil
}

func (a *App) RefreshTracker() error {
	a.mu.RLock()
	mode := a.status.Tracker.Mode
	a.mu.RUnlock()
	if mode == "" {
		return errors.New("EFT profile mode has not been detected from logs")
	}
	return a.refreshTrackerMode(mode)
}

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
		a.status.Tracker.Profiles = append(a.status.Tracker.Profiles, model.TrackerProfileSummary{AccountID: profile.AccountID, ProfileID: profile.ProfileID, Mode: profile.Mode, FirstSeen: profile.FirstSeen, LastSeen: profile.LastSeen, BoundKeyID: boundID, Current: a.status.Tracker.AccountID == profile.AccountID && a.status.Tracker.ProfileID == profile.ProfileID && a.status.Tracker.Mode == profile.Mode})
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

func maskProfileID(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:4] + "…" + value[len(value)-4:]
}

func maskToken(value string) string {
	if len(value) <= 10 {
		return "••••"
	}
	return value[:4] + "••••" + value[len(value)-4:]
}
