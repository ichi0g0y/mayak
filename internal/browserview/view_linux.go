//go:build linux

package browserview

/*
#cgo !gtk3 pkg-config: gtk4 webkitgtk-6.0
#cgo gtk3 pkg-config: gtk+-3.0 webkit2gtk-4.1
#cgo gtk3 CFLAGS: -DMAYAK_GTK3
#include <stdlib.h>
#include "browser_view_linux.h"
*/
import "C"
import (
	"fmt"
	"github.com/local/mayak/internal/appdir"
	"github.com/wailsapp/wails/v3/pkg/application"
	"os"
	"path/filepath"
	"runtime/cgo"
	"unsafe"
)

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

			if m.native.overlay == nil {
				m.native.overlay = C.rl_browser_overlay(m.window.NativeWindow())
			}
			if m.native.overlay == nil {
				h.Delete()
				return fmt.Errorf("could not create browser overlay")
			}
			d, e := os.UserConfigDir()
			if e != nil {
				h.Delete()
				return e
			}
			dir := C.CString(filepath.Join(d, appdir.Name, "browser-webdata", o.ID))
			defer C.free(unsafe.Pointer(dir))
			ptr := C.rl_browser_new((*C.GtkWidget)(m.native.overlay), C.uintptr_t(h), dir)
			if ptr == nil {
				h.Delete()
				return fmt.Errorf("could not create browser view")
			}
			v = &nativeView{ptr, h}
			views[o.ID] = v
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
