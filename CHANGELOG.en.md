# Changelog

What changed in each version of MAYAK, from the user's side. The Japanese version is [CHANGELOG.md](CHANGELOG.md); the published page is https://mayak.ich.sh/changelog, which the app's update notice opens for "What changed".

Format: newest version first, a `## v0.1.8 (2026-09-26)` heading and bullet points. Unreleased changes collect under `## Unreleased` and get a version heading at release time.

## Unreleased

- The app icon is now silver on dark grey, so it sits next to the official launcher's without clashing (Windows taskbar, Start menu and tray, macOS Dock).
- "Check for updates" moved from Status to About, with this version, the latest one, the last check, and the download and restart buttons. The tray's right-click menu gets "Check for updates" too.
- The "Tabs" heading in the sidebar opens a page listing every open tab, searchable by name and address. Sidebar text can no longer be selected by accident.

## v0.1.9 (2026-09-26)

- The window is redrawn by difference instead of rebuilt, which ends the flicker on hover and the clicks that were lost mid-redraw. Links to a page already open (the changelog, About) switch to that tab.
- Update notice: while a newer version is found, downloading or ready, a status strip shows along the bottom of the window. "Restart to apply" installs it right away, "Later" hides it for that version; it still installs when MAYAK quits.
- The update check runs right after start (without holding it up) and, when it fails, retries after five minutes, backing off to an hour. Release information comes through mayak.ich.sh, so GitHub's rate limit no longer gets in the way.
- "What changed" opens this changelog instead of GitHub.
- The goon report's lookup and submission no longer hold up the interface either.

## v0.1.8 (2026-09-26)

- Fixed the sidebar and settings button not responding for minutes right after start or on a flaky connection. Item price refreshes, searches and picks sat in the interface's action queue while they waited on the network; they now run separately.

## v0.1.7 (2026-09-26)

- Pairing another PC now uses an 8-digit code: enter the number shown on the Host into the other PC. Exchanging the long codes by hand remains available.
- The window resizes from its left, right and bottom edges through Windows' own invisible sizing border, so the corner cursor is easy to find and nothing inside, the item panel's scrollbar included, is covered.
- The landing page reads the latest release through mayak.ich.sh (it showed "could not fetch the latest version" when GitHub's rate limit hit).

## v0.1.6 (2026-09-25)

- Release files carry the version in their names (`Mayak-Setup-0.1.6-windows-amd64.exe` and so on). Automatic updates from 0.1.5 or earlier cannot find the new names, so this one version needs a manual install.

## v0.1.5 (2026-09-25)

- When started at a low priority by a launcher, MAYAK puts itself back at normal priority; left low, the window looked frozen whenever the PC was busy.

## v0.1.4 (2026-09-25)

- Fixed the first-run tutorial sometimes showing only its dark backdrop, ahead of the page, and blocking the window. A click on the backdrop closes it too.

## v0.1.3 (2026-09-25)

- A first-run tutorial in seven steps: folders, the screenshot key, the map, use from another PC and TarkovTracker. It is always available again under Settings → Appearance.
- "About MAYAK" in the settings: version, website, source code, release notes, licenses and credits.
- Quitting with the settings open no longer brings the settings back on the next start; it starts on the map.
- The item panel's right padding matches the left.

## v0.1.2 (2026-09-25)

- macOS disk images (Apple Silicon and Intel) join the downloads.
- The landing page at https://mayak.ich.sh went live.

## v0.1.1 (2026-09-25)

- A Windows installer (`Mayak-Setup-…exe`): per user, no administrator rights, with a Start menu entry and an uninstaller.

## v0.1.0 (2026-09-25)

- Automatic updates from GitHub Releases: checked at start and every six hours, downloaded in the background and applied when MAYAK quits.
- First public release.
