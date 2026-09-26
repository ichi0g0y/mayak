package hideoutlog

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func failure(stamp time.Time, ts int64) string {
	return fmt.Sprintf("%s|version|Error|backend_queue|Error: Inventory queue failed on the following commands:\n[\n {\n  \"Action\": \"HideoutUpgradeComplete\",\n  \"areaType\": 22,\n  \"timestamp\": %d,\n  \"token\": \"secret-do-not-store\"\n }\n]\n", stamp.Format("2006-01-02 15:04:05.000"), ts)
}

func TestChunkSafeFailuresAndSanitization(t *testing.T) {
	text := failure(time.Now(), 123)
	for split := 0; split <= len(text); split++ {
		var p Parser
		got := p.Feed(text[:split])
		got = append(got, p.Feed(text[split:])...)
		if len(got) != 1 || got[0].Kind != "upgrade-failed" || got[0].Status != "failed" || got[0].AreaType != 22 || got[0].ActionTimestamp != 123 {
			t.Fatalf("split %d: %+v", split, got)
		}
		data, _ := json.Marshal(got)
		if strings.Contains(string(data), "secret") || strings.Contains(string(data), "token") {
			t.Fatal("raw payload leaked")
		}
	}
	var p Parser
	var got []Event
	for _, b := range []byte(text) {
		got = append(got, p.Feed(string([]byte{b}))...)
	}
	if len(got) != 1 {
		t.Fatal("byte-at-a-time parsing failed")
	}
}

func TestStartupUnknownAndRecovery(t *testing.T) {
	var p Parser
	if got := p.Feed("StartLoadHideoutBundles\nHideoutGame:SessionRun\nHideoutAreaLevel:Enable\nOnHideoutStart\n"); len(got) != 0 {
		t.Fatal(got)
	}
	got := p.Feed("2026-09-21 10:00:00.000|Info|request /client/hideout/areas?token=secret https://secret-gateway.invalid\n")
	if len(got) != 1 || got[0].Action != "/client/hideout/areas" || got[0].Status != "info" {
		t.Fatal(got)
	}
	got = p.Feed("2026-09-21 10:00:01.000|Info|response /client/hideout/areas responseText:\n")
	if len(got) != 1 || got[0].Kind != "metadata-response" {
		t.Fatal(got)
	}
	got = p.Feed("2026-09-21 10:00:02.000|Info|backend|{\"Action\":\"HideoutUpgradeComplete\",\"areaType\":22}\n")
	if len(got) != 1 || got[0].Status != "unknown" || got[0].Actionable() {
		t.Fatal("unconfirmed command became completion or error", got)
	}
	p.Feed("[" + strings.Repeat("x", maxBuffer*2))
	got = p.Feed("\n" + failure(time.Now(), 7))
	if len(got) != 1 {
		t.Fatal("parser did not recover from oversized document")
	}
}

func TestDedupePersistenceIsolationBounds(t *testing.T) {
	path := t.TempDir() + "/events.json"
	s := NewStore(path)
	now := time.Now()
	for file := 0; file < 3; file++ {
		for i := 0; i < 4; i++ {
			e := Event{Identity: Identity{"a", "p", "pve"}, Kind: "upgrade-failed", Action: "HideoutUpgradeComplete", AreaType: 22, ActionTimestamp: int64(i + 1), OccurredAt: now, Source: fmt.Sprint(file)}
			if added := s.Add(e); added != (file == 0) {
				t.Fatalf("dedupe %d/%d %v", file, i, added)
			}
		}
	}
	if len(s.Events()) != 4 {
		t.Fatal("12 records did not become four")
	}
	// The writes are coalesced: the file holds the events once flushed.
	if _, err := os.Stat(path); err == nil {
		t.Fatal("written before the flush")
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	restored := NewStore(path)
	e := s.Events()[0]
	if restored.Add(e) {
		t.Fatal("restart replay")
	}
	for _, identity := range []Identity{{"a", "p", "pvp"}, {"a", "p", "seasonal"}, {"a", "other", "pve"}, {"other", "p", "pve"}} {
		e.Identity = identity
		if !s.Add(e) {
			t.Fatal("identities mixed")
		}
	}
	e.OccurredAt = now.Add(-HistoryAge - time.Hour)
	if s.Add(e) {
		t.Fatal("expired event retained")
	}
	memory := NewStore("")
	for i := 0; i < HistoryLimit+20; i++ {
		e.ActionTimestamp = int64(i)
		e.OccurredAt = now.Add(time.Duration(i) * time.Millisecond)
		memory.Add(e)
	}
	events := memory.Events()
	if len(events) != HistoryLimit || !events[0].OccurredAt.After(events[len(events)-1].OccurredAt) {
		t.Fatal("history unbounded or unsorted")
	}
}

func TestCompleteJSONWithoutTrailingNewline(t *testing.T) {
	var p Parser
	events := p.Feed(strings.TrimSuffix(failure(time.Now(), 42), "\n"))
	if len(events) != 1 || events[0].Kind != "upgrade-failed" {
		t.Fatal("complete JSON was left pending", events)
	}
}
func TestClearKeepsReplayProtection(t *testing.T) {
	path := t.TempDir() + "/events.json"
	store := NewStore(path)
	e := Event{Action: "HideoutUpgradeComplete", ActionTimestamp: 1, OccurredAt: time.Now()}
	store.Add(e)
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	restored := NewStore(path)
	if len(restored.Events()) != 0 {
		t.Fatal("cleared rows restored")
	}
	if restored.Add(e) {
		t.Fatal("clearing history allowed replay")
	}
}
