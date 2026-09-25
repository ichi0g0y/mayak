//go:build windows

package browserview

import (
	_ "embed"
	"encoding/json"

	"github.com/local/mayak/internal/adblock"
	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// collapseScript hides blocked elements and the empty slots they leave.
//
//go:embed collapse.js
var collapseScript string

// cosmeticScript adds the element hiding stylesheet once per document.
const cosmeticScript = `(css=>{const id="mayak-cosmetic";if(document.getElementById(id))return;
const style=document.createElement("style");style.id=id;style.textContent=css;
(document.head||document.documentElement).appendChild(style);
if(window.__mayakTidy)window.__mayakTidy();})`

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
	return f, nil
}

func (f *tabFilter) close() {
	f.closed = true
	f.chromium.WebResourceRequestedCallback = nil
	if f.removeDOM != nil {
		f.removeDOM()
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

func (f *tabFilter) contentLoaded() {
	if f.closed {
		return
	}
	css := f.blocker.CosmeticCSS(f.chromium.Source())
	if css == "" {
		return
	}
	data, _ := json.Marshal(css)
	_ = f.chromium.ExecuteScript(cosmeticScript + "(" + string(data) + ")")
}
