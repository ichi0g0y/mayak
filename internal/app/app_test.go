package app

import (
	"testing"
	"time"

	"github.com/local/mayak/internal/config"
)

func TestRunRemoteIfCurrentSkipsStaleAnalysis(t *testing.T) {
	app := NewApp()
	app.analysisSequence.Store(2)
	called := false
	sent, err := app.runRemoteIfCurrent(1, func() error {
		called = true
		return nil
	})
	if err != nil || sent || called {
		t.Fatalf("stale operation ran: sent=%v called=%v err=%v", sent, called, err)
	}
}

func TestRunRemoteIfCurrentRunsLatestAnalysis(t *testing.T) {
	app := NewApp()
	app.analysisSequence.Store(2)
	called := false
	sent, err := app.runRemoteIfCurrent(2, func() error {
		called = true
		return nil
	})
	if err != nil || !sent || !called {
		t.Fatalf("current operation did not run: sent=%v called=%v err=%v", sent, called, err)
	}
}

func TestQueuedRemoteOperationIsDroppedWhenNewerAnalysisArrives(t *testing.T) {
	app := NewApp()
	app.analysisSequence.Store(1)
	app.remoteOperationMu.Lock()
	result := make(chan bool, 1)
	ready := make(chan struct{})
	go func() {
		close(ready)
		sent, _ := app.runRemoteIfCurrent(1, func() error {
			result <- true
			return nil
		})
		if !sent {
			result <- false
		}
	}()
	<-ready
	app.analysisSequence.Store(2)
	app.remoteOperationMu.Unlock()
	select {
	case called := <-result:
		if called {
			t.Fatal("stale queued operation ran")
		}
	case <-time.After(time.Second):
		t.Fatal("queued operation did not finish")
	}
}

func TestNormalizeMapSettingMigratesLegacySlugs(t *testing.T) {
	cases := map[string]string{
		"laboratory":     "the-lab",
		"Labyrinth":      "the-labyrinth",
		"factory4_night": "night-factory",
		" Customs ":      "customs",
	}
	for input, want := range cases {
		if got := normalizeMapSetting(input); got != want {
			t.Errorf("%q: got %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeSettingsRepairsInvalidEnumsAndBounds(t *testing.T) {
	got := normalizeSettings(config.Settings{Language: "xx", OCREngine: "cloud", GameMode: "unknown", SoundVolume: 500, RemoteID: " ABCD "})
	if got.Language != "ja" || got.OCREngine != "tesseract" || got.GameMode != "auto" || got.SoundVolume != 100 || got.RemoteID != "ABCD" || len(got.RemoteTargets) != 1 || !got.RemoteTargets[0].Map || !got.RemoteTargets[0].Tasks || got.RunThroughSeconds != 430 {
		t.Fatalf("settings were not normalized: %+v", got)
	}
}

func TestRemoteTargetRoutingAndDeduplication(t *testing.T) {
	settings := normalizeSettings(config.Settings{RemoteTargets: []config.RemoteTarget{
		{ID: " MAP ", Name: "Map display", Map: true},
		{ID: "TASK", Name: "Task display", Tasks: true},
		{ID: "MAP", Name: "duplicate", Tasks: true},
		{ID: " ", Name: "empty", Map: true},
	}})
	if len(settings.RemoteTargets) != 2 {
		t.Fatalf("targets were not normalized: %+v", settings.RemoteTargets)
	}
	if got := remoteTargetIDs(settings, "map"); len(got) != 1 || got[0] != "MAP" {
		t.Fatalf("map routing = %v", got)
	}
	if got := remoteTargetIDs(settings, "tasks"); len(got) != 1 || got[0] != "TASK" {
		t.Fatalf("task routing = %v", got)
	}
}
