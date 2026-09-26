package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/eftdetect"
	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/itemmatch"
	"github.com/local/mayak/internal/locale"
	"github.com/local/mayak/internal/logdetect"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/position"
	"github.com/local/mayak/internal/questapi"
	"github.com/local/mayak/internal/questmatch"
	"github.com/local/mayak/internal/taskdetect"
	"github.com/local/mayak/internal/watcher"
)

// Screenshot recognition: positions, task and item titles, OCR.

const recognitionSoundCooldown = 3 * time.Second

func tesseractUnavailable() bool {
	if _, ok := ocr.BundledTesseract(); ok {
		return false
	}
	_, err := exec.LookPath("tesseract")
	return err != nil
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
