package app

import (
	"context"
	"github.com/local/mayak/internal/appdir"
	"os"
	"time"

	"github.com/local/mayak/internal/catalog"
	"github.com/local/mayak/internal/hideoutlog"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/sound"
)

func hideoutDirectory() string {
	dir, _ := appdir.Path("hideout")
	return dir
}
func (a *App) OpenHideoutDiagnostics() error {
	if err := os.MkdirAll(hideoutDirectory(), 0700); err != nil {
		return err
	}
	return openDirectory(hideoutDirectory())
}
func (a *App) RefreshHideout() error {
	if err := a.refreshCatalog(true); err != nil {
		return err
	}
	return a.RefreshTracker()
}
func catalogMode(mode string) string {
	switch mode {
	case "pvp":
		return "regular"
	case "seasonal":
		return "pvp-season"
	case "pve":
		return "pve"
	}
	return ""
}

func (a *App) startHideoutDetector(dir string) {
	a.stopHideoutDetector()
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return
	}
	d := hideoutlog.NewWithContext(dir, a.handleHideoutEvent)
	a.mu.Lock()
	a.hideoutDetector = d
	a.mu.Unlock()
	d.Start(a.ctx)
}
func (a *App) stopHideoutDetector() {
	a.mu.Lock()
	d := a.hideoutDetector
	a.hideoutDetector = nil
	a.mu.Unlock()
	if d != nil {
		d.Close()
	}
}

func (a *App) handleHideoutEvent(parent context.Context, e hideoutlog.Event) {
	if a.quitting.Load() {
		return
	}
	if mode := catalogMode(e.Mode); e.AreaType >= 0 && catalog.ValidMode(mode) {
		ctx, cancel := context.WithTimeout(parent, 60*time.Second)
		for _, s := range a.hideoutStationsFor(ctx, mode) {
			if s.AreaType == e.AreaType {
				e.StationName = s.Name
				break
			}
		}
		cancel()
	}
	if parent.Err() != nil || a.quitting.Load() {
		return
	}
	if !a.hideoutStore.Add(e) {
		return
	}
	a.mu.Lock()
	a.status.Hideout.Events = a.hideoutStore.Events()
	settings := a.settings
	status := a.status
	a.mu.Unlock()
	// The logs replayed at a start bring hundreds of events in a row: the
	// status (with the whole list in it) goes to the window once they pause,
	// not once per event. A live event is shown at once.
	if e.Historical {
		a.emitStatusSoon()
	} else {
		a.emitStatus(status)
	}
	notify := shouldNotifyHideout(e, settings.HideoutErrorNotifications)
	if notify {
		if a.ctx != nil {
			a.emitEvent("hideout:alert", e)
		}
		if settings.SoundsEnabled {
			playNotification(sound.Error, settings.HideoutErrorSoundPath, settings.SoundVolume)
		}
	}
}

// hideoutStationsFor is the catalog's hideout stations of a mode: those
// on show when it is their mode (the catalog refresh keeps them current),
// else a copy kept for ten minutes and dropped when that mode's catalog is
// read again. The logs replayed at a start ask once per event, and each
// answer would be a decode of the catalog otherwise.
func (a *App) hideoutStationsFor(ctx context.Context, mode string) []catalog.HideoutStation {
	// The mode on show: the stations the catalog refresh keeps current.
	a.mu.RLock()
	current, currentMode := a.hideoutStations, a.hideoutCatalogMode
	a.mu.RUnlock()
	if currentMode == mode && current != nil {
		return current
	}
	a.hideoutStationsMu.Lock()
	cached, ok := a.hideoutStationsCache[mode]
	a.hideoutStationsMu.Unlock()
	if ok && time.Since(cached.at) < 10*time.Minute {
		return cached.stations
	}
	stations, err := a.catalogClient.Hideout(ctx, mode)
	if err != nil {
		return cached.stations
	}
	a.hideoutStationsMu.Lock()
	if a.hideoutStationsCache == nil {
		a.hideoutStationsCache = map[string]hideoutStationsEntry{}
	}
	a.hideoutStationsCache[mode] = hideoutStationsEntry{stations: stations, at: time.Now()}
	a.hideoutStationsMu.Unlock()
	return stations
}

type hideoutStationsEntry struct {
	stations []catalog.HideoutStation
	at       time.Time
}

// hideoutSaved is told how each write of the hideout history went (the
// writes happen later, coalesced): a failure shows in the status until a
// later write works.
func (a *App) hideoutSaved(err error) {
	a.mu.Lock()
	if err != nil {
		a.status.Hideout.LastError = "history-save-failed"
	} else if a.status.Hideout.LastError == "history-save-failed" {
		a.status.Hideout.LastError = ""
	}
	a.mu.Unlock()
	if err != nil {
		a.addLog("Warn", "Hideout", "Could not save the hideout history: "+err.Error())
	}
	a.emitStatusSoon()
}

// shouldNotifyHideout alerts on new failed hideout actions. Actions whose
// outcome could not be confirmed are not alerted: nothing can be done
// about them.
func shouldNotifyHideout(e hideoutlog.Event, errors bool) bool {
	return !e.Historical && e.Actionable() && errors
}

// Called under a.mu: publish a single identity's read-only progress with its catalog.
func (a *App) updateHideoutLocked() {
	previous := a.status.Hideout
	current := a.status.Tracker
	next := model.HideoutStatus{State: "waiting-profile", Mode: current.Mode, AccountID: current.AccountID, ProfileID: current.ProfileID, Events: previous.Events, LastError: previous.LastError}
	if !a.settings.TarkovTrackerEnabled {
		next.State = "disabled"
	} else if current.Mode == "" || current.ProfileID == "" || current.AccountID == "" {
		next.State = "waiting-profile"
	} else if a.trackerData.TokenFor(current.AccountID, current.ProfileID, current.Mode) == "" {
		next.State = "missing-token"
	} else if a.hideoutProgress == nil || a.hideoutProgressToken != a.trackerData.TokenFor(current.AccountID, current.ProfileID, current.Mode) {
		next.State = "waiting-progress"
	} else if a.hideoutCatalogMode != catalogMode(current.Mode) {
		next.State = "waiting-catalog"
	} else {
		progress := model.HideoutProgress(a.hideoutStations, a.hideoutProgress)
		next.Stations = progress.Stations
		next.Completed = progress.Completed
		next.Total = progress.Total
		next.State = "ready"
	}
	a.status.Hideout = next
}
