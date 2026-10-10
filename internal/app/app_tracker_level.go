package app

import (
	"context"
	"fmt"

	"github.com/local/mayak/internal/model"
)

// A level read from the Overall screen that is not above TarkovTracker's is
// kept for its profile (levelWaiting): after a Prestige, the screen is often
// taken before MAYAK has seen TarkovTracker's reset, and its level would be
// lost (2026-10-10: level 3 read at 09:47, the old level still loaded). Once
// a progress read finds TarkovTracker's level below it, it is sent.
type waitingLevel struct {
	accountID, profileID, mode string
	level                      int
}

// keepLevel keeps level for the profile tr is, and reads the progress again
// at once when the profile waits for its Prestige reset, rather than in up to
// prestigeRefreshEvery.
func (a *App) keepLevel(tr model.TrackerStatus, level int) {
	a.mu.Lock()
	a.levelWaiting = waitingLevel{accountID: tr.AccountID, profileID: tr.ProfileID, mode: tr.Mode, level: level}
	_, pending := a.trackerData.PrestigePending(tr.AccountID, tr.ProfileID, tr.Mode)
	a.mu.Unlock()
	a.addLog("Debug", "TarkovTracker", fmt.Sprintf("Level %d from the Overall screen kept: TarkovTracker has %d", level, tr.PlayerLevel))
	if pending {
		go func() { _ = a.refreshTrackerIdentity(tr.Mode, tr.ProfileID, tr.AccountID) }()
	}
}

// sendWaitingLevel sends the kept level once TarkovTracker's level, just read
// for the profile, is below it.
func (a *App) sendWaitingLevel(accountID, profileID, mode string, trackerLevel int) {
	a.mu.Lock()
	w := a.levelWaiting
	due := w.level > trackerLevel && w.accountID == accountID && w.profileID == profileID && w.mode == mode
	if due {
		a.levelWaiting = waitingLevel{}
	}
	settings := a.settings
	a.mu.Unlock()
	if !due {
		return
	}
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("TarkovTracker's level is %d now (reset): sending the level %d read from the Overall screen", trackerLevel, w.level))
	a.trackerRaiseLevel(context.Background(), settings, w.level)
}
