package app

import (
	"slices"
)

// The notification events: each can sound (the ones with an alert,
// sound.Kind), show as a toast and show as a desktop notification of the OS.
// The settings page lists them by group (main.tsx notifyGroups); the keys are
// the same.
var notifyEvents = []string{
	// Raid and game (the alerts).
	"matchFound", "raidStart", "runThrough", "questItems", "taskFailed", "gameStart", "gameExit",
	// What a screenshot was read as.
	"quest", "taskNotMatched", "item", "itemNotMatched", "position", "profile", "error",
	// TarkovTracker.
	"trackerTask", "trackerFailed", "trackerLevel", "trackerHistory", "prestige",
	// The squad.
	"squadSelf", "squadMembers", "squadRelay",
	// Remote Control.
	"remoteError",
}

// toastLegacyKinds are the kinds of notices the settings turned off before the
// events (toastsOff "tracker", "sound", "recognition", "squad"), each the
// events it covered.
var toastLegacyKinds = map[string][]string{
	ToastTracker:     {"trackerTask", "trackerFailed", "trackerLevel", "trackerHistory", "prestige"},
	ToastSound:       {"matchFound", "raidStart", "runThrough", "questItems", "taskFailed", "gameStart", "gameExit", "remoteError"},
	ToastRecognition: {"quest", "taskNotMatched", "item", "itemNotMatched", "position", "profile", "error"},
	ToastSquad:       {"squadSelf", "squadMembers", "squadRelay"},
}

// notifyEventList keeps the known events of list, each once, in the events'
// order; a former kind of notices stands for its events.
func notifyEventList(list []string) []string {
	expanded := []string{}
	for _, key := range list {
		if events, ok := toastLegacyKinds[key]; ok {
			expanded = append(expanded, events...)
		} else {
			expanded = append(expanded, key)
		}
	}
	out := []string{}
	for _, event := range notifyEvents {
		if slices.Contains(expanded, event) {
			out = append(out, event)
		}
	}
	return out
}

// notifyOn tells whether an event shows as a toast and as a desktop
// notification, by the settings.
func (a *App) notifyOn(event string) (toast, desktop bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return !slices.Contains(a.settings.ToastsOff, event), slices.Contains(a.settings.DesktopOn, event)
}
