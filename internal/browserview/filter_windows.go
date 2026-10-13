//go:build windows

package browserview

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/local/mayak/internal/adblock"
	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/wails/v3/pkg/application"
)

var resourceKinds = map[edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT]string{
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_STYLESHEET:       adblock.KindStylesheet,
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_IMAGE:            adblock.KindImage,
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_MEDIA:            adblock.KindMedia,
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_FONT:             adblock.KindFont,
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_SCRIPT:           adblock.KindScript,
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_XML_HTTP_REQUEST: adblock.KindXHR,
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_FETCH:            adblock.KindXHR,
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_WEBSOCKET:        adblock.KindWebsocket,
	edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_PING:             adblock.KindPing,
}

// tabFilter applies a ContentBlocker to one tab. It is only used on the UI thread.
type tabFilter struct {
	chromium  *edge.Chromium
	blocker   ContentBlocker
	collapse  []string
	scheduled bool
	closed    bool
	removeDOM func()
	removeNav func()
}

func attachFilter(c *edge.Chromium, b ContentBlocker) (*tabFilter, error) {
	f := &tabFilter{chromium: c, blocker: b}
	if err := c.AddDocumentScript(collapseScript); err != nil {
		return nil, err
	}
	c.WebResourceRequestedCallback = f.request
	if err := c.FilterAllRequests(); err != nil {
		return nil, err
	}
	remove, err := c.OnDOMContentLoaded(f.contentLoaded)
	if err != nil {
		return nil, err
	}
	f.removeDOM = remove
	if f.removeNav, err = c.OnNavigationStarting(f.navigationStarting); err != nil {
		f.close()
		return nil, err
	}
	return f, nil
}

func (f *tabFilter) close() {
	f.closed = true
	f.chromium.WebResourceRequestedCallback = nil
	if f.removeDOM != nil {
		f.removeDOM()
	}
	if f.removeNav != nil {
		f.removeNav()
	}
}

func (f *tabFilter) request(req *edge.ICoreWebView2WebResourceRequest, args *edge.ICoreWebView2WebResourceRequestedEventArgs) {
	if f.closed {
		return
	}
	uri, err := req.GetUri()
	if err != nil {
		return
	}
	context, err := args.GetResourceContext()
	if err != nil {
		return
	}
	kind := resourceKinds[context]
	if context == edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT {
		// The page itself is never blocked; other documents are iframes.
		if uri == f.chromium.NavigatingURL() {
			return
		}
		kind = adblock.KindSubdocument
	}
	if kind == "" {
		kind = adblock.KindOther
	}
	if !f.blocker.Block(uri, f.chromium.Source(), kind) || f.chromium.BlockRequest(args) != nil {
		return
	}
	if kind == adblock.KindImage || kind == adblock.KindSubdocument || kind == adblock.KindMedia {
		f.collapseLater(uri)
	}
}

// collapseLater batches blocked URLs; scripts must not run inside the
// WebResourceRequested handler itself.
func (f *tabFilter) collapseLater(uri string) {
	f.collapse = append(f.collapse, uri)
	if f.scheduled {
		return
	}
	f.scheduled = true
	application.InvokeAsync(func() {
		urls := f.collapse
		f.collapse, f.scheduled = nil, false
		if f.closed {
			return
		}
		data, _ := json.Marshal(urls)
		_ = f.chromium.ExecuteScript("window.__mayakCollapse&&window.__mayakCollapse(" + string(data) + ")")
	})
}

// navigationStarting sets the hiding stylesheet of the page being opened to go
// in as its document is created, so the page never shows the ads (or the
// space they take) first. It only runs on that page's host: the next
// navigation may start before this one's document comes.
func (f *tabFilter) navigationStarting(uri string) {
	if f.closed {
		return
	}
	script := ""
	cosmetic := f.blocker.CosmeticCSS(uri)
	if u, err := url.Parse(uri); err == nil && cosmetic != (adblock.Cosmetic{}) {
		host, _ := json.Marshal(strings.ToLower(u.Hostname()))
		script = "if(location.hostname===" + string(host) + ")" + cosmeticCall(cosmetic.CSS, cosmetic.Generic)
	}
	_ = f.chromium.SetDocumentScript(script)
}

// contentLoaded adds the stylesheet when navigationStarting's script did not
// (it was not in yet, or the page's address changed on the way).
func (f *tabFilter) contentLoaded() {
	if f.closed {
		return
	}
	cosmetic := f.blocker.CosmeticCSS(f.chromium.Source())
	if cosmetic == (adblock.Cosmetic{}) {
		return
	}
	_ = f.chromium.ExecuteScript(cosmeticCall(cosmetic.CSS, cosmetic.Generic))
}
