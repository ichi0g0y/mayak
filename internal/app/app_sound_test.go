package app

import (
	"testing"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/sound"
)

// A notification speaks with its own voice, "beep" for the beeps, else the
// voice for all.
func TestVoiceFor(t *testing.T) {
	s := config.Settings{SoundVoice: "tsumugi", SoundVoices: map[string]string{"raidStart": "kenzaki", "error": "beep"}}
	for kind, want := range map[sound.Kind]string{sound.RaidStart: "kenzaki", sound.Error: "", sound.Quest: "tsumugi"} {
		if got := voiceFor(s, kind); got != want {
			t.Fatalf("%s: %q, want %q", kind, got, want)
		}
	}
}

// Choices of a notification or a voice that no longer exist are dropped.
func TestNormalizeSettingsDropsUnknownVoices(t *testing.T) {
	s := normalizeSettings(config.Settings{SoundVoices: map[string]string{"raidStart": "kenzaki", "error": "beep", "gone": "hau", "quest": "nobody"}})
	if len(s.SoundVoices) != 2 || s.SoundVoices["raidStart"] != "kenzaki" || s.SoundVoices["error"] != "beep" {
		t.Fatalf("voices %v", s.SoundVoices)
	}
}
