import { Check, CloudSync, ExternalLink, History, KeyRound, RefreshCw, X } from 'lucide-react'
import { BrowserOpenURL, DiscoverTrackerProfiles, ImportTrackerToken, RemoveTrackerKey, SetTrackerProfileKey, SyncTrackerProfileHistory } from './desktop'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { Label } from './components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import { Badge } from './components/ui/badge'
import { Switch } from './components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './components/ui/select'
import { Settings, TrackerKey, TrackerProfile, Status } from './settings-model'
import { MessageKey } from './i18n'
import { Dispatch, SetStateAction } from 'react'

type Props={settings:Settings;status:Status;t:(key:MessageKey)=>string;busy:boolean;run:(action:()=>Promise<unknown>,success:string)=>Promise<void>;patch:(value:Partial<Settings>)=>void;setBusy:(busy:boolean)=>void;setNotice:(notice:string)=>void;setNoticeError:(error:boolean)=>void;trackerModeLabel:(mode:string)=>string;trackerToken:string;setTrackerToken:Dispatch<SetStateAction<string>>}

// The TarkovTracker section: the API keys and the profiles they are assigned
// to, and each assigned profile's sync of its past logs.
export function TrackerSection({settings,status,t,busy,run,patch,setBusy,setNotice,setNoticeError,trackerModeLabel,trackerToken,setTrackerToken}:Props){
  const importTrackerKey=async()=>{setBusy(true);setNotice('');setNoticeError(false);try{const assigned=await ImportTrackerToken(trackerToken);setTrackerToken('');setNotice(assigned?t('trackerTokenAssigned').replace('{profile}',assigned):t('trackerTokenSaved'))}catch(error){setNotice(String(error));setNoticeError(true)}finally{setBusy(false)}}
  // The profiles by mode (PvP, Season and PvE are played side by side, so
  // each mode has a profile of its own). Of an account's profiles of a
  // mode, the latest is the one in use; the earlier ones are past wipes the
  // logs still hold, shown folded. Then the keys not on any profile.
  const shortProfile=(id:string)=>`${id.slice(0,6)}…${id.slice(-4)}`
  const seenDate=(iso:string)=>new Date(iso).toLocaleDateString(settings.language==='ja'?'ja-JP':'en-US')
  const modeOrder=['pvp','seasonal','pve']
  const byLastSeen=(a:TrackerProfile,b:TrackerProfile)=>Date.parse(b.lastSeen)-Date.parse(a.lastSeen)
  const profileGroups=[...new Set([...modeOrder,...status.tracker.profiles.map(p=>p.mode)])].map(mode=>{const seen=new Set<string>();const latest:TrackerProfile[]=[];const older:TrackerProfile[]=[];for(const profile of status.tracker.profiles.filter(p=>p.mode===mode).sort(byLastSeen)){if(seen.has(profile.accountId))older.push(profile);else{seen.add(profile.accountId);latest.push(profile)}}return {mode,latest,older}}).filter(group=>group.latest.length>0)
  const latestProfiles=profileGroups.flatMap(group=>group.latest)
  const unassignedKeys=status.tracker.keys.filter(key=>!key.bound)
  const profileRow=(profile:TrackerProfile,latest:boolean)=>{const key=status.tracker.keys.find(k=>k.id===profile.boundKeyId);const choices=status.tracker.keys.filter(k=>k.mode===profile.mode&&(!k.bound||k.id===profile.boundKeyId));return <div className={`tracker-profile${profile.current?' current':''}`} key={profileKey(profile)}><div className="tracker-profile-info"><span>{latest&&<Badge className="current">{t('trackerLatest')}</Badge>}{!key&&<Badge className="warn">{t('trackerUnassigned')}</Badge>}</span><strong>{t('trackerAccount')} {profile.accountId}</strong><small>{t('trackerProfile')} {shortProfile(profile.profileId)} · {t('trackerLastSeen')} {seenDate(profile.lastSeen)}</small></div><div className="tracker-profile-key">{key?<span className="tracker-key-chip"><KeyRound/><strong>{key.name||trackerModeLabel(key.mode)}</strong><code>{key.maskedToken}</code></span>:choices.length===0?<small className="tracker-no-key">{t('trackerNoKeyForMode')}</small>:null}<div className="tracker-profile-actions">{choices.length>0&&<Select value={profile.boundKeyId||'__none'} onValueChange={value=>void assignTrackerKey(profile,value)}><SelectTrigger aria-label={t('trackerAssignedKey')} className="tracker-key-select"><SelectValue placeholder={t('trackerChooseKey')}/></SelectTrigger><SelectContent><SelectItem value="__none">{t('trackerUnassigned')}</SelectItem>{choices.map(k=><SelectItem key={k.id} value={k.id}>{k.name?`${k.name} · ${k.maskedToken}`:k.maskedToken}</SelectItem>)}</SelectContent></Select>}</div></div><div className="tracker-profile-tools">{key&&<Button type="button" variant="ghost" disabled={busy} title={t('trackerSyncPastLogsHelp')} onClick={()=>void syncProfileHistory(profile)}><History/>{t('trackerSyncPastLogs')}</Button>}</div></div>}
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
  // Sends what the profile's EFT logs recorded to its key, from the first
  // session on (SyncTrackerProfileHistory).
  const syncProfileHistory=async(profile:TrackerProfile)=>{setBusy(true);setNotice('');setNoticeError(false);try{const sent=await SyncTrackerProfileHistory(profile.accountId,profile.profileId,profile.mode);setNotice(t('trackerHistorySynced').replace('{mode}',trackerModeLabel(profile.mode)).replace('{n}',String(sent)))}catch(error){setNotice(String(error));setNoticeError(true)}finally{setBusy(false)}}
  return <>
          <Card className="tracker-card"><CardHeader><div className="card-title-actions"><div className="icon-title"><CloudSync/><div><CardTitle>{t('trackerTitle')}</CardTitle><CardDescription>{t('trackerDescription')}</CardDescription></div></div><Button type="button" variant="secondary" onClick={scanTrackerProfiles} disabled={busy||!settings.logsDirectory}><RefreshCw/>{t('trackerScanLogs')}</Button></div></CardHeader><CardContent>
            <div className="switch-row"><div><Label htmlFor="tracker-enabled">{t('trackerEnable')}</Label><p className="help">{t('trackerEnableHelp')}</p></div><Switch id="tracker-enabled" checked={settings.tarkovTrackerEnabled} onCheckedChange={tarkovTrackerEnabled=>patch({tarkovTrackerEnabled})}/></div>
            {settings.tarkovTrackerEnabled&&<div className="tracker-modes">
              <section className="tracker-step"><div className="tracker-step-heading"><span className="step-pill">{t('trackerStep').replace('{n}','1')}</span><div><Label>{t('trackerKeysTitle')}</Label><p className="help">{t('trackerKeysHelp')}</p></div></div>
              <div className="tracker-import"><div><Label htmlFor="tracker-token">{t('trackerTokenLabel')}</Label><div className="secret-input"><KeyRound/><Input id="tracker-token" type="password" value={trackerToken} onChange={event=>setTrackerToken(event.target.value)} placeholder="PVP_… / SZN_… / PVE_…" autoComplete="off"/></div></div><Button type="button" variant="secondary" disabled={!trackerToken.trim()||busy} onClick={()=>void importTrackerKey()}>{t('trackerImportToken')}</Button></div>
              {unassignedKeys.length>0&&<section className="tracker-unassigned"><div><Label>{t('trackerUnassignedKeys')}</Label><p className="help">{t('trackerUnassignedKeysHelp')}</p></div>{unassignedKeys.map(key=>{const latestFree=latestProfiles.filter(p=>p.mode===key.mode&&!p.boundKeyId);const targets=[...latestFree,...(profileGroups.find(g=>g.mode===key.mode)?.older.filter(p=>!p.boundKeyId)??[])];const current=latestFree.length===1?latestFree[0]:latestFree.find(p=>p.current);return <div className="tracker-unassigned-key" key={key.id}><span className="tracker-key-chip"><KeyRound/><strong>{key.name||trackerModeLabel(key.mode)}</strong><code>{key.maskedToken}</code><Badge>{trackerModeLabel(key.mode)}</Badge></span><div className="tracker-profile-actions">{current&&<Button type="button" size="sm" variant="secondary" disabled={busy} onClick={()=>void assignTrackerKey(current,key.id)}><Check/>{t('trackerAssignToLatest')}</Button>}{targets.length>0?<Select value="__none" onValueChange={value=>void assignKeyToProfile(key,value)}><SelectTrigger aria-label={t('trackerAssignTo')} className="tracker-key-select"><SelectValue placeholder={t('trackerChooseProfile')}/></SelectTrigger><SelectContent>{targets.map(p=><SelectItem key={profileKey(p)} value={profileKey(p)}>{profileLabel(p)}</SelectItem>)}</SelectContent></Select>:<small className="tracker-no-key">{t('trackerNoProfileForMode')}</small>}<Button type="button" variant="ghost" disabled={busy} aria-label={t('trackerRemoveToken')} title={t('trackerRemoveToken')} onClick={()=>void removeTrackerKey(key.id)}><X/></Button></div></div>})}</section>}
              {status.tracker.keys.length>0&&<details className="tracker-manage"><summary>{t('trackerManageKeys')}</summary><p className="help">{t('trackerKeyNamesHelp')}</p><div className="tracker-manage-list">{status.tracker.keys.map(key=><div className="tracker-manage-key" key={key.id}><span className="tracker-key-chip"><KeyRound/><strong>{key.name||trackerModeLabel(key.mode)}</strong><code>{key.maskedToken}</code><Badge>{trackerModeLabel(key.mode)}</Badge></span><small>{key.bound?`${t('trackerAccount')} ${key.accountId} · ${t('trackerProfile')} ${shortProfile(key.profileId)}`:t('trackerUnassigned')}</small><Button type="button" variant="ghost" disabled={busy||key.bound} aria-label={t('trackerRemoveToken')} title={key.bound?t('trackerUnassignBeforeRemove'):t('trackerRemoveToken')} onClick={()=>void removeTrackerKey(key.id)}><X/></Button></div>)}</div></details>}
              </section>
              <section className="tracker-step"><div className="tracker-step-heading"><span className="step-pill">{t('trackerStep').replace('{n}','2')}</span><div><Label>{t('trackerProfilesTitle')}</Label><p className="help">{t('trackerProfilesHelp')}</p></div></div>
              <div className="tracker-profiles">{profileGroups.length===0?<p className="empty">{t('trackerNoProfiles')}</p>:profileGroups.map(group=><section className="tracker-mode-group" key={group.mode}><header className="tracker-group-heading"><strong>{trackerModeLabel(group.mode)}</strong><small>{t('trackerModeProfiles').replace('{n}',String(group.latest.length))}</small></header><div className="tracker-group-columns" aria-hidden="true"><span>{t('trackerProfile')}</span><span>{t('trackerKeyColumn')}</span><span/></div>{group.latest.map(profile=>profileRow(profile,true))}{group.older.some(p=>p.boundKeyId)&&<details className="tracker-older"><summary>{t('trackerOlderProfiles')}</summary>{group.older.filter(p=>p.boundKeyId).map(profile=>profileRow(profile,false))}</details>}</section>)}</div>
              </section>
            </div>}
            <p className="help"><button type="button" className="inline-link" onClick={()=>BrowserOpenURL('https://tarkovtracker.org/settings#api')}><ExternalLink/>{t('trackerOpenSettings')}</button></p>
          </CardContent></Card>
  </>
}
