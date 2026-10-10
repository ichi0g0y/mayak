package eftdetect

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestGameLanguage(t *testing.T) {
	for input, want := range map[string]string{
		`{"Language": "en", "Other": 1}`: "en",
		`{"Language": "jp"}`:             "ja",
		`{"Language": "fr"}`:             "en",
		`{"Language": "ru"}`:             "",
		`{}`:                             "",
		`not json`:                       "",
	} {
		if got := gameLanguage([]byte(input)); got != want {
			t.Errorf("%s: got %q, want %q", input, got, want)
		}
	}
}

// EFT's screenshots are looked for under Windows' Documents folder first,
// then under the home's Documents and OneDrive's; never in the game's folder.
func TestScreenshotPlaces(t *testing.T) {
	got := screenshotPlaces(filepath.Join("D:", "Docs"), filepath.Join("C:", "Users", "u"))
	want := []string{
		filepath.Join("D:", "Docs", "Escape from Tarkov", "Screenshots"),
		filepath.Join("C:", "Users", "u", "Documents", "Escape from Tarkov", "Screenshots"),
		filepath.Join("C:", "Users", "u", "OneDrive", "Documents", "Escape from Tarkov", "Screenshots"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
	if got := screenshotPlaces("", ""); len(got) != 0 {
		t.Fatalf("no places: %v", got)
	}
}

// EFT's screenshot key as Control.ini has it (the user's, 2026-10-10), an
// unbound one, and a mouse button.
func TestScreenshotKeys(t *testing.T) {
	user := `{"keyBindings":[{"keyName":"Jump","variants":[{"keyCode":["Space"]}]},{"keyName":"MakeScreenshot","variants":[{"keyCode":["Semicolon","LeftControl"]},{"keyCode":[]}],"pressType":"Press"}]}`
	if keys, known := screenshotKeys([]byte(user)); !known || !slices.Equal(keys, []string{"Ctrl + ;"}) {
		t.Fatalf("user's: %v %v", keys, known)
	}
	unbound := `{"keyBindings":[{"keyName":"MakeScreenshot","variants":[{"keyCode":[]},{"keyCode":[]}]}]}`
	if keys, known := screenshotKeys([]byte(unbound)); !known || len(keys) != 0 {
		t.Fatalf("unbound: %v %v", keys, known)
	}
	mouse := `{"keyBindings":[{"keyName":"MakeScreenshot","variants":[{"keyCode":["Mouse3"]},{"keyCode":["Print"]}]}]}`
	if keys, _ := screenshotKeys([]byte(mouse)); !slices.Equal(keys, []string{"Mouse4", "PrintScreen"}) {
		t.Fatalf("mouse: %v", keys)
	}
	if _, known := screenshotKeys([]byte(`{}`)); known {
		t.Fatal("no binding counted as known")
	}
}
