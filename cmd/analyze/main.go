// Command analyze runs MAYAK' screenshot recognition on images and prints
// what it finds: the task list (with Windows OCR and the task catalog) and
// the item window (with the item catalog), with scores and readings.
//
//	go run ./cmd/analyze <image>...
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/local/mayak/internal/itemapi"
	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/itemmatch"
	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/questapi"
	"github.com/local/mayak/internal/questmatch"
	"github.com/local/mayak/internal/taskdetect"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: analyze <image>...")
	}
	for _, path := range os.Args[1:] {
		result, err := taskdetect.AnalyzeFile(path, taskdetect.Preset2560)
		var readings []string
		if err == nil && result.IsTasks {
			readings, err = (ocr.Windows{}).RecognizeCandidates(context.Background(), result.Crop)
		}
		raw := ""
		best := ""
		confidence := 0.0
		var catalogErr error
		if len(readings) > 0 {
			quests, err := questapi.New().Quests(context.Background())
			catalogErr = err
			base := make([]questmatch.Quest, len(quests))
			for i, quest := range quests {
				base[i] = quest.Quest
			}
			for _, reading := range readings {
				matches := questmatch.Match(reading, base)
				if len(matches) > 0 && matches[0].Confidence > confidence {
					raw, best, confidence = reading, matches[0].Quest.Name, matches[0].Confidence
				}
			}
		}
		fmt.Printf("%s tasks=%v layout=%s score=%.3f trader=%.3f character=%.3f tab=%.3f crop=%+v ocr=%q readings=%q match=%q confidence=%.3f err=%v catalogErr=%v\n", path, result.IsTasks, result.Layout, result.Score, result.TraderScore, result.CharacterScore, result.CharacterTabBright, result.CropRect, raw, readings, best, confidence, err, catalogErr)

		itemResult, itemErr := itemdetect.AnalyzeFile(path)
		itemRaw, itemBest := "", ""
		itemConfidence := 0.0
		if itemErr == nil && itemResult.IsItem {
			itemReadings, ocrErr := (ocr.Windows{}).RecognizeCandidates(context.Background(), itemResult.Crop)
			if ocrErr != nil {
				itemErr = ocrErr
			} else {
				items, loadErr := itemapi.New().ItemsForMode(context.Background(), "regular")
				if loadErr != nil {
					itemErr = loadErr
				} else {
					for _, reading := range itemReadings {
						matches := itemmatch.Match(reading, items)
						if len(matches) > 0 && matches[0].Confidence > itemConfidence {
							itemRaw, itemBest, itemConfidence = reading, matches[0].Item.Name, matches[0].Confidence
						}
					}
				}
			}
		}
		fmt.Printf("%s item=%v score=%.3f window=%+v crop=%+v ocr=%q match=%q confidence=%.3f err=%v\n", path, itemResult.IsItem, itemResult.Score, itemResult.WindowRect, itemResult.CropRect, itemRaw, itemBest, itemConfidence, itemErr)
	}
}
