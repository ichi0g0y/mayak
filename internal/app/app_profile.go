package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"runtime"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/profiledetect"
	"github.com/local/mayak/internal/squad"
	"golang.org/x/image/draw"
)

// The character's Overall screen (internal/profiledetect): a screenshot of
// it is read for the nickname, the level, the experience and the stats
// beside the level, kept on this PC (profile.json, with a picture of the
// character's panel in profile.jpg), told to the squad, and the level set
// on TarkovTracker when it went up.
//
// Each part is read by every OCR engine at hand (the bundled Tesseract in
// English, Windows OCR in English and Japanese: each misreads different
// glyphs) and the value most of them agree on is taken. The level counts
// as sure when the number read and the level the experience makes agree.

// profilePictureWidth is the picture's width (its panel is 848 px wide).
const profilePictureWidth = 360

func profilePath() string {
	p, _ := appdir.Path("profile.json")
	return p
}

func profilePicturePath() string {
	p, _ := appdir.Path("profile.jpg")
	return p
}

// loadPlayerProfile reads the profile kept on this PC, or nil.
func loadPlayerProfile() *model.PlayerProfile {
	data, err := os.ReadFile(profilePath())
	if err != nil {
		return nil
	}
	var p model.PlayerProfile
	if json.Unmarshal(data, &p) != nil || p.At == "" {
		return nil
	}
	return &p
}

// profileReaders are the OCR engines a profile is read with.
func profileReaders(settings config.Settings) []ocr.Engine {
	var out []ocr.Engine
	engine := tesseractEngine(settings)
	engine.Language = "eng"
	out = append(out, engine)
	if runtime.GOOS == "windows" {
		out = append(out, ocr.Windows{Language: "en"}, ocr.Windows{Language: "ja"})
	}
	return out
}

// tryProfileScreenshot reads path as the Overall screen when it is one,
// and tells whether it was.
func (a *App) tryProfileScreenshot(ctx context.Context, sequence uint64, path string, settings config.Settings) bool {
	detected, err := profiledetect.AnalyzeFile(path)
	if err != nil || !detected.IsProfile {
		return false
	}
	a.handleProfileScreenshot(ctx, sequence, path, settings, detected)
	return true
}

func (a *App) handleProfileScreenshot(ctx context.Context, sequence uint64, path string, settings config.Settings, detected profiledetect.Result) {
	a.setAnalysisStage(sequence, path, "プロフィールを読み取り中")
	readers := profileReaders(settings)
	read := func(img image.Image) []string {
		inked := profiledetect.Ink(img)
		out := make([]string, len(readers))
		for i, r := range readers {
			if text, err := r.Recognize(ctx, inked); err == nil {
				out[i] = text
			}
		}
		return out
	}
	var levels []int
	for _, text := range read(detected.Level) {
		levels = append(levels, profiledetect.Level(text))
	}
	stats := make([]profiledetect.Stats, len(readers))
	lefts, rights := make([][]string, len(readers)), make([][]string, len(readers))
	for _, line := range detected.StatsLeft {
		for i, text := range read(line) {
			lefts[i] = append(lefts[i], text)
		}
	}
	for _, line := range detected.StatsRight {
		for i, text := range read(line) {
			rights[i] = append(rights[i], text)
		}
	}
	for i := range readers {
		stats[i] = profiledetect.ReadStats(lefts[i], rights[i])
	}
	// The row the most engines read as experience; the nickname is the row
	// above it.
	rows := make([][]string, len(detected.Rows))
	expRow, expVotes := -1, 0
	for i, row := range detected.Rows {
		rows[i] = read(row)
		votes := 0
		for _, text := range rows[i] {
			if _, ok := profiledetect.Experience(text); ok {
				votes++
			}
		}
		if votes > expVotes {
			expRow, expVotes = i, votes
		}
	}
	var exps []int
	var names []string
	if expRow >= 0 {
		for _, text := range rows[expRow] {
			n, _ := profiledetect.Experience(text)
			exps = append(exps, n)
		}
		if expRow > 0 {
			for _, text := range rows[expRow-1] {
				names = append(names, profiledetect.Nickname(text))
			}
		}
	}
	voted := profiledetect.VoteStats(stats)
	profile := model.PlayerProfile{
		Name: profiledetect.Vote(names), Level: profiledetect.Vote(levels), Exp: profiledetect.Vote(exps),
		Raids: voted.Raids, Kills: voted.Kills, SurvivalRate: voted.SurvivalRate, KD: voted.KD, Hours: voted.Hours,
		At: time.Now().UTC().Format(time.RFC3339),
	}
	sure := false
	if steps := a.levelSteps(ctx, settings); profile.Exp > 0 && len(steps) > 0 {
		byExp := profiledetect.LevelOf(profile.Exp, steps)
		switch {
		case profile.Level == 0:
			profile.Level = byExp
		case profile.Level == byExp:
			sure = true
		case agreeing(levels, byExp) > 0:
			// An engine read the level the experience makes, though the vote
			// went to another reading (10 read as 1, 10 and 12).
			profile.Level, sure = byExp, true
		case agreeing(exps, profile.Exp) >= 2:
			// Engines that agree on the experience, a long number, outweigh a
			// level's glyph dropped or misread (16 read as 6).
			a.addLog("Info", "Profile", fmt.Sprintf("Read level %d, but %d experience, read alike by %d engines, makes level %d: level %d it is", profile.Level, profile.Exp, agreeing(exps, profile.Exp), byExp, byExp))
			profile.Level, sure = byExp, true
		default:
			a.addLog("Warn", "Profile", fmt.Sprintf("Read level %d, but %d experience makes level %d", profile.Level, profile.Exp, byExp))
		}
	}
	if err := saveProfilePicture(detected.Panel); err != nil {
		a.addLog("Warn", "Profile", "Could not keep the character's picture: "+err.Error())
	} else {
		profile.Picture = true
	}
	if data, err := json.MarshalIndent(profile, "", "  "); err == nil {
		if err := os.WriteFile(profilePath(), data, 0o600); err != nil {
			a.addLog("Warn", "Profile", "Could not keep the profile: "+err.Error())
		}
	}
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.Unlock()
		return
	}
	a.status.ScreenshotType = "profile"
	a.status.AnalysisStage = "プロフィールを読み取りました"
	a.status.LastError = ""
	a.status.Profile = &profile
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	a.addLog("Info", "Profile", fmt.Sprintf("Read the Overall screen: %s, level %d (%d exp), %d raids, K/D %.2f", profile.Name, profile.Level, profile.Exp, profile.Raids, profile.KD))
	a.toast(Toast{Event: "profile", Category: ToastRecognition, Level: "success", Message: "toastProfileRead", Params: map[string]string{"name": profile.Name, "level": fmt.Sprint(profile.Level)}})
	a.squadUpdateProfile()
	if sure {
		a.trackerRaiseLevel(ctx, settings, profile.Level)
	}
}

// levelKept takes away the notice that TarkovTracker's level went back once
// it reads at the level the notice was about or above.
func (a *App) levelKept(level int) {
	if warned := a.levelWarned.Load(); warned > 0 && int64(level) >= warned && a.levelWarned.CompareAndSwap(warned, 0) {
		a.endToast("trackerLevelOverride")
	}
}

// agreeing counts the readings that are v.
func agreeing(readings []int, v int) int {
	n := 0
	for _, r := range readings {
		if r == v {
			n++
		}
	}
	return n
}

// setAnalysisStage shows how far a screenshot's analysis is.
func (a *App) setAnalysisStage(sequence uint64, path, stage string) {
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence || a.status.LastScreenshot != path {
		a.mu.Unlock()
		return
	}
	a.status.AnalysisStage = stage
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
}

// saveProfilePicture keeps the character's panel, made smaller, as a JPEG.
func saveProfilePicture(panel image.Image) error {
	if panel == nil {
		return errors.New("no picture")
	}
	b := panel.Bounds()
	height := b.Dy() * profilePictureWidth / b.Dx()
	small := image.NewRGBA(image.Rect(0, 0, profilePictureWidth, height))
	draw.CatmullRom.Scale(small, small.Bounds(), panel, b, draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: 82}); err != nil {
		return err
	}
	return os.WriteFile(profilePicturePath(), buf.Bytes(), 0o600)
}

// levelSteps is each level's experience over the one before it, from the
// game's data (tarkov.dev's playerLevels): nil without it.
func (a *App) levelSteps(ctx context.Context, settings config.Settings) []int {
	var items struct {
		Data struct {
			PlayerLevels []struct {
				Exp int `json:"exp"`
			} `json:"playerLevels"`
		} `json:"data"`
	}
	if a.catalogClient == nil {
		return nil
	}
	// The table is the same in every mode: the one played, else any kept.
	for _, mode := range []string{a.effectiveCatalogMode(settings.GameMode), "regular", "pve"} {
		if a.catalogClient.Get(ctx, mode, "items", &items) == nil && len(items.Data.PlayerLevels) > 0 {
			break
		}
	}
	out := make([]int, len(items.Data.PlayerLevels))
	for i, l := range items.Data.PlayerLevels {
		out[i] = l.Exp
	}
	return out
}

// trackerRaiseLevel sets level on TarkovTracker when it is above the one
// there (never lower: a level read wrong must not take progress back).
func (a *App) trackerRaiseLevel(ctx context.Context, settings config.Settings, level int) {
	a.mu.RLock()
	tr := a.status.Tracker
	token := a.trackerData.TokenFor(tr.AccountID, tr.ProfileID, tr.Mode)
	a.mu.RUnlock()
	if !settings.TarkovTrackerEnabled || tr.Connection != "connected" || token == "" {
		return
	}
	if level <= tr.PlayerLevel {
		a.keepLevel(tr, level)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := a.trackerClient.SetLevel(ctx, token, level); err != nil {
		a.addLog("Warn", "TarkovTracker", fmt.Sprintf("Could not set the level to %d: %s", level, err))
		return
	}
	a.mu.Lock()
	if a.status.Tracker.AccountID == tr.AccountID && a.status.Tracker.ProfileID == tr.ProfileID && a.status.Tracker.Mode == tr.Mode {
		a.status.Tracker.PlayerLevel = level
	}
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Level set to %d (from %d) from the Overall screen", level, tr.PlayerLevel))
	a.toast(Toast{Event: "trackerLevel", Category: ToastTracker, Level: "success", Message: "toastLevelSet", Params: map[string]string{"level": fmt.Sprint(level)}})
	a.squadUpdateTracker()
	time.AfterFunc(trackerLevelRecheck, func() { a.checkTrackerLevelKept(tr, token, level, false) })
}

// trackerLevelRecheck is how long after setting the level MAYAK reads it back,
// and trackerLevelSettle how long after a lower reading it reads again: a
// TarkovTracker page left open writes back what it holds, its Automatic Level
// Calculation's level or, for a while, a save queued before MAYAK's (seen
// 2026-10-07: 15 again after 90 s, 16 later, the page showing 16 all along).
var (
	trackerLevelRecheck = 90 * time.Second
	trackerLevelSettle  = 4 * time.Minute
)

// checkTrackerLevelKept reads the level back after MAYAK set it; lower, it
// reads again later (last false), and lower then too, a notice says so and how
// to keep the level from the Overall screen. It is not sent again: the two
// would only take turns.
func (a *App) checkTrackerLevelKept(tr model.TrackerStatus, token string, level int, last bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	progress, err := a.trackerClient.Progress(ctx, token)
	if err != nil {
		return
	}
	now := progress.Data.PlayerLevel
	if now >= level {
		a.levelKept(now)
		return
	}
	if !last {
		a.addLog("Debug", "TarkovTracker", fmt.Sprintf("The level set to %d reads %d; reading again in %s", level, now, trackerLevelSettle))
		time.AfterFunc(trackerLevelSettle, func() { a.checkTrackerLevelKept(tr, token, level, true) })
		return
	}
	a.mu.Lock()
	if a.status.Tracker.AccountID == tr.AccountID && a.status.Tracker.ProfileID == tr.ProfileID && a.status.Tracker.Mode == tr.Mode {
		a.status.Tracker.PlayerLevel = now
	}
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	a.addLog("Warn", "TarkovTracker", fmt.Sprintf("The level set to %d is %d again, minutes later: an open TarkovTracker page wrote it back (its Automatic Level Calculation, or what it held)", level, now))
	a.levelWarned.Store(int64(level))
	a.toast(Toast{
		Key: "trackerLevelOverride", Event: "trackerLevel", Category: ToastTracker, Level: "warn", Persistent: true,
		Message: "toastLevelOverridden", Params: map[string]string{"sent": fmt.Sprint(level), "level": fmt.Sprint(now)},
		Actions: []ToastAction{{ID: "trackerExperience", Label: "toastOpenTrackerExperience"}},
	})
	a.squadUpdateTracker()
}

// squadProfile is this player's Overall screen in short, for the squad.
func (a *App) squadProfile() *squad.Profile {
	if a.browserClient.Load() {
		return nil
	}
	a.mu.RLock()
	p := a.status.Profile
	a.mu.RUnlock()
	if p == nil {
		return nil
	}
	out := &squad.Profile{Name: p.Name, Level: p.Level, Raids: p.Raids, Kills: p.Kills, SurvivalRate: p.SurvivalRate, KD: p.KD, Hours: p.Hours}
	if at, err := time.Parse(time.RFC3339, p.At); err == nil && p.Picture {
		out.Picture = at.Unix()
	}
	return out
}

// squadUpdateProfile tells the squad this player's Overall screen when it
// has changed.
func (a *App) squadUpdateProfile() {
	squadMu.Lock()
	client := squadClient
	squadMu.Unlock()
	if client == nil {
		return
	}
	r := client.Mine()
	next := a.squadProfile()
	if (r.Profile == nil && next == nil) || (r.Profile != nil && next != nil && *r.Profile == *next) {
		return
	}
	r.Profile = next
	client.Report(r)
}

// squadPicture is what the squad's store keeps of a member's picture.
type squadPicture struct {
	Key     string `json:"key"`
	Picture string `json:"picture"`
}

// SquadSharePicture puts this player's character picture in the squad's
// store (on), or takes it out (off), for the others' cards.
func (a *App) SquadSharePicture(on bool) error {
	squadMu.Lock()
	client := squadClient
	squadMu.Unlock()
	if client == nil {
		return nil
	}
	if !on {
		return client.StoreIn("picture", nil)
	}
	data, err := os.ReadFile(profilePicturePath())
	if err != nil {
		return nil
	}
	payload, err := json.Marshal(squadPicture{Key: client.Mine().Key, Picture: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		return err
	}
	return client.StoreIn("picture", payload)
}

// SquadPictures returns the members' character pictures in the squad's
// store, as JSON: member key → data URL.
func (a *App) SquadPictures() (string, error) {
	squadMu.Lock()
	client := squadClient
	squadMu.Unlock()
	out := map[string]string{}
	if client != nil {
		list, err := client.Stored()
		if err != nil {
			return "", err
		}
		for _, raw := range list {
			var p squadPicture
			if json.Unmarshal(raw, &p) == nil && squad.ValidKey(p.Key) && len(p.Picture) > 23 && p.Picture[:23] == "data:image/jpeg;base64," {
				out[p.Key] = p.Picture
			}
		}
	}
	data, err := json.Marshal(out)
	return string(data), err
}

// BrowserProfilePicture returns this player's character picture as a data
// URL ("" without one), for their own card.
func (a *App) BrowserProfilePicture() string {
	data, err := os.ReadFile(profilePicturePath())
	if err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
}
