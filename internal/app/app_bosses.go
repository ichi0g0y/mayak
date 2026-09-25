package app

import (
	"context"
	"errors"
	"time"

	"github.com/local/mayak/internal/bossinfo"
)

// BrowserBosses returns the bosses of each map and the latest Goons reports
// for the sidebar's boss section, in the game mode being played (mode "") or
// the one given, with names in lang.
func (a *App) BrowserBosses(mode, lang string) (bossinfo.Info, error) {
	if a.bossInfo == nil {
		return bossinfo.Info{}, errors.New("boss details are unavailable")
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	return a.bossInfo.Info(ctx, a.itemMode(mode), lang)
}
