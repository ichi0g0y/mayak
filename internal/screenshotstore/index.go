package screenshotstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// maxRecords bounds the index; the oldest records go first.
const maxRecords = 3000

// Record is what MAYAK made of one screenshot, kept for the screenshot
// list: what it was recognized as, how sure it was, and what it matched.
type Record struct {
	At     time.Time `json:"at"`
	Type   string    `json:"type"` // tasks, item, position or unknown
	Layout string    `json:"layout,omitempty"`
	Score  float64   `json:"score,omitempty"`
	Map    string    `json:"map,omitempty"`
	Raid   bool      `json:"raid"`
	Stage  string    `json:"stage,omitempty"`
	// Match is the task or item recognized; Detail its trader or short name.
	Match      string   `json:"match,omitempty"`
	MatchID    string   `json:"matchId,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	OCR        string   `json:"ocr,omitempty"`
	Position   string   `json:"position,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// Index keeps the records of the screenshots by file name, in a JSON file.
type Index struct {
	path    string
	mu      sync.Mutex
	loaded  bool
	records map[string]Record
}

func NewIndex(path string) *Index { return &Index{path: path} }

func (x *Index) load() {
	if x.loaded {
		return
	}
	x.loaded = true
	x.records = map[string]Record{}
	data, err := os.ReadFile(x.path)
	if err != nil {
		return
	}
	var records map[string]Record
	if json.Unmarshal(data, &records) == nil && records != nil {
		x.records = records
	}
}

// Put records a screenshot's analysis, replacing an earlier one.
func (x *Index) Put(name string, r Record) error {
	if name == "" || name != filepath.Base(name) {
		return errors.New("invalid screenshot name")
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.load()
	x.records[name] = r
	if len(x.records) > maxRecords {
		names := make([]string, 0, len(x.records))
		for n := range x.records {
			names = append(names, n)
		}
		sort.Slice(names, func(a, b int) bool { return x.records[names[a]].At.Before(x.records[names[b]].At) })
		for _, n := range names[:len(names)-maxRecords] {
			delete(x.records, n)
		}
	}
	data, err := json.Marshal(x.records)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(x.path), 0700); err != nil {
		return err
	}
	return writeAtomic(x.path, data)
}

// Get returns the record of a screenshot, if there is one.
func (x *Index) Get(name string) (Record, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.load()
	r, ok := x.records[name]
	return r, ok
}

// Fill adds the records of screenshots the index does not know yet.
func (x *Index) Fill(records map[string]Record) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.load()
	added := false
	for name, r := range records {
		if _, ok := x.records[name]; !ok && name == filepath.Base(name) && len(x.records) < maxRecords {
			x.records[name] = r
			added = true
		}
	}
	if !added {
		return nil
	}
	data, err := json.Marshal(x.records)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(x.path), 0700); err != nil {
		return err
	}
	return writeAtomic(x.path, data)
}

// DebugRecords reads the records of screenshots from the recognition debug
// data saved next to them (SaveDebug), by screenshot name: the analyses made
// before the index kept them.
func DebugRecords(root string) map[string]Record {
	out := map[string]Record{}
	// The debug data of the app's earlier name counts too.
	for _, name := range []string{DebugDirectory, legacyDebugDirectory} {
		readDebugRecords(filepath.Join(root, name), out)
	}
	return out
}

func readDebugRecords(dir string, out map[string]Record) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type match struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Trader     string  `json:"trader"`
		ShortName  string  `json:"shortName"`
		Confidence float64 `json:"confidence"`
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil || len(data) > 4<<20 {
			continue
		}
		var m struct {
			CapturedAt      time.Time                  `json:"capturedAt"`
			SourceFile      string                     `json:"sourceFile"`
			ScreenshotType  string                     `json:"screenshotType"`
			RaidActive      bool                       `json:"raidActive"`
			Map             string                     `json:"map"`
			DetectionLayout string                     `json:"detectionLayout"`
			DetectionScore  float64                    `json:"detectionScore"`
			AnalysisStage   string                     `json:"analysisStage"`
			OCRRaw          string                     `json:"ocrRaw"`
			Quest           *match                     `json:"quest"`
			QuestCandidates []match                    `json:"questCandidates"`
			Item            *match                     `json:"item"`
			ItemCandidates  []match                    `json:"itemCandidates"`
			Position        *struct{ X, Y, Z float64 } `json:"position"`
			Error           string                     `json:"error"`
		}
		if json.Unmarshal(data, &m) != nil || m.SourceFile == "" {
			continue
		}
		r := Record{At: m.CapturedAt, Type: m.ScreenshotType, Layout: m.DetectionLayout, Score: m.DetectionScore, Map: m.Map, Raid: m.RaidActive, Stage: m.AnalysisStage, Error: m.Error}
		found, candidates := m.Quest, m.QuestCandidates
		if m.ScreenshotType == "item" {
			found, candidates = m.Item, m.ItemCandidates
		}
		if found != nil && (m.ScreenshotType == "tasks" || m.ScreenshotType == "item") {
			r.Match, r.MatchID, r.Confidence = found.Name, found.ID, found.Confidence
			r.Detail = found.Trader
			if found.ShortName != "" {
				r.Detail = found.ShortName
			}
			for _, c := range candidates[:min(3, len(candidates))] {
				r.Candidates = append(r.Candidates, fmt.Sprintf("%s %.0f%%", c.Name, c.Confidence*100))
			}
		}
		if ocr := []rune(m.OCRRaw); len(ocr) > 400 {
			r.OCR = string(ocr[:400]) + "…"
		} else {
			r.OCR = m.OCRRaw
		}
		if m.Position != nil && m.ScreenshotType == "position" {
			r.Position = fmt.Sprintf("%.1f, %.1f, %.1f", m.Position.X, m.Position.Y, m.Position.Z)
		}
		// The newest analysis of a screenshot wins.
		if old, ok := out[m.SourceFile]; !ok || old.At.Before(r.At) {
			out[m.SourceFile] = r
		}
	}
}
