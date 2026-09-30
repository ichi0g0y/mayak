package app

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/ocr"
	"github.com/local/mayak/internal/profiledetect"
	"github.com/local/mayak/internal/questmatch"
	"github.com/local/mayak/internal/taskdetect"
)

// Tasks done in the game but not on TarkovTracker: a Tasks screenshot with
// "Show completed" ticked lists a trader's tasks, each row's colour telling
// its state (taskdetect.ListRows). The rows of completed tasks are read and
// matched; those TarkovTracker does not have completed are gathered, per
// EFT profile (account, profile and mode), across screenshots and restarts
// (completable-tasks.json), for the player to look over and apply
// (TrackerCompleteTasks), never on their own. The profile played shows its
// list (status.CompletableTasks).

// listMatchMin is how sure a row's match must be.
const listMatchMin = .85

// Tasks screenshots come one after another while the lists are gone
// through, and a new screenshot stops the last one's analysis. So a Tasks
// screenshot's list is queued (queueTaskList) as soon as it is found to be
// one, and read in turn, whatever comes next.
type taskListJob struct {
	path     string
	settings config.Settings
}

// queueTaskList queues path's list for reading.
func (a *App) queueTaskList(path string, settings config.Settings) {
	a.taskListOnce.Do(func() {
		a.taskLists = make(chan taskListJob, 256)
		go func() {
			for job := range a.taskLists {
				if a.quitting.Load() {
					return
				}
				ctx := a.ctx
				if ctx == nil {
					ctx = context.Background()
				}
				a.scanTaskList(ctx, job.path, job.settings)
			}
		}()
	})
	select {
	case a.taskLists <- taskListJob{path, settings}:
	default:
		a.addLog("Warn", "TarkovTracker", "Too many Tasks screenshots waiting; this one's list is skipped")
	}
}

func completablePath() string {
	p, _ := appdir.Path("completable-tasks.json")
	return p
}

// profileKey names the EFT profile played, as TarkovTracker's key is chosen.
func profileKey(tr model.TrackerStatus) string {
	return tr.AccountID + "|" + tr.ProfileID + "|" + tr.Mode
}

// loadCompletable reads the lists kept. a.mu must be held.
func (a *App) loadCompletableLocked() {
	if a.completable != nil {
		return
	}
	a.completable = map[string]map[string]model.CompletableTask{}
	if data, err := os.ReadFile(completablePath()); err == nil {
		_ = json.Unmarshal(data, &a.completable)
	}
}

// saveCompletable keeps the lists. a.mu must be held.
func (a *App) saveCompletableLocked() {
	data, err := json.MarshalIndent(a.completable, "", "  ")
	if err == nil {
		err = os.WriteFile(completablePath(), data, 0o600)
	}
	if err != nil {
		a.addLog("Warn", "TarkovTracker", "Could not keep the tasks to complete: "+err.Error())
	}
}

// scanTaskList adds the completed tasks the list on path shows that
// TarkovTracker does not have completed, while it is synced.
func (a *App) scanTaskList(ctx context.Context, path string, settings config.Settings) {
	a.mu.RLock()
	tr := a.status.Tracker
	a.mu.RUnlock()
	if !settings.TarkovTrackerEnabled || tr.Connection != "connected" {
		return
	}
	img, err := decodeImage(path)
	if err != nil {
		return
	}
	rows, err := taskdetect.ListRows(img)
	if err != nil || len(rows) == 0 {
		return
	}
	quests, err := a.questClient.QuestsForMode(ctx, a.effectiveCatalogMode(settings.GameMode))
	if err != nil {
		return
	}
	base := make([]questmatch.Quest, len(quests))
	for i, q := range quests {
		base[i] = q.Quest
	}
	readers := listReaders(settings)
	var matched []questmatch.Quest
	for _, row := range rows {
		if row.State != "completed" {
			continue
		}
		if match, ok := readListRow(ctx, readers, row.Name, base); ok {
			matched = append(matched, match)
		}
	}
	key := profileKey(tr)
	found := 0
	a.mu.Lock()
	if profileKey(a.status.Tracker) != key {
		a.mu.Unlock()
		return
	}
	a.loadCompletableLocked()
	list := a.completable[key]
	if list == nil {
		list = map[string]model.CompletableTask{}
	}
	for _, q := range matched {
		if a.trackerTasks[q.ID] == "completed" {
			continue
		}
		if _, known := list[q.ID]; !known {
			found++
		}
		list[q.ID] = model.CompletableTask{ID: q.ID, Name: q.Name, Trader: q.Trader}
	}
	a.completable[key] = list
	if found > 0 {
		a.saveCompletableLocked()
	}
	a.mu.Unlock()
	if found > 0 {
		a.addLog("Info", "TarkovTracker", fmt.Sprintf("%d task(s) done in the game are not completed on TarkovTracker", found))
	}
	a.publishCompletable()
}

// listReaders are the OCR engines a row's name is read with, the first
// tried first: task names are English in every language of the game.
func listReaders(settings config.Settings) []ocr.Engine {
	var out []ocr.Engine
	if runtime.GOOS == "windows" {
		out = append(out, ocr.Windows{Language: "en"})
	}
	engine := tesseractEngine(settings)
	engine.Language = "eng"
	return append(out, engine)
}

// readListRow reads a row's name and matches it, with the next engine while
// the match is not sure.
func readListRow(ctx context.Context, readers []ocr.Engine, name image.Image, base []questmatch.Quest) (questmatch.Quest, bool) {
	inked := profiledetect.Ink(name)
	for _, r := range readers {
		text, err := r.Recognize(ctx, inked)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		if m := questmatch.Match(text, base); len(m) > 0 && m[0].Confidence >= listMatchMin {
			return m[0].Quest, true
		}
	}
	return questmatch.Quest{}, false
}

// publishCompletable drops from the profile played's list the tasks
// TarkovTracker has completed meanwhile (once its progress is loaded) and
// puts the rest in the status, by trader and name.
func (a *App) publishCompletable() {
	a.mu.Lock()
	a.loadCompletableLocked()
	key := profileKey(a.status.Tracker)
	synced := a.status.Tracker.Connection == "connected"
	stored := a.completable[key]
	list := make([]model.CompletableTask, 0, len(stored))
	pruned := false
	for id, task := range stored {
		if synced && a.trackerTasks[id] == "completed" {
			delete(stored, id)
			pruned = true
			continue
		}
		list = append(list, task)
	}
	if pruned {
		a.saveCompletableLocked()
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Trader != list[j].Trader {
			return list[i].Trader < list[j].Trader
		}
		return list[i].Name < list[j].Name
	})
	a.status.CompletableTasks = list
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
}

// TrackerCompleteTasks sets the tasks (their IDs, among the profile
// played's list) as completed on TarkovTracker, one after another, and
// tells how many were.
func (a *App) TrackerCompleteTasks(ids []string) (int, error) {
	a.mu.Lock()
	a.loadCompletableLocked()
	tr := a.status.Tracker
	token := a.trackerData.TokenFor(tr.AccountID, tr.ProfileID, tr.Mode)
	list := a.completable[profileKey(tr)]
	var todo []string
	for _, id := range ids {
		if _, ok := list[id]; ok {
			todo = append(todo, id)
		}
	}
	a.mu.Unlock()
	if tr.Connection != "connected" || token == "" {
		return 0, fmt.Errorf("TarkovTracker is not connected")
	}
	done := 0
	for _, id := range todo {
		a.syncTrackerTask(tr.Mode, tr.ProfileID, tr.AccountID, token, id, "completed")
		a.mu.RLock()
		ok := a.trackerTasks[id] == "completed"
		a.mu.RUnlock()
		if ok {
			done++
		}
	}
	a.addLog("Info", "TarkovTracker", fmt.Sprintf("Completed %d of %d task(s) from the Tasks screen's list", done, len(todo)))
	a.publishCompletable()
	return done, nil
}

// TrackerDismissTasks takes tasks off the profile played's list without
// applying them (all of them when ids is empty).
func (a *App) TrackerDismissTasks(ids []string) {
	a.mu.Lock()
	a.loadCompletableLocked()
	key := profileKey(a.status.Tracker)
	if len(ids) == 0 {
		delete(a.completable, key)
	}
	for _, id := range ids {
		delete(a.completable[key], id)
	}
	a.saveCompletableLocked()
	a.mu.Unlock()
	a.publishCompletable()
}
