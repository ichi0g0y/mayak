package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/local/mayak/internal/catalog"
	"github.com/local/mayak/internal/model"
)

func (a *App) effectiveCatalogMode(configured string) string {
	if configured != "" && configured != "auto" {
		return configured
	}
	a.mu.RLock()
	mode := a.status.Tracker.Mode
	a.mu.RUnlock()
	switch mode {
	case "pve":
		return "pve"
	case "pvp":
		return "regular"
	case "seasonal":
		return "pvp-season"
	default:
		return "auto"
	}
}

func (a *App) RefreshCatalog() error { return a.refreshCatalog(true) }

// catalogChanged reports whether a part of mode's catalog differs from the
// version seen last, and remembers the new one.
func (a *App) catalogChanged(mode, part, version string) bool {
	a.catalogVersionsMu.Lock()
	defer a.catalogVersionsMu.Unlock()
	if a.catalogVersions == nil {
		a.catalogVersions = make(map[string]string)
	}
	key := mode + "/" + part
	if a.catalogVersions[key] == version {
		return false
	}
	a.catalogVersions[key] = version
	return true
}

func (a *App) refreshCatalog(force bool) error {
	a.catalogRefreshMu.Lock()
	defer a.catalogRefreshMu.Unlock()
	if a.quitting.Load() || a.browserClient.Load() {
		return context.Canceled
	}
	a.mu.RLock()
	configured := a.settings.GameMode
	a.mu.RUnlock()
	mode := a.effectiveCatalogMode(configured)
	if !catalog.ValidMode(mode) {
		return errors.New("EFT game mode has not been detected; select a game mode or start log monitoring")
	}
	a.mu.Lock()
	// A refresh abandoned for another mode puts back what was shown before it,
	// unless a newer refresh has started meanwhile.
	a.catalogRefreshes++
	refresh, previous := a.catalogRefreshes, a.status.Catalog
	a.status.Catalog.State = "loading"
	a.status.Catalog.LastError = ""
	status := a.status
	a.mu.Unlock()
	a.emitStatus(status)
	abandon := func() error {
		a.mu.Lock()
		if a.catalogRefreshes != refresh {
			a.mu.Unlock()
			return context.Canceled
		}
		a.status.Catalog = previous
		status := a.status
		a.mu.Unlock()
		a.emitStatus(status)
		return context.Canceled
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	snapshot, err := a.catalogClient.Refresh(ctx, mode, force)
	a.mu.RLock()
	currentMode := a.settings.GameMode
	a.mu.RUnlock()
	if a.effectiveCatalogMode(currentMode) != mode {
		return abandon()
	}
	data := model.CatalogStatus{State: "error", Mode: mode}
	hideoutChanged := true
	if snapshot != nil {
		data = model.CatalogStatus{State: "ready", Mode: mode, UpdatedAt: snapshot.UpdatedAt.Format(time.RFC3339), Items: snapshot.Items, Maps: snapshot.Maps, Traders: snapshot.Traders, Tasks: snapshot.Tasks, HideoutStations: snapshot.HideoutStations, ScavCooldownSeconds: snapshot.ScavCooldownSeconds, PlayerLevels: snapshot.PlayerLevels}
		if time.Since(snapshot.UpdatedAt) >= catalog.RefreshInterval {
			data.State = "stale"
		}
		// Only what changed is built again: a refresh that brought new flea
		// prices keeps the task list, which is costly to build.
		if a.catalogChanged(mode, "tasks", snapshot.Version(catalog.TaskResources...)) {
			a.questClient.Invalidate()
		}
		if a.catalogChanged(mode, "items", snapshot.Version(catalog.ItemResources...)) {
			a.itemClient.Invalidate()
		}
		hideoutChanged = a.catalogChanged(mode, "hideout", snapshot.Version(catalog.HideoutResources...))
	}
	if err != nil {
		data.LastError = err.Error()
	}
	a.mu.RLock()
	hideoutKept := !hideoutChanged && a.hideoutCatalogMode == mode && a.hideoutStations != nil
	a.mu.RUnlock()
	var stations []catalog.HideoutStation
	var hideoutErr error
	if !hideoutKept {
		stations, hideoutErr = a.catalogClient.Hideout(ctx, mode)
	}
	a.mu.Lock()
	currentCatalogMode := a.settings.GameMode
	if currentCatalogMode == "" || currentCatalogMode == "auto" {
		currentCatalogMode = catalogMode(a.status.Tracker.Mode)
	}
	if currentCatalogMode != mode {
		a.mu.Unlock()
		return abandon()
	}
	if !hideoutKept && hideoutErr == nil {
		a.hideoutStations = stations
		a.hideoutCatalogMode = mode
	}
	if !hideoutKept {
		a.updateHideoutLocked()
	}
	a.status.Catalog = data
	status = a.status
	a.mu.Unlock()
	a.emitStatus(status)
	if err != nil {
		a.addLog("Warn", "Catalog", "Catalog update failed for "+mode+": "+err.Error())
		if snapshot != nil {
			a.addLog("Warn", "Catalog", "Using last-known-good catalog from "+data.UpdatedAt)
		}
		return err
	}
	a.addLog("Info", "Catalog", fmt.Sprintf("Loaded %s catalog from tarkov.dev: %d items, %d maps, %d traders, %d tasks, %d hideout stations (%d player levels; base Scav cooldown %ds)", mode, data.Items, data.Maps, data.Traders, data.Tasks, data.HideoutStations, data.PlayerLevels, data.ScavCooldownSeconds))
	return nil
}

func (a *App) watchCatalog() {
	a.mu.RLock()
	configured := a.settings.GameMode
	a.mu.RUnlock()
	if catalog.ValidMode(a.effectiveCatalogMode(configured)) {
		_ = a.refreshCatalog(false)
	}
	ticker := time.NewTicker(catalog.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.done:
			return
		case <-ticker.C:
			a.mu.RLock()
			configured = a.settings.GameMode
			a.mu.RUnlock()
			if catalog.ValidMode(a.effectiveCatalogMode(configured)) {
				_ = a.refreshCatalog(false)
			}
		}
	}
}
