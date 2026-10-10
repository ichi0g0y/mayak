import type * as AppService from '../bindings/github.com/local/mayak/internal/app/app'

// Only the bundled settings document reads this same-origin parent bridge.
// The runtime and event dispatcher are instantiated once, in the trusted shell.
export type DesktopBridge = {
  backend: typeof AppService
  on: (name: string, callback: (...args: any[]) => void) => () => void
  openURL: (url: string) => void
}

export const desktop = (window.parent as Window & { mayakDesktop: DesktopBridge }).mayakDesktop
export const EventsOn = desktop.on
export const BrowserOpenURL = desktop.openURL
export const {
  AnalyzeLatestScreenshot,
  AutoDetectEFTDirectories,
  AutoDetectRemoteID,
  CheckForUpdates,
  ChooseLogsDirectory,
  ChooseScreenshotDirectory,
  ChooseSoundFile,
  ClearLogs,
  DiscoverTrackerProfiles,
  DownloadUpdate,
  EFTScreenshotKey,
  GameLanguages,
  GetLogs,
  GetSettings,
  GetStatus,
  GetUpdateStatus,
  ImportTrackerToken,
  InstallUpdate,
  OpenDebugDirectory,
  OpenLogsDirectory,
  OpenScreenshotDirectory,
  PersistSettings,
  PreviewSound,
  RefreshCatalog,
  OpenQuestPage,
  OpenHideoutDiagnostics,
  RefreshTracker,
  RemoveTrackerKey,
  RefreshTrackerKeyNames,
  SaveSettings,
  ConfirmTrackerPrestigeReset,
  SetTrackerHistoryFrom,
  SetTrackerProfileKey,
  StartMonitoring,
  StopMonitoring,
  SyncTrackerProfileHistory,
  TestRemote,
  VoicePacks,
} = desktop.backend
