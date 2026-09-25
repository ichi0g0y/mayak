package sound

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type Kind string

const (
	Quest        Kind = "quest"
	Error        Kind = "error"
	MatchFound   Kind = "matchFound"
	RaidStart    Kind = "raidStart"
	RunThrough   Kind = "runThrough"
	QuestItems   Kind = "questItems"
	RestartTasks Kind = "restartTasks"
)

const sampleRate = 44100

func ValidateFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("sound file is required")
	}
	if strings.ContainsAny(path, "\r\n\"") {
		return errors.New("sound file path contains unsupported characters")
	}
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".wav" && extension != ".mp3" {
		return errors.New("only WAV and MP3 sound files are supported")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("sound file path is a directory")
	}
	return nil
}

func Wave(kind Kind, volume int) []byte {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	duration := .24
	switch kind {
	case Error:
		duration = .3
	case MatchFound:
		duration = .38
	case RaidStart:
		duration = .42
	case RunThrough:
		duration = .46
	case QuestItems, RestartTasks:
		duration = .4
	}
	count := int(sampleRate * duration)
	pcm := make([]int16, count)
	gain := float64(volume) / 100 * .22
	var noise uint32 = 0x4a3b2c1d
	for i := range pcm {
		t := float64(i) / sampleRate
		noise = noise*1664525 + 1013904223
		n := (float64(int32(noise>>16)-32768) / 32768)
		var sample float64
		switch kind {
		case Error:
			sample = pulse(t, 0, .11, 185) + .78*pulse(t, .15, .11, 145)
		case MatchFound:
			sample = .72*pulse(t, 0, .14, 610) + pulse(t, .16, .18, 840)
		case RaidStart:
			sample = .82*pulse(t, 0, .16, 245) + pulse(t, .17, .21, 365)
		case RunThrough:
			sample = .55*pulse(t, 0, .16, 440) + .72*pulse(t, .13, .18, 610) + pulse(t, .27, .15, 790)
		case QuestItems:
			sample = .72*pulse(t, 0, .14, 330) + pulse(t, .18, .17, 520)
		case RestartTasks:
			sample = pulse(t, 0, .14, 260) + .75*pulse(t, .17, .18, 210)
		default:
			sample = .72*pulse(t, 0, .09, 520) + pulse(t, .085, .13, 760)
			if t < .035 {
				sample += n * (1 - t/.035) * .18
			}
		}
		pcm[i] = int16(math.MaxInt16 * gain * clampSample(sample))
	}

	var out bytes.Buffer
	dataSize := uint32(len(pcm) * 2)
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(36)+dataSize)
	out.WriteString("WAVEfmt ")
	_ = binary.Write(&out, binary.LittleEndian, uint32(16))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&out, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(&out, binary.LittleEndian, uint16(2))
	_ = binary.Write(&out, binary.LittleEndian, uint16(16))
	out.WriteString("data")
	_ = binary.Write(&out, binary.LittleEndian, dataSize)
	for _, sample := range pcm {
		_ = binary.Write(&out, binary.LittleEndian, sample)
	}
	return out.Bytes()
}

func pulse(t, start, duration, frequency float64) float64 {
	if t < start || t >= start+duration {
		return 0
	}
	local := (t - start) / duration
	envelope := math.Sin(math.Pi*math.Min(local/.12, 1)) * math.Pow(1-local, 1.8)
	return math.Sin(2*math.Pi*frequency*(t-start)) * envelope
}

func clampSample(v float64) float64 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}
