package trackerlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	// The notification is stamped with the session's day (log_2026.01.03_… → 2026-01-03).
	day := strings.ReplaceAll(strings.SplitN(strings.TrimPrefix(name, "log_"), "_", 2)[0], ".", "-")
	notification := day + " 10:00:02.000|x|Info|push-notifications|Got notification | ChatMessageReceived\n" + payload
	if err := os.WriteFile(filepath.Join(dir, "push-notifications_000.log"), []byte(notification), 0600); err != nil {
		t.Fatal(err)
	}
}

// A profile's history from its first session, read in one pass: the later
// state of a task wins, and another profile's sessions are left out.
func TestProfileTaskHistoryFromTheFirstSession(t *testing.T) {
	root := t.TempDir()
	writeSession(t, root, "log_2026.01.01_10-00-00_1.0.0", "p1", "10", "Pve", `{"eventId":"a","message":{"type":12,"templateId":"cccccccccccccccccccccccc"}}`)
	writeSession(t, root, "log_2026.01.02_10-00-00_1.0.0", "other", "10", "Pve", `{"eventId":"b","message":{"type":12,"templateId":"bbbbbbbbbbbbbbbbbbbbbbbb"}}`)
	writeSession(t, root, "log_2026.01.03_10-00-00_1.1.0", "p1", "10", "Pve", `{"eventId":"c","message":{"type":12,"templateId":"aaaaaaaaaaaaaaaaaaaaaaaa"}}`)
	states, sessions, _ := ProfileTaskHistory(root, "10", "p1", "pve", time.Time{})
	if sessions != 2 {
		t.Fatalf("read %d sessions, want the profile's 2", sessions)
	}
	if len(states) != 2 || states["cccccccccccccccccccccccc"] != "completed" || states["aaaaaaaaaaaaaaaaaaaaaaaa"] != "completed" {
		t.Fatalf("unexpected states: %#v", states)
	}
	if _, none, _ := ProfileTaskHistory(root, "10", "missing", "pve", time.Time{}); none != 0 {
		t.Fatalf("an unknown profile read %d sessions", none)
	}
	// From a day on (a Prestige): the sessions before it are left out.
	states, _, _ = ProfileTaskHistory(root, "10", "p1", "pve", time.Date(2026, 1, 3, 0, 0, 0, 0, time.Local))
	if len(states) != 1 || states["aaaaaaaaaaaaaaaaaaaaaaaa"] != "completed" {
		t.Fatalf("from a day: %#v", states)
	}
}

// A Prestige (EFT 1.2.0.0) keeps the profile's ID and resets its progress:
// the task changes before it, in the same session too, are left out, and
// a Prestige in another mode changes nothing.
func TestProfileTaskHistoryAfterPrestige(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "log_2026.10.07_7-03-16_1.2.0.0.47888")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	application := "2026-10-07 07:04:00.000|1.2.0.0.47888|Info|application|Session mode: Pve\n" +
		"2026-10-07 07:04:01.000|1.2.0.0.47888|Info|application|PrepareSelectedProfileLocally ProfileId:p1 AccountId:10\n"
	notification := func(at, event, task string) string {
		return at + "|1.2.0.0.47888|Info|output|backend|WebSocketSharp - message received: NOTIFICATION " + event + " new_message [{\"eventId\":\"" + event + "\",\"message\":{\"type\":12,\"templateId\":\"" + task + " x\"}}]\n"
	}
	obtained := func(at, gateway string) string {
		return at + "|1.2.0.0.47888|Info|output|backend|<--- Response HTTPS, id [472]: URL: https://gw-" + gateway + ".escapefromtarkov.com/client/prestige/obtain, DownloadSeconds: 0.612, responseText: \n"
	}
	output := notification("2026-10-07 07:50:00.000", "e1", "aaaaaaaaaaaaaaaaaaaaaaaa") +
		obtained("2026-10-07 08:00:00.000", "pvp") +
		notification("2026-10-07 08:10:00.000", "e2", "bbbbbbbbbbbbbbbbbbbbbbbb") +
		obtained("2026-10-07 08:27:10.296", "pve") +
		notification("2026-10-07 08:40:00.000", "e3", "cccccccccccccccccccccccc")
	if err := os.WriteFile(filepath.Join(dir, "application_000.log"), []byte(application), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "output_000.log"), []byte(output), 0600); err != nil {
		t.Fatal(err)
	}
	states, sessions, prestige := ProfileTaskHistory(root, "10", "p1", "pve", time.Time{})
	want := time.Date(2026, 10, 7, 8, 27, 10, 296000000, time.Local)
	if sessions != 1 || !prestige.Equal(want) {
		t.Fatalf("%d sessions, prestige %v", sessions, prestige)
	}
	if len(states) != 1 || states["cccccccccccccccccccccccc"] != "completed" {
		t.Fatalf("after the Prestige: %#v", states)
	}
}

// A session can switch mode (2026-10-07: PvE, Seasonal, then PvE again): a
// notification belongs to the profile played when it came, not to every
// profile the session played.
func TestProfileTaskHistorySplitsASessionByMode(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "log_2026.10.07_7-03-16_1.2.0.0.47888")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	line := func(at, text string) string {
		return "2026-10-07 " + at + "|1.2.0.0.47888|Info|application|" + text + "\n"
	}
	application := line("07:03:25.908", "Session mode: Pve") +
		line("07:04:22.946", "CompleteSelectedProfile ProfileId:pve1 AccountId:70") +
		line("08:07:02.363", "Session mode: PvpSeason") +
		line("08:07:09.485", "CompleteSelectedProfile ProfileId:szn1 AccountId:70") +
		line("08:09:33.532", "Session mode: Pve") +
		line("08:09:41.268", "CompleteSelectedProfile ProfileId:pve1 AccountId:70")
	notification := func(at, event, task string) string {
		return "2026-10-07 " + at + "|1.2.0.0.47888|Info|output|backend|WebSocketSharp - message received: NOTIFICATION " + event + " new_message [{\"eventId\":\"" + event + "\",\"message\":{\"type\":12,\"templateId\":\"" + task + " x\"}}]\n"
	}
	output := notification("07:50:00.000", "e1", "aaaaaaaaaaaaaaaaaaaaaaaa") +
		notification("08:08:00.000", "e2", "bbbbbbbbbbbbbbbbbbbbbbbb") +
		notification("08:08:59.000", "e3", "dddddddddddddddddddddddd") +
		notification("08:40:00.000", "e4", "cccccccccccccccccccccccc")
	if err := os.WriteFile(filepath.Join(dir, "application_000.log"), []byte(application), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "output_000.log"), []byte(output), 0600); err != nil {
		t.Fatal(err)
	}
	pve, sessions, _ := ProfileTaskHistory(root, "70", "pve1", "pve", time.Time{})
	if sessions != 1 || len(pve) != 2 || pve["aaaaaaaaaaaaaaaaaaaaaaaa"] == "" || pve["cccccccccccccccccccccccc"] == "" {
		t.Fatalf("PvE: %d sessions, %#v", sessions, pve)
	}
	seasonal, _, _ := ProfileTaskHistory(root, "70", "szn1", "seasonal", time.Time{})
	if len(seasonal) != 2 || seasonal["bbbbbbbbbbbbbbbbbbbbbbbb"] == "" || seasonal["dddddddddddddddddddddddd"] == "" {
		t.Fatalf("Seasonal: %#v", seasonal)
	}
}
