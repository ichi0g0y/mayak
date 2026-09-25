# MAYAK implementation constraints

- The application must remain Go + Wails. Do not introduce Electron or an
  Electron-based Client. The user explicitly rejected that architecture.
- The former Electron source was retired outside the repository in
  `C:/Users/ichi0/Abyss/.cache/raidlens-electron-retired-20260922`.
  Ignored dependency/build directories may still exist under `client`; they are
  obsolete and are not part of the active application or its build.
- Browser tabs, bookmarks, and direct WebRTC display transport now have a Wails
  implementation. See `wails-browser.md` for architecture and verification limits.
  External sites use independent native views; only the trusted, bundled settings
  document uses a same-origin iframe for CSS isolation and uninterrupted saving.
- Preserve the Japanese OCR/catalog improvements, task website selection, and
  immediate settings persistence already implemented in the Wails application.
- The user plays EFT while development proceeds. Launching/restarting the app,
  manipulating windows, live OCR, or network connection tests require an explicit
  request to perform the actual test. Use static checks, compilation, and isolated
  non-network unit tests otherwise. Do not stop running processes for cleanup.

2026-09-22 rollback verification: Go compilation (`go test -run '^$' ./...`) and
the frontend production build passed. No app launch, network listener, window
operation, or connection test was performed during this rollback.

2026-09-22 Wails browser conversion: Windows native code and the frontend compile.
Offline state/pairing validation and local temporary-file persistence tests pass.
Native display, resizing, WebRTC connectivity and macOS/Linux builds still require
explicitly requested testing. No running application has been replaced or stopped.

2026-09-22 v3 migration: the user explicitly approved migration to Wails v3.
The project pins v3.0.0-beta.24, uses official Wails v3 without a local fork, and
keeps isolated native tabs in internal/browserview. See wails-v3-migration.md.
The prohibition on launching/restarting or live connection tests still applies.
