package app

import (
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// raisePriority puts the process back at normal priority when its launcher
// handed it a lower class. MAYAK is an interactive window: started from a
// below-normal shell, launcher or its own updater relaunch, it and the WebView2
// processes it spawns would be starved whenever the machine is busy, and the
// window then looks frozen and drops clicks.
func raisePriority() {
	class, err := windows.GetPriorityClass(windows.CurrentProcess())
	if err != nil {
		return
	}
	switch class {
	case windows.IDLE_PRIORITY_CLASS, windows.BELOW_NORMAL_PRIORITY_CLASS:
		_ = windows.SetPriorityClass(windows.CurrentProcess(), windows.NORMAL_PRIORITY_CLASS)
	}
}

// guardPriority keeps MAYAK and the WebView2 browser processes it spawned at
// normal priority until done closes. A priority manager (Process Lasso's
// ProBalance) lowers a process that keeps the CPU busy in the background,
// which MAYAK does for its first seconds (the catalog, the filter lists) and
// while it reads a screenshot; a WebView2 browser process spawned meanwhile
// inherits the lower class and, unlike MAYAK itself, is never given it back,
// so the window stays sluggish for good: under `task dev`, which starts the
// app again after every rebuild, on every start. Checked a few seconds after
// the start, then every ten, while enabled says so (the KeepPriority
// setting: such a tool is unusual, so this is the user's call).
func guardPriority(done <-chan struct{}, enabled func() bool) {
	wait := 3 * time.Second
	for {
		select {
		case <-done:
			return
		case <-time.After(wait):
		}
		wait = 10 * time.Second
		if !enabled() {
			continue
		}
		raisePriority()
		raiseWebViewPriority()
	}
}

// raiseWebViewPriority raises the WebView2 browser processes MAYAK spawned
// (its direct children; they set their own children's classes themselves)
// from idle or below-normal to normal.
func raiseWebViewPriority() {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snapshot)
	self := windows.GetCurrentProcessId()
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ParentProcessID != self || !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "msedgewebview2.exe") {
			continue
		}
		process, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_SET_INFORMATION, false, entry.ProcessID)
		if err != nil {
			continue
		}
		if class, err := windows.GetPriorityClass(process); err == nil && (class == windows.IDLE_PRIORITY_CLASS || class == windows.BELOW_NORMAL_PRIORITY_CLASS) {
			_ = windows.SetPriorityClass(process, windows.NORMAL_PRIORITY_CLASS)
		}
		windows.CloseHandle(process)
	}
}
