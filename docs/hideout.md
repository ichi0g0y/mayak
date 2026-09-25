# Hideout observability

The Logs page includes a Hideout category and a compact progress card. Expand the station list to inspect the next incomplete level and its construction time, item, station, trader, and skill requirements. Requirements do not establish item ownership or construction eligibility.

Progress is read from TarkovTracker for the exact detected account/profile/mode and assigned key. Station catalogs remain isolated between regular PvP, Seasonal PvP, and PvE. Changing the profile, key, or catalog mode cannot reuse another identity's displayed progress. Hideout log events never write progress to TarkovTracker or imply construction completion.

Dedicated backend logs are preferred; output/errors are fallback sources. Historical files, newly discovered files, truncated files, and monitoring restarts initialize silently. Only newly appended actionable failures can produce an in-app alert and the configured sound. Notifications default to off. Enable Hideout error notifications in Sounds; actions whose outcome could not be confirmed are not notified. Sound volume and the master sound switch are shared with existing alerts.

The Diagnostics button opens `%APPDATA%/Mayak/hideout`. Its structured JSON retains at most 500 events for 90 days, newest first. Only event classification, action, area, timestamp, source basename, station name, and detected identity are stored. Raw log payloads, headers, cookies, gateway addresses, and URL queries are excluded. Clearing the log hides these entries while retaining bounded deduplication fingerprints across restart.

## Validation

```powershell
go test ./...
cd frontend
bun run build
cd ..
wails build
```

Optional local verification (no personal log fixtures are committed):

```powershell
$env:MAYAK_EFT_LOGS = '<EFT Logs directory>'
$env:MAYAK_LIVE_CATALOG = '1'
go test ./internal/hideoutlog ./internal/catalog -run 'TestRealEFTLogs|TestLiveHideoutAreaMapping' -v
```

The real-log check asserts the handoff's known baseline of twelve failed completion records representing four logical events. Update that expectation if later EFT sessions introduce additional failures. The live catalog check verifies area 22 maps to Defective Wall in all three modes.

Implementation validation on 2026-09-21: Go tests, frontend build, and Wails build passed. Local logs produced twelve records / four logical failures, with all identities resolved. Public catalogs passed all three area mappings. Browser rendering with synthetic backend data passed Japanese/English labels, filtering, chronological ordering, and requirement expansion without JavaScript errors.

Native launch validation remains incomplete: a separate test instance failed before application startup with WebView2 controller error `80080005` (Server execution failed). The existing running application was preserved. No commit was made because the handoff requires a successful native launch smoke test before committing.
