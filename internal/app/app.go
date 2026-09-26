package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
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
	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/iteminfo"
	"github.com/local/mayak/internal/itemmatch"
	"github.com/local/mayak/internal/locale"
	"github.com/local/mayak/internal/logdetect"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/position"
	"github.com/local/mayak/internal/questapi"
	"github.com/local/mayak/internal/questmatch"
	"github.com/local/mayak/internal/remote"
	"github.com/local/mayak/internal/remoteid"
	"github.com/local/mayak/internal/screenshotstore"
	"github.com/local/mayak/internal/sound"
	"github.com/local/mayak/internal/taskdetect"
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
	runThroughCancel  context.CancelFunc
	done              chan struct{}
	doneOnce          sync.Once
	quitting          atomic.Bool
	analysisSequence  atomic.Uint64
	// update is the newer release being fetched (app_update.go).
	update updateState
}

const recognitionSoundCooldown = 3 * time.Second

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

func tesseractUnavailable() bool {
	if _, ok := ocr.BundledTesseract(); ok {
		return false
	}
	_, err := exec.LookPath("tesseract")
	return err != nil
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

// GameLanguages are the game languages MAYAK reads (settings' choices):
// English and those in internal/locale.
func (a *App) GameLanguages() []string { return append([]string{"en"}, locale.Languages...) }

func (a *App) GetSettings() config.Settings { a.mu.RLock(); defer a.mu.RUnlock(); return a.settings }

func (a *App) AutoDetectRemoteID() (string, error) {
	return remoteid.Detect()
}
func (a *App) GetStatus() model.Status { a.mu.RLock(); defer a.mu.RUnlock(); return a.status }

func (a *App) ImportTrackerToken(token string) error {
	token = strings.TrimSpace(token)
	mode, ok := tracker.ModeForToken(token)
	if !ok {
		return errors.New("expected a PVP_, PVE_, or SZN_ TarkovTracker token")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	info, err := a.trackerClient.TokenInfo(ctx, token)
	if err != nil {
		return err
	}
	if tracker.Mode(info.GameMode) != mode {
		return fmt.Errorf("TarkovTracker reports this token belongs to %s", info.GameMode)
	}
	if info.Token != "" && info.Token != token {
		return errors.New("TarkovTracker returned a different token identity")
	}
	if !tracker.HasPermissions(info, "GP", "WP") {
		return errors.New("TarkovTracker token requires Get Progress and Write Progress permissions")
	}
	a.trackerStoreMu.Lock()
	defer a.trackerStoreMu.Unlock()
	a.mu.Lock()
	document := a.trackerData.Clone()
	if _, err := document.AddKey(string(mode), token, info.Note); err != nil {
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
	a.addLog("Info", "TarkovTracker", "Verified and stored an unassigned "+string(mode)+" key")
	return nil
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

func (a *App) GetTrackerHistoryBreakpoints(accountID, profileID, mode string) []model.TrackerHistoryBreakpoint {
	a.mu.RLock()
	root := a.settings.LogsDirectory
	a.mu.RUnlock()
	points := trackerlog.HistoryBreakpoints(root, accountID, profileID, mode)
	result := make([]model.TrackerHistoryBreakpoint, 0, len(points))
	for _, point := range points {
		result = append(result, model.TrackerHistoryBreakpoint{
			ID: point.ID, Version: point.Version, StartAt: point.StartAt.Format(time.RFC3339),
		})
	}
	return result
}

func (a *App) SyncTrackerHistory(accountID, profileID, mode, breakpointID string) error {
	a.trackerSyncMu.Lock()
	defer a.trackerSyncMu.Unlock()
	a.mu.RLock()
	root := a.settings.LogsDirectory
	enabled := a.settings.TarkovTrackerEnabled
	token := a.trackerData.TokenFor(accountID, profileID, mode)
	a.mu.RUnlock()
	if !enabled {
		return errors.New("TarkovTracker sync is disabled")
	}
	if token == "" {
		return errors.New("no key is assigned to this EFT profile")
	}
	states, err := trackerlog.TaskHistory(root, breakpointID, accountID, profileID, mode)
	if err != nil {
		return err
	}
	updates := make([]tracker.TaskUpdate, 0, len(states))
	for taskID, state := range states {
		updates = append(updates, tracker.TaskUpdate{ID: taskID, State: state})
	}
	sort.Slice(updates, func(i, j int) bool { return updates[i].ID < updates[j].ID })
	if len(updates) == 0 {
		return errors.New("no task changes were found after the selected breakpoint")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 45*time.Second)
	defer cancel()
	if err = a.trackerClient.SetTasks(ctx, token, updates); err != nil {
		a.addLog("Error", "TarkovTracker", "Historical sync failed: "+err.Error())
		return err
	}
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Synced %d task states from existing logs for %s (%s)", len(updates), maskProfileID(profileID), mode))
	a.mu.RLock()
	active := a.status.Tracker.AccountID == accountID && a.status.Tracker.ProfileID == profileID && a.status.Tracker.Mode == mode
	a.mu.RUnlock()
	if active {
		go func() { _ = a.refreshTrackerIdentity(mode, profileID, accountID) }()
	}
	return nil
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

func (a *App) AnalyzeLatestScreenshot() error {
	if a.browserClient.Load() || a.BrowserPlatform() != "windows" {
		return errors.New("analysis requires Windows Host mode")
	}
	a.mu.RLock()
	dir := a.settings.ScreenshotDirectory
	a.mu.RUnlock()
	if dir == "" {
		return errors.New("Screenshotsフォルダが未設定です")
	}
	latestPath, err := latestScreenshotPath(dir)
	if err != nil {
		return err
	}
	if latestPath == "" {
		return errors.New("解析できるスクリーンショットがありません")
	}
	a.processScreenshot(latestPath, true)
	return nil
}

func latestScreenshotPath(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var latestPath string
	var latestTime int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr == nil && info.ModTime().UnixNano() > latestTime {
			latestTime = info.ModTime().UnixNano()
			latestPath = filepath.Join(dir, entry.Name())
		}
	}
	return latestPath, nil
}

func (a *App) rememberExistingScreenshot() {
	a.mu.Lock()
	if a.lastProcessed.Path != "" || a.settings.ScreenshotDirectory == "" {
		a.mu.Unlock()
		return
	}
	dir := a.settings.ScreenshotDirectory
	a.mu.Unlock()
	latestPath, err := latestScreenshotPath(dir)
	if err != nil || latestPath == "" {
		return
	}
	fingerprint, err := config.FingerprintScreenshot(latestPath)
	if err != nil {
		return
	}
	a.mu.Lock()
	a.lastProcessed = fingerprint
	a.status.LastScreenshot = fingerprint.Path
	a.mu.Unlock()
	_ = config.SaveProcessedScreenshot(fingerprint)
}

func (a *App) AutoDetectEFTDirectories() (eftdetect.Result, error) {
	detected, err := eftdetect.Detect()
	if err != nil {
		return detected, err
	}
	a.settingsWriteMu.Lock()
	defer a.settingsWriteMu.Unlock()
	a.mu.Lock()
	if detected.ScreenshotDirectory != "" {
		a.settings.ScreenshotDirectory = detected.ScreenshotDirectory
	}
	if detected.LogsDirectory != "" {
		a.settings.LogsDirectory = detected.LogsDirectory
	}
	monitoring := a.status.Monitoring
	settings := a.settings
	a.mu.Unlock()
	if err := config.Save(settings); err != nil {
		return detected, err
	}
	if monitoring && detected.ScreenshotDirectory != "" {
		if err := a.startWatcher(detected.ScreenshotDirectory); err != nil {
			return detected, err
		}
	}
	if monitoring && detected.LogsDirectory != "" {
		a.startLogDetector(detected.LogsDirectory)
	}
	return detected, nil
}

func (a *App) ChooseScreenshotDirectory() (string, error) {
	dir, err := a.desktop.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{Title: "Select EFT Screenshots folder", CanChooseDirectories: true, Window: a.window}).PromptForSingleSelection()
	if err != nil || dir == "" {
		return dir, err
	}
	a.settingsWriteMu.Lock()
	defer a.settingsWriteMu.Unlock()
	a.mu.Lock()
	a.settings.ScreenshotDirectory = dir
	settings := a.settings
	a.mu.Unlock()
	if err := config.Save(settings); err != nil {
		return "", err
	}
	a.mu.RLock()
	monitoring := a.status.Monitoring
	a.mu.RUnlock()
	if monitoring {
		return dir, a.startWatcher(dir)
	}
	return dir, nil
}

func (a *App) ChooseLogsDirectory() (string, error) {
	dir, err := a.desktop.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{Title: "Select EFT Logs folder", CanChooseDirectories: true, Window: a.window}).PromptForSingleSelection()
	if err != nil || dir == "" {
		return dir, err
	}
	a.settingsWriteMu.Lock()
	defer a.settingsWriteMu.Unlock()
	a.mu.Lock()
	a.settings.LogsDirectory = dir
	settings := a.settings
	a.mu.Unlock()
	if err := config.Save(settings); err != nil {
		return "", err
	}
	a.mu.RLock()
	monitoring := a.status.Monitoring
	a.mu.RUnlock()
	if monitoring {
		a.startLogDetector(dir)
	}
	return dir, nil
}

func (a *App) OpenScreenshotDirectory() error {
	a.mu.RLock()
	dir := a.settings.ScreenshotDirectory
	a.mu.RUnlock()
	return openDirectory(dir)
}

func (a *App) OpenLogsDirectory() error {
	a.mu.RLock()
	dir := a.settings.LogsDirectory
	a.mu.RUnlock()
	return openDirectory(dir)
}

func (a *App) OpenDebugDirectory() error {
	a.mu.RLock()
	dir := a.settings.ScreenshotDirectory
	a.mu.RUnlock()
	if strings.TrimSpace(dir) == "" {
		return errors.New("Screenshots folder is not configured")
	}
	debugDir := filepath.Join(dir, screenshotstore.DebugDirectory)
	if err := os.MkdirAll(debugDir, 0700); err != nil {
		return err
	}
	return openDirectory(debugDir)
}

func openDirectory(dir string) error {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "." || !filepath.IsAbs(dir) {
		return errors.New("folder is not configured")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("configured path is not a folder")
	}
	return exec.Command("explorer.exe", dir).Start()
}

func (a *App) ChooseSoundFile() (string, error) {
	path, err := a.desktop.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{CanChooseFiles: true, Window: a.window,
		Title:   "Select notification sound",
		Filters: []application.FileFilter{{DisplayName: "Audio files (*.wav, *.mp3)", Pattern: "*.wav;*.mp3"}},
	}).PromptForSingleSelection()
	if err != nil || path == "" {
		return path, err
	}
	if err := sound.ValidateFile(path); err != nil {
		return "", err
	}
	return filepath.Clean(path), nil
}

func (a *App) SaveSettings(s config.Settings) error {
	a.settingsWriteMu.Lock()
	defer a.settingsWriteMu.Unlock()
	s = normalizeSettings(s)
	a.mu.Lock()
	oldTrackerEnabled := a.settings.TarkovTrackerEnabled
	oldDir := a.settings.ScreenshotDirectory
	oldGameMode := a.settings.GameMode
	oldLogs := a.settings.LogsDirectory
	oldLaunchAtStartup := a.settings.LaunchAtStartup
	oldAutoUpdate := a.settings.AutoUpdate
	oldLanguage := a.settings.Language
	oldCleanup, oldRetainCount, oldRetainHours := a.settings.ScreenshotCleanup, a.settings.ScreenshotRetainCount, a.settings.ScreenshotRetainHours
	monitoring := a.status.Monitoring
	s = a.keepWindowSettings(s)
	a.mu.Unlock()
	if err := applyAutostartChange(oldLaunchAtStartup, s.LaunchAtStartup); err != nil {
		return err
	}
	if err := config.Save(s); err != nil {
		_ = applyAutostartChange(s.LaunchAtStartup, oldLaunchAtStartup)
		return err
	}
	a.mu.Lock()
	a.settings = s
	a.applyTrackerTokenFlagsLocked()
	a.updateTrackerConnectionLocked()
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	if !oldTrackerEnabled && s.TarkovTrackerEnabled {
		go func() { _ = a.RefreshTracker() }()
	}
	if oldGameMode != s.GameMode {
		go func() { _ = a.refreshCatalog(false) }()
	}
	if !oldAutoUpdate && s.AutoUpdate {
		go func() { _, _ = a.checkForUpdates(true) }()
	}
	if oldLanguage != s.Language {
		a.setTrayLanguage(s.Language)
	}
	if oldCleanup != s.ScreenshotCleanup || oldRetainCount != s.ScreenshotRetainCount || oldRetainHours != s.ScreenshotRetainHours {
		go a.runScreenshotMaintenance()
	}
	if monitoring && oldLogs != s.LogsDirectory {
		a.startLogDetector(s.LogsDirectory)
	}
	if monitoring && oldDir != s.ScreenshotDirectory {
		if s.ScreenshotDirectory == "" {
			a.StopMonitoring()
			return nil
		}
		return a.startWatcher(s.ScreenshotDirectory)
	}
	return nil
}

// PersistSettings stores edits immediately without restarting active services.
// Explicit SaveSettings still applies folder changes to a running monitor.
func (a *App) PersistSettings(s config.Settings) error {
	a.settingsWriteMu.Lock()
	defer a.settingsWriteMu.Unlock()
	s = normalizeSettings(s)
	a.mu.Lock()
	oldLaunchAtStartup := a.settings.LaunchAtStartup
	oldAutoUpdate := a.settings.AutoUpdate
	oldTrackerEnabled := a.settings.TarkovTrackerEnabled
	oldGameMode := a.settings.GameMode
	oldCleanup, oldRetainCount, oldRetainHours := a.settings.ScreenshotCleanup, a.settings.ScreenshotRetainCount, a.settings.ScreenshotRetainHours
	s = a.keepWindowSettings(s)
	a.mu.Unlock()
	if err := applyAutostartChange(oldLaunchAtStartup, s.LaunchAtStartup); err != nil {
		return err
	}
	if err := config.Save(s); err != nil {
		_ = applyAutostartChange(s.LaunchAtStartup, oldLaunchAtStartup)
		return err
	}
	a.mu.Lock()
	a.settings = s
	a.applyTrackerTokenFlagsLocked()
	a.updateTrackerConnectionLocked()
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	if !oldTrackerEnabled && s.TarkovTrackerEnabled {
		go func() { _ = a.RefreshTracker() }()
	}
	if oldGameMode != s.GameMode {
		go func() { _ = a.refreshCatalog(false) }()
	}
	if !oldAutoUpdate && s.AutoUpdate {
		go func() { _, _ = a.checkForUpdates(true) }()
	}
	if oldCleanup != s.ScreenshotCleanup || oldRetainCount != s.ScreenshotRetainCount || oldRetainHours != s.ScreenshotRetainHours {
		go a.runScreenshotMaintenance()
	}
	return nil
}

func applyAutostartChange(oldValue, newValue bool) error {
	if oldValue == newValue {
		return nil
	}
	return autostart.Apply(newValue)
}

func normalizeSettings(s config.Settings) config.Settings {
	// The game languages MAYAK reads: English and those in internal/locale.
	if s.GameLanguage != "en" && !slices.Contains(locale.Languages, s.GameLanguage) {
		s.GameLanguage = "auto"
	}
	s.QuestSite = normalizeQuestSite(s.QuestSite)
	s.HideoutErrorSoundPath = normalizeSoundPath(s.HideoutErrorSoundPath)
	if s.ScreenshotDirectory != "" {
		s.ScreenshotDirectory = filepath.Clean(s.ScreenshotDirectory)
	}
	if s.LogsDirectory != "" {
		s.LogsDirectory = filepath.Clean(s.LogsDirectory)
	}
	s.RemoteID = strings.TrimSpace(s.RemoteID)
	s.RemoteTargets = normalizeRemoteTargets(s.RemoteTargets, s.RemoteID)
	if len(s.RemoteTargets) > 0 {
		s.RemoteID = s.RemoteTargets[0].ID
	} else {
		s.RemoteID = ""
	}
	if s.SoundVolume < 0 {
		s.SoundVolume = 0
	}
	if s.SoundVolume > 100 {
		s.SoundVolume = 100
	}
	if s.RunThroughSeconds < 1 || s.RunThroughSeconds > 3599 {
		s.RunThroughSeconds = 430
	}
	if s.ScreenshotRetainCount < 0 {
		s.ScreenshotRetainCount = 0
	}
	if s.ScreenshotRetainCount > 100000 {
		s.ScreenshotRetainCount = 100000
	}
	if s.ScreenshotRetainHours < 0 {
		s.ScreenshotRetainHours = 0
	}
	if s.ScreenshotRetainHours > 87600 {
		s.ScreenshotRetainHours = 87600
	}
	s.QuestSoundPath = normalizeSoundPath(s.QuestSoundPath)
	s.ErrorSoundPath = normalizeSoundPath(s.ErrorSoundPath)
	s.MatchFoundSoundPath = normalizeSoundPath(s.MatchFoundSoundPath)
	s.RaidStartSoundPath = normalizeSoundPath(s.RaidStartSoundPath)
	s.RunThroughSoundPath = normalizeSoundPath(s.RunThroughSoundPath)
	s.QuestItemsSoundPath = normalizeSoundPath(s.QuestItemsSoundPath)
	s.RestartTasksSoundPath = normalizeSoundPath(s.RestartTasksSoundPath)
	s.Map = normalizeMapSetting(s.Map)
	if s.Language != "ja" && s.Language != "en" {
		s.Language = "ja"
	}
	if s.OCREngine != "windows" && s.OCREngine != "tesseract" {
		s.OCREngine = "tesseract"
	}
	switch s.GameMode {
	case "auto", "regular", "pve", "pvp-season":
	default:
		s.GameMode = "auto"
	}
	return s
}

func normalizeSoundPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func normalizeRemoteTargets(targets []config.RemoteTarget, legacyID string) []config.RemoteTarget {
	if len(targets) == 0 && strings.TrimSpace(legacyID) != "" {
		targets = []config.RemoteTarget{{ID: legacyID, Name: "Remote 1", Map: true, Tasks: true}}
	}
	result := make([]config.RemoteTarget, 0, len(targets))
	seen := make(map[string]bool)
	for _, target := range targets {
		target.ID = strings.TrimSpace(target.ID)
		target.Name = strings.TrimSpace(target.Name)
		if target.ID == "" || seen[target.ID] {
			continue
		}
		if target.Name == "" {
			target.Name = fmt.Sprintf("Remote %d", len(result)+1)
		}
		seen[target.ID] = true
		result = append(result, target)
	}
	return result
}

func remoteTargetIDs(settings config.Settings, channel string) []string {
	ids := make([]string, 0, len(settings.RemoteTargets)+1)
	for _, target := range settings.RemoteTargets {
		if channel == "all" || channel == "map" && target.Map || channel == "tasks" && target.Tasks {
			ids = append(ids, target.ID)
		}
	}
	// The built-in browser's tarkov.dev map pages receive map and position
	// commands. Tasks are opened by the browser itself, not via Remote Control.
	if id := settings.BrowserRemoteID; id != "" && (channel == "all" || channel == "map") && !slices.Contains(ids, id) {
		ids = append(ids, id)
	}
	return ids
}

func normalizeMapSetting(mapName string) string {
	mapName = strings.ToLower(strings.TrimSpace(mapName))
	switch mapName {
	case "laboratory":
		return "the-lab"
	case "labyrinth":
		return "the-labyrinth"
	case "factory4_night":
		return "night-factory"
	}
	return mapName
}

func (a *App) keepWindowSettings(s config.Settings) config.Settings {
	s.WindowX = a.settings.WindowX
	s.WindowY = a.settings.WindowY
	s.WindowWidth = a.settings.WindowWidth
	s.WindowHeight = a.settings.WindowHeight
	s.WindowConfigured = a.settings.WindowConfigured
	s.BrowserRemoteID = a.settings.BrowserRemoteID
	s.OCRDefaultRevision = a.settings.OCRDefaultRevision
	return s
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

func (a *App) sendMap(remoteID, mapName string) error {
	return a.remoteClient(remoteID).SendMap(a.ctx, mapName)
}

func (a *App) sendTask(remoteID, normalizedName string) error {
	return a.remoteClient(remoteID).SendTask(a.ctx, normalizedName)
}

func (a *App) remoteClient(remoteID string) *remote.Client {
	a.mu.Lock()
	if client := a.remotes[remoteID]; client != nil {
		a.mu.Unlock()
		return client
	}
	client := remote.New(remoteID)
	if remoteID == a.settings.BrowserRemoteID {
		client.OnSend(a.noteBrowserRemoteSend)
	}
	if a.remotes == nil {
		a.remotes = make(map[string]*remote.Client)
	}
	a.remotes[remoteID] = client
	a.mu.Unlock()
	return client
}

func (a *App) sendMapTargets(settings config.Settings, mapName, channel string) error {
	a.emitEvent("browser:map", mapName)
	var result error
	for _, id := range remoteTargetIDs(settings, channel) {
		result = errors.Join(result, a.sendMap(id, mapName))
	}
	return result
}

func (a *App) sendTaskTargets(settings config.Settings, normalizedName string) error {
	var result error
	for _, id := range remoteTargetIDs(settings, "tasks") {
		result = errors.Join(result, a.sendTask(id, normalizedName))
	}
	return result
}

func (a *App) runRemoteIfCurrent(sequence uint64, operation func() error) (bool, error) {
	a.remoteOperationMu.Lock()
	defer a.remoteOperationMu.Unlock()
	if a.quitting.Load() || a.analysisSequence.Load() != sequence {
		return false, nil
	}
	return true, operation()
}

func (a *App) recordRemoteCommandIfCurrent(sequence uint64, command string) {
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence {
		a.mu.Unlock()
		return
	}
	a.status.Connection = "connected"
	a.status.LastRemoteCommand = command
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	a.addLog("Info", "Remote", "Sent "+command)
}

func (a *App) startWatcher(dir string) error {
	w, err := watcher.New(dir, a.handleScreenshot)
	if err != nil {
		return err
	}
	if err := w.Start(); err != nil {
		_ = w.Close()
		return err
	}
	a.mu.Lock()
	old := a.watcher
	a.watcher = w
	a.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (a *App) handleScreenshot(path string) {
	a.processScreenshot(path, false)
}

func (a *App) processScreenshot(path string, force bool) {
	if a.quitting.Load() || a.browserClient.Load() {
		return
	}
	if fingerprint, err := config.FingerprintScreenshot(path); err == nil {
		a.mu.Lock()
		alreadyProcessed := fingerprint.Matches(a.lastProcessed)
		if !alreadyProcessed || force {
			a.lastProcessed = fingerprint
		}
		a.mu.Unlock()
		if alreadyProcessed && !force {
			a.addLog("Debug", "Screenshot", "Skipped previously processed "+filepath.Base(path))
			return
		}
		if err = config.SaveProcessedScreenshot(fingerprint); err != nil {
			a.addLog("Warn", "Screenshot", "Could not remember processed screenshot: "+err.Error())
		}
	}
	a.addLog("Debug", "Screenshot", "Detected "+filepath.Base(path))
	// The shell's screenshot views show the new one.
	a.emitEvent("browser:screenshot", filepath.Base(path))
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	analysisContext, cancel := context.WithCancel(parent)
	parsed, parseErr := position.ParseFilename(filepath.Base(path))
	a.mu.Lock()
	sequence := a.analysisSequence.Add(1)
	previousCancel := a.analysisCancel
	a.analysisCancel = cancel
	a.status.LastScreenshot = path
	a.status.ScreenshotType = "unknown"
	a.status.Position = nil
	a.status.LastError = ""
	a.status.AnalysisStage = "画像を分類中"
	if a.status.CurrentMap == "" && a.settings.Map != "" {
		a.status.CurrentMap = a.settings.Map
	}
	status := a.status
	settings := a.settings
	if status.CurrentMap != "" {
		settings.Map = status.CurrentMap
	}
	a.mu.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
	a.emitEvent("status:update", status)
	if parseErr != nil {
		go func() {
			defer cancel()
			defer a.afterScreenshotAnalysis(sequence, path, settings)
			a.handleTaskScreenshot(analysisContext, sequence, path, settings)
		}()
		return
	}
	// EFT may attach stale coordinates to menu screenshots. Always inspect the
	// image first, then permit position sending only while logs prove a raid is active.
	go func() {
		defer cancel()
		defer a.afterScreenshotAnalysis(sequence, path, settings)
		a.handleCoordinateScreenshot(analysisContext, sequence, path, settings, parsed)
	}()
}

// Screenshots are classified in order: an open item inspection window (its
// item information wins over whatever is behind it), then a task list, then
// the position from the file name.
func (a *App) handleCoordinateScreenshot(ctx context.Context, sequence uint64, path string, settings config.Settings, parsed model.Position) {
	if itemDetected, err := itemdetect.AnalyzeFile(path); err == nil && itemDetected.IsItem {
		a.handleItemAnalysis(ctx, sequence, path, settings, itemDetected)
		return
	}
	detected, detectErr := taskdetect.AnalyzeFile(path, taskdetect.Preset2560)
	if detectErr == nil && detected.IsTasks {
		analysis, analysisErr := a.recognizeQuest(ctx, settings, detected.Crop)
		if analysisErr == nil && topConfidence(analysis.matches) >= .78 {
			a.handleTaskAnalysis(ctx, sequence, path, settings, detected, &analysis)
			return
		}
		a.addLog("Debug", "Recognition", fmt.Sprintf("Rejected Tasks-like raid image (layout=%s score=%.2f)", detected.Layout, detected.Score))
	}
	active, _ := logdetect.RaidState(settings.LogsDirectory)
	latestMap, mapKnown := logdetect.LatestMap(settings.LogsDirectory)
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.Unlock()
		return
	}
	configuredFallback := a.settings.Map
	previousMap := a.status.CurrentMap
	settings.Map = configuredFallback
	if active && mapKnown {
		settings.Map = latestMap
		a.status.CurrentMap = latestMap
	}
	a.status.RaidActive = active
	if detectErr == nil {
		a.status.DetectionScore = detected.Score
		a.status.DetectionLayout = detected.Layout
		a.status.CropPreview = detected.CropDataURL
	}
	if !active {
		a.status.ScreenshotType = "unknown"
		a.status.AnalysisStage = "レイド外の座標メタデータを無視しました"
		status := a.status
		a.mu.Unlock()
		a.emitEvent("status:update", status)
		a.addLog("Debug", "Position", "Ignored coordinate metadata outside an active raid")
		return
	}
	a.status.ScreenshotType = "position"
	a.status.Position = &parsed
	a.status.AnalysisStage = "レイド中の位置を検出"
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	if active && mapKnown && previousMap != latestMap {
		a.addLog("Info", "Raid", fmt.Sprintf("Map refreshed from latest EFT session: %s", latestMap))
	}
	a.addLog("Info", "Position", fmt.Sprintf("Detected x=%.2f y=%.2f z=%.2f rotation=%.2f map=%s", parsed.X, parsed.Y, parsed.Z, parsed.Rotation, settings.Map))
	// The built-in browser brings its map view forward: the last detection
	// decides the tab shown (a task's page after a task, the map after this).
	if settings.Map != "" {
		a.emitEvent("browser:position", settings.Map)
	}
	if len(remoteTargetIDs(settings, "map")) > 0 && settings.Map != "" {
		sent, err := a.runRemoteIfCurrent(sequence, func() error {
			return a.sendPosition(settings, parsed)
		})
		if err != nil {
			a.setErrorIfCurrent(sequence, err)
		} else if sent {
			a.recordRemoteCommandIfCurrent(sequence, "playerPosition/"+settings.Map)
		}
	}
}

// handleTaskScreenshot handles screenshots without position metadata: an open
// item inspection window first, otherwise the task list.
func (a *App) handleTaskScreenshot(ctx context.Context, sequence uint64, path string, settings config.Settings) {
	itemDetected, itemErr := itemdetect.AnalyzeFile(path)
	if itemErr != nil {
		a.updateAnalysisError(sequence, path, itemErr)
		return
	}
	if itemDetected.IsItem {
		a.handleItemAnalysis(ctx, sequence, path, settings, itemDetected)
		return
	}
	detected, err := taskdetect.AnalyzeFile(path, taskdetect.Preset2560)
	if err != nil {
		a.updateAnalysisError(sequence, path, err)
		return
	}
	a.handleTaskAnalysis(ctx, sequence, path, settings, detected, nil)
}

func (a *App) handleItemAnalysis(ctx context.Context, sequence uint64, path string, settings config.Settings, detected itemdetect.Result) {
	active, _ := logdetect.RaidState(settings.LogsDirectory)
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.Unlock()
		return
	}
	a.status.RaidActive = active
	a.status.ScreenshotType = "item"
	a.status.DetectionScore = detected.Score
	a.status.DetectionLayout = "item-detail"
	if detected.Offer {
		a.status.DetectionLayout = "flea-offer"
	}
	a.status.CropPreview = detected.CropDataURL
	a.status.AnalysisStage = "アイテム詳細を検出・OCR実行中"
	a.status.OCRRaw = ""
	a.status.LastItem = ""
	a.status.ItemID = ""
	a.status.ItemShortName = ""
	a.status.ItemURL = ""
	a.status.ItemIconURL = ""
	a.status.ItemConfidence = 0
	a.status.ItemCandidates = nil
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	a.addLog("Info", "Recognition", fmt.Sprintf("Item detected in %s (score=%.2f)", status.DetectionLayout, detected.Score))

	readings, byWindows, err := recognizeTitleCandidates(ctx, settings, detected.Crop)
	if err != nil {
		a.updateAnalysisError(sequence, path, err)
		return
	}
	items, err := a.itemClient.ItemsForMode(ctx, a.effectiveCatalogMode(settings.GameMode))
	if err != nil {
		a.updateAnalysisError(sequence, path, err)
		return
	}
	raw, matches := bestItemReading(readings, items)
	// A reading that matches no item well gets a second look from Windows OCR,
	// which reads Japanese titles differently from Tesseract.
	if !byWindows && (len(matches) == 0 || matches[0].Confidence < .82) {
		if extra, err := (ocr.Windows{Language: ocrLanguage(settings)}).RecognizeCandidates(ctx, detected.Crop); err == nil && len(extra) > 0 {
			raw, matches = bestItemReading(append([]string{raw}, extra...), items)
		}
	}
	limit := min(5, len(matches))
	candidates := make([]model.ItemCandidate, 0, limit)
	for _, match := range matches[:limit] {
		candidates = append(candidates, model.ItemCandidate{ID: match.Item.ID, Name: match.Item.Name, ShortName: match.Item.ShortName, URL: match.Item.Link, IconURL: match.Item.IconLink, Confidence: match.Confidence})
	}

	a.mu.Lock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.Unlock()
		return
	}
	a.status.OCRRaw = raw
	a.status.ItemCandidates = candidates
	a.status.LastError = ""
	if len(candidates) > 0 {
		best := candidates[0]
		a.status.ItemConfidence = best.Confidence
		if best.Confidence >= .82 {
			a.status.LastItem = best.Name
			a.status.ItemID = best.ID
			a.status.ItemShortName = best.ShortName
			a.status.ItemURL = best.URL
			a.status.ItemIconURL = best.IconURL
		}
	}
	a.status.AnalysisStage = "解析完了"
	status = a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	if status.ItemConfidence >= .82 {
		a.addLog("Info", "Item", fmt.Sprintf("Matched %s (%.0f%%) from OCR: %s", status.LastItem, status.ItemConfidence*100, raw))
		go a.showBrowserItem(a.effectiveCatalogMode(settings.GameMode), status.ItemID)
	} else {
		a.addLog("Warn", "Item", fmt.Sprintf("No confident match (%.0f%%) from OCR: %s", status.ItemConfidence*100, raw))
	}
}

type questRecognition struct {
	raw     string
	quests  []questapi.Quest
	matches []questmatch.Result
}

func (a *App) recognizeQuest(ctx context.Context, settings config.Settings, crop image.Image) (questRecognition, error) {
	raw, err := recognizeQuestTitle(ctx, settings, crop)
	if err != nil {
		return questRecognition{}, err
	}
	quests, err := a.questClient.QuestsForMode(ctx, a.effectiveCatalogMode(settings.GameMode))
	if err != nil {
		return questRecognition{}, err
	}
	base := make([]questmatch.Quest, len(quests))
	for i, q := range quests {
		base[i] = q.Quest
	}
	matches := questmatch.Match(raw, base)
	if settings.OCREngine == "windows" && topConfidence(matches) < .98 {
		if alternatives, retryErr := (ocr.Windows{Language: ocrLanguage(settings)}).RecognizeAlternatives(ctx, crop); retryErr == nil {
			raw, matches = bestQuestReading(append([]string{raw}, alternatives...), base)
		}
	} else if settings.OCREngine != "windows" && topConfidence(matches) < .9 {
		// As for items: a weak Tesseract reading gets a second look from Windows OCR.
		if alternatives, retryErr := (ocr.Windows{Language: ocrLanguage(settings)}).RecognizeAlternatives(ctx, crop); retryErr == nil && len(alternatives) > 0 {
			raw, matches = bestQuestReading(append([]string{raw}, alternatives...), base)
		}
	}
	return questRecognition{raw: raw, quests: quests, matches: matches}, nil
}

func (a *App) handleTaskAnalysis(ctx context.Context, sequence uint64, path string, settings config.Settings, detected taskdetect.Result, prepared *questRecognition) {
	active, _ := logdetect.RaidState(settings.LogsDirectory)
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.Unlock()
		return
	}
	a.status.RaidActive = active
	a.status.DetectionScore = detected.Score
	a.status.DetectionLayout = detected.Layout
	a.status.CropPreview = detected.CropDataURL
	if detected.IsTasks {
		a.status.ScreenshotType = "tasks"
		a.status.AnalysisStage = "Task画面を検出・OCR実行中"
		a.status.OCRRaw = ""
		a.status.MatchConfidence = 0
		a.status.QuestCandidates = nil
	} else {
		a.status.AnalysisStage = "Task画面ではありません"
	}
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	if !detected.IsTasks {
		a.addLog("Debug", "Recognition", fmt.Sprintf("Not a Tasks screen (layout=%s score=%.2f)", detected.Layout, detected.Score))
		return
	}
	a.addLog("Info", "Recognition", fmt.Sprintf("Tasks screen detected (layout=%s score=%.2f)", detected.Layout, detected.Score))
	var analysis questRecognition
	var err error
	if prepared != nil {
		analysis = *prepared
	} else {
		analysis, err = a.recognizeQuest(ctx, settings, detected.Crop)
	}
	if err != nil {
		a.updateAnalysisError(sequence, path, err)
		return
	}
	raw, quests, matches := analysis.raw, analysis.quests, analysis.matches
	limit := 5
	if len(matches) < limit {
		limit = len(matches)
	}
	candidates := make([]model.QuestCandidate, 0, limit)
	for _, m := range matches[:limit] {
		candidates = append(candidates, model.QuestCandidate{ID: m.Quest.ID, Name: m.Quest.Name, Trader: m.Quest.Trader, Map: m.Quest.Map, Confidence: m.Confidence})
	}
	taskSlug := ""
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.Unlock()
		return
	}
	a.status.OCRRaw = raw
	a.status.AnalysisStage = "OCR完了・クエスト照合中"
	a.status.QuestCandidates = candidates
	a.status.LastError = ""
	if len(candidates) > 0 {
		best := candidates[0]
		a.status.MatchConfidence = best.Confidence
		if best.Confidence >= .78 {
			a.status.LastQuest = best.Name
			a.status.QuestID = best.ID
			a.status.QuestTrader = best.Trader
			a.status.QuestMap = best.Map
			a.status.QuestURL = ""
			a.status.QuestWikiURL = ""
			a.status.QuestObjectives = []model.QuestObjective{}
			for _, q := range quests {
				if q.ID != best.ID {
					continue
				}
				a.status.QuestWikiURL = q.WikiLink
				if q.NormalizedName != "" {
					taskSlug = q.NormalizedName
					a.status.QuestURL = "https://tarkov.dev/task/" + q.NormalizedName
				}
				a.status.QuestObjectives = make([]model.QuestObjective, 0, len(q.Objectives))
				for _, objective := range q.Objectives {
					a.status.QuestObjectives = append(a.status.QuestObjectives, model.QuestObjective{Description: objective.Description, Maps: objective.Maps})
				}
				break
			}
		}
	}
	a.status.AnalysisStage = "解析完了"
	status = a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	if status.MatchConfidence >= .78 {
		a.addLog("Info", "Quest", fmt.Sprintf("Matched %s (%.0f%%) from OCR: %s", status.LastQuest, status.MatchConfidence*100, raw))
	} else {
		a.addLog("Warn", "Quest", fmt.Sprintf("No confident match (%.0f%%) from OCR: %s", status.MatchConfidence*100, raw))
	}
	if a.analysisSequence.Load() != sequence {
		return
	}
	if status.MatchConfidence >= .78 {
		a.playQuestSound(status.QuestID)
	} else {
		a.playErrorSound("quest-match")
	}
	if status.MatchConfidence >= .78 {
		a.showBrowserTask(status, settings.QuestSite)
	}
	// Sending to Remote Control follows each target's tasks role alone; the
	// quest site only decides where tasks open.
	if status.MatchConfidence >= .78 && len(remoteTargetIDs(settings, "tasks")) > 0 {
		var command string
		var operation func() error
		if taskSlug != "" {
			command = "task/" + taskSlug
			operation = func() error { return a.sendTaskTargets(settings, taskSlug) }
		} else if status.QuestMap != "" {
			command = "map/" + status.QuestMap
			operation = func() error { return a.sendMapTargets(settings, status.QuestMap, "tasks") }
		}
		if operation != nil {
			sent, err := a.runRemoteIfCurrent(sequence, operation)
			if err != nil {
				a.setErrorIfCurrent(sequence, err)
			} else if sent {
				a.recordRemoteCommandIfCurrent(sequence, command)
			}
		}
	}
}

func (a *App) updateAnalysisError(sequence uint64, path string, err error) {
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.Unlock()
		return
	}
	a.status.LastError = err.Error()
	a.status.AnalysisStage = "解析エラー"
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	a.addLog("Error", "Recognition", err.Error())
	a.playErrorSound("analysis:" + err.Error())
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

// ocrLanguage resolves "auto" to the game's own display language, so English
// titles are read as English even on a Japanese Windows (whose OCR would
// otherwise misread them), falling back to the Windows language settings.
func ocrLanguage(settings config.Settings) string {
	if settings.GameLanguage == "en" || slices.Contains(locale.Languages, settings.GameLanguage) {
		return settings.GameLanguage
	}
	if language := eftdetect.GameLanguage(); language != "" {
		return language
	}
	return "auto"
}

// tesseractEngine uses the configured tesseract.exe, else the one bundled
// with MAYAK, else Tesseract on PATH.
func tesseractEngine(settings config.Settings) ocr.Tesseract {
	engine := ocr.Tesseract{Executable: settings.TesseractPath}
	if engine.Executable == "" {
		if bundled, ok := ocr.BundledTesseract(); ok {
			engine = bundled
		}
	}
	engine.Language = ocrLanguage(settings)
	return engine
}

func recognizeQuestTitle(ctx context.Context, settings config.Settings, crop image.Image) (string, error) {
	if settings.OCREngine == "tesseract" {
		raw, err := tesseractEngine(settings).Recognize(ctx, crop)
		if err == nil || settings.TesseractPath != "" {
			return raw, err
		}
	}
	return (ocr.Windows{Language: ocrLanguage(settings)}).Recognize(ctx, crop)
}

// recognizeTitleCandidates reads a title; byWindows reports that Windows OCR
// read it (chosen, or instead of a failed bundled Tesseract).
func recognizeTitleCandidates(ctx context.Context, settings config.Settings, crop image.Image) (readings []string, byWindows bool, err error) {
	if settings.OCREngine == "tesseract" {
		raw, err := tesseractEngine(settings).Recognize(ctx, crop)
		if err == nil {
			return []string{raw}, false, nil
		}
		if settings.TesseractPath != "" {
			return nil, false, err
		}
	}
	readings, err = (ocr.Windows{Language: ocrLanguage(settings)}).RecognizeCandidates(ctx, crop)
	return readings, true, err
}

// bestItemReading is the reading whose best match is strongest. Without any
// match it is the first reading, so what was read stays on record.
func firstReading(readings []string) string {
	for _, raw := range readings {
		if strings.TrimSpace(raw) != "" {
			return raw
		}
	}
	return ""
}

func bestItemReading(readings []string, items []itemmatch.Item) (string, []itemmatch.Result) {
	bestRaw := firstReading(readings)
	var best []itemmatch.Result
	for _, raw := range readings {
		matches := itemmatch.Match(raw, items)
		if len(matches) == 0 {
			continue
		}
		if len(best) == 0 || matches[0].Confidence > best[0].Confidence {
			bestRaw, best = raw, matches
		}
	}
	return bestRaw, best
}

// bestQuestReading is bestItemReading for tasks.
func bestQuestReading(readings []string, quests []questmatch.Quest) (string, []questmatch.Result) {
	bestRaw := firstReading(readings)
	var best []questmatch.Result
	for _, raw := range readings {
		matches := questmatch.Match(raw, quests)
		if len(matches) == 0 {
			continue
		}
		if len(best) == 0 || matches[0].Confidence > best[0].Confidence {
			bestRaw, best = raw, matches
		}
	}
	return bestRaw, best
}

func topConfidence(matches []questmatch.Result) float64 {
	if len(matches) == 0 {
		return 0
	}
	return matches[0].Confidence
}

func (a *App) sendPosition(s config.Settings, p model.Position) error {
	var result error
	for _, id := range remoteTargetIDs(s, "map") {
		result = errors.Join(result, a.remoteClient(id).SendPosition(a.ctx, s.Map, p, s.NavigateMapOnShot))
	}
	return result
}

func (a *App) TestRemote() error {
	a.mu.RLock()
	s := a.settings
	a.mu.RUnlock()
	ids := remoteTargetIDs(s, "all")
	if len(ids) == 0 {
		return errors.New("Remote ID is required")
	}
	a.remoteOperationMu.Lock()
	defer a.remoteOperationMu.Unlock()
	if a.quitting.Load() {
		return context.Canceled
	}
	for _, id := range ids {
		if err := a.remoteClient(id).Connect(a.ctx); err != nil {
			a.setError(fmt.Errorf("%s: %w", id, err))
			return err
		}
	}
	a.mu.Lock()
	a.status.Connection = "connected"
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	a.addLog("Info", "Remote", fmt.Sprintf("Connection test succeeded for %d target(s)", len(ids)))
	return nil
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
