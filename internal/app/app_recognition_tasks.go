package app

import (
	"context"
	"fmt"
	"image"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/logdetect"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/questapi"
	"github.com/local/mayak/internal/questmatch"
	"github.com/local/mayak/internal/sound"
	"github.com/local/mayak/internal/taskdetect"
)

// handleTaskScreenshot handles screenshots without position metadata: an open
// item inspection window first, otherwise the task list.
func (a *App) handleTaskScreenshot(ctx context.Context, sequence uint64, path string, settings config.Settings) {
	// The character's Overall screen has a reading of its own (app_profile.go).
	if a.tryProfileScreenshot(ctx, sequence, path, settings) {
		return
	}
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
	// The list goes in its own queue first: a next screenshot may stop this
	// analysis before its end.
	if detected.IsTasks {
		a.queueTaskList(path, settings)
	}
	a.handleTaskAnalysis(ctx, sequence, path, settings, detected, nil)
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
		a.notifyOnce(sound.Quest, status.QuestID)
	} else {
		a.notifyOnce(sound.TaskNotMatched, raw)
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
