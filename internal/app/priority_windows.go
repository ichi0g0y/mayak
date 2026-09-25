package app

import "golang.org/x/sys/windows"

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
