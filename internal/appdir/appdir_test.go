package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrate(t *testing.T) {
	base := t.TempDir()
	// Nothing to move.
	if moved, err := migrate(base); moved || err != nil {
		t.Fatalf("moved = %v, err = %v", moved, err)
	}
	old := filepath.Join(base, legacyName)
	if err := os.MkdirAll(old, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "settings.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, legacyLog), []byte("log"), 0600); err != nil {
		t.Fatal(err)
	}
	moved, err := migrate(base)
	if !moved || err != nil {
		t.Fatalf("moved = %v, err = %v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(base, Name, "settings.json")); err != nil {
		t.Fatal("settings were not moved:", err)
	}
	if _, err := os.Stat(filepath.Join(base, Name, logName)); err != nil {
		t.Fatal("the log was not renamed:", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("the old folder remains")
	}
	// With both folders present nothing moves.
	if err := os.MkdirAll(old, 0700); err != nil {
		t.Fatal(err)
	}
	if moved, err := migrate(base); moved || err != nil {
		t.Fatalf("moved = %v, err = %v", moved, err)
	}
}

// Path joins its parts under the app's data folder, as Config does.
func TestPathIsUnderTheDataFolder(t *testing.T) {
	path, err := Path("snapnotes", "note.json")
	if err != nil {
		t.Skip("no user configuration directory here")
	}
	if want := filepath.Join(Config(), "snapnotes", "note.json"); path != want {
		t.Fatalf("Path = %q, want %q", path, want)
	}
}
