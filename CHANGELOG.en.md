# Changelog

What changed in each version of MAYAK, from the user's side. The Japanese version is [CHANGELOG.md](CHANGELOG.md); the published page is https://mayak.ich.sh/changelog, which the app's update notice opens for "What changed".

Format: newest version first, a `## v0.1.8 (2026-09-26)` heading and bullet points. Unreleased changes collect under `## Unreleased` and get a version heading at release time.

## Unreleased

- Choosing an own image for the player position marker could leave the marker unchanged: the file's kind was told from its content alone, and a PNG or SVG that could not be told that way was refused. The extension decides when the content cannot, and a refusal names the file and what it read as. The preview in the settings now draws tarkov.dev's actual marker (a green disc with a white rim and a white arrow).
- The player position marker keeps its size: the effects (white outline, glows, ring, beacon) reach beyond the icon, but the icon, and an own image, stay at tarkov.dev's 24 px. The "Own image" card opens the file chooser when no image is chosen yet.

## v0.1.15 (2026-09-26)

- The marker for your position on tarkov.dev's map can be replaced (Settings → tarkov.dev → Player position marker): five that stand out (large with a white outline, red glow, green glow, pulsing red ring, yellow beacon) or your own image (PNG / SVG / JPEG / WebP / GIF). The heading stays the rotation tarkov.dev applies. Only maps opened in MAYAK's built-in browser are affected.
- The story tasks screen (Character → Tasks → Story) is recognized: the chapter's name ("Tour", "The Ticket", "Blue Fire", "They Are Already Here"…) is read and its page in the official wiki's "Story chapters" opens. tarkov.dev has no page for these tasks, so with tarkov.dev selected only the map shows.
- Items with a short name, such as the Dorm overseer key, were recognized as items but did not show in the item panel. Two causes: an equipment slot's frame ("Headwear" and the like) to the left of the window made the name crop extend over it, and Windows OCR read nothing from a short name at the left of a wide title bar. The name is read from the window's own left border, and only where text is drawn.

## v0.1.14 (2026-09-26)

- The built-in browser takes Chrome's keyboard shortcuts: Ctrl+T for a new tab, Ctrl+W to close one, Ctrl+Shift+T to reopen the last closed tab, Ctrl+Tab and Ctrl+Shift+Tab to switch tabs, Ctrl+1 to 9 for the nth tab, Ctrl+L, Alt+D and F6 for the address bar, Ctrl+D to bookmark. They work with the keyboard in a page too (Windows).
- Anything typed in the address bar that is not an address is searched on Google. Host names with a dot and `localhost:8080` open as before.
- Signing in to tarkov.dev with a Google account in the built-in browser got stuck at `accounts.google.com/gsi/transform`. Dialogs such as sign-in (a `window.open` with a size) now open in a popup window of their own instead of a tab, so they can report back to the page that opened them.
- Error notices such as "Could not complete the action" were hidden under the page. They now take a row of their own along the bottom of the window, like the update notice.
- The tab shown follows the last recognition: a position screenshot after a task screen brings the map tab back (the page stays as it is when it already shows that map).
- The tutorial's wording is brought up to date: screenshots taken in the game, no step count that contradicts its seven screens, and the 8-digit pairing code in the other-PC step.

## v0.1.13 (2026-09-26)

- The app icon is dressed like the official launcher's: a rounded (same ratio) dark grey plate, the mark in a silver gradient with a soft haze around it, fine grain and scanlines. The taskbar, tray, About, favicon and the site's mark share it.

## v0.1.12 (2026-09-26)

- The app icon is black and white like the site (a white mark on near-black), with the original shape; the taskbar, tray, About, favicon and the site's mark all use the same artwork. The About logo could show the old image from a cache; fixed.

## v0.1.11 (2026-09-26)

- The About section's "Source code" button is now "GitHub".
- The tutorial now says it right: to recognise a task, open it in the Tasks screen before taking the screenshot (the list alone does not show which task is selected).
- The "All tabs" page listed the fixed tabs and the app's own pages (settings, bosses and so on), with close buttons; it now lists exactly what the sidebar's "Tabs" section does.

## v0.1.10 (2026-09-26)

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
