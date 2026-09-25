package trackerlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentityParserCombinesModeAndProfile(t *testing.T) {
	parser := &IdentityParser{}
	events := parser.Parse("2026-09-13|Info|application|Session mode: PvpSeason\n" +
		"2026-09-13|Info|application|PrepareSelectedProfileLocally ProfileId:abc123 AccountId:7090035\n")
	if len(events) != 1 || events[0].Mode != "seasonal" || events[0].ProfileID != "abc123" || events[0].AccountID != "7090035" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestTaskParserHandlesChunkedNotification(t *testing.T) {
	parser := &TaskParser{}
	first := "2026|push-notifications|Got notification | ChatMessageReceived\n{\"eventId\":\"evt-1\",\"message\":{\"type\":12,"
	if got := parser.Feed(first); len(got) != 0 {
		t.Fatalf("unexpected early events: %+v", got)
	}
	got := parser.Feed("\"templateId\":\"5a27b87686f77460de0252a8 successMessageText\"}}\n")
	if len(got) != 1 || got[0].TaskID != "5a27b87686f77460de0252a8" || got[0].TaskState != "completed" {
		t.Fatalf("unexpected events: %+v", got)
	}
}

func TestTaskParserMapsAllStatesAndDeduplicates(t *testing.T) {
	parser := &TaskParser{}
	text := ""
	for _, item := range []struct {
		id    string
		typeN int
	}{{"a", 10}, {"b", 11}, {"c", 12}} {
		text += "Got notification | ChatMessageReceived\n{\"eventId\":\"" + item.id + "\",\"message\":{\"type\":" + string(rune('0'+item.typeN/10)) + string(rune('0'+item.typeN%10)) + ",\"templateId\":\"5a27b87686f77460de0252a8 x\"}}\n"
	}
	got := parser.Feed(text)
	if len(got) != 3 || got[0].TaskState != "uncompleted" || got[1].TaskState != "failed" || got[2].TaskState != "completed" {
		t.Fatalf("unexpected states: %+v", got)
	}
	if duplicate := parser.Feed("Got notification | ChatMessageReceived\n{\"eventId\":\"c\",\"message\":{\"type\":12,\"templateId\":\"5a27b87686f77460de0252a8 x\"}}\n"); len(duplicate) != 0 {
		t.Fatalf("duplicate emitted: %+v", duplicate)
	}
}

func TestDiscoverProfilesAcrossExistingSessions(t *testing.T) {
	root := t.TempDir()
	for index, entry := range []struct{ mode, account, profile string }{{"Pve", "100", "aaaaaaaaaaaaaaaaaaaaaaaa"}, {"PvpSeason", "200", "bbbbbbbbbbbbbbbbbbbbbbbb"}} {
		dir := filepath.Join(root, "log-session-"+string(rune('a'+index)))
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		text := "2026|Info|application|Session mode: " + entry.mode + "\n2026|Info|application|PrepareSelectedProfileLocally ProfileId:" + entry.profile + " AccountId:" + entry.account + "\n"
		if err := os.WriteFile(filepath.Join(dir, "application_000.log"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	profiles := DiscoverProfiles(root)
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %+v", profiles)
	}
	modes := map[string]bool{}
	for _, profile := range profiles {
		modes[profile.Mode] = true
	}
	if !modes["pve"] || !modes["seasonal"] {
		t.Fatalf("unexpected modes: %+v", modes)
	}
}
