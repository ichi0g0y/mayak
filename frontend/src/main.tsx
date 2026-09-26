import React, { useEffect, useRef, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { Activity, Bug, Check, CircleX, Clock3, CloudSync, Download, ExternalLink, FolderOpen, History, KeyRound, MapPinned, MonitorCog, Play, Plus, Radio, RefreshCw, RotateCcw, ScanLine, Search, Trash2, Volume2, Wifi, X } from 'lucide-react'
import { BrowserOpenURL, EventsOn, desktop } from './desktop'
import { GameLanguages, AnalyzeLatestScreenshot, AutoDetectEFTDirectories, AutoDetectRemoteID, CheckForUpdates, DownloadUpdate, GetUpdateStatus, InstallUpdate, ChooseLogsDirectory, ChooseScreenshotDirectory, ChooseSoundFile, PlayerMarkerPreviewCSS, ClearLogs, DiscoverTrackerProfiles, GetLogs, GetSettings, GetStatus, GetTrackerHistoryBreakpoints, ImportTrackerToken, OpenDebugDirectory, OpenLogsDirectory, OpenScreenshotDirectory, PersistSettings, PreviewSound, RefreshCatalog, OpenQuestPage, OpenHideoutDiagnostics, RefreshTracker, RemoveTrackerKey, RefreshTrackerKeyNames, SaveSettings, SetTrackerProfileKey, SyncTrackerHistory, TestRemote } from './desktop'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Badge } from './components/ui/badge'
import { Switch } from './components/ui/switch'
import { Tabs, TabsContent } from './components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './components/ui/select'
import { Language, translate, translateAnalysisStage } from './i18n'
import './style.css'
import {HideoutStatus, HideoutEvent, hideoutMessage} from './Hideout'

type RemoteTarget={id:string;name:string;map:boolean;tasks:boolean}
type Settings = { gameLanguage:string; questSite:string; hideoutErrorNotifications:boolean;hideoutErrorSoundPath:string; language:Language; screenshotDirectory:string; logsDirectory:string; remoteId:string; remoteTargets:RemoteTarget[]; browserRemoteId?:string; map:string; gameMode:string; ocrEngine:string; tesseractPath:string; debug:boolean; saveRecognitionDebug:boolean; screenshotCleanup:boolean; screenshotRetainCount:number; screenshotRetainHours:number; soundsEnabled:boolean; questSoundEnabled:boolean; questSoundPath:string; errorSoundEnabled:boolean; errorSoundPath:string; soundVolume:number; autoStartMonitoring:boolean; openMapOnRaidStart:boolean; navigateMapOnPositionScreenshot:boolean; playerMarkerEffect:string; playerMarkerColor:string; tarkovTrackerEnabled:boolean; matchFoundSoundEnabled:boolean; matchFoundSoundPath:string; raidStartSoundEnabled:boolean; raidStartSoundPath:string; runThroughSoundEnabled:boolean; runThroughSoundPath:string; runThroughSeconds:number; questItemsSoundEnabled:boolean; questItemsSoundPath:string; restartTasksSoundEnabled:boolean; restartTasksSoundPath:string; startMinimized:boolean; minimizeToTray:boolean;closeToTray:boolean; keepPriority:boolean; launchAtStartup:boolean; autoUpdate:boolean }
type Candidate = { id:string; name:string; trader:string; map:string; confidence:number }
type ItemCandidate = { id:string; name:string; shortName:string; url:string; iconUrl:string; confidence:number }
type Objective = { description:string; maps:string[] }
type TrackerKey={id:string;name:string;mode:string;maskedToken:string;accountId:string;profileId:string;bound:boolean}
type TrackerProfile={accountId:string;profileId:string;mode:string;firstSeen:string;lastSeen:string;boundKeyId:string;current:boolean}
type TrackerHistoryBreakpoint={id:string;version:string;startAt:string}
type CatalogStatus={state:string;mode:string;updatedAt:string;items:number;maps:number;traders:number;tasks:number;hideoutStations:number;scavCooldownSeconds:number;playerLevels:number;lastError:string}
type TrackerStatus={connection:string;mode:string;profileId:string;accountId:string;displayName:string;playerLevel:number;completedTasks:number;failedTasks:number;pvpConfigured:boolean;pveConfigured:boolean;seasonalConfigured:boolean;lastSync:string;lastEvent:string;lastError:string;keys:TrackerKey[];profiles:TrackerProfile[]}
type Status = { hideout?:HideoutStatus; catalog?:CatalogStatus; connection:string; monitoring:boolean; currentMap:string; raidActive:boolean; raidStartedAt:string; runThroughAt:string; lastQueueSeconds:number; lastScreenshot:string; screenshotType:string; position?:{x:number;y:number;z:number;rotation:number;detectedAt:string}|null;lastError:string;detectionScore:number;detectionLayout:string;analysisStage:string;ocrRaw:string;lastQuest:string;questTrader:string;questMap:string;matchConfidence:number;questCandidates:Candidate[];cropPreview:string;questUrl:string;questObjectives:Objective[];lastItem:string;itemId:string;itemShortName:string;itemUrl:string;itemIconUrl:string;itemConfidence:number;itemCandidates:ItemCandidate[];lastRemoteCommand:string;tracker:TrackerStatus }
type LogEntry = { hideout?:HideoutEvent; id:number|string; timestamp:string; level:'Error'|'Warn'|'Info'|'Debug'; category:string; message:string }
type UpdateStatus={current:string;latest:string;state:string;progress:number;platform:string;releaseUrl:string;releaseName:string;notes:string;publishedAt:string;checkedAt:string;lastError:string}

const defaults:Settings={gameLanguage:"auto",questSite:"tarkov-dev",hideoutErrorNotifications:false,hideoutErrorSoundPath:"",language:'ja',screenshotDirectory:'',logsDirectory:'',remoteId:'',remoteTargets:[],map:'',gameMode:'auto',ocrEngine:'tesseract',tesseractPath:'',debug:false,saveRecognitionDebug:false,screenshotCleanup:false,screenshotRetainCount:500,screenshotRetainHours:168,soundsEnabled:true,questSoundEnabled:true,questSoundPath:'',errorSoundEnabled:true,errorSoundPath:'',soundVolume:28,autoStartMonitoring:true,openMapOnRaidStart:true,navigateMapOnPositionScreenshot:true,playerMarkerEffect:'none',playerMarkerColor:'',tarkovTrackerEnabled:false,matchFoundSoundEnabled:false,matchFoundSoundPath:'',raidStartSoundEnabled:false,raidStartSoundPath:'',runThroughSoundEnabled:false,runThroughSoundPath:'',runThroughSeconds:430,questItemsSoundEnabled:false,questItemsSoundPath:'',restartTasksSoundEnabled:false,restartTasksSoundPath:'',startMinimized:false,minimizeToTray:false,closeToTray:false,keepPriority:false,launchAtStartup:false,autoUpdate:true}
const emptyUpdate:UpdateStatus={current:"",latest:"",state:"idle",progress:0,platform:"",releaseUrl:"",releaseName:"",notes:"",publishedAt:"",checkedAt:"",lastError:""}
const emptyTracker:TrackerStatus={connection:'disabled',mode:'',profileId:'',accountId:'',displayName:'',playerLevel:0,completedTasks:0,failedTasks:0,pvpConfigured:false,pveConfigured:false,seasonalConfigured:false,lastSync:'',lastEvent:'',lastError:'',keys:[],profiles:[]}
const emptyStatus:Status={connection:'disconnected',monitoring:false,currentMap:'',raidActive:false,raidStartedAt:'',runThroughAt:'',lastQueueSeconds:0,lastScreenshot:'',screenshotType:'unknown',lastError:'',detectionScore:0,detectionLayout:'',analysisStage:'待機中',ocrRaw:'',lastQuest:'',questTrader:'',questMap:'',matchConfidence:0,questCandidates:[],cropPreview:'',questUrl:'',questObjectives:[],lastItem:'',itemId:'',itemShortName:'',itemUrl:'',itemIconUrl:'',itemConfidence:0,itemCandidates:[],lastRemoteCommand:'',tracker:emptyTracker}
const maps=['customs','factory','night-factory','ground-zero','ground-zero-21','interchange','icebreaker','the-lab','the-lab-dark','the-labyrinth','lighthouse','reserve','shoreline','streets-of-tarkov','terminal','woods']
// The player marker effects (app_marker.go), in the gallery's order, and
// the colour each has until one is chosen.
const markerEffects=['none','outline','glow','pulse','beacon'] as const
const markerEffectNames:Record<string,Parameters<typeof translate>[1]>={none:'markerEffectNone',outline:'markerOutline',glow:'markerGlow',pulse:'markerPulse',beacon:'markerBeacon'}
const markerOwnColor=(effect:string)=>effect==='outline'?'#ffffff':'#ff3b30'

function normalizeStatus(value:Partial<Status>|null|undefined):Status{
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
const hostSections=['status','logs','folders','recognition','remote','tracker','sounds','startup','debug'] as const
type HostSection=typeof hostSections[number]
// Times of day follow the browser's time format (set on the frame by the
// browser shell): 24-hour unless it is 12.
const hour12=()=>document.documentElement.dataset.clock==='12'
const hashSection=():HostSection=>{const hash=location.hash.slice(1) as HostSection;return hostSections.includes(hash)?hash:'status'}
const sectionTitles:Record<HostSection,Parameters<typeof translate>[1]>={status:'secStatus',logs:'secLogs',folders:'secFolders',recognition:'secRecognition',remote:'secRemote',tracker:'secTracker',sounds:'secSounds',startup:'secStartup',debug:'secDebug'}

// Names of the game languages, in their own language.
const languageNames:Record<string,string>={ja:'日本語',en:'English'}

function App(){
  const [settings,setSettings]=useState<Settings>(defaults)
  const [activeTab,setActiveTab]=useState<HostSection>(hashSection)
  const [status,setStatus]=useState<Status>(emptyStatus)
  const [updateStatus,setUpdateStatus]=useState<UpdateStatus>(emptyUpdate)
  const [notice,setNotice]=useState('')
  const [noticeError,setNoticeError]=useState(false)
  const [remoteTestResult,setRemoteTestResult]=useState<'idle'|'testing'|'success'|'error'>('idle')
  const [busy,setBusy]=useState(false)
  const [logs,setLogs]=useState<LogEntry[]>([])
  const [logLevel,setLogLevel]=useState('all')
  const [logCategory,setLogCategory]=useState('all')
  // The marker gallery's sheet, built by the Go side in the colour chosen.
  // The colour control follows the pointer; the setting takes it once the
  // pointer rests, so the map view is not rebuilt for every shade passed.
  const [markerCSS,setMarkerCSS]=useState('')
  const [markerColor,setMarkerColor]=useState('')
  const markerColorTimer=useRef<number|undefined>(undefined)
  useEffect(()=>{setMarkerColor(settings.playerMarkerColor)},[settings.playerMarkerColor])
  useEffect(()=>{let live=true;PlayerMarkerPreviewCSS(markerColor).then(css=>{if(live)setMarkerCSS(css)}).catch(()=>{if(live)setMarkerCSS('')});return()=>{live=false}},[markerColor])
  const pickMarkerColor=(color:string)=>{setMarkerColor(color);window.clearTimeout(markerColorTimer.current);markerColorTimer.current=window.setTimeout(()=>patch({playerMarkerColor:color}),400)}
  const [hideoutAlert,setHideoutAlert]=useState<HideoutEvent|null>(null)
  const [logQuery,setLogQuery]=useState('')
  const [trackerToken,setTrackerToken]=useState('')
  const [historyProfile,setHistoryProfile]=useState<TrackerProfile|null>(null)
  const [historyBreakpoints,setHistoryBreakpoints]=useState<TrackerHistoryBreakpoint[]>([])
  const [historyBreakpoint,setHistoryBreakpoint]=useState('')
  const [now,setNow]=useState(Date.now())
  const settingsRef=useRef<Settings>(defaults)
  const [saveState,setSaveState]=useState<'idle'|'saving'|'saved'|'error'>('idle')
  const [gameLanguages,setGameLanguages]=useState<string[]>(['ja','en'])
  const saveRevision=useRef(0)
  const backendReady=useRef(false)
  const persistQueue=useRef<Promise<unknown>>(Promise.resolve())
  const t=(key:Parameters<typeof translate>[1])=>translate(settings.language,key)
  useEffect(()=>{
    let active=true
    const unsubscribers:(()=>void)[]=[]
    const boot=async()=>{
      if(!active)return
      if(!desktop?.backend){setNotice(translate(defaults.language,'backendWait'));setNoticeError(true);return}
      try{
        // The game languages MAYAK reads come from the Go side (internal/locale).
        void GameLanguages().then(list=>{if(active&&Array.isArray(list)&&list.length)setGameLanguages(list)}).catch(()=>{})
        const [s,v,l,u]=await Promise.all([GetSettings(),GetStatus(),GetLogs(),GetUpdateStatus().catch(()=>emptyUpdate)])
        if(!active)return
        settingsRef.current={...defaults,...s,remoteTargets:Array.isArray((s as any).remoteTargets)?(s as any).remoteTargets:[]} as Settings
        setSettings(settingsRef.current)
        setStatus(normalizeStatus(v))
        setLogs(Array.isArray(l)?l as LogEntry[]:[])
        setUpdateStatus({...emptyUpdate,...(u??{})})
        backendReady.current=true
        unsubscribers.push(EventsOn('status:update',(next:Status)=>setStatus(current=>normalizeStatus({...current,...next}))))
        unsubscribers.push(EventsOn('log:entry',(entry:LogEntry)=>setLogs(current=>[...current.slice(-499),entry])))
        unsubscribers.push(EventsOn('hideout:alert',(event:HideoutEvent)=>setHideoutAlert(event)))
        unsubscribers.push(EventsOn('log:clear',()=>setLogs([])))
        unsubscribers.push(EventsOn('update:status',(next:UpdateStatus)=>setUpdateStatus({...emptyUpdate,...next})))
      }catch(error){if(active){setNotice(`${translate(defaults.language,'initError')}: ${String(error)}`);setNoticeError(true)}}
    }
    void boot()
    return()=>{active=false;unsubscribers.forEach(unsubscribe=>unsubscribe())}
  },[])
  useEffect(()=>{const timer=window.setInterval(()=>setNow(Date.now()),1000);return()=>window.clearInterval(timer)},[])
  // Success toasts fade out on their own.
  useEffect(()=>{if(saveState!=='saved')return;const timer=window.setTimeout(()=>setSaveState(state=>state==='saved'?'idle':state),2000);return()=>window.clearTimeout(timer)},[saveState])
  useEffect(()=>{if(!notice||noticeError)return;const timer=window.setTimeout(()=>setNotice(''),3000);return()=>window.clearTimeout(timer)},[notice,noticeError])
  useEffect(()=>{
    const changed=async()=>{
      setActiveTab(hashSection())
      if(!backendReady.current)return
      // Pending saves land first, so the reload includes them.
      await persistQueue.current.catch(()=>undefined)
      try{const s=await GetSettings();settingsRef.current={...defaults,...s,remoteTargets:Array.isArray((s as any).remoteTargets)?(s as any).remoteTargets:[]} as Settings;setSettings(settingsRef.current)}catch{/* Keep the current values. */}
    }
    window.addEventListener('hashchange',changed);return()=>window.removeEventListener('hashchange',changed)
  },[])
  // The shell's display language is the Host's too: it sets <html lang>.
  useEffect(()=>{
    const sync=()=>{const language=document.documentElement.lang;if((language==='ja'||language==='en')&&backendReady.current&&settingsRef.current.language!==language)patch({language})}
    const observer=new MutationObserver(sync);observer.observe(document.documentElement,{attributes:true,attributeFilter:['lang']})
    const timer=window.setInterval(sync,1000)
    return()=>{observer.disconnect();window.clearInterval(timer)}
  },[])
  useEffect(()=>{
    if(activeTab!=='tracker')return
    let pending=false
    const refresh=async()=>{if(!backendReady.current||pending)return;pending=true;try{await RefreshTrackerKeyNames()}catch{/* Keep the last fetched names when offline. */}finally{pending=false}}
    void refresh()
    const timer=window.setInterval(()=>void refresh(),60000)
    window.addEventListener('focus',refresh)
    return()=>{window.clearInterval(timer);window.removeEventListener('focus',refresh)}
  },[activeTab])
  const patch=(value:Partial<Settings>)=>{
    if(!backendReady.current)return
    const next={...settingsRef.current,...value}
    settingsRef.current=next
    setSettings(next)
    const revision=++saveRevision.current
    setSaveState('saving')
    persistQueue.current=persistQueue.current.catch(()=>undefined).then(()=>PersistSettings(next as any)).then(()=>{
      if(revision===saveRevision.current)setSaveState('saved')
    }).catch(error=>{
      if(revision===saveRevision.current){setSaveState('error');setNotice(String(error));setNoticeError(true)}
    })
  }
  const persistCurrent=()=>{
    const revision=++saveRevision.current
    const next=settingsRef.current
    setSaveState('saving')
    const pending=persistQueue.current.catch(()=>undefined).then(()=>SaveSettings(next as any))
    persistQueue.current=pending
    return pending.then(()=>{if(revision===saveRevision.current)setSaveState('saved')}).catch(error=>{if(revision===saveRevision.current)setSaveState('error');throw error})
  }
  const run=async(action:()=>Promise<unknown>,success:string)=>{setBusy(true);setNotice('');setNoticeError(false);try{await action();setNotice(success)}catch(error){setNotice(String(error));setNoticeError(true)}finally{setBusy(false)}}
  const save=()=>run(()=>persistCurrent(),t('saved'))
  const test=async()=>{setBusy(true);setRemoteTestResult('testing');try{await persistCurrent();await TestRemote();setRemoteTestResult('success')}catch{setRemoteTestResult('error')}finally{setBusy(false)}}
  const browseScreens=async()=>{const path=await ChooseScreenshotDirectory();if(path)patch({screenshotDirectory:path})}
  const browseLogs=async()=>{const path=await ChooseLogsDirectory();if(path)patch({logsDirectory:path})}
  const openFolder=async(action:()=>Promise<void>)=>{try{await action()}catch(error){setNotice(String(error));setNoticeError(true)}}
  const detectRemote=()=>run(async()=>{setRemoteTestResult('idle');const id=await AutoDetectRemoteID();if(settings.remoteTargets.some(target=>target.id===id))return;patch({remoteTargets:[...settings.remoteTargets,{id,name:`Remote ${settings.remoteTargets.length+1}`,map:true,tasks:true}]})},t('remoteDetected'))
  const updateRemoteTarget=(index:number,value:Partial<RemoteTarget>)=>patch({remoteTargets:settings.remoteTargets.map((target,i)=>i===index?{...target,...value}:target)})
  const addRemoteTarget=()=>patch({remoteTargets:[...settings.remoteTargets,{id:'',name:`Remote ${settings.remoteTargets.length+1}`,map:true,tasks:true}]})
  const removeRemoteTarget=(index:number)=>patch({remoteTargets:settings.remoteTargets.filter((_,i)=>i!==index)})
  const autoDetect=()=>run(async()=>{const detected=await AutoDetectEFTDirectories();patch({screenshotDirectory:detected.screenshotDirectory||settings.screenshotDirectory,logsDirectory:detected.logsDirectory||settings.logsDirectory})},t('foldersDetected'))
  const analyzeLatest=()=>run(()=>AnalyzeLatestScreenshot(),t('latestAnalyzed'))
  type SoundPathKey='hideoutErrorSoundPath'|'questSoundPath'|'errorSoundPath'|'matchFoundSoundPath'|'raidStartSoundPath'|'runThroughSoundPath'|'questItemsSoundPath'|'restartTasksSoundPath'
  const chooseSound=async(key:SoundPathKey)=>{try{const path=await ChooseSoundFile();if(path)patch({[key]:path} as Partial<Settings>)}catch(error){setNotice(String(error));setNoticeError(true)}}
  const previewSound=async(kind:string,path:string)=>{try{await PreviewSound(kind,path,settings.soundVolume)}catch(error){setNotice(String(error));setNoticeError(true)}}
  const soundAlerts=[
    {id:"hideout-error",label:t("hideoutErrorNotifications"),kind:"error",enabledKey:"hideoutErrorNotifications",pathKey:"hideoutErrorSoundPath"},
    {id:'quest-sound',label:t('questSuccess'),kind:'quest',enabledKey:'questSoundEnabled',pathKey:'questSoundPath'},
    {id:'error-sound',label:t('recognitionError'),kind:'error',enabledKey:'errorSoundEnabled',pathKey:'errorSoundPath'},
    {id:'match-found-sound',label:t('matchFoundSound'),kind:'matchFound',enabledKey:'matchFoundSoundEnabled',pathKey:'matchFoundSoundPath'},
    {id:'raid-start-sound',label:t('raidStartSound'),kind:'raidStart',enabledKey:'raidStartSoundEnabled',pathKey:'raidStartSoundPath'},
    {id:'run-through-sound',label:t('runThroughSound'),kind:'runThrough',enabledKey:'runThroughSoundEnabled',pathKey:'runThroughSoundPath'},
    {id:'quest-items-sound',label:t('questItemsSound'),kind:'questItems',enabledKey:'questItemsSoundEnabled',pathKey:'questItemsSoundPath'},
    {id:'restart-tasks-sound',label:t('restartTasksSound'),kind:'restartTasks',enabledKey:'restartTasksSoundEnabled',pathKey:'restartTasksSoundPath'},
  ] as const
  const allLogs:LogEntry[]=[...logs,...(settings.debug||logCategory==='Hideout'?status.hideout?.events??[]:[]).map((event,index)=>({id:`hideout-${index}`,timestamp:event.occurredAt,level:(event.status==='failed'?'Error':event.status==='unknown'?'Warn':'Info') as LogEntry['level'],category:'Hideout',message:hideoutMessage(event,settings.language),hideout:event}))]
  const filteredLogs=allLogs.filter(entry=>{
    if(logCategory!=='all'&&entry.category!==logCategory)return false
    if(logLevel!=='all'&&entry.level!==logLevel)return false
    const query=logQuery.trim().toLocaleLowerCase()
    return !query||`${entry.category} ${entry.message}`.toLocaleLowerCase().includes(query)
  })
  const displayedLogs=[...filteredLogs].sort((a,b)=>Date.parse(b.timestamp)-Date.parse(a.timestamp))
  const clearLogs=()=>run(()=>ClearLogs(),t('logsCleared'))
  const updateStateLabel=(update:UpdateStatus)=>{
    switch(update.state){
      case 'checking':return t('processing')
      case 'current':return t('updateUpToDate')
      case 'available':return t('updateAvailable')
      case 'downloading':return `${t('updateDownloading')} ${update.progress}%`
      case 'ready':return t('updateReady')
      case 'unsupported':return `${t('updateUnsupported')} (${update.platform})`
      case 'error':return t('updateFailed')
      default:return t('updateIdle')
    }
  }
  const trackerModeLabel=(mode:string)=>mode==='pve'?t('trackerPVE'):mode==='seasonal'?t('trackerSeasonal'):t('trackerPVP')
  const importTrackerKey=async()=>{setBusy(true);setNotice('');setNoticeError(false);try{const assigned=await ImportTrackerToken(trackerToken);setTrackerToken('');setNotice(assigned?t('trackerTokenAssigned').replace('{profile}',assigned):t('trackerTokenSaved'))}catch(error){setNotice(String(error));setNoticeError(true)}finally{setBusy(false)}}
  // The profiles by mode (PvP, Season and PvE are played side by side, so
  // each mode has a profile of its own): the mode played last first, the
  // latest seen first within a mode; the keys not on any profile.
  const shortProfile=(id:string)=>`${id.slice(0,6)}…${id.slice(-4)}`
  const seenDate=(iso:string)=>new Date(iso).toLocaleDateString(settings.language==='ja'?'ja-JP':'en-US')
  const modeOrder=['pvp','seasonal','pve']
  const lastPlayedMode=status.tracker.profiles.find(p=>p.current)?.mode
  const profileGroups=[...new Set([...(lastPlayedMode?[lastPlayedMode]:[]),...modeOrder,...status.tracker.profiles.map(p=>p.mode)])].map(mode=>({mode,profiles:status.tracker.profiles.filter(p=>p.mode===mode).sort((a,b)=>Date.parse(b.lastSeen)-Date.parse(a.lastSeen))})).filter(group=>group.profiles.length>0)
  const unassignedKeys=status.tracker.keys.filter(key=>!key.bound)
  const scanTrackerProfiles=()=>run(()=>DiscoverTrackerProfiles(),t('trackerProfilesScanned'))
  const assignTrackerKey=(profile:TrackerProfile,keyId:string)=>run(()=>SetTrackerProfileKey(profile.accountId,profile.profileId,profile.mode,keyId==='__none'?'':keyId),t('trackerAssignmentSaved'))
  // The same assignment from the key's side: a profile of the key's mode, or
  // none, which frees the key from the profile it is on.
  const profileKey=(p:{accountId:string;profileId:string;mode:string})=>`${p.accountId}/${p.profileId}/${p.mode}`
  const profileLabel=(p:TrackerProfile)=>`${t('trackerAccount')} ${p.accountId} · ${p.profileId.slice(0,6)}…${p.profileId.slice(-4)}${p.current?` · ${t('trackerCurrent')}`:''}`
  const assignKeyToProfile=(key:TrackerKey,value:string)=>{
    if(value==='__none'){if(key.bound)return run(()=>SetTrackerProfileKey(key.accountId,key.profileId,key.mode,''),t('trackerAssignmentSaved'));return}
    const profile=status.tracker.profiles.find(p=>profileKey(p)===value);if(profile)return assignTrackerKey(profile,key.id)
  }
  const removeTrackerKey=(keyId:string)=>run(()=>RemoveTrackerKey(keyId),t('trackerTokenRemoved'))
  const openTrackerHistory=async(profile:TrackerProfile)=>{setBusy(true);setNotice('');setNoticeError(false);try{const points=await GetTrackerHistoryBreakpoints(profile.accountId,profile.profileId,profile.mode) as TrackerHistoryBreakpoint[];setHistoryProfile(profile);setHistoryBreakpoints(points);setHistoryBreakpoint(points.at(-1)?.id??'');if(points.length===0){setNotice(t('trackerHistoryEmpty'));setNoticeError(true)}}catch(error){setNotice(String(error));setNoticeError(true)}finally{setBusy(false)}}
  const syncTrackerHistory=()=>historyProfile&&historyBreakpoint?run(()=>SyncTrackerHistory(historyProfile.accountId,historyProfile.profileId,historyProfile.mode,historyBreakpoint),t('trackerHistorySynced')):undefined
  const trackerConnectionLabel={disabled:t('trackerStatusDisabled'),'waiting-profile':t('trackerStatusWaitingProfile'),connecting:t('trackerStatusConnecting'),connected:t('trackerStatusConnected'),'missing-token':t('trackerStatusMissingToken'),error:t('trackerStatusError')}[status.tracker.connection]??status.tracker.connection
  const formatDuration=(seconds:number)=>`${Math.floor(Math.max(0,seconds)/60).toString().padStart(2,'0')}:${Math.floor(Math.max(0,seconds)%60).toString().padStart(2,'0')}`
  const raidElapsed=status.raidActive&&status.raidStartedAt?formatDuration((now-new Date(status.raidStartedAt).getTime())/1000):'—'
  const runThroughRemaining=status.raidActive&&status.runThroughAt?Math.max(0,(new Date(status.runThroughAt).getTime()-now)/1000):0
  const runThroughLabel=status.raidActive&&status.runThroughAt?(runThroughRemaining>0?formatDuration(runThroughRemaining):t('runThroughReady')):'—'
  // Saves, notices and hideout alerts show as toasts at the bottom right:
  // successes fade out, failures stay until dismissed or retried.
  const toasts=<div className="toast-stack" aria-live="polite">
    {saveState==='saved'&&<div className="toast success" role="status"><Check/><span>{t('settingsSaved')}</span></div>}
    {saveState==='error'&&<div className="toast error" role="alert"><span>{t('settingsSaveFailed')}{notice&&<small>{notice}</small>}</span><Button type="button" size="sm" variant="secondary" onClick={save} disabled={busy}><RefreshCw/>{t('settingsRetry')}</Button></div>}
    {notice&&saveState!=='error'&&<div className={`toast ${noticeError?'error':'success'}`} role={noticeError?'alert':'status'}>{!noticeError&&<Check/>}<span>{notice}</span><button type="button" className="toast-close" onClick={()=>setNotice('')} aria-label="×"><X/></button></div>}
    {hideoutAlert&&<div className="toast warn" role="alert"><span>{hideoutMessage(hideoutAlert,settings.language)}</span><button type="button" className="toast-close" onClick={()=>setHideoutAlert(null)} aria-label={t('hideoutDismiss')}><X/></button></div>}
  </div>
  return <main className="shell">
    <Tabs className="app-tabs" value={activeTab}>
      <div className={`section-head${["status","logs"].includes(activeTab)?"":" narrow"}`}><h1>{t(sectionTitles[activeTab])}</h1></div>
      <TabsContent value="folders"><div className="settings-stack">
          <Card><CardHeader><div className="icon-title"><FolderOpen/><div><CardTitle>{t('foldersTitle')}</CardTitle><CardDescription>{t('foldersDescription')}</CardDescription></div></div></CardHeader><CardContent>
            <Button className="detect-button" type="button" variant="secondary" onClick={autoDetect} disabled={busy}><RefreshCw/>{t('autoDetect')}</Button>
            <div className="field"><Label htmlFor="screenshots">{t('screenshotsFolder')}</Label><div className="input-action"><Input id="screenshots" value={settings.screenshotDirectory} onChange={e=>patch({screenshotDirectory:e.target.value})} placeholder="C:\Users\...\Escape from Tarkov\Screenshots"/><Button type="button" variant="secondary" onClick={browseScreens}><FolderOpen/>{t('select')}</Button></div></div>
            <div className="field"><Label htmlFor="logs">{t('logsFolder')}</Label><div className="input-action"><Input id="logs" value={settings.logsDirectory} onChange={e=>patch({logsDirectory:e.target.value})} placeholder="Escape from Tarkov\Logs"/><Button type="button" variant="secondary" onClick={browseLogs}><FolderOpen/>{t('select')}</Button></div></div>
            <div className="switch-stack">
              
              <div className="switch-row"><div><Label htmlFor="screenshot-cleanup">{t('screenshotCleanup')}</Label><p className="help">{t('screenshotCleanupHelp')}</p></div><Switch id="screenshot-cleanup" checked={settings.screenshotCleanup} onCheckedChange={screenshotCleanup=>patch({screenshotCleanup})}/></div>
            </div>
            {settings.screenshotCleanup&&<div className="retention-fields"><div className="field"><Label htmlFor="retain-count">{t('retainCount')}</Label><Input id="retain-count" type="number" min="0" max="100000" value={settings.screenshotRetainCount} onChange={e=>patch({screenshotRetainCount:Math.max(0,Number(e.target.value)||0)})}/><p className="help">{t('zeroDisables')}</p></div><div className="field"><Label htmlFor="retain-hours">{t('retainHours')}</Label><Input id="retain-hours" type="number" min="0" max="87600" value={settings.screenshotRetainHours} onChange={e=>patch({screenshotRetainHours:Math.max(0,Number(e.target.value)||0)})}/><p className="help">{t('zeroDisables')}</p></div></div>}
          </CardContent></Card>
      </div></TabsContent>
      <TabsContent value="recognition"><div className="settings-stack">
          <Card><CardHeader><CardTitle>{t('analysisTitle')}</CardTitle><CardDescription>{t('analysisDescription')}</CardDescription></CardHeader><CardContent>
            <div className="field"><Label>{t('gameMode')}</Label><Select value={settings.gameMode} onValueChange={gameMode=>patch({gameMode})}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent><SelectItem value="auto">{t('gameModeAuto')}</SelectItem><SelectItem value="pve">{t('gameModePVE')}</SelectItem><SelectItem value="regular">{t('gameModePVP')}</SelectItem><SelectItem value="pvp-season">{t('gameModeSeason')}</SelectItem></SelectContent></Select></div>
            <div className="field"><Label>{t('gameLanguage')}</Label><Select value={settings.gameLanguage||'auto'} onValueChange={gameLanguage=>patch({gameLanguage})}><SelectTrigger aria-label={t('gameLanguage')}><SelectValue/></SelectTrigger><SelectContent><SelectItem value="auto">{t('gameLanguageAuto')}</SelectItem>{gameLanguages.map(language=><SelectItem key={language} value={language}>{languageNames[language]||language}</SelectItem>)}</SelectContent></Select><p className="help-text">{t('gameLanguageHelp')}</p></div><div className="field"><Label>{t('ocrEngine')}</Label><Select value={settings.ocrEngine} onValueChange={ocrEngine=>patch({ocrEngine})}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent><SelectItem value="tesseract">{t('tesseractOCR')}</SelectItem><SelectItem value="windows">{t('windowsOCR')}</SelectItem></SelectContent></Select></div>
            {settings.ocrEngine==='tesseract'&&<div className="field"><Label htmlFor="tesseract">{t('tesseractPath')}</Label><Input id="tesseract" value={settings.tesseractPath} onChange={e=>patch({tesseractPath:e.target.value})} placeholder={t('tesseractPlaceholder')}/></div>}
          </CardContent></Card>
          
      </div></TabsContent>
      <TabsContent value="remote"><div className="settings-stack">
          <Card className="remote-card"><CardHeader><div className="icon-title"><Wifi/><div><CardTitle>{t('remoteTitle')}</CardTitle><CardDescription>{t('remoteDescription')}</CardDescription></div></div></CardHeader><CardContent>
            <div className="remote-target-heading"><Label>{t('remoteTargets')}</Label><div><Button type="button" size="sm" variant="secondary" onClick={addRemoteTarget}><Plus/>{t('addRemote')}</Button><Button type="button" size="sm" variant="secondary" onClick={detectRemote} disabled={busy}><RefreshCw/>{t('detectRemote')}</Button></div></div>
            <div className="remote-targets">{settings.remoteTargets.length===0?<p className="empty">{t('noRemoteTargets')}</p>:settings.remoteTargets.map((target,index)=><div className="remote-target" key={index}><div className="remote-target-inputs"><Input aria-label={t('remoteName')} value={target.name} onChange={e=>updateRemoteTarget(index,{name:e.target.value})} placeholder={t('remoteName')}/><Input aria-label={t('tarkovRemoteId')} value={target.id} onChange={e=>{setRemoteTestResult('idle');updateRemoteTarget(index,{id:e.target.value})}} placeholder={t('remotePlaceholder')} autoComplete="off"/><Button type="button" size="sm" variant="ghost" aria-label={t('removeRemote')} onClick={()=>removeRemoteTarget(index)}><X/></Button></div><div className="remote-target-roles"><label><Switch checked={target.map} onCheckedChange={map=>updateRemoteTarget(index,{map})}/><span>{t('remoteMapRole')}</span></label><label><Switch checked={target.tasks} onCheckedChange={tasks=>updateRemoteTarget(index,{tasks})}/><span>{t('remoteTaskRole')}</span></label></div></div>)}</div>
            {settings.browserRemoteId&&<p className="help">{t('browserRemote').replace('{id}',settings.browserRemoteId)}</p>}
            <p className="help remote-help"><span>{t('remoteHelp')}</span><button type="button" className="inline-link" onClick={()=>BrowserOpenURL('https://tarkov.dev/')}><ExternalLink/>{t('openTarkovDevRemote')}</button></p>
            <div className="field remote-map-fallback"><Label>{t('mapFallback')}</Label><Select value={settings.map||'__auto'} onValueChange={map=>patch({map:map==='__auto'?'':map})}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent><SelectItem value="__auto">{t('mapAuto')}</SelectItem>{maps.map(map=><SelectItem key={map} value={map}>{map}</SelectItem>)}</SelectContent></Select></div>
            <div className="switch-stack remote-switches">
              <div className="switch-row"><div><Label htmlFor="open-map-on-raid-start">{t('openMapOnRaidStart')}</Label><p className="help">{t('openMapOnRaidStartHelp')}</p></div><Switch id="open-map-on-raid-start" checked={settings.openMapOnRaidStart} onCheckedChange={openMapOnRaidStart=>patch({openMapOnRaidStart})}/></div>
              <div className="switch-row"><div><Label htmlFor="navigate-map-on-shot">{t('navigateMapOnPositionScreenshot')}</Label><p className="help">{t('navigateMapOnPositionScreenshotHelp')}</p></div><Switch id="navigate-map-on-shot" checked={settings.navigateMapOnPositionScreenshot} onCheckedChange={navigateMapOnPositionScreenshot=>patch({navigateMapOnPositionScreenshot})}/></div>
            </div>
            <div className="field marker-field"><Label>{t('markerTitle')}</Label><p className="help">{t('markerDescription')}</p>
              <style>{markerCSS}</style>
              <div className="marker-gallery" role="radiogroup" aria-label={t('markerTitle')}>{markerEffects.map(effect=><button type="button" key={effect} role="radio" aria-checked={settings.playerMarkerEffect===effect} className={`marker-choice${settings.playerMarkerEffect===effect?' selected':''}`} onClick={()=>patch({playerMarkerEffect:effect})}><span className="marker-preview" data-effect={effect}><span className="marker-icon"><img src="/marker-arrow.svg?v=2" alt="" style={{width:24,height:24,rotate:'35deg'}}/></span></span><span className="marker-name">{t(markerEffectNames[effect]??'markerEffectNone')}</span></button>)}</div>
              <div className="marker-color-row"><Label htmlFor="marker-color">{t('markerColor')}</Label><input id="marker-color" type="color" className="marker-color-input" value={markerColor||markerOwnColor(settings.playerMarkerEffect)} disabled={settings.playerMarkerEffect==='none'} onChange={e=>pickMarkerColor(e.target.value)}/><span className="marker-color-value">{markerColor||t('markerColorOwn')}</span>{markerColor&&<Button type="button" size="sm" variant="ghost" onClick={()=>pickMarkerColor('')}><RotateCcw/>{t('markerColorReset')}</Button>}</div>
              <p className="help">{t('markerHelp')}</p>
            </div>
            <div className="connection-test-row"><Button type="button" variant="secondary" onClick={test} disabled={busy||!settings.remoteTargets.some(target=>target.id.trim())}><Wifi/>{t('testConnection')}</Button>{remoteTestResult!=='idle'&&<span className={`connection-test-result ${remoteTestResult}`}>{remoteTestResult==='success'?<Check/>:remoteTestResult==='error'?<CircleX/>:<RefreshCw className="spin"/>}{remoteTestResult==='success'?t('remoteConnected'):remoteTestResult==='error'?t('remoteConnectionFailed'):t('remoteConnecting')}</span>}</div>
          </CardContent></Card>
      </div></TabsContent>
      <TabsContent value="tracker"><div className="settings-stack">
          <Card className="tracker-card"><CardHeader><div className="card-title-actions"><div className="icon-title"><CloudSync/><div><CardTitle>{t('trackerTitle')}</CardTitle><CardDescription>{t('trackerDescription')}</CardDescription></div></div><Button type="button" variant="secondary" onClick={scanTrackerProfiles} disabled={busy||!settings.logsDirectory}><RefreshCw/>{t('trackerScanLogs')}</Button></div></CardHeader><CardContent>
            <div className="switch-row"><div><Label htmlFor="tracker-enabled">{t('trackerEnable')}</Label><p className="help">{t('trackerEnableHelp')}</p></div><Switch id="tracker-enabled" checked={settings.tarkovTrackerEnabled} onCheckedChange={tarkovTrackerEnabled=>patch({tarkovTrackerEnabled})}/></div>
            {settings.tarkovTrackerEnabled&&<div className="tracker-modes">
              <section className="tracker-step"><div className="tracker-step-heading"><span className="step-pill">{t('trackerStep').replace('{n}','1')}</span><div><Label>{t('trackerKeysTitle')}</Label><p className="help">{t('trackerKeysHelp')}</p></div></div>
              <div className="tracker-import"><div><Label htmlFor="tracker-token">{t('trackerTokenLabel')}</Label><div className="secret-input"><KeyRound/><Input id="tracker-token" type="password" value={trackerToken} onChange={event=>setTrackerToken(event.target.value)} placeholder="PVP_… / SZN_… / PVE_…" autoComplete="off"/></div></div><Button type="button" variant="secondary" disabled={!trackerToken.trim()||busy} onClick={()=>void importTrackerKey()}>{t('trackerImportToken')}</Button></div>
              {unassignedKeys.length>0&&<section className="tracker-unassigned"><div><Label>{t('trackerUnassignedKeys')}</Label><p className="help">{t('trackerUnassignedKeysHelp')}</p></div>{unassignedKeys.map(key=>{const targets=status.tracker.profiles.filter(p=>p.mode===key.mode&&!p.boundKeyId);const current=targets.find(p=>p.current);return <div className="tracker-unassigned-key" key={key.id}><span className="tracker-key-chip"><KeyRound/><strong>{key.name||trackerModeLabel(key.mode)}</strong><code>{key.maskedToken}</code><Badge>{trackerModeLabel(key.mode)}</Badge></span><div className="tracker-profile-actions">{current&&<Button type="button" size="sm" variant="secondary" disabled={busy} onClick={()=>void assignTrackerKey(current,key.id)}><Check/>{t('trackerAssignToCurrent')}</Button>}{targets.length>0?<Select value="__none" onValueChange={value=>void assignKeyToProfile(key,value)}><SelectTrigger aria-label={t('trackerAssignTo')} className="tracker-key-select"><SelectValue placeholder={t('trackerChooseProfile')}/></SelectTrigger><SelectContent>{targets.map(p=><SelectItem key={profileKey(p)} value={profileKey(p)}>{profileLabel(p)}</SelectItem>)}</SelectContent></Select>:<small className="tracker-no-key">{t('trackerNoProfileForMode')}</small>}<Button type="button" variant="ghost" disabled={busy} aria-label={t('trackerRemoveToken')} title={t('trackerRemoveToken')} onClick={()=>void removeTrackerKey(key.id)}><X/></Button></div></div>})}</section>}
              {status.tracker.keys.length>0&&<details className="tracker-manage"><summary>{t('trackerManageKeys')}</summary><p className="help">{t('trackerKeyNamesHelp')}</p><div className="tracker-manage-list">{status.tracker.keys.map(key=><div className="tracker-manage-key" key={key.id}><span className="tracker-key-chip"><KeyRound/><strong>{key.name||trackerModeLabel(key.mode)}</strong><code>{key.maskedToken}</code><Badge>{trackerModeLabel(key.mode)}</Badge></span><small>{key.bound?`${t('trackerAccount')} ${key.accountId} · ${t('trackerProfile')} ${shortProfile(key.profileId)}`:t('trackerUnassigned')}</small><Button type="button" variant="ghost" disabled={busy||key.bound} aria-label={t('trackerRemoveToken')} title={key.bound?t('trackerUnassignBeforeRemove'):t('trackerRemoveToken')} onClick={()=>void removeTrackerKey(key.id)}><X/></Button></div>)}</div></details>}
              </section>
              <section className="tracker-step"><div className="tracker-step-heading"><span className="step-pill">{t('trackerStep').replace('{n}','2')}</span><div><Label>{t('trackerProfilesTitle')}</Label><p className="help">{t('trackerProfilesHelp')}</p></div></div>
              <div className="tracker-profiles">{profileGroups.length===0?<p className="empty">{t('trackerNoProfiles')}</p>:profileGroups.map(group=><section className="tracker-mode-group" key={group.mode}><header className="tracker-group-heading"><strong>{trackerModeLabel(group.mode)}</strong><small>{t('trackerModeProfiles').replace('{n}',String(group.profiles.length))}</small>{group.profiles.some(p=>p.current)&&<Badge className="current">{t('trackerLastPlayed')}</Badge>}</header><div className="tracker-group-columns" aria-hidden="true"><span>{t('trackerProfile')}</span><span>{t('trackerKeyColumn')}</span><span/></div>{group.profiles.map((profile,index)=>{const key=status.tracker.keys.find(k=>k.id===profile.boundKeyId);const choices=status.tracker.keys.filter(k=>k.mode===profile.mode&&(!k.bound||k.id===profile.boundKeyId));return <div className={`tracker-profile${profile.current?' current':''}`} key={profileKey(profile)}><div className="tracker-profile-info"><span>{profile.current&&<Badge className="current">{t('trackerLastPlayed')}</Badge>}{!profile.current&&index===0&&group.profiles.length>1&&<Badge>{t('trackerLatestInMode')}</Badge>}{!key&&<Badge className="warn">{t('trackerUnassigned')}</Badge>}</span><strong>{t('trackerAccount')} {profile.accountId}</strong><small>{t('trackerProfile')} {shortProfile(profile.profileId)} · {t('trackerLastSeen')} {seenDate(profile.lastSeen)}</small></div><div className="tracker-profile-key">{key?<span className="tracker-key-chip"><KeyRound/><strong>{key.name||trackerModeLabel(key.mode)}</strong><code>{key.maskedToken}</code></span>:choices.length===0?<small className="tracker-no-key">{t('trackerNoKeyForMode')}</small>:null}<div className="tracker-profile-actions">{choices.length>0&&<Select value={profile.boundKeyId||'__none'} onValueChange={value=>void assignTrackerKey(profile,value)}><SelectTrigger aria-label={t('trackerAssignedKey')} className="tracker-key-select"><SelectValue placeholder={t('trackerChooseKey')}/></SelectTrigger><SelectContent><SelectItem value="__none">{t('trackerUnassigned')}</SelectItem>{choices.map(k=><SelectItem key={k.id} value={k.id}>{k.name?`${k.name} · ${k.maskedToken}`:k.maskedToken}</SelectItem>)}</SelectContent></Select>}</div></div><div className="tracker-profile-tools">{key&&<Button type="button" variant="ghost" disabled={busy} onClick={()=>void openTrackerHistory(profile)}><History/>{t('trackerReadPastLogs')}</Button>}</div></div>})}</section>)}</div>
              {historyProfile&&<div className="tracker-history"><div><Label>{t('trackerHistoryStart')}</Label><p className="help">{trackerModeLabel(historyProfile.mode)} · {t('trackerAccount')} {historyProfile.accountId} · {historyProfile.profileId.slice(0,6)}…{historyProfile.profileId.slice(-4)}</p></div>{historyBreakpoints.length>0&&<><Select value={historyBreakpoint} onValueChange={setHistoryBreakpoint}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent>{historyBreakpoints.map(point=><SelectItem key={point.id} value={point.id}>{point.version} · {new Date(point.startAt).toLocaleDateString(settings.language==='ja'?'ja-JP':'en-US')}</SelectItem>)}</SelectContent></Select><Button type="button" disabled={busy||!historyBreakpoint} onClick={()=>void syncTrackerHistory()}><CloudSync/>{t('trackerSyncPastLogs')}</Button></>}</div>}
              </section>
            </div>}
            <p className="help"><button type="button" className="inline-link" onClick={()=>BrowserOpenURL('https://tarkovtracker.org/settings#api')}><ExternalLink/>{t('trackerOpenSettings')}</button></p>
          </CardContent></Card>
      </div></TabsContent>
      <TabsContent value="sounds"><div className="settings-stack">
          <Card><CardHeader><div className="icon-title"><Volume2/><div><CardTitle>{t('soundsTitle')}</CardTitle><CardDescription>{t('soundsDescription')}</CardDescription></div></div></CardHeader><CardContent>
            <div className="switch-stack"><div className="switch-row"><div><Label htmlFor="sounds-enabled">{t('soundsEnabled')}</Label><p className="help">{t('soundsHelp')}</p></div><Switch id="sounds-enabled" checked={settings.soundsEnabled} onCheckedChange={soundsEnabled=>patch({soundsEnabled})}/></div></div>
            {settings.soundsEnabled&&<div className="sound-options">
              {soundAlerts.map(alert=>{const path=settings[alert.pathKey];return <div className="sound-setting" key={alert.id}><div className="switch-row"><Label htmlFor={alert.id}>{alert.label}</Label><Switch id={alert.id} checked={settings[alert.enabledKey]} onCheckedChange={checked=>patch({[alert.enabledKey]:checked} as Partial<Settings>)}/></div><div className="sound-file-row"><Button type="button" size="sm" variant="secondary" onClick={()=>void chooseSound(alert.pathKey)}><FolderOpen/>{t('chooseSound')}</Button><span className="sound-file-name" title={path||t('builtInSound')}>{path?path.split(/[\\/]/).pop():t('builtInSound')}</span><Button type="button" size="sm" variant="ghost" disabled={!path} onClick={()=>patch({[alert.pathKey]:''} as Partial<Settings>)}><RotateCcw/>{t('resetSound')}</Button><Button type="button" size="sm" variant="ghost" onClick={()=>void previewSound(alert.kind,path)}><Play/>{t('previewSound')}</Button></div>{alert.kind==='runThrough'&&settings.runThroughSoundEnabled&&<div className="field"><Label>{t('runThroughTime')}</Label><div className="time-fields"><div><Input aria-label={t('minutes')} type="number" min="0" max="59" value={Math.floor(settings.runThroughSeconds/60)} onChange={e=>patch({runThroughSeconds:Math.max(1,Math.min(3599,Number(e.target.value)*60+settings.runThroughSeconds%60))})}/><span>{t('minutes')}</span></div><div><Input aria-label={t('seconds')} type="number" min="0" max="59" value={settings.runThroughSeconds%60} onChange={e=>patch({runThroughSeconds:Math.max(1,Math.min(3599,Math.floor(settings.runThroughSeconds/60)*60+Number(e.target.value)))})}/><span>{t('seconds')}</span></div></div><p className="help">{t('runThroughTimeHelp')}</p></div>}</div>})}
              <div className="field"><Label htmlFor="sound-volume">{t('volume')} {settings.soundVolume}%</Label><input className="range" id="sound-volume" type="range" min="0" max="100" step="1" value={settings.soundVolume} onChange={e=>patch({soundVolume:Number(e.target.value)})}/></div>
            </div>}
          </CardContent></Card>
      </div></TabsContent>
      <TabsContent value="startup"><div className="settings-stack">
          <Card><CardHeader><div className="icon-title"><MonitorCog/><div><CardTitle>{t('startupTitle')}</CardTitle><CardDescription>{t('startupDescription')}</CardDescription></div></div></CardHeader><CardContent>
            <div className="switch-stack">
              <div className="switch-row"><div><Label htmlFor="launch-at-startup">{t('launchAtStartup')}</Label><p className="help">{t('launchAtStartupHelp')}</p></div><Switch id="launch-at-startup" checked={settings.launchAtStartup} onCheckedChange={launchAtStartup=>patch({launchAtStartup})}/></div>
              <div className="switch-row"><div><Label htmlFor="start-minimized">{t('startMinimized')}</Label><p className="help">{t('startMinimizedHelp')}</p></div><Switch id="start-minimized" checked={settings.startMinimized} onCheckedChange={startMinimized=>patch({startMinimized})}/></div>
              <div className="switch-row"><div><Label htmlFor="auto-monitoring">{t('autoStartMonitoring')}</Label><p className="help">{t('autoStartMonitoringHelp')}</p></div><Switch id="auto-monitoring" checked={settings.autoStartMonitoring} onCheckedChange={autoStartMonitoring=>patch({autoStartMonitoring})}/></div>
              <div className="switch-row"><div><Label htmlFor="minimize-to-tray">{t('minimizeToTray')}</Label><p className="help">{t('minimizeToTrayHelp')}</p></div><Switch id="minimize-to-tray" checked={settings.minimizeToTray} onCheckedChange={minimizeToTray=>patch({minimizeToTray})}/></div>
              <div className="switch-row"><div><Label htmlFor="close-to-tray">{t('closeToTray')}</Label><p className="help">{t('closeToTrayHelp')}</p></div><Switch id="close-to-tray" checked={settings.closeToTray} onCheckedChange={closeToTray=>patch({closeToTray})}/></div>
              <div className="switch-row"><div><Label htmlFor="keep-priority">{t('keepPriority')}</Label><p className="help">{t('keepPriorityHelp')}</p></div><Switch id="keep-priority" checked={settings.keepPriority} onCheckedChange={keepPriority=>patch({keepPriority})}/></div>
              <div className="switch-row"><div><Label htmlFor="auto-update">{t('autoUpdate')}</Label><p className="help">{t('autoUpdateHelp')}</p></div><Switch id="auto-update" checked={settings.autoUpdate} onCheckedChange={autoUpdate=>patch({autoUpdate})}/></div>
            </div>
          </CardContent></Card>
      </div></TabsContent>
      <TabsContent value="status"><div className="status-grid">
        <Metric icon={<Wifi/>} label={t('remoteConnection')} value={status.connection==='connected'?t('connected'):status.connection==='disconnected'?t('disconnected'):status.connection}/>
        <Metric icon={<MapPinned/>} label={t('currentMap')} value={status.currentMap||t('notDetected')}/>
        <Metric icon={<Activity/>} label={t('raidState')} value={status.raidActive?t('raidIn'):t('raidOut')}/>
        <Metric icon={<ScanLine/>} label={t('screenshot')} value={status.screenshotType||t('unknown')}/>

        <Card className="wide tracker-status"><CardHeader><div className="card-title-actions"><div><CardTitle>{t('trackerTitle')}</CardTitle><CardDescription>{trackerConnectionLabel}</CardDescription></div><Button type="button" variant="secondary" disabled={busy||!settings.tarkovTrackerEnabled||!status.tracker.mode} onClick={()=>run(()=>RefreshTracker(),t('trackerRefreshed'))}><RefreshCw/>{t('trackerRefresh')}</Button></div></CardHeader><CardContent><dl><dt>{t('trackerCurrentMode')}</dt><dd>{status.tracker.mode||'—'}</dd><dt>{t('trackerProfile')}</dt><dd>{status.tracker.displayName||status.tracker.profileId||'—'}</dd><dt>{t('trackerLevel')}</dt><dd>{status.tracker.playerLevel||'—'}</dd><dt>{t('trackerCompleted')}</dt><dd>{status.tracker.completedTasks}</dd><dt>{t('trackerFailed')}</dt><dd>{status.tracker.failedTasks}</dd><dt>{t('trackerLastEvent')}</dt><dd>{status.tracker.lastEvent||'—'}</dd><dt>{t('trackerLastSync')}</dt><dd>{status.tracker.lastSync?new Date(status.tracker.lastSync).toLocaleString(settings.language==='ja'?'ja-JP':'en-US',{hour12:hour12()}):'—'}</dd></dl>{status.tracker.lastError&&<p className="error">{status.tracker.lastError}</p>}</CardContent></Card>
        <Card className="wide"><CardHeader><CardTitle>{t('lastDetection')}</CardTitle></CardHeader><CardContent><Button type="button" variant="secondary" onClick={analyzeLatest} disabled={busy}><RefreshCw/>{t('analyzeLatest')}</Button><dl><dt>{t('processState')}</dt><dd>{translateAnalysisStage(settings.language,status.analysisStage)}</dd><dt>{t('screenLayout')}</dt><dd>{status.detectionLayout||'—'}</dd><dt>{t('remoteSend')}</dt><dd>{status.lastRemoteCommand||'—'}</dd><dt>{t('file')}</dt><dd>{status.lastScreenshot||t('waiting')}</dd>{status.screenshotType==='position'&&status.position&&<><dt>{t('position')}</dt><dd>{status.position.x.toFixed(2)}, {status.position.y.toFixed(2)}, {status.position.z.toFixed(2)} / {status.position.rotation.toFixed(1)}°</dd></>}{status.screenshotType==='item'?<><dt>{t('item')}</dt><dd>{status.lastItem||'—'} {status.itemConfidence>0&&`(${Math.round(status.itemConfidence*100)}%)`}</dd></>:<><dt>{t('quest')}</dt><dd>{status.lastQuest||'—'} {status.matchConfidence>0&&`(${Math.round(status.matchConfidence*100)}%)`}</dd>{status.lastQuest&&<><dt>{t('questSiteTitle')}</dt><dd><Button type="button" variant="secondary" onClick={()=>void openFolder(OpenQuestPage)}><ExternalLink/>{t('openQuestPage')}</Button></dd></>}</>}<dt>{t('ocrRaw')}</dt><dd>{status.ocrRaw||'—'}</dd></dl>{status.lastError&&<p className="error">{status.lastError}</p>}</CardContent></Card>
        <Card className="wide"><CardHeader><div className="card-title-actions"><div><CardTitle>{t('catalogTitle')}</CardTitle><CardDescription>{t('catalogHelp')}</CardDescription></div><Button type="button" variant="secondary" disabled={busy||status.catalog?.state==='loading'} onClick={()=>run(()=>RefreshCatalog(),t('catalogRefreshed'))}><RefreshCw/>{t('catalogRefresh')}</Button></div></CardHeader><CardContent><dl><dt>{t('gameMode')}</dt><dd>{status.catalog?.mode||t('notDetected')}</dd><dt>{t('processState')}</dt><dd>{status.catalog?.state==='loading'?t('processing'):status.catalog?.state==='stale'?t('catalogStale'):status.catalog?.state==='ready'?t('catalogReady'):t('waiting')}</dd><dt>{t('catalogItems')}</dt><dd>{status.catalog?.items||0}</dd><dt>{t('catalogMaps')}</dt><dd>{status.catalog?.maps||0}</dd><dt>{t('catalogTraders')}</dt><dd>{status.catalog?.traders||0}</dd><dt>{t('catalogTasks')}</dt><dd>{status.catalog?.tasks||0}</dd><dt>{t('catalogHideout')}</dt><dd>{status.catalog?.hideoutStations||0}</dd><dt>{t('catalogUpdated')}</dt><dd>{status.catalog?.updatedAt?new Date(status.catalog.updatedAt).toLocaleString(settings.language==='ja'?'ja-JP':'en-US',{hour12:hour12()}):'—'}</dd></dl>{status.catalog?.lastError&&<p className="error">{status.catalog.lastError}</p>}</CardContent></Card>
      </div></TabsContent>
      <TabsContent value="logs"><div className="log-dashboard"><div className="log-summary"><Metric icon={<MapPinned/>} label={t('currentMap')} value={status.currentMap||t('notDetected')}/><Metric icon={<Clock3/>} label={t('raidTimer')} value={raidElapsed}/><Metric icon={<Activity/>} label={t('runThroughTimer')} value={runThroughLabel}/><Metric icon={<Radio/>} label={t('lastQueueTime')} value={status.lastQueueSeconds>0?`${status.lastQueueSeconds.toFixed(1)}s`:'—'}/></div><Card className="logs-card"><CardHeader><div className="folder-toolbar"><Button type="button" variant="secondary" disabled={!settings.screenshotDirectory} onClick={()=>void openFolder(OpenScreenshotDirectory)}><FolderOpen/>{t('openScreenshots')}</Button><Button type="button" variant="secondary" disabled={!settings.logsDirectory} onClick={()=>void openFolder(OpenLogsDirectory)}><FolderOpen/>{t('openEftLogs')}</Button><Button type="button" variant="secondary" disabled={!settings.screenshotDirectory} onClick={()=>void openFolder(OpenDebugDirectory)}><Bug/>{t('openDebugFolder')}</Button></div><div className="logs-toolbar"><Select value={logCategory} onValueChange={setLogCategory}><SelectTrigger className="hideout-filter" aria-label={t('hideoutFilterAll')}><SelectValue/></SelectTrigger><SelectContent><SelectItem value="all">{t('hideoutFilterAll')}</SelectItem><SelectItem value="Hideout">{t('hideoutLogCategory')}</SelectItem></SelectContent></Select><div className="log-filter"><Select value={logLevel} onValueChange={setLogLevel}><SelectTrigger aria-label={t('logLevel')}><SelectValue/></SelectTrigger><SelectContent><SelectItem value="all">{t('allLevels')}</SelectItem><SelectItem value="Error">Error</SelectItem><SelectItem value="Warn">Warn</SelectItem><SelectItem value="Info">Info</SelectItem><SelectItem value="Debug">Debug</SelectItem></SelectContent></Select></div><div className="log-search"><Search/><Input value={logQuery} onChange={event=>setLogQuery(event.target.value)} placeholder={t('searchLogs')}/></div><span className="log-count">{filteredLogs.length} / {allLogs.length}</span><Button type="button" variant="secondary" onClick={clearLogs} disabled={busy||allLogs.length===0}><Trash2/>{t('clearLogs')}</Button></div></CardHeader><CardContent className="logs-content">{displayedLogs.length===0?<p className="empty">{t('noLogs')}</p>:<div className="log-list" role="log">{displayedLogs.map(entry=><div className="log-row" key={entry.id}><time dateTime={entry.timestamp}>{new Date(entry.timestamp).toLocaleTimeString(settings.language==='ja'?'ja-JP':'en-US',{hour12:hour12()})}</time><span className={`log-level ${entry.level.toLowerCase()}`}>{entry.hideout?t(entry.hideout.status==='failed'?'hideoutResultFailed':entry.hideout.status==='unknown'?'hideoutResultUnknown':'hideoutResultInfo'):entry.level}</span><span className="log-category">{entry.hideout?t('hideoutLogCategory'):entry.category}</span><span className="log-message">{entry.hideout&&(entry.hideout.status==='failed'?'⚠ ':entry.hideout.status==='unknown'?'? ':'ⓘ ')}{entry.message}{entry.hideout&&<small className="hideout-identity">{entry.hideout.mode?trackerModeLabel(entry.hideout.mode):t('unknown')} · {entry.hideout.accountId||t('unknown')} / {entry.hideout.profileId?`${entry.hideout.profileId.slice(0,6)}…${entry.hideout.profileId.slice(-4)}`:t('unknown')} · {new Date(entry.timestamp).toLocaleDateString(settings.language)} {entry.hideout.historical?`· ${t('hideoutHistory')}`:''}</small>}</span></div>)}</div>}</CardContent></Card></div></TabsContent>
      <TabsContent value="debug"><div className="settings-stack"><Card className="debug-settings-card"><CardHeader><div className="icon-title"><Bug/><CardTitle>{t('debug')}</CardTitle></div></CardHeader><CardContent>
            <div className="switch-stack"><div className="switch-row"><div><Label htmlFor="debug-mode">{t('debugMode')}</Label><p className="help">{t('debugHelp')}</p></div><Switch id="debug-mode" checked={settings.debug} onCheckedChange={debug=>patch({debug})}/></div><div className="switch-row"><div><Label htmlFor="save-recognition-debug">{t('saveRecognitionDebug')}</Label><p className="help">{t('saveRecognitionDebugHelp')}</p></div><Switch id="save-recognition-debug" checked={settings.saveRecognitionDebug} onCheckedChange={saveRecognitionDebug=>patch({saveRecognitionDebug})}/></div></div>
          </CardContent></Card></div>{settings.debug&&<div className="debug-grid"><Card className="wide"><CardHeader><CardTitle>{t('hideoutLogCategory')}</CardTitle><CardDescription>{t('hideoutDiagnosticsHelp')}</CardDescription></CardHeader><CardContent><Button type="button" variant="secondary" onClick={()=>void openFolder(OpenHideoutDiagnostics)}><FolderOpen/>{t('hideoutDiagnostics')}</Button>{status.hideout?.lastError&&<p className="error">{t('hideoutSaveFailed')}</p>}</CardContent></Card><Card><CardHeader><CardTitle>{status.screenshotType==='item'?t('itemCandidates'):t('candidates')}</CardTitle></CardHeader><CardContent>{status.screenshotType==='item'?<><ol className="candidates">{status.itemCandidates.map(item=><li key={item.id}><span><b>{item.name}</b><small>{item.shortName}</small></span><Badge>{Math.round(item.confidence*100)}%</Badge></li>)}</ol>{status.itemUrl&&<Button variant="secondary" onClick={()=>BrowserOpenURL(status.itemUrl)}><ExternalLink/>{t('openItemTarkovDev')}</Button>}</>:<><ol className="candidates">{status.questCandidates.map(q=><li key={q.id}><span><b>{q.name}</b><small>{q.trader} · {q.map||t('anyMap')}</small></span><Badge>{Math.round(q.confidence*100)}%</Badge></li>)}</ol>{status.questUrl&&<Button variant="secondary" onClick={()=>void openFolder(OpenQuestPage)}><ExternalLink/>{t('openQuestPage')}</Button>}</>}</CardContent></Card><Card><CardHeader><CardTitle>{status.screenshotType==='item'?t('itemCropTitle'):t('cropTitle')}</CardTitle></CardHeader><CardContent>{status.cropPreview?<img className="crop" src={status.cropPreview} alt={status.screenshotType==='item'?t('itemCropTitle'):t('cropTitle')}/>:<p className="empty">{t('noImage')}</p>}</CardContent></Card>{status.screenshotType!=='item'&&<Card className="wide"><CardHeader><CardTitle>{t('objectives')}</CardTitle></CardHeader><CardContent><ul className="objectives">{status.questObjectives.map((o,i)=><li key={i}>{o.description}<small>{o.maps.join(', ')}</small></li>)}</ul></CardContent></Card>}</div>}</TabsContent>
    </Tabs>
    {toasts}
  </main>
}

function Metric({icon,label,value}:{icon:React.ReactNode;label:string;value:string}){return <Card><CardContent className="metric"><span>{icon}</span><div><small>{label}</small><strong>{value}</strong></div></CardContent></Card>}

createRoot(document.getElementById('root')!).render(<App/>)
