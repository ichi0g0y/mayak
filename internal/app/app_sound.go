package app

import (
	"errors"
	"sync"
	"time"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/sound"
)

// VoicePacks lists the built-in voices the notifications can speak with.
func (a *App) VoicePacks() []sound.VoicePack {
	return sound.VoicePacks()
}

// PreviewSound plays a notification as it would sound with a file ("" for
// none) and a built-in voice ("" for the beeps).
func (a *App) PreviewSound(kind, path, voice string, volume int) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	soundKind, ok := sound.ParseKind(kind)
	if !ok {
		return errors.New("unknown sound kind")
	}
	if path != "" {
		if path = soundFilePath(path); path == "" {
			return errors.New("unknown sound file")
		}
		if err := sound.ValidateFile(path); err != nil {
			return err
		}
	}
	if voice != "" && !sound.HasVoice(voice) {
		return errors.New("unknown voice")
	}
	playNotification(soundKind, path, voice, volume)
	return nil
}

// Notifications play one after another, not over each other: a raid's start
// can bring three at once. A queue that is full drops the newest.
var (
	notifications     = make(chan queuedSound, 8)
	startNotification sync.Once
)

type queuedSound struct {
	kind   sound.Kind
	path   string
	voice  string
	volume int
}

// playNotification plays the file chosen for a notification, else the
// built-in voice's line, else the built-in beep.
func playNotification(kind sound.Kind, path, voice string, volume int) {
	startNotification.Do(func() {
		go func() {
			for n := range notifications {
				if n.path != "" && sound.PlayFile(n.path, n.volume) == nil {
					continue
				}
				if n.voice != "" && sound.PlayVoice(n.voice, n.kind, n.volume) == nil {
					continue
				}
				sound.Play(n.kind, n.volume)
			}
		}()
	})
	select {
	case notifications <- queuedSound{kind, path, voice, volume}:
	default:
	}
}

// soundChoice is whether a notification is on and the file chosen for it
// ("" for the built-in sound).
func soundChoice(s config.Settings, kind sound.Kind) (bool, string) {
	switch kind {
	case sound.Quest:
		return s.QuestSoundEnabled, s.QuestSoundPath
	case sound.TaskNotMatched:
		return s.TaskNotMatchedSoundEnabled, s.TaskNotMatchedSoundPath
	case sound.Item:
		return s.ItemSoundEnabled, s.ItemSoundPath
	case sound.ItemNotMatched:
		return s.ItemNotMatchedSoundEnabled, s.ItemNotMatchedSoundPath
	case sound.Error:
		return s.ErrorSoundEnabled, s.ErrorSoundPath
	case sound.RemoteError:
		return s.RemoteErrorSoundEnabled, s.RemoteErrorSoundPath
	case sound.MatchFound:
		return s.MatchFoundSound, s.MatchFoundSoundPath
	case sound.RaidStart:
		return s.RaidStartSound, s.RaidStartSoundPath
	case sound.RunThrough:
		return s.RunThroughSound, s.RunThroughSoundPath
	case sound.QuestItems:
		return s.QuestItemsSound, s.QuestItemsSoundPath
	case sound.RestartTasks:
		return s.RestartTasksSound, s.RestartTasksSoundPath
	}
	return false, ""
}

// notify plays a notification when sounds and it are on.
func (a *App) notify(s config.Settings, kind sound.Kind) {
	enabled, path := soundChoice(s, kind)
	if !s.SoundsEnabled || !enabled {
		return
	}
	a.addLog("Info", "Sound", "Playing "+string(kind)+" alert")
	playNotification(kind, soundFilePath(path), voiceFor(s, kind), volumeFor(s, kind))
}

// volumeFor is a notification's volume: the one for all with its own
// adjustment, within 0–100.
func volumeFor(s config.Settings, kind sound.Kind) int {
	return min(max(s.SoundVolume+s.SoundVolumeOffsets[string(kind)], 0), 100)
}

// voiceFor is the built-in voice a notification speaks with: its own choice,
// else the one for all ("" for the beeps).
func voiceFor(s config.Settings, kind sound.Kind) string {
	if voice, ok := s.SoundVoices[string(kind)]; ok && voice != "custom" {
		if voice == "beep" {
			return ""
		}
		return voice
	}
	return baseVoice(s)
}

// baseVoice is the voice for all: the one chosen, "beep" for the beeps, and
// none chosen the default for the language.
func baseVoice(s config.Settings) string {
	switch s.SoundVoice {
	case "beep":
		return ""
	case "":
		return sound.DefaultVoice(s.Language)
	}
	return s.SoundVoice
}

// notifyOnce plays a notification of a screenshot or error, but not again
// for the same key (the same task, item or error) within a few seconds.
func (a *App) notifyOnce(kind sound.Kind, key string) {
	a.mu.Lock()
	s := a.settings
	now := time.Now()
	last := a.lastSounds[kind]
	if last.key == key && now.Sub(last.at) < recognitionSoundCooldown {
		a.mu.Unlock()
		return
	}
	if enabled, _ := soundChoice(s, kind); s.SoundsEnabled && enabled {
		if a.lastSounds == nil {
			a.lastSounds = map[sound.Kind]recentSound{}
		}
		a.lastSounds[kind] = recentSound{key: key, at: now}
	}
	a.mu.Unlock()
	a.notify(s, kind)
}

type recentSound struct {
	key string
	at  time.Time
}
