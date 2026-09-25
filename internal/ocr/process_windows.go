//go:build windows

package ocr

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureHiddenProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
