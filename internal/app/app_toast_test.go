package app

import (
	"testing"

	"github.com/local/mayak/internal/config"
)

// A notice with a key replaces the one shown with the same key, in place and
// as one line of the history (an update's progress); the others stack.
func TestToastReplacesByKey(t *testing.T) {
	a := &App{}
	first := a.toast(Toast{Key: "update", Category: ToastUpdate, Persistent: true, Message: "updateDownloading", Progress: 10})
	a.toast(Toast{Category: ToastTracker, Message: "toastLevelSet"})
	second := a.toast(Toast{Key: "update", Category: ToastUpdate, Persistent: true, Message: "updateDownloading", Progress: 60})
	if first == "" || first != second {
		t.Fatalf("ids %q, %q", first, second)
	}
	shown := a.BrowserToastList()
	if len(shown) != 2 || shown[0].Progress != 60 {
		t.Fatalf("shown %+v", shown)
	}
	if history := a.BrowserToastHistory(); len(history) != 2 || history[0].Progress != 60 {
		t.Fatalf("history %+v", history)
	}
	a.endToast("update")
	if shown := a.BrowserToastList(); len(shown) != 1 || shown[0].Key != "" {
		t.Fatalf("after its end %+v", shown)
	}
}

// A kind turned off in the settings shows nothing and leaves no history;
// errors always show; a running job is shown but not kept.
func TestToastKindsOff(t *testing.T) {
	a := &App{settings: config.Settings{ToastsOff: []string{ToastSound}}}
	if id := a.toast(Toast{Category: ToastSound, Message: "toastAlert_raidStart"}); id != "" {
		t.Fatal("a kind turned off showed")
	}
	a.toast(Toast{Category: ToastError, Level: "error", Text: "boom"})
	a.toast(Toast{Key: "history:x", Category: ToastWorking, Persistent: true, Message: "toastHistoryWorking"})
	if shown := a.BrowserToastList(); len(shown) != 2 {
		t.Fatalf("shown %+v", shown)
	}
	if history := a.BrowserToastHistory(); len(history) != 1 || history[0].Category != ToastError {
		t.Fatalf("history %+v", history)
	}
}

// At most toastMaxShown notices show; the oldest of those that would go anyway
// make room, the persistent ones stay.
func TestToastMaxShown(t *testing.T) {
	a := &App{}
	a.toast(Toast{Key: "update", Category: ToastUpdate, Persistent: true, Message: "updateReady"})
	for i := 0; i < toastMaxShown+2; i++ {
		a.toast(Toast{Category: ToastTracker, Message: "toastLevelSet"})
	}
	shown := a.BrowserToastList()
	if len(shown) != toastMaxShown || shown[0].Key != "update" {
		t.Fatalf("shown %d, first %+v", len(shown), shown[0])
	}
}

// A press passes to the shell only for a button the notice has.
func TestToastActionNeedsItsButton(t *testing.T) {
	a := &App{}
	id := a.toast(Toast{Category: ToastTracker, Persistent: true, Message: "toastPrestige", Actions: []ToastAction{{ID: "trackerPrestige", Label: "toastOpenTrackerPrestige"}}})
	a.BrowserToastAction(id, "updateInstall") // no such button: nothing, no panic
	a.BrowserToastClose(id)
	if len(a.BrowserToastList()) != 0 {
		t.Fatal("not closed")
	}
}
