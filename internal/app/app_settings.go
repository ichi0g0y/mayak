package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/local/mayak/internal/autostart"
	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/eftdetect"
	"github.com/local/mayak/internal/locale"
	"github.com/local/mayak/internal/screenshotstore"
	"github.com/local/mayak/internal/sound"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Settings: folders, sounds, validation and the save paths.

// GameLanguages are the game languages MAYAK reads (settings' choices):
// English and those in internal/locale.
func (a *App) GameLanguages() []string { return append([]string{"en"}, locale.Languages...) }

func (a *App) GetSettings() config.Settings { a.mu.RLock(); defer a.mu.RUnlock(); return a.settings }

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

// SaveSettings stores s and applies it, folders included: a running monitor
// moves to changed folders. It is the settings page's explicit save.
func (a *App) SaveSettings(s config.Settings) error { return a.saveSettings(s, true) }

// PersistSettings stores s and applies it as an edit is made in the settings
// page, without restarting the monitor for a changed folder (SaveSettings
// does that).
func (a *App) PersistSettings(s config.Settings) error { return a.saveSettings(s, false) }

// saveSettings validates s, writes it and applies what changed: the
// autostart entry, TarkovTracker, the catalog's game mode, updates, the
// tray's language, the map marker's style sheet, the screenshot cleanup and,
// with restartMonitor, the monitored folders.
func (a *App) saveSettings(s config.Settings, restartMonitor bool) error {
	a.settingsWriteMu.Lock()
	defer a.settingsWriteMu.Unlock()
	s = normalizeSettings(s)
	a.mu.Lock()
	old := a.settings
	monitoring := a.status.Monitoring
	s = a.keepWindowSettings(s)
	a.mu.Unlock()
	if err := applyAutostartChange(old.LaunchAtStartup, s.LaunchAtStartup); err != nil {
		return err
	}
	if err := config.Save(s); err != nil {
		_ = applyAutostartChange(s.LaunchAtStartup, old.LaunchAtStartup)
		return err
	}
	a.mu.Lock()
	a.settings = s
	a.applyTrackerTokenFlagsLocked()
	a.updateTrackerConnectionLocked()
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	if !old.TarkovTrackerEnabled && s.TarkovTrackerEnabled {
		go func() { _ = a.RefreshTracker() }()
	}
	if old.GameMode != s.GameMode {
		go func() { _ = a.refreshCatalog(false) }()
	}
	if !old.AutoUpdate && s.AutoUpdate {
		go func() { _, _ = a.checkForUpdates(true) }()
	}
	if old.Language != s.Language {
		a.setTrayLanguage(s.Language)
	}
	// The map view is created again with the new marker style (api.js).
	if old.PlayerMarkerEffect != s.PlayerMarkerEffect || old.PlayerMarkerColor != s.PlayerMarkerColor {
		a.applyBrowserScript()
		a.emitEvent("browser:document-script")
	}
	if old.ScreenshotCleanup != s.ScreenshotCleanup || old.ScreenshotRetainCount != s.ScreenshotRetainCount || old.ScreenshotRetainHours != s.ScreenshotRetainHours {
		go a.runScreenshotMaintenance()
	}
	if !restartMonitor || !monitoring {
		return nil
	}
	if old.LogsDirectory != s.LogsDirectory {
		a.startLogDetector(s.LogsDirectory)
	}
	if old.ScreenshotDirectory != s.ScreenshotDirectory {
		if s.ScreenshotDirectory == "" {
			a.StopMonitoring()
			return nil
		}
		return a.startWatcher(s.ScreenshotDirectory)
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
	if s.PlayerMarker != "" && s.PlayerMarkerEffect == "" {
		s.PlayerMarkerEffect, s.PlayerMarkerColor = legacyPlayerMarker(s.PlayerMarker)
	}
	s.PlayerMarker = ""
	s.PlayerMarkerEffect = normalizePlayerMarkerEffect(s.PlayerMarkerEffect)
	s.PlayerMarkerColor = normalizePlayerMarkerColor(s.PlayerMarkerColor)
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
