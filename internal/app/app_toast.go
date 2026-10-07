package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/sound"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Notices over the pages (toasts): the pages are native views above the
// shell, so the shell cannot draw over them; a small frameless window of its
// own (toast.html), owned by the main window, shows the notices stacked at the
// bottom middle of the main window, and takes no focus (a game in front keeps
// it). The Go side sends them (TarkovTracker, the alerts, what a screenshot
// was read as, the squad) and so does the shell (updates, its errors). Each
// goes to the history too (the shell's notifications page), kept on this PC.

// Toast is one notice. Message is a key of the shell's words (words.js) and
// Params its values ({name}); Text is shown as is instead (an error's
// message). Key, when set, names a notice that a newer one with the same Key
// replaces in place (an update's progress, a running job). A persistent one
// stays until closed or replaced; the others go after a few seconds.
type Toast struct {
	ID         string            `json:"id"`
	Key        string            `json:"key,omitempty"`
	Category   string            `json:"category"`
	Level      string            `json:"level"`
	Message    string            `json:"message,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
	Text       string            `json:"text,omitempty"`
	Persistent bool              `json:"persistent,omitempty"`
	// Progress is a bar's fill (0–100), -1 or absent for none.
	Progress int           `json:"progress,omitempty"`
	Actions  []ToastAction `json:"actions,omitempty"`
	At       time.Time     `json:"at"`
}

// ToastAction is a button of a notice: Label is a key of the shell's words.
// Pressing it tells the shell ("toast:action"), which does it.
type ToastAction struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// The categories a notice is of. The first ones can be turned off in the
// settings (config ToastsOff); updates, errors and running jobs always show.
const (
	ToastTracker     = "tracker"
	ToastSound       = "sound"
	ToastRecognition = "recognition"
	ToastSquad       = "squad"
	ToastUpdate      = "update"
	ToastError       = "error"
	ToastWorking     = "working"
)

// toastCategories are the categories the settings turn on and off.
var toastCategories = []string{ToastTracker, ToastSound, ToastRecognition, ToastSquad}

const (
	toastShown      = 5 * time.Second
	toastShownLong  = 9 * time.Second
	toastMaxShown   = 5
	toastHistoryMax = 200
	// toastWidth is the toast window's width in the shell's pixels.
	toastWidth = 420
)

type toastCenter struct {
	mu      sync.Mutex
	window  *application.WebviewWindow
	ready   bool // shown once (toastPrime), so it shows with no focus from now on
	shown   []Toast
	history []Toast
	loaded  bool
	height  int // the toast page's height, in the shell's pixels
	saving  bool
}

// toast shows a notice and keeps it in the history; a category turned off in
// the settings is dropped.
func (a *App) toast(t Toast) string {
	a.mu.RLock()
	off := a.settings.ToastsOff
	a.mu.RUnlock()
	if slices.Contains(toastCategories, t.Category) && slices.Contains(off, t.Category) {
		return ""
	}
	if t.Level == "" {
		t.Level = "info"
	}
	if t.ID == "" {
		t.ID = toastID()
	}
	t.At = time.Now()
	c := &a.toasts
	c.mu.Lock()
	c.loadHistoryLocked()
	replaced := false
	if t.Key != "" {
		for i, shown := range c.shown {
			if shown.Key == t.Key {
				t.ID = shown.ID
				c.shown[i] = t
				replaced = true
				break
			}
		}
	}
	if !replaced {
		c.shown = append(c.shown, t)
		// The oldest that would go anyway make room first.
		for len(c.shown) > toastMaxShown {
			drop := 0
			for i, shown := range c.shown {
				if !shown.Persistent {
					drop = i
					break
				}
			}
			c.shown = slices.Delete(c.shown, drop, drop+1)
		}
	}
	// A notice replacing itself (an update's progress) is one line of the
	// history, which follows it.
	recorded := false
	if t.Key != "" {
		for i := len(c.history) - 1; i >= 0 && i >= len(c.history)-20; i-- {
			if c.history[i].Key == t.Key && c.history[i].ID == t.ID {
				c.history[i] = t
				recorded = true
				break
			}
		}
	}
	if !recorded && t.Category != ToastWorking {
		c.history = append(c.history, t)
		if over := len(c.history) - toastHistoryMax; over > 0 {
			c.history = slices.Delete(c.history, 0, over)
		}
	}
	shown := slices.Clone(c.shown)
	c.mu.Unlock()
	if !t.Persistent {
		ttl := toastShown
		if t.Level == "warn" || t.Level == "error" {
			ttl = toastShownLong
		}
		id, at := t.ID, t.At
		time.AfterFunc(ttl, func() { a.dropToast(func(s Toast) bool { return s.ID == id && s.At.Equal(at) }) })
	}
	a.emitEvent("toast:list", shown)
	a.emitEvent("toast:history", t)
	a.saveToastHistory()
	a.placeToasts()
	return t.ID
}

// dropToast takes the notices match picks off the screen (they stay in the
// history).
func (a *App) dropToast(match func(Toast) bool) {
	c := &a.toasts
	c.mu.Lock()
	before := len(c.shown)
	c.shown = slices.DeleteFunc(c.shown, match)
	changed := len(c.shown) != before
	shown := slices.Clone(c.shown)
	c.mu.Unlock()
	if changed {
		a.emitEvent("toast:list", shown)
		a.placeToasts()
	}
}

// endToast takes the notice with key off the screen (a job done).
func (a *App) endToast(key string) {
	if key != "" {
		a.dropToast(func(t Toast) bool { return t.Key == key })
	}
}

// BrowserToast shows a notice for the shell (an update, its errors) and
// returns its ID.
func (a *App) BrowserToast(t Toast) (string, error) {
	if len(t.Key) > 80 || len(t.Message) > 80 || len(t.Text) > 600 || len(t.Params) > 8 || len(t.Actions) > 4 {
		return "", errors.New("invalid notice")
	}
	for k, v := range t.Params {
		if len(k) > 40 || len(v) > 300 {
			return "", errors.New("invalid notice")
		}
	}
	return a.toast(t), nil
}

// BrowserToastEnd takes the notice with key off the screen.
func (a *App) BrowserToastEnd(key string) { a.endToast(key) }

// BrowserToastClose closes a notice (its close button).
func (a *App) BrowserToastClose(id string) { a.dropToast(func(t Toast) bool { return t.ID == id }) }

// BrowserToastAction passes a notice's button to the shell, which does it.
func (a *App) BrowserToastAction(id, action string) {
	c := &a.toasts
	c.mu.Lock()
	var key string
	found := false
	for _, t := range c.shown {
		if t.ID == id {
			key, found = t.Key, slices.ContainsFunc(t.Actions, func(x ToastAction) bool { return x.ID == action })
			break
		}
	}
	c.mu.Unlock()
	if found {
		a.emitEvent("toast:action", map[string]string{"id": id, "key": key, "action": action})
	}
}

// BrowserToastList is the notices on the screen now (the toast page asks when
// it loads).
func (a *App) BrowserToastList() []Toast {
	c := &a.toasts
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.shown)
}

// BrowserToastReady is the toast page's height once it has drawn the notices
// (the shell's pixels); the window takes it, at the bottom of the main window.
func (a *App) BrowserToastReady(height int) {
	c := &a.toasts
	c.mu.Lock()
	c.height = min(max(height, 0), 900)
	c.mu.Unlock()
	a.placeToasts()
}

// BrowserToastHistory is the notices shown, oldest first.
func (a *App) BrowserToastHistory() []Toast {
	c := &a.toasts
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadHistoryLocked()
	return slices.Clone(c.history)
}

// BrowserToastHistoryClear empties the history.
func (a *App) BrowserToastHistoryClear() {
	c := &a.toasts
	c.mu.Lock()
	c.loadHistoryLocked()
	c.history = nil
	c.mu.Unlock()
	a.saveToastHistory()
	a.emitEvent("toast:history", nil)
}

// setupToasts makes the toast window once the main window is ready, and keeps
// it at the main window's bottom as that moves or changes size; a minimised
// main window hides it.
func (a *App) setupToasts() {
	if a.desktop == nil || a.window == nil {
		return
	}
	c := &a.toasts
	c.mu.Lock()
	if c.window != nil {
		c.mu.Unlock()
		return
	}
	window := a.desktop.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "toast", Title: "MAYAK", Width: toastWidth, Height: 80,
		URL: "/toast.html", Frameless: true, Hidden: true, ZoomControlEnabled: false, DisableResize: true,
		Windows: application.WindowsWindow{HiddenOnTaskbar: true, ExStyle: toastExStyle},
	})
	c.window = window
	c.mu.Unlock()
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) { ownPopup(window, a.window) })
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { e.Cancel() })
	for _, event := range []events.WindowEventType{events.Common.WindowDidMove, events.Common.WindowDidResize, events.Common.WindowMinimise, events.Common.WindowRestore, events.Common.WindowUnMinimise, events.Common.WindowShow, events.Common.WindowHide} {
		a.window.OnWindowEvent(event, func(*application.WindowEvent) { go a.placeToasts() })
	}
	go a.primeToasts()
}

// primeToasts shows the toast window once while the main window shows, which
// its page needs to load; from then on it shows with no focus (toastPlace).
func (a *App) primeToasts() {
	c := &a.toasts
	for i := 0; i < 600; i++ {
		if !a.window.IsMinimised() && a.window.IsVisible() {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	c.mu.Lock()
	window := c.window
	c.mu.Unlock()
	if window == nil {
		return
	}
	ownPopup(window, a.window)
	window.SetSize(toastWidth, 1)
	window.Show()
	ownPopup(window, a.window)
	toastHide(window)
	c.mu.Lock()
	c.ready = true
	c.mu.Unlock()
	a.placeToasts()
}

// placeToasts puts the toast window at the bottom middle of the main window,
// as high as its notices, or hides it when there are none (or the main
// window is minimised).
func (a *App) placeToasts() {
	c := &a.toasts
	c.mu.Lock()
	window, ready, height, count := c.window, c.ready, c.height, len(c.shown)
	c.mu.Unlock()
	if window == nil || !ready {
		return
	}
	if count == 0 || height <= 0 || a.window.IsMinimised() || !a.window.IsVisible() {
		toastHide(window)
		return
	}
	toastPlace(window, a.window, toastWidth, height)
}

// toastAlert shows an alert (the sounds and voices) in words too, whether it
// is heard or not. What a screenshot was read as says so itself, with the
// name read (app_recognition*.go).
func (a *App) toastAlert(kind sound.Kind) {
	level := "info"
	switch kind {
	case sound.Quest, sound.TaskNotMatched, sound.Item, sound.ItemNotMatched, sound.Error:
		return
	case sound.RemoteError, sound.TaskFailed:
		level = "warn"
	}
	a.toast(Toast{Category: ToastSound, Level: level, Message: "toastAlert_" + string(kind)})
}

// taskName is a task's name for a notice, from the task list of mode (its
// ID when the list is not at hand or lacks it).
func (a *App) taskName(mode, id string) string {
	ctx, cancel := context.WithTimeout(a.parentContext(), 5*time.Second)
	defer cancel()
	quests, err := a.questClient.QuestsForMode(ctx, catalogMode(mode))
	if err == nil {
		for _, q := range quests {
			if q.ID == id {
				return q.Name
			}
		}
	}
	return id
}

func toastID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// The history is this PC's (notifications.json), not synced: what a PC was
// told.
func toastHistoryPath() (string, error) { return appdir.Path("notifications.json") }

func (c *toastCenter) loadHistoryLocked() {
	if c.loaded {
		return
	}
	c.loaded = true
	path, err := toastHistoryPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var history []Toast
	if json.Unmarshal(data, &history) == nil {
		if over := len(history) - toastHistoryMax; over > 0 {
			history = history[over:]
		}
		c.history = history
	}
}

// saveToastHistory writes the history a moment later, once for a burst.
func (a *App) saveToastHistory() {
	c := &a.toasts
	c.mu.Lock()
	if c.saving {
		c.mu.Unlock()
		return
	}
	c.saving = true
	c.mu.Unlock()
	time.AfterFunc(2*time.Second, func() {
		c.mu.Lock()
		c.saving = false
		history := slices.Clone(c.history)
		c.mu.Unlock()
		path, err := toastHistoryPath()
		if err != nil {
			return
		}
		data, err := json.Marshal(history)
		if err != nil {
			return
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, data, 0o600)
	})
}
