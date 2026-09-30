package app

import (
	"testing"

	"github.com/local/mayak/internal/model"
)

// The tasks to complete are kept per EFT profile: the one played shows its
// own, without those TarkovTracker has completed meanwhile, and they stay
// across a restart.
func TestCompletableTasksPerProfile(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	a := NewApp()
	pve := model.TrackerStatus{Connection: "connected", AccountID: "a1", ProfileID: "p1", Mode: "pve"}
	pvp := model.TrackerStatus{Connection: "connected", AccountID: "a1", ProfileID: "p1", Mode: "regular"}
	a.status.Tracker = pve
	a.completable = map[string]map[string]model.CompletableTask{
		profileKey(pve): {"t1": {ID: "t1", Name: "Debut", Trader: "Prapor"}, "t2": {ID: "t2", Name: "Escort", Trader: "Prapor"}},
		profileKey(pvp): {"t3": {ID: "t3", Name: "Shortage", Trader: "Therapist"}},
	}
	a.trackerTasks["t2"] = "completed"
	a.mu.Lock()
	a.saveCompletableLocked()
	a.mu.Unlock()
	a.publishCompletable()
	if got := a.status.CompletableTasks; len(got) != 1 || got[0].ID != "t1" {
		t.Fatalf("PvE list = %+v", got)
	}
	a.status.Tracker = pvp
	a.publishCompletable()
	if got := a.status.CompletableTasks; len(got) != 1 || got[0].ID != "t3" {
		t.Fatalf("PvP list = %+v", got)
	}
	// Read again after a restart; dismissing empties the profile's own list.
	b := NewApp()
	b.status.Tracker = pve
	b.publishCompletable()
	if got := b.status.CompletableTasks; len(got) != 1 || got[0].ID != "t1" {
		t.Fatalf("kept list = %+v", got)
	}
	b.TrackerDismissTasks(nil)
	if len(b.status.CompletableTasks) != 0 {
		t.Fatal("dismissed tasks stayed")
	}
	b.status.Tracker = pvp
	b.publishCompletable()
	if len(b.status.CompletableTasks) != 1 {
		t.Fatal("another profile's list went too")
	}
}
