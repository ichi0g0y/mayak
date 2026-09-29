package app

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

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
	// menu is the window the shell's menus open in, above the page (app_menu.go).
	menu          shellMenu
	adblock       *adblock.Blocker
	browserClient atomic.Bool
	// hostMode is the Host's game mode as its link told this Client
	// (BrowserSetHostMode); guarded by mu.
	hostMode              string
	browserBackgroundOnce sync.Once
	hideoutProgressToken  string
	// catalogRefreshes counts the catalog refreshes started (app_catalog.go).
	catalogRefreshes   uint64
	hideoutDetector    *hideoutlog.Detector
	hideoutStore       *hideoutlog.Store
	hideoutStations    []catalog.HideoutStation
	hideoutCatalogMode string
	// catalogVersions: the version of each part of a mode's catalog last
	// applied (catalogChanged), so a refresh rebuilds only what changed.
	// pendingHistory: past-log syncs of profiles given a key while EFT ran,
	// run when it closes (watchGame).
	pendingHistoryMu  sync.Mutex
	pendingHistory    map[[3]string]bool
	catalogVersionsMu sync.Mutex
	catalogVersions   map[string]string
	hideoutProgress   map[string]bool
	ctx               context.Context
	mu                sync.RWMutex
	settingsWriteMu   sync.Mutex
	screenshotStoreMu sync.Mutex
	remoteOperationMu sync.Mutex
	settings          config.Settings
	status            model.Status
	watcher           *watcher.Watcher
	remotes           map[string]*remote.Client
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
	lastRaid       *GoonRaid
	goonReports    map[string]bool
	faviconOnce    sync.Once
	favicons       *faviconCache
	trackerClient  *tracker.Client
	trackerStore   trackerstore.Store
	trackerStoreMu sync.Mutex
	// trackerJobs is the work a key assignment starts (a sync of the
	// profile's past logs, a refresh), for a test to wait on.
	trackerJobs    sync.WaitGroup
	trackerNamesMu sync.Mutex
	trackerData    trackerstore.Document
	trackerTasks   map[string]string
	trackerSyncMu  sync.Mutex
	logs           *applog.Store
	lastSounds     map[sound.Kind]recentSound
	lastProcessed  config.ProcessedScreenshot
	analysisCancel context.CancelFunc
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
	// Story chapters are known from the wiki only; fetch its list before the
	// first recognition needs it.
	a.questClient.WarmWiki()
	if removed, err := autostart.RemoveLegacy(); err != nil {
		a.addLog("Warn", "Application", "Could not remove the RaidLens startup entry: "+err.Error())
	} else if removed {
		a.addLog("Info", "Application", "Removed the RaidLens startup entry")
	}
	loadedSettings, loadErr := config.Load()
	settings := normalizeSettings(loadedSettings)
	importSoundFiles(&settings)
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
	a.notifyOnce(sound.RemoteError, err.Error())
}
