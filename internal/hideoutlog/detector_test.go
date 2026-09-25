package hideoutlog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMonitorHistoryLiveRestartAndFallback(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "session")
	os.Mkdir(dir, 0700)
	path := filepath.Join(dir, "backend_queue_000.log")
	old := failure(time.Now().Add(-time.Hour), 1)
	os.WriteFile(path, []byte(old), 0600)
	os.WriteFile(filepath.Join(dir, "output_000.log"), []byte(old), 0600)
	events := make(chan Event, 20)
	d := New(root, func(e Event) { events <- e })
	d.Start(context.Background())
	defer d.Close()
	receive := func() Event {
		t.Helper()
		select {
		case e := <-events:
			return e
		case <-time.After(6 * time.Second):
			t.Fatal("no event")
			return Event{}
		}
	}
	if e := receive(); !e.Historical {
		t.Fatal("startup was live")
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString(failure(time.Now().Add(time.Second), 2))
	f.Close()
	if e := receive(); e.Historical {
		t.Fatal("new append was historical")
	}
	d.Close()
	select {
	case <-events:
		t.Fatal("fallback duplicated primary")
	default:
	}
	restarted := New(root, func(e Event) { events <- e })
	restarted.Start(context.Background())
	defer restarted.Close()
	if !receive().Historical || !receive().Historical {
		t.Fatal("restart replay was live")
	}
	restarted.Close()
	os.Remove(path)
	fallback := New(root, func(e Event) { events <- e })
	fallback.Start(context.Background())
	defer fallback.Close()
	if e := receive(); e.Source != "output_000.log" || !e.Historical {
		t.Fatal("fallback failed", e)
	}
}

// Opt-in local validation never writes EFT content to fixtures or diagnostics.
func TestRealEFTLogs(t *testing.T) {
	root := os.Getenv("MAYAK_EFT_LOGS")
	if root == "" {
		t.Skip("set MAYAK_EFT_LOGS for local verification")
	}
	counts := map[string]bool{}
	raw := 0
	unknown := 0
	dirs, _ := os.ReadDir(root)
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		path := filepath.Join(root, dir.Name())
		identities := timeline(path)
		entries, _ := os.ReadDir(path)
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) != ".log" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(path, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			var p Parser
			for _, e := range p.Feed(string(data)) {
				if e.Kind != "upgrade-failed" {
					continue
				}
				raw++
				for _, point := range identities {
					if !point.at.IsZero() && !point.at.After(e.OccurredAt) {
						e.Identity = point.identity
					}
				}
				if e.Mode == "" {
					unknown++
				}
				counts[e.Fingerprint()] = true
				if e.AreaType != 22 || e.Status != "failed" {
					t.Fatal("unexpected known failure")
				}
			}
		}
	}
	t.Logf("failed upgrade records=%d, logical events=%d, unknown identity records=%d", raw, len(counts), unknown)
	if raw != 12 || len(counts) != 4 {
		t.Fatalf("expected 12 duplicate records / 4 logical events; got %d / %d", raw, len(counts))
	}
}

func TestIdentityTimelineClearsProfileAcrossModeSwitch(t *testing.T) {
	dir := t.TempDir()
	text := "2026-09-21 10:00:00.000|Info|application|Session mode: Pve\n" +
		"2026-09-21 10:00:01.000|Info|application|SelectProfile ProfileId:abc AccountId:1\n" +
		"2026-09-21 11:00:00.000|Info|application|Session mode: PvpSeason\n" +
		"2026-09-21 11:00:01.000|Info|application|SelectProfile ProfileId:def AccountId:1\n"
	os.WriteFile(filepath.Join(dir, "application_000.log"), []byte(text), 0600)
	points := timeline(dir)
	if len(points) != 4 || points[1].identity.Mode != "pve" || points[2].identity.ProfileID != "" || points[3].identity.ProfileID != "def" || points[3].identity.Mode != "seasonal" {
		t.Fatal("mode switch reused previous profile", points)
	}
}
