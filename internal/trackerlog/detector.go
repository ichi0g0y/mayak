package trackerlog

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type EventKind string

const (
	ProfileDetected EventKind = "profileDetected"
	TaskChanged     EventKind = "taskChanged"
)

type Event struct {
	Kind      EventKind `json:"kind"`
	Mode      string    `json:"mode,omitempty"`
	ProfileID string    `json:"profileId,omitempty"`
	AccountID string    `json:"accountId,omitempty"`
	TaskID    string    `json:"taskId,omitempty"`
	TaskState string    `json:"taskState,omitempty"`
	SeenAt    time.Time `json:"-"`
}

type ObservedProfile struct {
	AccountID string
	ProfileID string
	Mode      string
	FirstSeen time.Time
	LastSeen  time.Time
}

// DiscoverProfiles reads existing EFT application logs and returns every
// complete account/profile/mode identity observed on this PC.
func DiscoverProfiles(root string) []ObservedProfile {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	seen := make(map[string]ObservedProfile)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		path := findLog(dir, "application")
		if path == "" {
			path = findLog(dir, "output")
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		seenAt := time.Time{}
		if info, infoErr := os.Stat(path); infoErr == nil {
			seenAt = info.ModTime().UTC()
		}
		parser := IdentityParser{}
		for _, event := range parser.Parse(string(data)) {
			key := event.AccountID + "\x00" + event.ProfileID + "\x00" + event.Mode
			current, exists := seen[key]
			if !exists {
				seen[key] = ObservedProfile{AccountID: event.AccountID, ProfileID: event.ProfileID, Mode: event.Mode, FirstSeen: seenAt, LastSeen: seenAt}
				continue
			}
			if seenAt.Before(current.FirstSeen) {
				current.FirstSeen = seenAt
			}
			if seenAt.After(current.LastSeen) {
				current.LastSeen = seenAt
			}
			seen[key] = current
		}
	}
	profiles := make([]ObservedProfile, 0, len(seen))
	for _, profile := range seen {
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].LastSeen.After(profiles[j].LastSeen) })
	return profiles
}

type Detector struct {
	root     string
	callback func(Event)
	cancel   context.CancelFunc
	once     sync.Once
}

func New(root string, callback func(Event)) *Detector {
	return &Detector{root: root, callback: callback}
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
	var session string
	var applicationPath, outputPath string
	var applicationOffset, outputOffset int64
	var identity IdentityParser
	var tasks TaskParser
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			latest := latestSession(d.root)
			if latest == "" {
				continue
			}
			if latest != session {
				session = latest
				applicationPath = findLog(session, "application")
				outputPath = findLog(session, "output")
				applicationOffset = 0
				outputOffset = fileSize(outputPath)
				identity = IdentityParser{}
				tasks = TaskParser{}
			}
			if applicationPath == "" {
				applicationPath = findLog(session, "application")
			}
			if outputPath == "" {
				outputPath = findLog(session, "output")
				outputOffset = fileSize(outputPath)
			}
			if text, next := readNew(applicationPath, applicationOffset); next >= 0 {
				applicationOffset = next
				for _, event := range identity.Parse(text) {
					event.SeenAt = fileModTime(applicationPath)
					d.emit(event)
				}
			}
			if text, next := readNew(outputPath, outputOffset); next >= 0 {
				outputOffset = next
				for _, event := range tasks.Feed(text) {
					d.emit(event)
				}
			}
		}
	}
}

func (d *Detector) emit(event Event) {
	if d.callback != nil {
		d.callback(event)
	}
}

var (
	modePattern    = regexp.MustCompile(`(?i)\|application\|Session mode:\s*([A-Za-z]+)`)
	profilePattern = regexp.MustCompile(`(?i)(?:Select(?:ed)?Profile|PrepareSelectedProfileLocally|CompleteSelectedProfile) ProfileId:([A-Za-z0-9]+) AccountId:([0-9]+)`)
)

type IdentityParser struct {
	mode      string
	profileID string
	accountID string
	lastKey   string
}

func (p *IdentityParser) Parse(text string) []Event {
	var events []Event
	for _, line := range strings.Split(text, "\n") {
		if match := modePattern.FindStringSubmatch(line); len(match) == 2 {
			p.mode = normalizeMode(match[1])
		}
		if match := profilePattern.FindStringSubmatch(line); len(match) == 3 {
			p.profileID, p.accountID = match[1], match[2]
		}
		if p.mode == "" || p.profileID == "" || p.accountID == "" {
			continue
		}
		key := p.mode + "\x00" + p.profileID + "\x00" + p.accountID
		if key != p.lastKey {
			p.lastKey = key
			events = append(events, Event{Kind: ProfileDetected, Mode: p.mode, ProfileID: p.profileID, AccountID: p.accountID})
		}
	}
	return events
}

func normalizeMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pve":
		return "pve"
	case "regular", "pvp":
		return "pvp"
	case "pvpseason", "seasonal", "szn":
		return "seasonal"
	default:
		return ""
	}
}

const notificationMarker = "Got notification | ChatMessageReceived"

type TaskParser struct {
	buffer  string
	pending bool
	seen    map[string]bool
	order   []string
}

func (p *TaskParser) Feed(text string) []Event {
	p.buffer += text
	if len(p.buffer) > 2<<20 {
		p.buffer = p.buffer[len(p.buffer)-(1<<20):]
	}
	var events []Event
	for {
		if !p.pending {
			marker := strings.Index(p.buffer, notificationMarker)
			if marker < 0 {
				if len(p.buffer) > len(notificationMarker) {
					p.buffer = p.buffer[len(p.buffer)-len(notificationMarker):]
				}
				break
			}
			p.buffer = p.buffer[marker+len(notificationMarker):]
			p.pending = true
		}
		start := strings.IndexByte(p.buffer, '{')
		if start < 0 {
			break
		}
		decoder := json.NewDecoder(strings.NewReader(p.buffer[start:]))
		var payload struct {
			EventID string `json:"eventId"`
			Message struct {
				Type       int    `json:"type"`
				TemplateID string `json:"templateId"`
			} `json:"message"`
		}
		if err := decoder.Decode(&payload); err != nil {
			break
		}
		p.buffer = p.buffer[start+int(decoder.InputOffset()):]
		p.pending = false
		state := taskState(payload.Message.Type)
		taskID := strings.Fields(payload.Message.TemplateID)
		if state == "" || len(taskID) == 0 || !validTaskID(taskID[0]) || p.isDuplicate(payload.EventID) {
			continue
		}
		events = append(events, Event{Kind: TaskChanged, TaskID: taskID[0], TaskState: state})
	}
	return events
}

func (p *TaskParser) isDuplicate(eventID string) bool {
	if eventID == "" {
		return false
	}
	if p.seen == nil {
		p.seen = make(map[string]bool)
	}
	if p.seen[eventID] {
		return true
	}
	p.seen[eventID] = true
	p.order = append(p.order, eventID)
	if len(p.order) > 256 {
		delete(p.seen, p.order[0])
		p.order = p.order[1:]
	}
	return false
}

func taskState(messageType int) string {
	switch messageType {
	case 10:
		return "uncompleted"
	case 11:
		return "failed"
	case 12:
		return "completed"
	default:
		return ""
	}
}

func validTaskID(value string) bool {
	if len(value) != 24 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func latestSession(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	type candidate struct {
		path string
		mod  time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			candidates = append(candidates, candidate{path: filepath.Join(root, entry.Name()), mod: info.ModTime()})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].mod.After(candidates[j].mod) })
	for _, candidate := range candidates {
		if findLog(candidate.path, "application") != "" {
			return candidate.path
		}
	}
	return ""
}

func findLog(dir, kind string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var found string
	var newest time.Time
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if entry.IsDir() || !strings.HasSuffix(name, ".log") || !strings.Contains(name, kind) {
			continue
		}
		info, err := entry.Info()
		if err == nil && (found == "" || info.ModTime().After(newest)) {
			found, newest = filepath.Join(dir, entry.Name()), info.ModTime()
		}
	}
	return found
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now().UTC()
	}
	return info.ModTime().UTC()
}

func readNew(path string, offset int64) (string, int64) {
	if path == "" {
		return "", -1
	}
	file, err := os.Open(path)
	if err != nil {
		return "", -1
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", -1
	}
	if info.Size() < offset {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", -1
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", -1
	}
	return string(data), offset + int64(len(data))
}
