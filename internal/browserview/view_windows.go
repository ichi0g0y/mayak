//go:build windows

package browserview

import (
	"errors"
	"fmt"
	"github.com/local/mayak/internal/appdir"
	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"
)

type nativeView struct {
	chromium *edge.Chromium
	host     w32.HWND
	options  Options
	cleanup  func()
}
type nativeManager struct {
	views   map[string]*nativeView
	hwnd    uintptr
	closed  bool
	active  string
	showing bool
}

var subclassDLL = windows.NewLazySystemDLL("comctl32.dll")
var setSubclass = subclassDLL.NewProc("SetWindowSubclass")
var removeSubclass = subclassDLL.NewProc("RemoveWindowSubclass")
var defSubclass = subclassDLL.NewProc("DefSubclassProc")

// The registry is only accessed on Wails' UI thread. No Go pointer is stored by Win32.
var managers = map[uintptr]*Manager{}
var subclassProc uintptr

func init() {
	subclassProc = windows.NewCallback(func(hwnd uintptr, msg uint32, wp, lp, id, data uintptr) uintptr {
		result, _, _ := defSubclass.Call(hwnd, uintptr(msg), wp, lp)
		if m := managers[hwnd]; m != nil {
			switch msg {
			case 0x0018: // WM_SHOWWINDOW, including restoration by the OS/launcher.
				if wp != 0 && !m.native.showing {
					// Showing an HWND directly does not restore the controller that
					// Wails hid for tray mode. Reconcile both visibility states.
					m.native.showing = true
					m.window.Show()
					m.native.showing = false
				}
			case 0x0005: // WM_SIZE. Do not resize to a minimised zero-sized client area.
				if wp != 1 {
					m.resize()
				}
			case 0x02e0:
				m.resize() // WM_DPICHANGED
			case 0x0003, 0x0216: // WM_MOVE / WM_MOVING (WebView2 popup/input coordinates)
				if v := m.native.views[m.native.active]; v != nil {
					_ = v.chromium.NotifyParentWindowPositionChanged()
				}
			case 0x0082:
				m.closeNative() // WM_NCDESTROY fallback
			case 0x0083: // WM_NCCALCSIZE
				// The window is frameless, so Wails makes the whole window client
				// area and resizes it from the page's edges. The page views are
				// child HWNDs that take the mouse, so that never sees the right and
				// bottom edges under them. Keep Windows' own sizing borders on the
				// left, right and bottom instead: the invisible strip outside the
				// visible edge that every ordinary window (Chrome included) resizes
				// from, so nothing inside, scrollbar arrows included, is covered.
				if wp != 0 && !w32.IsZoomed(w32.HWND(hwnd)) && !m.window.IsFullscreen() {
					rect := (*w32.RECT)(unsafe.Pointer(lp))
					frame := int32(frameWidth(w32.HWND(hwnd)))
					rect.Left += frame
					rect.Right -= frame
					rect.Bottom -= frame
				}
			case 0x0084: // WM_NCHITTEST: those borders resize the window.
				if hit := frameHit(w32.HWND(hwnd), lp); hit != 0 {
					return hit
				}
			}
		}
		return result
	})
}

// frameWidth is the width of the sizing border Windows gives a window at its
// DPI (the sizing frame plus the padded border, 8px at 96 DPI).
func frameWidth(hwnd w32.HWND) int {
	dpi := w32.GetDpiForWindow(hwnd)
	if dpi == 0 {
		dpi = 96
	}
	return w32.GetSystemMetricsForDpi(w32.SM_CXSIZEFRAME, dpi) + w32.GetSystemMetricsForDpi(w32.SM_CXPADDEDBORDER, dpi)
}

// frameHit maps a screen point in the window's left, right or bottom sizing
// border to its hit code, or 0 inside the client area or outside the window.
func frameHit(hwnd w32.HWND, lp uintptr) uintptr {
	if w32.IsZoomed(hwnd) {
		return 0
	}
	x, y := int32(int16(lp&0xffff)), int32(int16((lp>>16)&0xffff))
	window := w32.GetWindowRect(hwnd)
	if window == nil || x < window.Left || x >= window.Right || y < window.Top || y >= window.Bottom {
		return 0
	}
	frame := int32(frameWidth(hwnd))
	left, right, bottom := x < window.Left+frame, x >= window.Right-frame, y >= window.Bottom-frame
	switch {
	case bottom && left:
		return w32.HTBOTTOMLEFT
	case bottom && right:
		return w32.HTBOTTOMRIGHT
	case bottom:
		return w32.HTBOTTOM
	case left:
		return w32.HTLEFT
	case right:
		return w32.HTRIGHT
	}
	return 0
}

func (m *Manager) resize() {
	dpi := w32.GetDpiForWindow(w32.HWND(m.native.hwnd))
	if dpi == 0 {
		dpi = 96
	}
	if v := m.native.views[m.native.active]; v != nil {
		m.place(v, dpi)
	}
}

// place sizes v to its area of the window without changing its visibility.
func (m *Manager) place(v *nativeView, dpi uint) {
	bounds := w32.GetClientRect(w32.HWND(m.native.hwnd))
	if bounds == nil {
		return
	}
	left, top := v.options.Left*int(dpi)/96, v.options.Top*int(dpi)/96
	right, bottom := v.options.Right*int(dpi)/96, v.options.Bottom*int(dpi)/96
	width, height := max(0, int(bounds.Right)-left-right), max(0, int(bounds.Bottom)-top-bottom)
	// A separate clipped child HWND keeps the site's input surface and z-order
	// independent from Wails' full-window shell controller during live resize.
	w32.SetWindowPos(v.host, w32.HWND_TOP, left, top, width, height, w32.SWP_NOACTIVATE)
	v.chromium.ResizeWithBounds(&edge.Rect{Right: int32(width), Bottom: int32(height)})
}
func (m *Manager) command(command string, o Options) error {
	var pending chan error
	var initializing *edge.Chromium
	defer func() { runtime.KeepAlive(initializing) }()
	err := application.InvokeSyncWithError(func() error {
		if m.native.closed {
			return errors.New("browser has closed")
		}
		if m.native.views == nil {
			m.native.views = map[string]*nativeView{}
		}
		if m.native.hwnd == 0 {
			hwnd := uintptr(m.window.NativeWindow())
			if hwnd == 0 {
				return errors.New("native window is not ready")
			}
			ok, _, err := setSubclass.Call(hwnd, subclassProc, 0x524c, 0)
			if ok == 0 {
				return err
			}
			m.native.hwnd = hwnd
			managers[hwnd] = m
			// Apply the sizing borders (WM_NCCALCSIZE above) to the existing frame.
			w32.SetWindowPos(w32.HWND(hwnd), 0, 0, 0, 0, 0, w32.SWP_FRAMECHANGED|w32.SWP_NOMOVE|w32.SWP_NOSIZE|w32.SWP_NOZORDER|w32.SWP_NOACTIVATE)
		}
		views := m.native.views
		v := views[o.ID]
		// "focus" moves the keyboard to the shell (ID "shell") or to a page.
		// The shell's own focus method exits the process on a WebView2 error;
		// a page's focus is asked for directly and a refusal is ignored.
		if command == "focus" {
			if o.ID == "shell" {
				m.window.Focus()
			} else if v != nil {
				if controller := v.chromium.GetController(); controller != nil {
					_ = controller.MoveFocus(edge.COREWEBVIEW2_MOVE_FOCUS_REASON_PROGRAMMATIC)
				}
			}
			return nil
		}
		// "preload" creates and loads a tab without showing it, so it is ready
		// when it is first shown.
		if command == "preload" && v != nil {
			return nil
		}
		if (command == "show" || command == "preload") && v == nil {
			if len(views) >= 80 {
				return errors.New("too many browser tabs")
			}
			dir, err := os.UserConfigDir()
			if err != nil {
				return err
			}
			c := edge.NewChromium()
			initializing = c
			c.DisableHostMessaging = true
			c.DataPath = filepath.Join(dir, appdir.Name, "browser-webdata")
			c.MessageCallback = func(string) {}
			// The shell's shortcuts (Ctrl+T and the like) work with the
			// keyboard in a page too: WebView2 offers every key with a modifier
			// held before the page gets it.
			id := o.ID
			c.AcceleratorKeyCallback = func(vk uint) bool { return m.acceleratorKey(id, vk) }
			host := w32.CreateWindowEx(0, w32.MustStringToUTF16Ptr("STATIC"), nil,
				w32.WS_CHILD|w32.WS_CLIPCHILDREN|w32.WS_CLIPSIBLINGS,
				0, 0, 1, 1, w32.HWND(m.native.hwnd), 0, w32.GetModuleHandle(""), nil)
			if host == 0 {
				return errors.New("could not create browser host")
			}
			// Do not run Embed's nested GetMessage loop inside a Wails callback.
			// Initialise asynchronously and complete the RPC on its calling goroutine.
			// The current tab stays visible until the new one can replace it.
			pending = make(chan error, 1)
			c.EmbedAsync(uintptr(host), func(initErr error) {
				if initErr != nil {
					w32.DestroyWindow(host)
					pending <- initErr
					return
				}
				if m.native.closed {
					_ = c.BrowserCommand("close", "")
					w32.DestroyWindow(host)
					pending <- errors.New("browser has closed")
					return
				}
				setup := func() error {
					_ = c.Hide()
					setBackground(c, o.Background)
					cleanup, err := c.BrowserIsolation(func(e edge.BrowserEvent) {
						m.notify(Event{ID: o.ID, URL: e.URL, Title: e.Title, CanBack: e.CanBack, CanForward: e.CanForward, Popup: e.Popup, Favicon: e.Favicon, Loading: e.Loading})
					})
					if err != nil {
						_ = c.BrowserCommand("close", "")
						w32.DestroyWindow(host)
						return err
					}
					if script := m.currentDocumentScript(); script != "" {
						_ = c.AddDocumentScript(script)
					}
					if blocker := m.contentBlocker(); blocker != nil {
						filter, err := attachFilter(c, blocker)
						if err != nil {
							cleanup()
							_ = c.BrowserCommand("close", "")
							w32.DestroyWindow(host)
							return err
						}
						isolationCleanup := cleanup
						cleanup = func() { filter.close(); isolationCleanup() }
					}
					v = &nativeView{chromium: c, host: host, options: o, cleanup: cleanup}
					views[o.ID] = v
					if err = c.BrowserCommand("navigate", o.URL); err != nil {
						cleanup()
						_ = c.BrowserCommand("close", "")
						w32.DestroyWindow(host)
						delete(views, o.ID)
						return err
					}
					if command == "preload" {
						dpi := w32.GetDpiForWindow(w32.HWND(m.native.hwnd))
						if dpi == 0 {
							dpi = 96
						}
						m.place(v, dpi)
						return nil
					}
					m.native.active = o.ID
					return m.showOnly(v)
				}
				pending <- setup()
			})
			return nil

		}
		if command == "hideAll" {
			for _, other := range views {
				hideView(other)
			}
			m.native.active = ""
		}
		if v == nil {
			return nil
		}
		if command == "show" {
			if v.options.Background != o.Background {
				setBackground(v.chromium, o.Background)
			}
			v.options = o
			m.native.active = o.ID
			return m.showOnly(v)
		}
		if command == "hideAll" {
			return nil
		}
		if command == "close" {
			v.cleanup()
			err := v.chromium.BrowserCommand("close", "")
			w32.DestroyWindow(v.host)
			delete(views, o.ID)
			if m.native.active == o.ID {
				m.native.active = ""
			}
			return err
		}
		return v.chromium.BrowserCommand(command, o.URL)
	})
	if err != nil {
		return err
	}
	if pending != nil {
		return <-pending
	}
	return nil
}

// showOnly raises v first and hides the other tabs afterwards, so the shell
// behind them is never exposed in between (which shows as a flicker).
func (m *Manager) showOnly(v *nativeView) error {
	m.resize()
	w32.ShowWindow(v.host, w32.SW_SHOWNOACTIVATE)
	err := v.chromium.Show()
	for _, other := range m.native.views {
		if other != v {
			hideView(other)
		}
	}
	return err
}

func hideView(v *nativeView) {
	w32.ShowWindow(v.host, w32.SW_HIDE)
	_ = v.chromium.Hide()
}

func setBackground(c *edge.Chromium, color string) {
	if len(color) != 7 || color[0] != '#' {
		return
	}
	var rgb [3]uint8
	if _, err := fmt.Sscanf(color[1:], "%02x%02x%02x", &rgb[0], &rgb[1], &rgb[2]); err != nil {
		return
	}
	_ = c.SetDefaultBackground(rgb[0], rgb[1], rgb[2])
}

func (m *Manager) closeNative() {
	if m.native.closed {
		return
	}
	m.native.closed = true
	for id, v := range m.native.views {
		v.cleanup()
		_ = v.chromium.BrowserCommand("close", "")
		w32.DestroyWindow(v.host)
		delete(m.native.views, id)
	}
	if m.native.hwnd != 0 {
		removeSubclass.Call(m.native.hwnd, subclassProc, 0x524c)
		delete(managers, m.native.hwnd)
		m.native.hwnd = 0
	}
}
func (m *Manager) close() { application.InvokeSync(m.closeNative) }

// acceleratorKey offers a key pressed in view id, with a modifier held, to
// the key handler under the name the shell's keyboard events give it (F6
// counts alone, as in Chrome). Other keys, and keys the handler declines,
// stay with the page.
func (m *Manager) acceleratorKey(id string, vk uint) bool {
	down := func(key int32) bool { return w32.GetKeyState(key) < 0 }
	ctrl, shift, alt := down(w32.VK_CONTROL), down(w32.VK_SHIFT), down(w32.VK_MENU)
	var name string
	switch {
	case vk >= '0' && vk <= '9', vk >= 'A' && vk <= 'Z':
		name = strings.ToLower(string(rune(vk)))
	case vk == w32.VK_TAB:
		name = "tab"
	case vk == w32.VK_PRIOR:
		name = "pageup"
	case vk == w32.VK_NEXT:
		name = "pagedown"
	case vk == w32.VK_F4:
		name = "f4"
	case vk == w32.VK_F6:
		name = "f6"
	default:
		return false
	}
	if !ctrl && !alt && name != "f6" {
		return false
	}
	handler := m.keyHandler()
	return handler != nil && handler(Key{ID: id, Key: name, Ctrl: ctrl, Shift: shift, Alt: alt})
}
