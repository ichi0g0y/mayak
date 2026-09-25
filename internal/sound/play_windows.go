//go:build windows

package sound

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	sndMemory    = 0x0004
	sndNoDefault = 0x0002
)

var playSound = syscall.NewLazyDLL("winmm.dll").NewProc("PlaySoundW")
var mciSendString = syscall.NewLazyDLL("winmm.dll").NewProc("mciSendStringW")
var soundAliasSequence atomic.Uint64

func Play(kind Kind, volume int) {
	wave := Wave(kind, volume)
	if len(wave) == 0 {
		return
	}
	// Synchronous memory playback keeps the backing slice alive, while callers
	// run Play in a goroutine so screenshot processing is never blocked.
	_, _, _ = playSound.Call(uintptr(unsafe.Pointer(&wave[0])), 0, sndMemory|sndNoDefault)
}

func PlayFile(path string, volume int) error {
	if err := ValidateFile(path); err != nil {
		return err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	alias := fmt.Sprintf("mayak_sound_%d", soundAliasSequence.Add(1))
	deviceType := "waveaudio"
	if strings.EqualFold(filepath.Ext(absolute), ".mp3") {
		deviceType = "mpegvideo"
	}
	if err := sendMCI(fmt.Sprintf("open \"%s\" type %s alias %s", absolute, deviceType, alias)); err != nil {
		return err
	}
	defer func() { _ = sendMCI("close " + alias) }()
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	// Some Windows WAV codecs do not implement per-stream MCI volume. Playback
	// remains useful at the system volume when that optional command is rejected.
	_ = sendMCI(fmt.Sprintf("setaudio %s volume to %d", alias, volume*10))
	return sendMCI("play " + alias + " wait")
}

func sendMCI(command string) error {
	commandPointer, err := syscall.UTF16PtrFromString(command)
	if err != nil {
		return err
	}
	code, _, _ := mciSendString.Call(uintptr(unsafe.Pointer(commandPointer)), 0, 0, 0)
	if code != 0 {
		return fmt.Errorf("Windows audio error %d", code)
	}
	return nil
}
