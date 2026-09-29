//go:build darwin

package browserview

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework WebKit
#include <stdlib.h>
#include "browser_view_darwin.h"
*/
import "C"
import (
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"runtime/cgo"
	"sync"
	"unsafe"
)

// webkitBlocker is a ContentBlocker the Mac can use (adblock.Blocker): WKWebView
// cannot be asked about each request, so its lists become one WebKit content
// blocker that every tab takes, made again when they change.
type webkitBlocker interface {
	WebKitRules() (all, network, version string)
	Enabled() bool
	OnChange(func())
}

var (
	rulesMu      sync.Mutex
	rulesWatched = map[webkitBlocker]bool{}
	rulesVersion string
)

// watchRules hands b's rules to WebKit, and again when they change.
func watchRules(b ContentBlocker) {
	w, ok := b.(webkitBlocker)
	if !ok {
		return
	}
	rulesMu.Lock()
	seen := rulesWatched[w]
	rulesWatched[w] = true
	rulesMu.Unlock()
	if seen {
		return
	}
	w.OnChange(func() { applyRules(w) })
	go applyRules(w)
}

func applyRules(w webkitBlocker) {
	if !w.Enabled() {
		C.rl_browser_rules_enabled(0)
		return
	}
	all, network, version := w.WebKitRules()
	if version == "" {
		return
	}
	rulesMu.Lock()
	same := version == rulesVersion
	rulesVersion = version
	rulesMu.Unlock()
	if same {
		C.rl_browser_rules_enabled(1)
		return
	}
	ca, cn, cv := C.CString(all), C.CString(network), C.CString(version)
	defer C.free(unsafe.Pointer(ca))
	defer C.free(unsafe.Pointer(cn))
	defer C.free(unsafe.Pointer(cv))
	C.rl_browser_rules(ca, cn, cv)
	C.rl_browser_rules_enabled(1)
}

func (m *Manager) contentBlocker() ContentBlocker {
	m.scriptMu.Lock()
	defer m.scriptMu.Unlock()
	return m.blocker
}

type nativeView struct {
	ptr    unsafe.Pointer
	handle cgo.Handle
}
type nativeManager struct {
	views   map[string]*nativeView
	overlay unsafe.Pointer
	closed  bool
}
type browserCallback struct {
	manager *Manager
	id      string
}

//export mayakBrowserEvent
func mayakBrowserEvent(h C.uintptr_t, u, t *C.char, back, forward, popup C.int) {
	c := cgo.Handle(h).Value().(browserCallback)
	c.manager.notify(Event{ID: c.id, URL: C.GoString(u), Title: C.GoString(t), CanBack: back != 0, CanForward: forward != 0, Popup: popup != 0})
}
func (v *nativeView) action(command, url string, left, top, right, bottom int) {
	c, u := C.CString(command), C.CString(url)
	defer C.free(unsafe.Pointer(c))
	defer C.free(unsafe.Pointer(u))
	C.rl_browser_action(v.ptr, c, u, C.int(left), C.int(top), C.int(right), C.int(bottom))
}
func (m *Manager) command(command string, o Options) error {
	return application.InvokeSyncWithError(func() error {
		if m.native.closed {
			return fmt.Errorf("browser has closed")
		}
		// Keyboard focus is not moved between the shell and the pages here.
		if command == "focus" {
			return nil
		}
		if m.native.views == nil {
			m.native.views = map[string]*nativeView{}
		}
		views := m.native.views
		v := views[o.ID]
		if command == "show" && v == nil {
			if len(views) >= 80 {
				return fmt.Errorf("too many browser tabs")
			}
			if m.window.NativeWindow() == nil {
				return fmt.Errorf("native window is not ready")
			}
			h := cgo.NewHandle(browserCallback{m, o.ID})
			ptr := C.rl_browser_new(m.window.NativeWindow(), C.uintptr_t(h))
			if ptr == nil {
				h.Delete()
				return fmt.Errorf("could not create browser view")
			}
			v = &nativeView{ptr, h}
			views[o.ID] = v
			if b := m.contentBlocker(); b != nil {
				watchRules(b)
			}
			v.action("navigate", o.URL, 0, 0, 0, 0)
		}
		if command == "show" || command == "hideAll" {
			for _, other := range views {
				other.action("hide", "", 0, 0, 0, 0)
			}
		}
		if v != nil && command != "hideAll" {
			v.action(command, o.URL, o.Left, o.Top, o.Right, o.Bottom)
			if command == "close" {
				v.handle.Delete()
				delete(views, o.ID)
			}
		}
		return nil
	})
}
func (m *Manager) close() {
	application.InvokeSync(func() {
		if m.native.closed {
			return
		}
		m.native.closed = true
		for id, v := range m.native.views {
			v.action("close", "", 0, 0, 0, 0)
			v.handle.Delete()
			delete(m.native.views, id)
		}
	})
}
