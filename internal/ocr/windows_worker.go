package ocr

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	// A recognition normally takes well under a second once the worker runs.
	workerTimeout = 30 * time.Second
	// An idle worker is stopped to release PowerShell's memory.
	workerIdle = 10 * time.Minute
)

// windowsWorker keeps one PowerShell process with Windows OCR loaded (see
// windows_ocr.ps1). Starting PowerShell and WinRT for every crop cost several
// hundred milliseconds; the worker pays that once. Requests are serialized.
type windowsWorker struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	script string
	idle   *time.Timer
}

var sharedWindowsWorker windowsWorker

type workerReply struct {
	OK    bool   `json:"ok"`
	Text  string `json:"text"`
	Error string `json:"error"`
}

func (w *windowsWorker) recognize(ctx context.Context, path, language string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.start(); err != nil {
		return "", err
	}
	if w.idle != nil {
		w.idle.Stop()
	}
	defer func() { w.idle = time.AfterFunc(workerIdle, w.stopIdle) }()

	request, err := json.Marshal(map[string]string{"path": path, "language": language})
	if err != nil {
		return "", err
	}
	stdin, stdout := w.stdin, w.stdout
	done := make(chan struct {
		reply workerReply
		err   error
	}, 1)
	go func() {
		var result struct {
			reply workerReply
			err   error
		}
		if _, result.err = stdin.Write(append(request, '\n')); result.err == nil {
			var line []byte
			if line, result.err = stdout.ReadBytes('\n'); result.err == nil {
				result.err = json.Unmarshal(line, &result.reply)
			}
		}
		done <- result
	}()

	timeout := time.NewTimer(workerTimeout)
	defer timeout.Stop()
	select {
	case result := <-done:
		if result.err != nil {
			// The worker is out of sync or gone; the next request starts a new one.
			w.stop()
			return "", errors.New("Windows OCR worker failed: " + result.err.Error())
		}
		if !result.reply.OK {
			return "", errors.New(strings.TrimSpace(result.reply.Error))
		}
		return strings.TrimSpace(result.reply.Text), nil
	case <-ctx.Done():
		w.stop()
		return "", ctx.Err()
	case <-timeout.C:
		w.stop()
		return "", errors.New("Windows OCR timed out")
	}
}

func (w *windowsWorker) start() error {
	if w.cmd != nil {
		return nil
	}
	scriptFile, err := os.CreateTemp("", "mayak-windows-ocr-*.ps1")
	if err != nil {
		return err
	}
	if _, err = scriptFile.Write(windowsOCRScript); err != nil {
		scriptFile.Close()
		os.Remove(scriptFile.Name())
		return err
	}
	if err = scriptFile.Close(); err != nil {
		os.Remove(scriptFile.Name())
		return err
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptFile.Name())
	configureHiddenProcess(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.Remove(scriptFile.Name())
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.Remove(scriptFile.Name())
		return err
	}
	if err = cmd.Start(); err != nil {
		os.Remove(scriptFile.Name())
		return err
	}
	w.cmd, w.stdin, w.stdout, w.script = cmd, stdin, bufio.NewReader(stdout), scriptFile.Name()
	return nil
}

func (w *windowsWorker) stopIdle() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stop()
}

// stop ends the worker. The caller holds w.mu. A worker left behind by a
// crash of MAYAK also exits, because its standard input closes.
func (w *windowsWorker) stop() {
	if w.cmd == nil {
		return
	}
	_ = w.stdin.Close()
	_ = w.cmd.Process.Kill()
	_ = w.cmd.Wait()
	os.Remove(w.script)
	w.cmd, w.stdin, w.stdout, w.script = nil, nil, nil, ""
}

// StopWindowsWorker ends the background Windows OCR process, if any.
func StopWindowsWorker() {
	sharedWindowsWorker.stopIdle()
}
