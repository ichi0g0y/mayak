// Command ocreval measures OCR engines on MAYAK' recognition debug data
// (Screenshots/Mayak-Debug): each *_tasks_*_crop.png is read by every
// engine and compared with the quest the recognition matched.
//
//	go run ./cmd/ocreval -dir "<Screenshots>/Mayak-Debug" [-tesseract path\to\tesseract.exe] [-tessdata dir] [-lang en]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/questmatch"
)

type sample struct {
	crop  image.Image
	label string
	name  string
}

type engine struct {
	name      string
	recognize func(context.Context, image.Image) (string, error)
}

func main() {
	dir := flag.String("dir", "", "Mayak-Debug directory")
	tesseract := flag.String("tesseract", "", "tesseract.exe to compare (optional)")
	tessdata := flag.String("tessdata", "", "tessdata directory for -tesseract")
	// MAYAK' language code (see internal/ocr/language.go), not Tesseract's:
	// ja reads with jpn (or eftjpn from -tessdata) plus English.
	lang := flag.String("lang", "en", "the game's language: en, or another MAYAK language code (ja)")
	winLang := flag.String("winlang", "auto", "Windows OCR language: auto, ja, en")
	minConfidence := flag.Float64("min-confidence", .8, "only use samples whose recorded match was at least this confident")
	flag.Parse()
	samples, err := load(*dir, *minConfidence)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d labelled task crops\n\n", len(samples))

	engines := []engine{
		{"windows (new process each time)", func(ctx context.Context, img image.Image) (string, error) {
			ocr.StopWindowsWorker()
			return ocr.Windows{Language: *winLang}.Recognize(ctx, img)
		}},
		{"windows (resident worker)", ocr.Windows{Language: *winLang}.Recognize},
	}
	if *tesseract != "" {
		engines = append(engines, engine{"tesseract " + *lang, ocr.Tesseract{Executable: *tesseract, Language: *lang, DataDir: *tessdata}.Recognize})
	}
	ctx := context.Background()
	for _, e := range engines {
		// One untimed call warms caches, so the table shows steady-state cost.
		_, _ = e.recognize(ctx, samples[0].crop)
		var exact, matched int
		var total time.Duration
		for _, s := range samples {
			start := time.Now()
			text, err := e.recognize(ctx, s.crop)
			total += time.Since(start)
			if err != nil {
				fmt.Printf("  %-34s %s: error %v\n", e.name, s.name, err)
				continue
			}
			if questmatch.Normalize(text) == questmatch.Normalize(s.label) {
				exact++
			}
			if best := questmatch.Match(text, []questmatch.Quest{{Name: s.label}}); len(best) > 0 && best[0].Confidence >= .9 {
				matched++
			}
			if strings.TrimSpace(text) != s.label {
				fmt.Printf("  %-34s %-28q -> %q\n", e.name, s.label, text)
			}
		}
		n := len(samples)
		fmt.Printf("%-34s exact %2d/%d  close %2d/%d  avg %v\n\n", e.name, exact, n, matched, n, (total / time.Duration(n)).Round(time.Millisecond))
	}
	ocr.StopWindowsWorker()
}

// load reads Mayak-Debug records (*_tasks_*.json with *_crop.png) or crops
// collected by cmd/ocrharvest (*.json with a label next to the same *.png).
func load(dir string, minConfidence float64) ([]sample, error) {
	metas, err := filepath.Glob(filepath.Join(dir, "*_tasks_*.json"))
	if err != nil {
		return nil, err
	}
	cropSuffix := "_crop.png"
	if len(metas) == 0 {
		metas, _ = filepath.Glob(filepath.Join(dir, "*.json"))
		cropSuffix = ".png"
	}
	var samples []sample
	for _, meta := range metas {
		var record struct {
			Label string `json:"label"`
			Quest struct {
				Name       string  `json:"name"`
				Confidence float64 `json:"confidence"`
			} `json:"quest"`
		}
		data, err := os.ReadFile(meta)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(meta), err)
		}
		if record.Label != "" {
			record.Quest.Name, record.Quest.Confidence = record.Label, 1
		}
		if record.Quest.Name == "" || record.Quest.Confidence < minConfidence {
			continue
		}
		f, err := os.Open(strings.TrimSuffix(meta, ".json") + cropSuffix)
		if err != nil {
			continue
		}
		crop, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		samples = append(samples, sample{crop: crop, label: record.Quest.Name, name: filepath.Base(meta)})
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no labelled task crops in %s", dir)
	}
	return samples, nil
}
