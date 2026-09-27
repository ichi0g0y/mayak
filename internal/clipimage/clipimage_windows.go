//go:build windows

package clipimage

import (
	"errors"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard          = user32.NewProc("OpenClipboard")
	procCloseClipboard         = user32.NewProc("CloseClipboard")
	procEmptyClipboard         = user32.NewProc("EmptyClipboard")
	procSetClipboardData       = user32.NewProc("SetClipboardData")
	procRegisterClipboardFormW = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc            = kernel32.NewProc("GlobalAlloc")
	procGlobalFree             = kernel32.NewProc("GlobalFree")
	procGlobalLock             = kernel32.NewProc("GlobalLock")
	procGlobalUnlock           = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory          = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfDIB       = 8
	gmemMovable = 0x0002
)

// CopyPNG puts the PNG data on the clipboard.
func CopyPNG(data []byte) error {
	img, err := decode(data)
	if err != nil {
		return err
	}
	bitmap := dib(img)
	// The clipboard belongs to a thread while it is open.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	opened := false
	for range 10 {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
			break
		}
		// Another app has it open for a moment.
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		return errors.New("the clipboard is in use")
	}
	defer procCloseClipboard.Call()
	if r, _, err := procEmptyClipboard.Call(); r == 0 {
		return err
	}
	if err := set(cfDIB, bitmap); err != nil {
		return err
	}
	name, _ := syscall.UTF16PtrFromString("PNG")
	if format, _, _ := procRegisterClipboardFormW.Call(uintptr(unsafe.Pointer(name))); format != 0 {
		// The bitmap is there already; PNG is extra.
		_ = set(format, data)
	}
	return nil
}

// set puts data on the open clipboard as format; the clipboard owns the
// memory once it takes it.
func set(format uintptr, data []byte) error {
	handle, _, err := procGlobalAlloc.Call(gmemMovable, uintptr(len(data)))
	if handle == 0 {
		return err
	}
	ptr, _, err := procGlobalLock.Call(handle)
	if ptr == 0 {
		procGlobalFree.Call(handle)
		return err
	}
	// The copy is made by Windows: the locked memory is only an address here.
	procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	procGlobalUnlock.Call(handle)
	if r, _, err := procSetClipboardData.Call(format, handle); r == 0 {
		procGlobalFree.Call(handle)
		return err
	}
	return nil
}
