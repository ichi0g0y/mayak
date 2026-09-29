# Changelog

What changed in each version of MAYAK, from the user's side. The Japanese version is [CHANGELOG.md](CHANGELOG.md); the published page is https://mayak.ich.sh/changelog, which the app's update notice opens for "What changed".

Format: newest version first, a `## v0.1.8 (2026-09-26)` heading and bullet points under `### New

- "Map", as a trial (nightly build only): a map over the whole page, opened from "Map" at the top of the sidebar. It has tarkov.dev's maps, markers and icons: filters show extracts, transits, spawns, bosses, locks, minefields, containers, loose loot, task objectives and more, and a search finds them. The floor follows your height, or is chosen from a list or with Ctrl + wheel, and how faint the other floors show is a setting. Settings also switch between Abstract (SVG) and Satellite, between PvP, PvE and Season data, set the size of extract and place names and keep sniper spawns always shown. A marker tells more when pressed, and a task objective opens the task's page. A button on the left makes the map as it shows into a snap note.
- Squads: under "Squad" on the map, create a squad and share its code: where each of you takes a screenshot in a raid shows on everyone's map with a name, a colour and the direction faced. The display name starts as your TarkovTracker name. Pressing a squadmate in the list shows them, on their map and floor. A recent squad is joined again with one press. A Host and its Clients join and leave squads together. Only display names and positions are shared, encrypted with the squad code; the relay cannot read them.
- "Your pen" on the map: the pencil in the column of buttons on the left draws lines of your own on the map, kept by map and floor for the next time it opens. It has an eraser, six colours, three widths, undo (Ctrl+Z), hiding your lines and removing all of them on the floor shown; while drawing, right-drag or Space + drag moves the map. A snap note of the map carries the lines as its first layer, to erase or add to later.
- A "Squad" section in the sidebar and a squad page (nightly build only). The section shows how many are in the squad and whether it is connected, then each squadmate and their map; pressing one shows them on the map. Creating, joining, the display name and leaving are on the squad page, with a "Squad colour" for your marker on the map (one someone else has cannot be chosen; Automatic takes a free one).
- A "Squad pen" on the map (nightly build only): the whole squad draws on the same map at once, each in their squad colour, seeing lines as they are drawn and where the others' pens are. Only your own lines can be erased; "Clear all" removes the map's squad lines for everyone (undoable for a few seconds). Pointing at a line names who drew it. Those who join later, or come back after a drop, get the lines too. A snap note of the map carries the squad's lines as its second layer.

### Improved

- The connection to other PCs (from the Host to Clients) goes through a relay (mayak-relay.ich.sh): no network keeps it from connecting any more, and paired PCs connect again on their own at every start (and after a drop). What they say is encrypted with a key only the paired PCs have; the relay cannot read it. The 8-digit code from the Host, entered on the other PC, is all it takes (no response code any more), and a Host takes any number of PCs. Each Client chooses what it shows: tasks, maps and positions, items. The STUN setting is gone.
- The TARKOV.DEV and TarkovTracker fixed tabs are gone; they are bookmarks pinned to the sidebar. Map detections show on "Map".
- The connection status and the game mode (PvP, PvE, Season) at the top of the sidebar open the setting they come from when pressed. The game mode is bold, without a frame.
- Settings → Connection names the modes "Host: Escape from Tarkov runs on this PC", "Client: Escape from Tarkov runs on another PC (the Host)" and "Off (nothing detected or received)", with a line on the Host and the Client each.
- With a TarkovTracker key registered, its state shows left of the monitoring switch: green synced, amber connecting or waiting for a profile, red when the profile played has no key or it fails. It opens the TarkovTracker settings.
- The item panel's scroll bar shows only while it scrolls or the pointer moves over it.
- The update channel under Settings → About MAYAK can be chosen on a Mac and on Linux too (it was on the Windows Host only).
- On a Mac the window's close, minimize and zoom buttons are macOS's own red, yellow and green ones at the top left, instead of Windows-style ones at the top right: the green one goes full screen, zooms with Option held and shows the tiling menu when held down. The sidebar and the toolbar move right for them (not in full screen).
- The sidebar's collapse button moved from the top of the sidebar to the buttons at its bottom, with an arrow that shows which way it opens or closes, so it is easy to tell from the placement button. In the collapsed sidebar the "Tabs" heading is an icon too (it opens the list of tabs).
- Squad codes, the codes pairing other PCs and tarkov.dev Remote IDs show as dots, and the eye button beside one shows it; what is typed into those fields shows as dots too. A stream or a screenshot no longer gives a code away.

### Fixed

- A position found in a screenshot with coordinates did not reach the Client.
- The corners of an item's picture in the item panel looked faint.
- Settings → TarkovTracker: an unassigned key's remove button sat out of place and broke the layout; it is a trash can now. The page is as wide as the other settings, and adding a key has links to open TarkovTracker and to create API tokens. The "keys waiting to be assigned" box is gone: the keys added are always listed (no longer folded), each led by its name on TarkovTracker, and a key is assigned on a profile below.

## v0.1.18 (2026-09-29)

### New

- Snap notes: the brush button in the built-in browser's toolbar captures what is on screen or the whole page, to draw on with a pen. A note taken from a page stays linked to it: the page's button counts its notes and its menu opens them. Notes can be unlinked, or started from a blank sheet, an image file or an image on the clipboard. "Snap notes" in the sidebar opens the list.
- Snap notes draw on layers: three layers over the original image, which is never edited; the original and each layer can be hidden.
- Snap notes take text. With "T" in the editor, click where the text goes and type; click a text to rewrite it, drag it to move it (while typing, with the handle above the box). The palette sets its font (any installed), colour (any colour too), size, and outline colour (auto, a swatch or any colour) and width. The caret is white or black against the picture under it.
- The snap note editor has viewing adjustments: brightness, contrast and lift shadows sliders make dark screenshots easier to look into. The original image stays as it is.
- Snap notes have Share: copy the picture with its drawing to the clipboard, save it as a PNG file under a name you choose, or post it to X (the picture is copied and a new post opens; paste it with Ctrl+V).
- Snap notes can be starred. The star on a card and next to the name in the editor toggles it, the list has a Favorites filter (and a card can be deleted from its other corner, with a second press), and a page's menu lists its starred notes first.
- The screenshot viewer has "Make a snap note": it turns a game screenshot into a standalone snap note, opened for drawing.
- A snap note made from a game screenshot with a position keeps where it was taken (map, position, direction). "Show on tarkov.dev" in the editor shows that position and direction on the map tab (and the maps connected by Remote Control); a screenshot whose map is not known lets you choose it. A post to X carries the place and a link to the tarkov.dev map (a tarkov.dev link cannot hold a position, so the position and direction go as text).
- The built-in browser has a translate button (toolbar, web tabs). WebView2 has no page translation of its own, so the page is reopened through Google Translate (translate.goog); pressing again returns to the page itself. Settings → Tasks → "Open the official wiki (English) translated" opens task pages on the official wiki translated from the start. A screenshot of a task whose page is open translated goes to that tab, translated as it is. On the translated official wiki, Fandom's right rail (a sign-up card, since a translated page is always signed out) is hidden so the article has the width.
- The toolbar's icons on the right (translate, snap notes, wiki search, open in the default browser) can be reordered by dragging; the order is kept across restarts.
- Notifications can speak. Choose a voice under Settings → Sounds → Voice for all, one of five VOICEVOX characters (Kasukabe Tsumugi, Amehare Hau, WhiteCUL, Haruka Nana, Kenzaki Mesuo), and each notification is said in that character's own words (in Japanese), or an English voice (female or male); it starts as Kasukabe Tsumugi in Japanese and the English female voice in English, and the beeps are still there. Each notification can also have its own voice (one button puts them all back on the voice for all), with what it says shown under it and its own volume adjustment (a slider, up or down from the volume for all), and the credits and terms are under Licenses & credits. A file of your own is chosen with Custom.
- Settings → About has Licenses & credits: MAYAK's license, the data it shows, the built-in voices' credits and terms, and the bundled libraries' licenses (THIRD_PARTY_NOTICES.txt).
- Nightly builds are now published. Set "Update channel" under Settings → About MAYAK to Nightly to update to the version in development, built every day. On nightly, a newer stable release is installed instead.
- Assigning a TarkovTracker key to a profile now syncs that profile's past logs by itself, with the count shown in a toast. Before, only progress made after the assignment was sent, and earlier progress needed a manual sync.

### Improved

- Snap notes: the menu shows over the page, the editor fills the page area and zooms with Ctrl+wheel or its toolbar, and captures of translated pages leave out Google Translate's bar.
- The error sound is split: "Task not identified", "Screenshot could not be read", and "Map link error" each turn on or off and take a file of their own. Item screenshots have "Item recognized" and "Item not identified" too (off at first). Failed Hideout actions are no longer notified (EFT only logs them and nothing can be done about them; the Logs page still shows them under Hideout).
- Notifications that come at once play one after another instead of over each other. On a new install sounds are off; turned on, "Match found", "Raid started" and the run-through alert play from the start too.
- The quest items reminder at the raid's start is now a notification back from a raid (kind words that fit a survival and a death alike: the logs do not tell them apart), and a game start and the game closing have their own. The failed tasks reminder plays back at the menu too instead of at the raid's start, when it was too late to restart them.
- A sound file of your own chosen for a notification is copied into MAYAK's data folder, so it still plays after the original is moved or deleted.
- TarkovTracker's past-log sync now lives on each profile's row. Every profile with a key has "Recheck past logs", which reads that profile's logs from its first session and sends them. The shared panel at the bottom and its wipe/version choice are gone (a profile is one wipe). By default it used to send only what followed the latest game version.
- TarkovTracker's "Recheck past logs" is not available while EFT is running: a sync then could overwrite a task state changed during play with an older one. The sync that follows a key assignment also waits until EFT closes. A profile whose past logs were never rechecked shows the button in red, with a note under its row.
- Pressing the sidebar's monitoring dot with no Screenshots folder set opens Settings → Folders instead of starting the watch, so a failed folder detection leads straight to where the folder is chosen.
- The CPU-heavy first seconds after a start are fixed. Replaying the hideout logs at start-up rewrote the whole history file and sent the whole status to the window once per event, hundreds of times. Writes are now coalesced half a second later, the window gets one status every 0.3 s during a replay, and duplicates are told by a set instead of a scan.
- Screenshot thumbnails are kept on disk too (`Mayak\thumbs`). They were made again at every start, which kept the CPU busy for seconds right after it.
- Lighter: the catalog check every five minutes no longer rebuilds the task list and the hideout when they did not change (new flea prices update the items only).
- Internal: the pixel access and brightness code the detectors and the OCR preprocessing each had of their own lives in `internal/imaging`, reading pixels straight from the byte buffer. Results are unchanged; a screenshot is analysed faster.

### Fixed

- A task not yet in tarkov.dev (the To the Light series and others) or a story chapter opened on the Japanese wiki as a search, not its page. New tasks now take their trader from the official wiki, and story chapters open under ストーリータスク. The task site dropdown no longer shows tarkov.dev after the Japanese wiki is chosen.
- A screenshot of the "To the Light - Getting Acquainted" task was not recognized.
- Story chapters with a large picture (They Are Already Here, Accidental Witness and the like) are recognized as task screens again, and the picture beside a chapter's name is no longer read as letters.
- The first story task recognized after a start (Batya and the like) no longer fails once and matches only when the same screen is captured again. Story chapters come from the list read from the official wiki, and the first recognition matched without waiting for it. The list is now read at start, and the first match waits for it (up to 10 seconds).
- After a search from the built-in browser's address bar, the address bar kept the words typed instead of showing the pages the tab went to (a link followed included).
- Requests to web services always gave the app's version as 0.1.0.
- The tutorial's "open the map" step now opens the map tab.

## v0.1.17 (2026-09-26)

### New

- Settings → Startup and window has "Keep the window at normal priority" (off by default). With a priority manager such as Process Lasso's ProBalance lowering MAYAK's priority during its start-up CPU work, the window's WebView2 processes spawned meanwhile keep the lower class and the UI looks frozen under load; switched on, MAYAK checks itself and its window processes every 10 seconds and puts them back to normal. It showed with `task dev`, which starts the app again after every rebuild.

### Improved

- Assigning TarkovTracker keys is simpler. A key you add goes onto the only EFT profile of its mode, when its mode has one latest profile. The settings go in two steps: Step 1 "Add TarkovTracker keys" (the token input with the keys waiting to be assigned right under it, one click putting a key on the profile played last, and a folded "Manage keys") and Step 2 "Assign keys to EFT profiles" (a panel per mode, each profile with its key and a dropdown to change it; an account’s older profiles fold away and only the latest is marked). Opening a dropdown no longer shifts the page.

## v0.1.16 (2026-09-26)

### Improved

- The player position marker is now tarkov.dev's own icon with an effect: the effect (none, outline, glow, pulsing ring, beacon) and its colour (white for the outline, red otherwise, until chosen) are picked separately. The icon keeps its size; only the effect reaches beyond it. 0.1.15's "own image" and "large" are gone, and what was chosen carries over as an effect (the green glow as glow in green). The preview in the settings draws tarkov.dev's actual marker (a green disc with a white rim and arrow).

## v0.1.15 (2026-09-26)

### New

- The marker for your position on tarkov.dev's map can be replaced (Settings → tarkov.dev → Player position marker): five that stand out (large with a white outline, red glow, green glow, pulsing red ring, yellow beacon) or your own image (PNG / SVG / JPEG / WebP / GIF). The heading stays the rotation tarkov.dev applies. Only maps opened in MAYAK's built-in browser are affected.
- The story tasks screen (Character → Tasks → Story) is recognized: the chapter's name ("Tour", "The Ticket", "Blue Fire", "They Are Already Here"…) is read and its page in the official wiki's "Story chapters" opens. tarkov.dev has no page for these tasks, so with tarkov.dev selected only the map shows.

### Fixed

- Items with a short name, such as the Dorm overseer key, were recognized as items but did not show in the item panel. Two causes: an equipment slot's frame ("Headwear" and the like) to the left of the window made the name crop extend over it, and Windows OCR read nothing from a short name at the left of a wide title bar. The name is read from the window's own left border, and only where text is drawn.

## v0.1.14 (2026-09-26)

### New

- The built-in browser takes Chrome's keyboard shortcuts: Ctrl+T for a new tab, Ctrl+W to close one, Ctrl+Shift+T to reopen the last closed tab, Ctrl+Tab and Ctrl+Shift+Tab to switch tabs, Ctrl+1 to 9 for the nth tab, Ctrl+L, Alt+D and F6 for the address bar, Ctrl+D to bookmark. They work with the keyboard in a page too (Windows).
- Anything typed in the address bar that is not an address is searched on Google. Host names with a dot and `localhost:8080` open as before.

### Improved

- The tab shown follows the last recognition: a position screenshot after a task screen brings the map tab back (the page stays as it is when it already shows that map).
- The tutorial's wording is brought up to date: screenshots taken in the game, no step count that contradicts its seven screens, and the 8-digit pairing code in the other-PC step.

### Fixed

- Signing in to tarkov.dev with a Google account in the built-in browser got stuck at `accounts.google.com/gsi/transform`. Dialogs such as sign-in (a `window.open` with a size) now open in a popup window of their own instead of a tab, so they can report back to the page that opened them.
- Error notices such as "Could not complete the action" were hidden under the page. They now take a row of their own along the bottom of the window, like the update notice.

## v0.1.13 (2026-09-26)

### Improved

- The app icon is dressed like the official launcher's: a rounded (same ratio) dark grey plate, the mark in a silver gradient with a soft haze around it, fine grain and scanlines. The taskbar, tray, About, favicon and the site's mark share it.

## v0.1.12 (2026-09-26)

### Improved

- The app icon is black and white like the site (a white mark on near-black), with the original shape; the taskbar, tray, About, favicon and the site's mark all use the same artwork. The About logo could show the old image from a cache; fixed.

## v0.1.11 (2026-09-26)

### Improved

- The About section's "Source code" button is now "GitHub".
- The tutorial now says it right: to recognise a task, open it in the Tasks screen before taking the screenshot (the list alone does not show which task is selected).

### Fixed

- The "All tabs" page listed the fixed tabs and the app's own pages (settings, bosses and so on), with close buttons; it now lists exactly what the sidebar's "Tabs" section does.

## v0.1.10 (2026-09-26)

### New

- The "Tabs" heading in the sidebar opens a page listing every open tab, searchable by name and address. Sidebar text can no longer be selected by accident.

### Improved

- The app icon is now silver on dark grey, so it sits next to the official launcher's without clashing (Windows taskbar, Start menu and tray, macOS Dock).
- "Check for updates" moved from Status to About, with this version, the latest one, the last check, and the download and restart buttons. The tray's right-click menu gets "Check for updates" too.

## v0.1.9 (2026-09-26)

### New

- Update notice: while a newer version is found, downloading or ready, a status strip shows along the bottom of the window. "Restart to apply" installs it right away, "Later" hides it for that version; it still installs when MAYAK quits.

### Improved

- The update check runs right after start (without holding it up) and, when it fails, retries after five minutes, backing off to an hour. Release information comes through mayak.ich.sh, so GitHub's rate limit no longer gets in the way.
- "What changed" opens this changelog instead of GitHub.
- The goon report's lookup and submission no longer hold up the interface either.

### Fixed

- The window is redrawn by difference instead of rebuilt, which ends the flicker on hover and the clicks that were lost mid-redraw. Links to a page already open (the changelog, About) switch to that tab.

## v0.1.8 (2026-09-26)

### Fixed

- Fixed the sidebar and settings button not responding for minutes right after start or on a flaky connection. Item price refreshes, searches and picks sat in the interface's action queue while they waited on the network; they now run separately.

## v0.1.7 (2026-09-26)

### New

- Pairing another PC now uses an 8-digit code: enter the number shown on the Host into the other PC. Exchanging the long codes by hand remains available.

### Improved

- The window resizes from its left, right and bottom edges through Windows' own invisible sizing border, so the corner cursor is easy to find and nothing inside, the item panel's scrollbar included, is covered.

### Fixed

- The landing page reads the latest release through mayak.ich.sh (it showed "could not fetch the latest version" when GitHub's rate limit hit).

## v0.1.6 (2026-09-25)

### Improved

- Release files carry the version in their names (`Mayak-Setup-0.1.6-windows-amd64.exe` and so on). Automatic updates from 0.1.5 or earlier cannot find the new names, so this one version needs a manual install.

## v0.1.5 (2026-09-25)

### Improved

- When started at a low priority by a launcher, MAYAK puts itself back at normal priority; left low, the window looked frozen whenever the PC was busy.

## v0.1.4 (2026-09-25)

### Fixed

- Fixed the first-run tutorial sometimes showing only its dark backdrop, ahead of the page, and blocking the window. A click on the backdrop closes it too.

## v0.1.3 (2026-09-25)

### New

- A first-run tutorial in seven steps: folders, the screenshot key, the map, use from another PC and TarkovTracker. It is always available again under Settings → Appearance.
- "About MAYAK" in the settings: version, website, source code, release notes, licenses and credits.

### Improved

- Quitting with the settings open no longer brings the settings back on the next start; it starts on the map.
- The item panel's right padding matches the left.

## v0.1.2 (2026-09-25)

### New

- macOS disk images (Apple Silicon and Intel) join the downloads.
- The landing page at https://mayak.ich.sh went live.

## v0.1.1 (2026-09-25)

### New

- A Windows installer (`Mayak-Setup-…exe`): per user, no administrator rights, with a Start menu entry and an uninstaller.

## v0.1.0 (2026-09-25)

### New

- Automatic updates from GitHub Releases: checked at start and every six hours, downloaded in the background and applied when MAYAK quits.
- First public release.
