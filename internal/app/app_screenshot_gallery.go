package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/local/mayak/internal/screenshotstore"
	"golang.org/x/image/draw"
)

// The browser shell shows the game's screenshots: the latest in the sidebar
// and all of them on a page of their own. Images go to the shell as JPEG
// data URLs; thumbnails are kept in memory, so the page scrolls smoothly.

// ScreenshotEntry is one screenshot in the screenshot folder.
type ScreenshotEntry struct {
	Name string `json:"name"`
	// Time is when it was saved (RFC 3339).
	Time string `json:"time"`
	// Meta is what MAYAK recognized it as, if it analyzed it.
	Meta *screenshotstore.Record `json:"meta,omitempty"`
}

// thumbnailWidth is the width thumbnails are made at (twice the widest place
// they show, for sharp results on high-DPI screens).
const thumbnailWidth = 480

// maxThumbnails bounds the thumbnail cache.
const maxThumbnails = 400

type screenshotThumbs struct {
	mu    sync.Mutex
	cache map[string]string
	order []string
}

var thumbs = screenshotThumbs{cache: map[string]string{}}

// backfilled is the screenshot folder whose debug data went into the index.
var backfilled atomic.Pointer[string]

// BrowserScreenshots lists the screenshot folder's screenshots, newest first,
// at most limit of them.
func (a *App) BrowserScreenshots(limit int) ([]ScreenshotEntry, error) {
	dir := a.screenshotDir()
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// Analyses from before the index: once per folder, from the debug data.
	if last := backfilled.Load(); a.screenshotIndex != nil && (last == nil || *last != dir) {
		backfilled.Store(&dir)
		if err := a.screenshotIndex.Fill(screenshotstore.DebugRecords(dir)); err != nil {
			a.addLog("Warn", "Screenshot", "Could not record earlier screenshot analyses: "+err.Error())
		}
	}
	type file struct {
		name string
		at   time.Time
	}
	var files []file
	for _, entry := range entries {
		if entry.IsDir() || !screenshotName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, file{entry.Name(), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.After(files[j].at) })
	if limit <= 0 || limit > 5000 {
		limit = 5000
	}
	out := make([]ScreenshotEntry, 0, min(limit, len(files)))
	for _, f := range files[:min(limit, len(files))] {
		entry := ScreenshotEntry{Name: f.name, Time: f.at.UTC().Format(time.RFC3339)}
		if a.screenshotIndex != nil {
			if r, ok := a.screenshotIndex.Get(f.name); ok {
				entry.Meta = &r
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// BrowserScreenshotImage returns a screenshot as a JPEG data URL: a thumbnail
// when thumbnail is set, else at full size.
func (a *App) BrowserScreenshotImage(name string, thumbnail bool) (string, error) {
	dir := a.screenshotDir()
	// Only a file directly in the screenshot folder: no paths.
	if dir == "" || name != filepath.Base(name) || !screenshotName(name) {
		return "", errors.New("invalid screenshot")
	}
	path := filepath.Join(dir, name)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s|%d|%d", name, info.ModTime().UnixNano(), info.Size())
	if thumbnail {
		thumbs.mu.Lock()
		cached, ok := thumbs.cache[key]
		thumbs.mu.Unlock()
		if ok {
			return cached, nil
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(file)
	file.Close()
	if err != nil {
		return "", err
	}
	quality := 90
	if thumbnail {
		b := img.Bounds()
		height := max(1, b.Dy()*thumbnailWidth/max(1, b.Dx()))
		small := image.NewRGBA(image.Rect(0, 0, thumbnailWidth, height))
		draw.ApproxBiLinear.Scale(small, small.Bounds(), img, b, draw.Src, nil)
		img, quality = small, 80
	}
	var buffer bytes.Buffer
	if err = jpeg.Encode(&buffer, img, &jpeg.Options{Quality: quality}); err != nil {
		return "", err
	}
	data := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())
	if thumbnail {
		thumbs.mu.Lock()
		if _, ok := thumbs.cache[key]; !ok {
			thumbs.cache[key] = data
			thumbs.order = append(thumbs.order, key)
			if len(thumbs.order) > maxThumbnails {
				delete(thumbs.cache, thumbs.order[0])
				thumbs.order = thumbs.order[1:]
			}
		}
		thumbs.mu.Unlock()
	}
	return data, nil
}

func (a *App) screenshotDir() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings.ScreenshotDirectory
}

// screenshotName reports whether name is a screenshot file (the formats the
// screenshot watcher takes).
func screenshotName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg":
		return !strings.HasPrefix(name, ".")
	}
	return false
}
