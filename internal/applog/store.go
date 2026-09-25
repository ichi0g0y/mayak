package applog

import (
	"bufio"
	"encoding/json"
	"github.com/local/mayak/internal/appdir"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/model"
)

const defaultLimit = 500

type Store struct {
	mu      sync.RWMutex
	entries []model.LogEntry
	nextID  uint64
	limit   int
	path    string
	// writeMu keeps the file's writes in order without holding mu (readers
	// do not wait for the disk); written counts the file's lines.
	writeMu sync.Mutex
	written int
}

func New(limit int) *Store {
	if limit <= 0 {
		limit = defaultLimit
	}
	store := &Store{limit: limit}
	if configDir, err := os.UserConfigDir(); err == nil {
		store.path = filepath.Join(configDir, appdir.Name, "mayak.log")
		store.load()
	}
	return store
}

func (s *Store) Add(level, category, message string) model.LogEntry {
	entry := model.LogEntry{
		Timestamp: time.Now(),
		Level:     normalizeLevel(level),
		Category:  strings.TrimSpace(category),
		Message:   strings.TrimSpace(message),
	}
	s.mu.Lock()
	s.nextID++
	entry.ID = s.nextID
	s.entries = append(s.entries, entry)
	if overflow := len(s.entries) - s.limit; overflow > 0 {
		s.entries = append([]model.LogEntry(nil), s.entries[overflow:]...)
	}
	path := s.path
	if path == "" {
		s.mu.Unlock()
		return entry
	}
	// Taken before mu is released, so the file keeps the entries' order.
	s.writeMu.Lock()
	// The file only needs what is kept in memory: once it holds twice that,
	// it is rewritten with the kept entries, so it does not grow forever.
	var kept []model.LogEntry
	if s.written+1 > 2*s.limit {
		kept = append([]model.LogEntry(nil), s.entries...)
	}
	s.mu.Unlock()
	if kept != nil {
		if rewrite(path, kept) == nil {
			s.written = len(kept)
		}
	} else if appendEntry(path, entry) == nil {
		s.written++
	}
	s.writeMu.Unlock()
	return entry
}

func (s *Store) Entries() []model.LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]model.LogEntry(nil), s.entries...)
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.entries = nil
	s.written = 0
	path := s.path
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0o600)
}

func (s *Store) load() {
	file, err := os.Open(s.path)
	if err != nil {
		return
	}
	defer file.Close()
	entries := make([]model.LogEntry, 0, s.limit)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		s.written++
		var entry model.LogEntry
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.ID > s.nextID {
			s.nextID = entry.ID
		}
		entries = append(entries, entry)
		if len(entries) > s.limit {
			entries = entries[len(entries)-s.limit:]
		}
	}
	s.entries = entries
}

// rewrite replaces the file with entries.
func rewrite(path string, entries []model.LogEntry) error {
	var data []byte
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		data = append(append(data, line...), '\n')
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func appendEntry(path string, entry model.LogEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(entry)
}

func normalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		return "Error"
	case "warn", "warning":
		return "Warn"
	case "debug":
		return "Debug"
	default:
		return "Info"
	}
}
