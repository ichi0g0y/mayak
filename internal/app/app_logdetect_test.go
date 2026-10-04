package app

import (
	"context"
	"testing"

	"github.com/local/mayak/internal/applog"
	"github.com/local/mayak/internal/logdetect"
	"github.com/local/mayak/internal/model"
)

// The last position goes when a raid starts or ends and when the map
// changes: it is not where the player is any more.
func TestLastPositionClears(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	placed := func() *App {
		return &App{ctx: ctx, logs: applog.New(10), status: model.Status{CurrentMap: "customs", RaidActive: true, Position: &model.Position{X: 1, Y: 2, Z: 3}}}
	}

	for _, kind := range []logdetect.EventKind{logdetect.RaidStarted, logdetect.RaidExited} {
		a := placed()
		a.handleLogEvent(logdetect.Event{Kind: kind})
		if a.status.Position != nil {
			t.Errorf("%s kept the last position", kind)
		}
	}

	a := placed()
	a.handleMap("woods")
	if a.status.Position != nil {
		t.Error("a new map kept the last position")
	}

	// The same map (detected again) keeps it.
	a = placed()
	a.status.RaidActive = false
	a.handleMap("customs")
	if a.status.Position == nil {
		t.Error("the same map dropped the position")
	}
}
