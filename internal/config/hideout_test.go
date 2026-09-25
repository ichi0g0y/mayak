package config

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestHideoutSettingsSurviveReload(t *testing.T) {
	s := defaults()
	s.HideoutErrorNotifications = true
	s.HideoutErrorSoundPath = `C:\Sounds\hideout.wav`
	data, _ := json.Marshal(s)
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := writeFileAtomic(path, data); err != nil {
		t.Fatal(err)
	}
	got, err := loadFile(path)
	if err != nil || !got.HideoutErrorNotifications || got.HideoutErrorSoundPath != s.HideoutErrorSoundPath {
		t.Fatal("settings lost", err)
	}
	legacy, err := decodeSettings([]byte(`{}`))
	if err != nil || legacy.HideoutErrorNotifications {
		t.Fatal("notifications should be opt-in")
	}
}
