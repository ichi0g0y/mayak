package update

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// waitEnv carries the process ID of the program that restarts into a new
// version. The new program waits for it to exit before it starts, so the
// two never run at once (MAYAK runs as a single instance).
const waitEnv = "MAYAK_UPDATE_WAIT_PID"

// Relaunch starts the program at exe, its own path as it was before Apply
// (on Linux, os.Executable follows the running file to its renamed name
// afterwards), with the same arguments and working directory, told to wait
// for this process to exit. The caller quits afterwards.
func Relaunch(exe string) error {
	exe = strings.TrimSuffix(exe, oldSuffix)
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), waitEnv+"="+strconv.Itoa(os.Getpid()))
	if dir, err := os.Getwd(); err == nil {
		cmd.Dir = dir
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// WaitForPreviousInstance blocks while the process named by waitEnv, the
// version this one replaces, is still running (up to timeout). It reports
// whether this start is such a restart.
func WaitForPreviousInstance(timeout time.Duration) bool {
	value := os.Getenv(waitEnv)
	if value == "" {
		return false
	}
	os.Unsetenv(waitEnv)
	pid, err := strconv.Atoi(value)
	if err != nil || pid <= 0 || pid == os.Getpid() {
		return false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) && processRunning(pid) {
		time.Sleep(100 * time.Millisecond)
	}
	return true
}
