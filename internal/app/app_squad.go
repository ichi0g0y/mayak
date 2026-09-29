package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/catalog"
	"github.com/local/mayak/internal/locale"
	"github.com/local/mayak/internal/mapdata"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/squad"
	"github.com/local/mayak/internal/version"
)

// The squad room (internal/squad) and the squad map's geometry
// (internal/mapdata). The shell keeps the squad code (browser.json) and the
// display name (browser preferences) and joins again at start; here the
// room is only joined, reported to and left. Every change of the room goes
// to the shell as "squad:state".
//
// It is a nightly feature for now: a release build (version.IsPrerelease
// false) refuses to join and the shell does not offer it.

var (
	squadJoinMu sync.Mutex
	squadMu     sync.Mutex
	squadClient *squad.Client
	// squadColor is the squad colour chosen in the shell (SquadSetColor),
	// sent with this PC's reports; guarded by squadMu.
	squadColor string

	squadMapsOnce sync.Once
	squadMaps     *mapdata.Source
)

var errSquadOff = errors.New("squads are not available in this version")

// squadEndpoint is the relay; MAYAK_SQUAD_RELAY points a development build
// at another one (`task relay:dev`: ws://127.0.0.1:8787/squad/).
func squadEndpoint() string {
	if v := os.Getenv("MAYAK_SQUAD_RELAY"); v != "" && version.IsPrerelease() {
		return v
	}
	return squad.Endpoint
}

// SquadAvailable reports whether this build offers squads.
func (a *App) SquadAvailable() bool { return version.IsPrerelease() }

// SquadNewCode returns a fresh squad code ("ABCD-1234") to create a squad
// with.
func (a *App) SquadNewCode() string { return squad.Format(squad.NewCode()) }

// SquadJoin joins the squad of code as name, leaving the one joined before.
// It returns the code in its canonical form ("ABCD-1234").
func (a *App) SquadJoin(code, name string) (string, error) {
	if !version.IsPrerelease() {
		return "", errSquadOff
	}
	canonical, err := squad.Normalize(code)
	if err != nil {
		return "", err
	}
	// Joins and leaves one at a time; squadMu only guards squadClient, and is
	// never held while the client runs its change callback (which takes it):
	// holding it across Report deadlocked a join again to the same squad.
	squadJoinMu.Lock()
	defer squadJoinMu.Unlock()
	squadMu.Lock()
	old := squadClient
	if old != nil && old.Code() != canonical {
		squadClient = nil
	}
	squadMu.Unlock()
	if old != nil {
		if old.Code() == canonical {
			old.Report(a.squadReport(name))
			return squad.Format(canonical), nil
		}
		old.Close()
	}
	var client *squad.Client
	// The connection's phase as last logged, to log each change once.
	phase := ""
	client, err = squad.Join(canonical, squadEndpoint(), version.UserAgent(), a.squadReport(name), func(s squad.State) {
		squadMu.Lock()
		current := client != nil && squadClient == client
		changed := current && s.Phase != phase
		if changed {
			phase = s.Phase
		}
		squadMu.Unlock()
		if !current {
			return
		}
		if changed {
			switch s.Phase {
			case squad.PhaseConnected:
				a.addLog("Info", "Squad", "Connected to the squad relay")
			case squad.PhaseOffline:
				a.addLog("Warn", "Squad", "Lost the squad relay; trying again")
			case squad.PhaseFull:
				a.addLog("Warn", "Squad", "The squad is full")
			}
		}
		a.emitEvent("squad:state", s)
	})
	if err != nil {
		return "", err
	}
	squadMu.Lock()
	squadClient = client
	squadMu.Unlock()
	a.addLog("Info", "Squad", "Joined a squad")
	go func() {
		select {
		case <-a.done:
			a.SquadLeave()
		case <-client.Done():
		}
	}()
	a.emitEvent("squad:state", client.State())
	return squad.Format(canonical), nil
}

// SquadLeave leaves the squad.
func (a *App) SquadLeave() {
	squadJoinMu.Lock()
	defer squadJoinMu.Unlock()
	squadMu.Lock()
	client := squadClient
	squadClient = nil
	squadMu.Unlock()
	if client != nil {
		client.Close()
		a.addLog("Info", "Squad", "Left the squad")
		a.emitEvent("squad:state", nil)
	}
}

// SquadRename changes the name this PC shows in its squad.
func (a *App) SquadRename(name string) {
	squadMu.Lock()
	client := squadClient
	squadMu.Unlock()
	if client != nil {
		client.Report(a.squadReport(name))
	}
}

// SquadSetColor sets the squad colour this PC's reports carry ("#rrggbb", or
// empty to take one the others give), and tells the squad joined.
func (a *App) SquadSetColor(color string) {
	if !squad.ValidColor(color) {
		color = ""
	}
	squadMu.Lock()
	squadColor = color
	client := squadClient
	squadMu.Unlock()
	if client != nil {
		r := client.Mine()
		r.Color = color
		client.Report(r)
	}
}

// SquadState returns the squad joined, or nil.
func (a *App) SquadState() *squad.State {
	squadMu.Lock()
	client := squadClient
	squadMu.Unlock()
	if client == nil {
		return nil
	}
	s := client.State()
	return &s
}

// squadReport is this PC's report under name: its position when it is in a
// raid and has one from this raid's map.
func (a *App) squadReport(name string) squad.Report {
	// A client only watches: its Host, the same player, reports the position.
	squadMu.Lock()
	color := squadColor
	squadMu.Unlock()
	r := squad.Report{Name: strings.TrimSpace(name), Viewer: a.browserClient.Load(), Color: color}
	a.mu.RLock()
	status := a.status
	a.mu.RUnlock()
	if status.RaidActive && status.CurrentMap != "" && status.Position != nil && !a.browserClient.Load() {
		r.Map = status.CurrentMap
		r.Pos, r.At = squadPosition(*status.Position)
	}
	return r
}

func squadPosition(p model.Position) (*squad.Position, time.Time) {
	at, err := time.Parse(time.RFC3339Nano, p.DetectedAt)
	if err != nil {
		at = time.Now()
	}
	return &squad.Position{X: p.X, Y: p.Y, Z: p.Z, Rot: p.Rotation}, at
}

// squadSendPosition tells the squad where this PC's player is (a position
// screenshot in a raid on mapName).
func (a *App) squadSendPosition(mapName string, p model.Position) {
	squadMu.Lock()
	client := squadClient
	squadMu.Unlock()
	if client == nil {
		return
	}
	r := client.Mine()
	r.Map = mapName
	r.Pos, r.At = squadPosition(p)
	client.Report(r)
}

// squadLeftRaid tells the squad this PC's player is out of the raid.
func (a *App) squadLeftRaid() {
	squadMu.Lock()
	client := squadClient
	squadMu.Unlock()
	if client == nil {
		return
	}
	r := client.Mine()
	if r.Map == "" {
		return
	}
	client.Report(squad.Report{Name: r.Name, Viewer: r.Viewer})
}

func squadMapSource() *mapdata.Source {
	squadMapsOnce.Do(func() {
		dir, _ := appdir.Path("maps")
		squadMaps = mapdata.New(dir, version.UserAgent())
	})
	return squadMaps
}

// BrowserSquadMaps returns the interactive maps' geometry for the squad map.
func (a *App) BrowserSquadMaps() ([]mapdata.Map, error) {
	if !version.IsPrerelease() {
		return nil, errSquadOff
	}
	ctx, cancel := context.WithTimeout(a.baseContext(), 40*time.Second)
	defer cancel()
	maps, err := squadMapSource().Maps(ctx)
	if err != nil {
		a.addLog("Warn", "Squad", "Map data is unavailable: "+err.Error())
	}
	return maps, err
}

// BrowserSquadMapImage returns the picture of map (a key or an alias) at a
// floor (a layer's svgLayer, or "" for the ground level), the ground faded
// to fade percent under a floor, as a data URL.
func (a *App) BrowserSquadMapImage(name, layer string, fade int) (string, error) {
	if !version.IsPrerelease() {
		return "", errSquadOff
	}
	ctx, cancel := context.WithTimeout(a.baseContext(), 40*time.Second)
	defer cancel()
	source := squadMapSource()
	maps, err := source.Maps(ctx)
	if err != nil {
		return "", err
	}
	m, ok := mapdata.Find(maps, name)
	if !ok {
		return "", errors.New("unknown map")
	}
	img, err := source.Image(ctx, m, layer, fade)
	if err != nil {
		a.addLog("Warn", "Squad", "Map picture is unavailable: "+err.Error())
	}
	return img, err
}

// BrowserMapTile returns a tile of a map drawn from tiles (on
// assets.tarkov.dev) as a data URL, for the map view's picture made into a
// snap note.
func (a *App) BrowserMapTile(url string) (string, error) {
	if !version.IsPrerelease() {
		return "", errSquadOff
	}
	ctx, cancel := context.WithTimeout(a.baseContext(), 20*time.Second)
	defer cancel()
	return squadMapSource().Tile(ctx, url)
}

func (a *App) baseContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// mapMarkersCache keeps the markers built last per mode, map and language:
// the catalog's maps resource is large to decode for each map shown.
var (
	mapMarkersMu    sync.Mutex
	mapMarkersCache = map[string]mapMarkersEntry{}
)

type mapMarkersEntry struct {
	at      time.Time
	markers mapdata.MapMarkers
}

// BrowserMapMarkers returns what the map view can show on map (a key or an
// alias), the way tarkov.dev's map shows it: extracts, spawns, loot, task
// objectives… from the tarkov.dev catalog of the game mode played, with
// item and task names in language ("ja" or "en"). With TarkovTracker
// connected, a task done or failed is marked inactive.
//
// mode is the catalog's game mode ("regular", "pve", "pvp-season"), or ""
// for the one played (the Host's setting, or what TarkovTracker and the
// logs tell; PvP when unknown); the markers say which it was.
func (a *App) BrowserMapMarkers(name, language, mode string) (mapdata.MapMarkers, error) {
	if !version.IsPrerelease() {
		return mapdata.MapMarkers{}, errSquadOff
	}
	a.mu.RLock()
	if !catalog.ValidMode(mode) {
		mode = a.effectiveCatalogMode(a.settings.GameMode)
	}
	var taskStates map[string]string
	if a.status.Tracker.Connection == "connected" {
		taskStates = make(map[string]string, len(a.trackerTasks))
		for id, s := range a.trackerTasks {
			taskStates[id] = s
		}
	}
	a.mu.RUnlock()
	if !catalog.ValidMode(mode) {
		mode = "regular"
	}
	lang := "en"
	if slices.Contains(locale.Languages, language) {
		lang = language
	}
	key := mode + "|" + name + "|" + lang + "|" + strconv.Itoa(len(taskStates)) + "|" + fmt.Sprint(taskStates != nil)
	mapMarkersMu.Lock()
	if e, ok := mapMarkersCache[key]; ok && time.Since(e.at) < 10*time.Minute {
		mapMarkersMu.Unlock()
		return e.markers, nil
	}
	mapMarkersMu.Unlock()
	ctx, cancel := context.WithTimeout(a.baseContext(), 40*time.Second)
	defer cancel()
	var src mapdata.Sources
	if err := a.catalogClient.Get(ctx, mode, "maps", &src.Maps); err != nil {
		return mapdata.MapMarkers{}, err
	}
	_ = a.catalogClient.Get(ctx, mode, "maps_en", &src.MapNames)
	_ = a.catalogClient.Get(ctx, mode, "items", &src.Items)
	_ = a.catalogClient.Get(ctx, mode, "items_en", &src.ItemNamesEn)
	_ = a.catalogClient.Get(ctx, mode, "tasks", &src.Tasks)
	_ = a.catalogClient.Get(ctx, mode, "tasks_en", &src.TaskNamesEn)
	if lang != "en" {
		_ = a.catalogClient.Get(ctx, mode, locale.Resource("items", lang), &src.ItemNames)
		_ = a.catalogClient.Get(ctx, mode, locale.Resource("tasks", lang), &src.TaskNames)
	}
	if taskStates != nil {
		src.TaskActive = func(id string) bool {
			s := taskStates[id]
			return s != "completed" && s != "failed"
		}
	}
	names := []string{name}
	if list, err := squadMapSource().Maps(ctx); err == nil {
		if m, ok := mapdata.Find(list, name); ok {
			names = append([]string{m.Key}, m.Aliases...)
		}
	}
	markers, err := mapdata.Markers(src, names...)
	if err != nil {
		return mapdata.MapMarkers{}, err
	}
	markers.Mode = mode
	mapMarkersMu.Lock()
	mapMarkersCache[key] = mapMarkersEntry{at: time.Now(), markers: markers}
	mapMarkersMu.Unlock()
	return markers, nil
}
