package sound

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWaveHeaderAndVolumeBounds(t *testing.T) {
	for _, tc := range []struct {
		kind   Kind
		volume int
	}{{Quest, -10}, {Quest, 30}, {Error, 150}, {MatchFound, 30}, {RaidStart, 30}, {RunThrough, 30}} {
		wave := Wave(tc.kind, tc.volume)
		if len(wave) <= 44 || string(wave[:4]) != "RIFF" || string(wave[8:12]) != "WAVE" {
			t.Fatalf("invalid wave for %+v", tc)
		}
	}
}

func TestValidateFileAcceptsExistingWAVAndMP3(t *testing.T) {
	for _, name := range []string{"alert.wav", "alert.MP3"} {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte("audio"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := ValidateFile(path); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestValidateFileRejectsUnsupportedFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alert.txt")
	if err := os.WriteFile(path, []byte("audio"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFile(path); err == nil {
		t.Fatal("unsupported sound file was accepted")
	}
}
