package app

import (
	"image"
	"os"
	"sync"
	"time"

	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/profiledetect"
	"github.com/local/mayak/internal/taskdetect"
)

// The latest screenshots decoded, so a screenshot is decoded once for all
// that read it: the Overall, item and task detectors, the task list and the
// debug data each decoded it again before (60 ms a time at 2560x1440). A
// capture of MAYAK's own (app_capture.go) is put here as it is written, so
// it is not decoded at all.

// shotImageKeep is how many decoded screenshots are kept: the one being read
// and the one before, which the task list may still be reading.
const shotImageKeep = 2

type shotImages struct {
	mu sync.Mutex
	// entries are the newest last.
	entries []shotImage
}

// shotImage is a decoded screenshot and the file it came from as it was
// then, so a file written again is decoded again.
type shotImage struct {
	path string
	size int64
	mod  time.Time
	img  image.Image
}

// screenshotImage is the screenshot at path, decoded once while it stays
// the same.
func (a *App) screenshotImage(path string) (image.Image, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	c := &a.shotImages
	c.mu.Lock()
	for _, e := range c.entries {
		if e.path == path && e.size == info.Size() && e.mod.Equal(info.ModTime()) {
			c.mu.Unlock()
			return e.img, nil
		}
	}
	c.mu.Unlock()
	img, err := decodeImage(path)
	if err != nil {
		return nil, err
	}
	a.rememberScreenshotImage(path, info, img)
	return img, nil
}

// rememberScreenshotImage keeps img as the screenshot at path, as the file
// info describes it.
func (a *App) rememberScreenshotImage(path string, info os.FileInfo, img image.Image) {
	c := &a.shotImages
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.entries[:0]
	for _, e := range c.entries {
		if e.path != path {
			kept = append(kept, e)
		}
	}
	kept = append(kept, shotImage{path: path, size: info.Size(), mod: info.ModTime(), img: img})
	if len(kept) > shotImageKeep {
		kept = append([]shotImage(nil), kept[len(kept)-shotImageKeep:]...)
	}
	c.entries = kept
}

// detectProfile, detectItem and detectTasks run the detectors on the
// screenshot at path, decoded once for all of them.
func (a *App) detectProfile(path string) (profiledetect.Result, error) {
	img, err := a.screenshotImage(path)
	if err != nil {
		return profiledetect.Result{}, err
	}
	return profiledetect.Analyze(img)
}

func (a *App) detectItem(path string) (itemdetect.Result, error) {
	img, err := a.screenshotImage(path)
	if err != nil {
		return itemdetect.Result{}, err
	}
	return itemdetect.Analyze(img)
}

func (a *App) detectTasks(path string) (taskdetect.Result, error) {
	img, err := a.screenshotImage(path)
	if err != nil {
		return taskdetect.Result{}, err
	}
	return taskdetect.Analyze(img, taskdetect.Preset2560)
}
