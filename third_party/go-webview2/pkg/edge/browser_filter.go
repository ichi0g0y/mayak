//go:build windows

package edge

// MAYAK extension: content blocking hooks for browser tabs. Unlike the
// upstream helpers, these return errors instead of exiting the process, since
// they also run while a tab is being closed. All calls run on the UI thread.
import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// GetResourceContext reports what kind of resource the request loads.
func (i *ICoreWebView2WebResourceRequestedEventArgs) GetResourceContext() (COREWEBVIEW2_WEB_RESOURCE_CONTEXT, error) {
	var ctx COREWEBVIEW2_WEB_RESOURCE_CONTEXT
	hr, _, _ := i.vtbl.GetResourceContext.Call(uintptr(unsafe.Pointer(i)), uintptr(unsafe.Pointer(&ctx)))
	if windows.Handle(hr) != windows.S_OK {
		return 0, windows.Errno(hr)
	}
	return ctx, nil
}

// Source is the URL of the top-level document.
func (e *Chromium) Source() string {
	if e.webview == nil {
		return ""
	}
	return browserString(e.webview.vtbl.GetSource, uintptr(unsafe.Pointer(e.webview)))
}

// NavigatingURL is the URL of the latest top-level navigation, including
// redirects. BrowserIsolation records it; WebResourceRequested uses it to tell
// the main document apart from iframes, which share the DOCUMENT context.
func (e *Chromium) NavigatingURL() string { return e.navigatingURL }

// FilterAllRequests routes every request of this tab through
// WebResourceRequestedCallback.
func (e *Chromium) FilterAllRequests() error {
	if e.webview == nil {
		return errors.New("webview is not initialised")
	}
	return e.webview.AddWebResourceRequestedFilter("*", COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL)
}

// BlockRequest answers the request with an empty 403 response, so it never
// reaches the network.
func (e *Chromium) BlockRequest(args *ICoreWebView2WebResourceRequestedEventArgs) error {
	if e.environment == nil {
		return errors.New("environment is not initialised")
	}
	response, err := e.environment.CreateWebResourceResponse(nil, 403, "Blocked", "")
	if err != nil {
		return err
	}
	defer response.Release()
	return args.PutResponse(response)
}

// ExecuteScript runs script in the top-level document and ignores the result.
func (e *Chromium) ExecuteScript(script string) error {
	if e.webview == nil || e.shuttingDown {
		return errors.New("webview is not available")
	}
	err := e.webview.ExecuteScript(script, nil)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		return nil
	}
	return err
}

// OnDOMContentLoaded calls fn when a top-level document has been parsed. The
// returned function unregisters it and must be kept to keep the handler alive.
func (e *Chromium) OnDOMContentLoaded(fn func()) (func(), error) {
	if e.webview == nil {
		return nil, errors.New("webview is not initialised")
	}
	webview2, err := e.webview.QueryInterface2()
	if err != nil {
		return nil, err
	}
	guid, err := windows.GUIDFromString("{4BAC7E9C-199E-49ED-87ED-249303ACF019}")
	if err != nil {
		webview2.Release()
		return nil, err
	}
	h := &browserHandler{vtbl: &browserVtbl, invoke: func(uintptr) { fn() }, iid: guid}
	ptr := uintptr(unsafe.Pointer(webview2))
	var token _EventRegistrationToken
	hr, _, _ := webview2.vtbl.AddDomContentLoaded.Call(ptr, uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&token)))
	if windows.Handle(hr) != windows.S_OK {
		webview2.Release()
		return nil, windows.Errno(hr)
	}
	return func() {
		h.invoke = func(uintptr) {}
		webview2.vtbl.RemoveDomContentLoaded.Call(ptr, uintptr(token.value))
		webview2.Release()
	}, nil
}

// OnNavigationStarting calls fn with the URL of each top-level navigation
// (redirects included) to a web page, before its document is created. The
// returned function unregisters it and must be kept to keep the handler alive.
func (e *Chromium) OnNavigationStarting(fn func(uri string)) (func(), error) {
	w := e.webview
	if w == nil {
		return nil, errors.New("webview is not initialised")
	}
	guid, err := windows.GUIDFromString("{9ADBE429-F36D-432B-9DDC-F8881FBD76E3}")
	if err != nil {
		return nil, err
	}
	h := &browserHandler{vtbl: &browserVtbl, iid: guid, invoke: func(args uintptr) {
		v := *(**[4]ComProc)(unsafe.Pointer(args))
		if u := browserString(v[3], args); browserURL(u) {
			fn(u)
		}
	}}
	ptr := uintptr(unsafe.Pointer(w))
	var token _EventRegistrationToken
	hr, _, _ := w.vtbl.AddNavigationStarting.Call(ptr, uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&token)))
	if windows.Handle(hr) != windows.S_OK {
		return nil, windows.Errno(hr)
	}
	return func() {
		h.invoke = func(uintptr) {}
		w.vtbl.RemoveNavigationStarting.Call(ptr, uintptr(token.value))
	}, nil
}

// SetDocumentScript makes script run as each document is created, before the
// page's own scripts, in place of the script it set before ("" sets none).
// Adding a script completes later: a script still being added when the next
// call comes is removed once it is in.
func (e *Chromium) SetDocumentScript(script string) error {
	w := e.webview
	if w == nil || e.shuttingDown {
		return errors.New("webview is not available")
	}
	e.documentScriptGen++
	gen := e.documentScriptGen
	if e.documentScriptID != "" {
		removeDocumentScript(w, e.documentScriptID)
		e.documentScriptID = ""
	}
	if script == "" {
		return nil
	}
	js, err := windows.UTF16PtrFromString(script)
	if err != nil {
		return err
	}
	guid, _ := windows.GUIDFromString("{B99369F3-9B11-47B5-BC6F-8E7895FCEA17}")
	h := &browserHandler{vtbl: &browserVtbl, iid: guid}
	h.result = func(code, value uintptr) {
		delete(devToolsPending, h)
		if code != 0 || value == 0 {
			return
		}
		id := UTF16PtrToString((*uint16)(unsafe.Pointer(value)))
		if gen != e.documentScriptGen || e.webview == nil {
			removeDocumentScript(w, id)
			return
		}
		e.documentScriptID = id
	}
	devToolsPending[h] = struct{}{}
	hr, _, _ := w.vtbl.AddScriptToExecuteOnDocumentCreated.Call(uintptr(unsafe.Pointer(w)), uintptr(unsafe.Pointer(js)), uintptr(unsafe.Pointer(h)))
	if hr != 0 {
		delete(devToolsPending, h)
		return windows.Errno(hr)
	}
	return nil
}

func removeDocumentScript(w *ICoreWebView2, id string) {
	p, err := windows.UTF16PtrFromString(id)
	if err != nil {
		return
	}
	w.vtbl.RemoveScriptToExecuteOnDocumentCreated.Call(uintptr(unsafe.Pointer(w)), uintptr(unsafe.Pointer(p)))
}

// SetDefaultBackground sets the color shown before a page paints. Unlike
// SetBackgroundColour it returns errors instead of exiting.
func (e *Chromium) SetDefaultBackground(r, g, b uint8) error {
	if e.controller == nil {
		return errors.New("controller is not initialised")
	}
	controller2 := e.controller.GetICoreWebView2Controller2()
	if controller2 == nil {
		return errors.New("ICoreWebView2Controller2 is unavailable")
	}
	return controller2.PutDefaultBackgroundColor(COREWEBVIEW2_COLOR{A: 255, R: r, G: g, B: b})
}
