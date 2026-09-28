package sound

import (
	"encoding/binary"
	"testing"
)

// Every built-in voice has its credit and terms and a line for every
// notification, and each line decodes.
func TestVoicePacksHaveEveryLine(t *testing.T) {
	packs := VoicePacks()
	if len(packs) == 0 {
		t.Fatal("no voices")
	}
	for _, pack := range packs {
		if pack.Credit == "" || pack.Terms == "" || pack.TermsChecked == "" || pack.Name["ja"] == "" || pack.Name["en"] == "" {
			t.Fatalf("pack %+v", pack)
		}
		for _, kind := range Kinds {
			wave, err := voiceWave(pack.ID, kind, 50)
			if err != nil {
				t.Fatalf("%s/%s: %v", pack.ID, kind, err)
			}
			if rate, pcm, err := decodeWave(wave); err != nil || rate < 16000 || len(pcm) < rate/4 {
				t.Fatalf("%s/%s: rate %d, %d samples, %v", pack.ID, kind, rate, len(pcm), err)
			}
		}
	}
	if _, err := voiceWave("../voices/tsumugi", Quest, 50); err == nil {
		t.Fatal("a path was taken for a voice")
	}
}

// The volume scales the samples; 0 is silence.
func TestVoiceVolume(t *testing.T) {
	id := VoicePacks()[0].ID
	loud, _ := voiceWave(id, Quest, 100)
	quiet, _ := voiceWave(id, Quest, 0)
	_, a, _ := decodeWave(loud)
	_, b, _ := decodeWave(quiet)
	peak := 0
	for i := range a {
		peak = max(peak, int(a[i]), -int(a[i]))
		if b[i] != 0 {
			t.Fatalf("sample %d at volume 0: %d", i, b[i])
		}
	}
	if peak < 1000 {
		t.Fatalf("peak %d", peak)
	}
}

// A block of IMA ADPCM: its header sample first, then two samples a byte.
func TestDecodeADPCMBlock(t *testing.T) {
	block := make([]byte, 6)
	binary.LittleEndian.PutUint16(block, uint16(1000))
	block[2] = 0
	block[4] = 0x04 // 4: +7 (step 7: 7>>3 + 7), then 0: +1 (step 9 now: 9>>3)
	got := decodeADPCM(block, len(block))
	if len(got) != 5 || got[0] != 1000 || got[1] != 1007 || got[2] != 1008 {
		t.Fatalf("got %v", got)
	}
}

// Every voice has what it says for every notification, for the settings.
func TestVoicePacksHaveLineTexts(t *testing.T) {
	for _, pack := range VoicePacks() {
		for _, kind := range Kinds {
			if pack.Lines[string(kind)] == "" {
				t.Fatalf("%s: no text for %s", pack.ID, kind)
			}
		}
	}
	if DefaultVoice("ja") != "tsumugi" || DefaultVoice("en") != "heart" {
		t.Fatalf("defaults %q %q", DefaultVoice("ja"), DefaultVoice("en"))
	}
}
