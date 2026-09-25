// Package adblock blocks ads and trackers in the built-in browser with
// EasyList-style filter lists, using AdGuard's urlfilter engine.
//
// Network rules block requests; element hiding rules become a stylesheet that
// removes ad slots, so pages do not keep blank space where an ad would be.
package adblock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AdguardTeam/urlfilter"
	"github.com/AdguardTeam/urlfilter/filterlist"
	"github.com/AdguardTeam/urlfilter/rules"
)

// List is a filter list downloaded from URL and cached as Name.txt.
type List struct {
	ID   rules.ListID
	Name string
	URL  string
}

// DefaultLists cover ads and trackers, including Japanese sites.
var DefaultLists = []List{
	{ID: 1, Name: "easylist", URL: "https://easylist.to/easylist/easylist.txt"},
	{ID: 2, Name: "easyprivacy", URL: "https://easylist.to/easylist/easyprivacy.txt"},
	{ID: 3, Name: "adguard-japanese", URL: "https://filters.adtidy.org/extension/chromium/filters/7.txt"},
}

// Exempt sites are never filtered: MAYAK relies on them, and tarkov.dev
// is funded by its ads.
var Exempt = []string{"tarkov.dev"}

const (
	maxAge       = 4 * 24 * time.Hour // EasyList's own "Expires: 4 days"
	retryAfter   = 6 * time.Hour
	maxListBytes = 32 << 20
	maxCSSHosts  = 256
)

// Request kinds, as reported by the browser view.
const (
	KindSubdocument = "subdocument"
	KindScript      = "script"
	KindStylesheet  = "stylesheet"
	KindImage       = "image"
	KindMedia       = "media"
	KindFont        = "font"
	KindXHR         = "xhr"
	KindWebsocket   = "websocket"
	KindPing        = "ping"
	KindOther       = "other"
)

var requestTypes = map[string]rules.RequestType{
	KindSubdocument: rules.TypeSubdocument, KindScript: rules.TypeScript, KindStylesheet: rules.TypeStylesheet,
	KindImage: rules.TypeImage, KindMedia: rules.TypeMedia, KindFont: rules.TypeFont, KindXHR: rules.TypeXmlhttprequest,
	KindWebsocket: rules.TypeWebsocket, KindPing: rules.TypePing, KindOther: rules.TypeOther,
}

type Blocker struct {
	dir     string
	lists   []List
	client  *http.Client
	logf    func(level, message string)
	enabled atomic.Bool
	engine  atomic.Pointer[urlfilter.Engine]

	cssMu sync.Mutex
	css   map[string]string
}

// New creates a blocker that caches its lists in dir. It is enabled but blocks
// nothing until Start has loaded the lists.
func New(dir string, logf func(level, message string)) *Blocker {
	b := &Blocker{dir: dir, lists: DefaultLists, client: &http.Client{Timeout: 90 * time.Second}, logf: logf, css: map[string]string{}}
	b.enabled.Store(true)
	return b
}

func (b *Blocker) SetEnabled(enabled bool) { b.enabled.Store(enabled) }
func (b *Blocker) Enabled() bool           { return b.enabled.Load() }

// Start loads the cached lists, then keeps them up to date until ctx ends.
func (b *Blocker) Start(ctx context.Context) {
	if err := b.rebuild(); err != nil {
		b.logf("Debug", "No cached filter lists yet: "+err.Error())
	}
	for {
		updated, err := b.update(ctx)
		if err != nil {
			b.logf("Warn", "Could not update filter lists: "+err.Error())
		}
		if updated || b.engine.Load() == nil {
			if err := b.rebuild(); err != nil {
				b.logf("Warn", "Could not load filter lists: "+err.Error())
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryAfter):
		}
	}
}

// update downloads lists whose cache is missing or older than maxAge.
func (b *Blocker) update(ctx context.Context) (updated bool, err error) {
	if err = os.MkdirAll(b.dir, 0o700); err != nil {
		return false, err
	}
	var errs []error
	for _, list := range b.lists {
		path := b.path(list)
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) < maxAge {
			continue
		}
		if err := b.download(ctx, list, path); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", list.Name, err))
			continue
		}
		updated = true
	}
	return updated, errors.Join(errs...)
}

func (b *Blocker) download(ctx context.Context, list List, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, list.URL, nil)
	if err != nil {
		return err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxListBytes+1))
	if err != nil {
		return err
	}
	if len(body) > maxListBytes {
		return errors.New("list is too large")
	}
	// A captive portal or error page must not replace a working list.
	if head := strings.TrimSpace(string(body[:min(len(body), 256)])); !strings.HasPrefix(head, "[Adblock") && !strings.HasPrefix(head, "!") {
		return errors.New("response is not a filter list")
	}
	tmp, err := os.CreateTemp(b.dir, list.Name+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (b *Blocker) path(list List) string { return filepath.Join(b.dir, list.Name+".txt") }

func (b *Blocker) rebuild() error {
	var lists []filterlist.Interface
	for _, list := range b.lists {
		text, err := os.ReadFile(b.path(list))
		if err != nil {
			continue
		}
		lists = append(lists, filterlist.NewBytes(&filterlist.BytesConfig{ID: list.ID, RulesText: text}))
	}
	if len(lists) == 0 {
		return errors.New("no filter lists")
	}
	storage, err := filterlist.NewRuleStorage(lists)
	if err != nil {
		return err
	}
	b.engine.Store(urlfilter.NewEngine(storage))
	b.cssMu.Lock()
	b.css = map[string]string{}
	b.cssMu.Unlock()
	b.logf("Info", fmt.Sprintf("Ad blocking ready (%d filter lists)", len(lists)))
	return nil
}

// Block reports whether a subresource of pageURL should be blocked. Top-level
// documents are never passed here: the user opened them on purpose.
func (b *Blocker) Block(rawURL, pageURL, kind string) bool {
	engine := b.active(pageURL)
	if engine == nil {
		return false
	}
	typ, ok := requestTypes[kind]
	if !ok {
		typ = rules.TypeOther
	}
	rule := engine.MatchRequest(rules.NewRequest(rawURL, pageURL, typ)).GetBasicResult()
	return rule != nil && !rule.Whitelist
}

// CosmeticCSS returns a stylesheet that hides ad elements on pageURL, or "".
func (b *Blocker) CosmeticCSS(pageURL string) string {
	engine := b.active(pageURL)
	if engine == nil {
		return ""
	}
	u, err := url.Parse(pageURL)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	b.cssMu.Lock()
	css, ok := b.css[host]
	b.cssMu.Unlock()
	if ok {
		return css
	}
	option := engine.MatchRequest(rules.NewRequest(pageURL, "", rules.TypeDocument)).GetCosmeticOption()
	result := engine.GetCosmeticResult(host, option)
	selectors := [][]string{result.ElementHiding.Generic, result.ElementHiding.Specific}
	// urlfilter looks site rules up by exact hostname, so "fandom.com##.ad"
	// would miss escapefromtarkov.fandom.com. Add the parent domains' rules;
	// generic rules are already included above.
	for _, parent := range parentDomains(host) {
		selectors = append(selectors, engine.GetCosmeticResult(parent, option&^rules.CosmeticOptionGenericCSS).ElementHiding.Specific)
	}
	css = hidingCSS(selectors...)
	b.cssMu.Lock()
	if len(b.css) >= maxCSSHosts {
		b.css = map[string]string{}
	}
	b.css[host] = css
	b.cssMu.Unlock()
	return css
}

func (b *Blocker) active(pageURL string) *urlfilter.Engine {
	if !b.enabled.Load() || exempt(pageURL) {
		return nil
	}
	return b.engine.Load()
}

// parentDomains returns "b.example.com" and "example.com" for
// "a.b.example.com". Bare top-level domains are not included.
func parentDomains(host string) []string {
	var parents []string
	for i := strings.IndexByte(host, '.'); i >= 0; i = strings.IndexByte(host, '.') {
		host = host[i+1:]
		if !strings.Contains(host, ".") {
			break
		}
		parents = append(parents, host)
	}
	return parents
}

func exempt(pageURL string) bool {
	u, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, site := range Exempt {
		if host == site || strings.HasSuffix(host, "."+site) {
			return true
		}
	}
	return false
}

// hidingCSS writes one rule per selector: a single invalid selector in a
// selector list would otherwise disable the whole list. --rl marks elements
// hidden by MAYAK, so the page script can collapse the slots around them.
func hidingCSS(selectorLists ...[]string) string {
	var sb strings.Builder
	seen := map[string]bool{}
	for _, selectors := range selectorLists {
		for _, selector := range selectors {
			selector = strings.TrimSpace(selector)
			if selector == "" || seen[selector] || strings.ContainsAny(selector, "{}") || strings.Contains(selector, "</") {
				continue
			}
			seen[selector] = true
			sb.WriteString(selector)
			sb.WriteString("{display:none!important;--rl:1}\n")
		}
	}
	return sb.String()
}
