package app

import (
	"context"
	"errors"
	"testing"

	"github.com/local/mayak/internal/tracker"
	"github.com/local/mayak/internal/trackerstore"
)

func TestTrackerRemoteNamesRefreshAndPreserveConcurrentChanges(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	a := &App{trackerData: trackerstore.Empty()}
	first, _ := a.trackerData.AddKey("pve", "PVE_one", "local name")
	second, _ := a.trackerData.AddKey("pvp", "PVP_two", "cached name")
	err := a.refreshTrackerKeyNames(func(_ context.Context, token string) (tracker.TokenInfo, error) {
		if token == second.Token {
			return tracker.TokenInfo{}, errors.New("offline")
		}
		// Simulate a binding change while metadata is in flight.
		a.trackerData.Keys[0].AccountID = "account"
		a.trackerData.Keys[0].ProfileID = "profile"
		return tracker.TokenInfo{Token: token, GameMode: "pve", Note: "remote name"}, nil
	})
	if err == nil {
		t.Fatal("partial failure not reported")
	}
	if a.trackerData.Keys[0].Name != "remote name" || a.trackerData.Keys[1].Name != "cached name" || !a.trackerData.Keys[0].IsBound() {
		t.Fatal("metadata refresh lost cached names or bindings")
	}
	loaded, err := a.trackerStore.Load()
	if err != nil || loaded.Keys[0].Name != "remote name" {
		t.Fatal("remote name not persisted", err)
	}
	_ = a.refreshTrackerKeyNames(func(_ context.Context, token string) (tracker.TokenInfo, error) {
		if token == first.Token {
			return tracker.TokenInfo{Token: token, GameMode: "pve", Note: ""}, nil
		}
		return tracker.TokenInfo{Token: "wrong identity", GameMode: "pvp", Note: "wrong"}, nil
	})
	if a.trackerData.Keys[0].Name != "" || a.trackerData.Keys[1].Name != "cached name" {
		t.Fatal("remote deletion or identity validation failed")
	}
	_ = a.refreshTrackerKeyNames(func(_ context.Context, token string) (tracker.TokenInfo, error) {
		if token == first.Token {
			_ = a.trackerData.RemoveKey(second.ID)
		}
		return tracker.TokenInfo{Token: token, GameMode: "pve", Note: "updated"}, nil
	})
	if len(a.trackerData.Keys) != 1 {
		t.Fatal("removed key resurrected")
	}
}
