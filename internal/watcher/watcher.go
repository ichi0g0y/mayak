package watcher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	dir      string
	fs       *fsnotify.Watcher
	callback func(string)
	done     chan struct{}
	once     sync.Once
	mu       sync.Mutex
	seen     map[string]time.Time
	scanning atomic.Bool
}

func New(dir string, callback func(string)) (*Watcher, error) {
	if callback == nil {
		return nil, errors.New("callback is required")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("screenshot path is not a directory")
	}
	f, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{dir: dir, fs: f, callback: callback, done: make(chan struct{}), seen: make(map[string]time.Time)}, nil
}

func (w *Watcher) Start() error {
	if err := w.fs.Add(w.dir); err != nil {
		return err
	}
	go w.loop()
	go w.dispatchLatestRecent(15 * time.Minute)
	return nil
}
func (w *Watcher) Close() error { w.once.Do(func() { close(w.done) }); return w.fs.Close() }
func (w *Watcher) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	events := w.fs.Events
	errors := w.fs.Errors
	for {
		select {
		case <-w.done:
			return
		case e, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if e.Op&(fsnotify.Create|fsnotify.Rename|fsnotify.Write) != 0 && isImage(e.Name) && w.mark(e.Name) {
				go w.waitAndDispatch(e.Name)
			}
		case _, ok := <-errors:
			if !ok {
				errors = nil
			}
		case <-ticker.C:
			// Windows can occasionally coalesce or drop directory notifications.
			// Polling only the newest filename keeps monitoring reliable without
			// re-running image analysis for files already seen.
			go w.dispatchLatestRecent(2 * time.Minute)
		}
	}
}
func isImage(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}
func (w *Watcher) mark(p string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.seen[p]; ok {
		return false
	}
	w.seen[p] = time.Now()
	return true
}
func (w *Watcher) forget(p string) {
	w.mu.Lock()
	delete(w.seen, p)
	w.mu.Unlock()
}
func (w *Watcher) waitAndDispatch(p string) {
	var prior int64 = -1
	for i := 0; i < 20; i++ {
		info, err := os.Stat(p)
		if err == nil && info.Size() > 0 && info.Size() == prior {
			f, e := os.Open(p)
			if e == nil {
				_ = f.Close()
				select {
				case <-w.done:
					return
				default:
					w.callback(p)
				}
				return
			}
		}
		if err == nil {
			prior = info.Size()
		}
		select {
		case <-w.done:
			return
		case <-time.After(150 * time.Millisecond):
		}
	}
	// Permit the polling fallback to retry only when the file never became
	// readable. Successfully dispatched screenshots remain permanently seen.
	w.forget(p)
}

// Replays one very recent screenshot so an app restart during OCR/build work
// does not silently lose the image. mark keeps it from being handled twice if
// fsnotify reports the same file at startup.
func (w *Watcher) dispatchLatestRecent(maxAge time.Duration) {
	if !w.scanning.CompareAndSwap(false, true) {
		return
	}
	defer w.scanning.Store(false)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	var latestPath string
	var latestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() || !isImage(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr == nil && info.ModTime().After(latestTime) {
			latestPath = filepath.Join(w.dir, entry.Name())
			latestTime = info.ModTime()
		}
	}
	if latestPath != "" && time.Since(latestTime) <= maxAge && w.mark(latestPath) {
		w.waitAndDispatch(latestPath)
	}
}
