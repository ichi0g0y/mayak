// Command ocrharvest collects labelled title crops from Escape from Tarkov
// screenshots for OCR evaluation and training. For every screenshot with a
// task list or an item inspection window it reads the title with several OCR
// engines and matches the readings against a MAYAK catalog. A crop is kept
// only when the engines agree on a confident match (or one reading is exact),
// so labels are trustworthy without manual work.
//
//	go run ./cmd/ocrharvest -screens "<Documents>\Escape from Tarkov\Screenshots" \
//	  -catalog %AppData%\Mayak\catalog\pve.json -tesseract build\bin\tesseract -out <dir>
//
// It writes <out>/<kind>_<n>.png (the original crop) with <kind>_<n>.json.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/itemmatch"
	"github.com/local/mayak/internal/locale"
	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/questmatch"
	"github.com/local/mayak/internal/taskdetect"
)

type engine struct {
	name string
	read func(context.Context, image.Image) (string, error)
}

type record struct {
	Kind     string            `json:"kind"`
	Label    string            `json:"label"`
	Source   string            `json:"source"`
	Readings map[string]string `json:"readings"`
}

func main() {
	screens := flag.String("screens", "", "Escape from Tarkov screenshot directory")
	catalog := flag.String("catalog", "", "MAYAK catalog snapshot")
	tesseract := flag.String("tesseract", "build/bin/tesseract", "bundled Tesseract directory (tesseract.exe, tessdata)")
	out := flag.String("out", "", "output directory")
	lang := flag.String("lang", "en", "the game's language: en, or another MAYAK language code (ja)")
	minConfidence := flag.Float64("min-confidence", .9, "minimum match confidence")
	flag.Parse()
	if *screens == "" || *catalog == "" || *out == "" {
		log.Fatal("-screens, -catalog and -out are required")
	}
	quests, items, err := load(*catalog)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	exe := filepath.Join(*tesseract, "tesseract.exe")
	data := filepath.Join(*tesseract, "tessdata")
	engines := []engine{
		{"windows", ocr.Windows{Language: *lang}.Recognize},
		{"eft", ocr.Tesseract{Executable: exe, DataDir: data, Language: *lang}.Recognize},
		// Without a data directory Tesseract uses its own tessdata (eng).
		{"eng", ocr.Tesseract{Executable: exe, Language: *lang}.Recognize},
	}
	defer ocr.StopWindowsWorker()

	files, _ := filepath.Glob(filepath.Join(*screens, "*.png"))
	// The same formats the screenshot watcher takes.
	for _, pattern := range []string{"*.jpg", "*.jpeg"} {
		more, _ := filepath.Glob(filepath.Join(*screens, pattern))
		files = append(files, more...)
	}
	sort.Strings(files)
	ctx := context.Background()
	counts := map[string]int{}
	for i, file := range files {
		kind, crop := detect(file)
		if crop == nil {
			continue
		}
		counts[kind+" found"]++
		readings := map[string]string{}
		votes := map[string]int{}
		exact := ""
		for _, e := range engines {
			text, err := e.read(ctx, crop)
			if err != nil {
				log.Printf("%s: %s: %v", filepath.Base(file), e.name, err)
				continue
			}
			readings[e.name] = text
			label, confidence := best(kind, text, quests, items)
			if confidence >= *minConfidence {
				votes[label]++
			}
			if confidence >= .999 {
				exact = label
			}
		}
		label := exact
		for candidate, n := range votes {
			if n >= 2 && (label == "" || label == candidate) {
				label = candidate
			}
		}
		if label == "" {
			fmt.Printf("  unlabelled %s %s %q\n", kind, filepath.Base(file), readings)
			continue
		}
		counts[kind+" labelled"]++
		base := filepath.Join(*out, fmt.Sprintf("%s_%04d", kind, i))
		must(writePNG(base+".png", crop))
		data, _ := json.MarshalIndent(record{Kind: kind, Label: label, Source: filepath.Base(file), Readings: readings}, "", "  ")
		must(os.WriteFile(base+".json", data, 0o644))
	}
	fmt.Printf("%d screenshots: %v\n", len(files), counts)
}

// detect returns the title crop of an item inspection window or a task list,
// in the same order MAYAK classifies screenshots.
func detect(path string) (string, image.Image) {
	if item, err := itemdetect.AnalyzeFile(path); err == nil && item.IsItem && item.Crop != nil {
		return "item", item.Crop
	}
	if task, err := taskdetect.AnalyzeFile(path, taskdetect.Preset2560); err == nil && task.IsTasks && task.Crop != nil {
		return "task", task.Crop
	}
	return "", nil
}

func best(kind, text string, quests []questmatch.Quest, items []itemmatch.Item) (string, float64) {
	if kind == "task" {
		if r := questmatch.Match(text, quests); len(r) > 0 {
			return shown(text, append([]string{r[0].Quest.Name}, r[0].Quest.Aliases...)), r[0].Confidence
		}
		return "", 0
	}
	if r := itemmatch.Match(text, items); len(r) > 0 {
		return shown(text, append([]string{r[0].Item.Name}, r[0].Item.Aliases...)), r[0].Confidence
	}
	return "", 0
}

// shown is the one of a match's names (English, Japanese) the reading is
// closest to: the text the game showed, which is what the crop is labelled.
func shown(text string, names []string) string {
	best, score := names[0], -1.0
	for _, name := range names {
		if s := questmatch.Similarity(text, name); s > score {
			best, score = name, s
		}
	}
	return best
}

func load(path string) ([]questmatch.Quest, []itemmatch.Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var snapshot struct {
		Resources struct {
			Tasks struct {
				Data struct {
					Tasks map[string]struct{ ID, Name string } `json:"tasks"`
				} `json:"data"`
			} `json:"tasks"`
			TasksEN struct {
				Data map[string]string `json:"data"`
			} `json:"tasks_en"`
			Items struct {
				Data struct {
					Items map[string]struct{ ID, Name, ShortName string } `json:"items"`
				} `json:"data"`
			} `json:"items"`
			ItemsEN struct {
				Data map[string]string `json:"data"`
			} `json:"items_en"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, nil, err
	}
	r := snapshot.Resources
	// The names in the other languages (see internal/locale).
	names, err := localeNames(data)
	if err != nil {
		return nil, nil, err
	}
	aliases := func(base, key, english string) []string {
		var out []string
		for _, lang := range locale.Languages {
			if name := strings.TrimSpace(names[locale.Resource(base, lang)][key]); name != "" && name != english {
				out = append(out, name)
			}
		}
		return out
	}
	var quests []questmatch.Quest
	for _, t := range r.Tasks.Data.Tasks {
		if name := strings.TrimSpace(r.TasksEN.Data[t.Name]); name != "" {
			quests = append(quests, questmatch.Quest{ID: t.ID, Name: name, Aliases: aliases("tasks", t.Name, name)})
		}
	}
	var items []itemmatch.Item
	for _, it := range r.Items.Data.Items {
		if name := strings.TrimSpace(r.ItemsEN.Data[it.Name]); name != "" {
			items = append(items, itemmatch.Item{ID: it.ID, Name: name, ShortName: strings.TrimSpace(r.ItemsEN.Data[it.ShortName]), Aliases: aliases("items", it.Name, name)})
		}
	}
	return quests, items, nil
}

// localeNames reads the snapshot's names in other languages, by resource.
func localeNames(data []byte) (map[string]map[string]string, error) {
	var snapshot struct {
		Resources map[string]json.RawMessage `json:"resources"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	for _, lang := range locale.Languages {
		for _, base := range []string{"tasks", "items"} {
			var resource struct {
				Data map[string]string `json:"data"`
			}
			if raw := snapshot.Resources[locale.Resource(base, lang)]; len(raw) > 0 && json.Unmarshal(raw, &resource) == nil {
				out[locale.Resource(base, lang)] = resource.Data
			}
		}
	}
	return out, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err = png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
