# TarkovMonitor feature parity

This file is MAYAK's implementation checklist for the user-facing capabilities
available in TarkovMonitor. The implementation is independent: the reference
project is used to understand observable EFT log events and service behavior,
not as a source of copied code or UI.

## Safety boundary

Every feature below must use only EFT-created screenshots and log files, public
web APIs, and documented browser remote-control channels. Process memory,
injection, hooks, packet capture, and automated input to EFT are out of scope.

## Current coverage

| Capability | MAYAK status | Acceptance condition |
| --- | --- | --- |
| Log-first main screen | Complete | Logs open on launch, newest event is first, filters and search work |
| EFT profile and game-mode detection | Complete | Account, profile, PvP, Season PvP, and PvE are kept separate |
| Public game-data catalog | Complete | Mode-specific items, maps, traders, tasks, and hideout stations are checked for changes every 5 minutes; validated disk cache survives outages |
| Live TarkovTracker task sync | Complete | Start, fail, and completion events update only the exact bound profile |
| Past-log TarkovTracker sync | Complete | User chooses a profile and game-version breakpoint before one bulk update |
| Match-found alert | Complete | Log event, optional built-in/custom sound, volume control |
| Raid-start alert | Complete | Log event, optional built-in/custom sound, volume control |
| Run-through alert and timer | Complete | Countdown is based on the raid-start event and is cancelled on exit |
| Quest-item reminder | Complete | Optional reminder plays when a raid starts |
| Failed-task restart reminder | Complete | Optional reminder uses failed state already read from TarkovTracker |
| tarkov.dev map navigation | Complete | Map follows EFT logs and can open automatically on raid start |
| tarkov.dev player position | Complete | Screenshot coordinates, direction, floor, and map are sent only in raid |
| Task-screen recognition | Complete for current presets | Character and trader layouts use cropped local OCR and fuzzy matching |
| Item inspection recognition | Initial coverage | Movable item detail windows use content anchors and local OCR |
| Tray, startup, and window state | Complete | Single instance, tray restore, startup, bounds, and settings persist |
| Japanese/English app UI | Complete | All current settings and status labels are localized |

## Remaining parity work

| Capability | Planned implementation | Done when |
| --- | --- | --- |
| Scav cooldown | Read the profile's `SavageLockTime`; reconcile with public game-data bonuses | Timer survives restart and the optional sound fires once |
| Air-filter on/off | Read TarkovTracker hideout progress and public station/bonus data | Entering and leaving raids produces the correct optional reminder |
| Flea sale and expiry history | Parse system chat notifications into a local persistent event store | Item, count, price, fee, expiry, profile, and timestamp are queryable |
| Local statistics | Aggregate raid counts by map/mode and flea totals from the event store | Stats can be filtered by profile and wipe breakpoint |
| Group status | Parse invite, settings, member-ready, leave, and disband events | Current party members and ready state are visible without retaining loadout JSON |
| Queue-time contribution | Post queue duration, map, raid type, and game mode to the current public endpoint | Explicit opt-in, documented payload, retry/rate-limit handling |
| Goon sighting report | Offer the action only after a valid raid on a supported map | Explicit confirmation explains account ID/IP handling before submission |
| Media pause/resume | Use Windows media controls without inspecting EFT | Explicit opt-in; only sessions paused by MAYAK are resumed |
| Delete position screenshots | Delete only successfully processed raid-position screenshots | Off by default, exact-path validation, retention option, and recoverability warning |
| Always on top | Wails window option | Setting applies live and persists |
| Update check | GitHub Releases API | Manual and optional startup checks; no silent install |
| Control-binding validation | Read EFT control settings written to logs/files | Warn when the position-screenshot binding cannot produce metadata |

## Deliberate differences

- MAYAK keeps its own small event store instead of adopting another app's
  database schema.
- Historical Tracker writes are never automatic. The user must choose the exact
  EFT profile and wipe/build breakpoint.
- Group loadout payloads can contain a large inventory snapshot. MAYAK will
  parse only the minimum fields needed for the party UI and will not persist raw
  loadouts by default.
- Goon and queue submissions are opt-in because they transmit data to a third
  party. Remote map control and Tracker writes remain independently configurable.
