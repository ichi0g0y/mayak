<p align="center"><img src="docs/assets/mayak-mark.png" width="96" alt="MAYAK"></p>

<h1 align="center">MAYAK</h1>

<p align="center">
  An Escape from Tarkov companion that never touches the game.<br>
  It reads the screenshots and logs the game saves, and keeps the tarkov.dev map, your tasks and item info in sync in a browser.
</p>

<p align="center">
  <a href="https://mayak.ich.sh"><b>mayak.ich.sh</b></a> ·
  <a href="https://github.com/ichi0g0y/mayak/releases/latest">Download</a> ·
  <a href="https://buymeacoffee.com/ichi0g0y">Buy Me a Coffee</a>
</p>

## Download

Get the latest release from [mayak.ich.sh](https://mayak.ich.sh) or the [releases page](https://github.com/ichi0g0y/mayak/releases/latest).

| Platform | File | Notes |
| --- | --- | --- |
| Windows 11 (64-bit; Windows 10 should work but is untested) | `Mayak-Setup-x.y.z-windows-amd64.exe` | Per-user installer, no administrator rights. SmartScreen warns once because the build is unsigned: **More info → Run anyway**. Updates are automatic afterwards. |
| Windows, portable | `Mayak-x.y.z-windows-amd64.zip` | Unzip and run `Mayak.exe`. |
| macOS (Apple Silicon / Intel) | `Mayak-x.y.z-darwin-arm64.dmg` / `Mayak-x.y.z-darwin-amd64.dmg` | Preview. Open the disk image and drag Mayak.app to Applications. Unsigned: right-click → Open the first time (or System Settings → Privacy & Security → Open Anyway). Receive-only client for maps and tasks. |
| Linux (x86-64) | `Mayak-x.y.z-linux-amd64.tar.gz` | Preview. Receive-only client; needs WebKitGTK. |

`SHA256SUMS.txt` lists the checksum of every file. MAYAK checks GitHub Releases for a newer version at start and every 6 hours, downloads it in the background, verifies the checksum and installs it when it quits (Settings → Startup → Automatic updates).

## What it does

- **Position on the map.** Press EFT's screenshot key in a raid: the coordinates and heading in the file name move your marker on the tarkov.dev map. Map and floor are detected from the logs and the coordinates.
- **Tasks screen.** Screenshot the task list and local OCR reads the selected task, then opens it on tarkov.dev or the wiki. English and Japanese game text.
- **Item inspection.** Screenshot an item window and the item panel shows flea and trader prices, price history, and the tasks and hideout stations that need it.
- **TarkovTracker sync.** Task started, failed and completed events from the notification logs go to TarkovTracker, per PvP, Season and PvE profile. Past logs can be synced in one go.
- **Raid alerts.** Sounds for match found, raid start and the run-through timer, with custom WAV / MP3 files.
- **Built-in browser.** tarkov.dev, TarkovTracker and the wiki in tabs, with ad blocking.
- Japanese and English UI, tray, autostart, automatic updates.

## Safety boundary

MAYAK only reads files created by EFT, local application settings and public web APIs. It does not read process memory, inject DLLs, hook the game, sniff network traffic or automate game input. Screenshots are not uploaded; OCR and image analysis run locally. TarkovTracker API keys are stored encrypted with Windows DPAPI.

Whether a companion tool is acceptable to you is your own call: read the game's terms and use MAYAK at your own risk. MAYAK is not affiliated with Battlestate Games, tarkov.dev or TarkovTracker.

## Setup

1. Run the installer (or unzip the portable build) and start MAYAK. On the first start it walks you through the steps below; the guide can be reopened under Settings → Appearance.
2. Check the EFT Screenshots and Logs folders under Settings → Folders; they are detected automatically when possible.
3. In EFT, bind Settings → Controls → Screenshot to a key you can reach in a fight (the default is PrintScreen). Only EFT's own screenshots carry coordinates.
4. Press that key in a raid. The map tab in MAYAK's built-in browser follows your position with no further setup; on one PC with a second monitor that is all you need.
5. Optional, for another PC or your usual browser: open the tarkov.dev map there, click the connect button at the bottom left, and enter the Remote ID it shows under Settings → tarkov.dev (browsers on the same PC are detected automatically).
6. Optional: TarkovTracker records your task progress. Create an API token (GP and WP) on its [settings page](https://tarkovtracker.org/settings#api), paste it under Settings → TarkovTracker and assign it to the profile MAYAK finds in the logs (one token per PvP / Season / PvE).

The full guide is on [mayak.ich.sh](https://mayak.ich.sh).

## Development

Requirements: Windows 11, [mise](https://mise.jdx.dev/) (installs the pinned Go, bun, Task, Node and wrangler from `mise.toml`), 7-Zip for the bundled Tesseract, WebView2 Runtime.

```powershell
git clone https://github.com/ichi0g0y/mayak.git
cd mayak
mise install
task dev          # the app with hot reload
task build        # build/bin/Mayak.exe
task installer    # build/dist/Mayak-Setup-<version>-windows-amd64.exe
task dev:web      # the landing page at http://127.0.0.1:5173
go test ./...
```

- `internal/` holds the app: recognition, log parsing, catalog, tracker sync, the updater (`internal/update`) and the Wails bindings (`internal/app`).
- `frontend/` is the app's React UI; `site/` is the landing page (React, Tailwind, shadcn/ui, jotai), published to Cloudflare with `task site:deploy`.
- `docs/` is the specification in Japanese, kept as Markdown for reference. Start with [docs/index.md](docs/index.md); [docs/development.md](docs/development.md) covers the repository layout, tasks, tests and the release flow.

Release: tag a commit `vX.Y.Z` and push the tag. The Release workflow builds every platform, publishes the installer and the archives with `SHA256SUMS.txt`, and running copies of MAYAK update themselves from there.

## Support

MAYAK is free and stays free. If it helps your raids, [buy me a coffee](https://buymeacoffee.com/ichi0g0y) or [sponsor on GitHub](https://github.com/sponsors/ichi0g0y). Bugs and ideas go to the [issues](https://github.com/ichi0g0y/mayak/issues).

## Credits

- Game data (items, maps, traders, tasks, hideout, boss and Goons sightings) comes from the free, community-run [tarkov.dev API](https://tarkov.dev/api/). Map and position sync uses tarkov.dev Remote Control. Item images and names are Battlestate Games' property, shown as tarkov.dev shows them.
- Task progress sync uses the [TarkovTracker API](https://tarkovtracker.org/) with API tokens you create there.
- The task list is completed from the [Escape from Tarkov Wiki](https://escapefromtarkov.fandom.com/) (CC BY-NC-SA).
- Ad blocking in the built-in browser uses [EasyList and EasyPrivacy](https://easylist.to/) (GPLv3 / CC BY-SA 3.0) and the [AdGuard Japanese filter](https://github.com/AdguardTeam/AdguardFilters) (GPLv3).
- OCR uses [Tesseract](https://github.com/tesseract-ocr/tesseract) (Apache-2.0) from the [UB Mannheim](https://github.com/UB-Mannheim/tesseract) build with models derived from `tessdata_best`, or Windows OCR.
- Color themes are based on the [Catppuccin](https://catppuccin.com/), [Nord](https://www.nordtheme.com/), [Dracula](https://draculatheme.com/), Gruvbox, Tokyo Night and [Solarized](https://ethanschoonover.com/solarized/) palettes.

Escape from Tarkov and related names are trademarks of Battlestate Games.

## License

MAYAK is free software under the [GNU General Public License v3.0](LICENSE). `task build` writes `build/bin/THIRD_PARTY_NOTICES.txt` with the licenses of everything in the binary and beside it; it ships with every release.
