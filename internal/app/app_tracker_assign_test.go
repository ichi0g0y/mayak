package app

import (
	"testing"

	"github.com/local/mayak/internal/trackerstore"
)

func TestAutoAssignTrackerKeyGoesToTheOnlyFreeProfileOrTheOnePlayed(t *testing.T) {
	document := trackerstore.Empty()
	document.Profiles = []trackerstore.Profile{{AccountID: "1", ProfileID: "p1", Mode: "pve"}, {AccountID: "1", ProfileID: "p2", Mode: "pvp"}}
	key, err := document.AddKey("pve", "PVE_token1")
	if err != nil {
		t.Fatal(err)
	}
	// One PvE profile without a key: the key goes there, whatever is played.
	assigned, to := autoAssignTrackerKey(&document, key, trackerstore.Profile{AccountID: "1", ProfileID: "p2", Mode: "pvp"})
	if !assigned || to.ProfileID != "p1" || document.TokenFor("1", "p1", "pve") != "PVE_token1" {
		t.Fatalf("assigned=%v to=%+v", assigned, to)
	}
	// A second PvE key: the only PvE profile has a key, so it stays free.
	second, _ := document.AddKey("pve", "PVE_token2")
	if assigned, _ := autoAssignTrackerKey(&document, second, trackerstore.Profile{}); assigned {
		t.Fatal("a key was put on a profile that has one")
	}
	// Two free regular profiles: the one being played gets the key; none
	// played among them, the key stays free.
	document.Profiles = append(document.Profiles, trackerstore.Profile{AccountID: "2", ProfileID: "p3", Mode: "pvp"})
	pvp, _ := document.AddKey("pvp", "PVP_token")
	if assigned, _ := autoAssignTrackerKey(&document, pvp, trackerstore.Profile{AccountID: "9", ProfileID: "px", Mode: "pvp"}); assigned {
		t.Fatal("assigned with two free profiles and none played")
	}
	assigned, to = autoAssignTrackerKey(&document, pvp, trackerstore.Profile{AccountID: "2", ProfileID: "p3", Mode: "pvp"})
	if !assigned || to.ProfileID != "p3" || document.TokenFor("2", "p3", "pvp") != "PVP_token" {
		t.Fatalf("assigned=%v to=%+v", assigned, to)
	}
	// An account's earlier profile of the mode (a past wipe) does not count:
	// with the latest one free, the key goes there although two are free.
	document.Profiles = []trackerstore.Profile{{AccountID: "3", ProfileID: "old", Mode: "seasonal", LastSeen: "2026-01-01T00:00:00Z"}, {AccountID: "3", ProfileID: "new", Mode: "seasonal", LastSeen: "2026-09-01T00:00:00Z"}}
	season, _ := document.AddKey("seasonal", "SZN_token")
	if assigned, to := autoAssignTrackerKey(&document, season, trackerstore.Profile{}); !assigned || to.ProfileID != "new" {
		t.Fatalf("assigned=%v to=%+v", assigned, to)
	}
}
