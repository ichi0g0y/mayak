package app

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/local/mayak/internal/bossinfo"
)

// Goons reports go to tarkov.dev with an EFT account ID the user picks from
// the accounts and game modes seen in this PC's EFT logs, for a raid of the
// ones the logs showed (the current one or the last).

// GoonIdentity is an account and game mode seen in the EFT logs.
type GoonIdentity struct {
	AccountID string `json:"accountId"`
	ProfileID string `json:"profileId"`
	Mode      string `json:"mode"` // pvp or pve
	LastSeen  string `json:"lastSeen"`
	Current   bool   `json:"current"`
}

// GoonRaid is a raid a report can be for.
type GoonRaid struct {
	Map       string `json:"map"`
	StartedAt string `json:"startedAt"`
	Mode      string `json:"mode"`
	AccountID string `json:"accountId"`
	ProfileID string `json:"profileId"`
	Active    bool   `json:"active"`
	Reported  bool   `json:"reported"`
}

type GoonReportInfo struct {
	Raid       *GoonRaid      `json:"raid"`
	Identities []GoonIdentity `json:"identities"`
}

// GoonReport is what the user chose to report: the map (its normalized name),
// the account and mode, and the raid start (RFC 3339; empty for now).
type GoonReport struct {
	Map       string `json:"map"`
	AccountID string `json:"accountId"`
	Mode      string `json:"mode"`
	StartedAt string `json:"startedAt"`
}

// rememberRaidLocked keeps the raid ending now for a later Goons report.
// a.mu must be held.
func (a *App) rememberRaidLocked() {
	if a.status.RaidStartedAt == "" || a.status.CurrentMap == "" {
		return
	}
	a.lastRaid = &GoonRaid{Map: a.status.CurrentMap, StartedAt: a.status.RaidStartedAt, Mode: a.status.Tracker.Mode, AccountID: a.status.Tracker.AccountID, ProfileID: a.status.Tracker.ProfileID}
}

func goonReportKey(accountID, mode, startedAt string) string {
	return accountID + "|" + mode + "|" + startedAt
}

// BrowserGoonReportInfo returns what a Goons report can be about: the raid in
// progress or the last one, and the accounts and modes seen in the logs.
func (a *App) BrowserGoonReportInfo() GoonReportInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	info := GoonReportInfo{Identities: []GoonIdentity{}}
	if a.status.RaidActive && a.status.RaidStartedAt != "" && a.status.CurrentMap != "" {
		info.Raid = &GoonRaid{Map: a.status.CurrentMap, StartedAt: a.status.RaidStartedAt, Mode: a.status.Tracker.Mode, AccountID: a.status.Tracker.AccountID, ProfileID: a.status.Tracker.ProfileID, Active: true}
	} else if a.lastRaid != nil {
		raid := *a.lastRaid
		info.Raid = &raid
	}
	if info.Raid != nil {
		info.Raid.Reported = a.goonReports[goonReportKey(info.Raid.AccountID, info.Raid.Mode, info.Raid.StartedAt)]
	}
	// One entry per account and mode: a report names no profile, and an
	// account can have had several profiles of a mode (after a wipe).
	index := map[string]int{}
	for _, p := range a.trackerData.Profiles {
		if p.AccountID == "" || (p.Mode != "pvp" && p.Mode != "pve") {
			continue
		}
		current := p.AccountID == a.status.Tracker.AccountID && p.ProfileID == a.status.Tracker.ProfileID && p.Mode == a.status.Tracker.Mode
		key := p.AccountID + "|" + p.Mode
		if i, ok := index[key]; ok {
			id := &info.Identities[i]
			if p.LastSeen > id.LastSeen {
				id.ProfileID, id.LastSeen = p.ProfileID, p.LastSeen
			}
			id.Current = id.Current || current
			continue
		}
		index[key] = len(info.Identities)
		info.Identities = append(info.Identities, GoonIdentity{AccountID: p.AccountID, ProfileID: p.ProfileID, Mode: p.Mode, LastSeen: p.LastSeen, Current: current})
	}
	// The account being played first, then the most recently seen.
	sort.SliceStable(info.Identities, func(i, j int) bool {
		if info.Identities[i].Current != info.Identities[j].Current {
			return info.Identities[i].Current
		}
		return info.Identities[i].LastSeen > info.Identities[j].LastSeen
	})
	return info
}

// BrowserReportGoons submits a Goons sighting to tarkov.dev, for an account and
// mode seen in the logs, on a map the Goons spawn on, once per raid.
func (a *App) BrowserReportGoons(r GoonReport) error {
	if a.bossInfo == nil {
		return errors.New("Goons reports are unavailable")
	}
	a.mu.RLock()
	known := false
	for _, p := range a.trackerData.Profiles {
		if p.AccountID == r.AccountID && p.Mode == r.Mode {
			known = true
			break
		}
	}
	a.mu.RUnlock()
	if !known || (r.Mode != "pvp" && r.Mode != "pve") {
		return errors.New("choose an account and game mode seen in the EFT logs")
	}
	at := time.Now()
	if r.StartedAt != "" {
		started, err := time.Parse(time.RFC3339, r.StartedAt)
		if err != nil || started.After(at.Add(5*time.Minute)) || at.Sub(started) > 24*time.Hour {
			return errors.New("the raid is too old to report")
		}
		at = started
	}
	key := goonReportKey(r.AccountID, r.Mode, r.StartedAt)
	a.mu.RLock()
	done := r.StartedAt != "" && a.goonReports[key]
	a.mu.RUnlock()
	if done {
		return errors.New("this raid was already reported")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	mode := catalogMode(r.Mode)
	info, err := a.bossInfo.Info(ctx, mode, "en")
	if err != nil {
		return err
	}
	nameID := ""
	for _, m := range info.Maps {
		if m.Key != r.Map {
			continue
		}
		for _, b := range m.Bosses {
			if b.ID == "bossKnight" {
				nameID = m.NameID
			}
		}
	}
	if nameID == "" {
		return errors.New("the Goons do not spawn on this map")
	}
	if err := a.bossInfo.SendReport(ctx, bossinfo.Report{MapNameID: nameID, Mode: bossinfo.Mode(mode), Time: at, AccountID: r.AccountID}); err != nil {
		a.addLog("Warn", "Goons", "The Goons report could not be sent: "+err.Error())
		return err
	}
	a.mu.Lock()
	if a.goonReports == nil {
		a.goonReports = map[string]bool{}
	}
	if r.StartedAt != "" {
		a.goonReports[key] = true
	}
	a.mu.Unlock()
	a.addLog("Info", "Goons", "Reported the Goons on "+r.Map)
	return nil
}
