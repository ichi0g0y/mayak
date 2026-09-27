package app

import (
	"context"
	"fmt"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/itemmatch"
	"github.com/local/mayak/internal/logdetect"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/ocr"
)

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
