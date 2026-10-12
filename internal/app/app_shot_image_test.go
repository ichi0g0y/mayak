package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeShot(t *testing.T, path string, shade uint8) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{shade, shade, shade, 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestScreenshotImageDecodedOnce(t *testing.T) {
	a := &App{}
	dir := t.TempDir()
	first := filepath.Join(dir, "a.png")
	writeShot(t, first, 10)
	img, err := a.screenshotImage(first)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := a.screenshotImage(first)
	if again != img {
		t.Fatal("the same file was decoded again")
	}
	// A file written again is decoded again.
	writeShot(t, first, 200)
	later := time.Now().Add(time.Second)
	_ = os.Chtimes(first, later, later)
	if rewritten, _ := a.screenshotImage(first); rewritten == img {
		t.Fatal("a rewritten file came from the cache")
	}
	// A picture in hand (a capture) is not decoded at all.
	second := filepath.Join(dir, "b.png")
	writeShot(t, second, 50)
	info, _ := os.Stat(second)
	held := image.NewRGBA(image.Rect(0, 0, 8, 8))
	a.rememberScreenshotImage(second, info, held)
	if got, _ := a.screenshotImage(second); got != held {
		t.Fatal("the picture in hand was not used")
	}
	// Only the newest shotImageKeep stay.
	third := filepath.Join(dir, "c.png")
	writeShot(t, third, 90)
	_, _ = a.screenshotImage(third)
	if len(a.shotImages.entries) != shotImageKeep || a.shotImages.entries[0].path != second {
		t.Fatalf("entries = %+v", a.shotImages.entries)
	}
	if _, err := a.screenshotImage(filepath.Join(dir, "gone.png")); err == nil {
		t.Fatal("a missing file decoded")
	}
}
