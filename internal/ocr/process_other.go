//go:build !windows

package ocr

import "os/exec"

func configureHiddenProcess(*exec.Cmd) {}
