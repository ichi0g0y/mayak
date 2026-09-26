package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/mayak/internal/applog"
	"github.com/local/mayak/internal/config"
)

func TestThumbnailsAreCachedOnDisk(t *testing.T) {
	data := t.TempDir()
	t.Setenv("APPDATA", data)
	t.Setenv("XDG_CONFIG_HOME", data)
	t.Setenv("HOME", data)
	shots := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 7), 90, 255})
		}
	}
	file, err := os.Create(filepath.Join(shots, "shot.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	file.Close()
	a := &App{logs: applog.New(10), settings: config.Settings{ScreenshotDirectory: shots}}
	first, err := a.BrowserScreenshotImage("shot.png", true)
	if err != nil || !strings.HasPrefix(first, "data:image/jpeg;base64,") {
		t.Fatalf("thumbnail: %.40q %v", first, err)
	}
	dir, _ := thumbnailDir()
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 || filepath.Ext(files[0].Name()) != ".jpg" {
		t.Fatalf("thumbnail files: %v %v", files, err)
	}
	// With the memory cache gone, the file serves the same thumbnail.
	thumbs.mu.Lock()
	thumbs.cache = map[string]string{}
	thumbs.order = nil
	thumbs.mu.Unlock()
	again, err := a.BrowserScreenshotImage("shot.png", true)
	if err != nil || again != first {
		t.Fatalf("from disk: same=%v err=%v", again == first, err)
	}
	// The full image is never cached.
	full, err := a.BrowserScreenshotImage("shot.png", false)
	if err != nil || !strings.HasPrefix(full, "data:image/jpeg;base64,") || full == first {
		t.Fatalf("full image: %.40q %v", full, err)
	}
	if files, _ = os.ReadDir(dir); len(files) != 1 {
		t.Fatalf("thumbnail files after the full image: %d", len(files))
	}
	// The folder is trimmed to the newest files.
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(dir, strings.Repeat("a", 5)+string(rune('0'+i))+".jpg"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	trimThumbnails(dir, 3)
	if files, _ = os.ReadDir(dir); len(files) != 3 {
		t.Fatalf("trimmed to %d files", len(files))
	}
}
