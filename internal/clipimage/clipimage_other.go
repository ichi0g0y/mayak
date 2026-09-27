//go:build !windows

package clipimage

import "errors"

// CopyPNG is Windows-only so far.
func CopyPNG([]byte) error { return errors.New("copying pictures is not supported on this system") }
