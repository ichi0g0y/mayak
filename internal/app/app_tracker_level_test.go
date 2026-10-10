package app

import (
	"testing"

	"github.com/local/mayak/internal/applog"
	"github.com/local/mayak/internal/model"
)

// A level kept from the Overall screen goes once TarkovTracker's level of the
// same profile reads below it, and stays for another profile or mode, or
// while TarkovTracker's level is not below it.
func TestWaitingLevelGoesAfterTheReset(t *testing.T) {
	a := &App{logs: applog.New(10)}
	a.keepLevel(model.TrackerStatus{AccountID: "1", ProfileID: "p", Mode: "pve", PlayerLevel: 45}, 3)
	if a.levelWaiting.level != 3 {
		t.Fatalf("kept %+v", a.levelWaiting)
	}
	a.sendWaitingLevel("1", "p", "pvp", 1)
	a.sendWaitingLevel("2", "p", "pve", 1)
	a.sendWaitingLevel("1", "p", "pve", 45)
	if a.levelWaiting.level != 3 {
		t.Fatalf("went before the reset: %+v", a.levelWaiting)
	}
	a.sendWaitingLevel("1", "p", "pve", 1)
	if a.levelWaiting.level != 0 {
		t.Fatalf("still kept after the reset: %+v", a.levelWaiting)
	}
}
