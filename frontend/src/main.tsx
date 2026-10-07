import { useEffect, useRef, useState } from 'react'
import { createRoot } from 'react-dom/client'
import {
  Activity,
  Bug,
  Check,
  CircleX,
  ExternalLink,
  Eye,
  EyeOff,
  FolderOpen,
  MapPinned,
  MonitorCog,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  ScanLine,
  Volume2,
  Bell,
  ChevronRight,
  Wifi,
  X,
} from 'lucide-react'
import {
  BrowserOpenURL,
  EventsOn,
  desktop,
  GameLanguages,
  AnalyzeLatestScreenshot,
  AutoDetectEFTDirectories,
  AutoDetectRemoteID,
  GetUpdateStatus,
  ChooseLogsDirectory,
  ChooseScreenshotDirectory,
  ChooseSoundFile,
  GetLogs,
  GetSettings,
  GetStatus,
  PersistSettings,
  PreviewSound,
  RefreshCatalog,
  OpenQuestPage,
  OpenHideoutDiagnostics,
  RefreshTracker,
  RefreshTrackerKeyNames,
  SaveSettings,
  TestRemote,
  VoicePacks,
} from './desktop'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Badge } from './components/ui/badge'
import { Switch } from './components/ui/switch'
import { Tabs, TabsContent } from './components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './components/ui/select'
import type { VoicePack } from '../bindings/github.com/local/mayak/internal/sound/models'
import { type MessageKey, translate, translateAnalysisStage } from './i18n'
import {
  RemoteTarget,
  Settings,
  Status,
  LogEntry,
  UpdateStatus,
  defaults,
  emptyUpdate,
  emptyStatus,
  maps,
  normalizeStatus,
  HostSection,
  hour12,
  hashSection,
  sectionTitles,
  languageNames,
  SOUND_DELAY_DEFAULTS,
  SOUND_DELAY_MAX,
} from './settings-model'
import { TrackerSection } from './TrackerSection'
import { LogsSection } from './LogsSection'
import { Metric } from './Metric'
import './style.css'

function App() {
  const [settings, setSettings] = useState<Settings>(defaults)
  const [activeTab, setActiveTab] = useState<HostSection>(hashSection)
  // The notification events whose sound details are open (the Notifications table).
  const [openEvents, setOpenEvents] = useState<string[]>([])
  const [status, setStatus] = useState<Status>(emptyStatus)
  const [updateStatus, setUpdateStatus] = useState<UpdateStatus>(emptyUpdate)
  const [notice, setNotice] = useState('')
  const [noticeError, setNoticeError] = useState(false)
  // Work that takes a while (a recheck of past logs): its toast stays until
  // it ends.
  const [working, setWorking] = useState('')
  const [remoteTestResult, setRemoteTestResult] = useState<'idle' | 'testing' | 'success' | 'error'>('idle')
  // The Remote IDs shown instead of dots (by row; -1 is the in-app browser's).
  const [shownRemoteIds, setShownRemoteIds] = useState<Set<number>>(() => new Set())
  const [busy, setBusy] = useState(false)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [logLevel, setLogLevel] = useState('all')
  const [logCategory, setLogCategory] = useState('all')
  const [logQuery, setLogQuery] = useState('')
  const [trackerToken, setTrackerToken] = useState('')
  const [now, setNow] = useState(Date.now())
  const settingsRef = useRef<Settings>(defaults)
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const [gameLanguages, setGameLanguages] = useState<string[]>(['ja', 'en'])
  const [voicePacks, setVoicePacks] = useState<VoicePack[]>([])
  const saveRevision = useRef(0)
  const backendReady = useRef(false)
  const persistQueue = useRef<Promise<unknown>>(Promise.resolve())
  const t = (key: Parameters<typeof translate>[1]) => translate(settings.language, key)
  useEffect(() => {
    let active = true
    const unsubscribers: (() => void)[] = []
    const boot = async () => {
      if (!active) return
      if (!desktop?.backend) {
        setNotice(translate(defaults.language, 'backendWait'))
        setNoticeError(true)
        return
      }
      try {
        // The game languages MAYAK reads come from the Go side (internal/locale).
        void GameLanguages()
          .then((list) => {
            if (active && Array.isArray(list) && list.length) setGameLanguages(list)
          })
          .catch(() => {})
        void VoicePacks()
          .then((list) => {
            if (active && Array.isArray(list)) setVoicePacks(list as VoicePack[])
          })
          .catch(() => {})
        const [s, v, l, u] = await Promise.all([
          GetSettings(),
          GetStatus(),
          GetLogs(),
          GetUpdateStatus().catch(() => emptyUpdate),
        ])
        if (!active) return
        settingsRef.current = {
          ...defaults,
          ...s,
          remoteTargets: Array.isArray((s as any).remoteTargets) ? (s as any).remoteTargets : [],
        } as Settings
        setSettings(settingsRef.current)
        setStatus(normalizeStatus(v))
        setLogs(Array.isArray(l) ? (l as LogEntry[]) : [])
        setUpdateStatus({ ...emptyUpdate, ...(u ?? {}) })
        backendReady.current = true
        unsubscribers.push(
          EventsOn('status:update', (next: Status) => setStatus((current) => normalizeStatus({ ...current, ...next }))),
        )
        unsubscribers.push(
          EventsOn('log:entry', (entry: LogEntry) => setLogs((current) => [...current.slice(-499), entry])),
        )
        unsubscribers.push(EventsOn('log:clear', () => setLogs([])))
        // A key assignment syncs the profile's past logs (syncAssignedHistory).
        unsubscribers.push(
          EventsOn(
            'tracker:history',
            (result: {
              mode: string
              sent: number
              error?: string
              deferred?: boolean
              running?: boolean
              idle?: boolean
            }) => {
              const language = settingsRef.current.language
              const mode = translate(
                language,
                result.mode === 'pve' ? 'trackerPVE' : result.mode === 'seasonal' ? 'trackerSeasonal' : 'trackerPVP',
              )
              if (result.running) {
                setWorking(translate(language, 'trackerHistoryWorking').replace('{mode}', mode))
                return
              }
              setWorking('')
              if (result.idle) return
              if (result.deferred) {
                setNotice(`${mode}: ${translate(language, 'trackerHistoryDeferred')}`)
                setNoticeError(false)
              } else if (result.error) {
                setNotice(`${mode}: ${translate(language, 'trackerHistoryFailed')} ${result.error}`)
                setNoticeError(true)
              } else {
                setNotice(
                  translate(language, 'trackerHistorySynced')
                    .replace('{mode}', mode)
                    .replace('{n}', String(result.sent)),
                )
                setNoticeError(false)
              }
            },
          ),
        )
        unsubscribers.push(
          EventsOn('update:status', (next: UpdateStatus) => setUpdateStatus({ ...emptyUpdate, ...next })),
        )
      } catch (error) {
        if (active) {
          setNotice(`${translate(defaults.language, 'initError')}: ${String(error)}`)
          setNoticeError(true)
        }
      }
    }
    void boot()
    return () => {
      active = false
      unsubscribers.forEach((unsubscribe) => unsubscribe())
    }
  }, [])
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [])
  // Success toasts fade out on their own.
  useEffect(() => {
    if (saveState !== 'saved') return
    const timer = window.setTimeout(() => setSaveState((state) => (state === 'saved' ? 'idle' : state)), 2000)
    return () => window.clearTimeout(timer)
  }, [saveState])
  useEffect(() => {
    if (!notice || noticeError) return
    const timer = window.setTimeout(() => setNotice(''), 3000)
    return () => window.clearTimeout(timer)
  }, [notice, noticeError])
  useEffect(() => {
    const changed = async () => {
      setActiveTab(hashSection())
      if (!backendReady.current) return
      // Pending saves land first, so the reload includes them.
      await persistQueue.current.catch(() => undefined)
      try {
        const s = await GetSettings()
        settingsRef.current = {
          ...defaults,
          ...s,
          remoteTargets: Array.isArray((s as any).remoteTargets) ? (s as any).remoteTargets : [],
        } as Settings
        setSettings(settingsRef.current)
      } catch {
        /* Keep the current values. */
      }
    }
    window.addEventListener('hashchange', changed)
    return () => window.removeEventListener('hashchange', changed)
  }, [])
  // The shell's display language is the Host's too: it sets <html lang>.
  useEffect(() => {
    const sync = () => {
      const language = document.documentElement.lang
      if ((language === 'ja' || language === 'en') && backendReady.current && settingsRef.current.language !== language)
        patch({ language })
    }
    const observer = new MutationObserver(sync)
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['lang'] })
    const timer = window.setInterval(sync, 1000)
    return () => {
      observer.disconnect()
      window.clearInterval(timer)
    }
  }, [])
  useEffect(() => {
    if (activeTab !== 'tracker') return
    let pending = false
    const refresh = async () => {
      if (!backendReady.current || pending) return
      pending = true
      try {
        await RefreshTrackerKeyNames()
      } catch {
        /* Keep the last fetched names when offline. */
      } finally {
        pending = false
      }
    }
    void refresh()
    const timer = window.setInterval(() => void refresh(), 60000)
    window.addEventListener('focus', refresh)
    return () => {
      window.clearInterval(timer)
      window.removeEventListener('focus', refresh)
    }
  }, [activeTab])
  const patch = (value: Partial<Settings>) => {
    if (!backendReady.current) return
    const next = { ...settingsRef.current, ...value }
    settingsRef.current = next
    setSettings(next)
    const revision = ++saveRevision.current
    setSaveState('saving')
    persistQueue.current = persistQueue.current
      .catch(() => undefined)
      .then(() => PersistSettings(next as any))
      .then(() => {
        if (revision === saveRevision.current) setSaveState('saved')
      })
      .catch((error) => {
        if (revision === saveRevision.current) {
          setSaveState('error')
          setNotice(String(error))
          setNoticeError(true)
        }
      })
  }
  const persistCurrent = () => {
    const revision = ++saveRevision.current
    const next = settingsRef.current
    setSaveState('saving')
    const pending = persistQueue.current.catch(() => undefined).then(() => SaveSettings(next as any))
    persistQueue.current = pending
    return pending
      .then(() => {
        if (revision === saveRevision.current) setSaveState('saved')
      })
      .catch((error) => {
        if (revision === saveRevision.current) setSaveState('error')
        throw error
      })
  }
  const run = async (action: () => Promise<unknown>, success: string) => {
    setBusy(true)
    setNotice('')
    setNoticeError(false)
    try {
      await action()
      setNotice(success)
    } catch (error) {
      setNotice(String(error))
      setNoticeError(true)
    } finally {
      setBusy(false)
    }
  }
  const save = () => run(() => persistCurrent(), t('saved'))
  const test = async () => {
    setBusy(true)
    setRemoteTestResult('testing')
    try {
      await persistCurrent()
      await TestRemote()
      setRemoteTestResult('success')
    } catch {
      setRemoteTestResult('error')
    } finally {
      setBusy(false)
    }
  }
  const browseScreens = async () => {
    const path = await ChooseScreenshotDirectory()
    if (path) patch({ screenshotDirectory: path })
  }
  const browseLogs = async () => {
    const path = await ChooseLogsDirectory()
    if (path) patch({ logsDirectory: path })
  }
  const openFolder = async (action: () => Promise<void>) => {
    try {
      await action()
    } catch (error) {
      setNotice(String(error))
      setNoticeError(true)
    }
  }
  const detectRemote = () =>
    run(async () => {
      setRemoteTestResult('idle')
      const id = await AutoDetectRemoteID()
      if (settings.remoteTargets.some((target) => target.id === id)) return
      patch({
        remoteTargets: [
          ...settings.remoteTargets,
          { id, name: `Remote ${settings.remoteTargets.length + 1}`, map: true, tasks: true },
        ],
      })
    }, t('remoteDetected'))
  const updateRemoteTarget = (index: number, value: Partial<RemoteTarget>) =>
    patch({
      remoteTargets: settings.remoteTargets.map((target, i) => (i === index ? { ...target, ...value } : target)),
    })
  const addRemoteTarget = () =>
    patch({
      remoteTargets: [
        ...settings.remoteTargets,
        { id: '', name: `Remote ${settings.remoteTargets.length + 1}`, map: true, tasks: true },
      ],
    })
  const removeRemoteTarget = (index: number) => {
    setShownRemoteIds(new Set())
    patch({ remoteTargets: settings.remoteTargets.filter((_, i) => i !== index) })
  }
  const toggleRemoteId = (index: number) =>
    setShownRemoteIds((shown) => {
      const next = new Set(shown)
      if (!next.delete(index)) next.add(index)
      return next
    })
  // The eye button that shows or hides a Remote ID, like the shell's codes.
  const revealRemoteId = (index: number) => {
    const shown = shownRemoteIds.has(index)
    const label = t(shown ? 'hideRemoteId' : 'showRemoteId')
    return (
      <Button
        type="button"
        size="sm"
        variant="ghost"
        className="reveal"
        aria-label={label}
        title={label}
        aria-pressed={shown}
        onClick={() => toggleRemoteId(index)}
      >
        {shown ? <EyeOff /> : <Eye />}
      </Button>
    )
  }
  const autoDetect = () =>
    run(async () => {
      const detected = await AutoDetectEFTDirectories()
      patch({
        screenshotDirectory: detected.screenshotDirectory || settings.screenshotDirectory,
        logsDirectory: detected.logsDirectory || settings.logsDirectory,
      })
    }, t('foldersDetected'))
  const analyzeLatest = () => run(() => AnalyzeLatestScreenshot(), t('latestAnalyzed'))
  type SoundPathKey =
    | 'questSoundPath'
    | 'errorSoundPath'
    | 'taskNotMatchedSoundPath'
    | 'remoteErrorSoundPath'
    | 'itemSoundPath'
    | 'itemNotMatchedSoundPath'
    | 'matchFoundSoundPath'
    | 'raidStartSoundPath'
    | 'runThroughSoundPath'
    | 'gameStartSoundPath'
    | 'questItemsSoundPath'
    | 'taskFailedSoundPath'
    | 'gameExitSoundPath'
  const chooseSound = async (key: SoundPathKey) => {
    try {
      const path = await ChooseSoundFile()
      if (path) patch({ [key]: path } as Partial<Settings>)
    } catch (error) {
      setNotice(String(error))
      setNoticeError(true)
    }
  }
  const previewSound = async (kind: string, path: string) => {
    try {
      await PreviewSound(kind, choiceOf(kind, path) === 'custom' ? path : '', voiceOf(kind)?.id ?? '', volumeOf(kind))
    } catch (error) {
      setNotice(String(error))
      setNoticeError(true)
    }
  }
  const voiceName = (pack: VoicePack) => pack.name[settings.language] || pack.name.en || pack.id
  // The voice for all: the one chosen, "beep" for the beeps, and none chosen
  // the language's default (as internal/app/app_sound.go baseVoice).
  const defaultVoiceId = settings.language === 'ja' ? 'tsumugi' : 'heart'
  const baseVoiceId = settings.soundVoice || defaultVoiceId
  const voice = voicePacks.find((pack) => pack.id === baseVoiceId)
  // A notification's own choice: a voice, "beep", or "custom" (its file);
  // none, the voice for all. A file chosen is always "custom".
  const soundVoices = settings.soundVoices ?? {}
  const choiceOf = (kind: string, path: string) => (path ? 'custom' : (soundVoices[kind] ?? '__default'))
  const voiceOf = (kind: string) => {
    const own = soundVoices[kind]
    if (own === undefined || own === 'custom') return voice
    return voicePacks.find((pack) => pack.id === own)
  }
  const setChoiceOf = (kind: string, pathKey: SoundPathKey, value: string) => {
    const next = { ...soundVoices }
    if (value === '__default') delete next[kind]
    else next[kind] = value === '__beep' ? 'beep' : value
    // Another choice than a file lets go of the file.
    patch({ soundVoices: next, ...(value !== 'custom' ? { [pathKey]: '' } : {}) } as Partial<Settings>)
  }
  // A notification's volume: the one for all with its own adjustment (a
  // slider, ±50), within 0–100 (as internal/app/app_sound.go volumeFor).
  const volumeOffsets = settings.soundVolumeOffsets ?? {}
  const offsetOf = (kind: string) => volumeOffsets[kind] ?? 0
  const volumeOf = (kind: string) => Math.min(100, Math.max(0, settings.soundVolume + offsetOf(kind)))
  const setOffsetOf = (kind: string, offset: number) => {
    const next = { ...volumeOffsets }
    const clamped = Math.min(50, Math.max(-50, offset))
    if (clamped === 0) delete next[kind]
    else next[kind] = clamped
    patch({ soundVolumeOffsets: next })
  }
  // A notification's delay in seconds, so it does not speak over the game
  // (as internal/app/app_sound.go delayFor; a kind not set has its default).
  const soundDelays = settings.soundDelays ?? {}
  const delayOf = (kind: string) => soundDelays[kind] ?? SOUND_DELAY_DEFAULTS[kind] ?? 0
  const setDelayOf = (kind: string, seconds: number) =>
    patch({ soundDelays: { ...soundDelays, [kind]: Math.min(SOUND_DELAY_MAX, Math.max(0, Math.round(seconds))) } })
  const offsetLabel = (offset: number) => (offset > 0 ? `+${offset}` : offset === 0 ? '±0' : `${offset}`)
  // A sound file's name as chosen: taken in, it is sounds/<hash>-<name>.
  const soundFileName = (path: string) => (path.split(/[\\/]/).pop() || '').replace(/^[0-9a-f]{16}-/, '')
  const quote = (text: string) => (settings.language === 'ja' ? `「${text}」` : `“${text}”`)
  // What a notification plays: its file, the line of its voice, or the beeps.
  const saidBy = (kind: string, path: string) => {
    if (choiceOf(kind, path) === 'custom') return path ? soundFileName(path) : t('noSoundFile')
    const pack = voiceOf(kind)
    return pack ? `${voiceName(pack)}：${quote(pack.lines?.[kind] || '')}` : t('soundVoiceBeep')
  }
  const soundAlerts = [
    {
      id: 'quest-sound',
      label: t('questSuccess'),
      kind: 'quest',
      enabledKey: 'questSoundEnabled',
      pathKey: 'questSoundPath',
    },
    {
      id: 'task-not-matched-sound',
      label: t('taskNotMatchedSound'),
      kind: 'taskNotMatched',
      enabledKey: 'taskNotMatchedSoundEnabled',
      pathKey: 'taskNotMatchedSoundPath',
    },
    {
      id: 'item-sound',
      label: t('itemSound'),
      kind: 'item',
      enabledKey: 'itemSoundEnabled',
      pathKey: 'itemSoundPath',
    },
    {
      id: 'item-not-matched-sound',
      label: t('itemNotMatchedSound'),
      kind: 'itemNotMatched',
      enabledKey: 'itemNotMatchedSoundEnabled',
      pathKey: 'itemNotMatchedSoundPath',
    },
    {
      id: 'error-sound',
      label: t('recognitionError'),
      kind: 'error',
      enabledKey: 'errorSoundEnabled',
      pathKey: 'errorSoundPath',
    },
    {
      id: 'remote-error-sound',
      label: t('remoteErrorSound'),
      kind: 'remoteError',
      enabledKey: 'remoteErrorSoundEnabled',
      pathKey: 'remoteErrorSoundPath',
    },
    {
      id: 'match-found-sound',
      label: t('matchFoundSound'),
      kind: 'matchFound',
      enabledKey: 'matchFoundSoundEnabled',
      pathKey: 'matchFoundSoundPath',
    },
    {
      id: 'raid-start-sound',
      label: t('raidStartSound'),
      kind: 'raidStart',
      enabledKey: 'raidStartSoundEnabled',
      pathKey: 'raidStartSoundPath',
    },
    {
      id: 'run-through-sound',
      label: t('runThroughSound'),
      kind: 'runThrough',
      enabledKey: 'runThroughSoundEnabled',
      pathKey: 'runThroughSoundPath',
    },
    {
      id: 'game-start-sound',
      label: t('gameStartSound'),
      kind: 'gameStart',
      enabledKey: 'gameStartSoundEnabled',
      pathKey: 'gameStartSoundPath',
    },
    {
      id: 'quest-items-sound',
      label: t('questItemsSound'),
      kind: 'questItems',
      enabledKey: 'questItemsSoundEnabled',
      pathKey: 'questItemsSoundPath',
    },
    {
      id: 'task-failed-sound',
      label: t('taskFailedSound'),
      kind: 'taskFailed',
      enabledKey: 'taskFailedSoundEnabled',
      pathKey: 'taskFailedSoundPath',
    },
    {
      id: 'game-exit-sound',
      label: t('gameExitSound'),
      kind: 'gameExit',
      enabledKey: 'gameExitSoundEnabled',
      pathKey: 'gameExitSoundPath',
    },
  ] as const
  // The notification events by group (app_notify.go notifyEvents): those with
  // an alert can sound; all can show as a toast and as a desktop notification.
  const notifyGroups: { label: MessageKey; events: string[] }[] = [
    { label: 'notifyGroup_raid', events: ['matchFound', 'raidStart', 'runThrough', 'questItems', 'taskFailed'] },
    { label: 'notifyGroup_game', events: ['gameStart', 'gameExit'] },
    {
      label: 'notifyGroup_screenshot',
      events: ['quest', 'taskNotMatched', 'item', 'itemNotMatched', 'position', 'profile', 'error'],
    },
    {
      label: 'notifyGroup_tracker',
      events: ['trackerTask', 'trackerFailed', 'trackerLevel', 'trackerHistory', 'prestige'],
    },
    { label: 'notifyGroup_squad', events: ['squadSelf', 'squadMembers', 'squadRelay'] },
    { label: 'notifyGroup_remote', events: ['remoteError'] },
  ]
  const notifyEvents = notifyGroups.flatMap((group) => group.events)
  const alertOf = (event: string) => soundAlerts.find((alert) => alert.kind === event)
  const eventLabel = (event: string) => alertOf(event)?.label ?? t(`notifyEvent_${event}` as MessageKey)
  const toastsOff = settings.toastsOff ?? []
  const desktopOn = settings.desktopOn ?? []
  const setToast = (event: string, on: boolean) =>
    patch({ toastsOff: on ? toastsOff.filter((e) => e !== event) : [...toastsOff.filter((e) => e !== event), event] })
  const setDesktop = (event: string, on: boolean) =>
    patch({ desktopOn: on ? [...desktopOn.filter((e) => e !== event), event] : desktopOn.filter((e) => e !== event) })
  // The "all" row: on when every event is; turning it sets them all.
  const allSounds = soundAlerts.every((alert) => settings[alert.enabledKey])
  const allToasts = notifyEvents.every((event) => !toastsOff.includes(event))
  const allDesktop = notifyEvents.every((event) => desktopOn.includes(event))
  const setAllSounds = (on: boolean) =>
    patch(Object.fromEntries(soundAlerts.map((alert) => [alert.enabledKey, on])) as Partial<Settings>)
  const toggleEvent = (event: string) =>
    setOpenEvents((open) => (open.includes(event) ? open.filter((e) => e !== event) : [...open, event]))
  const updateStateLabel = (update: UpdateStatus) => {
    switch (update.state) {
      case 'checking':
        return t('processing')
      case 'current':
        return t('updateUpToDate')
      case 'available':
        return t('updateAvailable')
      case 'downloading':
        return `${t('updateDownloading')} ${update.progress}%`
      case 'ready':
        return t('updateReady')
      case 'unsupported':
        return `${t('updateUnsupported')} (${update.platform})`
      case 'error':
        return t('updateFailed')
      default:
        return t('updateIdle')
    }
  }
  const trackerModeLabel = (mode: string) =>
    mode === 'pve' ? t('trackerPVE') : mode === 'seasonal' ? t('trackerSeasonal') : t('trackerPVP')
  const trackerConnectionLabel =
    {
      disabled: t('trackerStatusDisabled'),
      'waiting-profile': t('trackerStatusWaitingProfile'),
      connecting: t('trackerStatusConnecting'),
      connected: t('trackerStatusConnected'),
      'missing-token': t('trackerStatusMissingToken'),
      error: t('trackerStatusError'),
    }[status.tracker.connection] ?? status.tracker.connection
  const formatDuration = (seconds: number) =>
    `${Math.floor(Math.max(0, seconds) / 60)
      .toString()
      .padStart(2, '0')}:${Math.floor(Math.max(0, seconds) % 60)
      .toString()
      .padStart(2, '0')}`
  const raidElapsed =
    status.raidActive && status.raidStartedAt
      ? formatDuration((now - new Date(status.raidStartedAt).getTime()) / 1000)
      : '—'
  const runThroughRemaining =
    status.raidActive && status.runThroughAt ? Math.max(0, (new Date(status.runThroughAt).getTime() - now) / 1000) : 0
  const runThroughLabel =
    status.raidActive && status.runThroughAt
      ? runThroughRemaining > 0
        ? formatDuration(runThroughRemaining)
        : t('runThroughReady')
      : '—'
  // Saves and notices show as toasts at the bottom right:
  // successes fade out, failures stay until dismissed or retried.
  const toasts = (
    <div className="toast-stack" aria-live="polite">
      {working && (
        <div className="toast working" role="status">
          <RefreshCw className="spin" />
          <span>{working}</span>
        </div>
      )}
      {saveState === 'saved' && (
        <div className="toast success" role="status">
          <Check />
          <span>{t('settingsSaved')}</span>
        </div>
      )}
      {saveState === 'error' && (
        <div className="toast error" role="alert">
          <span>
            {t('settingsSaveFailed')}
            {notice && <small>{notice}</small>}
          </span>
          <Button type="button" size="sm" variant="secondary" onClick={save} disabled={busy}>
            <RefreshCw />
            {t('settingsRetry')}
          </Button>
        </div>
      )}
      {notice && saveState !== 'error' && (
        <div className={`toast ${noticeError ? 'error' : 'success'}`} role={noticeError ? 'alert' : 'status'}>
          {!noticeError && <Check />}
          <span>{notice}</span>
          <button type="button" className="toast-close" onClick={() => setNotice('')} aria-label="×">
            <X />
          </button>
        </div>
      )}
    </div>
  )
  return (
    <main className="shell">
      <Tabs className="app-tabs" value={activeTab}>
        <div className={`section-head${['status', 'logs'].includes(activeTab) ? '' : ' narrow'}`}>
          <h1>{t(sectionTitles[activeTab])}</h1>
        </div>
        <TabsContent value="folders">
          <div className="settings-stack">
            <Card>
              <CardHeader>
                <div className="icon-title">
                  <FolderOpen />
                  <div>
                    <CardTitle>{t('foldersTitle')}</CardTitle>
                    <CardDescription>{t('foldersDescription')}</CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <Button
                  className="detect-button"
                  type="button"
                  variant="secondary"
                  onClick={autoDetect}
                  disabled={busy}
                >
                  <RefreshCw />
                  {t('autoDetect')}
                </Button>
                <div className="field">
                  <Label htmlFor="screenshots">{t('screenshotsFolder')}</Label>
                  <div className="input-action">
                    <Input
                      id="screenshots"
                      value={settings.screenshotDirectory}
                      onChange={(e) => patch({ screenshotDirectory: e.target.value })}
                      placeholder="C:\Users\...\Escape from Tarkov\Screenshots"
                    />
                    <Button type="button" variant="secondary" onClick={browseScreens}>
                      <FolderOpen />
                      {t('select')}
                    </Button>
                  </div>
                </div>
                <div className="field">
                  <Label htmlFor="logs">{t('logsFolder')}</Label>
                  <div className="input-action">
                    <Input
                      id="logs"
                      value={settings.logsDirectory}
                      onChange={(e) => patch({ logsDirectory: e.target.value })}
                      placeholder="Escape from Tarkov\Logs"
                    />
                    <Button type="button" variant="secondary" onClick={browseLogs}>
                      <FolderOpen />
                      {t('select')}
                    </Button>
                  </div>
                </div>
                <div className="switch-stack">
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="screenshot-cleanup">{t('screenshotCleanup')}</Label>
                      <p className="help">{t('screenshotCleanupHelp')}</p>
                    </div>
                    <Switch
                      id="screenshot-cleanup"
                      checked={settings.screenshotCleanup}
                      onCheckedChange={(screenshotCleanup) => patch({ screenshotCleanup })}
                    />
                  </div>
                </div>
                {settings.screenshotCleanup && (
                  <div className="retention-fields">
                    <div className="field">
                      <Label htmlFor="retain-count">{t('retainCount')}</Label>
                      <Input
                        id="retain-count"
                        type="number"
                        min="0"
                        max="100000"
                        value={settings.screenshotRetainCount}
                        onChange={(e) => patch({ screenshotRetainCount: Math.max(0, Number(e.target.value) || 0) })}
                      />
                      <p className="help">{t('zeroDisables')}</p>
                    </div>
                    <div className="field">
                      <Label htmlFor="retain-hours">{t('retainHours')}</Label>
                      <Input
                        id="retain-hours"
                        type="number"
                        min="0"
                        max="87600"
                        value={settings.screenshotRetainHours}
                        onChange={(e) => patch({ screenshotRetainHours: Math.max(0, Number(e.target.value) || 0) })}
                      />
                      <p className="help">{t('zeroDisables')}</p>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="recognition">
          <div className="settings-stack">
            <Card>
              <CardHeader>
                <CardTitle>{t('analysisTitle')}</CardTitle>
                <CardDescription>{t('analysisDescription')}</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="field">
                  <Label>{t('gameMode')}</Label>
                  <Select value={settings.gameMode} onValueChange={(gameMode) => patch({ gameMode })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="auto">{t('gameModeAuto')}</SelectItem>
                      <SelectItem value="pve">{t('gameModePVE')}</SelectItem>
                      <SelectItem value="regular">{t('gameModePVP')}</SelectItem>
                      <SelectItem value="pvp-season">{t('gameModeSeason')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="field">
                  <Label>{t('gameLanguage')}</Label>
                  <Select
                    value={settings.gameLanguage || 'auto'}
                    onValueChange={(gameLanguage) => patch({ gameLanguage })}
                  >
                    <SelectTrigger aria-label={t('gameLanguage')}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="auto">{t('gameLanguageAuto')}</SelectItem>
                      {gameLanguages.map((language) => (
                        <SelectItem key={language} value={language}>
                          {languageNames[language] || language}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="help-text">{t('gameLanguageHelp')}</p>
                </div>
                <div className="field">
                  <Label>{t('ocrEngine')}</Label>
                  <Select value={settings.ocrEngine} onValueChange={(ocrEngine) => patch({ ocrEngine })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="tesseract">{t('tesseractOCR')}</SelectItem>
                      <SelectItem value="windows">{t('windowsOCR')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {settings.ocrEngine === 'tesseract' && (
                  <div className="field">
                    <Label htmlFor="tesseract">{t('tesseractPath')}</Label>
                    <Input
                      id="tesseract"
                      value={settings.tesseractPath}
                      onChange={(e) => patch({ tesseractPath: e.target.value })}
                      placeholder={t('tesseractPlaceholder')}
                    />
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="remote">
          <div className="settings-stack">
            <Card className="remote-card">
              <CardHeader>
                <div className="icon-title">
                  <Wifi />
                  <div>
                    <CardTitle>{t('remoteTitle')}</CardTitle>
                    <CardDescription>{t('remoteDescription')}</CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="remote-target-heading">
                  <Label>{t('remoteTargets')}</Label>
                  <div>
                    <Button type="button" size="sm" variant="secondary" onClick={addRemoteTarget}>
                      <Plus />
                      {t('addRemote')}
                    </Button>
                    <Button type="button" size="sm" variant="secondary" onClick={detectRemote} disabled={busy}>
                      <RefreshCw />
                      {t('detectRemote')}
                    </Button>
                  </div>
                </div>
                <div className="remote-targets">
                  {settings.remoteTargets.length === 0 ? (
                    <p className="empty">{t('noRemoteTargets')}</p>
                  ) : (
                    settings.remoteTargets.map((target, index) => (
                      <div className="remote-target" key={index}>
                        <div className="remote-target-inputs">
                          <Input
                            aria-label={t('remoteName')}
                            value={target.name}
                            onChange={(e) => updateRemoteTarget(index, { name: e.target.value })}
                            placeholder={t('remoteName')}
                          />
                          <div className="secret-field">
                            <Input
                              aria-label={t('tarkovRemoteId')}
                              className={shownRemoteIds.has(index) ? '' : 'masked'}
                              value={target.id}
                              onChange={(e) => {
                                setRemoteTestResult('idle')
                                updateRemoteTarget(index, { id: e.target.value })
                              }}
                              placeholder={t('remotePlaceholder')}
                              autoComplete="off"
                              spellCheck={false}
                            />
                            {revealRemoteId(index)}
                          </div>
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            aria-label={t('removeRemote')}
                            onClick={() => removeRemoteTarget(index)}
                          >
                            <X />
                          </Button>
                        </div>
                        <div className="remote-target-roles">
                          <label>
                            <Switch
                              checked={target.map}
                              onCheckedChange={(map) => updateRemoteTarget(index, { map })}
                            />
                            <span>{t('remoteMapRole')}</span>
                          </label>
                          <label>
                            <Switch
                              checked={target.tasks}
                              onCheckedChange={(tasks) => updateRemoteTarget(index, { tasks })}
                            />
                            <span>{t('remoteTaskRole')}</span>
                          </label>
                        </div>
                      </div>
                    ))
                  )}
                </div>
                {settings.browserRemoteId && (
                  <p className="help secret-help">
                    {t('browserRemote').replace(
                      '{id}',
                      shownRemoteIds.has(-1) ? settings.browserRemoteId : settings.browserRemoteId.replace(/S/g, '•'),
                    )}
                    {revealRemoteId(-1)}
                  </p>
                )}
                <p className="help remote-help">
                  <span>{t('remoteHelp')}</span>
                  <button type="button" className="inline-link" onClick={() => BrowserOpenURL('https://tarkov.dev/')}>
                    <ExternalLink />
                    {t('openTarkovDevRemote')}
                  </button>
                </p>
                <div className="field remote-map-fallback">
                  <Label>{t('mapFallback')}</Label>
                  <Select
                    value={settings.map || '__auto'}
                    onValueChange={(map) => patch({ map: map === '__auto' ? '' : map })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__auto">{t('mapAuto')}</SelectItem>
                      {maps.map((map) => (
                        <SelectItem key={map} value={map}>
                          {map}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="switch-stack remote-switches">
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="open-map-on-raid-start">{t('openMapOnRaidStart')}</Label>
                      <p className="help">{t('openMapOnRaidStartHelp')}</p>
                    </div>
                    <Switch
                      id="open-map-on-raid-start"
                      checked={settings.openMapOnRaidStart}
                      onCheckedChange={(openMapOnRaidStart) => patch({ openMapOnRaidStart })}
                    />
                  </div>
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="navigate-map-on-shot">{t('navigateMapOnPositionScreenshot')}</Label>
                      <p className="help">{t('navigateMapOnPositionScreenshotHelp')}</p>
                    </div>
                    <Switch
                      id="navigate-map-on-shot"
                      checked={settings.navigateMapOnPositionScreenshot}
                      onCheckedChange={(navigateMapOnPositionScreenshot) => patch({ navigateMapOnPositionScreenshot })}
                    />
                  </div>
                </div>
                <div className="connection-test-row">
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={test}
                    disabled={busy || !settings.remoteTargets.some((target) => target.id.trim())}
                  >
                    <Wifi />
                    {t('testConnection')}
                  </Button>
                  {remoteTestResult !== 'idle' && (
                    <span className={`connection-test-result ${remoteTestResult}`}>
                      {remoteTestResult === 'success' ? (
                        <Check />
                      ) : remoteTestResult === 'error' ? (
                        <CircleX />
                      ) : (
                        <RefreshCw className="spin" />
                      )}
                      {remoteTestResult === 'success'
                        ? t('remoteConnected')
                        : remoteTestResult === 'error'
                          ? t('remoteConnectionFailed')
                          : t('remoteConnecting')}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="tracker">
          <div className="settings-stack">
            <TrackerSection
              settings={settings}
              status={status}
              t={t}
              busy={busy}
              run={run}
              patch={patch}
              setBusy={setBusy}
              setNotice={setNotice}
              setNoticeError={setNoticeError}
              setWorking={setWorking}
              trackerModeLabel={trackerModeLabel}
              trackerToken={trackerToken}
              setTrackerToken={setTrackerToken}
            />
          </div>
        </TabsContent>
        <TabsContent value="sounds">
          <div className="settings-stack">
            <Card>
              <CardHeader>
                <div className="icon-title">
                  <Volume2 />
                  <div>
                    <CardTitle>{t('notifyAllTitle')}</CardTitle>
                    <CardDescription>{t('notifyAllDescription')}</CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="switch-stack">
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="sounds-enabled">{t('soundsEnabled')}</Label>
                      <p className="help">{t('soundsHelp')}</p>
                    </div>
                    <Switch
                      id="sounds-enabled"
                      checked={settings.soundsEnabled}
                      onCheckedChange={(soundsEnabled) => patch({ soundsEnabled })}
                    />
                  </div>
                  {settings.soundsEnabled && (
                    <div className="field">
                      <Label htmlFor="sound-volume">
                        {t('volume')} {settings.soundVolume}%
                      </Label>
                      <input
                        className="range"
                        id="sound-volume"
                        type="range"
                        min="0"
                        max="100"
                        step="1"
                        value={settings.soundVolume}
                        onChange={(e) => patch({ soundVolume: Number(e.target.value) })}
                      />
                    </div>
                  )}
                </div>
                {settings.soundsEnabled && (
                  <div className="sound-options">
                    <h3 className="sound-section">{t('soundVoicesSection')}</h3>
                    <div className="field">
                      <Label>{t('soundVoice')}</Label>
                      <Select
                        value={voice ? voice.id : '__beep'}
                        onValueChange={(id) => patch({ soundVoice: id === '__beep' ? 'beep' : id })}
                      >
                        <SelectTrigger aria-label={t('soundVoice')} className="sound-voice">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {voicePacks.map((pack) => (
                            <SelectItem key={pack.id} value={pack.id}>
                              {voiceName(pack)}
                              {pack.id === defaultVoiceId ? t('soundVoiceStandard') : ''}
                            </SelectItem>
                          ))}
                          <SelectItem value="__beep">{t('soundVoiceBeep')}</SelectItem>
                        </SelectContent>
                      </Select>
                      <p className="help-text">
                        {voice && (
                          <>
                            {voice.credit} ·{' '}
                            <button type="button" className="inline-link" onClick={() => BrowserOpenURL(voice.terms)}>
                              {t('soundVoiceTerms')}
                            </button>
                            <br />
                          </>
                        )}
                        {t('soundVoiceHelp')}
                      </p>
                      <div>
                        <Button
                          type="button"
                          size="sm"
                          variant="secondary"
                          disabled={Object.keys(soundVoices).length === 0}
                          onClick={() => patch({ soundVoices: {} })}
                        >
                          <RotateCcw />
                          {t('soundVoiceAll')}
                        </Button>
                      </div>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
            {/* The notification events × the ways they show (app_notify.go): a
                sound, a toast over the pages, a desktop notification of the OS. */}
            <Card>
              <CardHeader>
                <div className="icon-title">
                  <Bell />
                  <div>
                    <CardTitle>{t('notifyEventsTitle')}</CardTitle>
                    <CardDescription>{t('notifyEventsDescription')}</CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="notify-table" role="table" aria-label={t('notifyEventsTitle')}>
                  <div className="notify-row notify-head" role="row">
                    <span role="columnheader">{t('notifyEvent')}</span>
                    <span role="columnheader">{t('notifySound')}</span>
                    <span role="columnheader">{t('notifyToast')}</span>
                    <span role="columnheader">{t('notifyDesktop')}</span>
                  </div>
                  <div className="notify-row notify-all" role="row">
                    <span role="rowheader">{t('notifyAll')}</span>
                    <Switch
                      id={'notify-all-sound'}
                      aria-label={`${t('notifyAll')}: ${t('notifySound')}`}
                      checked={allSounds}
                      onCheckedChange={setAllSounds}
                      disabled={!settings.soundsEnabled}
                    />
                    <Switch
                      id={'notify-all-toast'}
                      aria-label={`${t('notifyAll')}: ${t('notifyToast')}`}
                      checked={allToasts}
                      onCheckedChange={(on) => patch({ toastsOff: on ? [] : notifyEvents })}
                    />
                    <Switch
                      id={'notify-all-desktop'}
                      aria-label={`${t('notifyAll')}: ${t('notifyDesktop')}`}
                      checked={allDesktop}
                      onCheckedChange={(on) => patch({ desktopOn: on ? notifyEvents : [] })}
                    />
                  </div>
                  {notifyGroups.map((group) => (
                    <div className="notify-group" role="rowgroup" key={group.label}>
                      <div className="notify-group-label">{t(group.label)}</div>
                      {group.events.map((event) => {
                        const alert = alertOf(event)
                        const open = !!alert && openEvents.includes(event)
                        const path = alert ? settings[alert.pathKey] : ''
                        return (
                          <div className={`notify-event${open ? ' open' : ''}`} key={event}>
                            <div className="notify-row" role="row">
                              <span role="rowheader" className="notify-name">
                                {alert ? (
                                  <button
                                    type="button"
                                    className="notify-expand"
                                    aria-expanded={open}
                                    onClick={() => toggleEvent(event)}
                                  >
                                    <ChevronRight />
                                    {eventLabel(event)}
                                  </button>
                                ) : (
                                  <span className="notify-plain">{eventLabel(event)}</span>
                                )}
                              </span>
                              {alert ? (
                                <Switch
                                  aria-label={`${eventLabel(event)}: ${t('notifySound')}`}
                                  checked={settings[alert.enabledKey]}
                                  disabled={!settings.soundsEnabled}
                                  onCheckedChange={(checked) =>
                                    patch({ [alert.enabledKey]: checked } as Partial<Settings>)
                                  }
                                />
                              ) : (
                                <span className="notify-none" aria-label={t('notifyNoSound')}>
                                  —
                                </span>
                              )}
                              <Switch
                                aria-label={`${eventLabel(event)}: ${t('notifyToast')}`}
                                checked={!toastsOff.includes(event)}
                                onCheckedChange={(on) => setToast(event, on)}
                              />
                              <Switch
                                aria-label={`${eventLabel(event)}: ${t('notifyDesktop')}`}
                                checked={desktopOn.includes(event)}
                                onCheckedChange={(on) => setDesktop(event, on)}
                              />
                            </div>
                            {open && alert && (
                              <div className="notify-detail sound-setting">
                                <div className="sound-voice-row">
                                  <Select
                                    value={
                                      choiceOf(alert.kind, path) === 'beep' ? '__beep' : choiceOf(alert.kind, path)
                                    }
                                    onValueChange={(value) => setChoiceOf(alert.kind, alert.pathKey, value)}
                                  >
                                    <SelectTrigger
                                      aria-label={`${alert.label}: ${t('soundVoiceOf')}`}
                                      className="sound-voice"
                                    >
                                      <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                      <SelectItem value="__default">
                                        {t('soundVoiceDefault')}（{voice ? voiceName(voice) : t('soundVoiceBeep')}）
                                      </SelectItem>
                                      {voicePacks.map((pack) => (
                                        <SelectItem key={pack.id} value={pack.id}>
                                          {voiceName(pack)}
                                        </SelectItem>
                                      ))}
                                      <SelectItem value="__beep">{t('soundVoiceBeep')}</SelectItem>
                                      <SelectItem value="custom">{t('soundVoiceCustom')}</SelectItem>
                                    </SelectContent>
                                  </Select>
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant="ghost"
                                    onClick={() => void previewSound(alert.kind, path)}
                                  >
                                    <Play />
                                    {t('previewSound')}
                                  </Button>
                                </div>
                                <div className="volume-offset">
                                  <span className="volume-offset-label">{t('soundVolumeOffset')}</span>
                                  <input
                                    className="range"
                                    type="range"
                                    min="-50"
                                    max="50"
                                    step="1"
                                    aria-label={`${alert.label}: ${t('soundVolumeOffset')}`}
                                    value={offsetOf(alert.kind)}
                                    onChange={(e) => setOffsetOf(alert.kind, Number(e.target.value))}
                                  />
                                  <button
                                    type="button"
                                    className="volume-offset-value"
                                    title={t('soundVolumeReset')}
                                    disabled={offsetOf(alert.kind) === 0}
                                    onClick={() => setOffsetOf(alert.kind, 0)}
                                  >
                                    {offsetLabel(offsetOf(alert.kind))}
                                  </button>
                                  <span className="volume-offset-actual" title={t('soundVolumeActual')}>
                                    → {volumeOf(alert.kind)}%
                                  </span>
                                </div>
                                <div className="volume-offset">
                                  <span className="volume-offset-label">{t('soundDelay')}</span>
                                  <input
                                    className="range"
                                    type="range"
                                    min="0"
                                    max={SOUND_DELAY_MAX}
                                    step="1"
                                    aria-label={`${alert.label}: ${t('soundDelay')}`}
                                    value={delayOf(alert.kind)}
                                    onChange={(e) => setDelayOf(alert.kind, Number(e.target.value))}
                                  />
                                  <button
                                    type="button"
                                    className="volume-offset-value"
                                    title={t('soundDelayReset')}
                                    disabled={delayOf(alert.kind) === 0}
                                    onClick={() => setDelayOf(alert.kind, 0)}
                                  >
                                    {t('soundDelaySeconds').replace('{n}', String(delayOf(alert.kind)))}
                                  </button>
                                </div>
                                {choiceOf(alert.kind, path) === 'custom' && (
                                  <div className="sound-file-row">
                                    <Button
                                      type="button"
                                      size="sm"
                                      variant="secondary"
                                      onClick={() => void chooseSound(alert.pathKey)}
                                    >
                                      <FolderOpen />
                                      {t('chooseSound')}
                                    </Button>
                                    <span className="sound-file-name" title={path || t('noSoundFile')}>
                                      {path ? soundFileName(path) : t('noSoundFile')}
                                    </span>
                                  </div>
                                )}
                                <p className="sound-said" title={saidBy(alert.kind, path)}>
                                  {saidBy(alert.kind, path)}
                                </p>
                                {alert.kind === 'runThrough' && settings.runThroughSoundEnabled && (
                                  <div className="field">
                                    <Label>{t('runThroughTime')}</Label>
                                    <div className="time-fields">
                                      <div>
                                        <Input
                                          aria-label={t('minutes')}
                                          type="number"
                                          min="0"
                                          max="59"
                                          value={Math.floor(settings.runThroughSeconds / 60)}
                                          onChange={(e) =>
                                            patch({
                                              runThroughSeconds: Math.max(
                                                1,
                                                Math.min(
                                                  3599,
                                                  Number(e.target.value) * 60 + (settings.runThroughSeconds % 60),
                                                ),
                                              ),
                                            })
                                          }
                                        />
                                        <span>{t('minutes')}</span>
                                      </div>
                                      <div>
                                        <Input
                                          aria-label={t('seconds')}
                                          type="number"
                                          min="0"
                                          max="59"
                                          value={settings.runThroughSeconds % 60}
                                          onChange={(e) =>
                                            patch({
                                              runThroughSeconds: Math.max(
                                                1,
                                                Math.min(
                                                  3599,
                                                  Math.floor(settings.runThroughSeconds / 60) * 60 +
                                                    Number(e.target.value),
                                                ),
                                              ),
                                            })
                                          }
                                        />
                                        <span>{t('seconds')}</span>
                                      </div>
                                    </div>
                                    <p className="help">{t('runThroughTimeHelp')}</p>
                                  </div>
                                )}
                              </div>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  ))}
                </div>
                <p className="help">{t('notifyEventsHelp')}</p>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="startup">
          <div className="settings-stack">
            <Card>
              <CardHeader>
                <div className="icon-title">
                  <MonitorCog />
                  <div>
                    <CardTitle>{t('startupTitle')}</CardTitle>
                    <CardDescription>{t('startupDescription')}</CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="switch-stack">
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="launch-at-startup">{t('launchAtStartup')}</Label>
                      <p className="help">{t('launchAtStartupHelp')}</p>
                    </div>
                    <Switch
                      id="launch-at-startup"
                      checked={settings.launchAtStartup}
                      onCheckedChange={(launchAtStartup) => patch({ launchAtStartup })}
                    />
                  </div>
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="start-minimized">{t('startMinimized')}</Label>
                      <p className="help">{t('startMinimizedHelp')}</p>
                    </div>
                    <Switch
                      id="start-minimized"
                      checked={settings.startMinimized}
                      onCheckedChange={(startMinimized) => patch({ startMinimized })}
                    />
                  </div>
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="auto-monitoring">{t('autoStartMonitoring')}</Label>
                      <p className="help">{t('autoStartMonitoringHelp')}</p>
                    </div>
                    <Switch
                      id="auto-monitoring"
                      checked={settings.autoStartMonitoring}
                      onCheckedChange={(autoStartMonitoring) => patch({ autoStartMonitoring })}
                    />
                  </div>
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="minimize-to-tray">{t('minimizeToTray')}</Label>
                      <p className="help">{t('minimizeToTrayHelp')}</p>
                    </div>
                    <Switch
                      id="minimize-to-tray"
                      checked={settings.minimizeToTray}
                      onCheckedChange={(minimizeToTray) => patch({ minimizeToTray })}
                    />
                  </div>
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="close-to-tray">{t('closeToTray')}</Label>
                      <p className="help">{t('closeToTrayHelp')}</p>
                    </div>
                    <Switch
                      id="close-to-tray"
                      checked={settings.closeToTray}
                      onCheckedChange={(closeToTray) => patch({ closeToTray })}
                    />
                  </div>
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="keep-priority">{t('keepPriority')}</Label>
                      <p className="help">{t('keepPriorityHelp')}</p>
                    </div>
                    <Switch
                      id="keep-priority"
                      checked={settings.keepPriority}
                      onCheckedChange={(keepPriority) => patch({ keepPriority })}
                    />
                  </div>
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="auto-update">{t('autoUpdate')}</Label>
                      <p className="help">{t('autoUpdateHelp')}</p>
                    </div>
                    <Switch
                      id="auto-update"
                      checked={settings.autoUpdate}
                      onCheckedChange={(autoUpdate) => patch({ autoUpdate })}
                    />
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="status">
          <div className="status-grid">
            <Metric
              icon={<Wifi />}
              label={t('remoteConnection')}
              value={
                status.connection === 'connected'
                  ? t('connected')
                  : status.connection === 'disconnected'
                    ? t('disconnected')
                    : status.connection
              }
            />
            <Metric icon={<MapPinned />} label={t('currentMap')} value={status.currentMap || t('notDetected')} />
            <Metric icon={<Activity />} label={t('raidState')} value={status.raidActive ? t('raidIn') : t('raidOut')} />
            <Metric icon={<ScanLine />} label={t('screenshot')} value={status.screenshotType || t('unknown')} />

            <Card className="wide tracker-status">
              <CardHeader>
                <div className="card-title-actions">
                  <div>
                    <CardTitle>{t('trackerTitle')}</CardTitle>
                    <CardDescription>{trackerConnectionLabel}</CardDescription>
                  </div>
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={busy || !settings.tarkovTrackerEnabled || !status.tracker.mode}
                    onClick={() => run(() => RefreshTracker(), t('trackerRefreshed'))}
                  >
                    <RefreshCw />
                    {t('trackerRefresh')}
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <dl>
                  <dt>{t('trackerCurrentMode')}</dt>
                  <dd>{status.tracker.mode || '—'}</dd>
                  <dt>{t('trackerProfile')}</dt>
                  <dd>{status.tracker.displayName || status.tracker.profileId || '—'}</dd>
                  <dt>{t('trackerLevel')}</dt>
                  <dd>{status.tracker.playerLevel || '—'}</dd>
                  <dt>{t('trackerCompleted')}</dt>
                  <dd>{status.tracker.completedTasks}</dd>
                  <dt>{t('trackerFailed')}</dt>
                  <dd>{status.tracker.failedTasks}</dd>
                  <dt>{t('trackerLastEvent')}</dt>
                  <dd>{status.tracker.lastEvent || '—'}</dd>
                  <dt>{t('trackerLastSync')}</dt>
                  <dd>
                    {status.tracker.lastSync
                      ? new Date(status.tracker.lastSync).toLocaleString(
                          settings.language === 'ja' ? 'ja-JP' : 'en-US',
                          { hour12: hour12() },
                        )
                      : '—'}
                  </dd>
                </dl>
                {status.tracker.lastError && <p className="error">{status.tracker.lastError}</p>}
              </CardContent>
            </Card>
            <Card className="wide">
              <CardHeader>
                <CardTitle>{t('lastDetection')}</CardTitle>
              </CardHeader>
              <CardContent>
                <Button type="button" variant="secondary" onClick={analyzeLatest} disabled={busy}>
                  <RefreshCw />
                  {t('analyzeLatest')}
                </Button>
                <dl>
                  <dt>{t('processState')}</dt>
                  <dd>{translateAnalysisStage(settings.language, status.analysisStage)}</dd>
                  <dt>{t('screenLayout')}</dt>
                  <dd>{status.detectionLayout || '—'}</dd>
                  <dt>{t('remoteSend')}</dt>
                  <dd>{status.lastRemoteCommand || '—'}</dd>
                  <dt>{t('file')}</dt>
                  <dd>{status.lastScreenshot || t('waiting')}</dd>
                  {status.screenshotType === 'position' && status.position && (
                    <>
                      <dt>{t('position')}</dt>
                      <dd>
                        {status.position.x.toFixed(2)}, {status.position.y.toFixed(2)}, {status.position.z.toFixed(2)} /{' '}
                        {status.position.rotation.toFixed(1)}°
                      </dd>
                    </>
                  )}
                  {status.screenshotType === 'item' ? (
                    <>
                      <dt>{t('item')}</dt>
                      <dd>
                        {status.lastItem || '—'}{' '}
                        {status.itemConfidence > 0 && `(${Math.round(status.itemConfidence * 100)}%)`}
                      </dd>
                    </>
                  ) : (
                    <>
                      <dt>{t('quest')}</dt>
                      <dd>
                        {status.lastQuest || '—'}{' '}
                        {status.matchConfidence > 0 && `(${Math.round(status.matchConfidence * 100)}%)`}
                      </dd>
                      {status.lastQuest && (
                        <>
                          <dt>{t('questSiteTitle')}</dt>
                          <dd>
                            <Button type="button" variant="secondary" onClick={() => void openFolder(OpenQuestPage)}>
                              <ExternalLink />
                              {t('openQuestPage')}
                            </Button>
                          </dd>
                        </>
                      )}
                    </>
                  )}
                  <dt>{t('ocrRaw')}</dt>
                  <dd>{status.ocrRaw || '—'}</dd>
                </dl>
                {status.lastError && <p className="error">{status.lastError}</p>}
              </CardContent>
            </Card>
            <Card className="wide">
              <CardHeader>
                <div className="card-title-actions">
                  <div>
                    <CardTitle>{t('catalogTitle')}</CardTitle>
                    <CardDescription>{t('catalogHelp')}</CardDescription>
                  </div>
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={busy || status.catalog?.state === 'loading'}
                    onClick={() => run(() => RefreshCatalog(), t('catalogRefreshed'))}
                  >
                    <RefreshCw />
                    {t('catalogRefresh')}
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <dl>
                  <dt>{t('gameMode')}</dt>
                  <dd>{status.catalog?.mode || t('notDetected')}</dd>
                  <dt>{t('processState')}</dt>
                  <dd>
                    {status.catalog?.state === 'loading'
                      ? t('processing')
                      : status.catalog?.state === 'stale'
                        ? t('catalogStale')
                        : status.catalog?.state === 'ready'
                          ? t('catalogReady')
                          : t('waiting')}
                  </dd>
                  <dt>{t('catalogItems')}</dt>
                  <dd>{status.catalog?.items || 0}</dd>
                  <dt>{t('catalogMaps')}</dt>
                  <dd>{status.catalog?.maps || 0}</dd>
                  <dt>{t('catalogTraders')}</dt>
                  <dd>{status.catalog?.traders || 0}</dd>
                  <dt>{t('catalogTasks')}</dt>
                  <dd>{status.catalog?.tasks || 0}</dd>
                  <dt>{t('catalogHideout')}</dt>
                  <dd>{status.catalog?.hideoutStations || 0}</dd>
                  <dt>{t('catalogUpdated')}</dt>
                  <dd>
                    {status.catalog?.updatedAt
                      ? new Date(status.catalog.updatedAt).toLocaleString(
                          settings.language === 'ja' ? 'ja-JP' : 'en-US',
                          { hour12: hour12() },
                        )
                      : '—'}
                  </dd>
                </dl>
                {status.catalog?.lastError && <p className="error">{status.catalog.lastError}</p>}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="logs">
          <LogsSection
            settings={settings}
            status={status}
            t={t}
            busy={busy}
            run={run}
            trackerModeLabel={trackerModeLabel}
            logs={logs}
            openFolder={openFolder}
            raidElapsed={raidElapsed}
            runThroughLabel={runThroughLabel}
            logLevel={logLevel}
            setLogLevel={setLogLevel}
            logCategory={logCategory}
            setLogCategory={setLogCategory}
            logQuery={logQuery}
            setLogQuery={setLogQuery}
          />
        </TabsContent>
        <TabsContent value="debug">
          <div className="settings-stack">
            <Card className="debug-settings-card">
              <CardHeader>
                <div className="icon-title">
                  <Bug />
                  <CardTitle>{t('debug')}</CardTitle>
                </div>
              </CardHeader>
              <CardContent>
                <div className="switch-stack">
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="debug-mode">{t('debugMode')}</Label>
                      <p className="help">{t('debugHelp')}</p>
                    </div>
                    <Switch id="debug-mode" checked={settings.debug} onCheckedChange={(debug) => patch({ debug })} />
                  </div>
                  <div className="switch-row">
                    <div>
                      <Label htmlFor="save-recognition-debug">{t('saveRecognitionDebug')}</Label>
                      <p className="help">{t('saveRecognitionDebugHelp')}</p>
                    </div>
                    <Switch
                      id="save-recognition-debug"
                      checked={settings.saveRecognitionDebug}
                      onCheckedChange={(saveRecognitionDebug) => patch({ saveRecognitionDebug })}
                    />
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
          {settings.debug && (
            <div className="debug-grid">
              <Card className="wide">
                <CardHeader>
                  <CardTitle>{t('hideoutLogCategory')}</CardTitle>
                  <CardDescription>{t('hideoutDiagnosticsHelp')}</CardDescription>
                </CardHeader>
                <CardContent>
                  <Button type="button" variant="secondary" onClick={() => void openFolder(OpenHideoutDiagnostics)}>
                    <FolderOpen />
                    {t('hideoutDiagnostics')}
                  </Button>
                  {status.hideout?.lastError && <p className="error">{t('hideoutSaveFailed')}</p>}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle>{status.screenshotType === 'item' ? t('itemCandidates') : t('candidates')}</CardTitle>
                </CardHeader>
                <CardContent>
                  {status.screenshotType === 'item' ? (
                    <>
                      <ol className="candidates">
                        {status.itemCandidates.map((item) => (
                          <li key={item.id}>
                            <span>
                              <b>{item.name}</b>
                              <small>{item.shortName}</small>
                            </span>
                            <Badge>{Math.round(item.confidence * 100)}%</Badge>
                          </li>
                        ))}
                      </ol>
                      {status.itemUrl && (
                        <Button variant="secondary" onClick={() => BrowserOpenURL(status.itemUrl)}>
                          <ExternalLink />
                          {t('openItemTarkovDev')}
                        </Button>
                      )}
                    </>
                  ) : (
                    <>
                      <ol className="candidates">
                        {status.questCandidates.map((q) => (
                          <li key={q.id}>
                            <span>
                              <b>{q.name}</b>
                              <small>
                                {q.trader} · {q.map || t('anyMap')}
                              </small>
                            </span>
                            <Badge>{Math.round(q.confidence * 100)}%</Badge>
                          </li>
                        ))}
                      </ol>
                      {status.questUrl && (
                        <Button variant="secondary" onClick={() => void openFolder(OpenQuestPage)}>
                          <ExternalLink />
                          {t('openQuestPage')}
                        </Button>
                      )}
                    </>
                  )}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle>{status.screenshotType === 'item' ? t('itemCropTitle') : t('cropTitle')}</CardTitle>
                </CardHeader>
                <CardContent>
                  {status.cropPreview ? (
                    <img
                      className="crop"
                      src={status.cropPreview}
                      alt={status.screenshotType === 'item' ? t('itemCropTitle') : t('cropTitle')}
                    />
                  ) : (
                    <p className="empty">{t('noImage')}</p>
                  )}
                </CardContent>
              </Card>
              {status.screenshotType !== 'item' && (
                <Card className="wide">
                  <CardHeader>
                    <CardTitle>{t('objectives')}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <ul className="objectives">
                      {status.questObjectives.map((o, i) => (
                        <li key={i}>
                          {o.description}
                          <small>{o.maps.join(', ')}</small>
                        </li>
                      ))}
                    </ul>
                  </CardContent>
                </Card>
              )}
            </div>
          )}
        </TabsContent>
      </Tabs>
      {toasts}
    </main>
  )
}

createRoot(document.getElementById('root')!).render(<App />)
