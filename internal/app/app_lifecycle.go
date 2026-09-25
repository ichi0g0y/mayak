package app

import (
	"context"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.startup(ctx)
	return nil
}
func (a *App) ServiceShutdown() error { a.shutdown(a.ctx); return nil }
