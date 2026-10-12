package app

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/local/mayak/internal/capture"
	"github.com/local/mayak/internal/itemdetect"
	"github.com/local/mayak/internal/profiledetect"
	"github.com/local/mayak/internal/taskdetect"
)

// MAYAK's capture key (experimental, internal/capture): config CaptureKey,
// empty for none. Pressed while EFT is in front, it grabs the game's window
// into the screenshot folder, where the watcher reads it like one of EFT's
// screenshots, without EFT's notice in the game and without a position.
//
// With config CaptureAuto, a click in the game does the same by itself, like
// RatScanner's automatic scan: once the clicks stop for captureSettle, the
// game's window is grabbed and kept only when it shows a screen MAYAK reads
// (captureKind) whose text differs from the last one of its kind.
//
// Only the grab waits for nothing else, so the screen is the one the click
// opened: telling the screen apart (captureSort) and writing the PNG
// (captureWrite) each run on a goroutine of their own. A grab after a click
// waiting to be told apart gives way to a newer one; a shot to keep waits
// for its turn to be written.

// captureTuning keeps the settings of the capture after clicks (config
// CaptureSettleMs, CaptureRepeatSeconds, CapturePollMs) in their ranges, the
// default for one out of them: how long after the last click the screen is
// taken (250 ms; RatScanner waits 200), how long the same screen is not
// taken again (a minute; 0 always), how often the mouse button is looked at
// (30 ms; a click is held 50 ms or more).
func captureTuning(settleMs, repeatSeconds, pollMs int) (int, int, int) {
	if settleMs < 50 || settleMs > 2000 {
		settleMs = 250
	}
	if repeatSeconds < 0 || repeatSeconds > 3600 {
		repeatSeconds = 60
	}
	if pollMs < 10 || pollMs > 100 {
		pollMs = 30
	}
	return settleMs, repeatSeconds, pollMs
}

type captureState struct {
	mu     sync.Mutex
	key    string
	hotkey *capture.Hotkey
	// grabbing keeps one grab at a time: a key held down or clicks while the
	// screen is being copied do nothing.
	grabbing atomic.Bool
	// clicks watches the game's clicks while CaptureAuto is on, with the
	// times it was started with.
	clicks       *capture.ClickWatcher
	clicksTiming [2]int
	// last is the signature of the last screen of each kind captured
	// automatically, and when.
	last map[string]capturedScreen
	// toSort carries the grabs after clicks to captureSort (one waiting,
	// the newest), toWrite the shots to keep to captureWrite.
	toSort  chan capturedShot
	toWrite chan capturedShot
	start   sync.Once
}

type capturedScreen struct {
	signature []byte
	at        time.Time
}

// capturedShot is a grab of the game's window, when, and the folder to
// write it to.
type capturedShot struct {
	img *image.RGBA
	at  time.Time
	dir string
	// auto is a grab after a click: the screen is told apart before it is
	// kept, and nothing is said when it is not; click is how it was made.
	auto  bool
	click capture.Click
}

// startCaptureWork starts the goroutines that tell the screens apart and
// write them, once.
func (a *App) startCaptureWork() {
	c := &a.capture
	c.start.Do(func() {
		c.toSort = make(chan capturedShot, 1)
		c.toWrite = make(chan capturedShot, 1)
		go a.captureSort()
		go a.captureWrite()
	})
}

// handOver puts shot in queue, in place of one still waiting there: of the
// grabs after clicks, the newest is the one to tell apart.
func handOver(queue chan capturedShot, shot capturedShot) {
	for {
		select {
		case queue <- shot:
			return
		default:
			select {
			case <-queue:
			default:
			}
		}
	}
}

// applyCaptureKey registers key (Format's text) in place of the one
// registered, or none for "". A Client of the link does not read
// screenshots, so it registers none.
func (a *App) applyCaptureKey(key string) {
	c := &a.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if a.browserClient.Load() || a.quitting.Load() {
		key = ""
	}
	if key == c.key {
		return
	}
	if c.hotkey != nil {
		c.hotkey.Stop()
		c.hotkey = nil
	}
	c.key = ""
	if key == "" {
		return
	}
	parsed, err := capture.ParseKey(key)
	if err != nil {
		a.addLog("Warn", "Capture", "Unknown capture key "+key+": "+err.Error())
		return
	}
	a.startCaptureWork()
	hotkey, err := capture.StartHotkey(parsed, a.captureShot)
	if err != nil {
		a.addLog("Warn", "Capture", "Could not register the capture key "+key+": "+err.Error())
		a.toast(Toast{Event: "error", Category: ToastRecognition, Level: "error", Message: "toastCaptureKeyTaken", Params: map[string]string{"key": key}})
		return
	}
	c.key, c.hotkey = key, hotkey
	a.addLog("Info", "Capture", "Capture key "+key+" registered")
}

// applyCaptureAuto starts or stops capturing after clicks in the game, and
// starts it again with new times; a Client of the link captures nothing.
func (a *App) applyCaptureAuto(on bool) {
	a.mu.RLock()
	timing := [2]int{a.settings.CaptureSettleMs, a.settings.CapturePollMs}
	a.mu.RUnlock()
	c := &a.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if a.browserClient.Load() || a.quitting.Load() {
		on = false
	}
	if on == (c.clicks != nil) && (!on || timing == c.clicksTiming) {
		return
	}
	if c.clicks != nil {
		c.clicks.Stop()
		c.clicks = nil
	}
	if !on {
		return
	}
	a.startCaptureWork()
	if c.last == nil {
		c.last = map[string]capturedScreen{}
	}
	settle, poll := time.Duration(timing[0])*time.Millisecond, time.Duration(timing[1])*time.Millisecond
	c.clicks, c.clicksTiming = capture.WatchClicks(poll, settle, a.captureAfterClick), timing
	a.addLog("Info", "Capture", "Capturing after clicks in the game ("+settle.String()+" after a click, mouse every "+poll.String()+")")
}

// grab copies the game's window for the screenshot folder; ok is false when
// it was being copied already.
func (a *App) grab() (shot capturedShot, ok bool, err error) {
	c := &a.capture
	if !c.grabbing.CompareAndSwap(false, true) {
		return capturedShot{}, false, nil
	}
	defer c.grabbing.Store(false)
	a.mu.RLock()
	shot.dir = a.settings.ScreenshotDirectory
	a.mu.RUnlock()
	if shot.dir == "" {
		return shot, true, errors.New("no screenshot folder to capture into")
	}
	shot.at = time.Now()
	shot.img, err = capture.Grab()
	return shot, true, err
}

// captureShot grabs EFT's window for the key and has it written; a key that
// took nothing says why.
func (a *App) captureShot() {
	shot, ok, err := a.grab()
	if !ok {
		return
	}
	if err != nil {
		message := "toastCaptureFailed"
		switch {
		case shot.dir == "":
			message = "toastCaptureNoFolder"
		case errors.Is(err, capture.ErrNotInFront):
			message = "toastCaptureNotInFront"
		case errors.Is(err, capture.ErrBlank):
			message = "toastCaptureBlank"
		}
		a.addLog("Warn", "Capture", err.Error())
		t := Toast{Event: "error", Category: ToastRecognition, Level: "error", Message: message}
		if message == "toastCaptureFailed" {
			t.Text = err.Error()
		}
		a.toast(t)
		return
	}
	a.capture.toWrite <- shot
}

// captureAfterClick grabs the game's window after clicks for captureSort;
// a grab that fails is let go without a word, as no key was pressed.
func (a *App) captureAfterClick(click capture.Click) {
	// With every screen turned off, a click takes nothing.
	a.mu.RLock()
	off := len(a.settings.CaptureAutoOff)
	a.mu.RUnlock()
	if off >= len(captureKinds) {
		return
	}
	shot, ok, err := a.grab()
	if !ok || err != nil {
		return
	}
	shot.auto, shot.click = true, click
	handOver(a.capture.toSort, shot)
}

// captureSort keeps the grabs after clicks that show a screen MAYAK reads
// it has not just read, and has them written.
func (a *App) captureSort() {
	c := &a.capture
	for shot := range c.toSort {
		a.mu.RLock()
		off := a.settings.CaptureAutoOff
		repeatFor := time.Duration(a.settings.CaptureRepeatSeconds) * time.Second
		triggers := map[string]string{"tasks": a.settings.CaptureTasksTrigger, "item": a.settings.CaptureItemTrigger, "profile": "click"}
		a.mu.RUnlock()
		kind, crop := captureKind(shot.img, off)
		// The tasks list is clicked through: only the screen's own click
		// takes it.
		if kind == "" || !captureTriggered(triggers[kind], shot.click) {
			continue
		}
		signature := capture.Signature(crop)
		c.mu.Lock()
		last, seen := c.last[kind]
		repeat := seen && capture.Same(last.signature, signature) && time.Since(last.at) < repeatFor
		if !repeat && c.last != nil {
			c.last[kind] = capturedScreen{signature: signature, at: shot.at}
		}
		c.mu.Unlock()
		if !repeat {
			a.addLog("Debug", "Capture", "A click opened "+kind)
			c.toWrite <- shot
		}
	}
}

// captureWrite writes the shots to the screenshot folder, where the watcher
// reads them.
func (a *App) captureWrite() {
	for shot := range a.capture.toWrite {
		path, err := capture.Save(shot.dir, shot.img, shot.at)
		if err != nil {
			a.addLog("Warn", "Capture", err.Error())
			if !shot.auto {
				a.toast(Toast{Event: "error", Category: ToastRecognition, Level: "error", Message: "toastCaptureFailed", Text: err.Error()})
			}
			continue
		}
		// The watcher reads it from the picture in hand, not the file.
		if info, err := os.Stat(path); err == nil {
			a.rememberScreenshotImage(path, info, shot.img)
		}
		a.addLog("Info", "Capture", "Captured "+filepath.Base(path))
	}
}

// captureTriggers are the clicks the tasks screen and an item's window can
// be taken after (config CaptureTasksTrigger, CaptureItemTrigger): a right
// click (the tasks' default: the list is gone through with the left button),
// a click with Shift, or any. A double click is none of them: EFT opens and
// closes a group of tasks on its clicks.
var captureTriggers = []string{"right", "shift", "click"}

// captureTriggered tells whether click is trigger.
func captureTriggered(trigger string, click capture.Click) bool {
	switch trigger {
	case "click":
		return true
	case "shift":
		return !click.Right && click.Shift
	default:
		return click.Right
	}
}

// captureKinds are the screens a click can be captured for, in the order
// the screenshots are read: the Overall screen, an item's window, the tasks.
var captureKinds = []string{"profile", "item", "tasks"}

// captureKindList keeps the known screens of list, each once, in
// captureKinds' order.
func captureKindList(list []string) []string {
	out := []string{}
	for _, kind := range captureKinds {
		if slices.Contains(list, kind) {
			out = append(out, kind)
		}
	}
	return out
}

// captureKind tells which screen MAYAK reads img shows, in the order the
// screenshots are read, with the crop that names what is on it; "" for none
// or one in off. It stops at the first that matches, and after the last not
// in off: a screen read before it (an item's window over the tasks) would
// still be read so, so those are looked for even when off.
func captureKind(img image.Image, off []string) (string, image.Image) {
	last := -1
	for i, kind := range captureKinds {
		if !slices.Contains(off, kind) {
			last = i
		}
	}
	for i, kind := range captureKinds[:last+1] {
		var found bool
		var crop image.Image
		switch kind {
		case "profile":
			r, err := profiledetect.Analyze(img)
			found, crop = err == nil && r.IsProfile && r.Level != nil, r.Level
		case "item":
			r, err := itemdetect.Analyze(img)
			found, crop = err == nil && r.IsItem && r.Crop != nil, r.Crop
		case "tasks":
			r, err := taskdetect.Analyze(img, taskdetect.Preset2560)
			found, crop = err == nil && r.IsTasks && r.Crop != nil, r.Crop
		}
		if found {
			if slices.Contains(off, captureKinds[i]) {
				return "", nil
			}
			return kind, crop
		}
	}
	return "", nil
}

// stopCapture unregisters the key and stops watching clicks as MAYAK quits.
func (a *App) stopCapture() {
	c := &a.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hotkey != nil {
		c.hotkey.Stop()
		c.hotkey = nil
	}
	c.key = ""
	if c.clicks != nil {
		c.clicks.Stop()
		c.clicks = nil
	}
}
