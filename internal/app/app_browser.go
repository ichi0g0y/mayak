package app

import (
	"encoding/json"
	"github.com/local/mayak/internal/appdir"

	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/browserview"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/userdata"
)

var browserStateMu sync.Mutex
var browserViewID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

func (a *App) browserStartsAsClient() bool {
	if goruntime.GOOS != "windows" {
		return true
	}
	raw, err := a.BrowserLoad()
	if err != nil {
		return false
	}
	var saved struct {
		Connection struct {
			Mode string `json:"mode"`
		} `json:"connection"`
	}
	if json.Unmarshal([]byte(raw), &saved) != nil {
		return false
	}
	return saved.Connection.Mode == "webrtc" || saved.Connection.Mode == "off"
}
func (a *App) BrowserSetMode(mode string) error {
	if mode != "local" && mode != "webrtc" && mode != "off" {
		return errors.New("invalid browser mode")
	}
	if mode == "local" && goruntime.GOOS != "windows" {
		return errors.New("Host mode requires Windows")
	}
	client := mode != "local"
	changed := a.browserClient.Swap(client) != client
	if changed && client {
		a.analysisSequence.Add(1)
		a.mu.Lock()
		cancel := a.analysisCancel
		a.analysisCancel = nil
		var closeClients []func() error
		for _, client := range a.remotes {
			closeClients = append(closeClients, client.Close)
		}
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		for _, closeClient := range closeClients {
			_ = closeClient()
		}
		a.StopMonitoring()
	}
	if changed && !client {
		a.startBrowserHostBackground()
	}
	return nil
}

func (a *App) startBrowserHostBackground() {
	if goruntime.GOOS != "windows" || a.browserClient.Load() {
		return
	}
	a.browserBackgroundOnce.Do(func() { go a.watchCatalog(); go a.watchScreenshotMaintenance(); go a.watchGame() })
}

func browserStatePath() (string, error) {
	return appdir.Path("browser.json")
}
func (a *App) BrowserPlatform() string { return goruntime.GOOS }

// The built-in browser's state is one object for the shell (state.js), kept
// in three files: its preferences key by key (browser-preferences.json) and
// its bookmarks record by record (bookmarks.json), which follow the user to
// another PC (internal/userdata), and the rest, this PC's (browser.json: the
// tabs, the panel sizes, the Host/client role). State saved before the split
// is all in browser.json; the next save moves the rest out.
var browserPreferenceKeys = map[string]bool{
	"language": true, "tutorialDone": true, "clock": true, "theme": true, "layout": true,
	"sidebarSide": true, "sidebarCollapsed": true, "bookmarksCollapsed": true,
	"screenshotsCollapsed": true, "snapNotesCollapsed": true, "toolOrder": true,
	"bossesView": true, "bookmarkView": true, "adblock": true, "taskMode": true,
	"questSite": true, "translateWiki": true, "bookmarkRevision": true,
}

func browserFile(name string) (string, error) { return appdir.Path(name) }

func (a *App) BrowserLoad() (string, error) {
	browserStateMu.Lock()
	defer browserStateMu.Unlock()
	return loadBrowserState()
}

func loadBrowserState() (string, error) {
	p, e := browserStatePath()
	if e != nil {
		return "", e
	}
	state := map[string]json.RawMessage{}
	b, e := os.ReadFile(p)
	switch {
	case e == nil:
		if json.Unmarshal(b, &state) != nil {
			state = map[string]json.RawMessage{}
		}
	case !os.IsNotExist(e):
		return "", e
	}
	if prefsPath, err := browserFile("browser-preferences.json"); err == nil {
		prefs, _, _ := userdata.LoadKeyed(prefsPath)
		for key, value := range prefs.Values {
			if browserPreferenceKeys[key] {
				state[key] = value
			}
		}
	}
	if bookmarksPath, err := browserFile("bookmarks.json"); err == nil {
		if records, ok, _ := userdata.LoadRecords(bookmarksPath); ok {
			list := []json.RawMessage{}
			for _, r := range records.Live() {
				list = append(list, r.Value)
			}
			state["bookmarks"], _ = json.Marshal(list)
		}
	}
	if len(state) == 0 {
		return "{}", nil
	}
	out, e := json.Marshal(state)
	return string(out), e
}

// BrowserSave keeps the shell's state: UI preferences, bookmarks and tabs.
// Tokens and temporary WebRTC descriptions are never stored in it.
func (a *App) BrowserSave(raw string) error {
	if len(raw) > 1024*1024 || !json.Valid([]byte(raw)) {
		return errors.New("invalid browser state")
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return errors.New("invalid browser state")
	}
	browserStateMu.Lock()
	defer browserStateMu.Unlock()
	return saveBrowserState(state, time.Now())
}

func saveBrowserState(state map[string]json.RawMessage, now time.Time) error {
	device := map[string]json.RawMessage{}
	prefs := map[string]json.RawMessage{}
	var bookmarks json.RawMessage
	for key, value := range state {
		switch {
		case key == "bookmarks":
			bookmarks = value
		case browserPreferenceKeys[key]:
			prefs[key] = value
		default:
			device[key] = value
		}
	}
	prefsPath, err := browserFile("browser-preferences.json")
	if err != nil {
		return err
	}
	doc, existed, _ := userdata.LoadKeyed(prefsPath)
	if doc.Set(prefs, now) || !existed {
		if err = userdata.SaveKeyed(prefsPath, doc); err != nil {
			return err
		}
	}
	if bookmarks != nil {
		var list []json.RawMessage
		if json.Unmarshal(bookmarks, &list) == nil {
			ids := make([]string, len(list))
			for i, item := range list {
				var b struct {
					ID string `json:"id"`
				}
				_ = json.Unmarshal(item, &b)
				ids[i] = b.ID
			}
			bookmarksPath, err := browserFile("bookmarks.json")
			if err != nil {
				return err
			}
			records, existed, _ := userdata.LoadRecords(bookmarksPath)
			if records.SetList(ids, list, now) || !existed {
				if err = userdata.SaveRecords(bookmarksPath, records); err != nil {
					return err
				}
			}
		}
	}
	p, err := browserStatePath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	out, err := json.Marshal(device)
	if err != nil {
		return err
	}
	return userdata.WriteAtomic(p, out)
}
func (a *App) BrowserView(command string, v browserview.Options) error {
	if !browserViewID.MatchString(v.ID) || v.Left < 0 || v.Left > 4096 || v.Top < 0 || v.Top > 4096 || v.Right < 0 || v.Right > 4096 {
		return errors.New("invalid browser bounds or ID")
	}
	switch command {
	case "show", "preload", "navigate":
		u, e := url.Parse(v.URL)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || strings.EqualFold(u.Hostname(), "wails.localhost") {
			return errors.New("invalid browser URL")
		}
	case "hideAll", "close", "back", "forward", "reload", "focus":
	default:
		return errors.New("invalid browser command")
	}
	if a.browserViews == nil {
		return errors.New("browser is not ready")
	}
	// A slow or failed view command is worth a line: the shell's commands
	// wait behind each other, so one slow one holds every view change after it.
	started := time.Now()
	err := a.browserViews.Command(command, v)
	if elapsed := time.Since(started); err != nil || elapsed > 500*time.Millisecond {
		a.addLog("Debug", "Browser", fmt.Sprintf("View command %s (%s) took %s: %v", command, v.ID, elapsed.Round(time.Millisecond), err))
	}
	return err
}
func (a *App) showBrowserTask(status model.Status, site string) {
	urls := map[string]string{}
	for _, s := range []string{"tarkov-dev", "official-wiki", "japanese-wiki"} {
		urls[s] = questStatusURL(s, status)
	}
	a.emitEvent("browser:task", map[string]interface{}{"id": status.QuestID, "name": status.LastQuest, "site": normalizeQuestSite(site), "urls": urls})
}

// browserShortcutKey is the set of keys the shell takes from a page: Chrome's
// tab shortcuts, as state.js shortcut() reads them (the two must agree, or a
// page loses a key nobody uses). Ctrl+T and Ctrl+Shift+T, Ctrl+Tab with or
// without Shift, Ctrl+W, Ctrl+F4, Ctrl+PageUp/PageDown, Ctrl+L, Alt+D, F6,
// Ctrl+D and Ctrl+1..9.
func browserShortcutKey(k browserview.Key) bool {
	switch {
	case !k.Ctrl && !k.Shift && (k.Key == "f6" || (k.Alt && k.Key == "d")):
		return true
	case !k.Ctrl || k.Alt:
		return false
	case k.Key == "t" || k.Key == "tab":
		return true
	case k.Shift:
		return false
	}
	switch k.Key {
	case "w", "f4", "pageup", "pagedown", "l", "d", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return true
	}
	return false
}

// browserShortcut hands a page's shortcut to the shell, which acts on it as
// on one pressed in the shell itself.
func (a *App) browserShortcut(k browserview.Key) bool {
	if !browserShortcutKey(k) {
		return false
	}
	a.emitEvent("browser:key", k)
	return true
}
