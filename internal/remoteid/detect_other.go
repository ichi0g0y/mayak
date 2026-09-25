//go:build !windows

package remoteid

import "errors"

func Detect() (string, error) {
	return "", errors.New("automatic Remote ID detection is available only on Windows")
}
