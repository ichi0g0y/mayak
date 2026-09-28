package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/mayak/internal/config"
)

// A sound file chosen by its path is taken into sounds/ once and named from
// there; the name resolves to the copy, a path to itself, anything else to
// nothing.
func TestSoundFilesAreTakenIntoTheDataFolder(t *testing.T) {
	// Not t.TempDir: other tests' background saves (they write to the data
	// folder, which this one moves) can land in it while it is removed.
	data, err := os.MkdirTemp("", "mayak-sounds-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(data) })
	t.Setenv("APPDATA", data)
	t.Setenv("XDG_CONFIG_HOME", data)
	t.Setenv("HOME", data)
	src := filepath.Join(t.TempDir(), `my "alert".wav`)
	src = strings.ReplaceAll(src, `"`, "")
	if err := os.WriteFile(src, []byte("RIFF....WAVE"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := config.Settings{QuestSoundPath: src, ErrorSoundPath: filepath.Join(data, "missing.wav")}
	if !importSoundFiles(&s) || !soundRef.MatchString(s.QuestSoundPath) || !strings.HasSuffix(s.QuestSoundPath, "-my alert.wav") {
		t.Fatalf("taken in as %q", s.QuestSoundPath)
	}
	if s.ErrorSoundPath != filepath.Join(data, "missing.wav") {
		t.Fatalf("a missing file lost its path: %q", s.ErrorSoundPath)
	}
	copy := soundFilePath(s.QuestSoundPath)
	if b, err := os.ReadFile(copy); err != nil || string(b) != "RIFF....WAVE" {
		t.Fatalf("copy %q: %q %v", copy, b, err)
	}
	again, err := importSoundFile(src)
	if err != nil || again != s.QuestSoundPath {
		t.Fatalf("the same file twice: %q %v", again, err)
	}
	for _, bad := range []string{"sounds/../settings.json", "sounds/x.wav", "relative/x.wav"} {
		if soundFilePath(bad) != "" || normalizeSoundPath(bad) != "" {
			t.Fatalf("accepted %q", bad)
		}
	}
}
