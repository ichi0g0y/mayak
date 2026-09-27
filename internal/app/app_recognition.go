package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/logdetect"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/position"
	"github.com/local/mayak/internal/taskdetect"
	"github.com/local/mayak/internal/watcher"
)

// Screenshot recognition: positions, task and item titles, OCR.

const recognitionSoundCooldown = 3 * time.Second

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
