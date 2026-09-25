//go:build !windows

package sound

func Play(Kind, int) {}

func PlayFile(path string, volume int) error {
	return ValidateFile(path)
}
