package app

import (
	"fmt"
	"github.com/local/mayak/internal/appdir"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/screenshotstore"
	"github.com/local/mayak/internal/taskdetect"
)

func (a *App) afterScreenshotAnalysis(sequence uint64, path string, settings config.Settings) {
	a.mu.RLock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.RUnlock()
		return
	}
	status := a.status
	a.mu.RUnlock()
	a.screenshotStoreMu.Lock()
	defer a.screenshotStoreMu.Unlock()
	// What the screenshot was recognized as, for the screenshot list.
	if a.screenshotIndex != nil {
		if err := a.screenshotIndex.Put(filepath.Base(path), screenshotRecord(status)); err != nil {
			a.addLog("Warn", "Screenshot", "Could not record the screenshot analysis: "+err.Error())
		} else {
			a.emitEvent("browser:screenshot", filepath.Base(path))
		}
	}
	if settings.SaveRecognitionDebug {
		metadata := screenshotMetadata(path, status)
		metadata.DetectionDetails = recognitionDetails(path)
		metaPath, err := screenshotstore.SaveDebug(settings.ScreenshotDirectory, path, metadata, status.CropPreview)
		if err != nil {
			a.addLog("Warn", "Screenshot", "Could not save recognition debug data: "+err.Error())
		} else {
			a.addLog("Debug", "Screenshot", "Saved recognition debug data to "+filepath.Base(metaPath))
		}
	}
	a.cleanupScreenshotsLocked(settings, path)
}

// recognitionDetails records both detectors' findings for the debug data;
// the screenshot is decoded once for both.
func recognitionDetails(path string) map[string]any {
	details := make(map[string]any)
	img, err := decodeImage(path)
	if err != nil {
		details["error"] = err.Error()
		return details
	}
	if task, err := taskdetect.Analyze(img, taskdetect.Preset2560); err == nil {
		details["tasks"] = map[string]any{
			"matched": task.IsTasks, "score": task.Score, "layout": task.Layout,
			"traderScore": task.TraderScore, "characterScore": task.CharacterScore,
			"characterTabBrightness": task.CharacterTabBright,
			"crop":                   map[string]int{"x": task.CropRect.X, "y": task.CropRect.Y, "width": task.CropRect.W, "height": task.CropRect.H},
		}
	} else {
		details["tasksError"] = err.Error()
	}
	if item, err := itemdetect.Analyze(img); err == nil {
		details["item"] = map[string]any{
			"matched": item.IsItem, "score": item.Score,
			"window": map[string]int{"x": item.WindowRect.X, "y": item.WindowRect.Y, "width": item.WindowRect.W, "height": item.WindowRect.H},
			"crop":   map[string]int{"x": item.CropRect.X, "y": item.CropRect.Y, "width": item.CropRect.W, "height": item.CropRect.H},
		}
	} else {
		details["itemError"] = err.Error()
	}
	return details
}

func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	return img, err
}

func screenshotMetadata(path string, status model.Status) screenshotstore.Metadata {
	metadata := screenshotstore.Metadata{
		CapturedAt: time.Now(), SourceFile: filepath.Base(path), ScreenshotType: status.ScreenshotType,
		RaidActive: status.RaidActive, Map: status.CurrentMap, DetectionLayout: status.DetectionLayout,
		DetectionScore: status.DetectionScore, AnalysisStage: status.AnalysisStage, Error: status.LastError,
	}
	switch status.ScreenshotType {
	case "tasks":
		metadata.OCRRaw = status.OCRRaw
		metadata.Quest = map[string]any{"id": status.QuestID, "name": status.LastQuest, "trader": status.QuestTrader, "map": status.QuestMap, "confidence": status.MatchConfidence}
		metadata.QuestCandidates = status.QuestCandidates
	case "item":
		metadata.OCRRaw = status.OCRRaw
		metadata.Item = map[string]any{"id": status.ItemID, "name": status.LastItem, "shortName": status.ItemShortName, "confidence": status.ItemConfidence}
		metadata.ItemCandidates = status.ItemCandidates
	case "position":
		metadata.Position = status.Position
	}
	return metadata
}

// screenshotIndexPath is where the screenshots' analysis records are kept.
func screenshotIndexPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, appdir.Name, "screenshots.json")
}

// screenshotRecord is the part of the analysis the screenshot list shows.
func screenshotRecord(status model.Status) screenshotstore.Record {
	r := screenshotstore.Record{
		At: time.Now().UTC(), Type: status.ScreenshotType, Layout: status.DetectionLayout, Score: status.DetectionScore,
		Map: status.CurrentMap, Raid: status.RaidActive, Stage: status.AnalysisStage, Error: status.LastError,
	}
	switch status.ScreenshotType {
	case "tasks":
		r.Match, r.MatchID, r.Detail, r.Confidence, r.OCR = status.LastQuest, status.QuestID, status.QuestTrader, status.MatchConfidence, trimOCR(status.OCRRaw)
		for _, c := range status.QuestCandidates[:min(3, len(status.QuestCandidates))] {
			r.Candidates = append(r.Candidates, fmt.Sprintf("%s %.0f%%", c.Name, c.Confidence*100))
		}
	case "item":
		r.Match, r.MatchID, r.Detail, r.Confidence, r.OCR = status.LastItem, status.ItemID, status.ItemShortName, status.ItemConfidence, trimOCR(status.OCRRaw)
		for _, c := range status.ItemCandidates[:min(3, len(status.ItemCandidates))] {
			r.Candidates = append(r.Candidates, fmt.Sprintf("%s %.0f%%", c.Name, c.Confidence*100))
		}
	case "position":
		if p := status.Position; p != nil {
			r.Position = fmt.Sprintf("%.1f, %.1f, %.1f", p.X, p.Y, p.Z)
		}
	}
	return r
}

func trimOCR(text string) string {
	if runes := []rune(text); len(runes) > 400 {
		return string(runes[:400]) + "…"
	}
	return text
}

func (a *App) cleanupScreenshotsLocked(settings config.Settings, protect string) {
	if a.browserClient.Load() {
		return
	}
	if !settings.ScreenshotCleanup || settings.ScreenshotDirectory == "" {
		return
	}
	deleted, err := screenshotstore.Cleanup(settings.ScreenshotDirectory, screenshotstore.CleanupOptions{
		MaxCount: settings.ScreenshotRetainCount,
		MaxAge:   time.Duration(settings.ScreenshotRetainHours) * time.Hour,
		Protect:  protect,
	})
	if err != nil {
		a.addLog("Warn", "Screenshot", "Automatic screenshot cleanup failed: "+err.Error())
		return
	}
	if len(deleted) > 0 {
		a.addLog("Info", "Screenshot", fmt.Sprintf("Automatically deleted %d old screenshot(s)", len(deleted)))
	}
}

func (a *App) runScreenshotMaintenance() {
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	a.screenshotStoreMu.Lock()
	a.cleanupScreenshotsLocked(settings, "")
	a.screenshotStoreMu.Unlock()
}

func (a *App) watchScreenshotMaintenance() {
	a.runScreenshotMaintenance()
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-ticker.C:
			a.runScreenshotMaintenance()
		}
	}
}
