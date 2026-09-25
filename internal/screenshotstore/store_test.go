package screenshotstore

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveDebugAndCleanupExcludeDebugDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "shot.png")
	if err := os.WriteFile(source, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	meta, err := SaveDebug(root, source, Metadata{CapturedAt: time.Now(), ScreenshotType: "tasks", DetectionScore: .75}, "data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("crop")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(meta); err != nil {
		t.Fatal(err)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, DebugDirectory, "*_source.png")); len(matches) != 1 {
		t.Fatalf("source copies = %d", len(matches))
	}
	if matches, _ := filepath.Glob(filepath.Join(root, DebugDirectory, "*_crop.png")); len(matches) != 1 {
		t.Fatalf("crop copies = %d", len(matches))
	}
	old := filepath.Join(root, "old.jpg")
	if err = os.WriteFile(old, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err = os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	deleted, err := Cleanup(root, CleanupOptions{MaxAge: 24 * time.Hour, Protect: source})
	if err != nil || len(deleted) != 1 || deleted[0] != old {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if _, err = os.Stat(filepath.Dir(meta)); err != nil {
		t.Fatal("debug directory was deleted")
	}
}

func TestCleanupAppliesCountAndProtectsCurrent(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for i := 0; i < 4; i++ {
		path := filepath.Join(root, string(rune('a'+i))+".png")
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	deleted, err := Cleanup(root, CleanupOptions{MaxCount: 2, Protect: paths[0]})
	if err != nil || len(deleted) != 1 || deleted[0] != paths[1] {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if _, err = os.Stat(paths[0]); err != nil {
		t.Fatal("protected screenshot was deleted")
	}
}
