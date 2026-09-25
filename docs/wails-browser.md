# Wails browser workspace

MAYAK remains one Go + Wails v3 application. There is no Electron dependency,
browser helper process, local HTTP bridge, or local WebSocket bridge to start.

## Native tabs

`frontend/src/browser` provides the workspace, vertical/horizontal tabs, bookmarks,
task-site selection and manual WebRTC pairing. The bundled `settings.html` is a
trusted same-origin frame, kept mounted while its tab is hidden/closed so existing
settings saves finish. Its Go calls use the trusted parent's runtime.

External pages are separate native views inside the same Wails window:

- Windows: WebView2 controllers owned by Wails' UI thread, a separate browser data
  directory, no Wails script injection, web messaging disabled, permissions denied.
- macOS: plain WKWebViews with a fresh user-content controller, no Go bindings.
- Linux: GTK4/WebKitGTK 6.0 overlay views with independent contexts/content
  managers and permissions denied; GTK3/WebKitGTK 4.1 is available with `-tags gtk3`.

Navigation accepts HTTP/HTTPS only, excludes credentials and the trusted Wails
  host, and rechecks native navigation/redirects. Popups become tabs when accepted;
  they never become a Wails-bound document. Native sizing anchors the content to
  the remaining client area. Windows tab sizing follows native resize/DPI messages and shell zoom is disabled
  to keep native bounds aligned with the fixed tab/toolbar dimensions.

Wails `v3.0.0-beta.24` is used without a framework fork. Native adapters live
in `internal/browserview` and use v3's public native-window/UI-thread APIs.
`third_party/go-webview2` retains the small browser isolation extension and its
upstream license. See [v3 migration details](wails-v3-migration.md).

## Persistence and transport

Browser preferences, tabs and bookmarks are written on every confirmed change
to the user configuration directory's `Mayak/browser.json` using a synced
temporary file and replacement. Host settings and protected tracker tokens retain
their existing separate stores. Native navigation updates stored URLs/titles.
Native views are created lazily on activation and retained until their tab closes.
Automatic navigation is capped at 80 tabs; repeated tasks/maps reuse matching tabs
and pinned tabs are protected from replacement.

The trusted Wails view owns RTCPeerConnection. Only task/map display messages are
accepted by the receiver; no received message invokes arbitrary Go methods.
Users exchange expiring offer/answer codes manually. No TURN relay is configured
or accepted; STUN is discovery only. Pairing codes and SDP remain in memory and
must be exchanged again after restart/disconnection. This currently supports one
peer. Websites still load from their own servers; existing tarkov.dev Remote ID
marker synchronization continues to use tarkov.dev's service.

macOS/Linux start as receive-only clients. Selecting receiver/off on Windows stops
monitoring and cancels in-progress analysis. Screenshot maintenance and catalog
refresh are suppressed while in client mode. Switching back permits Host operation;
monitoring can be started from the Host settings tab.

## Verification

Completed without app startup, UI automation or connection tests:

- `bun test ./frontend/src/browser/state.test.js`: seven offline cases.
- `go test -run '^TestBrowser' ./internal/app`: temporary-file persistence and URL/ID rejection.
- `go test -run '^$' ./...`: Windows Go package compilation only.
- Wails binding generation, TypeScript/Vite production build, Windows production
  executable compilation.

With the user's explicit live-debugging permission (2026-09-23), Windows
verification also covered native tarkov.dev rendering, dragging the window narrower,
switching to settings immediately after resizing, vertical/horizontal tab placement,
and clicking a link in the external page after the layout change.

Windows external views now use separate clipped child HWNDs. Only the active view
is resized from WM_SIZE/WM_DPICHANGED; WM_MOVE/WM_MOVING notify WebView2 so popup
and input coordinates follow the parent. Automatic WebView2 monitor scaling is
retained instead of the old forced rasterization scale. The shell uses neutral
dark grays, and the embedded settings page wraps controls at narrow widths.

Initial saved geometry is applied through Wails window creation options instead
of native calls during ServiceStartup. Startup shows the saved normal/maximized window and restores tabs. The legacy
StartMinimized preference is no longer applied. Minimized 160x28/parked coordinates
are not persisted; normal geometry is retained when maximizing.

Still unverified: mixed-DPI monitor movement, WebRTC connection establishment,
macOS/Linux compilation and execution. Live UI testing is now authorized for this
task; the earlier request to defer it was superseded.

## Logo menu (2026-09-23)

Home tabs are removed on restoration while bookmarks and other tabs are retained.
The logo sits above vertical tabs or at the right end of horizontal tabs. Its
popover contains Settings and a nested Bookmarks submenu with links only.
Bookmark add/edit/delete controls live in Settings. New curated bookmarks are
merged once into existing profiles without overwriting names or restoring later deletions.
Closing every tab leaves an empty workspace; Settings can always reopen from the
logo. The new-tab button creates an empty addressable tab rather than a home page.
The adjacent mode icon exposes localized connection details on hover or focus.
The logo opens a native Wails context menu with a Bookmarks submenu. Native web
views remain visible while the menu is open. Host settings load on first selection,
not in a hidden frame during startup. WebView2 controllers initialize asynchronously
without a nested UI message loop; COM callback owners stay rooted until completion.
Content offsets are 224/48 CSS pixels for vertical tabs and 0/92 for horizontal.
Verified live on Windows: both placements, menu settings recovery, and status
explanation display. Browser state regression suite now has eight passing cases.

Windows verification (2026-09-23): cold launch, restored tabs, new blank/Web tabs,
menu over a live website, Host settings followed by tab creation, and maximize
geometry persistence. Chrome extensions are not installed: Windows WebView2 has
AreBrowserExtensionsEnabled and AddBrowserExtension (unpacked folder) APIs, but
Go bindings, lifecycle/UI, extension compatibility and licensing need separate work.
This does not imply Chrome extension support in WKWebView or WebKitGTK.

Window restoration stores display ID/name and work-area origin alongside the
normal rectangle. Moving a maximized window also transfers its saved normal
rectangle to the new display. Placement is resolved once Wails enumerates displays;
startup move/resize events cannot overwrite the saved placement before restoration.
Automated regression coverage includes negative coordinates, changed layout,
changed monitor handles, disconnected displays, and maximized monitor transfers.
This change has not been interactively tested: the user requested no cursor/focus
interference and permits screen operations only on monitor 2.
