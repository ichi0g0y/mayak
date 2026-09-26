package hideoutlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const HistoryLimit = 500
const HistoryAge = 90 * 24 * time.Hour

// saveDelay is how long the store waits after a change before writing the
// file, so a burst of events (the logs replayed at a start: hundreds of
// them) is written once, not once per event.
const saveDelay = 500 * time.Millisecond

// Store keeps the hideout events, newest first, in memory and in one JSON
// file. Events are told apart by their fingerprint (kept in a set, so an
// addition costs no scan), and writes are coalesced: Add marks the store
// dirty and a timer writes it saveDelay later; Flush writes at once.
type Store struct {
	mu     sync.Mutex
	path   string
	events []Event
	seen   map[string]struct{}
	dirty  bool
	timer  *time.Timer
	// OnSaved receives every write's result from Flush (nil when it worked,
	// or when nothing was pending), so the app can show a history file that
	// fails to save and clear that once it works again. nil ignores it.
	OnSaved func(error)
}

func NewStore(path string) *Store {
	s := &Store{path: path, seen: map[string]struct{}{}}
	if data, err := os.ReadFile(path); err == nil && len(data) <= 2<<20 {
		_ = json.Unmarshal(data, &s.events)
	}
	s.prune(time.Now())
	return s
}

// prune drops events older than HistoryAge and beyond HistoryLimit, keeps
// the rest newest first, and rebuilds the fingerprint set when anything
// was dropped. s.mu is held.
func (s *Store) prune(now time.Time) {
	var kept []Event
	for _, e := range s.events {
		if !e.OccurredAt.Before(now.Add(-HistoryAge)) {
			kept = append(kept, e)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].OccurredAt.After(kept[j].OccurredAt) })
	if len(kept) > HistoryLimit {
		kept = kept[:HistoryLimit]
	}
	dropped := len(kept) != len(s.events)
	s.events = kept
	if dropped || len(s.seen) != len(kept) {
		s.seen = make(map[string]struct{}, len(kept))
		for _, e := range kept {
			s.seen[e.Fingerprint()] = struct{}{}
		}
	}
}

func (s *Store) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	visible := []Event{}
	for _, e := range s.events {
		if !e.Hidden {
			visible = append(visible, e)
		}
	}
	return visible
}

// Clear removes rows from the UI but retains bounded fingerprints against replay.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.events {
		s.events[i].Hidden = true
	}
	s.dirty = true
	return s.flushLocked()
}

// Add keeps e unless it is too old or already known, and reports whether
// it was kept. The file is written later (see saveDelay); Flush forces it.
func (s *Store) Add(e Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if e.OccurredAt.IsZero() || e.OccurredAt.Before(now.Add(-HistoryAge)) {
		return false
	}
	fingerprint := e.Fingerprint()
	if _, known := s.seen[fingerprint]; known {
		return false
	}
	// Kept newest first: the event goes where its time falls (replayed logs
	// come in order, so usually the front), and the list is pruned only when
	// it is over the limit or its oldest event has aged out.
	at := sort.Search(len(s.events), func(i int) bool { return !s.events[i].OccurredAt.After(e.OccurredAt) })
	s.events = append(s.events, Event{})
	copy(s.events[at+1:], s.events[at:])
	s.events[at] = e
	s.seen[fingerprint] = struct{}{}
	if len(s.events) > HistoryLimit || s.events[len(s.events)-1].OccurredAt.Before(now.Add(-HistoryAge)) {
		s.prune(now)
	}
	s.scheduleSave()
	return true
}

// scheduleSave arranges one write saveDelay from now. s.mu is held.
func (s *Store) scheduleSave() {
	if s.path == "" {
		return
	}
	s.dirty = true
	if s.timer != nil {
		return
	}
	s.timer = time.AfterFunc(saveDelay, func() { _ = s.Flush() })
}

// Flush writes the store now if it changed since the last write, and
// tells OnSaved how it went.
func (s *Store) Flush() error {
	s.mu.Lock()
	err := s.flushLocked()
	s.mu.Unlock()
	if s.OnSaved != nil {
		s.OnSaved(err)
	}
	return err
}

func (s *Store) flushLocked() error {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if !s.dirty || s.path == "" {
		s.dirty = false
		return nil
	}
	s.dirty = false
	if err := s.save(); err != nil {
		s.dirty = true
		return err
	}
	return nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.events, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".hideout-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), s.path)
}
