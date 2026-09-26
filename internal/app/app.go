package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/local/mayak/internal/adblock"
	"github.com/local/mayak/internal/applog"
	"github.com/local/mayak/internal/autostart"
	"github.com/local/mayak/internal/bossinfo"
	"github.com/local/mayak/internal/browserview"
	"github.com/local/mayak/internal/catalog"
	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/eftdetect"
	"github.com/local/mayak/internal/hideoutlog"
	"github.com/local/mayak/internal/itemapi"
	"github.com/local/mayak/internal/iteminfo"
	"github.com/local/mayak/internal/logdetect"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/questapi"
	"github.com/local/mayak/internal/remote"
	"github.com/local/mayak/internal/screenshotstore"
	"github.com/local/mayak/internal/sound"
	"github.com/local/mayak/internal/tracker"
	"github.com/local/mayak/internal/trackerlog"
	"github.com/local/mayak/internal/trackerstore"
	"github.com/local/mayak/internal/watcher"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	desktop              *application.App
	window               *application.WebviewWindow
	windowPlacementReady atomic.Bool
	browserViews         *browserview.Manager
	// The tray menu, to follow a change of language (tray.go).
	trayMenu                      *application.Menu
	trayShow, trayCheck, trayQuit *application.MenuItem
	popup                         itemPopup
	adblock                       *adblock.Blocker
	browserClient                 atomic.Bool
	browserBackgroundOnce         sync.Once
	hideoutProgressToken          string
	// catalogRefreshes counts the catalog refreshes started (app_catalog.go).
	catalogRefreshes   uint64
	hideoutDetector    *hideoutlog.Detector
	hideoutStore       *hideoutlog.Store
	hideoutStations    []catalog.HideoutStation
	hideoutCatalogMode string
	hideoutProgress    map[string]bool
	ctx                context.Context
	mu                 sync.RWMutex
	settingsWriteMu    sync.Mutex
	screenshotStoreMu  sync.Mutex
	remoteOperationMu  sync.Mutex
	settings           config.Settings
	status             model.Status
	watcher            *watcher.Watcher
	remotes            map[string]*remote.Client
	// The watch on the browser's Remote ID and the commands sent to it
	// (app_browser_remote.go).
	browserRemoteMu   sync.Mutex
	browserRemoteStop chan struct{}
	browserRemoteSent map[string]time.Time
	logDetector       *logdetect.Detector
	trackerDetector   *trackerlog.Detector
	questClient       *questapi.Client
	catalogClient     *catalog.Client
	catalogRefreshMu  sync.Mutex
	itemClient        *itemapi.Client
	itemInfo          *iteminfo.Service
	bossInfo          *bossinfo.Client
	screenshotIndex   *screenshotstore.Index
	// lastRaid is the last raid that ended, and goonReports the raids reported
	// (account|mode|start), for Goons reports.
	lastRaid          *GoonRaid
	goonReports       map[string]bool
	faviconOnce       sync.Once
	favicons          *faviconCache
	trackerClient     *tracker.Client
	trackerStore      trackerstore.Store
	trackerStoreMu    sync.Mutex
	trackerNamesMu    sync.Mutex
	trackerData       trackerstore.Document
	trackerTasks      map[string]string
	trackerSyncMu     sync.Mutex
	logs              *applog.Store
	lastQuestSoundKey string
	lastQuestSoundAt  time.Time
	lastErrorSoundKey string
	lastErrorSoundAt  time.Time
	lastProcessed     config.ProcessedScreenshot
	analysisCancel    context.CancelFunc
	// statusSoon is the pending coalesced status emission (emitStatusSoon).
	statusSoonMu sync.Mutex
	statusSoon   *time.Timer
	// The catalog's hideout stations by mode (hideoutStationsFor).
	hideoutStationsMu    sync.Mutex
	hideoutStationsCache map[string]hideoutStationsEntry
	runThroughCancel     context.CancelFunc
	done                 chan struct{}
	doneOnce             sync.Once
	quitting             atomic.Bool
	analysisSequence     atomic.Uint64
	// update is the newer release being fetched (app_update.go).
	update updateState
}

func NewApp() *App {
	processed, _ := config.LoadProcessedScreenshot()
	data := catalog.New()
	return &App{hideoutStore: hideoutlog.NewStore(filepath.Join(hideoutDirectory(), "events.json")), status: model.Status{Connection: "disconnected", ScreenshotType: "unknown", LastScreenshot: processed.Path, Tracker: model.TrackerStatus{Connection: "disabled"}}, lastProcessed: processed, remotes: make(map[string]*remote.Client), catalogClient: data, questClient: questapi.NewWithSource(data).EnableWiki(), itemClient: itemapi.NewWithSource(data), itemInfo: iteminfo.New(data), bossInfo: bossinfo.New(), screenshotIndex: screenshotstore.NewIndex(screenshotIndexPath()), trackerClient: tracker.New(), trackerData: trackerstore.Empty(), trackerTasks: make(map[string]string), logs: applog.New(500), done: make(chan struct{})}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.addLog("Info", "Application", "MAYAK started")
	if removed, err := autostart.RemoveLegacy(); err != nil {
		a.addLog("Warn", "Application", "Could not remove the RaidLens startup entry: "+err.Error())
	} else if removed {
		a.addLog("Info", "Application", "Removed the RaidLens startup entry")
	}
	loadedSettings, loadErr := config.Load()
	settings := normalizeSettings(loadedSettings)
	ensureBrowserRemoteID(&settings)
	// The window's hooks read the settings concurrently (see main.go).
	a.mu.Lock()
	a.settings = settings
	a.mu.Unlock()
	a.setupBrowserRemote()
	a.setupAdblock()
	startTray(a)
	if document, err := a.trackerStore.Load(); err != nil {
		a.addLog("Warn", "TarkovTracker", "Could not load protected API tokens: "+err.Error())
	} else {
		a.trackerData = document
	}
	a.updateTrackerTokenStatus()
	a.status.Hideout.Events = a.hideoutStore.Events()
	if loadErr == nil && !reflect.DeepEqual(a.settings, loadedSettings) {
		_ = config.Save(a.settings)
	}
	a.browserClient.Store(a.browserStartsAsClient())
	a.startUpdateChecks()

	if a.browserClient.Load() {
		return
	}
	// The bundled Tesseract became the default engine: settings still on the
	// old default move to it once. Windows OCR stays selectable afterwards.
	if a.settings.OCRDefaultRevision < 1 {
		if a.settings.OCREngine == "windows" && !tesseractUnavailable() {
			a.settings.OCREngine = "tesseract"
		}
		a.settings.OCRDefaultRevision = 1
		if loadErr == nil {
			_ = config.Save(a.settings)
		}
	}
	if a.settings.OCREngine == "tesseract" && a.settings.TesseractPath == "" && tesseractUnavailable() {
		a.settings.OCREngine = "windows"
		if loadErr == nil {
			_ = config.Save(a.settings)
		}
	}
	if detected, err := eftdetect.Detect(); err == nil {
		changed := false
		if a.settings.ScreenshotDirectory == "" && detected.ScreenshotDirectory != "" {
			a.settings.ScreenshotDirectory = detected.ScreenshotDirectory
			changed = true
		}
		if a.settings.LogsDirectory == "" && detected.LogsDirectory != "" {
			a.settings.LogsDirectory = detected.LogsDirectory
			changed = true
		}
		if changed && loadErr == nil {
			_ = config.Save(a.settings)
		}
	}
	a.rememberExistingScreenshot()
	if a.settings.AutoStartMonitoring && a.settings.ScreenshotDirectory != "" {
		_ = a.StartMonitoring()
	}
	if len(a.settings.RemoteTargets) > 0 {
		go func() { _ = a.TestRemote() }()
	}
	go func() { _ = a.RefreshTrackerKeyNames() }()
	a.startBrowserHostBackground()

}

func (a *App) shutdown(context.Context) {
	ocr.StopWindowsWorker()
	a.stopHideoutDetector()
	a.addLog("Info", "Application", "MAYAK stopped")
	a.quitting.Store(true)
	a.analysisSequence.Add(1)
	a.doneOnce.Do(func() { close(a.done) })
	a.mu.Lock()
	w, clients, detector, trackerDetector, cancel, runThroughCancel := a.watcher, a.remotes, a.logDetector, a.trackerDetector, a.analysisCancel, a.runThroughCancel
	a.watcher, a.remotes, a.logDetector, a.trackerDetector = nil, make(map[string]*remote.Client), nil, nil
	a.analysisCancel = nil
	a.runThroughCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if runThroughCancel != nil {
		runThroughCancel()
	}
	if w != nil {
		_ = w.Close()
	}
	if detector != nil {
		detector.Close()
	}
	if trackerDetector != nil {
		trackerDetector.Close()
	}
	a.remoteOperationMu.Lock()
	for _, client := range clients {
		_ = client.Close()
	}
	a.remoteOperationMu.Unlock()
	if a.popup.views != nil {
		a.popup.views.Close()
	}
	if a.browserViews != nil {
		a.browserViews.Close()
	}
	// The hideout history is written on a timer; whatever is pending goes now.
	_ = a.hideoutStore.Flush()
	// Last, so nothing above runs on files that are no longer this version's.
	a.applyStagedUpdateOnExit()
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.saveWindow(ctx)
	a.mu.RLock()
	closeToTray := a.settings.CloseToTray
	a.mu.RUnlock()
	if closeToTray && !a.quitting.Load() {
		a.window.Hide()
		return true
	}
	return false
}

var windowStateMu sync.Mutex

func (a *App) saveWindow(ctx context.Context) {
	if !a.windowPlacementReady.Load() {
		return
	}
	// Minimized Win32 windows report the parked (-32000,-32000), 160x28
	// rectangle. Preserve the last usable geometry instead of saving it.
	if a.window.IsMinimised() {
		return
	}
	x, y := a.window.Position()
	width, height := a.window.Size()
	if width < 760 || height < 560 {
		return
	}
	maximized := a.window.IsMaximised()
	screen, _ := a.window.GetScreen()
	windowStateMu.Lock()
	defer windowStateMu.Unlock()
	saved, _ := config.LoadWindow()
	saved = rememberWindowPlacement(saved, x, y, width, height, maximized, screen)
	_ = config.SaveWindow(saved)
}

func (a *App) GetStatus() model.Status { a.mu.RLock(); defer a.mu.RUnlock(); return a.status }

func (a *App) GetLogs() []model.LogEntry { return a.logs.Entries() }

func (a *App) ClearLogs() error {
	if err := a.logs.Clear(); err != nil {
		return err
	}
	if a.hideoutStore != nil {
		if err := a.hideoutStore.Clear(); err != nil {
			return err
		}
		a.mu.Lock()
		a.status.Hideout.Events = a.hideoutStore.Events()
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
	}
	if a.ctx != nil {
		a.emitEvent("log:clear")
	}
	return nil
}

func (a *App) addLog(level, category, message string) {
	entry := a.logs.Add(level, category, message)
	if a.ctx != nil {
		a.emitEvent("log:entry", entry)
	}
}

func (a *App) emitStatus(status model.Status) {
	if a.ctx != nil {
		a.emitEvent("status:update", status)
	}
}

// emitStatusSoon sends the status once, shortly, however often it is
// called meanwhile: for bursts of changes (the hideout logs replayed at a
// start) that would otherwise send the whole status hundreds of times.
func (a *App) emitStatusSoon() {
	a.statusSoonMu.Lock()
	defer a.statusSoonMu.Unlock()
	if a.statusSoon != nil {
		return
	}
	a.statusSoon = time.AfterFunc(300*time.Millisecond, func() {
		a.statusSoonMu.Lock()
		a.statusSoon = nil
		a.statusSoonMu.Unlock()
		a.mu.RLock()
		status := a.status
		a.mu.RUnlock()
		a.emitStatus(status)
	})
}

func (a *App) StartMonitoring() error {
	if a.browserClient.Load() || a.BrowserPlatform() != "windows" {
		return errors.New("monitoring is available in Windows Host mode")
	}
	a.mu.RLock()
	settings := a.settings
	alreadyRunning := a.status.Monitoring && a.watcher != nil
	a.mu.RUnlock()
	if alreadyRunning {
		return nil
	}
	if settings.ScreenshotDirectory == "" {
		err := errors.New("Screenshotsフォルダが未設定です")
		a.addLog("Error", "Watcher", err.Error())
		return err
	}
	if err := a.startWatcher(settings.ScreenshotDirectory); err != nil {
		a.addLog("Error", "Watcher", err.Error())
		return err
	}
	a.startLogDetector(settings.LogsDirectory)
	a.setMonitoring(true)
	a.addLog("Info", "Watcher", "Screenshot monitoring started")
	return nil
}

func (a *App) StopMonitoring() {
	a.stopHideoutDetector()
	a.mu.Lock()
	w := a.watcher
	d := a.logDetector
	td := a.trackerDetector
	runThroughCancel := a.runThroughCancel
	a.watcher = nil
	a.logDetector = nil
	a.trackerDetector = nil
	a.runThroughCancel = nil
	a.mu.Unlock()
	if w != nil {
		_ = w.Close()
	}
	if d != nil {
		d.Close()
	}
	if td != nil {
		td.Close()
	}
	if runThroughCancel != nil {
		runThroughCancel()
	}
	a.setMonitoring(false)
	a.addLog("Info", "Watcher", "Screenshot monitoring stopped")
}

func (a *App) setMonitoring(active bool) {
	a.mu.Lock()
	a.status.Monitoring = active
	status := a.status
	a.mu.Unlock()
	if a.ctx != nil {
		a.emitEvent("status:update", status)
	}
}

func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	// Restore the named v3 window and explicitly return keyboard focus after tray hide.
	a.window.UnMinimise()
	a.window.Show()
	a.window.Focus()
	a.mu.RLock()
	status := a.status
	a.mu.RUnlock()
	a.emitEvent("status:update", status)
}

func (a *App) quit() {
	if a.ctx == nil {
		return
	}
	a.quitting.Store(true)
	a.saveWindow(a.ctx)
	a.desktop.Quit()
}

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
	failedTasks := 0
	if a.status.Tracker.Connection == "connected" {
		for _, state := range a.trackerTasks {
			if state == "failed" {
				failedTasks++
			}
		}
	}
	a.mu.RUnlock()
	switch event.Kind {
	case logdetect.MatchFound:
		a.mu.Lock()
		a.status.LastQueueSeconds = event.QueueSeconds
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
		a.addLog("Info", "Raid", fmt.Sprintf("Match found after %.1f seconds", event.QueueSeconds))
		if settings.SoundsEnabled && settings.MatchFoundSound {
			a.addLog("Info", "Sound", "Playing match-found alert")
			playNotification(sound.MatchFound, settings.MatchFoundSoundPath, settings.SoundVolume)
		}
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
		if runThrough {
			a.status.RunThroughAt = started.Add(time.Duration(settings.RunThroughSeconds) * time.Second).Format(time.RFC3339)
		}
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
		a.addLog("Info", "Raid", "Raid started")
		a.retryMapAfterRaidStart()
		if settings.SoundsEnabled && settings.RaidStartSound {
			a.addLog("Info", "Sound", "Playing raid-start alert")
			playNotification(sound.RaidStart, settings.RaidStartSoundPath, settings.SoundVolume)
		}
		if settings.SoundsEnabled && settings.QuestItemsSound {
			a.addLog("Info", "Sound", "Playing quest-items reminder")
			playNotification(sound.QuestItems, settings.QuestItemsSoundPath, settings.SoundVolume)
		}
		if settings.SoundsEnabled && settings.RestartTasksSound && failedTasks > 0 {
			a.addLog("Warn", "TarkovTracker", fmt.Sprintf("%d failed task(s) may need to be restarted", failedTasks))
			playNotification(sound.RestartTasks, settings.RestartTasksSoundPath, settings.SoundVolume)
		}
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
		status := a.status
		a.mu.Unlock()
		a.emitEvent("status:update", status)
		a.addLog("Info", "Raid", "Raid ended")
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
		if active && current.SoundsEnabled && current.RunThroughSound && !a.quitting.Load() {
			a.addLog("Info", "Sound", "Playing run-through alert")
			playNotification(sound.RunThrough, current.RunThroughSoundPath, current.SoundVolume)
		}
	}()
}

func (a *App) PreviewSound(kind, path string, volume int) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	var soundKind sound.Kind
	switch kind {
	case string(sound.Quest):
		soundKind = sound.Quest
	case string(sound.Error):
		soundKind = sound.Error
	case string(sound.MatchFound):
		soundKind = sound.MatchFound
	case string(sound.RaidStart):
		soundKind = sound.RaidStart
	case string(sound.RunThrough):
		soundKind = sound.RunThrough
	case string(sound.QuestItems):
		soundKind = sound.QuestItems
	case string(sound.RestartTasks):
		soundKind = sound.RestartTasks
	default:
		return errors.New("unknown sound kind")
	}
	if path != "" {
		if err := sound.ValidateFile(path); err != nil {
			return err
		}
	}
	playNotification(soundKind, path, volume)
	return nil
}

func playNotification(kind sound.Kind, path string, volume int) {
	go func() {
		if path != "" && sound.PlayFile(path, volume) == nil {
			return
		}
		sound.Play(kind, volume)
	}()
}

func (a *App) playQuestSound(key string) {
	a.mu.Lock()
	s := a.settings
	now := time.Now()
	if !s.SoundsEnabled || !s.QuestSoundEnabled || (key == a.lastQuestSoundKey && now.Sub(a.lastQuestSoundAt) < recognitionSoundCooldown) {
		a.mu.Unlock()
		return
	}
	a.lastQuestSoundKey, a.lastQuestSoundAt = key, now
	a.mu.Unlock()
	a.addLog("Info", "Sound", "Playing quest-recognition alert")
	playNotification(sound.Quest, s.QuestSoundPath, s.SoundVolume)
}

func (a *App) playErrorSound(key string) {
	a.mu.Lock()
	s := a.settings
	now := time.Now()
	if !s.SoundsEnabled || !s.ErrorSoundEnabled || (key == a.lastErrorSoundKey && now.Sub(a.lastErrorSoundAt) < recognitionSoundCooldown) {
		a.mu.Unlock()
		return
	}
	a.lastErrorSoundKey, a.lastErrorSoundAt = key, now
	a.mu.Unlock()
	a.addLog("Info", "Sound", "Playing recognition-error alert")
	playNotification(sound.Error, s.ErrorSoundPath, s.SoundVolume)
}

func (a *App) setError(err error) {
	a.setErrorState(0, false, err)
}

func (a *App) setErrorIfCurrent(sequence uint64, err error) {
	a.setErrorState(sequence, true, err)
}

func (a *App) setErrorState(sequence uint64, requireCurrent bool, err error) {
	a.mu.Lock()
	if requireCurrent && a.analysisSequence.Load() != sequence {
		a.mu.Unlock()
		return
	}
	a.status.Connection = "error"
	a.status.LastError = err.Error()
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	a.addLog("Error", "Remote", err.Error())
	a.playErrorSound("runtime:" + err.Error())
}
