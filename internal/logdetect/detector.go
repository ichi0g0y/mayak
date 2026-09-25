package logdetect

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	scenePattern    = regexp.MustCompile(`(?i)scene preset path:maps/([a-z0-9_]+)\.bundle`)
	locationPattern = regexp.MustCompile(`(?i)Location:\s*([^,|\s]+)`)
	logTimePattern  = regexp.MustCompile(`(?m)^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}).*$`)
	queuePattern    = regexp.MustCompile(`(?i)MatchingCompleted:[0-9]+(?:[.,][0-9]+)?\s+real:([0-9]+(?:[.,][0-9]+)?)`)
)

// RaidState derives the current state only from EFT logs. Coordinate metadata
// alone is not sufficient because EFT can include stale coordinates in menu
// screenshots after leaving a raid.
func RaidState(root string) (active bool, known bool) {
	files := latestSessionLogs(root)
	var latest time.Time
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		start := tailStart(path, 4*1024*1024)
		_, _ = f.Seek(start, io.SeekStart)
		data, _ := io.ReadAll(f)
		_ = f.Close()
		for _, line := range strings.Split(string(data), "\n") {
			lower := strings.ToLower(line)
			isStart := strings.Contains(lower, "|application|gamestarted:")
			isEnd := isRaidEndLine(lower)
			if !isStart && !isEnd {
				continue
			}
			match := logTimePattern.FindStringSubmatch(line)
			if len(match) < 2 {
				continue
			}
			stamp, err := time.ParseInLocation("2006-01-02 15:04:05.000", match[1], time.Local)
			if err == nil && (!known || stamp.After(latest)) {
				latest, known, active = stamp, true, isStart
			}
		}
	}
	return active, known
}

// isRaidEndLine covers both online raid completion and the profile reload that
// EFT writes after returning from local/PvE post-raid screens. Recent PvE builds
// do not consistently write UserMatchOver, so relying on it leaves the previous
// GameStarted marker active for the rest of the game session.
func isRaidEndLine(lower string) bool {
	return strings.Contains(lower, "usermatchover") ||
		strings.Contains(lower, "|application|prepareselectedprofilelocally profileid:") ||
		strings.Contains(lower, "|application|completeselectedprofile profileid:")
}

// LatestMap reads the most recent EFT application log instead of relying on
// detector state that may belong to a previous game process.
func LatestMap(root string) (string, bool) {
	path := latestLog(root)
	if path == "" {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	_, _ = f.Seek(tailStart(path, 4*1024*1024), io.SeekStart)
	data, err := io.ReadAll(f)
	if err != nil {
		return "", false
	}
	return ParseMap(string(data))
}

func latestSessionLogs(root string) []string {
	application := latestLog(root)
	if application == "" {
		return nil
	}
	dir := filepath.Dir(application)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{application}
	}
	files := make([]string, 0, 2)
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if !entry.IsDir() && strings.HasSuffix(name, ".log") && (strings.Contains(name, "application") || strings.Contains(name, "backend")) {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

var aliases = map[string]string{
	"bigmap": "customs", "customs": "customs",
	"factory4_day": "factory", "factory_day": "factory", "factory": "factory",
	"factory4_night": "night-factory", "factory_night": "night-factory",
	"sandbox": "ground-zero", "groundzero": "ground-zero",
	"sandbox_high": "ground-zero-21", "sandbox_start": "ground-zero-tutorial",
	"interchange": "interchange", "shopping_mall": "interchange",
	"laboratory": "the-lab", "laboratory_dark": "the-lab-dark", "labyrinth": "the-labyrinth",
	"lighthouse": "lighthouse", "rezervbase": "reserve", "rezerv_base": "reserve", "reserve": "reserve",
	"shoreline": "shoreline", "tarkovstreets": "streets-of-tarkov", "streets": "streets-of-tarkov", "city": "streets-of-tarkov",
	"icebreaker": "icebreaker", "terminal": "terminal", "woods": "woods",
}

func ParseMap(text string) (string, bool) {
	type candidate struct {
		index int
		raw   string
	}
	var latest candidate
	found := false
	for _, pattern := range []*regexp.Regexp{scenePattern, locationPattern} {
		for _, match := range pattern.FindAllStringSubmatchIndex(text, -1) {
			if len(match) < 4 || (found && match[0] <= latest.index) {
				continue
			}
			latest = candidate{index: match[0], raw: text[match[2]:match[3]]}
			found = true
		}
	}
	if found {
		return normalize(latest.raw)
	}
	return "", false
}

func normalize(raw string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(raw))
	key = strings.TrimSuffix(key, "_preset")
	v, ok := aliases[key]
	return v, ok
}

type Detector struct {
	root          string
	mapCallback   func(string)
	eventCallback func(Event)
	cancel        context.CancelFunc
	once          sync.Once
}

type EventKind string

type Event struct {
	Kind               EventKind
	RunThroughEligible bool
	OccurredAt         time.Time
	QueueSeconds       float64
}

const (
	MatchFound  EventKind = "matchFound"
	RaidStarted EventKind = "raidStarted"
	RaidExited  EventKind = "raidExited"
)

type EventParser struct {
	gameStarting time.Time
	raidActive   bool
}

type Snapshot struct {
	Active    bool
	StartedAt time.Time
	// LastStartedAt is when the latest raid started, whether or not it ended.
	LastStartedAt      time.Time
	RunThroughEligible bool
	LastQueueSeconds   float64
}

// LatestSnapshot reconstructs timer state without replaying alerts when
// MAYAK starts while EFT is already in a raid.
func LatestSnapshot(root string) Snapshot {
	var snapshot Snapshot
	var latestEnd time.Time
	var latestStart time.Time
	for _, path := range latestSessionLogs(root) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(filepath.Base(path)), "application") {
			parser := EventParser{}
			for _, event := range parser.Parse(string(data)) {
				switch event.Kind {
				case MatchFound:
					if event.QueueSeconds > 0 {
						snapshot.LastQueueSeconds = event.QueueSeconds
					}
				case RaidStarted:
					if event.OccurredAt.After(latestStart) {
						latestStart = event.OccurredAt
						snapshot.RunThroughEligible = event.RunThroughEligible
					}
				}
			}
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !isRaidEndLine(strings.ToLower(line)) {
				continue
			}
			if stamp, ok := parseLogTime(line); ok && stamp.After(latestEnd) {
				latestEnd = stamp
			}
		}
	}
	snapshot.LastStartedAt = latestStart
	if !latestStart.IsZero() && latestStart.After(latestEnd) {
		snapshot.Active = true
		snapshot.StartedAt = latestStart
	}
	return snapshot
}

func New(root string, mapCallback func(string), eventCallback func(Event)) *Detector {
	return &Detector{root: root, mapCallback: mapCallback, eventCallback: eventCallback}
}
func (d *Detector) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	d.cancel = cancel
	go d.loop(ctx)
}
func (d *Detector) Close() {
	d.once.Do(func() {
		if d.cancel != nil {
			d.cancel()
		}
	})
}

func (d *Detector) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var current string
	var detectedMap string
	var eventParser EventParser
	initialized := false
	var offset int64
	var nextSessionDiscovery time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			path := current
			now := time.Now()
			if current == "" || !now.Before(nextSessionDiscovery) {
				if candidate := latestLog(d.root); candidate != "" {
					path = candidate
				}
				nextSessionDiscovery = now.Add(2 * time.Second)
			}
			if path == "" {
				continue
			}
			if path != current {
				current = path
				detectedMap = ""
				eventParser = EventParser{}
				initialized = false
				offset = tailStart(path, 1024*1024)
			}
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			info, err := f.Stat()
			if err != nil {
				_ = f.Close()
				continue
			}
			if info.Size() < offset {
				offset = 0
			}
			_, _ = f.Seek(offset, io.SeekStart)
			data, _ := io.ReadAll(f)
			_ = f.Close()
			offset = info.Size()
			text := string(data)
			if mapName, notify := mapUpdate(text, detectedMap); notify {
				detectedMap = mapName
				if d.mapCallback != nil {
					d.mapCallback(mapName)
				}
			}
			events := eventParser.Parse(text)
			if initialized && d.eventCallback != nil {
				for _, event := range events {
					d.eventCallback(event)
				}
			}
			initialized = true
		}
	}
}

func (p *EventParser) Parse(text string) []Event {
	events := make([]Event, 0, 2)
	for _, line := range strings.Split(text, "\n") {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "|application|gamestarting:"):
			if stamp, ok := parseLogTime(line); ok {
				p.gameStarting = stamp
			}
		case strings.Contains(lower, "|application|matchingcompleted:"):
			seconds := 0.0
			if match := queuePattern.FindStringSubmatch(line); len(match) == 2 {
				seconds, _ = strconv.ParseFloat(strings.ReplaceAll(match[1], ",", "."), 64)
			}
			stamp, _ := parseLogTime(line)
			events = append(events, Event{Kind: MatchFound, OccurredAt: stamp, QueueSeconds: seconds})
		case strings.Contains(lower, "|application|gamestarted:"):
			started, ok := parseLogTime(line)
			eligible := ok && !p.gameStarting.IsZero() && started.Sub(p.gameStarting) > 3*time.Second
			events = append(events, Event{Kind: RaidStarted, RunThroughEligible: eligible, OccurredAt: started})
			p.gameStarting = time.Time{}
			p.raidActive = true
		case isRaidEndLine(lower):
			if p.raidActive {
				stamp, _ := parseLogTime(line)
				events = append(events, Event{Kind: RaidExited, OccurredAt: stamp})
				p.raidActive = false
			}
		}
	}
	return events
}

func parseLogTime(line string) (time.Time, bool) {
	match := logTimePattern.FindStringSubmatch(line)
	if len(match) < 2 {
		return time.Time{}, false
	}
	stamp, err := time.ParseInLocation("2006-01-02 15:04:05.000", match[1], time.Local)
	return stamp, err == nil
}

func mapUpdate(text, currentMap string) (string, bool) {
	if mapName, ok := ParseMap(text); ok {
		return mapName, true
	}
	if currentMap != "" && strings.Contains(strings.ToLower(text), "|application|gamestarted:") {
		return currentMap, true
	}
	return currentMap, false
}

func tailStart(path string, max int64) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if info.Size() > max {
		return info.Size() - max
	}
	return 0
}
func latestLog(root string) string {
	var found string
	var newest time.Time
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".log") || !strings.Contains(name, "application") {
			return nil
		}
		info, e := entry.Info()
		if e == nil && info.ModTime().After(newest) {
			found = path
			newest = info.ModTime()
		}
		return nil
	})
	return found
}
