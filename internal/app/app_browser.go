package app

import (
	"encoding/json"
	"github.com/local/mayak/internal/appdir"

	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync"

	"github.com/local/mayak/internal/browserview"
	"github.com/local/mayak/internal/model"
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
	a.browserBackgroundOnce.Do(func() { go a.watchCatalog(); go a.watchScreenshotMaintenance() })
}

func browserStatePath() (string, error) {
	d, e := os.UserConfigDir()
	return filepath.Join(d, appdir.Name, "browser.json"), e
}
func (a *App) BrowserPlatform() string { return goruntime.GOOS }
func (a *App) BrowserLoad() (string, error) {
	browserStateMu.Lock()
	defer browserStateMu.Unlock()
	p, e := browserStatePath()
	if e != nil {
		return "", e
	}
	b, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return "{}", nil
	}
	return string(b), e
}

// Browser state contains only UI preferences, bookmarks and tabs. Tokens and
// temporary WebRTC descriptions are never stored in this file.
func (a *App) BrowserSave(raw string) error {
	if len(raw) > 1024*1024 || !json.Valid([]byte(raw)) {
		return errors.New("invalid browser state")
	}
	browserStateMu.Lock()
	defer browserStateMu.Unlock()
	p, e := browserStatePath()
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(p), "browser-*.tmp")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.WriteString(raw); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, p)
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
	case "hideAll", "close", "back", "forward", "reload":
	default:
		return errors.New("invalid browser command")
	}
	if a.browserViews == nil {
		return errors.New("browser is not ready")
	}
	return a.browserViews.Command(command, v)
}
func (a *App) showBrowserTask(status model.Status, site string) {
	urls := map[string]string{}
	for _, s := range []string{"tarkov-dev", "official-wiki", "japanese-wiki"} {
		urls[s] = questStatusURL(s, status)
	}
	a.emitEvent("browser:task", map[string]interface{}{"id": status.QuestID, "name": status.LastQuest, "site": normalizeQuestSite(site), "urls": urls})
}
