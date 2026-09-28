package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Settings saved before the split (all in settings.json) load as they were,
// and the next save moves the preferences to preferences.json, leaving this
// PC's settings in settings.json.
func TestSettingsSplitIntoDeviceAndPreferences(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	old := `{"language":"en","soundVolume":40,"screenshotDirectory":"C:/shots","browserRemoteId":"ABC"}`
	if err := os.WriteFile(p, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := loadFile(p)
	if err != nil || s.Language != "en" || s.SoundVolume != 40 || s.ScreenshotDirectory != "C:/shots" {
		t.Fatalf("old settings %+v, %v", s, err)
	}
	if err := saveFile(p, s, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var device map[string]any
	b, _ := os.ReadFile(p)
	_ = json.Unmarshal(b, &device)
	if device["screenshotDirectory"] != "C:/shots" || device["browserRemoteId"] != "ABC" || device["language"] != nil || device["soundVolume"] != nil {
		t.Fatalf("settings.json %v", device)
	}
	var prefs struct {
		Values    map[string]any       `json:"values"`
		UpdatedAt map[string]time.Time `json:"updatedAt"`
	}
	b, _ = os.ReadFile(filepath.Join(dir, "preferences.json"))
	_ = json.Unmarshal(b, &prefs)
	if prefs.Values["language"] != "en" || prefs.Values["soundVolume"] != float64(40) || prefs.Values["screenshotDirectory"] != nil || prefs.UpdatedAt["language"].IsZero() {
		t.Fatalf("preferences.json %+v", prefs)
	}
	back, err := loadFile(p)
	if err != nil || back.Language != "en" || back.SoundVolume != 40 || back.ScreenshotDirectory != "C:/shots" || back.BrowserRemoteID != "ABC" {
		t.Fatalf("reloaded %+v, %v", back, err)
	}
}

// A preference's time moves only when its value changes.
func TestPreferenceTimesMoveOnlyOnChange(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	first := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	s := defaults()
	if err := saveFile(p, s, first); err != nil {
		t.Fatal(err)
	}
	s.SoundVolume = 55
	if err := saveFile(p, s, first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var prefs struct {
		UpdatedAt map[string]time.Time `json:"updatedAt"`
	}
	b, _ := os.ReadFile(filepath.Join(dir, "preferences.json"))
	_ = json.Unmarshal(b, &prefs)
	if !prefs.UpdatedAt["soundVolume"].Equal(first.Add(time.Hour)) || !prefs.UpdatedAt["language"].Equal(first) {
		t.Fatalf("times %v", prefs.UpdatedAt)
	}
}
