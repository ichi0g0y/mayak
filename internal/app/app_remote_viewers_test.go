package app

import (
	"slices"
	"testing"

	"github.com/local/mayak/internal/config"
)

func TestViewerTargetIDsAreEachRegisteredIDOnce(t *testing.T) {
	s := config.Settings{
		RemoteTargets: []config.RemoteTarget{{ID: "TAB1", Map: true}, {ID: "TAB1", Tasks: true}, {ID: ""}, {ID: "TAB2", Tasks: true}},
	}
	if got := viewerTargetIDs(s); !slices.Equal(got, []string{"TAB1", "TAB2"}) {
		t.Fatalf("ids=%v", got)
	}
}

func TestRemoteTargetIDsByRole(t *testing.T) {
	s := config.Settings{RemoteTargets: []config.RemoteTarget{{ID: "BOTH", Map: true, Tasks: true}, {ID: "MAP", Map: true}, {ID: "TASKS", Tasks: true}}}
	for channel, want := range map[string][]string{"map": {"BOTH", "MAP"}, "tasks": {"BOTH", "TASKS"}, "all": {"BOTH", "MAP", "TASKS"}} {
		if got := remoteTargetIDs(s, channel); !slices.Equal(got, want) {
			t.Fatalf("%s targets = %v", channel, got)
		}
	}
}
