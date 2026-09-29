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
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
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
	dir, _ := appdir.Path("favicons")
	return &faviconCache{dir: dir, client: &http.Client{Timeout: 10 * time.Second}, allow: faviconURLAllowed}
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

// BrowserSiteIcon finds the icon of the page at pageURL without showing the
// page (a bookmark whose site was never opened): the icon its HTML names
// (<link rel="icon">, else apple-touch-icon), else /favicon.ico. The icon
// found is cached as BrowserFavicon caches; its URL is returned.
func (a *App) BrowserSiteIcon(pageURL string) (string, error) {
	a.faviconOnce.Do(func() { a.favicons = newFaviconCache() })
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	return a.favicons.siteIcon(parent, pageURL)
}

const siteIconPageBytes = 512 << 10

func (c *faviconCache) siteIcon(ctx context.Context, pageURL string) (string, error) {
	if !c.allow(pageURL) {
		return "", errors.New("invalid page URL")
	}
	var candidates []string
	if final, links, err := c.pageIcons(ctx, pageURL); err == nil {
		candidates = append(candidates, links...)
		candidates = append(candidates, faviconAt(final))
	}
	candidates = append(candidates, faviconAt(pageURL))
	seen := map[string]bool{}
	for _, u := range candidates {
		if u == "" || seen[u] || !c.allow(u) {
			continue
		}
		seen[u] = true
		if _, err := c.get(ctx, u, false); err == nil {
			return u, nil
		}
	}
	return "", errors.New("no icon found")
}

// faviconAt is /favicon.ico of the site of rawURL.
func faviconAt(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/favicon.ico"
}

// pageIcons reads the head of the page at pageURL and returns where it ended
// up (after redirects, each vetted as icon URLs are) and the icons it names,
// the best first.
func (c *faviconCache) pageIcons(ctx context.Context, pageURL string) (string, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 MAYAK/0.1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	client := *c.client
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !c.allow(r.URL.String()) {
			return errors.New("redirect not followed")
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	final := resp.Request.URL.String()
	if resp.StatusCode != http.StatusOK {
		return final, nil, errors.New("page request returned " + resp.Status)
	}
	if kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); kind != "" && !strings.Contains(kind, "html") {
		return final, nil, errors.New("page is not HTML")
	}
	return final, iconLinks(io.LimitReader(resp.Body, siteIconPageBytes), resp.Request.URL), nil
}

// iconLinks reads an HTML head for the icons it names, as absolute URLs:
// rel="icon" (and "shortcut icon") in their order, then apple-touch-icon;
// mask-icon (a one-colour outline) is left out.
func iconLinks(r io.Reader, page *url.URL) []string {
	base := page
	var icons, touch []string
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tag, hasAttr := z.TagName()
		name := string(tag)
		if name == "body" {
			break
		}
		if (name != "link" && name != "base") || !hasAttr {
			continue
		}
		attrs := map[string]string{}
		for more := true; more; {
			var k, v []byte
			k, v, more = z.TagAttr()
			attrs[strings.ToLower(string(k))] = string(v)
		}
		href := strings.TrimSpace(attrs["href"])
		if href == "" {
			continue
		}
		if name == "base" {
			if u, err := page.Parse(href); err == nil {
				base = u
			}
			continue
		}
		u, err := base.Parse(href)
		if err != nil {
			continue
		}
		rels := strings.Fields(strings.ToLower(attrs["rel"]))
		switch {
		case slices.Contains(rels, "icon"):
			icons = append(icons, u.String())
		case slices.Contains(rels, "apple-touch-icon") || slices.Contains(rels, "apple-touch-icon-precomposed"):
			touch = append(touch, u.String())
		}
	}
	return append(icons, touch...)
}
