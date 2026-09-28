package app

import (
	"testing"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/sound"
)

// A notification speaks with its own voice ("beep" for the beeps; "custom",
// its file, falls back on the voice for all), else the voice for all.
func TestVoiceFor(t *testing.T) {
	s := config.Settings{SoundVoice: "tsumugi", SoundVoices: map[string]string{"raidStart": "kenzaki", "error": "beep", "item": "custom"}}
	for kind, want := range map[sound.Kind]string{sound.RaidStart: "kenzaki", sound.Error: "", sound.Item: "tsumugi", sound.Quest: "tsumugi"} {
		if got := voiceFor(s, kind); got != want {
			t.Fatalf("%s: %q, want %q", kind, got, want)
		}
	}
}

// With none chosen, the voice for all is the language's default; the beeps
// are only when chosen.
func TestBaseVoiceDefaultsByLanguage(t *testing.T) {
	for _, tc := range []struct{ voice, language, want string }{
		{"", "ja", "tsumugi"},
		{"", "en", "heart"},
		{"beep", "ja", ""},
		{"kenzaki", "en", "kenzaki"},
	} {
		if got := baseVoice(config.Settings{SoundVoice: tc.voice, Language: tc.language}); got != tc.want {
			t.Fatalf("%+v: %q", tc, got)
		}
	}
}

// Choices of a notification or a voice that no longer exist are dropped.
func TestNormalizeSettingsDropsUnknownVoices(t *testing.T) {
	s := normalizeSettings(config.Settings{SoundVoice: "beep", SoundVoices: map[string]string{"raidStart": "kenzaki", "error": "beep", "item": "custom", "gone": "hau", "quest": "nobody"}})
	if s.SoundVoice != "beep" || len(s.SoundVoices) != 3 || s.SoundVoices["raidStart"] != "kenzaki" || s.SoundVoices["error"] != "beep" || s.SoundVoices["item"] != "custom" {
		t.Fatalf("voice %q, voices %v", s.SoundVoice, s.SoundVoices)
	}
	if s := normalizeSettings(config.Settings{SoundVoice: "nobody"}); s.SoundVoice != "" {
		t.Fatalf("unknown voice kept: %q", s.SoundVoice)
	}
}
