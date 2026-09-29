package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeedTakesPreferencesNotTheSamePC(t *testing.T) {
	base := t.TempDir()
	from, to := filepath.Join(base, "Mayak"), filepath.Join(base, DevName)
	write := func(name, text string) {
		p := filepath.Join(from, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o700)
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.json", "s")
	write("sounds/a.wav", "w")
	write("browser.json", "pairing")
	write("squad-key.txt", "key")
	if seeded, err := seed(from, to); err != nil || !seeded {
		t.Fatalf("seed = %v, %v", seeded, err)
	}
	for name, want := range map[string]bool{"settings.json": true, "sounds/a.wav": true, "browser.json": false, "squad-key.txt": false} {
		_, err := os.Stat(filepath.Join(to, name))
		if (err == nil) != want {
			t.Errorf("%s copied = %v, want %v", name, err == nil, want)
		}
	}
	// Seeded once: a folder that exists is left alone.
	write("settings.json", "changed")
	if seeded, _ := seed(from, to); seeded {
		t.Fatal("seeded again")
	}
	if b, _ := os.ReadFile(filepath.Join(to, "settings.json")); string(b) != "s" {
		t.Fatalf("settings.json = %q", b)
	}
}
