# Changelog

What changed in each version of MAYAK, from the user's side. The Japanese version is [CHANGELOG.md](CHANGELOG.md); the published page is https://mayak.ich.sh/changelog, which the app's update notice opens for "What changed".

Format: newest version first, a `## v0.1.8 (2026-09-26)` heading and bullet points under `### New`, `### Improved` and `### Fixed` (a heading with nothing under it is left out). Changes not released yet collect under `## Nightly` and get a version heading at release time.

## v0.1.31 (2026-10-10)

### Fixed

- After a Prestige, an Overall screenshot taken right after resetting TarkovTracker could leave the level unsent: MAYAK dropped a level read before it noticed the reset (up to 5 minutes). Now a screenshot taken while the reset is awaited reads TarkovTracker again at once, and a level still not sendable is kept and sent when the reset is seen.
- The Overall screen's experience could be misread, and the level with it: part of the "EXP+" badge before it was read as a digit, making 5 200 into 15 200 (level 3 into level 5).

## v0.1.30 (2026-10-09)

### New

- When a position screenshot opens the map and your arrow is outside the view, the map moves just enough to bring it in, the zoom kept. An arrow already in view stays where it is. It works while the map setting "Show the map on a position screenshot" is on.
- The item panel says "No use" for a key that has none: its door or safe is always unlocked (the Health Resort rooms…), or it opens no lock (the Pumping station doors…). It is read from the "usage" of the key's page on the official wiki. Before, a key with no places showed nothing, so it was not clear whether it had no use or the data was missing. A key with places on tarkov.dev shows them as before.

### Improved

- The map's floor is picked with a floor button of its own in the button column left of the map. Before, the floors were under the list of maps, and each floor change in a raid needed the maps panel. On a map with no floors (Woods…) the floor button shows disabled.
- The map's floors are listed top floor first, like a lift's buttons. Before, the floors above came after the ground and the underground (Reserve's Bunkers…) came last, hard to find; it now comes right below the ground.

## v0.1.29 (2026-10-07)

### Fixed

- The Overall screen of the character sometimes had its level misread and not sent to TarkovTracker: the "1" of level 16 was dropped, 6 was read, and it did not match the 16 the experience makes. The text is now read with room around it, and when the readings differ, the level the experience makes is taken.
- A notice could say the level sent from the Overall screen went back on TarkovTracker (16 to 15) while it showed 16 all along: a save from an open TarkovTracker page crossing MAYAK's write can put an older level in for a while. The level is now read again minutes later, and only a level still lower gets a notice, worded without blaming the Automatic Level Calculation; once TarkovTracker reads at the level again, the notice goes.

## v0.1.28 (2026-10-07)

### Fixed

- A screenshot of the character screen's Tasks sometimes went unrecognized (PvE since 1.2.0.0, and PvP; in a raid and out of one). In a mode with the Prestige tab the Tasks tab sits elsewhere, and MAYAK did not see a task screen and took it for a position screenshot. It now also looks at the Side and Operational switch under the tabs, so the screen is recognized whatever mode or tabs. The task's name is also cut around its line of letters, so taller rows do not cut it off.

## v0.1.27 (2026-10-07)

### New

- Notices (toasts) show at the bottom middle of the window: what was sent to TarkovTracker, the alerts in words, what a screenshot was read as, the squad coming and going, updates and errors. They show over the pages too and take no focus from the game or the page. Errors, updates and the like stay until closed; the others go after a few seconds. Each kind can be turned off under Notifications in the settings, and the bell at the bottom of the sidebar shows their history (the last 200). The update and error strips along the bottom are now these notices.
- The Notifications settings are one table of the events (match found, raid started, task read, sent to TarkovTracker, the squad…): each can sound, show as a toast and show as a desktop notification of Windows. The "All" row turns a column on or off at once, and a row opens to set its voice, volume and delay. Desktop notifications show only while MAYAK is not in front by default, or always.
- While MAYAK syncs with TarkovTracker, TarkovTracker shows in the sidebar above the map: a tab that stays, whose home button goes back to TarkovTracker's start page. The TarkovTracker pages that notices and the settings open open there too. "TarkovTracker in the sidebar" under Appearance in the browser settings hides it.

### Improved

- After a Prestige (in PvE too since 1.2.0.0), rechecking past logs and the sync when a key is assigned no longer send the tasks done before it to TarkovTracker. The profile stays the same through a Prestige, so MAYAK finds when it was taken in the EFT logs and skips the logs before it. For a Prestige the logs do not hold (taken on another PC), each TarkovTracker profile also gains "Logs from".
- When MAYAK finds a Prestige, the profile under TarkovTracker in the settings asks for the Prestige on TarkovTracker too: its progress can be reset on its website only. Once the reset is seen (checked every few minutes, or told with "Reset done"), the tasks done since the Prestige are sent again, so the reset does not drop them.
- When the level sent to TarkovTracker from the Overall screen goes back, a notice says so: with TarkovTracker's Automatic Level Calculation on, an open TarkovTracker page overwrites it with the level worked out from the XP of your tasks. Turning Automatic Level Calculation off (manual) under Experience & Level in TarkovTracker's settings keeps it; the notice's button opens that setting, and TarkovTracker in the settings explains the same.

### Fixed

- Since 1.2.0.0, a Transit to the next map also played the welcome back from a raid. EFT writes a Transit otherwise in its log, and MAYAK took it for a return to the menu.
- Opening the official wiki translated could stop at a "too many redirects" page. When Google Translate asks for a robot check, the page it returns to once the check is passed redirects to itself without end (Google's side). The tab now opens the translated page again without the part that loops. Only when Google asks for the check again right after does the page open untranslated, and recognized tasks then open untranslated for an hour.
- Tasks finished in the game sometimes did not sync to TarkovTracker. EFT writes a task's notification in two places of its log, a session can have it in only one, and MAYAK read only the other. It now reads both. Tasks that did not sync can be sent with "Recheck past logs" under TarkovTracker in the settings once EFT is closed.

## v0.1.26 (2026-10-05)

### New

- The sites tasks open on now have an order. In the browser settings, under "Opening tasks", drag the official wiki, the Japanese wiki and tarkov.dev into the order you want in "Site order". A task opens on the first site from the top that has its page, and on the first site when none has it. Recognized tasks and tasks opened from the item sidebar or the map all follow the order. A new install starts with the official wiki, the Japanese wiki, then tarkov.dev; if you had chosen a site before, it comes first. Another PC (Client) uses the Host's order, and reordering on a Client changes the Host's setting.

### Fixed

- Tasks of an event under way (All-Inclusive Support and others) were not recognized from a screenshot of the Tasks screen. Tasks tarkov.dev does not have yet are filled in from the official wiki, and that left out the tasks of an event under way as if the event were over.

## v0.1.25 (2026-10-04)

### Fixed

- Your arrow on the map stayed where it was in the last raid after a new raid began. The last position now goes when a raid starts or ends and when the map changes, and the arrow shows again with the raid's first position screenshot. The same on another PC (Client).

## v0.1.24 (2026-10-04)

### Fixed

- A transit between maps no longer plays the back-from-raid alert. EFT logs a transit the way it logs the way back from a raid, so the alert now waits 10 seconds and stays silent once the log shows a transit; it also stays silent when the transit fails and the game drops back to the menu.
- Going back to the menu from an aborted matching no longer plays the game-start alert.

## v0.1.23 (2026-10-04)

### New

- Keys show where they are used: the item panel's "Used at" lists, for keys tarkov.dev has placed, each map in a box of its own with every door or safe's floor and the nearest place name. Pressing a place brings the map there with a yellow ring and makes the key's markers glow (even when the filters hide keys or a search is on); pressing a map's name brings all of its places into view.

## v0.1.22 (2026-10-01)

### Improved

- The 8-digit code the Host issues for another PC shows as 1234-5678, with a hyphen (it was split by a space).

### Fixed

- On a Client's map the Host's arrow was drawn as a squadmate's (a black edge); it is drawn as your own now (a white edge), as on the Host.
- A Client (a Mac above all) left alone for a while could miss a task the Host recognized. The Mac build no longer naps in the background and checks its link as soon as its window is back; the Host keeps what it recognized while no Client was connected for a minute and sends it when one connects again.
- The map's squad column (squad pen, pin, follow the squad) looked washed over with a pale squad colour while its buttons could not be used.
- A Client's settings drifted from its Host's. The language, theme, clock, how tasks open, translating the wiki, the bosses' mode, bookmarks, the squad display name and the task site now take the Host's when they connect, and a change on either PC is made on the other (a task site chosen on a Client becomes the Host's). With two or more Clients, one's change reaches the others. A Client's game mode badge shows the Host's.
- On a Client (a Mac say), the map brought forward by the Host stayed at "Loading" until it was opened again from another tab.
- A Client's map had its own filters and settings, not the Host's: it now takes the Host's when they connect, and a change on either PC is made on the other too.
- A Client's map (another PC, a Mac say) did not switch to the map the Host recognized at matching or in a raid. The Host now tells its Clients the map it plays, whether in a raid and its position, and the Client's map shows that map (and, outside a squad, the Host's position as its own arrow).

## v0.1.21 (2026-09-30)

### New

- Screenshots can be shared with the squad: "Share with the squad" in the screenshot viewer sends one (at the size chosen on the squad page: as taken, 1080p or 720p; 1080p at first), and the same screenshot (told by its file's MD5) never goes twice. A squadmate opens it from the squad section or page and can keep it as a snap note.
- "Screenshots with the position" in the map's settings (the gear; off at first): a screenshot of the position shows in a bubble above your arrow on the map, large when pressed (pressing outside closes it), alone too. In a squad it goes to the squad as well, small (one every 20 seconds at most), and shows above your arrow on their maps; they can ask you for the large picture ("See it large").
- "Show the map when a raid starts" and "Show the map on a position screenshot" in the map's settings (both on at first): turned off, a raid starting or a position screenshot no longer switches from another tab to the map (a map showing still follows).

### Improved

- The squad and link relay has more limits so it stays clear with many players (how often the store is written, how much a link sends a minute, connecting too often from one place); MAYAK hears them from the relay and paces itself under them.
- In a squad, heavy things sent one after another (screenshots, snap notes) are held to so much a minute, so the relay is not flooded; over it, they wait a little before going.
- The sidebar's bookmarks moved to just above the tabs.

## v0.1.20 (2026-09-30)

### Improved

- Each notification sound can be delayed (0 to 15 seconds, Settings → Sounds) so it does not speak over the game; being back from a raid and a task failing wait 3 seconds unless set otherwise.
- While in a squad, the sidebar's squad heading has a leave button (left of the squad page's); pressed twice, it leaves the squad.
- The failed-task notification (Settings → Sounds, "When a task fails") no longer urges restarting failed tasks each time you are back at the menu: it says a task has failed when one fails in the game, without TarkovTracker too, once for tasks failing together. In Tarkov a failed task rarely needs restarting.
- The squad code and link code boxes show the hyphen between the 4th and 5th characters (it still need not be typed).

### Fixed

- A position screenshot no longer plays the "could not send to the map" alert or shows an error when no tarkov.dev map is connected: a Remote ID nothing is connected to cannot be sent to, so it is only logged as a warning (a send that fails once connected still alerts).

## v0.1.19 (2026-09-30)

### New

- A screenshot of the Tasks screen with "Show completed" ticked is read for the tasks its list shows completed (by the rows' colour); those not completed on TarkovTracker gather in a list, per profile. The TarkovTracker indicator in the sidebar counts them, and pressing it lists them to complete one by one or all at once (nothing is applied on its own).
- Screenshots of the character screen's Overall tab are read: level, nickname, experience, raids, kills, survival rate, K/D and time played; the level is raised on TarkovTracker when it is behind (only when it matches the level the experience makes). In a squad, pressing a squadmate's name opens their card with these and their character's picture (which the squad page can keep to yourself); on Windows it opens over the page.
- "Map": a map over the whole page, opened from "Map" at the top of the sidebar. It has tarkov.dev's maps, markers and icons: filters show extracts, transits, spawns, bosses, locks, minefields, containers, loose loot, task objectives and more, and a search finds them. The floor follows your height, or is chosen from a list or with Ctrl + wheel, and how faint the other floors show is a setting. Settings also switch between Abstract (SVG) and Satellite, between PvP, PvE and Season data, set the size of extract and place names and keep sniper spawns always shown. A marker tells more when pressed, and a task objective opens the task's page. A button on the left makes the map as it shows into a snap note.
- Squads: from "Squad" in the sidebar, create a squad and share its code: where each of you takes a screenshot in a raid shows on everyone's map with a name, a colour and the direction faced. The display name starts as your TarkovTracker name. Pressing a squadmate's place in the list shows them, on their map and floor. A recent squad is joined again with one press. A Host and its Clients join and leave squads together. Only display names and positions are shared, encrypted with the squad code; the relay cannot read them.
- "Your pen" on the map: the pencil in the column of buttons on the left draws lines of your own on the map, kept by map and floor for the next time it opens. It has an eraser, six colours, three widths, undo (Ctrl+Z), hiding your lines and removing all of them on the floor shown; while drawing, right-drag or Space + drag moves the map. A snap note of the map carries the lines as its first layer, to erase or add to later.
- A "Squad" section in the sidebar and a squad page. The section shows how many are in the squad and whether it is connected, then each squadmate and their map; pressing their place shows them on the map. Creating, joining, the display name and leaving are on the squad page, with a "Squad colour" for your marker on the map (one of the colours offered or any colour; one someone else has cannot be chosen; Automatic takes a free one).
- A "Squad pen" on the map: the whole squad draws on the same map at once, each in their squad colour, seeing lines as they are drawn and where the others' pens are. Only your own lines can be erased; "Clear all" removes the map's squad lines for everyone (undoable for a few seconds). Pointing at a line names who drew it. Those who join later, or come back after a drop, get the lines too. The lines are also kept on the relay, still encrypted, for a week (each use by the squad extends it), so they are there even after everyone left, for whoever opens the map alone; "Clear all" and its undo reach those who come later as well. A snap note of the map carries the squad's lines in its first layer, with your own. The squad pen's button is in the squad's column, and while it is up its tools are framed in your squad colour, so it is told apart from your own pen.
- Pages and snap notes can be shared with the squad: a page with the squad button in the toolbar, a snap note with "Share with the squad" in its share menu. What was shared, and who drew on which map, is listed in the sidebar's squad section and on the squad page; pressing one opens it (a snap note is kept as a note of your own).
- In the sidebar's squad section, each squadmate shows where they are and the latest page, snap note and pin they shared, opened when pressed; their place brings the map to them at the zoom it has, with rings going out from their arrow, and a "+N" lists the others of a kind. A dot beside a name tells they shared something not seen yet, and pressing the name opens their card (for one synced with TarkovTracker, with the tasks completed and a link to their TarkovTracker profile). The squad page's shares can be narrowed to one sharer as well as to a kind. A tab can be shared from its row in the sidebar too (left of its pin); a shared page already open in a tab brings that tab forward. When a squadmate draws on the map a dot shows beside "Map" in the sidebar,.
- The map's settings (the gear) choose the arrows' shape (arrowhead, tarkov.dev's arrow, dot and cone) and give them, yours and the squad's, an effect (outline, glow, pulse or beacon) in a colour chosen or each one's squad colour. A squadmate's arrow no longer fades with an old position, and stays until they start a new raid or leave one.
- The map pens' lines grow and shrink with the map's zoom, so writing drawn close up keeps its shape for a squadmate seeing it from afar; the squad pen's position goes twenty times a second and glides, and the squad pen is put down after three minutes unused.
- "Follow the squad" in the map's squad column (on from the start; nothing moves while you play alone): when someone's position updates on your raid's map, the map takes in the whole squad there.
- "Look here" pins: the pin button on the map drops a pin where you press, as on Google Maps, dropping in, and it shows on everyone's map in your squad colour. While the button is on, each press puts it again and a right drag moves the map; later you can still drag your pin, and its × takes it out (one each, gone after ten minutes). The squad pen and the pin sit in their own column under the map's buttons, framed in your squad colour. Pressing it in the squad list brings up its map and floor, closing in at the zoom it was dropped at.
- The squad code is typed into boxes of one character, as the pairing code is (no hyphen to type); the eighth joins. Out of a squad, the squad page tells how to make one and how to join a squadmate's.
- Ad blocking works on a Mac too: the same filter lists as on Windows (EasyList, EasyPrivacy, AdGuard Japanese) become a WebKit content blocker. After the first start or a list update it takes a moment to be ready. As on Windows, the empty space a hidden ad leaves is closed up.

### Improved

- Snap notes open to be looked at, and "Edit" lets you draw on them ("Done" goes back); one just captured or made from a blank sheet opens ready to draw. A snap note from a squadmate remembers who sent it and shows it.
- The setting that gave your position marker on tarkov.dev's map an effect in the built-in browser (Settings → tarkov.dev → Player position marker) is gone: tarkov.dev's pages are left as they are, and the effect is set in MAYAK's own map settings.
- Bookmark icons show even for sites never opened: those pinned to the sidebar are fetched at start and when pinned, and all of them when the bookmarks page opens, one at a time in the background (a site that refuses this, such as Fandom, gets its icon once opened).
- On a Mac, the menu bar icon is the logo alone (the hexagon and M) in black or white, to suit a light or dark menu bar.
- The connection to other PCs (from the Host to Clients) goes through a relay (mayak-relay.ich.sh): no network keeps it from connecting any more, and paired PCs connect again on their own at every start (and after a drop). What they say is encrypted with a key only the paired PCs have; the relay cannot read it. The 8-digit code from the Host, entered on the other PC, is all it takes (no response code any more), and a Host takes any number of PCs. Each Client chooses what it shows: tasks, maps and positions, items. The STUN setting is gone.
- The TARKOV.DEV and TarkovTracker fixed tabs are gone; they are bookmarks pinned to the sidebar. Map detections show on "Map".
- The connection status and the game mode (PvP, PvE, Season) at the top of the sidebar open the setting they come from when pressed. The game mode is bold, without a frame.
- Settings → Connection names the modes "Host: Escape from Tarkov runs on this PC" and "Client: … on another PC (the Host)", each with a line of help. "Off" is gone (it was a Client without a pairing); to stop detecting, turn monitoring off. Outside Windows, where only Client fits, no choice shows.
- With a TarkovTracker key registered, its state shows left of the monitoring switch: green synced, amber connecting or waiting for a profile, red when the profile played has no key or it fails. It opens the TarkovTracker settings.
- The item panel's scroll bar shows only while it scrolls or the pointer moves over it.
- The update channel under Settings → About MAYAK can be chosen on a Mac and on Linux too (it was on the Windows Host only).
- On a Mac the window's close, minimize and zoom buttons are macOS's own red, yellow and green ones at the top left, instead of Windows-style ones at the top right: the green one goes full screen, zooms with Option held and shows the tiling menu when held down. The sidebar and the toolbar move right for them (not in full screen).
- The sidebar's collapse button moved from the top of the sidebar to the buttons at its bottom, with an arrow that shows which way it opens or closes, so it is easy to tell from the placement button. In the collapsed sidebar the "Tabs" heading is an icon too (it opens the list of tabs).
- Squad codes, the codes pairing other PCs and tarkov.dev Remote IDs show as dots, and the eye button beside one shows it; what is typed into those fields shows as dots too. A stream or a screenshot no longer gives a code away.
- Snap notes have a note with a folded corner for their icon instead of a brush, and screenshots a camera instead of a picture, to tell them from the map's pens (a pencil).
- Pairing with another PC: a Client types the code into eight boxes of one digit (the typing moves on, a paste fills them, the eighth digit connects), and once linked each PC shows the other's computer name under Settings → Connection.

### Fixed

- An item's picture that does not load (the dogtag case's, say) is asked for once more, and shows as a "?" when it still fails, instead of a broken image.
- A bookmark opened, or a task recognized, whose page a tab shows already (translated too) brings that tab forward instead of opening another.
- Closing settings did nothing when there was no tab to go back to; a new tab opens instead.
- On a Mac, a sign-in popup ("Sign in with Google" on Fandom) did not open; it opens in a small window, as on Windows.
- The official wiki's (Fandom's) right rail is hidden whether the page is translated or not and signed in or not (Windows and Mac): its "New to Fandom?" card left the article narrow.
- A Client (another PC) could show the bosses, the Goons and the map's markers of another game mode than its Host's; it follows the Host's game mode.
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
