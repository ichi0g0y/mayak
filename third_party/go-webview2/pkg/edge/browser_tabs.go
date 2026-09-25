//go:build windows

package edge

// MAYAK extension. All calls run on the WebView's owning UI thread.
import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"net/url"
	"strings"
	"unsafe"
)

type BrowserEvent struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	CanBack    bool   `json:"canBack"`
	CanForward bool   `json:"canForward"`
	Popup      bool   `json:"popup"`
	Favicon    string `json:"favicon"`
	// Loading is set while the page loads.
	Loading bool `json:"loading"`
}
type browserHandler struct {
	vtbl   *browserHandlerVtbl
	invoke func(uintptr)
	iid    windows.GUID
}
type browserHandlerVtbl struct{ query, add, release, invoke ComProc }

var browserVtbl = browserHandlerVtbl{
	NewComProc(func(h *browserHandler, iid, out uintptr) uintptr {
		if out == 0 {
			return 0x80004003
		}
		*(*uintptr)(unsafe.Pointer(out)) = 0
		if iid == 0 {
			return 0x80004003
		}
		requested := *(*windows.GUID)(unsafe.Pointer(iid))
		unknown := windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
		if requested != unknown && requested != h.iid {
			return 0x80004002
		}
		*(*uintptr)(unsafe.Pointer(out)) = uintptr(unsafe.Pointer(h))
		return 0
	}),
	NewComProc(func(h *browserHandler) uintptr { return 1 }),
	NewComProc(func(h *browserHandler) uintptr { return 1 }),
	NewComProc(func(h *browserHandler, sender, args uintptr) uintptr { h.invoke(args); return 0 }),
}

// devToolsPending keeps completion handlers alive until WebView2 calls them.
var devToolsPending = map[*browserHandler]struct{}{}

// devToolsCall runs a DevTools protocol method without waiting for its result.
// It must be called on the UI thread, which also runs the completion handler.
func (e *Chromium) devToolsCall(method, params string) error {
	w := e.webview
	if w == nil || e.shuttingDown {
		return errors.New("webview is not available")
	}
	name, err := windows.UTF16PtrFromString(method)
	if err != nil {
		return err
	}
	args, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return err
	}
	guid, _ := windows.GUIDFromString("{5C4889F0-5EF6-4C5A-952C-D8F1B92D0574}")
	h := &browserHandler{vtbl: &browserVtbl, iid: guid}
	h.invoke = func(uintptr) { delete(devToolsPending, h) }
	devToolsPending[h] = struct{}{}
	hr, _, _ := w.vtbl.CallDevToolsProtocolMethod.Call(uintptr(unsafe.Pointer(w)), uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(h)))
	if hr != 0 {
		delete(devToolsPending, h)
		return fmt.Errorf("devtools %s: 0x%x", method, hr)
	}
	return nil
}

func browserString(proc ComProc, obj uintptr) string {
	var p *uint16
	hr, _, _ := proc.Call(obj, uintptr(unsafe.Pointer(&p)))
	if hr != 0 || p == nil {
		return ""
	}
	defer CoTaskMemFree(unsafe.Pointer(p))
	return UTF16PtrToString(p)
}
func browserURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil && !strings.EqualFold(u.Hostname(), "wails.localhost")
}

// BrowserIsolation registers no host objects or document scripts. It disables
// web messaging and rejects local/custom protocols even after redirects.
func (e *Chromium) BrowserIsolation(notify func(BrowserEvent)) (func(), error) {
	settings, err := e.GetSettings()
	if err != nil {
		return nil, err
	}
	if err = settings.PutIsWebMessageEnabled(false); err != nil {
		return nil, err
	}
	e.MessageCallback = func(string) {}
	e.SetGlobalPermission(CoreWebView2PermissionStateDeny)
	w := e.webview
	ptr := uintptr(unsafe.Pointer(w))
	handlers := []*browserHandler{}
	removers := []func(){}
	add := func(proc, remove ComProc, iid string, fn func(uintptr)) error {
		guid, err := windows.GUIDFromString(iid)
		if err != nil {
			return err
		}
		h := &browserHandler{&browserVtbl, fn, guid}
		var token _EventRegistrationToken
		hr, _, _ := proc.Call(ptr, uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&token)))
		if hr != 0 {
			return fmt.Errorf("browser event: 0x%x", hr)
		}
		handlers = append(handlers, h)
		removers = append(removers, func() { remove.Call(ptr, uintptr(token.value)) })
		return nil
	}
	changed := func(uintptr) {
		var back, forward int32
		w.vtbl.GetCanGoBack.Call(ptr, uintptr(unsafe.Pointer(&back)))
		w.vtbl.GetCanGoForward.Call(ptr, uintptr(unsafe.Pointer(&forward)))
		notify(BrowserEvent{URL: browserString(w.vtbl.GetSource, ptr), Title: browserString(w.vtbl.GetDocumentTitle, ptr), CanBack: back != 0, CanForward: forward != 0, Favicon: e.faviconURI(), Loading: e.browserLoading})
	}
	cleanup := func() {
		for _, remove := range removers {
			remove()
		}
		for _, h := range handlers {
			h.invoke = func(uintptr) {}
		}
	}
	if err = add(w.vtbl.AddNavigationStarting, w.vtbl.RemoveNavigationStarting, "{9ADBE429-F36D-432B-9DDC-F8881FBD76E3}", func(args uintptr) {
		v := *(**[10]ComProc)(unsafe.Pointer(args))
		u := browserString(v[3], args)
		if !browserURL(u) {
			v[8].Call(args, 1)
			return
		}
		e.navigatingURL = u
		e.browserLoading = true
		changed(0)
	}); err != nil {
		cleanup()
		return nil, err
	}
	if err = add(w.vtbl.AddNavigationCompleted, w.vtbl.RemoveNavigationCompleted, "{D33A35BF-1C49-4F98-93AB-006E0533FE1C}", func(uintptr) {
		e.browserLoading = false
		changed(0)
	}); err != nil {
		cleanup()
		return nil, err
	}
	if err = add(w.vtbl.AddNewWindowRequested, w.vtbl.RemoveNewWindowRequested, "{D4C185FE-C81C-4989-97AF-2D3FA7AB5651}", func(args uintptr) {
		v := *(**[11]ComProc)(unsafe.Pointer(args))
		v[6].Call(args, 1)
		var user int32
		v[8].Call(args, uintptr(unsafe.Pointer(&user)))
		u := browserString(v[3], args)
		if user != 0 && browserURL(u) {
			notify(BrowserEvent{URL: u, Popup: true})
		}
	}); err != nil {
		cleanup()
		return nil, err
	}
	for index, pair := range [][2]ComProc{{w.vtbl.AddSourceChanged, w.vtbl.RemoveSourceChanged}, {w.vtbl.AddHistoryChanged, w.vtbl.RemoveHistoryChanged}, {w.vtbl.AddDocumentTitleChanged, w.vtbl.RemoveDocumentTitleChanged}} {
		if err = add(pair[0], pair[1], []string{"{3C067F9F-5388-4772-8B48-79F7EF1AB37C}", "{C79A420C-EFD9-4058-9295-3E8B4BCAB645}", "{F5F2B923-953E-4042-9F95-F3A118E1AFD4}"}[index], changed); err != nil {
			cleanup()
			return nil, err
		}
	}
	// FaviconChanged needs Runtime 1.0.1245; without it the favicon is still
	// read with the other page changes above.
	if w15 := e.webview15(); w15 != 0 {
		v := *(**[113]ComProc)(unsafe.Pointer(w15))
		guid, _ := windows.GUIDFromString("{2913DA94-833D-4DE0-8DCA-900FC524A1A4}")
		h := &browserHandler{&browserVtbl, changed, guid}
		var token _EventRegistrationToken
		if hr, _, _ := v[109].Call(w15, uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&token))); hr == 0 {
			handlers = append(handlers, h)
			removers = append(removers, func() { v[110].Call(w15, uintptr(token.value)); v[2].Call(w15) })
		} else {
			v[2].Call(w15)
		}
	}
	return cleanup, nil
}

// webview15 returns an AddRef'd ICoreWebView2_15 pointer, or 0 if unsupported.
// Its vtable continues ICoreWebView2 through _14 (109 entries including
// IUnknown): add_FaviconChanged, remove_FaviconChanged, get_FaviconUri.
func (e *Chromium) webview15() uintptr {
	iid, err := windows.GUIDFromString("{517B2D1D-7DAE-4A66-A4F4-10352FFB9518}")
	if err != nil || e.webview == nil {
		return 0
	}
	var w15 uintptr
	hr, _, _ := e.webview.vtbl.QueryInterface.Call(uintptr(unsafe.Pointer(e.webview)), uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&w15)))
	if hr != 0 {
		return 0
	}
	return w15
}

// faviconURI is the page's favicon URL, or "" when unknown or unsupported.
func (e *Chromium) faviconURI() string {
	w15 := e.webview15()
	if w15 == 0 {
		return ""
	}
	v := *(**[113]ComProc)(unsafe.Pointer(w15))
	defer v[2].Call(w15)
	if u := browserString(v[111], w15); browserURL(u) {
		return u
	}
	return ""
}
func (e *Chromium) BrowserCommand(command, rawURL string) error {
	w := e.webview
	var proc ComProc
	switch command {
	case "navigate":
		if !browserURL(rawURL) {
			return fmt.Errorf("invalid browser URL")
		}
		return w.Navigate(rawURL)
	case "back":
		proc = w.vtbl.GoBack
	case "forward":
		proc = w.vtbl.GoForward
	case "reload":
		// Like Ctrl+Shift+R: a normal reload keeps using cached files, so a
		// broken cache entry (a site's script cached mid-deploy) never heals.
		if err := e.devToolsCall("Page.reload", `{"ignoreCache":true}`); err == nil {
			return nil
		}
		proc = w.vtbl.Reload
	case "close":
		hr, _, _ := e.controller.vtbl.Close.Call(uintptr(unsafe.Pointer(e.controller)))
		if hr != 0 {
			return fmt.Errorf("browser close: 0x%x", hr)
		}
		return nil
	default:
		return fmt.Errorf("unknown browser command")
	}
	hr, _, _ := proc.Call(uintptr(unsafe.Pointer(w)))
	if hr != 0 {
		return fmt.Errorf("browser command: 0x%x", hr)
	}
	return nil
}
