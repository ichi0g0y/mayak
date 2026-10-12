package capture

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/local/mayak/internal/position"
)

func TestParseAndFormatKey(t *testing.T) {
	for _, c := range []struct {
		in   string
		key  Key
		text string
	}{
		{"F10", Key{VK: 0x79}, "F10"},
		{"ctrl+shift+f10", Key{Mods: 2 | 4, VK: 0x79}, "Ctrl + Shift + F10"},
		{"Alt + K", Key{Mods: 1, VK: 'K'}, "Alt + K"},
		{"Win + 5", Key{Mods: 8, VK: '5'}, "Win + 5"},
		{"Num 7", Key{VK: 0x67}, "Num 7"},
		{"Shift + PageDown", Key{Mods: 4, VK: 0x22}, "Shift + PageDown"},
		{"F24", Key{VK: 0x87}, "F24"},
		// A symbol is kept as the character (StartHotkey finds its key).
		{"Alt + ;", Key{Mods: 1, Char: ';'}, "Alt + ;"},
		{"ctrl+@", Key{Mods: 2, Char: '@'}, "Ctrl + @"},
		{"`", Key{Char: '`'}, "`"},
	} {
		key, err := ParseKey(c.in)
		if err != nil || key != c.key || Format(key) != c.text {
			t.Errorf("ParseKey(%q) = %+v %v, Format %q", c.in, key, err, Format(key))
		}
	}
	for _, bad := range []string{"", "Ctrl", "Ctrl + ", "F25", "Hyper + K", "K + Ctrl", "Enter", "Alt + ;;"} {
		if _, err := ParseKey(bad); err == nil {
			t.Errorf("ParseKey(%q) took a bad key", bad)
		}
	}
	if Normalize(" shift+f9 ") != "Shift + F9" || Normalize("nonsense") != "" || Normalize("") != "" {
		t.Fatal("Normalize")
	}
}

func TestSave(t *testing.T) {
	at := time.Date(2026, 10, 12, 14, 50, 7, 0, time.Local)
	if name := FileName(at); name != "2026-10-12[14-50]_mayak-145007.png" {
		t.Fatalf("FileName = %q", name)
	}
	// A capture has no position: the name must not read as one of EFT's.
	if _, err := position.ParseFilename(FileName(at)); err == nil {
		t.Fatal("a capture's name reads as a position")
	}
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(1, 1, color.RGBA{200, 10, 10, 255})
	first, err := Save(dir, img, at)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Save(dir, img, at.Add(time.Millisecond*5))
	if err != nil || second == first {
		t.Fatalf("second save = %q %v", second, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 || filepath.Ext(entries[0].Name()) != ".png" || filepath.Ext(entries[1].Name()) != ".png" {
		t.Fatalf("files = %v", entries)
	}
}

func TestBlank(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	if !blank(img) {
		t.Fatal("a black image is not blank")
	}
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{30, 30, 30, 255})
		}
	}
	if blank(img) {
		t.Fatal("a dark grey image is blank")
	}
}

// line draws a grey line of text as bars, one per letter given as a width.
func line(shift uint8, bars ...int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 720, 96))
	for y := 0; y < 96; y++ {
		for x := 0; x < 720; x++ {
			img.Set(x, y, color.RGBA{130 + shift, 130 + shift, 120 + shift, 255})
		}
	}
	x := 60
	for _, w := range bars {
		for y := 38; y < 58; y++ {
			for i := 0; i < w; i++ {
				img.Set(x+i, y, color.RGBA{20, 20, 20, 255})
			}
		}
		x += w + 6
	}
	return img
}

func TestSame(t *testing.T) {
	name := line(0, 3, 9, 3, 12, 3, 3, 9)
	if !Same(Signature(name), Signature(line(0, 3, 9, 3, 12, 3, 3, 9))) {
		t.Fatal("the same name is not the same")
	}
	// A hover lights the whole line.
	if !Same(Signature(name), Signature(line(12, 3, 9, 3, 12, 3, 3, 9))) {
		t.Fatal("a lit line is not the same")
	}
	if Same(Signature(name), Signature(line(0, 3, 9, 3, 3, 12, 9, 3))) {
		t.Fatal("another name is the same")
	}
	if Same(nil, Signature(name)) || Same(Signature(name), Signature(image.NewRGBA(image.Rect(0, 0, 10, 2)))[:3]) {
		t.Fatal("a missing signature is the same")
	}
}
