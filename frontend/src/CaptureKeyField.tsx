import { useEffect, useState } from 'react'
import { Keyboard, MapPinOff, X } from 'lucide-react'
import { Button } from './components/ui/button'
import { Label } from './components/ui/label'
import { Badge } from './components/ui/badge'
import { Switch } from './components/ui/switch'
import { MessageKey } from './i18n'

type Props = {
  value: string
  // Capturing after a click in the game, like RatScanner's automatic scan.
  auto: boolean
  onAuto: (auto: boolean) => void
  // The screens a click is not captured for (app_capture.go captureKinds).
  autoOff: string[]
  onAutoOff: (off: string[]) => void
  // How the tasks screen and an item's window are read (app_capture.go
  // captureTriggers); onTuning sets one or both.
  tuning: CaptureTuning
  onTuning: (tuning: Partial<CaptureTuning>) => void
  // EFT's own screenshot keys (Control.ini), written the same way.
  eftKeys: string[]
  hasFolder: boolean
  t: (key: MessageKey) => string
  onChange: (key: string) => void
}

export type CaptureTuning = {
  captureTasksTrigger: string
  captureItemTrigger: string
}

// The clicks the tasks screen and an item's window can be taken after
// (app_capture.go captureTriggers): which setting holds each one's, and the
// choices in the words of what is done on that screen, the default first.
type CaptureTrigger = 'right' | 'shift' | 'click'
const triggerSettings: {
  kind: string
  key: 'captureTasksTrigger' | 'captureItemTrigger'
  label: MessageKey
  choices: [CaptureTrigger, MessageKey][]
}[] = [
  {
    kind: 'tasks',
    key: 'captureTasksTrigger',
    label: 'captureTasksTrigger',
    choices: [
      ['right', 'captureTasksRight'],
      ['shift', 'captureTasksShift'],
      ['click', 'captureTasksClick'],
    ],
  },
  {
    kind: 'item',
    key: 'captureItemTrigger',
    label: 'captureItemTrigger',
    choices: [
      ['click', 'captureItemClick'],
      ['right', 'captureItemRight'],
      ['shift', 'captureItemShift'],
    ],
  },
]

// The screens a click can be captured for, in the order they are read
// (app_capture.go captureKinds), and their names.
const captureKinds = ['tasks', 'item', 'profile'] as const
const captureKindLabels: Record<(typeof captureKinds)[number], MessageKey> = {
  tasks: 'captureKindTasks',
  item: 'captureKindItem',
  profile: 'captureKindProfile',
}

// The symbol keys by the browser's key codes (where a key is), as a US
// keyboard types them without Shift: what layout() does not answer.
const usSymbols: Record<string, string> = {
  Semicolon: ';',
  Quote: "'",
  Comma: ',',
  Period: '.',
  Slash: '/',
  Minus: '-',
  Equal: '=',
  BracketLeft: '[',
  BracketRight: ']',
  Backslash: '\\',
  Backquote: '`',
  IntlRo: '\\',
  IntlYen: '¥',
}

// layout is the keyboard's layout as the browser has it: what each symbol
// key types without Shift (";" and ":" are keys of their own on a Japanese
// keyboard, one key on a US one). Asked once; missing where the browser has
// no keyboard API.
type KeyboardLayout = { get(code: string): string | undefined }
let layout: Promise<KeyboardLayout | null> | null = null
function keyboardLayout(): Promise<KeyboardLayout | null> {
  const keyboard = (navigator as Navigator & { keyboard?: { getLayoutMap(): Promise<KeyboardLayout> } }).keyboard
  layout ??= keyboard ? keyboard.getLayoutMap().catch(() => null) : Promise.resolve(null)
  return layout
}

// The keys MAYAK can register (internal/capture ParseKey), by the browser's
// key codes: a symbol as the character its key types without Shift (the Go
// side finds the key from it), from the layout, or else the key typed
// without Shift, or else a US keyboard's.
function keyName(e: Pick<KeyboardEvent, 'code' | 'key' | 'shiftKey'>, map: KeyboardLayout | null): string {
  const code = e.code
  if (/^F([1-9]|1[0-9]|2[0-4])$/.test(code)) return code
  if (/^Key[A-Z]$/.test(code)) return code.slice(3)
  if (/^Digit[0-9]$/.test(code)) return code.slice(5)
  if (/^Numpad[0-9]$/.test(code)) return 'Num ' + code.slice(6)
  if (['Insert', 'Delete', 'Home', 'End', 'PageUp', 'PageDown', 'Pause', 'ScrollLock', 'PrintScreen'].includes(code)) {
    return code
  }
  if (!(code in usSymbols)) return ''
  const symbol = map?.get(code) || (!e.shiftKey && e.key.length === 1 ? e.key : '') || usSymbols[code]
  return symbol === '+' ? '' : symbol
}

// The key written as the Go side writes it: "Ctrl + Shift + F10", "Alt + ;".
export function recordedKey(
  e: Pick<KeyboardEvent, 'code' | 'key' | 'ctrlKey' | 'shiftKey' | 'altKey' | 'metaKey'>,
  map: KeyboardLayout | null = null,
): string {
  const key = keyName(e, map)
  if (!key) return ''
  const mods = [e.ctrlKey && 'Ctrl', e.shiftKey && 'Shift', e.altKey && 'Alt', e.metaKey && 'Win'].filter(Boolean)
  return [...mods, key].join(' + ')
}

// MAYAK's own capture (experimental): its key, pressed while EFT is in front,
// takes the game's screen into the screenshot folder, without EFT's notice;
// the switch does the same after a click in the game when the screen is one
// MAYAK reads. The button records the next key pressed (Esc gives up); the
// cross clears it.
export function CaptureKeyField({
  value,
  auto,
  onAuto,
  autoOff,
  onAutoOff,
  tuning,
  onTuning,
  eftKeys,
  hasFolder,
  t,
  onChange,
}: Props) {
  const [recording, setRecording] = useState(false)
  const [map, setMap] = useState<KeyboardLayout | null>(null)
  useEffect(() => {
    void keyboardLayout().then(setMap)
  }, [])
  useEffect(() => {
    if (!recording) return
    const onKey = (e: KeyboardEvent) => {
      e.preventDefault()
      e.stopPropagation()
      if (e.code === 'Escape') {
        setRecording(false)
        return
      }
      const key = recordedKey(e, map)
      if (!key) return
      setRecording(false)
      onChange(key)
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [recording, onChange, map])
  const sameAsEft = !!value && eftKeys.some((k) => k.toLowerCase() === value.toLowerCase())
  return (
    <div className="field">
      <div className="capture-no-position" role="note">
        <MapPinOff />
        <div>
          <strong>{t('captureNoPosition')}</strong>
          <p>{t('captureNoPositionHelp')}</p>
        </div>
      </div>
      <div className="capture-section">
        <Label htmlFor="capture-key" className="label-with-badge">
          {t('captureKey')}
          <Badge className="outline">{t('experimental')}</Badge>
        </Label>
        <div className="input-action">
          <Button
            id="capture-key"
            type="button"
            variant="secondary"
            className="capture-key-button"
            onClick={() => setRecording(!recording)}
            aria-pressed={recording}
          >
            <Keyboard />
            {recording ? t('captureKeyRecording') : value || t('captureKeyNone')}
          </Button>
          {value && (
            <Button type="button" variant="ghost" onClick={() => onChange('')} title={t('captureKeyClear')}>
              <X />
              {t('captureKeyClear')}
            </Button>
          )}
        </div>
        <p className="help">{t('captureKeyHelp')}</p>
        {sameAsEft && <p className="help warn-text">{t('captureKeySameAsEft')}</p>}
      </div>
      <div className="capture-section">
        <div className="switch-row">
          <div>
            <Label htmlFor="capture-auto" className="label-with-badge">
              {t('captureAuto')}
              <Badge className="outline">{t('experimental')}</Badge>
            </Label>
            <p className="help">{t('captureAutoHelp')}</p>
          </div>
          <Switch id="capture-auto" checked={auto} onCheckedChange={onAuto} />
        </div>
        {/* What the auto recognition reads, each one with how it is read
            under it. Its times (config CaptureSettleMs and the others) are
            not on the page: the defaults suit. */}
        {auto && (
          <div className="capture-panel">
            <div className="capture-panel-title">{t('captureKinds')}</div>
            {captureKinds.map((kind) => {
              const on = !autoOff.includes(kind)
              const method = triggerSettings.find((m) => m.kind === kind)
              return (
                <div className="capture-kind" key={kind}>
                  <label>
                    <input
                      type="checkbox"
                      checked={on}
                      onChange={(e) =>
                        onAutoOff(
                          e.target.checked
                            ? autoOff.filter((k) => k !== kind)
                            : captureKinds.filter((k) => autoOff.includes(k) || k === kind),
                        )
                      }
                    />
                    {t(captureKindLabels[kind])}
                  </label>
                  {on && method && (
                    <div className="capture-method" role="radiogroup" aria-label={t(method.label)}>
                      <span>{t(method.label)}</span>
                      {method.choices.map(([trigger, label]) => (
                        <label key={trigger}>
                          <input
                            type="radio"
                            name={`capture-${kind}-trigger`}
                            checked={tuning[method.key] === trigger}
                            onChange={() => onTuning({ [method.key]: trigger })}
                          />
                          {t(label)}
                        </label>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
            {autoOff.length >= captureKinds.length && <p className="help warn-text">{t('captureKindsNone')}</p>}
          </div>
        )}
      </div>
      {(value || auto) && !hasFolder && <p className="help warn-text">{t('captureKeyNoFolder')}</p>}
    </div>
  )
}
