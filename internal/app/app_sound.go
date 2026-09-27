
package app

import (
	"errors"
	"time"

	"github.com/local/mayak/internal/sound"
)

func (a *App) PreviewSound(kind, path string, volume int) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	var soundKind sound.Kind
	switch kind {
	case string(sound.Quest):
		soundKind = sound.Quest
	case string(sound.Error):
		soundKind = sound.Error
	case string(sound.MatchFound):
		soundKind = sound.MatchFound
	case string(sound.RaidStart):
		soundKind = sound.RaidStart
	case string(sound.RunThrough):
		soundKind = sound.RunThrough
	case string(sound.QuestItems):
		soundKind = sound.QuestItems
	case string(sound.RestartTasks):
		soundKind = sound.RestartTasks
	default:
		return errors.New("unknown sound kind")
	}
	if path != "" {
		if err := sound.ValidateFile(path); err != nil {
			return err
		}
	}
	playNotification(soundKind, path, volume)
	return nil
}

func playNotification(kind sound.Kind, path string, volume int) {
	go func() {
		if path != "" && sound.PlayFile(path, volume) == nil {
			return
		}
		sound.Play(kind, volume)
	}()
}

func (a *App) playQuestSound(key string) {
	a.mu.Lock()
	s := a.settings
	now := time.Now()
	if !s.SoundsEnabled || !s.QuestSoundEnabled || (key == a.lastQuestSoundKey && now.Sub(a.lastQuestSoundAt) < recognitionSoundCooldown) {
		a.mu.Unlock()
		return
	}
	a.lastQuestSoundKey, a.lastQuestSoundAt = key, now
	a.mu.Unlock()
	a.addLog("Info", "Sound", "Playing quest-recognition alert")
	playNotification(sound.Quest, s.QuestSoundPath, s.SoundVolume)
}

func (a *App) playErrorSound(key string) {
	a.mu.Lock()
	s := a.settings
	now := time.Now()
	if !s.SoundsEnabled || !s.ErrorSoundEnabled || (key == a.lastErrorSoundKey && now.Sub(a.lastErrorSoundAt) < recognitionSoundCooldown) {
		a.mu.Unlock()
		return
	}
	a.lastErrorSoundKey, a.lastErrorSoundAt = key, now
	a.mu.Unlock()
	a.addLog("Info", "Sound", "Playing recognition-error alert")
	playNotification(sound.Error, s.ErrorSoundPath, s.SoundVolume)
}
