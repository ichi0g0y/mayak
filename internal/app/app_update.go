package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/update"
	"github.com/local/mayak/internal/version"
)

// Updates come from GitHub Releases (internal/update). With autoUpdate on,
// MAYAK looks for a newer release shortly after it starts and every
// updateCheckInterval, downloads it in the background and puts it in place
// when it quits, so the next start runs it; "Restart to update" applies it
// at once. With autoUpdate off, the Status section's check and download
// buttons do the same steps by hand.
const (
	updateCheckDelay    = 20 * time.Second
	updateCheckInterval = 6 * time.Hour
	updateDownloadLimit = 30 * time.Minute
)

// updateState is the update in progress; App.update.mu guards it.
type updateState struct {
	mu      sync.Mutex
	status  model.UpdateStatus
	release update.Release
	staged  *update.Staged
	// busy is set while a check or download runs.
	busy bool
	// applied is set once the staged files are in place, so quitting does
	// not apply them again.
	applied bool
	cancel  context.CancelFunc
	once    sync.Once
}

// updateStagingDir is where downloaded releases wait.
func updateStagingDir() string {
	base := appdir.Config()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "updates")
}

func (a *App) updateClient() *update.Client { return update.NewClient(version.UserAgent()) }

// GetVersion is the running build's version, empty in development.
func (a *App) GetVersion() string { return version.Current() }

// GetUpdateStatus returns the update's state for the settings page.
func (a *App) GetUpdateStatus() model.UpdateStatus {
	a.update.mu.Lock()
	defer a.update.mu.Unlock()
	return a.update.status
}

// setUpdateStatus changes the status under the lock and tells the page.
func (a *App) setUpdateStatus(change func(status *model.UpdateStatus)) model.UpdateStatus {
	a.update.mu.Lock()
	change(&a.update.status)
	status := a.update.status
	a.update.mu.Unlock()
	a.emitEvent("update:status", status)
	return status
}

// startUpdateChecks restores an update staged by an earlier run and, with
// autoUpdate on, checks GitHub now and then.
func (a *App) startUpdateChecks() {
	a.update.once.Do(func() {
		a.setUpdateStatus(func(status *model.UpdateStatus) {
			status.Current = version.Current()
			status.Platform = runtime.GOOS + "/" + runtime.GOARCH
			status.State = "idle"
		})
		a.restoreStagedUpdate()
		go a.updateCheckLoop()
	})
}

// restoreStagedUpdate keeps an update that was downloaded but not applied
// (the program was killed, or autoUpdate was off) and drops one that is not
// newer than this build.
func (a *App) restoreStagedUpdate() {
	dir := updateStagingDir()
	if dir == "" {
		return
	}
	staged, ok := update.LoadStaged(dir)
	if !ok {
		_ = update.Discard(dir)
		return
	}
	if !update.IsNewer(version.Current(), staged.Version) {
		_ = update.Discard(dir)
		return
	}
	a.update.mu.Lock()
	a.update.staged = &staged
	a.update.mu.Unlock()
	a.setUpdateStatus(func(status *model.UpdateStatus) {
		status.State = "ready"
		status.Latest = staged.Version
		status.ReleaseURL = staged.URL
		status.Progress = 100
	})
	a.addLog("Info", "Update", "MAYAK "+staged.Version+" is downloaded and will be installed when MAYAK quits")
}

func (a *App) updateCheckLoop() {
	timer := time.NewTimer(updateCheckDelay)
	defer timer.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-timer.C:
		}
		a.mu.RLock()
		automatic := a.settings.AutoUpdate
		a.mu.RUnlock()
		if automatic {
			if _, err := a.checkForUpdates(true); err != nil {
				a.addLog("Warn", "Update", "Update check failed: "+err.Error())
			}
		}
		timer.Reset(updateCheckInterval)
	}
}

// CheckForUpdates asks GitHub for the latest release now. With autoUpdate
// on, a newer release starts downloading.
func (a *App) CheckForUpdates() (model.UpdateStatus, error) {
	a.mu.RLock()
	automatic := a.settings.AutoUpdate
	a.mu.RUnlock()
	return a.checkForUpdates(automatic)
}

func (a *App) checkForUpdates(download bool) (model.UpdateStatus, error) {
	a.update.mu.Lock()
	if a.update.busy {
		status := a.update.status
		a.update.mu.Unlock()
		return status, nil
	}
	a.update.busy = true
	a.update.mu.Unlock()
	a.setUpdateStatus(func(status *model.UpdateStatus) { status.State = "checking"; status.LastError = "" })
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	release, err := a.updateClient().Latest(ctx)
	cancel()
	if err != nil {
		a.update.mu.Lock()
		a.update.busy = false
		a.update.mu.Unlock()
		return a.setUpdateStatus(func(status *model.UpdateStatus) {
			status.State = "error"
			status.LastError = err.Error()
			status.CheckedAt = time.Now().Format(time.RFC3339)
			if a.update.staged != nil {
				status.State = "ready"
			}
		}), err
	}
	a.update.mu.Lock()
	a.update.release = release
	a.update.busy = false
	staged := a.update.staged
	a.update.mu.Unlock()
	_, supported := release.Archive()
	newer := update.IsNewer(version.Current(), release.Tag)
	status := a.setUpdateStatus(func(status *model.UpdateStatus) {
		status.Latest = release.Version()
		status.ReleaseURL = release.URL
		status.ReleaseName = release.Name
		status.Notes = release.Notes
		status.PublishedAt = release.PublishedAt.Format(time.RFC3339)
		status.CheckedAt = time.Now().Format(time.RFC3339)
		status.LastError = ""
		switch {
		case staged != nil && !update.IsNewer(staged.Version, release.Tag):
			status.State = "ready"
			status.Latest = staged.Version
		case !newer:
			status.State = "current"
		case !supported:
			status.State = "unsupported"
		default:
			status.State = "available"
			status.Progress = 0
		}
	})
	if status.State == "available" {
		a.addLog("Info", "Update", "MAYAK "+release.Version()+" is available (this is "+version.Current()+")")
		if download {
			go a.downloadUpdate()
		}
	}
	return status, nil
}

// DownloadUpdate downloads the release the last check found.
func (a *App) DownloadUpdate() error {
	a.update.mu.Lock()
	state := a.update.status.State
	a.update.mu.Unlock()
	if state != "available" && state != "error" {
		return fmt.Errorf("no update to download")
	}
	go a.downloadUpdate()
	return nil
}

func (a *App) downloadUpdate() {
	a.update.mu.Lock()
	if a.update.busy || a.update.release.Tag == "" {
		a.update.mu.Unlock()
		return
	}
	release := a.update.release
	a.update.busy = true
	ctx, cancel := context.WithTimeout(a.ctx, updateDownloadLimit)
	a.update.cancel = cancel
	a.update.mu.Unlock()
	defer func() {
		cancel()
		a.update.mu.Lock()
		a.update.busy = false
		a.update.cancel = nil
		a.update.mu.Unlock()
	}()
	dir := updateStagingDir()
	if dir == "" {
		a.setUpdateStatus(func(status *model.UpdateStatus) {
			status.State = "error"
			status.LastError = "no configuration directory"
		})
		return
	}
	a.setUpdateStatus(func(status *model.UpdateStatus) {
		status.State = "downloading"
		status.Progress = 0
		status.LastError = ""
	})
	lastPercent := -1
	staged, err := a.updateClient().Stage(ctx, release, dir, func(done, total int64) {
		if total <= 0 {
			return
		}
		percent := int(done * 100 / total)
		if percent != lastPercent {
			lastPercent = percent
			a.setUpdateStatus(func(status *model.UpdateStatus) { status.Progress = percent })
		}
	})
	if err != nil {
		_ = update.Discard(dir)
		if errors.Is(err, context.Canceled) {
			a.setUpdateStatus(func(status *model.UpdateStatus) { status.State = "available"; status.Progress = 0 })
			return
		}
		a.addLog("Warn", "Update", "Could not download MAYAK "+release.Version()+": "+err.Error())
		a.setUpdateStatus(func(status *model.UpdateStatus) { status.State = "error"; status.LastError = err.Error() })
		return
	}
	a.update.mu.Lock()
	a.update.staged = &staged
	a.update.mu.Unlock()
	a.setUpdateStatus(func(status *model.UpdateStatus) {
		status.State = "ready"
		status.Progress = 100
		status.Latest = staged.Version
	})
	a.addLog("Info", "Update", "MAYAK "+staged.Version+" is downloaded; it is installed when MAYAK quits, or now from Status")
}

// InstallUpdate puts the downloaded release in place and restarts into it.
func (a *App) InstallUpdate() error {
	// The path to start again, taken before the files move.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := a.applyStagedUpdate(); err != nil {
		return err
	}
	if err := update.Relaunch(exe); err != nil {
		a.addLog("Error", "Update", "Could not restart MAYAK: "+err.Error())
		return fmt.Errorf("the update is installed; start MAYAK again (%w)", err)
	}
	a.addLog("Info", "Update", "Restarting into the new version")
	go a.quit()
	return nil
}

// applyStagedUpdate replaces the program's files with the staged ones.
func (a *App) applyStagedUpdate() error {
	a.update.mu.Lock()
	staged, applied, busy := a.update.staged, a.update.applied, a.update.busy
	a.update.mu.Unlock()
	if applied {
		return nil
	}
	if staged == nil || busy {
		return fmt.Errorf("no update is ready to install")
	}
	dir, err := update.InstallDir()
	if err != nil {
		return err
	}
	if err := update.Apply(*staged, dir); err != nil {
		a.addLog("Error", "Update", "Could not install MAYAK "+staged.Version+": "+err.Error())
		a.setUpdateStatus(func(status *model.UpdateStatus) { status.LastError = err.Error() })
		return err
	}
	a.update.mu.Lock()
	a.update.applied = true
	a.update.mu.Unlock()
	if staging := updateStagingDir(); staging != "" {
		_ = update.Discard(staging)
	}
	a.addLog("Info", "Update", "Installed MAYAK "+staged.Version+" into "+dir)
	return nil
}

// applyStagedUpdateOnExit installs a downloaded update as MAYAK quits, when
// autoUpdate is on.
func (a *App) applyStagedUpdateOnExit() {
	a.mu.RLock()
	automatic := a.settings.AutoUpdate
	a.mu.RUnlock()
	a.update.mu.Lock()
	if cancel := a.update.cancel; cancel != nil {
		cancel()
	}
	ready := a.update.staged != nil && !a.update.applied && !a.update.busy
	a.update.mu.Unlock()
	if !automatic || !ready {
		return
	}
	_ = a.applyStagedUpdate()
}
