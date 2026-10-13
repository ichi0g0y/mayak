//go:build windows

package capture

import (
	"errors"
	"image"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// gameExe is the game's process (eftdetect's gameExe).
const gameExe = "EscapeFromTarkov.exe"

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procRegisterHotKey         = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey       = user32.NewProc("UnregisterHotKey")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procPeekMessageW           = user32.NewProc("PeekMessageW")
	procPostThreadMessageW     = user32.NewProc("PostThreadMessageW")
	procGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	procGetClientRect          = user32.NewProc("GetClientRect")
	procClientToScreen         = user32.NewProc("ClientToScreen")
	procGetAsyncKeyState       = user32.NewProc("GetAsyncKeyState")
	procVkKeyScanW             = user32.NewProc("VkKeyScanW")
	procGetCursorInfo          = user32.NewProc("GetCursorInfo")
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
)

const (
	modNoRepeat = 0x4000
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
	wmUser      = 0x0400
	srcCopy     = 0x00CC0020
)

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
	private uint32
}

// Hotkey is a registered key: pressing it anywhere calls the function given
// to StartHotkey, on a goroutine of its own. Windows takes the key, so the
// window in front does not get it.
type Hotkey struct {
	thread uint32
	done   chan struct{}
}

// StartHotkey registers key on a thread of its own that waits for it; an
// error is a key another program has taken, or a symbol the keyboard has
// no key for.
func StartHotkey(key Key, pressed func()) (*Hotkey, error) {
	if key.Char != 0 {
		// The key typing the symbol on the keyboard's layout; one needing
		// Shift (or more) to type it is not taken, Shift being a modifier
		// of its own here.
		r, _, _ := procVkKeyScanW.Call(uintptr(key.Char))
		if int16(r) == -1 || r>>8&0xFF != 0 {
			return nil, errors.New("no key types " + string(key.Char) + " on this keyboard")
		}
		key.VK = uint32(r & 0xFF)
	}
	h := &Hotkey{done: make(chan struct{})}
	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(h.done)
		h.thread = windows.GetCurrentThreadId()
		// The thread's message queue, so Stop can post to it.
		var m msg
		procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, wmUser, wmUser, 0)
		if ok, _, err := procRegisterHotKey.Call(0, 1, uintptr(key.Mods|modNoRepeat), uintptr(key.VK)); ok == 0 {
			started <- err
			return
		}
		defer procUnregisterHotKey.Call(0, 1)
		started <- nil
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			if m.message == wmHotkey {
				go pressed()
			}
		}
	}()
	if err := <-started; err != nil {
		<-h.done
		return nil, err
	}
	return h, nil
}

// Stop unregisters the key and ends its thread.
func (h *Hotkey) Stop() {
	procPostThreadMessageW.Call(uintptr(h.thread), wmQuit, 0, 0)
	<-h.done
}

// awayPoll is how often WatchClicks looks for the game to come to the front.
const awayPoll = 500 * time.Millisecond

// The physical mouse buttons, and Shift.
const (
	vkLButton = 0x01
	vkRButton = 0x02
	vkShift   = 0x10
)

// aiming reports whether the player is aiming in the game window hwnd: the
// cursor hidden and held at the window's centre, as EFT holds it while the
// player moves and shoots. On the menus and the character screen (Tab) the
// cursor is shown, or at least moves. A click while aiming is a shot, no
// click on a screen to read.
func aiming(hwnd uintptr) bool {
	info := struct {
		size, flags uint32
		cursor      uintptr
		x, y        int32
	}{}
	info.size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := procGetCursorInfo.Call(uintptr(unsafe.Pointer(&info))); ok == 0 || info.flags&1 != 0 { // CURSOR_SHOWING
		return false
	}
	var rect struct{ left, top, right, bottom int32 }
	if ok, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return false
	}
	centre := struct{ x, y int32 }{rect.right / 2, rect.bottom / 2}
	procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&centre)))
	dx, dy := info.x-centre.x, info.y-centre.y
	return dx >= -2 && dx <= 2 && dy >= -2 && dy <= 2
}

// ClickWatcher watches the mouse buttons while the game is in front.
type ClickWatcher struct {
	stop chan struct{}
	done chan struct{}
}

// WatchClicks calls settled once a mouse button (the left or the right) has
// been let go in the game and no other click came for settle: a burst of
// clicks is one call, told how its last click was made (Click). A click
// while aiming (a shot) is not one.
// It looks at the button's state every poll (no input hook; a click is held
// 50 ms or more, so 30 ms misses none) while the window in front is the
// game's, and only for the game every awayPoll while it is not; the window's
// process is looked up only when the window changes.
func WatchClicks(poll, settle time.Duration, settled func(Click)) *ClickWatcher {
	w := &ClickWatcher{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		timer := time.NewTimer(awayPoll)
		defer timer.Stop()
		var front uintptr
		var inGame, wasLeft, wasRight bool
		var due, rightDown time.Time
		var last Click
		for {
			select {
			case <-w.stop:
				return
			case now := <-timer.C:
				if hwnd, _, _ := procGetForegroundWindow.Call(); hwnd != front {
					front, inGame = hwnd, hwnd != 0 && isGame(hwnd)
				}
				if !inGame {
					wasLeft, wasRight, due = false, false, time.Time{}
					timer.Reset(awayPoll)
					continue
				}
				timer.Reset(poll)
				pressed := func(vk uintptr) bool {
					state, _, _ := procGetAsyncKeyState.Call(vk)
					return state&0x8000 != 0
				}
				left, right := pressed(vkLButton), pressed(vkRButton)
				if right && !wasRight {
					rightDown = now
				}
				// A right button held to turn MAYAK's wheel (ForwardWheel) made no click.
				scrolled := wasRight && !right && WheelTurnedSince(rightDown)
				if ((wasLeft && !left) || (wasRight && !right)) && !aiming(front) && !scrolled {
					last, due = Click{Right: wasRight && !right, Shift: pressed(vkShift)}, now.Add(settle)
				}
				wasLeft, wasRight = left, right
				if !due.IsZero() && !left && !right && now.After(due) {
					due = time.Time{}
					go settled(last)
				}
			}
		}
	}()
	return w
}

// Stop ends the watching.
func (w *ClickWatcher) Stop() {
	close(w.stop)
	<-w.done
}

// Grab takes the inside of the game's window from the screen, when the game
// is in front: one copy of its pixels (GDI), nothing kept running.
func Grab() (*image.RGBA, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 || !isGame(hwnd) {
		return nil, ErrNotInFront
	}
	return grab(hwnd)
}

// grab takes the inside of the window hwnd from the screen.
func grab(hwnd uintptr) (*image.RGBA, error) {
	var rect struct{ left, top, right, bottom int32 }
	if ok, _, err := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return nil, err
	}
	origin := struct{ x, y int32 }{}
	procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&origin)))
	width, height := int(rect.right-rect.left), int(rect.bottom-rect.top)
	if width <= 0 || height <= 0 {
		return nil, ErrNotInFront
	}
	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return nil, errors.New("no screen device context")
	}
	defer procReleaseDC.Call(0, screen)
	memory, _, _ := procCreateCompatibleDC.Call(screen)
	if memory == 0 {
		return nil, errors.New("no memory device context")
	}
	defer procDeleteDC.Call(memory)
	bitmap, _, _ := procCreateCompatibleBitmap.Call(screen, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return nil, errors.New("no bitmap")
	}
	defer procDeleteObject.Call(bitmap)
	previous, _, _ := procSelectObject.Call(memory, bitmap)
	ok, _, err := procBitBlt.Call(memory, 0, 0, uintptr(width), uintptr(height), screen, uintptr(origin.x), uintptr(origin.y), srcCopy)
	procSelectObject.Call(memory, previous)
	if ok == 0 {
		return nil, err
	}
	// 32-bit rows from the top (a negative height): blue, green, red, unused.
	header := struct {
		size                       uint32
		width, height              int32
		planes, bitCount           uint16
		compression, sizeImage     uint32
		xPerMeter, yPerMeter       int32
		colorsUsed, colorsRequired uint32
	}{size: 40, width: int32(width), height: -int32(height), planes: 1, bitCount: 32}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if lines, _, err := procGetDIBits.Call(memory, bitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&img.Pix[0])), uintptr(unsafe.Pointer(&header)), 0); lines == 0 {
		return nil, err
	}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+2], img.Pix[i+3] = img.Pix[i+2], img.Pix[i], 0xFF
	}
	if blank(img) {
		return nil, ErrBlank
	}
	return img, nil
}

// isGame reports whether hwnd is a window of the game's process. The process
// is told by name from the process list, without opening it.
func isGame(hwnd uintptr) bool {
	var pid uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return false
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ProcessID == pid {
			return strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), gameExe)
		}
	}
	return false
}
