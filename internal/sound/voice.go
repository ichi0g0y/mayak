package sound

import (
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"sort"
)

// The built-in voices: a folder a pack under voices/, with its pack.json (who
// speaks, the credit and terms, the lines) and a WAV (mono IMA ADPCM, a
// quarter of 16-bit PCM's size) a notification, made from the lines by
// tools/voices. The voices are not under MAYAK's license but their own terms
// (voices/LICENSE.md).
//
//go:embed voices
var voiceFiles embed.FS

// VoicePack is a built-in voice, as the settings list it.
type VoicePack struct {
	ID string `json:"id"`
	// Name is the speaker's name by UI language ("ja", "en").
	Name     map[string]string `json:"name"`
	Language string            `json:"language"`
	Order    int               `json:"order"`
	// Credit is the notice its terms ask for ("VOICEVOX:春日部つむぎ"),
	// Terms their page and TermsChecked when they were last read.
	Credit       string `json:"credit"`
	Terms        string `json:"terms"`
	TermsChecked string `json:"termsChecked"`
}

// VoicePacks lists the built-in voices, Japanese first.
func VoicePacks() []VoicePack {
	entries, err := fs.ReadDir(voiceFiles, "voices")
	if err != nil {
		return nil
	}
	packs := []VoicePack{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := voiceFiles.ReadFile("voices/" + entry.Name() + "/pack.json")
		if err != nil {
			continue
		}
		var pack VoicePack
		if json.Unmarshal(data, &pack) != nil {
			continue
		}
		pack.ID = entry.Name()
		packs = append(packs, pack)
	}
	sort.SliceStable(packs, func(i, j int) bool {
		if packs[i].Language != packs[j].Language {
			return packs[i].Language == "ja"
		}
		return packs[i].Order < packs[j].Order
	})
	return packs
}

// HasVoice tells a built-in voice by its ID.
func HasVoice(id string) bool {
	for _, pack := range VoicePacks() {
		if pack.ID == id {
			return true
		}
	}
	return false
}

// voiceWave is a voice's line for a notification, at a volume (0–100).
func voiceWave(pack string, kind Kind, volume int) ([]byte, error) {
	if !HasVoice(pack) {
		return nil, errors.New("unknown voice")
	}
	data, err := voiceFiles.ReadFile("voices/" + pack + "/" + string(kind) + ".wav")
	if err != nil {
		return nil, err
	}
	rate, pcm, err := decodeWave(data)
	if err != nil {
		return nil, err
	}
	factor := float64(min(max(volume, 0), 100)) / 100
	for i, v := range pcm {
		pcm[i] = int16(math.Round(float64(v) * factor))
	}
	return pcmWave(rate, pcm), nil
}

// decodeWave reads a mono WAV, 16-bit PCM or IMA ADPCM, into samples.
func decodeWave(data []byte) (int, []int16, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return 0, nil, errors.New("not a WAV file")
	}
	var format, channels, blockAlign, bits int
	var rate, count int
	for at := 12; at+8 <= len(data); {
		id := string(data[at : at+4])
		size := int(binary.LittleEndian.Uint32(data[at+4 : at+8]))
		body := at + 8
		if size > len(data)-body {
			size = len(data) - body
		}
		chunk := data[body : body+size]
		switch id {
		case "fmt ":
			if size < 16 {
				return 0, nil, errors.New("bad WAV format")
			}
			format = int(binary.LittleEndian.Uint16(chunk))
			channels = int(binary.LittleEndian.Uint16(chunk[2:]))
			rate = int(binary.LittleEndian.Uint32(chunk[4:]))
			blockAlign = int(binary.LittleEndian.Uint16(chunk[12:]))
			bits = int(binary.LittleEndian.Uint16(chunk[14:]))
		case "fact":
			if size >= 4 {
				count = int(binary.LittleEndian.Uint32(chunk))
			}
		case "data":
			switch {
			case channels != 1:
				return 0, nil, errors.New("only mono voices are supported")
			case format == 1 && bits == 16:
				pcm := make([]int16, size/2)
				for i := range pcm {
					pcm[i] = int16(binary.LittleEndian.Uint16(chunk[i*2:]))
				}
				return rate, pcm, nil
			case format == 0x11 && bits == 4 && blockAlign > 4:
				pcm := decodeADPCM(chunk, blockAlign)
				if count > 0 && count < len(pcm) {
					pcm = pcm[:count]
				}
				return rate, pcm, nil
			}
			return 0, nil, errors.New("only 16-bit PCM and IMA ADPCM voices are supported")
		}
		at = body + size + size%2
	}
	return 0, nil, errors.New("WAV file has no data")
}

var adpcmSteps = [89]int{
	7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21, 23, 25, 28, 31, 34, 37, 41, 45, 50, 55, 60, 66, 73, 80, 88, 97, 107,
	118, 130, 143, 157, 173, 190, 209, 230, 253, 279, 307, 337, 371, 408, 449, 494, 544, 598, 658, 724, 796, 876, 963,
	1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358, 5894,
	6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899, 15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794,
	32767,
}

var adpcmIndexShift = [8]int{-1, -1, -1, -1, 2, 4, 6, 8}

// decodeADPCM decodes mono IMA ADPCM blocks: each a 4-byte header (the first
// sample, the step index) and two samples a byte, the low nibble first.
func decodeADPCM(data []byte, blockAlign int) []int16 {
	pcm := make([]int16, 0, len(data)/blockAlign*((blockAlign-4)*2+1))
	for start := 0; start+4 <= len(data); start += blockAlign {
		block := data[start:min(start+blockAlign, len(data))]
		predictor := int(int16(binary.LittleEndian.Uint16(block)))
		index := min(max(int(block[2]), 0), 88)
		pcm = append(pcm, int16(predictor))
		for _, b := range block[4:] {
			for _, code := range [2]int{int(b & 0x0f), int(b >> 4)} {
				step := adpcmSteps[index]
				delta := step >> 3
				if code&4 != 0 {
					delta += step
				}
				if code&2 != 0 {
					delta += step >> 1
				}
				if code&1 != 0 {
					delta += step >> 2
				}
				if code&8 != 0 {
					predictor -= delta
				} else {
					predictor += delta
				}
				predictor = min(max(predictor, math.MinInt16), math.MaxInt16)
				index = min(max(index+adpcmIndexShift[code&7], 0), 88)
				pcm = append(pcm, int16(predictor))
			}
		}
	}
	return pcm
}
