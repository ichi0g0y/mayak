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
	"slices"
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

	cssMu   sync.Mutex
	css     map[string]Cosmetic
	generic map[string]string // Cosmetic.Generic, shared by the hosts with the same rules

	// Run when the lists are rebuilt or blocking is turned on or off: the
	// Mac's content blocker (WebKitRules) is made again or switched.
	hooksMu sync.Mutex
	hooks   []func()
}

// New creates a blocker that caches its lists in dir. It is enabled but blocks
// nothing until Start has loaded the lists.
func New(dir string, logf func(level, message string)) *Blocker {
	b := &Blocker{dir: dir, lists: DefaultLists, client: &http.Client{Timeout: 90 * time.Second}, logf: logf, css: map[string]Cosmetic{}, generic: map[string]string{}}
	b.enabled.Store(true)
	return b
}

func (b *Blocker) SetEnabled(enabled bool) {
	if b.enabled.Swap(enabled) != enabled {
		b.changed()
	}
}

// OnChange adds a function run (on its own goroutine) when the lists are
// rebuilt or blocking is turned on or off.
func (b *Blocker) OnChange(f func()) {
	b.hooksMu.Lock()
	defer b.hooksMu.Unlock()
	b.hooks = append(b.hooks, f)
}

func (b *Blocker) changed() {
	b.hooksMu.Lock()
	hooks := append([]func(){}, b.hooks...)
	b.hooksMu.Unlock()
	for _, f := range hooks {
		go f()
	}
}
func (b *Blocker) Enabled() bool { return b.enabled.Load() }

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
	b.css, b.generic = map[string]Cosmetic{}, map[string]string{}
	b.cssMu.Unlock()
	b.logf("Info", fmt.Sprintf("Ad blocking ready (%d filter lists)", len(lists)))
	b.changed()
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

// Cosmetic is how ad elements are hidden on a page.
type Cosmetic struct {
	// CSS is put on the page as it is: the rules written for its site, and
	// the generic ones no class or id narrows down (a few hundred).
	CSS string
	// Generic is the rest of the generic rules, one selector a line, each
	// starting with the class (".ad") or id ("#ad") it needs. EasyList has
	// some 13,000; put on every page they made a heavy one (the wiki's task
	// pages) restyle against all of them while it loaded, so the page adds
	// only those its elements have the class or id of (as uBlock Origin does).
	Generic string
}

// CosmeticCSS returns how ad elements are hidden on pageURL.
func (b *Blocker) CosmeticCSS(pageURL string) Cosmetic {
	engine := b.active(pageURL)
	if engine == nil {
		return Cosmetic{}
	}
	u, err := url.Parse(pageURL)
	if err != nil || u.Hostname() == "" {
		return Cosmetic{}
	}
	host := strings.ToLower(u.Hostname())
	b.cssMu.Lock()
	cosmetic, ok := b.css[host]
	b.cssMu.Unlock()
	if ok {
		return cosmetic
	}
	option := engine.MatchRequest(rules.NewRequest(pageURL, "", rules.TypeDocument)).GetCosmeticOption()
	hiding := engine.GetCosmeticResult(host, option).ElementHiding
	keyed, unkeyed := splitGeneric(hiding.Generic)
	selectors := [][]string{hiding.Specific}
	// urlfilter looks site rules up by exact hostname, so "fandom.com##.ad"
	// would miss escapefromtarkov.fandom.com. Add the parent domains' rules.
	for _, parent := range parentDomains(host) {
		selectors = append(selectors, engine.GetCosmeticResult(parent, option&^rules.CosmeticOptionGenericCSS).ElementHiding.Specific)
	}
	cosmetic.CSS = hidingCSS(append(selectors, unkeyed)...)
	b.cssMu.Lock()
	cosmetic.Generic = b.shared(strings.Join(keyed, "\n"))
	if len(b.css) >= maxCSSHosts {
		b.css = map[string]Cosmetic{}
	}
	b.css[host] = cosmetic
	b.cssMu.Unlock()
	return cosmetic
}

// shared returns the copy of generic already kept, so the hosts with the same
// generic rules (all but the few with exceptions) share one. b.cssMu is held.
func (b *Blocker) shared(generic string) string {
	if kept, ok := b.generic[generic]; ok {
		return kept
	}
	if len(b.generic) >= 8 {
		b.generic = map[string]string{}
	}
	b.generic[generic] = generic
	return generic
}

// splitGeneric sorts out the generic selectors starting with a class or id
// (".ad-box > img" needs an element with class ad-box) from the others
// ("[data-ad-name]", "AD-SLOT"). keyed is sorted, so the same rules make the
// same text.
func splitGeneric(selectors []string) (keyed, unkeyed []string) {
	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		switch {
		case selector == "" || strings.ContainsAny(selector, "{}\n") || strings.Contains(selector, "</"):
		case selectorKey(selector) != "":
			keyed = append(keyed, selector)
		default:
			unkeyed = append(unkeyed, selector)
		}
	}
	slices.Sort(keyed)
	return slices.Compact(keyed), unkeyed
}

// selectorKey returns ".name" or "#name" for a selector starting with that
// class or id, or "" (an escaped or non-ASCII name is not worth reading).
func selectorKey(selector string) string {
	if len(selector) < 2 || (selector[0] != '.' && selector[0] != '#') {
		return ""
	}
	end := 1
	for end < len(selector) && identChar(selector[end]) {
		end++
	}
	if end == 1 || (end < len(selector) && (selector[end] == '\\' || selector[end] >= 0x80)) {
		return ""
	}
	return selector[:end]
}

func identChar(c byte) bool {
	return c == '-' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
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
