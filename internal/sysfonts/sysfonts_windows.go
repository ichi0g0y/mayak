//go:build windows

package sysfonts

import (
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procEnumFontFamiliesEx = gdi32.NewProc("EnumFontFamiliesExW")
)

// logFont is LOGFONTW, which the enumeration's first argument starts with.
type logFont struct {
	Height, Width, Escapement, Orientation, Weight int32
	Italic, Underline, StrikeOut, CharSet          byte
	OutPrecision, ClipPrecision, Quality, Pitch    byte
	FaceName                                       [32]uint16
}

const defaultCharset = 1

var (
	mu       sync.Mutex
	found    []string
	callback = syscall.NewCallback(func(font *logFont, _ uintptr, _ uint32, _ uintptr) uintptr {
		found = append(found, syscall.UTF16ToString(font.FaceName[:]))
		return 1
	})
)

// List returns the installed font families, sorted.
func List() []string {
	mu.Lock()
	defer mu.Unlock()
	found = nil
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return nil
	}
	defer procReleaseDC.Call(0, dc)
	// Every family, in every character set: a blank name and DEFAULT_CHARSET.
	query := logFont{CharSet: defaultCharset}
	procEnumFontFamiliesEx.Call(dc, uintptr(unsafe.Pointer(&query)), callback, 0, 0)
	return clean(found)
}
