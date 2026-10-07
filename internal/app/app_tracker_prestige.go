package app

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// A Prestige resets an EFT profile's progress and keeps its ID. TarkovTracker
// is reset on its own website only (its API has no Prestige), so MAYAK marks
// the profile as waiting for it, asks for it on the settings page, and once
// TarkovTracker has been reset sends again what was done since the Prestige:
// the live sync sent that to the progress before the reset, which the reset
// dropped.

// prestigeRefreshEvery is how often the progress of a profile waiting for its
// TarkovTracker reset is read again, to see the reset (1,000 reads a day are
// free).
const prestigeRefreshEvery = 5 * time.Minute

// notePrestige marks a profile as waiting for its TarkovTracker reset after a
// Prestige at at, unless that Prestige is already known. It keeps the number
// of tasks completed on TarkovTracker now, to see the reset by its drop.
func (a *App) notePrestige(accountID, profileID, mode string, at time.Time) {
	a.trackerStoreMu.Lock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	completed := 0
	if tr := a.status.Tracker; tr.AccountID == accountID && tr.ProfileID == profileID && tr.Mode == mode && tr.Connection == "connected" {
		completed = tr.CompletedTasks
	}
	a.mu.Unlock()
	if !document.MarkPrestige(accountID, profileID, mode, at.UTC().Format(time.RFC3339Nano), completed) {
		a.trackerStoreMu.Unlock()
		return
	}
	err := a.trackerStore.Save(document)
	if err == nil {
		a.mu.Lock()
		a.trackerData = document
		a.applyTrackerTokenFlagsLocked()
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
	}
	a.trackerStoreMu.Unlock()
	if err != nil {
		a.addLog("Warn", "TarkovTracker", "Could not remember the Prestige: "+err.Error())
		return
	}
	a.addLog("Warn", "TarkovTracker", fmt.Sprintf("Prestige taken (%s) at %s: reset the progress on TarkovTracker too (its settings, Prestige); MAYAK then sends what was done since", mode, at.Local().Format("2006-01-02 15:04:05")))
	// It stays until TarkovTracker has been reset (clearPrestigePending).
	a.toast(Toast{
		Key: prestigeToastKey(accountID, profileID, mode), Event: "prestige", Category: ToastTracker, Level: "warn", Persistent: true,
		Message: "toastPrestige", Params: map[string]string{"mode": mode, "at": at.Local().Format("2006-01-02 15:04")},
		Actions: []ToastAction{{ID: "trackerPrestige", Label: "toastOpenTrackerPrestige"}},
	})
}

func prestigeToastKey(accountID, profileID, mode string) string {
	return "prestige:" + accountID + "/" + profileID + "/" + mode
}

// checkPrestigeReset sees a profile's TarkovTracker reset in its progress
// just read: the tasks completed there fell to under half of those at the
// Prestige. The profile then waits no more, and what was done since the
// Prestige is sent again.
// A resend still due (one a restart cut short) starts again here, as the
// progress is read at every start.
func (a *App) checkPrestigeReset(accountID, profileID, mode string, completed int) {
	a.mu.RLock()
	before, pending := a.trackerData.PrestigePending(accountID, profileID, mode)
	resync := a.trackerData.PrestigeResync(accountID, profileID, mode)
	a.mu.RUnlock()
	if pending && trackerWasReset(before, completed) {
		a.addLog("Info", "TarkovTracker", fmt.Sprintf("TarkovTracker was reset after the Prestige (%d completed, %d before)", completed, before))
		resync = a.clearPrestigePending(accountID, profileID, mode)
	}
	if resync {
		a.startPrestigeResync(accountID, profileID, mode)
	}
}

// prestigeResyncs are the profiles whose progress since their Prestige is
// being sent again, so a refresh meanwhile does not start it twice.
var prestigeResyncs sync.Map

// startPrestigeResync sends again what was done since a profile's Prestige
// (syncAssignedHistory, which waits for EFT to close); the profile's
// PrestigeResync stays until it succeeds (markHistorySynced).
func (a *App) startPrestigeResync(accountID, profileID, mode string) {
	key := accountID + "/" + profileID + "/" + mode
	if _, running := prestigeResyncs.LoadOrStore(key, true); running {
		return
	}
	go func() {
		defer prestigeResyncs.Delete(key)
		a.syncAssignedHistory(accountID, profileID, mode)
	}()
}

// trackerWasReset tells a TarkovTracker reset by its completed tasks: under
// half of the before (at the Prestige; at least 20 to tell, else the settings
// page tells).
func trackerWasReset(before, completed int) bool {
	return before >= 20 && completed*2 < before
}

// ConfirmTrackerPrestigeReset is the settings page telling that TarkovTracker
// has been reset after a profile's Prestige: what was done since is sent
// again (once EFT has closed).
func (a *App) ConfirmTrackerPrestigeReset(accountID, profileID, mode string) error {
	if !a.clearPrestigePending(accountID, profileID, mode) {
		return errors.New("this EFT profile is not waiting for a TarkovTracker reset")
	}
	a.startPrestigeResync(accountID, profileID, mode)
	return nil
}

func (a *App) clearPrestigePending(accountID, profileID, mode string) bool {
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	a.mu.Unlock()
	if !document.ClearPrestigePending(accountID, profileID, mode) {
		return false
	}
	if err := a.trackerStore.Save(document); err != nil {
		a.addLog("Warn", "TarkovTracker", "Could not save the TarkovTracker reset: "+err.Error())
		return false
	}
	a.mu.Lock()
	a.trackerData = document
	a.applyTrackerTokenFlagsLocked()
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	a.endToast(prestigeToastKey(accountID, profileID, mode))
	return true
}

// refreshPendingPrestige reads the progress of the current profile again
// while it waits for its TarkovTracker reset (watchGame, every
// prestigeRefreshEvery), so the reset is seen without a restart.
func (a *App) refreshPendingPrestige() {
	a.mu.RLock()
	tr := a.status.Tracker
	_, pending := a.trackerData.PrestigePending(tr.AccountID, tr.ProfileID, tr.Mode)
	enabled := a.settings.TarkovTrackerEnabled
	a.mu.RUnlock()
	if pending && enabled && tr.Connection == "connected" {
		_ = a.refreshTrackerIdentity(tr.Mode, tr.ProfileID, tr.AccountID)
	}
}
