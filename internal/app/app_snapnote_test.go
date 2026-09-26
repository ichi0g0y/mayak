package app

import (
	"testing"

	"github.com/local/mayak/internal/config"
)

// A snap note from a screenshot takes only an image file directly in the
// Screenshots folder.
func TestSnapNoteFromScreenshotRejectsOtherFiles(t *testing.T) {
	a := &App{}
	if _, err := a.SnapNoteFromScreenshot("shot.png", ""); err == nil {
		t.Fatal("accepted a screenshot with no Screenshots folder set")
	}
	a.settings = config.Settings{ScreenshotDirectory: t.TempDir()}
	for _, name := range []string{`..\settings.json`, "../shot.png", `sub\shot.png`, "notes.txt", ".hidden.png", ""} {
		if _, err := a.SnapNoteFromScreenshot(name, ""); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if _, err := a.SnapNoteFromScreenshot("missing.png", ""); err == nil {
		t.Fatal("accepted a screenshot that is not there")
	}
}
