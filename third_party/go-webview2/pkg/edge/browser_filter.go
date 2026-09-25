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
	h := &browserHandler{&browserVtbl, func(uintptr) { fn() }, guid}
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
