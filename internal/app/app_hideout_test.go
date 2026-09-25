package app

import (
	"context"
	"github.com/local/mayak/internal/catalog"
	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/hideoutlog"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/trackerstore"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHideoutProgressRequiresExactCurrentBindingAndMode(t *testing.T) {
	a := &App{settings: config.Settings{TarkovTrackerEnabled: true}, status: model.Status{Tracker: model.TrackerStatus{AccountID: "account", ProfileID: "profile", Mode: "pve"}}, trackerData: trackerstore.Document{Keys: []trackerstore.Key{{AccountID: "account", ProfileID: "profile", Mode: "pve", Token: "key"}}}, hideoutProgressToken: "key", hideoutProgress: map[string]bool{"level": true}, hideoutCatalogMode: "pve", hideoutStations: []catalog.HideoutStation{{ID: "station", Levels: []catalog.HideoutLevel{{ID: "level", Level: 1}}}}}
	a.updateHideoutLocked()
	if a.status.Hideout.Completed != 1 {
		t.Fatal("matching progress unavailable")
	}
	a.trackerData.Keys[0].Token = "replacement"
	a.updateHideoutLocked()
	if a.status.Hideout.State != "waiting-progress" || a.status.Hideout.Completed != 0 {
		t.Fatal("old key progress retained")
	}
	a.trackerData.Keys[0].Token = "key"
	a.hideoutCatalogMode = "regular"
	a.updateHideoutLocked()
	if a.status.Hideout.State != "waiting-catalog" || len(a.status.Hideout.Stations) > 0 {
		t.Fatal("catalog modes mixed")
	}
	a.hideoutCatalogMode = "pve"
	a.status.Tracker.ProfileID = "other"
	a.updateHideoutLocked()
	if a.status.Hideout.Completed != 0 {
		t.Fatal("profile progress leaked")
	}
	a.status.Tracker.ProfileID = "profile"
	a.settings.TarkovTrackerEnabled = false
	a.updateHideoutLocked()
	if a.status.Hideout.State != "disabled" || a.status.Hideout.Completed != 0 {
		t.Fatal("disabled progress retained")
	}
}

func TestHideoutNotificationsRequireNewActionableEvents(t *testing.T) {
	for _, status := range []string{"info", "failed", "unknown"} {
		for _, historical := range []bool{true, false} {
			event := hideoutlog.Event{Status: status, Historical: historical}
			if got := shouldNotifyHideout(event, true); got != (!historical && status == "failed") {
				t.Fatalf("incorrect error notification: %s %v", status, historical)
			}
			if shouldNotifyHideout(event, false) {
				t.Fatalf("notification while disabled: %s %v", status, historical)
			}
		}
	}
}

func TestHideoutSaveErrorClearsAfterRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.json")
	store := hideoutlog.NewStore(path)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	app := &App{hideoutStore: store}
	event := hideoutlog.Event{AreaType: -1, OccurredAt: time.Now(), Action: "HideoutUpgradeComplete", ActionTimestamp: 1, Historical: true}
	app.handleHideoutEvent(context.Background(), event)
	if app.status.Hideout.LastError != "history-save-failed" {
		t.Fatal("save failure was not reported")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	event.ActionTimestamp = 2
	app.handleHideoutEvent(context.Background(), event)
	if app.status.Hideout.LastError != "" {
		t.Fatal("recovered save error remained visible")
	}
	if len(hideoutlog.NewStore(path).Events()) != 2 {
		t.Fatal("history did not recover")
	}
}
