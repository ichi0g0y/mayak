package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/local/mayak/internal/tracker"
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

func (a *App) RefreshTracker() error {
	a.mu.RLock()
	mode := a.status.Tracker.Mode
	a.mu.RUnlock()
	if mode == "" {
		return errors.New("EFT profile mode has not been detected from logs")
	}
	return a.refreshTrackerMode(mode)
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
