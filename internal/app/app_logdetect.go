package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/logdetect"
	"github.com/local/mayak/internal/sound"
	"github.com/local/mayak/internal/trackerlog"
)

func (a *App) startLogDetector(dir string) {
	a.startHideoutDetector(dir)
	a.mu.Lock()
	old := a.logDetector
	oldTracker := a.trackerDetector
	a.logDetector = nil
	a.trackerDetector = nil
	a.mu.Unlock()
	if old != nil {
		old.Close()
	}
	if oldTracker != nil {
		oldTracker.Close()
	}
	if dir == "" {
		return
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return
	}
	snapshot := logdetect.LatestSnapshot(dir)
	d := logdetect.New(dir, a.handleMap, a.handleLogEvent)
	td := trackerlog.New(dir, a.handleTrackerLogEvent)
	// PvE also when it was detected rather than set (effectiveCatalogMode locks).
	pve := a.effectiveCatalogMode(a.GetSettings().GameMode) == "pve"
	a.mu.Lock()
	a.logDetector = d
	a.trackerDetector = td
	a.status.RaidActive = snapshot.Active
	a.status.LastQueueSeconds = snapshot.LastQueueSeconds
	a.status.RaidStartedAt = ""
	a.status.RunThroughAt = ""
	var runThroughAt time.Time
	// The last raid (for a Goons report) after a restart: the logs' latest
	// raid, on the map detected last.
	if !snapshot.Active && !snapshot.LastStartedAt.IsZero() && time.Since(snapshot.LastStartedAt) < 24*time.Hour && a.status.CurrentMap != "" && a.lastRaid == nil {
		a.lastRaid = &GoonRaid{Map: a.status.CurrentMap, StartedAt: snapshot.LastStartedAt.Format(time.RFC3339), Mode: a.status.Tracker.Mode, AccountID: a.status.Tracker.AccountID, ProfileID: a.status.Tracker.ProfileID}
	}
	if snapshot.Active {
		a.status.RaidStartedAt = snapshot.StartedAt.Format(time.RFC3339)
		if pve || snapshot.RunThroughEligible {
			runThroughAt = snapshot.StartedAt.Add(time.Duration(a.settings.RunThroughSeconds) * time.Second)
			a.status.RunThroughAt = runThroughAt.Format(time.RFC3339)
		}
	}
	status, settings := a.status, a.settings
	a.mu.Unlock()
	a.emitStatus(status)
	if !runThroughAt.IsZero() {
		a.scheduleRunThroughAlert(settings, runThroughAt)
	}
	d.Start(a.ctx)
	td.Start(a.ctx)
	go func() { _ = a.discoverTrackerProfiles() }()
}

func (a *App) handleMap(mapName string) {
	if a.quitting.Load() {
		return
	}
	sequence := a.analysisSequence.Load()
	a.mu.RLock()
	logsDirectory := a.settings.LogsDirectory
	a.mu.RUnlock()
	active, _ := logdetect.RaidState(logsDirectory)
	a.mu.Lock()
	if a.status.CurrentMap == mapName && a.status.RaidActive == active {
		a.mu.Unlock()
		return
	}
	// The last position was on the map played before.
	if a.status.CurrentMap != mapName {
		a.status.Position = nil
	}
	a.status.CurrentMap = mapName
	a.status.RaidActive = active
	status := a.status
	settings := a.settings
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	a.addLog("Info", "Raid", fmt.Sprintf("Map detected: %s (active=%t)", mapName, active))
	if active && settings.OpenMapOnRaidStart {
		sent, err := a.runRemoteIfCurrent(sequence, func() error {
			return a.sendMapTargets(settings, mapName, "map")
		})
		if err != nil {
			a.setErrorIfCurrent(sequence, err)
		} else if sent {
			a.recordRemoteCommandIfCurrent(sequence, "map/"+mapName)
		}
	}
}

func (a *App) handleLogEvent(event logdetect.Event) {
	if a.quitting.Load() {
		return
	}
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	switch event.Kind {
	case logdetect.MatchFound:
		a.mu.Lock()
		a.status.LastQueueSeconds = event.QueueSeconds
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
		a.addLog("Info", "Raid", fmt.Sprintf("Match found after %.1f seconds", event.QueueSeconds))
		a.notify(settings, sound.MatchFound)
	case logdetect.RaidStarted:
		started := event.OccurredAt
		if started.IsZero() {
			started = time.Now()
		}
		// PvE also when it was detected rather than set (effectiveCatalogMode locks).
		runThrough := a.effectiveCatalogMode(settings.GameMode) == "pve" || event.RunThroughEligible
		a.mu.Lock()
		a.status.RaidStartedAt = started.Format(time.RFC3339)
		a.status.RunThroughAt = ""
		// The last raid's position is not where the player is now; the new
		// one comes with the raid's first position screenshot.
		a.status.Position = nil
		if runThrough {
			a.status.RunThroughAt = started.Add(time.Duration(settings.RunThroughSeconds) * time.Second).Format(time.RFC3339)
		}
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
		a.addLog("Info", "Raid", "Raid started")
		a.squadDropPosition()
		a.retryMapAfterRaidStart()
		a.notify(settings, sound.RaidStart)
		if runThrough {
			a.scheduleRunThroughAlert(settings, started.Add(time.Duration(settings.RunThroughSeconds)*time.Second))
		} else {
			a.cancelRunThroughAlert()
		}
	case logdetect.RaidExited:
		a.cancelRunThroughAlert()
		a.mu.Lock()
		a.rememberRaidLocked()
		a.status.RaidActive = false
		a.status.RaidStartedAt = ""
		a.status.RunThroughAt = ""
		a.status.Position = nil
		status := a.status
		a.mu.Unlock()
		a.emitEvent("status:update", status)
		a.addLog("Info", "Raid", "Raid ended")
		a.squadDropPosition()
	case logdetect.MenuReached:
		// Back at the menu from a raid (dead or alive: the logs do not say),
		// or at the game's start: each has its own notification.
		if event.FromRaid {
			a.notify(settings, sound.QuestItems)
		} else {
			a.notify(settings, sound.GameStart)
		}
	}
}

// retryMapAfterRaidStart covers the brief window where tarkov.dev has only
// just reconnected and the first map command is accepted by the socket server
// before the browser is ready to receive it.
func (a *App) retryMapAfterRaidStart() {
	go func() {
		timer := time.NewTimer(1500 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-a.ctx.Done():
			return
		case <-timer.C:
		}

		a.mu.RLock()
		settings := a.settings
		mapName := a.status.CurrentMap
		raidActive := a.status.RaidActive
		a.mu.RUnlock()
		if !raidActive || !settings.OpenMapOnRaidStart || mapName == "" {
			return
		}

		sequence := a.analysisSequence.Load()
		sent, err := a.runRemoteIfCurrent(sequence, func() error {
			return a.sendMapTargets(settings, mapName, "map")
		})
		if err != nil {
			a.setErrorIfCurrent(sequence, err)
			return
		}
		if sent {
			a.recordRemoteCommandIfCurrent(sequence, "map/"+mapName)
			a.addLog("Debug", "Remote", "Retried map/"+mapName+" after raid start")
		}
	}()
}

func (a *App) cancelRunThroughAlert() {
	a.mu.Lock()
	cancel := a.runThroughCancel
	a.runThroughCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// scheduleRunThroughAlert plays the run-through alert at at, when the raid is
// still on then. A time already past schedules nothing.
func (a *App) scheduleRunThroughAlert(settings config.Settings, at time.Time) {
	a.mu.Lock()
	previous := a.runThroughCancel
	a.runThroughCancel = nil
	if !settings.SoundsEnabled || !settings.RunThroughSound {
		a.mu.Unlock()
		if previous != nil {
			previous()
		}
		return
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	a.runThroughCancel = cancel
	a.mu.Unlock()
	if previous != nil {
		previous()
	}
	delay := time.Until(at)
	if delay <= 0 {
		return
	}
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		a.mu.RLock()
		current := a.settings
		a.mu.RUnlock()
		active, _ := logdetect.RaidState(current.LogsDirectory)
		if active && !a.quitting.Load() {
			a.notify(current, sound.RunThrough)
		}
	}()
}
