package hideoutlog

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/trackerlog"
)

type cursor struct {
	offset    int64
	parser    Parser
	liveAfter time.Time
}
type identityPoint struct {
	at       time.Time
	identity Identity
}
type Detector struct {
	root     string
	callback func(context.Context, Event)
	cancel   context.CancelFunc
	done     chan struct{}
	once     sync.Once
}

func New(root string, callback func(Event)) *Detector {
	return NewWithContext(root, func(_ context.Context, e Event) {
		if callback != nil {
			callback(e)
		}
	})
}
func NewWithContext(root string, callback func(context.Context, Event)) *Detector {
	return &Detector{root: root, callback: callback, done: make(chan struct{})}
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
			<-d.done
		}
	})
}

func timeline(dir string) []identityPoint {
	entries, _ := os.ReadDir(dir)
	var result []identityPoint
	var parser trackerlog.IdentityParser
	for _, entry := range entries {
		if !strings.Contains(entry.Name(), "application") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 65536), maxBuffer)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), "|application|session mode:") {
				parser = trackerlog.IdentityParser{}
				result = append(result, identityPoint{at: LogTime(line)})
			}
			for _, e := range parser.Parse(line) {
				result = append(result, identityPoint{LogTime(line), Identity{e.AccountID, e.ProfileID, e.Mode}})
			}
		}
		f.Close()
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].at.Before(result[j].at) })
	return result
}

// Prefer dedicated backend files per session. Output/errors are fallback only.
func sessionFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var primary, fallback []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		name := strings.ToLower(e.Name())
		path := filepath.Join(dir, e.Name())
		if strings.Contains(name, "backend_") {
			primary = append(primary, path)
		} else if strings.Contains(name, "output_") || strings.Contains(name, "errors_") {
			fallback = append(fallback, path)
		}
	}
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

func (d *Detector) loop(ctx context.Context) {
	defer close(d.done)
	cursors := map[string]*cursor{}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		dirs, _ := os.ReadDir(d.root)
		for _, dir := range dirs {
			if ctx.Err() != nil {
				return
			}
			if !dir.IsDir() {
				continue
			}
			path := filepath.Join(d.root, dir.Name())
			files := sessionFiles(path)
			var identities []identityPoint
			loaded := false
			for _, file := range files {
				if ctx.Err() != nil {
					return
				}
				info, err := os.Stat(file)
				if err != nil {
					continue
				}
				c, known := cursors[file]
				historical := !known
				if !known {
					c = &cursor{liveAfter: time.Now()}
					cursors[file] = c
				}
				if info.Size() < c.offset {
					*c = cursor{liveAfter: time.Now()}
					historical = true
				}
				if info.Size() == c.offset {
					continue
				}
				if !loaded {
					identities = timeline(path)
					loaded = true
				}
				f, err := os.Open(file)
				if err != nil {
					continue
				}
				_, _ = f.Seek(c.offset, io.SeekStart)
				// Read only this snapshot. Bytes appended during initialization are live next time.
				reader := io.LimitReader(f, info.Size()-c.offset)
				buffer := make([]byte, 64<<10)
				for ctx.Err() == nil {
					n, readErr := reader.Read(buffer)
					c.offset += int64(n)
					for _, e := range c.parser.Feed(string(buffer[:n])) {
						e.Historical = historical || !e.OccurredAt.After(c.liveAfter)
						e.Source = filepath.Base(file)
						for _, point := range identities {
							if !point.at.IsZero() && !point.at.After(e.OccurredAt) {
								e.Identity = point.identity
							}
						}
						if d.callback != nil {
							d.callback(ctx, e)
						}
					}
					if readErr != nil {
						break
					}
				}
				f.Close()
			}
		}
		// Forget vanished files, bounding cursor memory across deleted EFT sessions.
		for path := range cursors {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				delete(cursors, path)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
