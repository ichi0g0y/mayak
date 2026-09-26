import * as AppService from '../../bindings/github.com/local/mayak/internal/app/app';
import {Events,Clipboard} from '@wailsio/runtime';
import {itemInfo,clampItemPanel,clampItemPanelHeight,historyPoints,names} from './item.js';
import {browserSections,hostSections,mapTabID,randomUUID,bookmarkGroup,clampSidebar,rememberFavicon,hostname,defaults,restore,webURL,pageURL,receiveTask,receiveMap,receivePosition,translatedURL,originalURL,isTranslated,moveTab,togglePin,pinBookmark,bookmarkTab,goHome,openLocal,tabAt,cycleTab} from './state.js';
import {encode,decode,iceServers,PAIR_RELAY} from './peer-code.js';
import {t} from './words.js';
import './transport.js';

let state,go,platform,returnTo='',remoteID='',host=null,hostQuestSite='tarkov-dev',popup=null,item=null,restoredItem=null,itemOpen=false,itemSearch={query:'',results:[]},searchSeq=0,itemBusy=false,itemHistory=null,notify=()=>{},onKey=()=>{},section='appearance',error='',peerState={phase:'idle'},invite=null;
// Tabs closed in this session, newest last, for Ctrl+Shift+T (not saved).
const closedTabs=[];
let queue=Promise.resolve(),nativeQueue=Promise.resolve(),expiry;
// While the shell shows an overlay (the tutorial), the native page views stay
// hidden whatever else asks to show them; closing it shows the active tab again.
let overlay=false;
// The updater's state (model.UpdateStatus) drives the update bar: a status
// strip along the bottom of the whole window while a newer version is found,
// downloading or ready. The page views are native windows above the shell,
// so the bar takes its own row (bounds() lifts the pages by it) rather than
// floating over them, where it would be covered.
let updateStatus=null,updateDismissed='';
const updateBarHeight=32;
function updateBarVisible(){const u=updateStatus;return !!(u&&['available','downloading','ready'].includes(u.state)&&u.latest&&u.latest!==updateDismissed);}
const views=new Map();
// Tabs whose page is loading, by view ID (not saved).
const loadingViews=new Set();
// The game's screenshots (on the Host): newest first, their thumbnails and
// the full image being viewed, as data URLs from the Go side.
const shots={list:[],thumbs:{},full:{},viewing:''};
const thumbQueue=[];let thumbLoading=false;
// Bosses of each map and the Goons reports (json.tarkov.dev, through the Go
// side), asked again every few minutes: unchanged data costs a small request.
let bosses=null,bossesLoading=false,bossesAgain=false;
// The Goons report being written: what it can be about (the raid and the
// accounts seen in the logs), and how sending it went.
let goonReport=null;
const bossesEvery=5*60*1000;
// Site icons as data URLs from the Host's icon cache, by icon URL: shown at
// once and offline, and replaced when a page reports a changed icon.
const faviconData={};
function loadFavicon(url,refresh=false){
 if(!webURL(url))return;
 go.BrowserFavicon(url,refresh).then(data=>{if(typeof data==='string'&&data.startsWith('data:image/')&&faviconData[url]!==data){faviconData[url]=data;update();}}).catch(()=>{});
}

const snapshot=()=>({...state,update:updateStatus,updateBar:updateBarVisible(),statusRows:statusRows(),goonReport,bosses:bosses&&{...bosses,current:host?.map||''},screenshots:shotsAvailable()?{list:shots.list,thumbs:shots.thumbs,viewing:shots.viewing,full:shots.full[shots.viewing]||''}:null,loadingTabs:[...loadingViews],popup:popup&&{key:popup.key},faviconData,host:state.connection.mode==='local'?host:null,hostQuestSite,item,itemOpen,itemSearch,itemBusy,itemHistory,settingsSection:section,localHost:platform==='windows',connectionStatus:state.connection.mode==='local'?'connected':state.connection.mode==='off'?'off':peerState.phase==='connected'?'connected':'disconnected',peer:peerState,error});
const update=()=>notify(snapshot());
// An error shows as a strip along the bottom, in a row of its own like the
// update bar (see there): a toast over the page would be under it. The row
// appears when the first error arrives; a failure to show the pages after
// that is not retried, or it would loop.
const messageError=e=>{const shown=!!error;error=t(state.language,'actionFailed')+String(e?.message||e);update();if(!shown)void show().catch(()=>{});};
const native=(command,o)=>{const result=nativeQueue.then(()=>go.BrowserView(command,o));nativeQueue=result.catch(()=>{});return result;};
// Every page starts below the toolbar row; fixed views show it without an
// address bar (reload and "open in browser" only).
// The item sidebar keeps its width free on the right while it is open.
// The page area: the window less the tab sidebar (left, right, or the top
// strip) and the item panel (left, right or bottom); both may share a side.
function bounds(){
 const nav=state.layout==='horizontal'?0:state.sidebarCollapsed?52:state.sidebarWidth;
 const dock=itemOpen?state.itemDock:'';
 const item=dock==='bottom'?0:state.itemPanelWidth;
 return {
  left:(state.sidebarSide==='left'?nav:0)+(dock==='left'?item:0),
  top:state.layout==='horizontal'?96:48,
  right:(state.sidebarSide==='right'?nav:0)+(dock==='right'?item:0),
  bottom:(dock==='bottom'?state.itemPanelHeight:0)+statusRows()*updateBarHeight,
 };
}
// The rows along the bottom that the pages make room for: the update bar and
// the error strip. The shell lays them out from the same count.
const statusRows=()=>(updateBarVisible()?1:0)+(error?1:0);
// The map view opens with the Host's Remote Control ID, so tarkov.dev connects
// on load and follows map/position commands itself.
function viewURL(tab){
 if(tab.id!==mapTabID||!remoteID)return tab.url;
 const url=new URL(tab.url),p=url.pathname;if(url.hostname!=='tarkov.dev'||!(p.startsWith('/map/')||p==='/maps'||p==='/maps/'))return tab.url;
 url.searchParams.set('connection',remoteID);return url.href;
}
// The theme background fills a tab until its page paints, instead of white.
const pageBackground=()=>getComputedStyle(document.documentElement).getPropertyValue('--bg').trim();
// Pages opened from the item sidebar show in a popup window of their own
// (see app_popup.go), which opens beside the sidebar, level with the row
// clicked (anchor, a y in this window).
const popupID='popup',popupGap=8,popupMargin=12;
function popupPlace(){
 const page=bounds(),width=innerWidth,height=innerHeight;
 const w=Math.round(Math.max(320,Math.min(960,width-page.left-page.right-popupGap-popupMargin)));
 const h=Math.round(Math.max(240,Math.min(800,height-page.top-page.bottom-popupMargin*2)));
 const top=Math.round(Math.min(Math.max(page.top+popupMargin,(popup?.anchor||0)-160),height-page.bottom-popupMargin-h));
 return {x:state.itemDock==='left'?page.left+popupGap:width-page.right-popupGap-w,y:top,width:w,height:h};
}
// popupClosed is the popup the window closed by itself (it lost the focus),
// so the click that took the focus does not open it again straight away.
let popupClosed={key:'',at:0};
async function closePopup(){
 popup=null;update();
 await go.BrowserPopupClose().catch(()=>{});
}
// openPopup shows a page in the popup; the same page again closes it.
async function openPopup(next){
 if(popup&&popup.key===next.key){await closePopup();return;}
 if(!popup&&popupClosed.key===next.key&&Date.now()-popupClosed.at<500)return;
 popup={...next,openedAt:Date.now()};update();
 await go.BrowserPopupShow({url:next.url,title:next.title,task:!!next.task,background:pageBackground(),theme:document.documentElement.dataset.theme||'',language:state.language},popupPlace());
}
async function show(){
 const tab=state.tabs.find(t=>t.id===state.active);
 if(overlay||tab?.kind!=='web'){await native('hideAll',{id:'shell'});return;}
 const previous=views.get(tab.id);
 await native('show',{id:tab.id,url:viewURL(tab),...bounds(tab),background:pageBackground()});
 if(previous&&previous!==tab.url)await native('navigate',{id:tab.id,url:viewURL(tab)});
 views.set(tab.id,tab.url);
}
async function persist(){
 // Construct the persisted representation explicitly. RTC codes and UI errors
 // never enter Go storage. Existing Host credentials stay in their own store.
 // Until a restored item has loaded, the saved one is kept.
 const shown=item||restoredItem;
 const itemPanel={open:itemOpen,id:shown?.id||'',mode:shown?.mode||''};
 const {version,bookmarkRevision,language,tutorialDone,clock,layout,sidebarSide,sidebarCollapsed,bookmarksCollapsed,screenshotsCollapsed,bossesView,bossMap,bossMode,sidebarWidth,itemPanelWidth,itemPanelHeight,itemDock,bookmarkView,favicons,theme,adblock,taskMode,questSite,translateWiki,bookmarks,tabs,active}=state;
 await go.BrowserSave(JSON.stringify({version,bookmarkRevision,language,tutorialDone,clock,layout,sidebarSide,sidebarCollapsed,bookmarksCollapsed,screenshotsCollapsed,bossesView,bossMap,bossMode,sidebarWidth,itemPanelWidth,itemPanelHeight,itemDock,itemPanel,bookmarkView,favicons,theme,adblock,taskMode,questSite,translateWiki,bookmarks,tabs,active,connection:{mode:state.connection.mode,stun:state.connection.stun}}));
}
async function changed(){update();await persist();void show().catch(messageError);}
function enqueue(fn){const result=queue.then(fn);queue=result.catch(messageError);return result.catch(()=>snapshot());}
function display(message,remote=false){
 return enqueue(async()=>{
  if(remote?state.connection.mode!=='webrtc':state.connection.mode!=='local')return;
  if(message.event==='browser:item'){const next=itemInfo(message.args[0]);if(!next)return;item=next;itemOpen=true;loadHistory();void persist().catch(()=>{});if(!remote)peer.send(message);update();await show();return;}
  // The last detection decides the tab shown: a task its page, a position
  // (after a task, say) the map view again.
  const tab=message.event==='browser:task'?receiveTask(state,message.args[0]):message.event==='browser:map'?receiveMap(state,message.args[0]):message.event==='browser:position'?receivePosition(state,message.args[0]):null;
  if(tab){if(!remote)peer.send(message);await changed();}
 });
}
// hostStatus keeps what the sidebar shows of a (possibly partial) Host status.
function hostStatus(s){
 const out={};if(!s||typeof s!=='object')return out;
 if('monitoring' in s)out.monitoring=!!s.monitoring;
 if('currentMap' in s)out.map=String(s.currentMap||'');
 if('raidActive' in s)out.raid=!!s.raidActive;
 if(s.tracker&&typeof s.tracker==='object'){out.tracker=String(s.tracker.connection||'');out.mode=String(s.tracker.mode||'');out.identity=[s.tracker.accountId,s.tracker.profileId,s.tracker.mode].map(v=>String(v||'')).join('|');}
 return out;
}
// The task site for pages opened from the item sidebar: the Host's current
// setting on the Host, else this browser's choice or the one the item came with.
function itemSite(){
 if(state.connection.mode==='local')return hostQuestSite;
 return state.questSite==='host'?item?.questSite||'tarkov-dev':state.questSite;
}
// Views are placed in whole pixels.
const anchorY=data=>Math.round(Number.isFinite(data?.anchor)?data.anchor:innerHeight/2);
// loadHistory fetches the price history of the current item for its chart,
// once per item unless again is set (the refresh button).
// Screenshots are the Host's: only there are they listed.
const shotsAvailable=()=>platform==='windows'&&state?.connection.mode==='local';
// What the Host recognized a screenshot as (see screenshotRecord in Go).
function shotMeta(m){
 if(!m||typeof m!=='object')return null;
 const text=(v,max=200)=>typeof v==='string'?v.slice(0,max):'';
 const num=v=>Number.isFinite(v)?v:0;
 return {type:['tasks','item','position'].includes(m.type)?m.type:'unknown',layout:text(m.layout,60),score:num(m.score),map:text(m.map,60),raid:m.raid===true,stage:text(m.stage),match:text(m.match),detail:text(m.detail,80),confidence:num(m.confidence),candidates:Array.isArray(m.candidates)?m.candidates.slice(0,3).map(c=>text(c)):[],ocr:text(m.ocr,500),position:text(m.position,80),error:text(m.error,300)};
}
async function loadShots(){
 if(!shotsAvailable())return;
 try{const list=await go.BrowserScreenshots(300);shots.list=(Array.isArray(list)?list:[]).filter(s=>s&&typeof s.name==='string').map(s=>({name:s.name,time:String(s.time||''),meta:shotMeta(s.meta)}));}catch{return;}
 if(shots.list[0])queueThumb(shots.list[0].name,true);
 // The screenshot page (also when restored open) shows them all.
 if(state.tabs.find(t=>t.id===state.active)?.kind==='screenshots')for(const s of shots.list.slice(0,120))queueThumb(s.name);
 update();
}
// Thumbnails load one at a time, the sidebar's latest first.
function queueThumb(name,first=false){
 if(shots.thumbs[name]||thumbQueue.includes(name))return;
 if(first)thumbQueue.unshift(name);else thumbQueue.push(name);
 void nextThumb();
}
async function nextThumb(){
 if(thumbLoading||!thumbQueue.length)return;
 thumbLoading=true;const name=thumbQueue.shift();
 try{const data=await go.BrowserScreenshotImage(name,true);if(typeof data==='string'&&data.startsWith('data:image/jpeg;base64,'))shots.thumbs[name]=data;update();}catch{}
 thumbLoading=false;void nextThumb();
}
// The full image of the one viewed, and of its neighbours for paging.
async function viewShot(name){
 shots.viewing=name;update();
 if(!name)return;
 const index=shots.list.findIndex(s=>s.name===name);
 const wanted=[name,shots.list[index+1]?.name,shots.list[index-1]?.name].filter(Boolean);
 for(const key of Object.keys(shots.full))if(!wanted.includes(key))delete shots.full[key];
 for(const want of wanted){
  if(shots.full[want])continue;
  try{const data=await go.BrowserScreenshotImage(want,false);if(typeof data==='string'&&data.startsWith('data:image/jpeg;base64,'))shots.full[want]=data;}catch{}
  if(want===name)update();
 }
}
// The action queue is serial: everything a click asks for waits behind what
// is already in it. Fetches from the network never go into it, then, or a
// slow or failing connection (a 45 s timeout per item lookup) would hold
// every click for minutes: the sidebar looks dead while the pages, in their
// own processes, still respond. These run on their own and update the item
// when they return, if it is still the one shown.
async function refreshItem(auto){
 if(!item||itemBusy)return;
 itemBusy=true;update();
 // The spinner stays long enough to be seen: a cached answer comes back at once.
 const started=Date.now(),id=item.id;
 try{const next=itemInfo(await go.BrowserItemInfo(item.mode,id));if(next&&next.id===item?.id)item=next;}
 catch(e){if(!auto)messageError(e);}
 finally{const wait=600-(Date.now()-started);if(wait>0)await new Promise(resolve=>setTimeout(resolve,wait));itemBusy=false;}
 void loadHistory(true);
 update();
}
async function reloadItem(){
 if(!item)return;
 try{const next=itemInfo(await go.BrowserItemInfo('',item.id));if(next&&next.id===item.id){item=next;void loadHistory();void persist().catch(()=>{});update();}}catch{}
}
// A reload asked for while one runs follows it: the game mode may have become
// known meanwhile, and the running one fetched the other mode's reports.
async function loadBosses(){
 if(bossesLoading){bossesAgain=true;return;}
 bossesLoading=true;
 try{const info=await go.BrowserBosses(state.bossMode,state.language);if(info&&Array.isArray(info.maps))bosses={mode:String(info.mode||''),fetched:String(info.fetched||''),maps:info.maps,goons:Array.isArray(info.goons)?info.goons:[]};}catch{}
 bossesLoading=false;update();
 if(bossesAgain){bossesAgain=false;void loadBosses();}
}
async function loadHistory(again=false){
 const {id,mode}=item||{};if(!id)return;
 if(!again&&itemHistory?.id===id&&itemHistory.mode===mode)return;
 if(!itemHistory||itemHistory.id!==id||itemHistory.mode!==mode)itemHistory={id,mode,points:[],loading:true};
 update();
 let next;
 try{next={id,mode,points:historyPoints(await go.BrowserItemHistory(mode,id))};}catch{next={id,mode,points:itemHistory?.points||[],failed:true};}
 if(item?.id===id&&item.mode===mode){itemHistory=next;update();}
}
// The pairing relay: the Host parks its invitation under an 8-digit code and
// polls for the answer; the other PC fetches the invitation by that code and
// posts its answer. The relay is optional: the long codes can still be
// exchanged by hand, and a relay error leaves that path open.
async function relay(method,path,body){
 const res=await fetch(PAIR_RELAY+path,{method,headers:body?{'content-type':'application/json'}:undefined,body:body?JSON.stringify(body):undefined,cache:'no-store'});
 if(res.status===204)return null;
 if(!res.ok)throw new Error(res.status===404?'pair-code-not-found':'relay-unavailable');
 return res.json();
}
let pairPoll=0;
function stopPairPoll(){clearInterval(pairPoll);pairPoll=0;}
function forgetPairCode(){const code=peerState.pairCode;if(code)void relay('DELETE','/'+code).catch(()=>{});}
function startPairPoll(code){
 stopPairPoll();
 pairPoll=setInterval(async()=>{
  if(peerState.pairCode!==code||peerState.phase!=='waiting-answer'){stopPairPoll();return;}
  try{const got=await relay('GET','/'+code+'/answer');if(got?.answer){stopPairPoll();await pairing('peerAnswer',got.answer);}}catch{}
 },2000);
}
const peer=new globalThis.MayakPeer({onState:next=>{peerState={...peerState,...next,busy:false};if(next.phase==='connected'){peerState.code='';stopPairPoll();forgetPairCode();clearTimeout(expiry);}update();},onMessage:message=>void display(message,true)});
function closePeer(){stopPairPoll();forgetPairCode();clearTimeout(expiry);invite=null;peer.close();peerState={phase:'idle'};update();}
async function pairing(type,data){
 if(peerState.busy)return snapshot();
 if(type==='peerClose'){closePeer();return snapshot();}
 if(type==='peerCopy'){await Clipboard.SetText(peerState.code||'');return snapshot();}
 if(type==='peerCopyPair'){await Clipboard.SetText(peerState.pairCode||'');return snapshot();}
 const pairCode=type==='peerAnswer'?peerState.pairCode||'':'';
 peerState={...peerState,busy:true,phase:type==='peerAnswer'?'connecting':'gathering',code:'',pairCode,relayError:''};update();
 try{
  if(type==='peerInvite'){
   if(platform!=='windows'||state.connection.mode!=='local')throw new Error('host-required');
   const sdp=await peer.offer({iceServers:iceServers(state.connection.stun)});
   invite={version:1,type:'offer',id:randomUUID(),createdAt:Date.now(),sdp};
   peerState={role:'sender',phase:'waiting-answer',code:encode(invite)};
   try{const posted=await relay('POST','',{invite:peerState.code});peerState.pairCode=posted.code;startPairPoll(posted.code);}
   catch(e){peerState.relayError=String(e?.message||e);}
  }else if(type==='peerJoin'){
   if(state.connection.mode!=='webrtc')throw new Error('receiver-required');
   const digits=String(data||'').replace(/\D/g,'');if(digits.length!==8)throw new Error('pair-code-invalid');
   const fetched=await relay('GET','/'+digits);const offer=decode(fetched?.invite);if(offer.type!=='offer')throw new Error('offer-required');
   const sdp=await peer.answer({iceServers:iceServers(state.connection.stun),sdp:offer.sdp});
   const answer=encode({...offer,type:'answer',sdp});await relay('PUT','/'+digits,{answer});
   invite=offer;peerState={role:'receiver',phase:'waiting-host',code:answer,pairCode:digits};
  }else if(type==='peerAccept'){
   if(state.connection.mode!=='webrtc')throw new Error('receiver-required');
   const offer=decode(data);if(offer.type!=='offer')throw new Error('offer-required');
   const sdp=await peer.answer({iceServers:iceServers(state.connection.stun),sdp:offer.sdp});
   invite=offer;peerState={role:'receiver',phase:'waiting-host',code:encode({...offer,type:'answer',sdp})};
  }else if(type==='peerAnswer'){
   const answer=decode(data);if(!invite||answer.type!=='answer'||answer.id!==invite.id||answer.createdAt!==invite.createdAt)throw new Error('answer-mismatch');
   await peer.acceptAnswer({sdp:answer.sdp});peerState={role:'sender',phase:'connecting',pairCode};
  }
  clearTimeout(expiry);if(invite)expiry=setTimeout(()=>{if(peerState.phase!=='connected'){peer.close();peerState={phase:'failed',reason:'invite-expired'};update();}},Math.max(0,600000-(Date.now()-invite.createdAt)));
 }catch(e){stopPairPoll();peer.close();peerState={phase:'failed'};messageError(e);}
 update();return snapshot();
}
const ready=(async()=>{
 go=AppService;platform=await go.BrowserPlatform();
 try{remoteID=await go.BrowserRemoteID();}catch{}
 let migrated=false;
 try{const saved=JSON.parse(await go.BrowserLoad());state=restore(saved);migrated=saved.bookmarkRevision!==state.bookmarkRevision;}catch(e){state=defaults();error=String(e);}
 if(migrated)try{await persist();}catch(e){error=String(e);}
 // The item sidebar comes back as it was, with current prices for its item.
 itemOpen=state.itemPanel.open;
 if(state.itemPanel.id){
  restoredItem=state.itemPanel;
  go.BrowserItemInfo(state.itemPanel.mode,state.itemPanel.id).then(raw=>{const next=itemInfo(raw);if(next&&!item){item=next;void loadHistory();update();}}).catch(()=>{});
 }
 if(!['local','webrtc','off'].includes(state.connection.mode))state.connection.mode='off';
 if(platform!=='windows'&&state.connection.mode==='local')state.connection.mode='webrtc';
 await go.BrowserSetMode(state.connection.mode);
 try{await go.BrowserSetAdblock(state.adblock);}catch(e){error=String(e);}
 // The trusted settings document shares only the trusted parent's runtime.
 // External websites are native views and never receive this object.
 window.mayakDesktop={backend:AppService,on:(name,callback)=>Events.On(name,event=>callback(event.data)),openURL:url=>void window.mayak.action('open',url)};

 window.mayakDesktop.on('browser:menu',event=>{if(event.action==='settings')void window.mayak.action('settings');else if(event.action==='bookmark'){const b=state.bookmarks.find(b=>b.id===event.id);if(b)void window.mayak.action('open',b.url);}});
 window.mayakDesktop.on('browser:task',task=>void display({event:'browser:task',args:[task]}));
 window.mayakDesktop.on('browser:map',map=>void display({event:'browser:map',args:[map]}));
 window.mayakDesktop.on('browser:position',map=>void display({event:'browser:position',args:[map]}));
 // The document script changed (the player marker style): a page keeps the
 // script it was created with, so the map view is created again.
 window.mayakDesktop.on('browser:document-script',()=>void enqueue(async()=>{
  const tab=state.tabs.find(t=>t.id===mapTabID);if(!tab||!views.has(tab.id))return;
  await native('close',{id:tab.id});views.delete(tab.id);loadingViews.delete(tab.id);
  if(state.active===tab.id)await show();else await preloadMap();
 }));
 // A shortcut pressed inside a page view (Ctrl+T and the like) is handled by
 // the shell, like one pressed in the shell itself.
 window.mayakDesktop.on('browser:key',key=>{if(key&&typeof key.key==='string')onKey({key:key.key,ctrl:!!key.ctrl,shift:!!key.shift,alt:!!key.alt});});
 // Another page used the map view's Remote ID, so Go replaced it: reopen the
 // map view with the new one (its tab keeps the old document script).
 window.mayakDesktop.on('browser:remote-id',id=>void enqueue(async()=>{
  if(typeof id!=='string'||!id)return;remoteID=id;
  const tab=state.tabs.find(t=>t.id===mapTabID);
  if(tab&&views.has(tab.id))await native('navigate',{id:tab.id,url:viewURL(tab)});
 }));
 // The Host's monitoring state for the sidebar's monitoring button.
 if(platform==='windows'){
  try{host=hostStatus(await go.GetStatus());hostQuestSite=(await go.GetSettings()).questSite||'tarkov-dev';}catch{}
  // On the Host the task site is one setting, the Host's; a site chosen in
  // the browser before becomes that setting once.
  if(state.connection.mode==='local'&&state.questSite!=='host'){try{const s=await go.GetSettings();s.questSite=state.questSite;await go.PersistSettings(s);hostQuestSite=state.questSite;state.questSite='host';await persist();}catch{}}
  // The game mode comes with TarkovTracker: the boss data follows it.
  // Another EFT account, profile or mode: the item shown is read again in the
  // mode now played, so its prices and progress are that profile's.
  window.mayakDesktop.on('status:update',next=>{const tracker=host?.tracker,mode=host?.mode,identity=host?.identity;host={...host,...hostStatus(next)};if(host.tracker!==tracker||host.mode!==mode)void loadBosses();if(identity!==undefined&&host.identity!==identity)void reloadItem();update();});
 }
 // The popup window closed by itself, or one of its buttons was pressed.
 window.mayakDesktop.on('popup:closed',()=>void enqueue(async()=>{if(!popup||Date.now()-popup.openedAt<300)return;popupClosed={key:popup.key,at:Date.now()};popup=null;update();}));
 window.mayakDesktop.on('popup:action',type=>void enqueue(async()=>{if(type==='toTab')await perform('popupToTab');else if(type==='close')await perform('popupClose');}));
 window.mayakDesktop.on('browser:item',info=>void display({event:'browser:item',args:[info]}));
 // The update toast follows the updater; a found, downloading or downloaded
 // version shows until it is applied or put off.
 // Progress arrives once per percent; the bar redraws at most a few times a second.
 let updateRedraw=0;
 const updateChanged=next=>{
  const shown=updateBarVisible(),before=updateStatus;updateStatus=next;
  const minor=before&&before.state===next?.state&&before.latest===next?.latest;
  if(minor){if(!updateRedraw)updateRedraw=setTimeout(()=>{updateRedraw=0;update();},250);}
  else{clearTimeout(updateRedraw);updateRedraw=0;update();}
  if(updateBarVisible()!==shown)void show();
 };
 window.mayakDesktop.on('update:status',updateChanged);
 go.GetUpdateStatus?.().then(updateChanged).catch(()=>{});
 // A new screenshot shows in the sidebar (and on the screenshot page).
 window.mayakDesktop.on('browser:screenshot',()=>void loadShots());
 void loadShots();
 void loadBosses();
 setInterval(()=>{if(document.visibilityState==='visible')void loadBosses();},bossesEvery);
 // A page's address and title show at once, not after the actions queued
 // before (an item reload at start-up waits for the network); only saving
 // them waits its turn.
 window.mayakDesktop.on('browser:navigation',event=>{
  if(event.popup){if(webURL(event.url))void enqueue(()=>perform('open',event.url));return;}
  if(event.id!==popupID&&!!event.loading!==loadingViews.has(event.id)){if(event.loading)loadingViews.add(event.id);else loadingViews.delete(event.id);update();}
  // Following links inside the popup keeps its title and address current.
  if(event.id===popupID){if(popup&&webURL(event.url)){popup.url=pageURL(event.url);popup.title=String(event.title||popup.title).slice(0,160);}return;}
  const tab=state.tabs.find(t=>t.id===event.id);if(!tab||!webURL(event.url))return;
  tab.url=pageURL(event.url);tab.title=String(event.title||tab.title||tab.url).slice(0,160);
  // A new page drops the old favicon until its own one is known.
  if(event.favicon){tab.favicon=webURL(event.favicon)||undefined;rememberFavicon(state,tab.url,tab.favicon);loadFavicon(tab.favicon,true);}else if(hostname(tab.faviconPage||'')!==hostname(tab.url))delete tab.favicon;
  tab.faviconPage=tab.url;tab.canBack=!!event.canBack;tab.canForward=!!event.canForward;
  views.set(tab.id,tab.url);update();void enqueue(persist);
 });
 for(const url of new Set([...Object.values(state.favicons),...state.tabs.map(t=>t.favicon)]))loadFavicon(url);
 void show().then(preloadMap).catch(messageError);return snapshot();
})();
// The fixed tarkov.dev map loads in the background at start-up, so the first
// switch to it (often triggered by a detection) shows a ready page.
async function preloadMap(){
 const tab=state.tabs.find(t=>t.id===mapTabID);
 if(!tab||views.has(tab.id))return;
 await native('preload',{id:tab.id,url:viewURL(tab),...bounds(tab),background:pageBackground()});
 views.set(tab.id,tab.url);
}
async function perform(type,data){
 const tab=state.tabs.find(t=>t.id===state.active);
 switch(type){
 case 'state':return snapshot();
 case 'newTab':{if(state.tabs.length>=80)throw new Error('tab-limit');const item={id:randomUUID(),kind:'blank'};state.tabs.push(item);state.active=item.id;break;}
 case 'menu':case 'settings':if(tab?.kind!=='settings')returnTo=state.active;openLocal(state,'settings');break;
 // Closing settings returns to the tab it was opened from, or TARKOV.DEV.
 case 'closeSettings':{const back=state.tabs.find(t=>t.id===returnTo&&t.kind!=='settings')||state.tabs.find(t=>t.id===mapTabID);if(back)state.active=back.id;break;}
 case 'bookmarks':openLocal(state,'bookmarks');break;
 case 'tabsPage':openLocal(state,'tabs');break;
 // The screenshot page: all of them (from the sidebar's heading), or one
 // shown large (from its thumbnail); screenshotView pages through them.
 case 'bosses':openLocal(state,'bosses');void loadBosses();break;
 case 'goonReportOpen':{
  // The raid and profile lookup may go to the network; it fills the dialog
  // when it returns rather than holding the queue.
  goonReport={raid:null,identities:[],busy:true,done:false,error:''};
  void (async()=>{
   let info=null,error='';
   try{info=await go.BrowserGoonReportInfo();}catch(e){error=String(e?.message||e);}
   if(!goonReport||goonReport.done)return;
   goonReport={raid:info?.raid||null,identities:Array.isArray(info?.identities)?info.identities:[],busy:false,done:false,error};update();
  })();
  return snapshot();
 }
 case 'goonReportClose':goonReport=null;return snapshot();
 case 'goonReportSend':{
  if(!goonReport||goonReport.busy)return snapshot();
  goonReport={...goonReport,busy:true,error:''};update();
  const report={map:String(data?.map||''),accountId:String(data?.accountId||''),mode:String(data?.mode||''),startedAt:String(data?.startedAt||'')};
  void (async()=>{
   try{
    await go.BrowserReportGoons(report);
    if(goonReport)goonReport={...goonReport,busy:false,done:true};
    // tarkov.dev adds reports to its data every ten minutes.
    setTimeout(()=>void loadBosses(),11*60*1000);
   }catch(e){if(goonReport)goonReport={...goonReport,busy:false,error:String(e?.message||e)};}
   update();
  })();
  return snapshot();
 }
 case 'screenshots':openLocal(state,'screenshots');shots.viewing='';void loadShots();break;
 case 'screenshotOpen':openLocal(state,'screenshots');await changed();void loadShots();void viewShot(String(data||''));return snapshot();
 case 'screenshotView':void viewShot(String(data||''));return snapshot();
 case 'screenshotFolder':try{await go.OpenScreenshotDirectory();}catch{}return snapshot();
 case 'home':goHome(state,tab?.id);break;
 case 'bookmarkTab':bookmarkTab(state,data?.id,data?.before??null);break;
 // A shell menu that reaches over the page hides the native page view meanwhile.
 case 'overlay':overlay=!!data;await show();return snapshot();
 case 'sidebarWidth':state.sidebarWidth=clampSidebar(data);await show();return snapshot();
 case 'itemPanelWidth':state.itemPanelWidth=clampItemPanel(data);await show();return snapshot();
 case 'itemPanelHeight':state.itemPanelHeight=clampItemPanelHeight(data);await show();return snapshot();
 case 'bookmarkPin':{const item=state.bookmarks.find(b=>b.id===(data?.id??data));if(item)pinBookmark(state,item.id,data?.pin??!item.sidebar,data?.before);break;}
 case 'settingsSection':if(data==='tasks'&&platform==='windows')try{hostQuestSite=(await go.GetSettings()).questSite||hostQuestSite;}catch{}
  if(browserSections.includes(data)||(platform==='windows'&&hostSections.includes(data)))section=data;break;
 case 'activate':if(state.tabs.some(t=>t.id===data))state.active=data;break;
 // openOrFocus goes to a tab already showing the address, if any, so a
 // link pressed twice (the changelog, About) does not open a second tab.
 case 'open':case 'navigate':case 'openOrFocus':{
  const url=webURL(data);if(!url)throw new Error(t(state.language,'enterWebURL'));
  if(type==='openOrFocus'){const same=state.tabs.find(t=>t.kind==='web'&&t.url===url);if(same){state.active=same.id;break;}}
  if(type==='navigate'&&tab&&['web','blank'].includes(tab.kind)){tab.kind='web';tab.url=url;delete tab.task;}
  else{if(state.tabs.length>=80)throw new Error('tab-limit');const item={id:randomUUID(),kind:'web',url};state.tabs.push(item);state.active=item.id;}break;
 }
 case 'close':{
  const index=state.tabs.findIndex(t=>t.id===data&&!t.pinned&&!t.fixed);if(index<0)break;
  await native('close',{id:data});views.delete(data);loadingViews.delete(data);const [closed]=state.tabs.splice(index,1);
  if(closed.kind==='web'){closedTabs.push({tab:closed,index});if(closedTabs.length>20)closedTabs.shift();}
  if(state.active===data)state.active=state.tabs[Math.min(index,state.tabs.length-1)]?.id;
  if(!state.tabs.length)state.active='';break;
 }
 // Ctrl+Shift+T: the last closed page comes back where it was, as a new tab.
 case 'reopenTab':{
  const last=closedTabs.pop();if(!last)return snapshot();
  if(state.tabs.length>=80)throw new Error('tab-limit');
  const next={...last.tab,id:randomUUID()};delete next.pinned;
  const before=state.tabs.slice(last.index).find(t=>!t.fixed)?.id??null;
  state.tabs.push(next);moveTab(state,next.id,before);state.active=next.id;break;
 }
 case 'cycleTab':if(!cycleTab(state,data<0?-1:1))return snapshot();break;
 case 'tabAt':if(!tabAt(state,Number(data)))return snapshot();break;
 // Keyboard focus moves to the shell (its address bar) or to the page shown;
 // the page views are native windows, so the shell cannot do it itself.
 case 'focus':if(data==='shell')await native('focus',{id:'shell'});else if(tab?.kind==='web')await native('focus',{id:tab.id});return snapshot();
 case 'pin':togglePin(state,data);break;
 case 'move':moveTab(state,data?.id,data?.before??null);break;
 case 'preferences':{
  const next=restore({...state,...data});state.language=next.language;state.tutorialDone=next.tutorialDone;state.layout=next.layout;state.sidebarSide=next.sidebarSide;state.theme=next.theme;state.clock=next.clock;state.sidebarCollapsed=next.sidebarCollapsed;state.bookmarksCollapsed=next.bookmarksCollapsed;state.screenshotsCollapsed=next.screenshotsCollapsed;state.bossesView=next.bossesView;state.bossMap=next.bossMap;state.bossMode=next.bossMode;if(data&&'bossMode' in data)void loadBosses();if(data&&'language' in data)void loadBosses();state.sidebarWidth=next.sidebarWidth;state.itemPanelWidth=next.itemPanelWidth;state.itemPanelHeight=next.itemPanelHeight;state.itemDock=next.itemDock;state.bookmarkView=next.bookmarkView;state.taskMode=next.taskMode;state.questSite=next.questSite;state.translateWiki=next.translateWiki;
  // Blocking applies to new requests; reload so the visible page matches the setting.
  if(next.adblock!==state.adblock){state.adblock=next.adblock;await go.BrowserSetAdblock(state.adblock);if(tab?.kind==='web')await native('reload',{id:tab.id});}
  break;
 }
 case 'connection':{
  if(data.mode!==undefined)await go.BrowserSetMode(data.mode);
  if(data.stun!==undefined){iceServers(data.stun);state.connection.stun=data.stun;}
  if(['local','webrtc','off'].includes(data.mode)&&(data.mode!=='local'||platform==='windows')){closePeer();state.connection.mode=data.mode;}break;
 }
 case 'bookmark':{
  const url=webURL(data.url);if(!url)throw new Error('invalid-url');const item={id:data.id||randomUUID(),name:String(data.name||url).slice(0,150),url,group:bookmarkGroup(data.group)};
  const index=state.bookmarks.findIndex(b=>b.id===item.id);if(index>=0){if(state.bookmarks[index].sidebar)item.sidebar=true;state.bookmarks[index]=item;}else if(state.bookmarks.length<100)state.bookmarks.push(item);break;
 }
 case 'deleteBookmark':state.bookmarks=state.bookmarks.filter(b=>b.id!==data);break;
 case 'site':if(tab?.task&&webURL(tab.task.urls[data]))tab.url=tab.task.urls[data];break;
 case 'back':case 'forward':case 'reload':if(tab?.kind==='web')await native(type,{id:tab.id});return snapshot();
 // The page opens through Google Translate's proxy, or back as itself.
 case 'translate':{
  if(tab?.kind!=='web'||tab.fixed)return snapshot();
  const url=isTranslated(tab.url)?originalURL(tab.url):translatedURL(tab.url,state.language);
  if(!url||url===tab.url)return snapshot();
  tab.url=url;break;
 }
 case 'windowTheme':try{await go.BrowserSetWindowTheme(data.caption,data.text,data.border,!!data.dark);}catch{}return snapshot();
 // A task in the item sidebar opens like a recognized task, on the task site
 // set now (on the Host its current setting, not the one when the item came).
 case 'itemTask':{
  const task=item?.tasks.find(t=>t.id===(data?.id??data));if(!task)return snapshot();
  const site=itemSite(),url=webURL(task.urls[site]);if(!url)return snapshot();
  await openPopup({key:'task:'+task.id,url,title:task.name,anchor:anchorY(data),task:{id:task.id,name:task.name,site,urls:task.urls}});return snapshot();
 }
 case 'itemPage':{const url=webURL(data?.url);if(!item||!url)return snapshot();await openPopup({key:'item:'+item.id,url,title:item.name,anchor:anchorY(data)});return snapshot();}
 case 'popupClose':if(popup)await closePopup();return snapshot();
 // "Open in a tab" moves the popup's current page into a tab of its own.
 case 'popupToTab':{
  if(!popup)return snapshot();if(state.tabs.length>=80)throw new Error('tab-limit');
  const next={id:randomUUID(),kind:'web',url:popup.url,title:popup.title};if(popup.task)next.task=structuredClone(popup.task);
  state.tabs.push(next);state.active=next.id;await closePopup();break;
 }
 case 'monitor':{
  if(platform!=='windows'||state.connection.mode!=='local')return snapshot();
  if(host?.monitoring){await go.StopMonitoring();return snapshot();}
  // Without a Screenshots folder there is nothing to watch: the folder
  // settings open instead, where it is chosen (or found again).
  let folder='';try{folder=String((await go.GetSettings()).screenshotDirectory||'');}catch{}
  if(!folder){section='folders';if(tab?.kind!=='settings')returnTo=state.active;openLocal(state,'settings');break;}
  await go.StartMonitoring();return snapshot();
 }
 // The task site is the Host's setting (it also decides what goes to
 // tarkov.dev Remote Control); the browser follows it.
 case 'hostQuestSite':{if(platform!=='windows'||!['tarkov-dev','official-wiki','japanese-wiki'].includes(data))return snapshot();const s=await go.GetSettings();s.questSite=data;await go.PersistSettings(s);hostQuestSite=data;state.questSite='host';break;}
 case 'itemClose':itemOpen=false;update();await persist();await show();return snapshot();
 // The item sidebar opens without an item too: it has a search box.
 case 'itemOpen':itemOpen=true;update();await persist();await show();return snapshot();
 case 'itemSearch':{
  const query=String(data??'').slice(0,80),seq=++searchSeq;itemSearch={...itemSearch,query};
  if(!query.trim()){itemSearch.results=[];update();return snapshot();}
  void (async()=>{let hits;try{hits=await go.BrowserItemSearch(query);}catch(e){messageError(e);return;}
  // A later keystroke's search wins over this one.
  if(seq===searchSeq)itemSearch={query,results:(Array.isArray(hits)?hits:[]).slice(0,20).map(h=>({id:String(h?.id||'').slice(0,40),name:String(h?.name||'').slice(0,160),shortName:String(h?.shortName||'').slice(0,60),names:names(h?.names),iconUrl:webURL(h?.iconUrl)||''})).filter(h=>h.id&&h.name)};
  update();})();
  return snapshot();
 }
 case 'itemSelect':{
  const id=String(data||'');
  void (async()=>{
   let next;try{next=itemInfo(await go.BrowserItemInfo('',id));}catch(e){messageError(e);return;}
   if(next){item=next;itemOpen=true;itemSearch={query:'',results:[]};void loadHistory();await persist().catch(()=>{});}
   update();await show();
  })();
  return snapshot();
 }
 // data.auto: the periodic refresh, which keeps the shown item quietly when
 // the catalog cannot be reached (the refresh button reports it).
 case 'itemRefresh':{
  if(!item||itemBusy)return snapshot();
  // The fetch runs outside the action queue (see refreshItem).
  void refreshItem(!!data?.auto);
  return snapshot();
 }
 // Applying restarts the app and downloading takes a while: neither holds the queue.
 case 'updateInstall':void go.InstallUpdate().catch(messageError);return snapshot();
 case 'updateDownload':void go.DownloadUpdate().catch(messageError);return snapshot();
 case 'updateCheck':void go.CheckForUpdates().catch(messageError);return snapshot();
 case 'updateDismiss':updateDismissed=updateStatus?.latest||'';await show();return snapshot();
 case 'dismiss':error='';update();await show();return snapshot();
 default:return snapshot();
 }
 await changed();return snapshot();
}
window.mayak={
 onState(fn){notify=fn;},onKey(fn){onKey=fn;},
 async action(type,data){await ready;if(type.startsWith('peer'))return pairing(type,data);return enqueue(()=>perform(type,data));},
};
window.addEventListener('beforeunload',()=>peer.close());
