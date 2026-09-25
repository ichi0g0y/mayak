<p align="center"><img src="docs/assets/mayak-mark.png" width="96" alt="MAYAK"></p>

# MAYAK

The full specification (in Japanese) is in [docs/](docs/index.md); build it as
a site with [mdBook](https://rust-lang.github.io/mdBook/): `mdbook serve docs`.

The [TarkovMonitor feature-parity checklist](docs/tarkovmonitor-parity.md) tracks
the supported behavior and the remaining safe, log-based integrations.

Game data (items, maps, traders, tasks, hideout stations, level data and base Scav
cooldown) is fetched from tarkov.dev per progression mode. A shared catalog warms
when EFT's mode is detected, checks for changes every 5 minutes (downloading only
resources that changed), and retains validated
last-known-good data under the user's MAYAK configuration directory for offline
use. The Status tab shows counts, mode and last retrieval time, and provides a
manual refresh. Catalog retrieval does not synchronize hideout progress or player
levels to TarkovTracker.

MAYAK is a local Windows companion for Escape from Tarkov. It watches screenshots created by the game and uses public web interfaces to keep a browser map in sync without interacting with the game process.

The current proof of concept supports three workflows:

- Position screenshots: parse coordinates and orientation embedded in the filename, detect the active map from EFT logs, and update the player marker on tarkov.dev.
- Tasks screenshots: detect supported Tasks screen layouts, OCR the selected quest name locally, match it against current quest data, and navigate tarkov.dev to the closest supported task or map view.
- Item inspection screenshots: locate the movable inspection window, OCR its title locally, and match it against the current tarkov.dev item catalog without opening a browser over the game.

## Safety boundary

MAYAK only reads files created by EFT, local application settings, and public web APIs. It does not read process memory, inject DLLs, hook the game, sniff network traffic, or automate game input.

Screenshots are not uploaded. OCR and image analysis run locally. Optional recognition archives are stored under `Screenshots/Mayak-Debug` with the source image, crop, and a JSON metadata file. Optional retention cleanup is disabled by default and only removes image files directly inside the configured Screenshots directory; it never removes the debug archive.

## Features

- Passive screenshot monitoring with stable-file detection and deduplication
- EFT coordinate, quaternion, direction, and floor-aware position updates
- Raid-state and map detection from ordinary EFT logs
- Multi-account TarkovTracker task synchronization for Normal PvP, Seasonal PvP, and PvE profiles
- Existing EFT log scan and explicit API-key assignment per account, profile, and mode
- Automatic task started, failed, and completed updates from EFT notification logs
- Per-mode TarkovTracker API tokens protected with Windows DPAPI
- tarkov.dev Remote Control over WebSocket
- Multiple Remote IDs with independent map/position and task-display routing
- Optional automatic tarkov.dev map navigation when a raid starts
- Automatic tarkov.dev Remote ID discovery for local Chrome, Edge, and Brave profiles
- Multiple 2560×1440 English Tasks screen layouts
- Windows OCR with optional Tesseract support
- Fuzzy quest-name matching against current tarkov.dev data
- Position-independent item inspection detection and fuzzy item-name matching
- Local success and error sounds with volume controls
- Optional match-found, raid-start, and run-through timer sound alerts from EFT logs
- Per-alert local WAV or MP3 assignment with built-in sound fallback
- In-app structured log viewer with level filtering, search, and JSON Lines persistence
- Japanese and English UI
- Persistent settings, window size, and window position
- Optional Windows startup, manual or automatic monitoring, minimized launch, and system-tray behavior
- React, TypeScript, Radix UI, and shadcn-style components

## Requirements

- Windows 11
- [mise](https://mise.jdx.dev/), which installs the Go, bun, and Task versions pinned in `mise.toml`
- Wails CLI v3.0.0-beta.24
- WebView2 Runtime
- Windows English OCR language pack when using Windows OCR

The Windows Host includes a native tab workspace; macOS/Linux are receive-only
Clients. Their native implementations still require platform build/runtime checks.
See [Wails v3 migration notes](docs/wails-v3-migration.md).

## Development

Install Wails if needed:

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.24
```

Clone and run:

```powershell
git clone https://github.com/ichi0g0y/mayak.git
cd mayak
wails3 dev
```

Build:

```powershell
wails3 task build
```

Run tests:

```powershell
task frontend:build
go test ./...
```

## Usage

1. Open tarkov.dev in a browser and enable Remote Control.
2. Enter the displayed Remote ID in MAYAK, or use automatic detection when the browser is on the same PC.
3. Select the EFT Screenshots and Logs directories, or use automatic folder detection.
4. Save the settings and leave MAYAK running.
5. Use PrintScreen in EFT. MAYAK processes the resulting file without sending input to the game.

### TarkovTracker synchronization

Create API tokens on the [TarkovTracker settings page](https://tarkovtracker.org/settings#api). Each token needs Get Progress (`GP`) and Write Progress (`WP`) permissions. MAYAK scans existing EFT logs for account/profile/mode identities. Assign each Normal PvP (`PVP_`), Seasonal PvP (`SZN_`), or PvE (`PVE_`) key to the matching detected profile in Settings. Unassigned keys are never used for synchronization.

MAYAK reads `Session mode`, selected account/profile, and `ChatMessageReceived` task notifications from EFT's application/output logs. It sends task started, failed, and completed state changes only when the complete account, profile, and mode binding matches the active EFT session. Multiple EFT accounts and multiple profiles in the same mode remain isolated. Saved tokens are kept outside `settings.json` and encrypted for the current Windows user with DPAPI.

Position metadata is ignored unless the EFT logs confirm an active raid. Tasks screen detection takes priority so menu screenshots containing stale coordinate metadata are not treated as live positions.

The run-through alert uses one user-configurable duration for all maps (7:10 by default). Its timer starts only for PMC and PvE raids; short Scav start timing and reconnects are excluded.

## Project layout

```text
MAYAK/
├─ main.go                  embeds the frontend and tray icon, starts internal/app
├─ internal/app/            Wails bindings and application lifecycle
├─ frontend/src/            React and TypeScript UI
├─ internal/config/         persistent local settings
├─ internal/eftdetect/      EFT directory discovery
├─ internal/logdetect/      raid-state and map detection
├─ internal/itemdetect/     movable item inspection window detection
├─ internal/itemapi/        item catalog client
├─ internal/itemmatch/      OCR-tolerant item matching
├─ internal/ocr/            local OCR adapters
├─ internal/position/       screenshot filename and direction parser
├─ internal/questapi/       quest catalog client
├─ internal/questmatch/     normalized fuzzy matching
├─ internal/remote/         tarkov.dev WebSocket client and payloads
├─ internal/remoteid/       local browser Remote ID discovery
├─ internal/sound/          local notification sounds
├─ internal/taskdetect/     Tasks screen layouts and crops
├─ internal/tracker/        TarkovTracker API client
├─ internal/trackerlog/     EFT profile and task-event log parser
├─ internal/trackerstore/   per-mode DPAPI-protected token storage
└─ internal/watcher/        screenshot watcher and stable-file gate
```

## Compatibility

MAYAK is an independent community project designed to interoperate with public EFT file formats and public tarkov.dev services. Escape from Tarkov and related names are trademarks of Battlestate Games. MAYAK is not affiliated with or endorsed by Battlestate Games, tarkov.dev or TarkovTracker.

MAYAK only reads files the game writes; it never touches the game process. Whether a companion tool is acceptable to you is nevertheless your own call: read the game's terms and use MAYAK at your own risk.

## Data sources and credits

- Game data (items, maps, traders, tasks, hideout, boss and Goons sightings) comes from the free, community-run [tarkov.dev API](https://tarkov.dev/api/). Map and position sync uses tarkov.dev Remote Control. Item images and names are Battlestate Games' property, shown as tarkov.dev shows them.
- Task progress sync uses the [TarkovTracker API](https://tarkovtracker.org/) with API tokens you create there.
- The task list is completed from the [Escape from Tarkov Wiki](https://escapefromtarkov.fandom.com/) category index. Wiki content is licensed [CC BY-NC-SA](https://www.fandom.com/licensing).
- The built-in browser's ad blocking uses [EasyList and EasyPrivacy](https://easylist.to/) (GPLv3 / CC BY-SA 3.0, by the EasyList authors) and the [AdGuard Japanese filter](https://github.com/AdguardTeam/AdguardFilters) (GPLv3), downloaded on first use.
- OCR uses [Tesseract](https://github.com/tesseract-ocr/tesseract) (Apache-2.0) from the [UB Mannheim](https://github.com/UB-Mannheim/tesseract) build, with models derived from `tessdata_best`, or Windows OCR.
- Color themes are based on the [Catppuccin](https://catppuccin.com/), [Nord](https://www.nordtheme.com/), [Dracula](https://draculatheme.com/), Gruvbox, Tokyo Night and [Solarized](https://ethanschoonover.com/solarized/) palettes.

## License

MAYAK is free software under the [GNU General Public License v3.0](LICENSE).

`task build` writes `build/bin/THIRD_PARTY_NOTICES.txt`, the licenses of the Go modules and frontend packages in the binary, the bundled OCR data, and the filter lists above; ship it next to `Mayak.exe`.
