package app

import (
	"reflect"
	"testing"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/trackerstore"
)

func TestImmediateSettingsAndTrackerAssignmentSurviveReload(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	a := NewApp()
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.TarkovTrackerEnabled = true
	settings.QuestSite = "japanese-wiki"
	settings.RemoteTargets = []config.RemoteTarget{{ID: "ABCD", Name: "First", Map: true}, {ID: "EFGH", Name: "Second", Tasks: true}}
	a.settings = settings
	if err := a.PersistSettings(settings); err != nil {
		t.Fatal(err)
	}
	// Load from disk without an explicit SaveSettings call or clean shutdown.
	reloaded, err := config.Load()
	if err != nil || !reloaded.TarkovTrackerEnabled || reloaded.QuestSite != "japanese-wiki" || !reflect.DeepEqual(reloaded.RemoteTargets, settings.RemoteTargets) {
		t.Fatal("autosaved tracker or remote settings lost", err)
	}
	a.trackerData.RememberProfiles([]trackerstore.Profile{{AccountID: "account", ProfileID: "profile", Mode: "pve"}})
	key, err := a.trackerData.AddKey("pve", "PVE_fixture", "Tracker name")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetTrackerProfileKey("account", "profile", "pve", key.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := a.trackerStore.Load()
	if err != nil || loaded.TokenFor("account", "profile", "pve") != "PVE_fixture" {
		t.Fatal("key assignment lost", err)
	}
	settings.SoundVolume = 45
	if err := a.PersistSettings(settings); err != nil {
		t.Fatal(err)
	}
	reloaded, err = config.Load()
	if err != nil || len(reloaded.RemoteTargets) != 2 || reloaded.SoundVolume != 45 || !reloaded.TarkovTrackerEnabled {
		t.Fatal("later edit lost earlier settings", err)
	}
}
