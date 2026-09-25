package logdetect

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSceneMap(t *testing.T) {
	got, ok := ParseMap("application|scene preset path:maps/bigmap.bundle")
	if !ok || got != "customs" {
		t.Fatalf("got %q %v", got, ok)
	}
}
func TestParseLocationMap(t *testing.T) {
	got, ok := ParseMap("TRACE-NetworkGameCreate profileStatus Location: TarkovStreets, RaidMode: Online")
	if !ok || got != "streets-of-tarkov" {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestParsePresetMap(t *testing.T) {
	got, ok := ParseMap("scene preset path:maps/customs_preset.bundle")
	if !ok || got != "customs" {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestParseFallsBackFromUnknownSceneToLocation(t *testing.T) {
	got, ok := ParseMap("scene preset path:maps/future_map.bundle\nLocation: bigmap")
	if !ok || got != "customs" {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestParseDoesNotFallBackToStaleLocationBeforeUnknownNewScene(t *testing.T) {
	got, ok := ParseMap("Location: TarkovStreets\nscene preset path:maps/future_map.bundle")
	if ok || got != "" {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestParseCurrentScenePaths(t *testing.T) {
	cases := map[string]string{
		"city":            "streets-of-tarkov",
		"factory_day":     "factory",
		"factory_night":   "night-factory",
		"shopping_mall":   "interchange",
		"rezerv_base":     "reserve",
		"laboratory":      "the-lab",
		"laboratory_dark": "the-lab-dark",
	}
	for scene, want := range cases {
		got, ok := ParseMap("scene preset path:maps/" + scene + "_preset.bundle")
		if !ok || got != want {
			t.Errorf("%s: got %q %v, want %q", scene, got, ok, want)
		}
	}
}

func TestMapUpdateRepeatsDetectedMapWhenRaidStarts(t *testing.T) {
	mapName, notify := mapUpdate("2026-09-01|application|GameStarted:54.06", "customs")
	if !notify || mapName != "customs" {
		t.Fatalf("got %q %v", mapName, notify)
	}
}

func TestMapUpdateDoesNotNotifyRaidStartWithoutDetectedMap(t *testing.T) {
	mapName, notify := mapUpdate("2026-09-01|application|GameStarted:54.06", "")
	if notify || mapName != "" {
		t.Fatalf("got %q %v", mapName, notify)
	}
}

func TestMapUpdateUsesNewSceneBeforeRaidStart(t *testing.T) {
	mapName, notify := mapUpdate("scene preset path:maps/woods.bundle\n2026-09-01|application|GameStarted:54.06", "customs")
	if !notify || mapName != "woods" {
		t.Fatalf("got %q %v", mapName, notify)
	}
}

func TestParseEventsKeepsLogOrder(t *testing.T) {
	text := "2026-09-01 18:55:00.000|application|MatchingCompleted:9.81 real:16.21 diff:6.40\n" +
		"2026-09-01 18:55:01.000|application|GameStarting:51.64\n" +
		"2026-09-01 18:55:06.000|application|GameStarted:51.64\n"
	got := (&EventParser{}).Parse(text)
	if len(got) != 2 || got[0].Kind != MatchFound || got[0].QueueSeconds != 16.21 || got[1].Kind != RaidStarted || !got[1].RunThroughEligible {
		t.Fatalf("unexpected events: %v", got)
	}
}

func TestParseEventsIgnoresStackTraceNames(t *testing.T) {
	got := (&EventParser{}).Parse("EFT.<GameStartingAsync>d__116:MoveNext()\nEFT.MatchingOperation:Execute()")
	if len(got) != 0 {
		t.Fatalf("unexpected events: %v", got)
	}
}

func TestEventParserDoesNotStartRunThroughTimerForScavTiming(t *testing.T) {
	parser := &EventParser{}
	_ = parser.Parse("2026-09-01 18:55:05.000|application|GameStarting:51.64\n")
	got := parser.Parse("2026-09-01 18:55:06.000|application|GameStarted:51.64\n")
	if len(got) != 1 || got[0].Kind != RaidStarted || got[0].RunThroughEligible {
		t.Fatalf("unexpected events: %v", got)
	}
}

func TestLatestSnapshotRestoresActiveRaidAndQueue(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "log_2026.09.01_18-00-00_1.0.0")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	application := "2026-09-01 18:55:00.000|application|MatchingCompleted:9.81 real:16.21 diff:6.40\n" +
		"2026-09-01 18:55:01.000|application|GameStarting:51.64\n" +
		"2026-09-01 18:55:06.000|application|GameStarted:51.64\n"
	if err := os.WriteFile(filepath.Join(dir, "application_000.log"), []byte(application), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := LatestSnapshot(root)
	if !snapshot.Active || snapshot.LastQueueSeconds != 16.21 || !snapshot.RunThroughEligible || snapshot.StartedAt.IsZero() {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestRaidStateUsesLatestEvent(t *testing.T) {
	dir := t.TempDir()
	application := filepath.Join(dir, "application_000.log")
	backend := filepath.Join(dir, "backend_000.log")
	if err := os.WriteFile(application, []byte("2026-09-01 18:55:06.772|x|Info|application|GameStarted:54.06\n"), 0600); err != nil {
		t.Fatal(err)
	}
	active, known := RaidState(dir)
	if !known || !active {
		t.Fatalf("after start: active=%v known=%v", active, known)
	}
	if err := os.WriteFile(backend, []byte("2026-09-01 19:12:39.090|x|Info|backend|userMatchOver []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	active, known = RaidState(dir)
	if !known || active {
		t.Fatalf("after exit: active=%v known=%v", active, known)
	}
}

func TestRaidStateEndsWhenPveReturnsToProfile(t *testing.T) {
	dir := t.TempDir()
	application := filepath.Join(dir, "application_000.log")
	contents := "2026-09-10 15:43:38.370|x|Info|application|GameStarted:68.86\n" +
		"2026-09-10 16:11:24.788|x|Info|application|PrepareSelectedProfileLocally ProfileId:abc AccountId:123\n"
	if err := os.WriteFile(application, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	active, known := RaidState(dir)
	if !known || active {
		t.Fatalf("after PvE profile reload: active=%v known=%v", active, known)
	}
}

func TestEventParserReportsPveRaidExit(t *testing.T) {
	parser := &EventParser{}
	_ = parser.Parse("2026-09-10 15:43:38.370|x|Info|application|GameStarted:68.86\n")
	got := parser.Parse("2026-09-10 16:11:24.788|x|Info|application|PrepareSelectedProfileLocally ProfileId:abc AccountId:123\n")
	if len(got) != 1 || got[0].Kind != RaidExited {
		t.Fatalf("unexpected events: %v", got)
	}
}

func TestEventParserReportsRaidExitOnlyOnce(t *testing.T) {
	parser := &EventParser{}
	_ = parser.Parse("2026-09-10 15:43:38.370|x|Info|application|GameStarted:68.86\n")
	got := parser.Parse("2026-09-10 16:11:24.788|x|Info|application|PrepareSelectedProfileLocally ProfileId:abc AccountId:123\n" +
		"2026-09-10 16:11:25.106|x|Info|application|CompleteSelectedProfile ProfileId:abc AccountId:123\n")
	if len(got) != 1 || got[0].Kind != RaidExited {
		t.Fatalf("unexpected events: %v", got)
	}
	if repeated := parser.Parse("2026-09-10 16:11:52.015|x|Info|application|PrepareSelectedProfileLocally ProfileId:abc AccountId:123\n"); len(repeated) != 0 {
		t.Fatalf("duplicate exit events: %v", repeated)
	}
}

func TestLatestSessionLogsStayWithinNewestApplicationSession(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "log_old")
	newDir := filepath.Join(root, "log_new")
	if err := os.MkdirAll(oldDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0700); err != nil {
		t.Fatal(err)
	}
	oldApplication := filepath.Join(oldDir, "application_000.log")
	newApplication := filepath.Join(newDir, "application_000.log")
	newBackend := filepath.Join(newDir, "backend_000.log")
	for _, path := range []string{oldApplication, newApplication, newBackend} {
		if err := os.WriteFile(path, []byte("log"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldApplication, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	got := latestSessionLogs(root)
	if len(got) != 2 || !containsPath(got, newApplication) || !containsPath(got, newBackend) {
		t.Fatalf("unexpected files: %v", got)
	}
}

func TestLatestMapMovesToApplicationLogCreatedAfterSessionDirectory(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "log_old")
	newDir := filepath.Join(root, "log_new")
	if err := os.MkdirAll(oldDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0700); err != nil {
		t.Fatal(err)
	}
	oldApplication := filepath.Join(oldDir, "application_000.log")
	if err := os.WriteFile(oldApplication, []byte("scene preset path:maps/rezerv_base_preset.bundle\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldApplication, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if got, ok := LatestMap(root); !ok || got != "reserve" {
		t.Fatalf("before new application log: got %q %v", got, ok)
	}

	newApplication := filepath.Join(newDir, "application_000.log")
	if err := os.WriteFile(newApplication, []byte("scene preset path:maps/factory_day_preset.bundle\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, ok := LatestMap(root); !ok || got != "factory" {
		t.Fatalf("after new application log: got %q %v", got, ok)
	}
}

func containsPath(paths []string, wanted string) bool {
	for _, path := range paths {
		if path == wanted {
			return true
		}
	}
	return false
}
