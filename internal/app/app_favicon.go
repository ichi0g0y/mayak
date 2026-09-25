package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/local/mayak/internal/appdir"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// faviconCache keeps site icons on disk, so the browser sidebar shows them at
// once and offline. A page reporting its icon again refreshes the copy, at
// most every faviconRefresh, so a changed icon shows up.
type faviconCache struct {
	dir    string
	client *http.Client
	mu     sync.Mutex
	// allow vets icon URLs (faviconURLAllowed); tests replace it.
	allow func(string) bool
}

const (
	faviconRefresh  = 10 * time.Minute
	faviconMaxBytes = 256 << 10
)

func newFaviconCache() *faviconCache {
	dir, _ := os.UserConfigDir()
	return &faviconCache{dir: filepath.Join(dir, appdir.Name, "favicons"), client: &http.Client{Timeout: 10 * time.Second}, allow: faviconURLAllowed}
}

// BrowserFavicon returns the icon at rawURL as a data URL, from the cache
// unless refresh asks for a current copy.
func (a *App) BrowserFavicon(rawURL string, refresh bool) (string, error) {
	a.faviconOnce.Do(func() { a.favicons = newFaviconCache() })
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	return a.favicons.get(parent, rawURL, refresh)
}

func (c *faviconCache) get(ctx context.Context, rawURL string, refresh bool) (string, error) {
	if !c.allow(rawURL) {
		return "", errors.New("invalid favicon URL")
	}
	sum := sha256.Sum256([]byte(rawURL))
	base := filepath.Join(c.dir, hex.EncodeToString(sum[:16]))
	c.mu.Lock()
	cached, kind, modified, err := readFavicon(base)
	c.mu.Unlock()
	if err == nil && (!refresh || time.Since(modified) < faviconRefresh) {
		return dataURL(kind, cached), nil
	}
	data, kind2, fetchErr := c.fetch(ctx, rawURL)
	if fetchErr != nil {
		if err == nil {
			return dataURL(kind, cached), nil
		}
		return "", fetchErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if mkErr := os.MkdirAll(c.dir, 0o700); mkErr == nil {
		_ = os.WriteFile(base+".img", data, 0o600)
		_ = os.WriteFile(base+".type", []byte(kind2), 0o600)
	}
	return dataURL(kind2, data), nil
}

func (c *faviconCache) fetch(ctx context.Context, rawURL string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 MAYAK/0.1.0")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.New("favicon request returned " + resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, faviconMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > faviconMaxBytes {
		return nil, "", errors.New("favicon is empty or too large")
	}
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if !strings.HasPrefix(kind, "image/") {
		kind = http.DetectContentType(data)
		// DetectContentType does not know .ico files.
		if len(data) >= 4 && data[0] == 0 && data[1] == 0 && data[2] == 1 && data[3] == 0 {
			kind = "image/x-icon"
		}
	}
	if !strings.HasPrefix(kind, "image/") {
		return nil, "", errors.New("favicon is not an image")
	}
	return data, kind, nil
}

func readFavicon(base string) ([]byte, string, time.Time, error) {
	info, err := os.Stat(base + ".img")
	if err != nil {
		return nil, "", time.Time{}, err
	}
	data, err := os.ReadFile(base + ".img")
	if err != nil {
		return nil, "", time.Time{}, err
	}
	kind, err := os.ReadFile(base + ".type")
	if err != nil || !strings.HasPrefix(string(kind), "image/") {
		return nil, "", time.Time{}, errors.New("favicon cache entry is incomplete")
	}
	return data, string(kind), info.ModTime(), nil
}

func dataURL(kind string, data []byte) string {
	return "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// faviconURLAllowed accepts public web addresses only: icons are fetched by
// the app itself, so a page must not point it at the local machine or LAN.
func faviconURLAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return false
	}
	return true
}
