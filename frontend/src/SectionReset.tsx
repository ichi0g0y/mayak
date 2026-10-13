import { RotateCcw } from 'lucide-react'
import { Button } from './components/ui/button'
import { MessageKey } from './i18n'
import { HostSection, Settings } from './settings-model'

// "Back to defaults" for a settings page, in two steps: the first press shows
// the page with its defaults and lists what changes (the values it goes back
// to coloured), the second saves them; "cancel" or any edit gives up. The
// defaults are the Go side's (DefaultSettings). Folders and Remote's targets
// stay: they are found or entered, and lost they are missed.

type Values = Record<string, unknown>

// The sound and notification settings: on the Sounds page, besides the few
// it names (soundRows), one row for all of them.
const soundSetting = (key: string) =>
  key.startsWith('sound') ||
  /Sound(Enabled|Path)?$/.test(key) ||
  ['toastsOff', 'desktopOn', 'desktopAlways', 'runThroughSeconds'].includes(key)

// The settings each page puts back. The capture's times are not on the
// Screenshots page but go back with it.
const pageKeys: Partial<Record<HostSection, (key: string) => boolean>> = {
  folders: (k) => ['screenshotCleanup', 'screenshotRetainCount', 'screenshotRetainHours'].includes(k),
  screenshots: (k) => k.startsWith('capture'),
  recognition: (k) => ['gameMode', 'gameLanguage', 'ocrEngine', 'tesseractPath'].includes(k),
  remote: (k) => ['map', 'openMapOnRaidStart', 'navigateMapOnPositionScreenshot'].includes(k),
  tracker: (k) => k === 'tarkovTrackerEnabled',
  sounds: soundSetting,
  startup: (k) =>
    [
      'launchAtStartup',
      'startMinimized',
      'autoStartMonitoring',
      'minimizeToTray',
      'closeToTray',
      'keepPriority',
      'autoUpdate',
    ].includes(k),
  gameInput: (k) =>
    [
      'gameWheel',
      'gameScrollCombo',
      'gameTabCombo',
      'gameCloseTab',
      'gamePinTab',
      'gameRemote',
      'gameRemoteSmooth',
    ].includes(k),
  debug: (k) => ['debug', 'saveRecognitionDebug'].includes(k),
}

export const resettable = (section: HostSection) => section in pageKeys

const same = (a: unknown, b: unknown) => JSON.stringify(a ?? null) === JSON.stringify(b ?? null)

// resetValues are the page's settings that differ from their defaults, at
// their defaults.
export function resetValues(section: HostSection, current: Settings, defaults: Settings): Partial<Settings> {
  const keep = pageKeys[section]
  const now = current as unknown as Values
  const out: Values = {}
  if (!keep) return {}
  for (const [key, value] of Object.entries(defaults as unknown as Values)) {
    if (keep(key) && !same(now[key], value)) out[key] = value
  }
  return out as Partial<Settings>
}

// The rows' names; the settings without one of their own go in a group row.
const labels: Record<string, MessageKey> = {
  screenshotCleanup: 'screenshotCleanup',
  screenshotRetainCount: 'retainCount',
  screenshotRetainHours: 'retainHours',
  captureKey: 'captureKey',
  captureAuto: 'captureAuto',
  captureAutoOff: 'captureKinds',
  captureTasksTrigger: 'captureTasksTrigger',
  captureItemTrigger: 'captureItemTrigger',
  gameMode: 'gameMode',
  gameLanguage: 'gameLanguage',
  ocrEngine: 'ocrEngine',
  tesseractPath: 'tesseractPath',
  map: 'mapFallback',
  openMapOnRaidStart: 'openMapOnRaidStart',
  navigateMapOnPositionScreenshot: 'navigateMapOnPositionScreenshot',
  tarkovTrackerEnabled: 'trackerEnable',
  launchAtStartup: 'launchAtStartup',
  startMinimized: 'startMinimized',
  autoStartMonitoring: 'autoStartMonitoring',
  minimizeToTray: 'minimizeToTray',
  closeToTray: 'closeToTray',
  keepPriority: 'keepPriority',
  autoUpdate: 'autoUpdate',
  gameWheel: 'gameWheel',
  gameScrollCombo: 'gameScrollCombo',
  gameTabCombo: 'gameTabCombo',
  gameCloseTab: 'gameCloseTab',
  gamePinTab: 'gamePinTab',
  gameRemote: 'gameRemote',
  gameRemoteSmooth: 'gameRemoteSmooth',
  debug: 'debugMode',
  saveRecognitionDebug: 'saveRecognitionDebug',
  soundsEnabled: 'soundsEnabled',
  soundVolume: 'volume',
  desktopAlways: 'notifyDesktopWhen',
}
const groupLabel = (key: string): MessageKey => (key.startsWith('capture') ? 'resetCaptureTimes' : 'notifyEventsTitle')

// The wheel's combos (capture.WheelCombos), as the In-game controls page names them.
const wheelChoices: Record<string, MessageKey> = {
  '': 'gameScroll_off',
  right: 'gameScroll_right',
  x1: 'gameScroll_x1',
  x2: 'gameScroll_x2',
  shift: 'gameScroll_shift',
  ctrl: 'gameScroll_ctrl',
  alt: 'gameScroll_alt',
}

// The buttons that close and pin a tab (capture.TabButtons).
const buttonChoices: Record<string, MessageKey> = {
  '': 'gameScroll_off',
  middle: 'gameButton_middle',
  'shift+middle': 'gameButton_shiftMiddle',
  'ctrl+middle': 'gameButton_ctrlMiddle',
  'alt+middle': 'gameButton_altMiddle',
  x1: 'gameScroll_x1',
  x2: 'gameScroll_x2',
}

// Values shown as their names on the page.
const choices: Record<string, Record<string, MessageKey>> = {
  gameMode: { auto: 'gameModeAuto', pve: 'gameModePVE', regular: 'gameModePVP', 'pvp-season': 'gameModeSeason' },
  ocrEngine: { tesseract: 'tesseractOCR', windows: 'windowsOCR' },
  gameLanguage: { auto: 'gameLanguageAuto' },
  captureTasksTrigger: { right: 'captureTasksRight', shift: 'captureTasksShift', click: 'captureTasksClick' },
  captureItemTrigger: { click: 'captureItemClick', right: 'captureItemRight', shift: 'captureItemShift' },
  gameScrollCombo: wheelChoices,
  gameTabCombo: wheelChoices,
  gameCloseTab: buttonChoices,
  gamePinTab: buttonChoices,
}

type Row = { label: string; from: string; to: string }

function rows(values: Values, current: Values, t: (key: MessageKey) => string): Row[] {
  const show = (key: string, value: unknown): string => {
    if (typeof value === 'boolean') return t(value ? 'resetOn' : 'resetOff')
    if (typeof value === 'number') return String(value)
    if (typeof value === 'string')
      return choices[key]?.[value] ? t(choices[key][value]) : value === '' ? t('resetEmpty') : value
    return ''
  }
  const out: Row[] = []
  const groups = new Set<MessageKey>()
  for (const key of Object.keys(values)) {
    const label = labels[key]
    const shown = label && show(key, values[key]) !== ''
    if (!shown) {
      groups.add(labels[key] ?? groupLabel(key))
      continue
    }
    out.push({ label: t(label), from: show(key, current[key]), to: show(key, values[key]) })
  }
  for (const label of groups) out.push({ label: t(label), from: t('resetChanged'), to: t('resetDefault') })
  return out
}

type Props = {
  // The values shown in place of the page's own, or null.
  preview: Partial<Settings> | null
  current: Settings
  t: (key: MessageKey) => string
  onCommit: () => void
  onCancel: () => void
}

// The list under the page's title while its defaults are shown.
export function ResetPanel({ preview, current, t, onCommit, onCancel }: Props) {
  if (!preview) return null
  const list = rows(preview as Values, current as unknown as Values, t)
  return (
    <div className="reset-panel" role="region" aria-label={t('resetTitle')}>
      {list.length ? (
        <>
          <p className="reset-panel-title">{t('resetTitle')}</p>
          <ul>
            {list.map((row) => (
              <li key={row.label}>
                <span className="reset-name">{row.label}</span>
                <span className="reset-from">{row.from}</span>
                <span className="reset-arrow">→</span>
                <span className="reset-to">{row.to}</span>
              </li>
            ))}
          </ul>
          <div className="reset-actions">
            <Button type="button" onClick={onCommit}>
              <RotateCcw />
              {t('resetCommit')}
            </Button>
            <Button type="button" variant="secondary" onClick={onCancel}>
              {t('resetCancel')}
            </Button>
          </div>
        </>
      ) : (
        <div className="reset-actions">
          <p className="reset-panel-title">{t('resetNothing')}</p>
          <Button type="button" variant="secondary" onClick={onCancel}>
            {t('resetClose')}
          </Button>
        </div>
      )}
    </div>
  )
}
