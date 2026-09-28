package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
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
	squadMu     sync.Mutex
	squadClient *squad.Client

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
	squadMu.Lock()
	defer squadMu.Unlock()
	if squadClient != nil {
		if squadClient.Code() == canonical {
			report := a.squadReport(name)
			squadClient.Report(report)
			return squad.Format(canonical), nil
		}
		squadClient.Close()
		squadClient = nil
	}
	var client *squad.Client
	client, err = squad.Join(canonical, squadEndpoint(), version.UserAgent(), a.squadReport(name), func(s squad.State) {
		squadMu.Lock()
		current := squadClient == client
		squadMu.Unlock()
		if current {
			a.emitEvent("squad:state", s)
		}
	})
	if err != nil {
		return "", err
	}
	squadClient = client
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
	r := squad.Report{Name: strings.TrimSpace(name)}
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
	client.Report(squad.Report{Name: r.Name})
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
// floor (a layer's svgLayer, or "" for the ground level), as a data URL.
func (a *App) BrowserSquadMapImage(name, layer string) (string, error) {
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
	img, err := source.Image(ctx, m, layer)
	if err != nil {
		a.addLog("Warn", "Squad", "Map picture is unavailable: "+err.Error())
	}
	return img, err
}

func (a *App) baseContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
