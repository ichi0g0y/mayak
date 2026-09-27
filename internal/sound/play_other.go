//go:build !windows

package sound

func Play(Kind, int) {}

func PlayVoice(pack string, kind Kind, volume int) error {
	_, err := voiceWave(pack, kind, volume)
	return err
}

func PlayFile(path string, volume int) error {
	return ValidateFile(path)
}
