// Package capture takes MAYAK's own screenshots of EFT, an experimental
// alternative to EFT's screenshot key, whose notice in the game cannot be
// turned off. A global hotkey (RegisterHotKey: no keyboard hook, nothing
// runs until it is pressed) or a click in the game (WatchClicks, like
// RatScanner's automatic scan) grabs the game's window once (Grab); Save
// writes it to the screenshot folder, where the folder's watcher reads it
// like one of EFT's. Such a shot has no position: EFT writes that in the
// names of its own.
package capture

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	// ErrNotInFront is a key pressed while the game's window was not in front.
	ErrNotInFront = errors.New("Escape from Tarkov is not in front")
	// ErrBlank is a grab that came out black: a game in exclusive full screen
	// cannot be read from the screen (borderless and windowed can).
	ErrBlank = errors.New("the capture is black (exclusive full screen?)")
	// ErrUnsupported is a platform without the capture.
	ErrUnsupported = errors.New("capture is not supported on this platform")
)

// Key is a hotkey: Windows' modifier flags (MOD_ALT 1, MOD_CONTROL 2,
// MOD_SHIFT 4, MOD_WIN 8) and virtual-key code, or for a symbol (";", "@")
// the character instead: which key types it depends on the keyboard's
// layout (";" is a key of its own on a Japanese one, Shift and another on a
// US one), so StartHotkey asks Windows when it registers the key.
type Key struct {
	Mods uint32
	VK   uint32
	Char rune
}

// modifiers in the order a key is written, like EFT's screenshot key
// (eftdetect.ScreenshotKeys): "Ctrl + Shift + F10".
var modifiers = []struct {
	name string
	flag uint32
}{{"Ctrl", 2}, {"Shift", 4}, {"Alt", 1}, {"Win", 8}}

// namedKeys are the keys besides F1–F24, A–Z, 0–9 and "Num 0"–"Num 9".
var namedKeys = map[string]uint32{
	"Insert": 0x2D, "Delete": 0x2E, "Home": 0x24, "End": 0x23, "PageUp": 0x21, "PageDown": 0x22,
	"Pause": 0x13, "ScrollLock": 0x91, "PrintScreen": 0x2C,
}

// ParseKey reads a key written as Format writes it (modifiers first, " + "
// between); case and spaces around the "+" do not matter.
func ParseKey(s string) (Key, error) {
	parts := strings.Split(s, "+")
	var key Key
	for i, part := range parts {
		part = strings.TrimSpace(part)
		last := i == len(parts)-1
		found := false
		for _, m := range modifiers {
			if strings.EqualFold(part, m.name) && !last {
				key.Mods |= m.flag
				found = true
			}
		}
		if found {
			continue
		}
		if !last {
			return Key{}, fmt.Errorf("unknown modifier %q", part)
		}
		if r := []rune(part); len(r) == 1 && symbol(r[0]) {
			key.Char = r[0]
			continue
		}
		vk, ok := virtualKey(part)
		if !ok {
			return Key{}, fmt.Errorf("unknown key %q", part)
		}
		key.VK = vk
	}
	if key.VK == 0 && key.Char == 0 {
		return Key{}, errors.New("no key")
	}
	return key, nil
}

// symbol tells a character that names a key by itself: printable, neither a
// letter nor a digit (those are keys by name), nor "+" (it joins the parts).
func symbol(r rune) bool {
	return unicode.IsPrint(r) && !unicode.IsSpace(r) && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '+'
}

func virtualKey(name string) (uint32, bool) {
	upper := strings.ToUpper(name)
	for named, vk := range namedKeys {
		if strings.EqualFold(named, name) {
			return vk, true
		}
	}
	switch {
	case len(upper) == 1 && upper[0] >= 'A' && upper[0] <= 'Z', len(upper) == 1 && upper[0] >= '0' && upper[0] <= '9':
		return uint32(upper[0]), true
	case strings.HasPrefix(upper, "NUM") && len(strings.TrimSpace(upper[3:])) == 1:
		if d := strings.TrimSpace(upper[3:])[0]; d >= '0' && d <= '9' {
			return 0x60 + uint32(d-'0'), true
		}
	case strings.HasPrefix(upper, "F"):
		if n, err := strconv.Atoi(upper[1:]); err == nil && n >= 1 && n <= 24 {
			return 0x70 + uint32(n-1), true
		}
	}
	return 0, false
}

// Format writes key as ParseKey reads it: "Ctrl + F10".
func Format(key Key) string {
	var parts []string
	for _, m := range modifiers {
		if key.Mods&m.flag != 0 {
			parts = append(parts, m.name)
		}
	}
	name := keyName(key.VK)
	if key.Char != 0 {
		name = string(key.Char)
	}
	return strings.Join(append(parts, name), " + ")
}

func keyName(vk uint32) string {
	for name, code := range namedKeys {
		if code == vk {
			return name
		}
	}
	switch {
	case vk >= 'A' && vk <= 'Z', vk >= '0' && vk <= '9':
		return string(rune(vk))
	case vk >= 0x60 && vk <= 0x69:
		return "Num " + string(rune('0'+vk-0x60))
	case vk >= 0x70 && vk <= 0x87:
		return "F" + strconv.Itoa(int(vk-0x70+1))
	}
	return ""
}

// Normalize writes a key the way Format does, "" for none or one ParseKey
// cannot read.
func Normalize(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	key, err := ParseKey(s)
	if err != nil {
		return ""
	}
	return Format(key)
}

// FileName is a capture's name at t: EFT's date and time first, like its own
// screenshots (2026-10-12[14-50]), and no position after them.
func FileName(t time.Time) string {
	return t.Format("2006-01-02[15-04]") + "_mayak-" + t.Format("150405") + ".png"
}

// Save writes img to dir as a PNG named for t, quickly compressed, under a
// temporary name first so the folder's watcher sees it whole. It returns the
// file's path.
func Save(dir string, img image.Image, t time.Time) (string, error) {
	path := filepath.Join(dir, FileName(t))
	if _, err := os.Stat(path); err == nil {
		path = strings.TrimSuffix(path, ".png") + "-" + strconv.Itoa(t.Nanosecond()/1e6) + ".png"
	}
	part := path + ".part"
	file, err := os.Create(part)
	if err != nil {
		return "", err
	}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err = encoder.Encode(file, img); err != nil {
		_ = file.Close()
		_ = os.Remove(part)
		return "", err
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	if err = os.Rename(part, path); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	return path, nil
}

// Click is how a burst of clicks ended (WatchClicks): its last click was
// made with the right button, or with Shift held.
type Click struct{ Right, Shift bool }

// signatureSize is the thumbnail Signature makes: wide, as the crops it is
// for are lines of text (a task's or an item's name), and fine enough that a
// letter's strokes show (a cell is about 6 px of a 720 px wide name).
var signatureSize = image.Rect(0, 0, 128, 16)

// Signature is a small grey thumbnail of img, each pixel the average of its
// area, for telling whether two crops show the same thing (Same).
func Signature(img image.Image) []byte {
	b := img.Bounds()
	if b.Empty() {
		return nil
	}
	gray := image.NewGray(b)
	draw.Draw(gray, b, img, b.Min, draw.Src)
	w, h := signatureSize.Dx(), signatureSize.Dy()
	out := make([]byte, w*h)
	for y := range h {
		for x := range w {
			x0, x1 := b.Min.X+x*b.Dx()/w, b.Min.X+(x+1)*b.Dx()/w
			y0, y1 := b.Min.Y+y*b.Dy()/h, b.Min.Y+(y+1)*b.Dy()/h
			sum, n := 0, 0
			for yy := y0; yy < max(y1, y0+1); yy++ {
				for xx := x0; xx < max(x1, x0+1); xx++ {
					sum += int(gray.GrayAt(xx, yy).Y)
					n++
				}
			}
			out[y*w+x] = byte(sum / n)
		}
	}
	return out
}

// Same reports whether two signatures show the same text: after taking
// out each one's average (a hover lights the whole line), hardly any cell
// differs by more than a little. Text takes a small part of a line, so a
// different name changes few cells, but those by much.
func Same(a, b []byte) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	// Different names differed in 24 cells or more (of 2048) on EFT's task
	// and item names, the same one in 0 to 2.
	return differingCells(a, b) <= len(a)/256
}

// differingCells counts the cells of two signatures that differ by more than
// 24 grey levels, each taken from its own average.
func differingCells(a, b []byte) int {
	meanA, meanB := 0, 0
	for i := range a {
		meanA += int(a[i])
		meanB += int(b[i])
	}
	meanA, meanB = meanA/len(a), meanB/len(b)
	count := 0
	for i := range a {
		d := (int(a[i]) - meanA) - (int(b[i]) - meanB)
		if d > 24 || d < -24 {
			count++
		}
	}
	return count
}

// blank reports whether an RGBA image is (nearly) all black, looking at a
// grid of its pixels.
func blank(img *image.RGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += max(1, b.Dy()/48) {
		for x := b.Min.X; x < b.Max.X; x += max(1, b.Dx()/64) {
			i := img.PixOffset(x, y)
			if img.Pix[i] > 8 || img.Pix[i+1] > 8 || img.Pix[i+2] > 8 {
				return false
			}
		}
	}
	return true
}

// TabButtons are what can be pressed to close the tab shown or pin it
// (WheelOptions.CloseTab, PinTab): the middle button, alone or with Shift,
// Ctrl or Alt, or a side button.
var TabButtons = []string{"middle", "shift+middle", "ctrl+middle", "alt+middle", "x1", "x2"}

// WheelCombos are what can be held to scroll MAYAK or switch its tabs with
// the wheel from EFT (ForwardWheel): the right button, the side buttons
// (back, forward), Shift, Ctrl or Alt.
var WheelCombos = []string{"right", "x1", "x2", "shift", "ctrl", "alt"}

// WheelOptions are what ForwardWheel does with the wheel turned in EFT.
type WheelOptions struct {
	// Scroll and Tabs are the combos (WheelCombos, "" for none) held to
	// scroll MAYAK and to switch its tabs.
	Scroll, Tabs string
	// SwitchTab is called with 1 for a notch turned towards the user, -1 away.
	SwitchTab func(step int)
	// CloseTab and PinTab are the buttons (TabButtons, "" for none) that
	// close the tab shown and pin or unpin it; then TabAction is called with
	// "close" or "pin".
	CloseTab, PinTab string
	TabAction        func(action string)
	// ScrollPage turns the wheel of the page in MAYAK's window hwnd (a
	// WebView's) at (x, y), CSS pixels, by dy (down for more than 0) with no
	// key held: for a key held to scroll, Windows' wheel message would let the
	// page read it as Shift's sideways scroll or Ctrl's zoom. A button's goes
	// as the message.
	ScrollPage func(hwnd uintptr, x, y, dy float64)
	// Point is where in MAYAK a scroll goes, in screen pixels: the middle of
	// the page shown (not the sidebar nor the item panel).
	Point func() (x, y int32, ok bool)
	// Note tells what a turn with a combo held did, at most once a second.
	Note func(text string)
}
