import { Language, translate } from './i18n'
import { HideoutStatus, HideoutEvent } from './Hideout'

// The settings page's model: the shapes the Go side sends, their defaults
// and normalizers, the sections and the marker gallery.
export type RemoteTarget={id:string;name:string;map:boolean;tasks:boolean}
export type Settings = { gameLanguage:string; questSite:string; hideoutErrorNotifications:boolean;hideoutErrorSoundPath:string; language:Language; screenshotDirectory:string; logsDirectory:string; remoteId:string; remoteTargets:RemoteTarget[]; browserRemoteId?:string; map:string; gameMode:string; ocrEngine:string; tesseractPath:string; debug:boolean; saveRecognitionDebug:boolean; screenshotCleanup:boolean; screenshotRetainCount:number; screenshotRetainHours:number; soundsEnabled:boolean; questSoundEnabled:boolean; questSoundPath:string; errorSoundEnabled:boolean; errorSoundPath:string; soundVolume:number; autoStartMonitoring:boolean; openMapOnRaidStart:boolean; navigateMapOnPositionScreenshot:boolean; playerMarkerEffect:string; playerMarkerColor:string; tarkovTrackerEnabled:boolean; matchFoundSoundEnabled:boolean; matchFoundSoundPath:string; raidStartSoundEnabled:boolean; raidStartSoundPath:string; runThroughSoundEnabled:boolean; runThroughSoundPath:string; runThroughSeconds:number; questItemsSoundEnabled:boolean; questItemsSoundPath:string; restartTasksSoundEnabled:boolean; restartTasksSoundPath:string; startMinimized:boolean; minimizeToTray:boolean;closeToTray:boolean; keepPriority:boolean; launchAtStartup:boolean; autoUpdate:boolean; updateChannel:string }
export type Candidate = { id:string; name:string; trader:string; map:string; confidence:number }
export type ItemCandidate = { id:string; name:string; shortName:string; url:string; iconUrl:string; confidence:number }
export type Objective = { description:string; maps:string[] }
export type TrackerKey={id:string;name:string;mode:string;maskedToken:string;accountId:string;profileId:string;bound:boolean}
export type TrackerProfile={accountId:string;profileId:string;mode:string;firstSeen:string;lastSeen:string;boundKeyId:string;current:boolean}
export type CatalogStatus={state:string;mode:string;updatedAt:string;items:number;maps:number;traders:number;tasks:number;hideoutStations:number;scavCooldownSeconds:number;playerLevels:number;lastError:string}
export type TrackerStatus={connection:string;mode:string;profileId:string;accountId:string;displayName:string;playerLevel:number;completedTasks:number;failedTasks:number;pvpConfigured:boolean;pveConfigured:boolean;seasonalConfigured:boolean;lastSync:string;lastEvent:string;lastError:string;keys:TrackerKey[];profiles:TrackerProfile[]}
export type Status = { hideout?:HideoutStatus; catalog?:CatalogStatus; connection:string; monitoring:boolean; currentMap:string; raidActive:boolean; raidStartedAt:string; runThroughAt:string; lastQueueSeconds:number; lastScreenshot:string; screenshotType:string; position?:{x:number;y:number;z:number;rotation:number;detectedAt:string}|null;lastError:string;detectionScore:number;detectionLayout:string;analysisStage:string;ocrRaw:string;lastQuest:string;questTrader:string;questMap:string;matchConfidence:number;questCandidates:Candidate[];cropPreview:string;questUrl:string;questObjectives:Objective[];lastItem:string;itemId:string;itemShortName:string;itemUrl:string;itemIconUrl:string;itemConfidence:number;itemCandidates:ItemCandidate[];lastRemoteCommand:string;tracker:TrackerStatus }
export type LogEntry = { hideout?:HideoutEvent; id:number|string; timestamp:string; level:'Error'|'Warn'|'Info'|'Debug'; category:string; message:string }
export type UpdateStatus={current:string;latest:string;state:string;progress:number;platform:string;releaseUrl:string;releaseName:string;notes:string;publishedAt:string;checkedAt:string;lastError:string}

export const defaults:Settings={gameLanguage:"auto",questSite:"tarkov-dev",hideoutErrorNotifications:false,hideoutErrorSoundPath:"",language:'ja',screenshotDirectory:'',logsDirectory:'',remoteId:'',remoteTargets:[],map:'',gameMode:'auto',ocrEngine:'tesseract',tesseractPath:'',debug:false,saveRecognitionDebug:false,screenshotCleanup:false,screenshotRetainCount:500,screenshotRetainHours:168,soundsEnabled:true,questSoundEnabled:true,questSoundPath:'',errorSoundEnabled:true,errorSoundPath:'',soundVolume:28,autoStartMonitoring:true,openMapOnRaidStart:true,navigateMapOnPositionScreenshot:true,playerMarkerEffect:'none',playerMarkerColor:'',tarkovTrackerEnabled:false,matchFoundSoundEnabled:false,matchFoundSoundPath:'',raidStartSoundEnabled:false,raidStartSoundPath:'',runThroughSoundEnabled:false,runThroughSoundPath:'',runThroughSeconds:430,questItemsSoundEnabled:false,questItemsSoundPath:'',restartTasksSoundEnabled:false,restartTasksSoundPath:'',startMinimized:false,minimizeToTray:false,closeToTray:false,keepPriority:false,launchAtStartup:false,autoUpdate:true,updateChannel:'stable'}
export const emptyUpdate:UpdateStatus={current:"",latest:"",state:"idle",progress:0,platform:"",releaseUrl:"",releaseName:"",notes:"",publishedAt:"",checkedAt:"",lastError:""}
export const emptyTracker:TrackerStatus={connection:'disabled',mode:'',profileId:'',accountId:'',displayName:'',playerLevel:0,completedTasks:0,failedTasks:0,pvpConfigured:false,pveConfigured:false,seasonalConfigured:false,lastSync:'',lastEvent:'',lastError:'',keys:[],profiles:[]}
export const emptyStatus:Status={connection:'disconnected',monitoring:false,currentMap:'',raidActive:false,raidStartedAt:'',runThroughAt:'',lastQueueSeconds:0,lastScreenshot:'',screenshotType:'unknown',lastError:'',detectionScore:0,detectionLayout:'',analysisStage:'待機中',ocrRaw:'',lastQuest:'',questTrader:'',questMap:'',matchConfidence:0,questCandidates:[],cropPreview:'',questUrl:'',questObjectives:[],lastItem:'',itemId:'',itemShortName:'',itemUrl:'',itemIconUrl:'',itemConfidence:0,itemCandidates:[],lastRemoteCommand:'',tracker:emptyTracker}
export const maps=['customs','factory','night-factory','ground-zero','ground-zero-21','interchange','icebreaker','the-lab','the-lab-dark','the-labyrinth','lighthouse','reserve','shoreline','streets-of-tarkov','terminal','woods']
// The player marker effects (app_marker.go), in the gallery's order, and
// the colour each has until one is chosen.
export const markerEffects=['none','outline','glow','pulse','beacon'] as const
export const markerEffectNames:Record<string,Parameters<typeof translate>[1]>={none:'markerEffectNone',outline:'markerOutline',glow:'markerGlow',pulse:'markerPulse',beacon:'markerBeacon'}
export const markerOwnColor=(effect:string)=>effect==='outline'?'#ffffff':'#ff3b30'

export function normalizeStatus(value:Partial<Status>|null|undefined):Status{
  const next={...emptyStatus,...(value??{})}
  return {
    ...next,
    tracker:{...emptyTracker,...(next.tracker??{}),keys:Array.isArray(next.tracker?.keys)?next.tracker.keys:[],profiles:Array.isArray(next.tracker?.profiles)?next.tracker.profiles:[]},
    questCandidates:Array.isArray(next.questCandidates)?next.questCandidates:[],
    itemCandidates:Array.isArray(next.itemCandidates)?next.itemCandidates:[],
    questObjectives:Array.isArray(next.questObjectives)
      ?next.questObjectives.map(objective=>({...objective,maps:Array.isArray(objective.maps)?objective.maps:[]}))
      :[],
  }
}

// The browser shell embeds this page and picks the section in the URL hash.
export const hostSections=['status','logs','folders','recognition','remote','tracker','sounds','startup','debug'] as const
export type HostSection=typeof hostSections[number]
// Times of day follow the browser's time format (set on the frame by the
// browser shell): 24-hour unless it is 12.
export const hour12=()=>document.documentElement.dataset.clock==='12'
export const hashSection=():HostSection=>{const hash=location.hash.slice(1) as HostSection;return hostSections.includes(hash)?hash:'status'}
export const sectionTitles:Record<HostSection,Parameters<typeof translate>[1]>={status:'secStatus',logs:'secLogs',folders:'secFolders',recognition:'secRecognition',remote:'secRemote',tracker:'secTracker',sounds:'secSounds',startup:'secStartup',debug:'secDebug'}

// Names of the game languages, in their own language.
export const languageNames:Record<string,string>={ja:'日本語',en:'English'}
