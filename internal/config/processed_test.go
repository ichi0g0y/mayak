package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScreenshotFingerprintMatchesSameFileVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := FingerprintScreenshot(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FingerprintScreenshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Matches(second) {
		t.Fatalf("same file version did not match: %+v %+v", first, second)
	}
	if err = os.WriteFile(path, []byte("different size"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := FingerprintScreenshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Matches(changed) {
		t.Fatal("changed file version matched the saved fingerprint")
	}
}
