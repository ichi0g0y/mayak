package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeSettingsKeepsLegacyAutoMonitoringDefault(t *testing.T) {
	s, err := decodeSettings([]byte(`{"language":"en"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !s.AutoStartMonitoring {
		t.Fatal("legacy settings should keep automatic monitoring enabled")
	}
	if !s.OpenMapOnRaidStart {
		t.Fatal("legacy settings should open the detected map when a raid starts")
	}
	if !s.NavigateMapOnShot {
		t.Fatal("legacy settings should preserve position screenshot map navigation")
	}
	if s.RunThroughSeconds != 430 {
		t.Fatalf("legacy settings should default to 430 seconds, got %d", s.RunThroughSeconds)
	}
	if s.GameMode != "auto" {
		t.Fatalf("legacy settings should default to auto game mode, got %q", s.GameMode)
	}
	if s.ScreenshotCleanup || s.SaveRecognitionDebug {
		t.Fatal("destructive cleanup and debug archiving must remain opt-in")
	}
	if s.ScreenshotRetainCount != 500 || s.ScreenshotRetainHours != 168 {
		t.Fatalf("unexpected retention defaults: count=%d hours=%d", s.ScreenshotRetainCount, s.ScreenshotRetainHours)
	}
}

func TestDecodeSettingsHonorsDisabledAutoMonitoring(t *testing.T) {
	s, err := decodeSettings([]byte(`{"autoStartMonitoring":false,"openMapOnRaidStart":false,"navigateMapOnPositionScreenshot":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.AutoStartMonitoring {
		t.Fatal("explicitly disabled automatic monitoring should remain disabled")
	}
	if s.OpenMapOnRaidStart {
		t.Fatal("explicitly disabled raid-start map navigation should remain disabled")
	}
	if s.NavigateMapOnShot {
		t.Fatal("explicitly disabled screenshot map navigation should remain disabled")
	}
}

func TestWriteFileAtomicReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("got %q", got)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".settings.json-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files remain: %v, err=%v", matches, err)
	}
}

func TestLoadFileRecoversFromValidBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".bak", []byte(`{"language":"en","remoteId":"ABCD"}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := loadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "en" || got.RemoteID != "ABCD" {
		t.Fatalf("backup was not restored: %+v", got)
	}
}

func TestOlderSettingsHaveNoOCRDefaultRevision(t *testing.T) {
	// Settings saved before the Tesseract default must still be migrated once.
	s, err := decodeSettings([]byte(`{"ocrEngine":"windows"}`))
	if err != nil || s.OCREngine != "windows" || s.OCRDefaultRevision != 0 {
		t.Fatalf("decoded %+v, %v", s, err)
	}
}

func TestCloseToTrayFollowsOlderMinimizeSetting(t *testing.T) {
	// Before closeToTray existed, minimizeToTray also covered closing.
	old, err := decodeSettings([]byte(`{"minimizeToTray":true}`))
	if err != nil || !old.MinimizeToTray || !old.CloseToTray {
		t.Fatalf("old settings = %+v, %v", old, err)
	}
	split, err := decodeSettings([]byte(`{"minimizeToTray":true,"closeToTray":false}`))
	if err != nil || !split.MinimizeToTray || split.CloseToTray {
		t.Fatalf("split settings = %+v, %v", split, err)
	}
}
