import { Language, translate } from './i18n'
import { HideoutStatus, HideoutEvent } from './Hideout'

// The settings page's model: the shapes the Go side sends, their defaults
// and normalizers, the sections and the marker gallery.
export type RemoteTarget = { id: string; name: string; map: boolean; tasks: boolean }
export type Settings = {
  gameLanguage: string
  questSite: string
  // The notification events (app_notify.go notifyEvents) not shown as toasts,
  // and those also shown as desktop notifications of the OS.
  toastsOff?: string[]
  desktopOn?: string[]
  language: Language
  screenshotDirectory: string
  logsDirectory: string
  remoteId: string
  remoteTargets: RemoteTarget[]
  browserRemoteId?: string
  map: string
  gameMode: string
  ocrEngine: string
  tesseractPath: string
  debug: boolean
  saveRecognitionDebug: boolean
  screenshotCleanup: boolean
  screenshotRetainCount: number
  screenshotRetainHours: number
  soundsEnabled: boolean
  questSoundEnabled: boolean
  questSoundPath: string
  errorSoundEnabled: boolean
  errorSoundPath: string
  taskNotMatchedSoundEnabled: boolean
  taskNotMatchedSoundPath: string
  remoteErrorSoundEnabled: boolean
  remoteErrorSoundPath: string
  itemSoundEnabled: boolean
  itemSoundPath: string
  itemNotMatchedSoundEnabled: boolean
  itemNotMatchedSoundPath: string
  soundVolume: number
  soundVoice: string
  soundVoices: Record<string, string>
  soundVolumeOffsets: Record<string, number>
  soundDelays: Record<string, number>
  autoStartMonitoring: boolean
  openMapOnRaidStart: boolean
  navigateMapOnPositionScreenshot: boolean
  tarkovTrackerEnabled: boolean
  matchFoundSoundEnabled: boolean
  matchFoundSoundPath: string
  raidStartSoundEnabled: boolean
  raidStartSoundPath: string
  runThroughSoundEnabled: boolean
  runThroughSoundPath: string
  runThroughSeconds: number
  gameStartSoundEnabled: boolean
  gameStartSoundPath: string
  questItemsSoundEnabled: boolean
  questItemsSoundPath: string
  taskFailedSoundEnabled: boolean
  taskFailedSoundPath: string
  gameExitSoundEnabled: boolean
  gameExitSoundPath: string
  startMinimized: boolean
  minimizeToTray: boolean
  closeToTray: boolean
  keepPriority: boolean
  launchAtStartup: boolean
  autoUpdate: boolean
  updateChannel: string
}
export type Candidate = { id: string; name: string; trader: string; map: string; confidence: number }
export type ItemCandidate = {
  id: string
  name: string
  shortName: string
  url: string
  iconUrl: string
  confidence: number
}
export type Objective = { description: string; maps: string[] }
export type TrackerKey = {
  id: string
  name: string
  mode: string
  maskedToken: string
  accountId: string
  profileId: string
  bound: boolean
}
export type TrackerProfile = {
  accountId: string
  profileId: string
  mode: string
  firstSeen: string
  lastSeen: string
  boundKeyId: string
  current: boolean
  historySyncedAt?: string
  // The day its past logs are read from (YYYY-MM-DD; after a Prestige), "" for all.
  historyFrom?: string
  // Its last Prestige seen in the EFT logs, and whether TarkovTracker still
  // waits for its reset.
  prestigeAt?: string
  prestigePending?: boolean
}
export type CatalogStatus = {
  state: string
  mode: string
  updatedAt: string
  items: number
  maps: number
  traders: number
  tasks: number
  hideoutStations: number
  scavCooldownSeconds: number
  playerLevels: number
  lastError: string
}
export type TrackerStatus = {
  connection: string
  mode: string
  profileId: string
  accountId: string
  displayName: string
  playerLevel: number
  completedTasks: number
  failedTasks: number
  pvpConfigured: boolean
  pveConfigured: boolean
  seasonalConfigured: boolean
  gameRunning?: boolean
  lastSync: string
  lastEvent: string
  lastError: string
  keys: TrackerKey[]
  profiles: TrackerProfile[]
}
export type Status = {
  hideout?: HideoutStatus
  catalog?: CatalogStatus
  connection: string
  monitoring: boolean
  currentMap: string
  raidActive: boolean
  raidStartedAt: string
  runThroughAt: string
  lastQueueSeconds: number
  lastScreenshot: string
  screenshotType: string
  position?: { x: number; y: number; z: number; rotation: number; detectedAt: string } | null
  lastError: string
  detectionScore: number
  detectionLayout: string
  analysisStage: string
  ocrRaw: string
  lastQuest: string
  questTrader: string
  questMap: string
  matchConfidence: number
  questCandidates: Candidate[]
  cropPreview: string
  questUrl: string
  questObjectives: Objective[]
  lastItem: string
  itemId: string
  itemShortName: string
  itemUrl: string
  itemIconUrl: string
  itemConfidence: number
  itemCandidates: ItemCandidate[]
  lastRemoteCommand: string
  tracker: TrackerStatus
}
export type LogEntry = {
  hideout?: HideoutEvent
  id: number | string
  timestamp: string
  level: 'Error' | 'Warn' | 'Info' | 'Debug'
  category: string
  message: string
}
export type UpdateStatus = {
  current: string
  latest: string
  state: string
  progress: number
  platform: string
  releaseUrl: string
  releaseName: string
  notes: string
  publishedAt: string
  checkedAt: string
  lastError: string
}

// A notification's delay (seconds, up to SOUND_DELAY_MAX) when none is set:
// back from a raid and a task failed come as the game makes its own sounds
// (internal/config/config.go soundDelayDefaults).
export const SOUND_DELAY_MAX = 15
export const SOUND_DELAY_DEFAULTS: Record<string, number> = { questItems: 3, taskFailed: 3 }

export const defaults: Settings = {
  gameLanguage: 'auto',
  questSite: 'official-wiki',
  language: 'ja',
  screenshotDirectory: '',
  logsDirectory: '',
  remoteId: '',
  remoteTargets: [],
  map: '',
  gameMode: 'auto',
  ocrEngine: 'tesseract',
  tesseractPath: '',
  debug: false,
  saveRecognitionDebug: false,
  screenshotCleanup: false,
  screenshotRetainCount: 500,
  screenshotRetainHours: 168,
  soundsEnabled: false,
  questSoundEnabled: true,
  questSoundPath: '',
  errorSoundEnabled: true,
  errorSoundPath: '',
  taskNotMatchedSoundEnabled: true,
  taskNotMatchedSoundPath: '',
  remoteErrorSoundEnabled: true,
  remoteErrorSoundPath: '',
  itemSoundEnabled: false,
  itemSoundPath: '',
  itemNotMatchedSoundEnabled: false,
  itemNotMatchedSoundPath: '',
  soundVolume: 28,
  soundVoice: '',
  soundVoices: {},
  soundVolumeOffsets: {},
  soundDelays: {},
  autoStartMonitoring: true,
  openMapOnRaidStart: true,
  navigateMapOnPositionScreenshot: true,
  tarkovTrackerEnabled: false,
  matchFoundSoundEnabled: true,
  matchFoundSoundPath: '',
  raidStartSoundEnabled: true,
  raidStartSoundPath: '',
  runThroughSoundEnabled: true,
  runThroughSoundPath: '',
  runThroughSeconds: 430,
  gameStartSoundEnabled: false,
  gameStartSoundPath: '',
  questItemsSoundEnabled: false,
  questItemsSoundPath: '',
  taskFailedSoundEnabled: false,
  taskFailedSoundPath: '',
  gameExitSoundEnabled: false,
  gameExitSoundPath: '',
  startMinimized: false,
  minimizeToTray: false,
  closeToTray: false,
  keepPriority: false,
  launchAtStartup: false,
  autoUpdate: true,
  updateChannel: 'stable',
}
export const emptyUpdate: UpdateStatus = {
  current: '',
  latest: '',
  state: 'idle',
  progress: 0,
  platform: '',
  releaseUrl: '',
  releaseName: '',
  notes: '',
  publishedAt: '',
  checkedAt: '',
  lastError: '',
}
export const emptyTracker: TrackerStatus = {
  connection: 'disabled',
  mode: '',
  profileId: '',
  accountId: '',
  displayName: '',
  playerLevel: 0,
  completedTasks: 0,
  failedTasks: 0,
  pvpConfigured: false,
  pveConfigured: false,
  seasonalConfigured: false,
  lastSync: '',
  lastEvent: '',
  lastError: '',
  keys: [],
  profiles: [],
}
export const emptyStatus: Status = {
  connection: 'disconnected',
  monitoring: false,
  currentMap: '',
  raidActive: false,
  raidStartedAt: '',
  runThroughAt: '',
  lastQueueSeconds: 0,
  lastScreenshot: '',
  screenshotType: 'unknown',
  lastError: '',
  detectionScore: 0,
  detectionLayout: '',
  analysisStage: '待機中',
  ocrRaw: '',
  lastQuest: '',
  questTrader: '',
  questMap: '',
  matchConfidence: 0,
  questCandidates: [],
  cropPreview: '',
  questUrl: '',
  questObjectives: [],
  lastItem: '',
  itemId: '',
  itemShortName: '',
  itemUrl: '',
  itemIconUrl: '',
  itemConfidence: 0,
  itemCandidates: [],
  lastRemoteCommand: '',
  tracker: emptyTracker,
}
export const maps = [
  'customs',
  'factory',
  'night-factory',
  'ground-zero',
  'ground-zero-21',
  'interchange',
  'icebreaker',
  'the-lab',
  'the-lab-dark',
  'the-labyrinth',
  'lighthouse',
  'reserve',
  'shoreline',
  'streets-of-tarkov',
  'terminal',
  'woods',
]
export function normalizeStatus(value: Partial<Status> | null | undefined): Status {
  const next = { ...emptyStatus, ...(value ?? {}) }
  return {
    ...next,
    tracker: {
      ...emptyTracker,
      ...(next.tracker ?? {}),
      keys: Array.isArray(next.tracker?.keys) ? next.tracker.keys : [],
      profiles: Array.isArray(next.tracker?.profiles) ? next.tracker.profiles : [],
    },
    questCandidates: Array.isArray(next.questCandidates) ? next.questCandidates : [],
    itemCandidates: Array.isArray(next.itemCandidates) ? next.itemCandidates : [],
    questObjectives: Array.isArray(next.questObjectives)
      ? next.questObjectives.map((objective) => ({
          ...objective,
          maps: Array.isArray(objective.maps) ? objective.maps : [],
        }))
      : [],
  }
}

// The browser shell embeds this page and picks the section in the URL hash.
export const hostSections = [
  'status',
  'logs',
  'folders',
  'recognition',
  'remote',
  'tracker',
  'sounds',
  'startup',
  'debug',
] as const
export type HostSection = (typeof hostSections)[number]
// Times of day follow the browser's time format (set on the frame by the
// browser shell): 24-hour unless it is 12.
export const hour12 = () => document.documentElement.dataset.clock === '12'
export const hashSection = (): HostSection => {
  const hash = location.hash.slice(1) as HostSection
  return hostSections.includes(hash) ? hash : 'status'
}
export const sectionTitles: Record<HostSection, Parameters<typeof translate>[1]> = {
  status: 'secStatus',
  logs: 'secLogs',
  folders: 'secFolders',
  recognition: 'secRecognition',
  remote: 'secRemote',
  tracker: 'secTracker',
  sounds: 'secSounds',
  startup: 'secStartup',
  debug: 'secDebug',
}

// Names of the game languages, in their own language.
export const languageNames: Record<string, string> = { ja: '日本語', en: 'English' }
