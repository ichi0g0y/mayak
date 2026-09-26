package app

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/remote"
)

// Settings.BrowserRemoteID is the Remote Control ID that tarkov.dev map pages
// in the built-in browser connect with automatically. It is generated on first
// run from tarkov.dev's alphabet. tarkov.dev's own IDs have 4 characters, few
// enough that another page may already use one; this ID is only used inside
// MAYAK, so it is long enough not to collide or be guessed.
const (
	browserRemoteIDAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	browserRemoteIDLength   = 12
	// A command relayed back within this time of sending the same one is ours.
	browserRemoteEcho = 15 * time.Second
)

var browserRemoteIDPattern = regexp.MustCompile(fmt.Sprintf(`^[A-Z0-9]{%d}$`, browserRemoteIDLength))

func newBrowserRemoteID() string {
	b := make([]byte, browserRemoteIDLength)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = browserRemoteIDAlphabet[int(b[i])%len(browserRemoteIDAlphabet)]
	}
	return string(b)
}

// ensureBrowserRemoteID reports whether a new ID had to be generated. An
// earlier, shorter ID is replaced too.
func ensureBrowserRemoteID(s *config.Settings) bool {
	if browserRemoteIDPattern.MatchString(s.BrowserRemoteID) {
		return false
	}
	s.BrowserRemoteID = newBrowserRemoteID()
	return true
}

// tarkovDevScript runs before tarkov.dev's own scripts, on map pages and the
// map index (the map view's start page before a map is detected) only.
// It adds the player marker's style sheet (css, see app_marker.go; none for
// tarkov.dev's own marker) and connects the page to Remote Control: tarkov.dev
// reads ?connection=<ID> on start-up, stores it as its session ID and enables
// Remote Control. A connected task page would be navigated away by map
// commands, hence map pages only. The browser shell strips the parameter
// from tab URLs.
// The page connects with the session ID it read from localStorage before that
// effect ran, so the ID is also written there first: otherwise a page that
// once had its own ID keeps connecting with it.
// A tab keeps the script it was created with, so after the ID is replaced
// (replaceBrowserRemoteID) an ID in the address wins over the one built in:
// the shell reopens its map view with the new one (and after the marker
// style changes, browser:document-script).
func tarkovDevScript(id, css string) string {
	quoted, _ := json.Marshal(id)
	sheet, _ := json.Marshal(css)
	return `(()=>{try{
const p=location.pathname;if(location.hostname!=="tarkov.dev"||!(p.startsWith("/map/")||p==="/maps"||p==="/maps/"))return;
const css=` + string(sheet) + `;
if(css){const add=()=>{if(document.getElementById("mayak-player-marker"))return;const s=document.createElement("style");s.id="mayak-player-marker";s.textContent=css;(document.head||document.documentElement).appendChild(s);};if(document.documentElement)add();else document.addEventListener("DOMContentLoaded",add);}
const u=new URL(location.href),given=u.searchParams.get("connection"),id=/^[A-Z0-9]{4,32}$/.test(given||"")?given:` + string(quoted) + `;
if(!id)return;
localStorage.setItem("sessionId",JSON.stringify(id));
if(given===id)return;
u.searchParams.set("connection",id);history.replaceState(history.state,"",u);
}catch(e){}})();`
}

// tarkovDevScript builds the document script from the current settings.
func (a *App) tarkovDevScript() string {
	a.mu.RLock()
	settings := a.settings
	a.mu.RUnlock()
	return tarkovDevScript(settings.BrowserRemoteID, a.playerMarkerCSS(settings))
}

// applyBrowserScript gives the page views, the popup's too, the document
// script for the current settings. Pages already open keep their script:
// the shell recreates its map view on browser:document-script.
func (a *App) applyBrowserScript() {
	script := a.tarkovDevScript()
	if a.browserViews != nil {
		a.browserViews.SetDocumentScript(script)
	}
	a.popup.mu.Lock()
	if a.popup.views != nil {
		a.popup.views.SetDocumentScript(script)
	}
	a.popup.mu.Unlock()
}

// BrowserRemoteID lets the browser shell open its fixed map view with
// ?connection=<ID> directly, instead of waiting for the document script.
func (a *App) BrowserRemoteID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings.BrowserRemoteID
}

func (a *App) setupBrowserRemote() {
	a.mu.RLock()
	id := a.settings.BrowserRemoteID
	a.mu.RUnlock()
	if a.browserViews == nil || id == "" {
		return
	}
	a.applyBrowserScript()
	a.watchBrowserRemote(id)
}

// watchBrowserRemote listens on the ID as the map page does. Only MAYAK sends
// to it, so a command MAYAK did not send means someone else uses the same ID:
// then it is replaced.
func (a *App) watchBrowserRemote(id string) {
	a.browserRemoteMu.Lock()
	if a.browserRemoteStop != nil {
		close(a.browserRemoteStop)
	}
	stop := make(chan struct{})
	a.browserRemoteStop = stop
	a.browserRemoteSent = make(map[string]time.Time)
	a.browserRemoteMu.Unlock()
	go func() {
		select {
		case <-a.done:
			a.browserRemoteMu.Lock()
			if a.browserRemoteStop == stop {
				close(stop)
				a.browserRemoteStop = nil
			}
			a.browserRemoteMu.Unlock()
		case <-stop:
		}
	}()
	go remote.Watch(stop, id, func(key string) { a.browserRemoteCommand(id, key) })
}

// noteBrowserRemoteSend records a command sent to the browser's ID.
func (a *App) noteBrowserRemoteSend(key string) {
	a.browserRemoteMu.Lock()
	defer a.browserRemoteMu.Unlock()
	if a.browserRemoteSent == nil {
		a.browserRemoteSent = make(map[string]time.Time)
	}
	now := time.Now()
	for k, at := range a.browserRemoteSent {
		if now.Sub(at) > browserRemoteEcho {
			delete(a.browserRemoteSent, k)
		}
	}
	a.browserRemoteSent[key] = now
}

// browserRemoteSentRecently reports whether MAYAK sent this command lately.
func (a *App) browserRemoteSentRecently(key string) bool {
	a.browserRemoteMu.Lock()
	defer a.browserRemoteMu.Unlock()
	at, ok := a.browserRemoteSent[key]
	return ok && time.Since(at) <= browserRemoteEcho
}

func (a *App) browserRemoteCommand(id, key string) {
	if !a.browserRemoteSentRecently(key) {
		a.replaceBrowserRemoteID(id)
	}
}

// replaceBrowserRemoteID gives the browser a new ID when id is still current.
func (a *App) replaceBrowserRemoteID(id string) {
	if a.quitting.Load() {
		return
	}
	a.settingsWriteMu.Lock()
	a.mu.Lock()
	if a.settings.BrowserRemoteID != id {
		a.mu.Unlock()
		a.settingsWriteMu.Unlock()
		return
	}
	next := newBrowserRemoteID()
	a.settings.BrowserRemoteID = next
	settings := a.settings
	old := a.remotes[id]
	delete(a.remotes, id)
	a.mu.Unlock()
	err := config.Save(settings)
	a.settingsWriteMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	a.addLog("Warn", "Remote", fmt.Sprintf("Remote Control ID %s received a command MAYAK did not send, so another page uses it; the built-in browser now uses %s", id, next))
	if err != nil {
		a.addLog("Error", "Remote", "Could not save the new Remote Control ID: "+err.Error())
	}
	a.setupBrowserRemote()
	a.emitEvent("browser:remote-id", next)
}
