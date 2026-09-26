package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/model"
	"github.com/local/mayak/internal/remote"
	"github.com/local/mayak/internal/remoteid"
)

// Remote MAYAK instances: sending maps, tasks and positions to them.

func (a *App) AutoDetectRemoteID() (string, error) {
	return remoteid.Detect()
}

func (a *App) sendMap(remoteID, mapName string) error {
	return a.remoteClient(remoteID).SendMap(a.ctx, mapName)
}

func (a *App) sendTask(remoteID, normalizedName string) error {
	return a.remoteClient(remoteID).SendTask(a.ctx, normalizedName)
}

func (a *App) remoteClient(remoteID string) *remote.Client {
	a.mu.Lock()
	if client := a.remotes[remoteID]; client != nil {
		a.mu.Unlock()
		return client
	}
	client := remote.New(remoteID)
	if remoteID == a.settings.BrowserRemoteID {
		client.OnSend(a.noteBrowserRemoteSend)
	}
	if a.remotes == nil {
		a.remotes = make(map[string]*remote.Client)
	}
	a.remotes[remoteID] = client
	a.mu.Unlock()
	return client
}

func (a *App) sendMapTargets(settings config.Settings, mapName, channel string) error {
	a.emitEvent("browser:map", mapName)
	var result error
	for _, id := range remoteTargetIDs(settings, channel) {
		result = errors.Join(result, a.sendMap(id, mapName))
	}
	return result
}

func (a *App) sendTaskTargets(settings config.Settings, normalizedName string) error {
	var result error
	for _, id := range remoteTargetIDs(settings, "tasks") {
		result = errors.Join(result, a.sendTask(id, normalizedName))
	}
	return result
}

func (a *App) runRemoteIfCurrent(sequence uint64, operation func() error) (bool, error) {
	a.remoteOperationMu.Lock()
	defer a.remoteOperationMu.Unlock()
	if a.quitting.Load() || a.analysisSequence.Load() != sequence {
		return false, nil
	}
	return true, operation()
}

func (a *App) recordRemoteCommandIfCurrent(sequence uint64, command string) {
	a.mu.Lock()
	if a.analysisSequence.Load() != sequence {
		a.mu.Unlock()
		return
	}
	a.status.Connection = "connected"
	a.status.LastRemoteCommand = command
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	a.addLog("Info", "Remote", "Sent "+command)
}

func (a *App) sendPosition(s config.Settings, p model.Position) error {
	var result error
	for _, id := range remoteTargetIDs(s, "map") {
		result = errors.Join(result, a.remoteClient(id).SendPosition(a.ctx, s.Map, p, s.NavigateMapOnShot))
	}
	return result
}

func (a *App) TestRemote() error {
	a.mu.RLock()
	s := a.settings
	a.mu.RUnlock()
	ids := remoteTargetIDs(s, "all")
	if len(ids) == 0 {
		return errors.New("Remote ID is required")
	}
	a.remoteOperationMu.Lock()
	defer a.remoteOperationMu.Unlock()
	if a.quitting.Load() {
		return context.Canceled
	}
	for _, id := range ids {
		if err := a.remoteClient(id).Connect(a.ctx); err != nil {
			a.setError(fmt.Errorf("%s: %w", id, err))
			return err
		}
	}
	a.mu.Lock()
	a.status.Connection = "connected"
	status := a.status
	a.mu.Unlock()
	a.emitEvent("status:update", status)
	a.addLog("Info", "Remote", fmt.Sprintf("Connection test succeeded for %d target(s)", len(ids)))
	return nil
}
