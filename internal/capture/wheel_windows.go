//go:build windows

package capture

import (
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandleW     = kernel32.NewProc("GetModuleHandleW")
	procSetWindowsHookExW    = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx  = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx       = user32.NewProc("CallNextHookEx")
	procMsgWaitForMultiple   = user32.NewProc("MsgWaitForMultipleObjectsEx")
	procWindowFromPoint      = user32.NewProc("WindowFromPoint")
	procGetAncestor          = user32.NewProc("GetAncestor")
	procGetParent            = user32.NewProc("GetParent")
	procGetClassNameW        = user32.NewProc("GetClassNameW")
	procPostMessageW         = user32.NewProc("PostMessageW")
	procEnumWindows          = user32.NewProc("EnumWindows")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procIsIconic             = user32.NewProc("IsIconic")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
	procScreenToClient       = user32.NewProc("ScreenToClient")
	procGetDpiForWindow      = user32.NewProc("GetDpiForWindow")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
)

const (
	whMouseLL          = 14
	wmMouseWheel       = 0x020A
	wmMouseHWheel      = 0x020E
	wmMButtonDown      = 0x0207
	wmXButtonDown      = 0x020B
	pmRemove           = 0x0001
	qsAllInput         = 0x04FF
	mwmoInputAvailable = 0x0004
	gaRoot             = 2
)

// wheelInGame and wheelAway are how often the forwarder looks at the window
// in front and the cursor, with EFT in front and without.
const (
	wheelInGame = 50 * time.Millisecond
	wheelAway   = 250 * time.Millisecond
)

// lastWheel is when a wheel turn was last taken for MAYAK (UnixNano), so a
// right click it was turned with is no click to capture after (WatchClicks).
var lastWheel atomic.Int64

// WheelForwarder takes the wheel turned in EFT with a combo held to MAYAK.
type WheelForwarder struct {
	thread uint32
	done   chan struct{}
}

// wheel is the forwarder's state the hook reads: one forwarder runs at a
// time (the hook's callback is made once for the process).
var wheel struct {
	// scrollVK and tabsVK are the combos' keys, 0 for none; byScript is
	// whether the scroll combo is a key (WheelOptions.ScrollPage).
	scrollVK, tabsVK atomic.Uintptr
	byScript         atomic.Bool
	opts             atomic.Pointer[WheelOptions]
	// tabTurn adds up a wheel's finer steps to a notch; noted is when the
	// last note was made (noteEvery).
	tabTurn int
	noted   time.Time
	// buttonUp is the release of a button pressed for the tabs, to take too.
	buttonUp uintptr
	once     sync.Once
	hookCB   uintptr
}

// ForwardWheel takes the wheel turned in EFT with the cursor shown to MAYAK
// (WheelOptions): a scroll to the page MAYAK shows, whatever the cursor was
// over in it last. The wheel is watched only then, by a low-level mouse hook
// put in when EFT is in front with its cursor shown and taken out when it is
// not (while the player aims or is in another window), so the game's mouse
// goes through nothing of MAYAK's while it is played. Windows lets a hook keep
// the turn from the game's window messages, but EFT reads the mouse itself
// (raw input) too, so it may turn there as well.
func ForwardWheel(o WheelOptions) (*WheelForwarder, error) {
	scrollVK, tabsVK := comboKeys[o.Scroll], comboKeys[o.Tabs]
	if (o.Scroll != "" && !slices.Contains(WheelCombos, o.Scroll)) || (o.Tabs != "" && !slices.Contains(WheelCombos, o.Tabs)) || (scrollVK|tabsVK == 0 && o.CloseTab == "" && o.PinTab == "") {
		return nil, ErrUnsupported
	}
	wheel.opts.Store(&o)
	wheel.scrollVK.Store(scrollVK)
	wheel.tabsVK.Store(tabsVK)
	wheel.byScript.Store(keyCombos[o.Scroll])
	wheel.once.Do(func() { wheel.hookCB = windows.NewCallback(wheelHook) })
	f := &WheelForwarder{done: make(chan struct{})}
	started := make(chan struct{})
	go func() {
		defer close(f.done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		f.thread = windows.GetCurrentThreadId()
		close(started)
		var hook, front uintptr
		var inGame bool
		defer func() {
			if hook != 0 {
				procUnhookWindowsHookEx.Call(hook)
			}
		}()
		for {
			var m msg
			for {
				r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
				if r == 0 {
					break
				}
				if m.message == wmQuit {
					return
				}
			}
			if hwnd, _, _ := procGetForegroundWindow.Call(); hwnd != front {
				front, inGame = hwnd, hwnd != 0 && isGame(hwnd)
			}
			wait := wheelAway
			if inGame {
				wait = wheelInGame
				if !aiming(front) {
					if hook == 0 {
						module, _, _ := procGetModuleHandleW.Call(0)
						hook, _, _ = procSetWindowsHookExW.Call(whMouseLL, wheel.hookCB, module, 0)
					}
				} else if hook != 0 {
					procUnhookWindowsHookEx.Call(hook)
					hook = 0
				}
			} else if hook != 0 {
				procUnhookWindowsHookEx.Call(hook)
				hook = 0
			}
			procMsgWaitForMultiple.Call(0, 0, uintptr(wait.Milliseconds()), qsAllInput, mwmoInputAvailable)
		}
	}()
	<-started
	return f, nil
}

// Stop ends the forwarding and takes the hook out.
func (f *WheelForwarder) Stop() {
	procPostThreadMessageW.Call(uintptr(f.thread), wmQuit, 0, 0)
	<-f.done
}

// comboKeys are the virtual keys of WheelCombos' buttons and keys.
var comboKeys = map[string]uintptr{"right": 0x02, "x1": 0x05, "x2": 0x06, "shift": 0x10, "ctrl": 0x11, "alt": 0x12}

// keyCombos are those held on the keyboard: a page told the wheel itself
// with one down reads Shift as sideways and Ctrl as zoom.
var keyCombos = map[string]bool{"shift": true, "ctrl": true, "alt": true}

// wheelHook takes a wheel turn made with the combo held to MAYAK; all else,
// and a turn there is no MAYAK window to take, goes on to the game.
func wheelHook(code, wParam uintptr, info *mouseHookInfo) uintptr {
	// The button that closes the tab shown, or pins or unpins it
	// (WheelOptions.CloseTab, PinTab). Its release is taken too.
	if int32(code) == 0 && (wParam == wmMButtonDown || wParam == wmXButtonDown) {
		if o := wheel.opts.Load(); o != nil && o.TabAction != nil {
			action := ""
			switch tabButton(wParam, info.mouseData) {
			case "":
			case o.CloseTab:
				action = "close"
			case o.PinTab:
				action = "pin"
			}
			if action != "" {
				go o.TabAction(action)
				wheel.buttonUp = wParam + 1 // WM_MBUTTONUP, WM_XBUTTONUP
				noteWheel("button: " + action + " the tab")
				return 1
			}
		}
	}
	if int32(code) == 0 && wheel.buttonUp != 0 && wParam == wheel.buttonUp {
		wheel.buttonUp = 0
		return 1
	}
	if int32(code) == 0 && (wParam == wmMouseWheel || wParam == wmMouseHWheel) {
		delta := int16(info.mouseData >> 16)
		kind := "wheel"
		// Some mice's software turn Shift with the wheel into the
		// horizontal wheel: right is down.
		if wParam == wmMouseHWheel {
			kind, delta = "horizontal wheel", -delta
		}
		// The tabs combo first: the two may share a key with a third held.
		if held(wheel.tabsVK.Load()) {
			lastWheel.Store(time.Now().UnixNano())
			if wheel.tabTurn = wheel.tabTurn + int(delta); wheel.tabTurn >= wheelNotch || wheel.tabTurn <= -wheelNotch {
				step := 1 // turned towards the user: the next tab
				if wheel.tabTurn > 0 {
					step = -1
				}
				wheel.tabTurn = 0
				if o := wheel.opts.Load(); o != nil && o.SwitchTab != nil {
					go o.SwitchTab(step)
				}
			}
			noteWheel(kind + " for the tabs")
			return 1
		}
		wheel.tabTurn = 0
		if held(wheel.scrollVK.Load()) {
			target, ok := sendWheel(delta)
			if ok {
				lastWheel.Store(time.Now().UnixNano())
				noteWheel(kind + " sent to " + target)
				return 1
			}
			noteWheel(kind + " left to the game: no MAYAK window to scroll")
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, code, wParam, uintptr(unsafe.Pointer(info)))
	return r
}

// noteEvery keeps the notes about the wheel few.
const noteEvery = time.Second

func noteWheel(text string) {
	if time.Since(wheel.noted) < noteEvery {
		return
	}
	wheel.noted = time.Now()
	if o := wheel.opts.Load(); o != nil && o.Note != nil {
		go o.Note(text)
	}
}

// notchPixels is how far a notch scrolls a page in Chromium: Windows' lines
// a notch (3 unless changed) at 100/3 CSS pixels a line.
func notchPixels() float64 {
	var lines uint32
	if ok, _, _ := procSystemParametersInfo.Call(0x0068, 0, uintptr(unsafe.Pointer(&lines)), 0); ok == 0 || lines == 0 || lines > 100 { // SPI_GETWHEELSCROLLLINES; a page a notch is 0xFFFFFFFF
		lines = 3
	}
	return float64(lines) * 100 / 3
}

// tabButton names the button pressed (TabButtons): the middle one, alone or
// with Shift, Ctrl or Alt held, or a side button.
func tabButton(wParam uintptr, mouseData uint32) string {
	if wParam == wmXButtonDown {
		switch mouseData >> 16 {
		case 1:
			return "x1"
		case 2:
			return "x2"
		}
		return ""
	}
	// The middle button with one key held, or with none; two keys held make no button.
	button := "middle"
	for _, key := range []struct {
		vk   uintptr
		name string
	}{{vkShift, "shift+middle"}, {vkControl, "ctrl+middle"}, {vkAlt, "alt+middle"}} {
		if held(key.vk) {
			if button != "middle" {
				return ""
			}
			button = key.name
		}
	}
	return button
}

// vkControl and vkAlt are the Ctrl and Alt keys.
const (
	vkControl = 0x11
	vkAlt     = 0x12
)

// wheelNotch is one notch of the wheel (WHEEL_DELTA).
const wheelNotch = 120

func held(vk uintptr) bool {
	if vk == 0 {
		return false
	}
	state, _, _ := procGetAsyncKeyState.Call(vk)
	return state&0x8000 != 0
}

// mouseHookInfo is the start of MSLLHOOKSTRUCT.
type mouseHookInfo struct {
	pt        struct{ x, y int32 }
	mouseData uint32
}

// sendWheel turns MAYAK's wheel by delta at the cursor's last place over it,
// or the middle of its window; target says where it went.
func sendWheel(delta int16) (target string, ok bool) {
	hwnd, at, ok := wheelTarget()
	if !ok {
		return "", false
	}
	if wheel.byScript.Load() {
		o := wheel.opts.Load()
		if o == nil || o.ScrollPage == nil {
			return "", false
		}
		pt := struct{ x, y int32 }{int32(uint32(at)), int32(uint32(at >> 32))}
		procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
		dpi, _, _ := procGetDpiForWindow.Call(hwnd)
		if dpi == 0 {
			dpi = 96
		}
		scale := 96 / float64(dpi)
		// A notch scrolls as far as in Chromium; up is back.
		dy := -float64(delta) / wheelNotch * notchPixels()
		go o.ScrollPage(hwnd, float64(pt.x)*scale, float64(pt.y)*scale, dy)
		return "the page script of " + className(hwnd), true
	}
	x, y := int16(int32(uint32(at))), int16(int32(uint32(at>>32)))
	lParam := uintptr(uint16(x)) | uintptr(uint16(y))<<16
	r, _, _ := procPostMessageW.Call(hwnd, wmMouseWheel, uintptr(uint16(delta))<<16, lParam)
	return className(hwnd), r != 0
}

// wheelTarget is MAYAK's window the wheel goes to and the screen point it
// is turned at (packPoint): WheelOptions.Point, or the middle of MAYAK's
// largest window.
func wheelTarget() (uintptr, uintptr, bool) {
	pid := windows.GetCurrentProcessId()
	var target uintptr
	x, y, ok := int32(0), int32(0), false
	if o := wheel.opts.Load(); o != nil && o.Point != nil {
		x, y, ok = o.Point()
	}
	if !ok {
		if x, y, ok = ownWindowMiddle(pid); !ok {
			return 0, 0, false
		}
	}
	at := packPoint(x, y)
	if w, _, _ := procWindowFromPoint.Call(at); ownWindow(w, pid) {
		target = w
	}
	if target == 0 {
		return 0, 0, false
	}
	// Chromium draws over its page with a window of its own that takes no
	// input; the page's is under it.
	if className(target) == "Intermediate D3D Window" {
		if parent, _, _ := procGetParent.Call(target); parent != 0 {
			target = parent
		}
	}
	return target, at, true
}

// WheelTurnedSince reports whether a wheel turn was sent to MAYAK after t.
func WheelTurnedSince(t time.Time) bool { return lastWheel.Load() > t.UnixNano() }

// packPoint makes a POINT passed by value (WindowFromPoint's argument).
func packPoint(x, y int32) uintptr { return uintptr(uint32(x)) | uintptr(uint32(y))<<32 }

// ownWindow reports whether hwnd is in one of this process's windows (a
// WebView's page belongs to its browser process, its root window to MAYAK).
func ownWindow(hwnd uintptr, pid uint32) bool {
	if hwnd == 0 {
		return false
	}
	root, _, _ := procGetAncestor.Call(hwnd, gaRoot)
	if root == 0 {
		return false
	}
	var owner uint32
	procGetWindowThreadProcess.Call(root, uintptr(unsafe.Pointer(&owner)))
	return owner == pid
}

// ownWindowMiddle is the middle of this process's largest visible window
// (the main one; popups are smaller).
func ownWindowMiddle(pid uint32) (x, y int32, ok bool) {
	var best int64
	cb := enumCallback()
	enumFound = func(hwnd uintptr) {
		var owner uint32
		procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner != pid {
			return
		}
		if v, _, _ := procIsWindowVisible.Call(hwnd); v == 0 {
			return
		}
		if m, _, _ := procIsIconic.Call(hwnd); m != 0 {
			return
		}
		var r struct{ left, top, right, bottom int32 }
		if got, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); got == 0 {
			return
		}
		if area := int64(r.right-r.left) * int64(r.bottom-r.top); area > best {
			best, x, y, ok = area, (r.left+r.right)/2, (r.top+r.bottom)/2, true
		}
	}
	procEnumWindows.Call(cb, 0)
	enumFound = nil
	return x, y, ok
}

// enumFound is what EnumWindows' callback calls; only the hook's thread
// enumerates.
var (
	enumFound func(uintptr)
	enumOnce  sync.Once
	enumCB    uintptr
)

func enumCallback() uintptr {
	enumOnce.Do(func() {
		enumCB = windows.NewCallback(func(hwnd, _ uintptr) uintptr {
			if enumFound != nil {
				enumFound(hwnd)
			}
			return 1
		})
	})
	return enumCB
}

func className(hwnd uintptr) string {
	var buf [64]uint16
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}
