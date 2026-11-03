package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/local/mayak/internal/appdir"
	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/version"
	_ "golang.org/x/image/webp"
)

// prestigeIcons are the pictures of the Prestiges' badges, by Prestige, the
// Overall screen's badge is told by (profiledetect.PrestigeLevel): from the
// catalog's prestige iconLink (tarkov.dev), kept in the catalog folder (a
// cache of this PC, fetched again when missing) and read once.
var prestigeIcons = struct {
	sync.Mutex
	icons map[int]image.Image
	// retryAt is when the pictures missing are fetched again: not on every
	// Overall screen while tarkov.dev cannot be reached.
	retryAt time.Time
}{}

// prestigeIconRetry is how long the pictures had are used before the
// missing ones are fetched again.
const prestigeIconRetry = time.Hour

// prestigeClient fetches from tarkov.dev's assets only, redirects too.
var prestigeClient = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
	if len(via) > 3 || !strings.HasPrefix(req.URL.String(), prestigeAssets) {
		return http.ErrUseLastResponse
	}
	return nil
}}

const prestigeAssets = "https://assets.tarkov.dev/"

// prestigeIconsFor returns the pictures, nil when none could be had (the
// Prestige is then not told).
func (a *App) prestigeIconsFor(ctx context.Context, settings config.Settings) map[int]image.Image {
	prestigeIcons.Lock()
	defer prestigeIcons.Unlock()
	if prestigeIcons.icons != nil && (prestigeIcons.retryAt.IsZero() || time.Now().Before(prestigeIcons.retryAt)) {
		return prestigeIcons.icons
	}
	if a.catalogClient == nil {
		return nil
	}
	var tasks struct {
		Data struct {
			Prestige []struct {
				PrestigeLevel int    `json:"prestigeLevel"`
				IconLink      string `json:"iconLink"`
			} `json:"prestige"`
		} `json:"data"`
	}
	// The Prestiges are the same in every mode: the one played, else any kept.
	for _, mode := range []string{a.effectiveCatalogMode(settings.GameMode), "regular", "pve"} {
		if a.catalogClient.Get(ctx, mode, "tasks", &tasks) == nil && len(tasks.Data.Prestige) > 0 {
			break
		}
	}
	icons := make(map[int]image.Image, len(tasks.Data.Prestige))
	for _, p := range tasks.Data.Prestige {
		if p.PrestigeLevel < 1 || !strings.HasPrefix(p.IconLink, prestigeAssets) {
			continue
		}
		img, err := prestigeIcon(ctx, p.PrestigeLevel, p.IconLink)
		if err != nil {
			a.addLog("Warn", "Profile", fmt.Sprintf("Could not get the picture of Prestige %d: %s", p.PrestigeLevel, err))
			continue
		}
		icons[p.PrestigeLevel] = img
	}
	// Those had are used; one missing is fetched again in a while.
	prestigeIcons.icons, prestigeIcons.retryAt = icons, time.Time{}
	if len(icons) == 0 || len(icons) < len(tasks.Data.Prestige) {
		prestigeIcons.retryAt = time.Now().Add(prestigeIconRetry)
	}
	return icons
}

// prestigeIcon reads a Prestige's picture from the cache, else from link.
func prestigeIcon(ctx context.Context, level int, link string) (image.Image, error) {
	path, err := appdir.Path("catalog", fmt.Sprintf("prestige-%d-icon.webp", level))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("User-Agent", version.UserAgent())
		resp, getErr := prestigeClient.Do(req)
		if getErr != nil {
			return nil, getErr
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", link, resp.Status)
		}
		if data, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err != nil {
			return nil, err
		}
		_ = os.WriteFile(path, data, 0o600)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		_ = os.Remove(path)
	}
	return img, err
}
