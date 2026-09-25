package app

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/remote"
)

func TestBrowserRemoteIDIsGeneratedOnce(t *testing.T) {
	var s config.Settings
	if !ensureBrowserRemoteID(&s) || !browserRemoteIDPattern.MatchString(s.BrowserRemoteID) {
		t.Fatalf("generated ID = %q", s.BrowserRemoteID)
	}
	id := s.BrowserRemoteID
	if ensureBrowserRemoteID(&s) || s.BrowserRemoteID != id {
		t.Fatalf("ID changed from %q to %q", id, s.BrowserRemoteID)
	}
}

func TestShortBrowserRemoteIDIsReplaced(t *testing.T) {
	s := config.Settings{BrowserRemoteID: "PANB"}
	if !ensureBrowserRemoteID(&s) || len(s.BrowserRemoteID) != browserRemoteIDLength {
		t.Fatalf("ID = %q", s.BrowserRemoteID)
	}
}

func TestBrowserRemoteTellsItsOwnCommandsFromOthers(t *testing.T) {
	var a App
	sent, _ := remote.MapPayload("AB12CD34EF56", "customs")
	a.noteBrowserRemoteSend(remote.CommandKey(sent))
	// The server relays the command without the session ID.
	relayed := []byte(`{"type":"command","data":{"type":"map","value":"customs"}}`)
	if !a.browserRemoteSentRecently(remote.CommandKey(relayed)) {
		t.Fatal("own command taken for someone else's")
	}
	other := []byte(`{"type":"command","data":{"type":"map","value":"woods"}}`)
	if a.browserRemoteSentRecently(remote.CommandKey(other)) {
		t.Fatal("someone else's command taken for own")
	}
	a.browserRemoteSent[remote.CommandKey(relayed)] = time.Now().Add(-2 * browserRemoteEcho)
	if a.browserRemoteSentRecently(remote.CommandKey(relayed)) {
		t.Fatal("an old command still counts as own")
	}
}

func TestBrowserRemoteIDReceivesMapButNotTasks(t *testing.T) {
	s := config.Settings{BrowserRemoteID: "AB12", RemoteTargets: []config.RemoteTarget{{ID: "USER", Map: true, Tasks: true}}}
	if got := remoteTargetIDs(s, "map"); !slices.Equal(got, []string{"USER", "AB12"}) {
		t.Fatalf("map targets = %v", got)
	}
	if got := remoteTargetIDs(s, "all"); !slices.Equal(got, []string{"USER", "AB12"}) {
		t.Fatalf("all targets = %v", got)
	}
	if got := remoteTargetIDs(s, "tasks"); !slices.Equal(got, []string{"USER"}) {
		t.Fatalf("task targets = %v", got)
	}
	s.RemoteTargets = []config.RemoteTarget{{ID: "AB12", Map: true}}
	if got := remoteTargetIDs(s, "map"); !slices.Equal(got, []string{"AB12"}) {
		t.Fatalf("duplicate target = %v", got)
	}
}

func TestTarkovDevConnectScriptTargetsMapPages(t *testing.T) {
	script := tarkovDevConnectScript("AB12")
	for _, want := range []string{`"tarkov.dev"`, `"/map/"`, `"/maps/"`, `"connection"`, `"sessionId"`, `"AB12"`} {
		if !strings.Contains(script, want) {
			t.Fatalf("script lacks %s: %s", want, script)
		}
	}
}
