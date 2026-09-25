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

type Store struct {
	mu     sync.Mutex
	path   string
	events []Event
}

func NewStore(path string) *Store {
	s := &Store{path: path}
	if data, err := os.ReadFile(path); err == nil && len(data) <= 2<<20 {
		_ = json.Unmarshal(data, &s.events)
	}
	s.prune(time.Now())
	return s
}
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
	s.events = kept
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
	return s.save()
}
func (s *Store) Add(e Event) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(time.Now())
	if e.OccurredAt.IsZero() || e.OccurredAt.Before(time.Now().Add(-HistoryAge)) {
		return false, nil
	}
	for _, old := range s.events {
		if old.Fingerprint() == e.Fingerprint() {
			return false, nil
		}
	}
	s.events = append(s.events, e)
	s.prune(time.Now())
	return true, s.save()
}
func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
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
