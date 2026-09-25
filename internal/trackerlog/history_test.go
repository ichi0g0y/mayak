package trackerlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryUsesOnlyExactProfileAndSelectedBreakpoint(t *testing.T) {
	root := t.TempDir()
	writeSession(t, root, "log_2026.01.01_10-00-00_1.0.0", "p1", "10", "Pve", `{"eventId":"a","message":{"type":10,"templateId":"aaaaaaaaaaaaaaaaaaaaaaaa"}}`)
	writeSession(t, root, "log_2026.01.02_10-00-00_1.0.0", "other", "10", "Pve", `{"eventId":"b","message":{"type":12,"templateId":"bbbbbbbbbbbbbbbbbbbbbbbb"}}`)
	writeSession(t, root, "log_2026.01.03_10-00-00_1.1.0", "p1", "10", "Pve", `{"eventId":"c","message":{"type":12,"templateId":"aaaaaaaaaaaaaaaaaaaaaaaa"}}`)

	points := HistoryBreakpoints(root, "10", "p1", "pve")
	if len(points) != 2 || points[1].Version != "1.1.0" {
		t.Fatalf("unexpected breakpoints: %#v", points)
	}
	states, err := TaskHistory(root, points[1].ID, "10", "p1", "pve")
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || states["aaaaaaaaaaaaaaaaaaaaaaaa"] != "completed" {
		t.Fatalf("unexpected states: %#v", states)
	}
}

func TestHistoryRejectsBreakpointFromAnotherProfile(t *testing.T) {
	root := t.TempDir()
	writeSession(t, root, "log_2026.01.01_10-00-00_1.0.0", "p2", "20", "Regular", `{}`)
	if _, err := TaskHistory(root, "log_2026.01.01_10-00-00_1.0.0", "10", "p1", "pvp"); err == nil {
		t.Fatal("expected profile mismatch")
	}
}

func writeSession(t *testing.T, root, name, profile, account, mode, payload string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	application := "2026-01-01 10:00:00.000|x|Info|application|Session mode: " + mode + "\n" +
		"2026-01-01 10:00:01.000|x|Info|application|PrepareSelectedProfileLocally ProfileId:" + profile + " AccountId:" + account
	if err := os.WriteFile(filepath.Join(dir, "application_000.log"), []byte(application), 0600); err != nil {
		t.Fatal(err)
	}
	notification := "2026-01-01 10:00:02.000|x|Info|push-notifications|Got notification | ChatMessageReceived\n" + payload
	if err := os.WriteFile(filepath.Join(dir, "push-notifications_000.log"), []byte(notification), 0600); err != nil {
		t.Fatal(err)
	}
}
